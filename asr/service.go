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
	config     *Config
	models     map[string]Model
	vadService *VADService
	streaming  *StreamingManager
	metrics    *ASRMetrics

	mu sync.RWMutex
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
		vadService, err := NewVADService(vadConfigFromASR(config))
		if err != nil {
			return nil, fmt.Errorf("failed to initialize VAD service: %w", err)
		}
		service.vadService = vadService
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
			return nil, fmt.Errorf("failed to initialize Sherpa ASR model %q: %w", oc.Name, err)
		}
		if err := service.RegisterModel(model); err != nil {
			return nil, fmt.Errorf("failed to register model %q: %w", oc.Name, err)
		}
	}

	if config.Logger != nil {
		config.Logger.Infof("ASR service initialized with %d online model(s) (default: %s), quantization: %s, VAD: %s",
			len(service.models), config.DefaultModel, config.Quantization, config.VADProvider)
	}

	return service, nil
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
	s.streaming.setSessionLanguage(sessionID, language)
	return nil
}

// RegisterModel registers a new ASR model
func (s *Service) RegisterModel(model Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	session := s.streaming.getOrCreateSession(sessionID)
	session.mu.Lock()
	defer session.mu.Unlock()
	state := session.State

	// Add audio to buffer
	if err := state.Buffer.Append(audio); err != nil {
		return nil, newSessionError("buffer_append", sessionID, err)
	}

	// Process VAD if available
	if s.vadService != nil && state.Buffer.IsReady() {
		chunk := state.Buffer.GetRecentChunk()
		if chunk != nil {
			defer state.Buffer.ReturnBuffer(chunk) // Return buffer to pool when done
			vadResult, err := s.vadService.Process(chunk, state.VADState)
			if err != nil {
				if s.config.Logger != nil {
					s.config.Logger.Warnf("VAD processing failed: %v", err)
				}
			} else {
				state.VADState = vadResult

				// Handle VAD events
				if vadResult.IsEndpoint {
					transcription, err := s.finalizeTranscription(ctx, session)
					if err != nil {
						return nil, fmt.Errorf("finalization failed: %w", err)
					}
					return transcription, nil
				}

				if !vadResult.IsSpeech {
					return emptyFinalTranscription(state.Language), nil
				}
			}
		}
	}

	// Process audio with ASR if buffer is ready
	if state.Buffer.IsReady() {
		transcription, err := s.processWithASR(ctx, session)
		if err != nil {
			if s.metrics != nil && s.metrics.ErrorsTotal != nil {
				s.metrics.ErrorsTotal.Inc()
			}
			return nil, newSessionError("processing", sessionID, err)
		}

		// Update metrics
		if s.metrics != nil {
			if s.metrics.ProcessedChunks != nil {
				s.metrics.ProcessedChunks.Inc()
			}
			if transcription.Confidence > 0 && s.metrics.ConfidenceScore != nil {
				s.metrics.ConfidenceScore.Observe(transcription.Confidence)
			}
		}

		return transcription, nil
	}

	// Buffer not ready yet, return empty result
	return emptyFinalTranscription(state.Language), nil
}

// FinishStream finalizes the streaming session identified by sessionID.
//
// It flushes any buffered audio through a final decode (signaling
// InputFinished to the underlying model) and returns a non-partial
// transcription. Callers use it to force a final hypothesis at their own
// utterance boundary instead of waiting for a VAD endpoint; it is the public
// entry point to the same finalization path the VAD endpoint takes internally.
//
// Finalizing a session that does not exist, or one with no buffered audio,
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
	if state.Buffer == nil || state.Buffer.Size() == 0 {
		return emptyFinalTranscription(state.Language), nil
	}

	transcription, err := s.finalizeTranscription(ctx, session)
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
	}
	session.asrModel = model
	session.asrModelLanguage = language
	return model, nil
}

// processWithASR processes audio using the session's bound ASR model.
func (s *Service) processWithASR(ctx context.Context, session *Session) (*types.Transcription, error) {
	state := session.State
	model, err := s.modelForSession(session, &types.ModelRequirements{
		MaxLatency: 500 * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}

	// Get audio chunk for processing
	chunk := state.Buffer.GetRecentChunk()
	if chunk == nil {
		return nil, fmt.Errorf("no audio chunk available")
	}
	defer state.Buffer.ReturnBuffer(chunk) // Return buffer to pool when done

	// Process audio with selected model
	transcription, err := model.ProcessAudio(ctx, chunk, state)
	if err != nil {
		return nil, newError("model_processing", err)
	}

	return transcription, nil
}

// finalizeTranscription finalizes transcription when VAD detects an endpoint or
// FinishStream is called.
func (s *Service) finalizeTranscription(ctx context.Context, session *Session) (*types.Transcription, error) {
	state := session.State
	model, err := s.modelForSession(session, &types.ModelRequirements{
		MaxLatency:     1 * time.Second,
		PreferAccuracy: true,
	})
	if err != nil {
		return nil, err
	}

	chunk := state.Buffer.GetRecentChunk()
	if chunk == nil {
		return nil, fmt.Errorf("no audio chunk available for finalization")
	}
	defer state.Buffer.ReturnBuffer(chunk)

	// Create a copy for finalization (since the model may modify it)
	remainingAudio := make([]float32, len(chunk))
	copy(remainingAudio, chunk)

	finalResult, err := processFinalAudio(ctx, model, remainingAudio, state)
	if err != nil {
		return nil, fmt.Errorf("finalization failed: %w", err)
	}

	finalResult.IsPartial = false
	finalResult.Timestamp = time.Now()

	state.Buffer.Reset()
	state.LastActivity = time.Now()

	return finalResult, nil
}

func processFinalAudio(ctx context.Context, model Model, audio []float32, state *types.StreamingState) (*types.Transcription, error) {
	if finalizable, ok := model.(finalizableModel); ok {
		return finalizable.FinishAudio(ctx, audio, state)
	}
	return model.ProcessAudio(ctx, audio, state)
}

// SelectModel selects the best model for given requirements
func (s *Service) SelectModel(language string, requirements *types.ModelRequirements) (Model, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
	defer s.mu.Unlock()

	var errs []error

	if s.streaming != nil {
		if err := s.streaming.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close streaming manager: %w", err))
		}
	}

	for name, model := range s.models {
		if err := model.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close model %s: %w", name, err))
		}
	}

	if s.vadService != nil {
		if err := s.vadService.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close VAD service: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple close errors: %v", errs)
	}

	return nil
}
