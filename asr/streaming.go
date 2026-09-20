package asr

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.glpx.pro/voicekit/types"
)

var errSessionRetired = errors.New("streaming session is no longer active")

// StreamingManager handles streaming ASR sessions
type StreamingManager struct {
	config        *types.StreamingConfig
	sessions      map[string]*Session
	activeStreams int
	maxStreams    int
	mu            sync.RWMutex
	metrics       *StreamingMetrics
	cleanupTicker *time.Ticker
	ctx           context.Context
	cancel        context.CancelFunc
	closed        bool
}

// Session represents a streaming ASR session
type Session struct {
	ID           string
	State        *types.StreamingState
	LastActivity time.Time
	Created      time.Time
	Config       *types.StreamingConfig

	// mu serializes operations on State (its Buffer and ASRState) so that an
	// in-flight process/finalize and idle cleanup cannot race on the same
	// session's audio buffer or native ASR stream. The manager's mu guards the
	// sessions map; this guards a single session's mutable state.
	mu sync.Mutex

	// asrModel and asrModelLanguage bind the session to a single ASR model so a
	// native stream (State.ASRState) created by one model is never handed to a
	// different recognizer. The binding is re-evaluated only when the session
	// language changes (see Service.modelForSession). Guarded by mu.
	asrModel         Model
	asrModelLanguage string

	// vadService is owned by this session. Neural VAD detectors retain waveform
	// and endpoint state, so sharing one detector between session IDs would mix
	// unrelated callers. Guarded by mu.
	vadService *VADService

	// acceptedSamples tracks audio submitted to the ASR model in the current
	// utterance. The rolling Buffer is retained for compatibility with custom
	// non-finalizable models, but it is not the source of truth for native
	// streaming progress. Guarded by mu.
	acceptedSamples int64

	// pendingAudio retains a bounded pre-roll until VAD first reports speech.
	// Once an utterance has started, every new chunk is sent directly to ASR.
	// Guarded by mu.
	pendingAudio *vadPreRoll

	// retired prevents operations that were admitted before removal, expiry, or
	// manager shutdown from recreating native resources on an unlinked session.
	// Guarded by mu.
	retired bool

	// admitted records whether this session currently occupies one configured
	// concurrent-stream slot. Guarded by the manager's mu.
	admitted bool

	// operations counts processing/finalization calls that acquired this session
	// but have not completed. It prevents one final call from releasing the slot
	// underneath another same-session call already queued on mu. Guarded by the
	// manager's mu.
	operations int
}

type vadPreRoll struct {
	samples []float32
}

// Use StreamingConfig from types package

// StreamingMetrics holds streaming metrics
type StreamingMetrics struct {
	ActiveSessions  Gauge
	SessionDuration Histogram
	SessionTimeouts Counter
	BufferOverflows Counter
	StateSize       Histogram
}

// NewStreamingManager creates a new streaming manager
func NewStreamingManager(config *types.StreamingConfig) *StreamingManager {
	if config == nil {
		config = &types.StreamingConfig{
			MaxConcurrentStreams: 10,
			ChunkSize:            16000,
			OverlapSize:          1600,
			BufferSize:           48000,
			SampleRate:           16000,
			StreamTimeout:        5 * time.Minute,
			IdleTimeout:          30 * time.Second,
			FlushInterval:        100 * time.Millisecond,
			PartialResults:       true,
			StabilityThreshold:   0.8,
			MinConfidence:        0.5,
		}
	}
	maxStreams := config.MaxConcurrentStreams
	if maxStreams <= 0 {
		maxStreams = 10
	}

	ctx, cancel := context.WithCancel(context.Background())
	manager := &StreamingManager{
		config:     config,
		sessions:   make(map[string]*Session),
		maxStreams: maxStreams,
		metrics:    &StreamingMetrics{},
		ctx:        ctx,
		cancel:     cancel,
	}

	// Start cleanup routine
	manager.startCleanup()

	return manager
}

// getOrCreateSession returns and admits the session for sessionID, creating it
// if absent. Callers must hold the returned session's mutex while operating on
// its State so those operations are serialized with idle cleanup.
func (m *StreamingManager) getOrCreateSession(sessionID string) (*Session, error) {
	return m.admitSession(sessionID, false)
}

// acquireSession admits a session and records one queued/in-flight operation.
// The caller must pair it with completeSessionOperation.
func (m *StreamingManager) acquireSession(sessionID string) (*Session, error) {
	return m.admitSession(sessionID, true)
}

func (m *StreamingManager) admitSession(sessionID string, trackOperation bool) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, fmt.Errorf("streaming manager is closed")
	}

	now := time.Now()
	session, exists := m.sessions[sessionID]
	if exists && session.admitted {
		session.LastActivity = now
		session.State.LastActivity = now
		if trackOperation {
			session.operations++
		}
		return session, nil
	}
	if m.activeStreams >= m.maxStreams {
		return nil, &StreamCapacityError{Limit: m.maxStreams, Active: m.activeStreams}
	}

	if !exists {
		session = &Session{
			ID:      sessionID,
			Created: now,
			Config:  m.config,
		}
		session.LastActivity = session.Created

		session.State = &types.StreamingState{
			SessionID:    sessionID,
			Language:     "en",
			Buffer:       NewAudioBuffer(m.config),
			LastActivity: session.Created,
		}

		m.sessions[sessionID] = session
	} else {
		session.Created = now
		session.LastActivity = now
		session.State.LastActivity = now
	}

	session.admitted = true
	m.activeStreams++
	if m.metrics.ActiveSessions != nil {
		m.metrics.ActiveSessions.Inc()
	}
	if trackOperation {
		session.operations++
	}

	return session, nil
}

// GetState gets or creates a streaming state for a session.
func (m *StreamingManager) GetState(sessionID string) (*types.StreamingState, error) {
	session, err := m.getOrCreateSession(sessionID)
	if err != nil {
		return nil, err
	}
	return session.State, nil
}

// setSessionLanguage sets the language on sessionID's StreamingState,
// creating the session if it does not exist yet. It holds the session's own
// mutex so it cannot race with an in-flight ProcessAudioChunk/FinishStream
// call or idle cleanup on the same session.
func (m *StreamingManager) setSessionLanguage(sessionID, language string) error {
	session, err := m.acquireSession(sessionID)
	if err != nil {
		return err
	}
	session.mu.Lock()
	defer func() {
		m.completeSessionOperation(session, false)
		session.mu.Unlock()
	}()
	if session.retired {
		return fmt.Errorf("streaming session %s is no longer active: %w", sessionID, errSessionRetired)
	}
	session.State.Language = language
	return nil
}

// sessionForFinalize returns an existing session, marking it active so an
// in-flight finalization is not concurrently reaped by the idle cleanup. It does
// not create a session; the second return value reports whether the session
// exists. Callers must hold the returned session's mutex while operating on its
// State.
func (m *StreamingManager) sessionForFinalize(sessionID string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, false
	}
	session.operations++
	session.LastActivity = time.Now()
	session.State.LastActivity = session.LastActivity
	return session, true
}

// RemoveSession removes a streaming session
func (m *StreamingManager) RemoveSession(sessionID string) error {
	m.mu.Lock()

	session, exists := m.sessions[sessionID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("session %s not found", sessionID)
	}

	if m.metrics.SessionDuration != nil {
		duration := time.Since(session.Created).Seconds()
		m.metrics.SessionDuration.Observe(duration)
	}

	delete(m.sessions, sessionID)
	m.mu.Unlock()

	// Close outside the manager lock, under the session lock, so an in-flight
	// process/finalize on this session finishes before its ASR state is closed.
	session.mu.Lock()
	session.retired = true
	err := closeSessionResources(session)
	session.mu.Unlock()
	m.releaseSession(session)
	if err != nil {
		return fmt.Errorf("failed to close session resources: %w", err)
	}

	return nil
}

// GetActiveSessions returns the number of active sessions
func (m *StreamingManager) GetActiveSessions() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeStreams
}

// releaseSession returns a finalized session's admission slot while retaining
// its metadata (including language selection) for later utterances.
func (m *StreamingManager) releaseSession(session *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.releaseSessionLocked(session)
}

// completeSessionOperation releases one queued/in-flight operation. A final
// operation returns the admission slot only when no same-session operation is
// still queued, so capacity cannot be released underneath serialized work.
func (m *StreamingManager) completeSessionOperation(session *Session, final bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if session == nil {
		return
	}
	if session.operations > 0 {
		session.operations--
	}
	if final && session.operations == 0 {
		m.releaseSessionLocked(session)
	}
}

func (m *StreamingManager) releaseSessionLocked(session *Session) {
	if session == nil || !session.admitted {
		return
	}
	session.admitted = false
	if m.activeStreams > 0 {
		m.activeStreams--
	}
	if m.metrics.ActiveSessions != nil {
		m.metrics.ActiveSessions.Dec()
	}
}

// Close closes the streaming manager
func (m *StreamingManager) Close() error {
	// Cancel context to stop cleanup goroutine
	if m.cancel != nil {
		m.cancel()
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	sessions := make([]*Session, 0, len(m.sessions))
	for sessionID, session := range m.sessions {
		sessions = append(sessions, session)
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()

	// Close each session's native resources under its own lock so a concurrent
	// process/finalize on that session cannot race with the close.
	var errs []error
	for _, session := range sessions {
		session.mu.Lock()
		session.retired = true
		if err := closeSessionResources(session); err != nil {
			errs = append(errs, fmt.Errorf("failed to close session %s resources: %w", session.ID, err))
		}
		session.mu.Unlock()
		m.releaseSession(session)
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple streaming close errors: %v", errs)
	}

	return nil
}

// startCleanup starts the session cleanup routine
func (m *StreamingManager) startCleanup() {
	m.cleanupTicker = time.NewTicker(30 * time.Second)

	go func() {
		defer m.cleanupTicker.Stop()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-m.cleanupTicker.C:
				m.cleanupExpiredSessions()
			}
		}
	}()
}

// cleanupExpiredSessions removes expired sessions and closes their ASR state.
func (m *StreamingManager) cleanupExpiredSessions() {
	now := time.Now()

	// Collect and unlink expired sessions under the manager lock, then close
	// their ASR state outside it (under each session's own lock) so cleanup
	// cannot race with an in-flight process/finalize on the same session.
	m.mu.Lock()
	var expired []*Session
	for sessionID, session := range m.sessions {
		idle := now.Sub(session.LastActivity) > m.config.IdleTimeout
		aged := now.Sub(session.Created) > m.config.StreamTimeout
		if !idle && !aged {
			continue
		}

		expired = append(expired, session)
		delete(m.sessions, sessionID)

		if m.metrics.SessionTimeouts != nil {
			m.metrics.SessionTimeouts.Inc()
		}
		if m.metrics.SessionDuration != nil {
			m.metrics.SessionDuration.Observe(time.Since(session.Created).Seconds())
		}
	}
	m.mu.Unlock()

	for _, session := range expired {
		session.mu.Lock()
		session.retired = true
		_ = closeSessionResources(session)
		session.mu.Unlock()
		m.releaseSession(session)
	}
}

func closeSessionResources(session *Session) error {
	if session == nil {
		return nil
	}

	var errs []error
	if err := closeBoundASRState(session); err != nil {
		errs = append(errs, fmt.Errorf("close ASR state: %w", err))
	}
	if session.vadService != nil {
		if err := session.vadService.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close VAD state: %w", err))
		}
		session.vadService = nil
	}
	session.acceptedSamples = 0
	session.pendingAudio = nil
	if session.State != nil && session.State.Buffer != nil {
		session.State.Buffer.Reset()
	}
	return errors.Join(errs...)
}

type stateDiscarder interface {
	discardState(*types.StreamingState) error
}

func closeBoundASRState(session *Session) error {
	if session == nil || session.State == nil || session.State.ASRState == nil {
		return nil
	}
	if discarder, ok := session.asrModel.(stateDiscarder); ok {
		return discarder.discardState(session.State)
	}
	return closeASRState(session.State)
}

func closeASRState(state *types.StreamingState) error {
	if state == nil || state.ASRState == nil {
		return nil
	}
	defer func() {
		state.ASRState = nil
	}()

	closer, ok := state.ASRState.(interface{ Close() error })
	if !ok {
		return nil
	}
	return closer.Close()
}
