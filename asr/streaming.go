package asr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/SamyRai/voicekit/types"
)

// StreamingManager handles streaming ASR sessions
type StreamingManager struct {
	config        *types.StreamingConfig
	sessions      map[string]*Session
	mu            sync.RWMutex
	metrics       *StreamingMetrics
	cleanupTicker *time.Ticker
	ctx           context.Context
	cancel        context.CancelFunc
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
			ChunkSize:          16000,
			OverlapSize:        1600,
			BufferSize:         48000,
			SampleRate:         16000,
			StreamTimeout:      5 * time.Minute,
			IdleTimeout:        30 * time.Second,
			FlushInterval:      100 * time.Millisecond,
			PartialResults:     true,
			StabilityThreshold: 0.8,
			MinConfidence:      0.5,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	manager := &StreamingManager{
		config:   config,
		sessions: make(map[string]*Session),
		metrics:  &StreamingMetrics{},
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start cleanup routine
	manager.startCleanup()

	return manager
}

// getOrCreateSession returns the session for sessionID, creating it if absent,
// and marks it active. Callers must hold the returned session's mutex while
// operating on its State so those operations are serialized with idle cleanup.
func (m *StreamingManager) getOrCreateSession(sessionID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		session = &Session{
			ID:      sessionID,
			Created: time.Now(),
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

		if m.metrics.ActiveSessions != nil {
			m.metrics.ActiveSessions.Inc()
		}
	} else {
		session.LastActivity = time.Now()
		session.State.LastActivity = session.LastActivity
	}

	return session
}

// GetState gets or creates a streaming state for a session.
func (m *StreamingManager) GetState(sessionID string) (*types.StreamingState, error) {
	return m.getOrCreateSession(sessionID).State, nil
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

	if m.metrics.ActiveSessions != nil {
		m.metrics.ActiveSessions.Dec()
	}
	m.mu.Unlock()

	// Close outside the manager lock, under the session lock, so an in-flight
	// process/finalize on this session finishes before its ASR state is closed.
	session.mu.Lock()
	defer session.mu.Unlock()
	if err := closeASRState(session.State); err != nil {
		return fmt.Errorf("failed to close session ASR state: %w", err)
	}

	return nil
}

// GetActiveSessions returns the number of active sessions
func (m *StreamingManager) GetActiveSessions() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// Close closes the streaming manager
func (m *StreamingManager) Close() error {
	// Cancel context to stop cleanup goroutine
	if m.cancel != nil {
		m.cancel()
	}

	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for sessionID, session := range m.sessions {
		sessions = append(sessions, session)
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()

	// Close each session's ASR state under its own lock so a concurrent
	// process/finalize on that session cannot race with the close.
	var errs []error
	for _, session := range sessions {
		session.mu.Lock()
		if err := closeASRState(session.State); err != nil {
			errs = append(errs, fmt.Errorf("failed to close session %s ASR state: %w", session.ID, err))
		}
		session.mu.Unlock()
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
		if m.metrics.ActiveSessions != nil {
			m.metrics.ActiveSessions.Dec()
		}
	}
	m.mu.Unlock()

	for _, session := range expired {
		session.mu.Lock()
		_ = closeASRState(session.State)
		session.mu.Unlock()
	}
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
