package asr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.glpx.pro/voicekit/types"
)

// Service provides real-time streaming ASR functionality
type Service struct {
	config    *Config
	models    map[string]Model
	streaming *StreamingManager
	metrics   *ASRMetrics

	mu     sync.RWMutex
	closed bool

	// vadPrototype validates native VAD construction at service startup and is
	// transferred to the first streaming session. Later sessions receive their
	// own detector from vadConfig so stateful neural VAD buffers are never shared
	// across session IDs.
	vadMu        sync.Mutex
	vadConfig    *VADConfig
	vadPrototype *VADService
}

type finalizableModel interface {
	FinishAudio(ctx context.Context, audio []float32, state *types.StreamingState) (*types.Transcription, error)
}

// Model represents an ASR model interface
type Model = types.ASRModel

// Use types from voicekit/types package

// ASRMetrics holds ASR service metrics
type ASRMetrics struct {
	ProcessLatency  Histogram
	E2ELatency      Histogram
	TTFT            Histogram
	RequestsTotal   Counter
	ActiveStreams   Gauge
	ProcessedChunks Counter
	ErrorsTotal     Counter
	ConfidenceScore Histogram
}

// Define basic metric interfaces to avoid circular imports
type Histogram interface {
	Observe(value float64)
}

type Counter interface {
	Inc()
	Add(delta float64)
}

type Gauge interface {
	Set(value float64)
	Inc()
	Dec()
	Add(delta float64)
}

// NewService creates a new ASR service
func NewService(config *Config) (*Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	config.ApplyDefaults()
	if !config.Enabled {
		return nil, fmt.Errorf("ASR service cannot be initialized when ASR is disabled")
	}
	if config.Backend != BackendSherpaOnline {
		return nil, fmt.Errorf("streaming ASR service requires backend %q, got %q", BackendSherpaOnline, config.Backend)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid ASR config: %w", err)
	}

	service := &Service{
		config:  config,
		models:  make(map[string]Model),
		metrics: &ASRMetrics{},
	}

	if config.VADProvider != VADProviderNone {
		vadConfig := vadConfigFromASR(config)
		vadService, err := NewVADService(vadConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize VAD service: %w", err)
		}
		service.vadConfig = vadConfig
		service.vadPrototype = vadService
	}

	// Initialize streaming manager
	streamingConfig := &types.StreamingConfig{
		ChunkSize:          config.ChunkSize,
		OverlapSize:        config.ChunkSize / 10,
		BufferSize:         config.ChunkSize * 3,
		SampleRate:         config.SampleRate,
		StreamTimeout:      time.Duration(config.StreamTimeout) * time.Second,
		IdleTimeout:        30 * time.Second,
		FlushInterval:      100 * time.Millisecond,
		PartialResults:     true,
		StabilityThreshold: 0.8,
		MinConfidence:      0.5,
	}
	service.streaming = NewStreamingManager(streamingConfig)

	// Build one SherpaOnlineModel per resolved online config entry: either
	// every Config.OnlineModels item, or a single model expanded from the
	// legacy Config.Online field for backward compatibility.
	for _, oc := range config.resolvedOnlineModelConfigs() {
		model, err := newSherpaOnlineModelFromOnline(config, oc)
		if err != nil {
			return nil, service.initializationError(fmt.Errorf("failed to initialize Sherpa ASR model %q: %w", oc.Name, err))
		}
		if err := service.RegisterModel(model); err != nil {
			_ = model.Close()
			return nil, service.initializationError(fmt.Errorf("failed to register model %q: %w", oc.Name, err))
		}
	}

	if config.Logger != nil {
		config.Logger.Infof("ASR service initialized with %d online model(s) (default: %s), quantization: %s, VAD: %s",
			len(service.models), config.DefaultModel, config.Quantization, config.VADProvider)
	}

	return service, nil
}

func (s *Service) initializationError(initializationErr error) error {
	if cleanupErr := s.Close(); cleanupErr != nil {
		return fmt.Errorf("%w; cleanup failed: %v", initializationErr, cleanupErr)
	}
	return initializationErr
}

// SetSessionLanguage sets the language used to route sessionID's subsequent
// ProcessAudioChunk/FinishStream calls to a language-matching model via
// SelectModel (see processWithASR/finalizeTranscription). It creates the
// session if it does not exist yet, so callers may set the language before
// the first audio chunk arrives. Sessions default to "en" until this is
// called.
//
// This only changes which already-registered model a session's calls are
// routed to; it does not perform language identification itself (see the
// planned LanguageIdentifier follow-on).
func (s *Service) SetSessionLanguage(sessionID, language string) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}
	if language == "" {
		return fmt.Errorf("language cannot be empty")
	}
	return s.streaming.setSessionLanguage(sessionID, language)
}

// RegisterModel registers a new ASR model
func (s *Service) RegisterModel(model Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("ASR service is closed")
	}

	name := model.Name()
	if _, exists := s.models[name]; exists {
		return fmt.Errorf("model %s already registered", name)
	}

	s.models[name] = model
	if s.config.Logger != nil {
		s.config.Logger.Infof("Registered ASR model: %s (language: %s, quantization: %s)",
			name, model.Language(), model.Quantization())
	}

	return nil
}

// GetModel returns a model by name
func (s *Service) GetModel(name string) (Model, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, false
	}

	model, exists := s.models[name]
	return model, exists
}

// ProcessAudioChunk processes a chunk of audio for real-time streaming ASR transcription.
//
// This function handles incremental audio processing for continuous speech recognition.
// It buffers audio chunks, performs voice activity detection, and generates transcriptions
// with low latency (<500ms end-to-end for typical configurations).
//
// The processing pipeline:
// 1. Buffers audio chunks in a streaming session
// 2. Performs VAD (Voice Activity Detection) if enabled
// 3. Triggers ASR processing when sufficient audio is available
// 4. Returns partial results for ongoing speech, final results for completed utterances
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - sessionID: unique identifier for the streaming session
//   - audio: float32 audio samples (typically 1 second at 16kHz)
//
// Returns:
//   - Transcription with text, confidence, and partial/final status
//   - Error if processing fails or context is canceled
//
// Session management: sessions are automatically created on first call and
// cleaned up after periods of inactivity. Multiple concurrent sessions supported.
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (s *Service) ProcessAudioChunk(ctx context.Context, sessionID string, audio []float32) (*types.Transcription, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID cannot be empty")
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("audio data cannot be empty")
	}

	startTime := time.Now()
	defer func() {
		if s.metrics != nil && s.metrics.ProcessLatency != nil {
			s.metrics.ProcessLatency.Observe(time.Since(startTime).Seconds())
		}
	}()

	// Get or create the session and hold its lock for the duration of the
	// operation so buffer/ASR-state mutations are serialized with idle cleanup
	// and any concurrent finalize on the same session.
	session, err := s.streaming.getOrCreateSession(sessionID)
	if err != nil {
		return nil, newSessionError("session", sessionID, err)
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	state := session.State

	// Add audio to buffer
	if err := state.Buffer.Append(audio); err != nil {
		return nil, newSessionError("buffer_append", sessionID, err)
	}

	// Process only the newly submitted audio through the session-owned VAD.
	// Neural detectors retain waveform state, so each session owns a distinct
	// VADService and passes only its own incremental samples.
	if s.vadConfig != nil {
		vadService, err := s.vadForSession(session)
		if err != nil {
			return nil, newSessionError("vad_initialization", sessionID, err)
		}
		vadResult, err := vadService.Process(audio, state.VADState)
		if err != nil {
			return nil, newSessionError("vad_processing", sessionID, err)
		}
		state.VADState = vadResult.State

		if vadResult.IsEndpoint {
			transcription, err := s.finalizeTranscription(ctx, session, audio)
			if err != nil {
				return nil, fmt.Errorf("finalization failed: %w", err)
			}
			return transcription, nil
		}

		if !vadResult.IsSpeech {
			return emptyFinalTranscription(state.Language), nil
		}
	}

	// Online recognizers consume each caller-provided chunk exactly once. The
	// rolling buffer remains only as a compatibility fallback for custom models
	// that do not implement finalization.
	transcription, err := s.processWithASR(ctx, session, audio)
	if err != nil {
		if s.metrics != nil && s.metrics.ErrorsTotal != nil {
			s.metrics.ErrorsTotal.Inc()
		}
		return nil, newSessionError("processing", sessionID, err)
	}

	if s.metrics != nil {
		if s.metrics.ProcessedChunks != nil {
			s.metrics.ProcessedChunks.Inc()
		}
		if transcription.Confidence > 0 && s.metrics.ConfidenceScore != nil {
			s.metrics.ConfidenceScore.Observe(transcription.Confidence)
		}
	}

	if !transcription.IsPartial {
		state.Buffer.Reset()
		session.acceptedSamples = 0
		if err := closeSessionVAD(session); err != nil {
			return nil, newSessionError("vad_reset", sessionID, err)
		}
	}
	return transcription, nil
}

// FinishStream finalizes the streaming session identified by sessionID.
//
// It signals InputFinished to the underlying model without replaying previously
// accepted audio, flushes the recognizer's internal feature buffers, and returns
// a non-partial transcription. Callers use it to force a final hypothesis at
// their own utterance boundary instead of waiting for a VAD endpoint; it is the
// public entry point to the same finalization path the VAD endpoint takes.
//
// Finalizing a session that does not exist, or one with no accepted speech,
// yields an empty non-partial transcription rather than an error, so callers
// may finalize idempotently.
//
// Thread-safe: can be called concurrently with ProcessAudioChunk for other
// sessions.
func (s *Service) FinishStream(ctx context.Context, sessionID string) (*types.Transcription, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID cannot be empty")
	}

	session, ok := s.streaming.sessionForFinalize(sessionID)
	if !ok {
		return emptyFinalTranscription("en"), nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()

	state := session.State
	if session.acceptedSamples == 0 {
		state.Buffer.Reset()
		if err := closeSessionVAD(session); err != nil {
			return nil, newSessionError("vad_reset", sessionID, err)
		}
		return emptyFinalTranscription(state.Language), nil
	}

	transcription, err := s.finalizeTranscription(ctx, session, nil)
	if err != nil {
		if s.metrics != nil && s.metrics.ErrorsTotal != nil {
			s.metrics.ErrorsTotal.Inc()
		}
		return nil, newSessionError("finalization", sessionID, err)
	}
	return transcription, nil
}

// emptyFinalTranscription builds a non-partial transcription with no text,
// used when there is nothing to emit for a session.
func emptyFinalTranscription(language string) *types.Transcription {
	return &types.Transcription{
		Text:       "",
		IsPartial:  false,
		Confidence: 0.0,
		Language:   language,
		Timestamp:  time.Now(),
	}
}

// modelForSession returns the ASR model bound to the session. It selects a model
// on first use and re-selects only when the session language changes; on a model
// change it closes the session's native ASR stream so the new model builds a
// fresh stream under its own recognizer (a native stream must never cross
// recognizers). Callers must hold session.mu.
func (s *Service) modelForSession(session *Session, reqs *types.ModelRequirements) (Model, error) {
	language := session.State.Language

	if session.asrModel != nil && session.asrModelLanguage == language {
		if _, ok := s.GetModel(session.asrModel.Name()); ok {
			return session.asrModel, nil
		}
	}

	model, err := s.SelectModel(language, reqs)
	if err != nil {
		var exists bool
		model, exists = s.GetModel(s.config.DefaultModel)
		if !exists {
			return nil, fmt.Errorf("default model not available: %w", err)
		}
	}

	// A model change orphans any native stream the previous model created for
	// this session; close it so the new model does not inherit a foreign stream.
	if session.asrModel != nil && session.asrModel.Name() != model.Name() {
		_ = closeASRState(session.State)
		session.acceptedSamples = 0
	}
	session.asrModel = model
	session.asrModelLanguage = language
	return model, nil
}

// processWithASR processes audio using the session's bound ASR model.
func (s *Service) processWithASR(ctx context.Context, session *Session, audio []float32) (*types.Transcription, error) {
	state := session.State
	model, err := s.modelForSession(session, &types.ModelRequirements{
		MaxLatency: 500 * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}

	transcription, err := model.ProcessAudio(ctx, audio, state)
	if err != nil {
		return nil, newError("model_processing", err)
	}
	session.acceptedSamples += int64(len(audio))

	return transcription, nil
}

// finalizeTranscription finalizes transcription when VAD detects an endpoint or
// FinishStream is called.
func (s *Service) finalizeTranscription(ctx context.Context, session *Session, finalAudio []float32) (*types.Transcription, error) {
	state := session.State
	model, err := s.modelForSession(session, &types.ModelRequirements{
		MaxLatency:     1 * time.Second,
		PreferAccuracy: true,
	})
	if err != nil {
		return nil, err
	}

	finalResult, err := processFinalAudio(ctx, model, finalAudio, state)
	if err != nil {
		return nil, fmt.Errorf("finalization failed: %w", err)
	}

	finalResult.IsPartial = false
	finalResult.Timestamp = time.Now()

	state.Buffer.Reset()
	state.LastActivity = time.Now()
	session.acceptedSamples = 0
	if err := closeSessionVAD(session); err != nil {
		return nil, fmt.Errorf("reset VAD after finalization: %w", err)
	}

	return finalResult, nil
}

func processFinalAudio(ctx context.Context, model Model, audio []float32, state *types.StreamingState) (*types.Transcription, error) {
	if finalizable, ok := model.(finalizableModel); ok {
		return finalizable.FinishAudio(ctx, audio, state)
	}
	if len(audio) == 0 && state != nil && state.Buffer != nil {
		chunk := state.Buffer.GetRecentChunk()
		if chunk == nil {
			return nil, fmt.Errorf("no audio available for finalization")
		}
		defer state.Buffer.ReturnBuffer(chunk)
		audio = chunk
	}
	return model.ProcessAudio(ctx, audio, state)
}

func (s *Service) vadForSession(session *Session) (*VADService, error) {
	if session.vadService != nil {
		return session.vadService, nil
	}

	s.vadMu.Lock()
	defer s.vadMu.Unlock()
	if s.vadPrototype != nil {
		session.vadService = s.vadPrototype
		s.vadPrototype = nil
		return session.vadService, nil
	}
	if s.vadConfig == nil {
		return nil, fmt.Errorf("VAD is not configured")
	}
	config := *s.vadConfig
	service, err := NewVADService(&config)
	if err != nil {
		return nil, err
	}
	session.vadService = service
	return service, nil
}

func closeSessionVAD(session *Session) error {
	if session == nil || session.vadService == nil {
		return nil
	}
	err := session.vadService.Close()
	session.vadService = nil
	session.State.VADState = nil
	return err
}

// SelectModel selects the best model for given requirements
func (s *Service) SelectModel(language string, requirements *types.ModelRequirements) (Model, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, fmt.Errorf("ASR service is closed")
	}

	var bestModel Model
	var bestScore float64

	for _, model := range s.models {
		if !model.SupportsLanguage(language) {
			continue
		}

		score := s.calculateModelScore(model, requirements)
		if bestModel == nil || score > bestScore {
			bestModel = model
			bestScore = score
		}
	}

	if bestModel == nil {
		return nil, fmt.Errorf("no suitable model found for language %s", language)
	}

	return bestModel, nil
}

// ModelRequirements is defined in types package

// calculateModelScore calculates how well a model matches requirements
func (s *Service) calculateModelScore(model Model, req *types.ModelRequirements) float64 {
	score := 1.0

	if req.MaxLatency > 0 && model.Latency() > req.MaxLatency {
		score *= 0.5
	}

	if req.PreferSpeed && model.Latency() < 500*time.Millisecond {
		score *= 1.2
	}

	if req.PreferAccuracy && model.Quantization() == "float32" {
		score *= 1.1
	}

	return score
}

// Close releases all resources
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	models := make(map[string]Model, len(s.models))
	for name, model := range s.models {
		models[name] = model
	}
	s.mu.Unlock()

	var errs []error

	if s.streaming != nil {
		if err := s.streaming.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close streaming manager: %w", err))
		}
	}

	for name, model := range models {
		if err := model.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close model %s: %w", name, err))
		}
	}

	s.vadMu.Lock()
	if s.vadPrototype != nil {
		if err := s.vadPrototype.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close VAD prototype: %w", err))
		}
		s.vadPrototype = nil
	}
	s.vadMu.Unlock()

	if len(errs) > 0 {
		return fmt.Errorf("multiple close errors: %v", errs)
	}

	return nil
}
