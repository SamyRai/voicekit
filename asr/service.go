package asr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"voicekit"
	"voicekit/types"
)

// Service provides real-time streaming ASR functionality
type Service struct {
	config     *voicekit.ASRConfig
	models     map[string]Model
	vadService *VADService
	streaming  *StreamingManager
	metrics    *ASRMetrics

	mu sync.RWMutex
}

// Config holds ASR service configuration
type Config struct {
	DefaultModel         string `json:"default_model"`
	Language             string `json:"language"`
	Quantization         string `json:"quantization"`
	MaxConcurrentStreams int    `json:"max_concurrent_streams"`
	StreamTimeout        time.Duration `json:"stream_timeout"`
	ChunkSize            int    `json:"chunk_size"`
	VADProvider          string `json:"vad_provider"`
}

// Model represents an ASR model interface
type Model interface {
	Name() string
	Language() string
	Quantization() string
	ProcessAudio(ctx context.Context, audio []float32, state *types.StreamingState) (*types.Transcription, error)
	SupportsLanguage(lang string) bool
	Latency() time.Duration
	Close() error
}

// Use types from voicekit/types package

// ASRMetrics holds ASR service metrics
type ASRMetrics struct {
	ProcessLatency   Histogram
	E2ELatency       Histogram
	TTFT             Histogram
	RequestsTotal    Counter
	ActiveStreams    Gauge
	ProcessedChunks  Counter
	ErrorsTotal      Counter
	ConfidenceScore  Histogram
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
func NewService(config *voicekit.ASRConfig) (*Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	service := &Service{
		config:  config,
		models:  make(map[string]Model),
		metrics: &ASRMetrics{},
	}

	// Initialize VAD service
	vadService, err := NewVADService(&VADConfig{
		Provider: config.VADProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize VAD service: %w", err)
	}
	service.vadService = vadService

	// Initialize streaming manager
	streamingConfig := &types.StreamingConfig{
		ChunkSize:          config.ChunkSize,
		OverlapSize:        config.ChunkSize / 10,
		BufferSize:         config.ChunkSize * 3,
		SampleRate:         16000,
		StreamTimeout:      time.Duration(config.StreamTimeout) * time.Second,
		IdleTimeout:        30 * time.Second,
		FlushInterval:      100 * time.Millisecond,
		PartialResults:     true,
		StabilityThreshold: 0.8,
		MinConfidence:      0.5,
	}
	service.streaming = NewStreamingManager(streamingConfig)

	// Register default Whisper model
	whisperModel := NewWhisperModel(config.DefaultModel, config.Language, config.Quantization)
	if err := service.RegisterModel(whisperModel); err != nil {
		return nil, fmt.Errorf("failed to register default model: %w", err)
	}

	if config.Logger != nil {
		config.Logger.Infof("ASR service initialized with model: %s, quantization: %s, VAD: %s",
			config.DefaultModel, config.Quantization, config.VADProvider)
	}

	return service, nil
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
//   - Error if processing fails or context is cancelled
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

	// Get or create streaming state
	state, err := s.streaming.GetState(sessionID)
	if err != nil {
		return nil, NewASRErrorWithSession("streaming_state", sessionID, err)
	}

	// Add audio to buffer
	if err := state.Buffer.Append(audio); err != nil {
		return nil, NewASRErrorWithSession("buffer_append", sessionID, err)
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
					transcription, err := s.finalizeTranscription(ctx, sessionID, state)
					if err != nil {
						return nil, fmt.Errorf("finalization failed: %w", err)
					}
					return transcription, nil
				}

				if !vadResult.IsSpeech {
					return &types.Transcription{
						Text:       "",
						IsPartial:  false,
						Confidence: 0.0,
						Language:   state.Language,
						Timestamp:  time.Now(),
					}, nil
				}
			}
		}
	}

	// Process audio with ASR if buffer is ready
	if state.Buffer.IsReady() {
		transcription, err := s.processWithASR(ctx, sessionID, state)
		if err != nil {
			if s.metrics != nil && s.metrics.ErrorsTotal != nil {
				s.metrics.ErrorsTotal.Inc()
			}
			return nil, NewASRErrorWithSession("processing", sessionID, err)
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
	return &types.Transcription{
		Text:       "",
		IsPartial:  false,
		Confidence: 0.0,
		Language:   state.Language,
		Timestamp:  time.Now(),
	}, nil
}

// processWithASR processes audio using the selected ASR model
func (s *Service) processWithASR(ctx context.Context, sessionID string, state *types.StreamingState) (*types.Transcription, error) {
	// Select appropriate model
	model, err := s.SelectModel(state.Language, &types.ModelRequirements{
		MaxLatency: 500 * time.Millisecond,
	})
	if err != nil {
		var exists bool
		model, exists = s.GetModel(s.config.DefaultModel)
		if !exists {
			return nil, fmt.Errorf("default model not available: %w", err)
		}
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
		return nil, NewASRError("model_processing", err)
	}

	return transcription, nil
}

// finalizeTranscription finalizes transcription when VAD detects endpoint
func (s *Service) finalizeTranscription(ctx context.Context, sessionID string, state *types.StreamingState) (*types.Transcription, error) {
	model, err := s.SelectModel(state.Language, &types.ModelRequirements{
		MaxLatency:     1 * time.Second,
		PreferAccuracy: true,
	})
	if err != nil {
		var exists bool
		model, exists = s.GetModel(s.config.DefaultModel)
		if !exists {
			return nil, fmt.Errorf("default model not available: %w", err)
		}
	}

	chunk := state.Buffer.GetRecentChunk()
	if chunk == nil {
		return nil, fmt.Errorf("no audio chunk available for finalization")
	}
	defer state.Buffer.ReturnBuffer(chunk)

	// Create a copy for finalization (since the model may modify it)
	remainingAudio := make([]float32, len(chunk))
	copy(remainingAudio, chunk)

	finalResult, err := model.ProcessAudio(ctx, remainingAudio, state)
	if err != nil {
		return nil, fmt.Errorf("finalization failed: %w", err)
	}

	finalResult.IsPartial = false
	finalResult.Timestamp = time.Now()

	state.Buffer.Reset()
	state.LastActivity = time.Now()

	return finalResult, nil
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

	if s.streaming != nil {
		if err := s.streaming.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close streaming manager: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple close errors: %v", errs)
	}

	return nil
}