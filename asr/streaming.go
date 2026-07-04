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

// GetState gets or creates a streaming state for a session
func (m *StreamingManager) GetState(sessionID string) (*types.StreamingState, error) {
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

	return session.State, nil
}

// RemoveSession removes a streaming session
func (m *StreamingManager) RemoveSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
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
	defer m.mu.Unlock()

	var errs []error
	for sessionID, session := range m.sessions {
		if err := closeASRState(session.State); err != nil {
			errs = append(errs, fmt.Errorf("failed to close session %s ASR state: %w", sessionID, err))
		}
		delete(m.sessions, sessionID)
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

// cleanupExpiredSessions removes expired sessions
func (m *StreamingManager) cleanupExpiredSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var expiredSessions []string

	for sessionID, session := range m.sessions {
		if now.Sub(session.LastActivity) > m.config.IdleTimeout {
			expiredSessions = append(expiredSessions, sessionID)
			continue
		}

		if now.Sub(session.Created) > m.config.StreamTimeout {
			expiredSessions = append(expiredSessions, sessionID)
			continue
		}
	}

	for _, sessionID := range expiredSessions {
		session := m.sessions[sessionID]

		if m.metrics.SessionTimeouts != nil {
			m.metrics.SessionTimeouts.Inc()
		}

		if m.metrics.SessionDuration != nil {
			duration := time.Since(session.Created).Seconds()
			m.metrics.SessionDuration.Observe(duration)
		}

		_ = closeASRState(session.State)
		delete(m.sessions, sessionID)

		if m.metrics.ActiveSessions != nil {
			m.metrics.ActiveSessions.Dec()
		}
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
