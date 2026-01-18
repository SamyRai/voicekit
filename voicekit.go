// Package voicekit provides a unified interface for voice processing operations
// including speaker recognition, diarization, and audio processing.
package voicekit

import (
	"context"
	"fmt"
	"time"

	"github.com/SamyRai/voicekit/audio"
	"github.com/SamyRai/voicekit/diarization"
	"github.com/SamyRai/voicekit/speaker"
	"github.com/SamyRai/voicekit/types"
)

// VoiceKit provides a unified interface for all voice processing operations
type VoiceKit struct {
	config         *Config
	speakerManager *speaker.Manager
	audioConverter *audio.Converter
	diarizationMgr *diarization.Manager
	asrService     ASRService
	metrics        *MetricsCollector
}

// NewVoiceKit creates a new VoiceKit instance with the provided configuration
func NewVoiceKit(config *Config) (*VoiceKit, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Validate configuration at initialization
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize speaker manager if configured
	var speakerMgr *speaker.Manager
	if config.Speaker.ModelPath != "" {
		speakerConfig := &speaker.Config{
			ModelPath:  config.Speaker.ModelPath,
			NumThreads: config.Speaker.NumThreads,
			Provider:   config.Speaker.Provider,
			Threshold:  config.Speaker.Threshold,
			DataDir:    config.Speaker.DataDir,
			Logger:     config.Speaker.Logger,
		}

		var err error
		speakerMgr, err = speaker.NewManager(speakerConfig)
		if err != nil {
			return nil, err
		}
	}

	// Initialize audio converter
	audioConfig := &audio.ConverterConfig{
		EnableResampling:    true,
		EnableNormalization: true,
		TargetRMS:           0.1,
		TempBufferSize:      8192, // Default buffer size
		NormalizeFactor:     float32(config.Audio.NormalizeFactor),
	}
	audioConverter, err := audio.NewConverter(audioConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create audio converter: %w", err)
	}

	// Initialize diarization manager
	diarizationConfig := &diarization.DiarizationConfig{
		Enabled:               config.Diarization.Enabled,
		MinSegmentLength:      config.Diarization.MinSegmentLength,
		MaxSegmentLength:      config.Diarization.MaxSegmentLength,
		SilenceThreshold:      config.Diarization.SilenceThreshold,
		SimilarityThreshold:   config.Diarization.SimilarityThreshold,
		MaxSpeakers:           config.Diarization.MaxSpeakers,
		ReassignmentThreshold: config.Diarization.ReassignmentThreshold,
		OverlapThreshold:      config.Diarization.OverlapThreshold,
		Logger:                config.Diarization.Logger,
	}

	var diarizationMgr *diarization.Manager
	if speakerMgr != nil {
		// Create a speaker database adapter
		speakerDB := &speakerDatabaseAdapter{manager: speakerMgr}
		diarizationMgr = diarization.NewManager(diarizationConfig, speakerDB)
	} else {
		// Create a mock speaker database for diarization
		mockDB := &mockSpeakerDatabase{}
		diarizationMgr = diarization.NewManager(diarizationConfig, mockDB)
	}

	// Initialize ASR service if enabled
	var asrService ASRService
	if config.ASR.Enabled {
		// ASR service will be initialized separately and set via SetASRService
		// This avoids circular imports
		asrService = nil // Will be set later
	}

	return &VoiceKit{
		config:         config,
		speakerManager: speakerMgr,
		audioConverter: audioConverter,
		diarizationMgr: diarizationMgr,
		asrService:     asrService,
		metrics:        NewMetricsCollector(),
	}, nil
}

// ASRService defines the interface for ASR services
type ASRService interface {
	ProcessAudioChunk(ctx context.Context, sessionID string, audio []float32) (*types.Transcription, error)
	RegisterModel(model ASRModel) error
	GetModel(name string) (ASRModel, bool)
	SelectModel(language string, requirements *types.ModelRequirements) (ASRModel, error)
	Close() error
}

// ASRModel represents an ASR model interface
type ASRModel interface {
	Name() string
	Language() string
	Quantization() string
	ProcessAudio(ctx context.Context, audio []float32, state *types.StreamingState) (*types.Transcription, error)
	SupportsLanguage(lang string) bool
	Latency() time.Duration
	Close() error
}

// Close releases all resources held by VoiceKit
func (vk *VoiceKit) Close() {
	if vk.speakerManager != nil {
		vk.speakerManager.Close()
	}
	if vk.asrService != nil {
		vk.asrService.Close()
	}
}

// Speaker returns the speaker recognition manager
func (vk *VoiceKit) Speaker() *speaker.Manager {
	return vk.speakerManager
}

// Audio returns audio processing utilities
func (vk *VoiceKit) Audio() *audio.Converter {
	return vk.audioConverter
}

// Diarization returns the diarization manager
func (vk *VoiceKit) Diarization() *diarization.Manager {
	return vk.diarizationMgr
}

// Metrics returns the metrics collector
func (vk *VoiceKit) Metrics() *MetricsCollector {
	return vk.metrics
}

// ASR returns the ASR service
func (vk *VoiceKit) ASR() ASRService {
	return vk.asrService
}

// SetASRService sets the ASR service (used to avoid circular imports)
func (vk *VoiceKit) SetASRService(service ASRService) {
	vk.asrService = service
}

// ProcessAudio processes audio data through the full voice pipeline
// This is a convenience method that combines audio conversion, speaker recognition, and diarization
func (vk *VoiceKit) ProcessAudio(audioData []byte, inputConfig *audio.AudioConfig, sessionID string) (*speaker.IdentifyResult, *diarization.DiarizationResult, error) {
	if len(audioData) == 0 {
		return nil, nil, fmt.Errorf("audioData cannot be empty")
	}
	if inputConfig == nil {
		return nil, nil, fmt.Errorf("inputConfig cannot be nil")
	}
	if sessionID == "" {
		return nil, nil, fmt.Errorf("sessionID cannot be empty")
	}

	startTime := time.Now()

	// Convert audio to float32
	float32Data, err := vk.audioConverter.ConvertToFloat32(audioData, inputConfig)
	audioProcessTime := time.Since(startTime)
	vk.metrics.RecordAudioProcessing(audioProcessTime, err == nil)

	if err != nil {
		return nil, nil, err
	}

	// Speaker identification
	var speakerResult *speaker.IdentifyResult
	if vk.speakerManager != nil {
		speakerStartTime := time.Now()
		speakerResult, err = vk.speakerManager.IdentifySpeaker(float32Data, inputConfig.SampleRate)
		speakerProcessTime := time.Since(speakerStartTime)
		vk.metrics.RecordSpeakerIdentify(speakerProcessTime, err == nil)

		if err != nil {
			return nil, nil, err
		}
	}

	// Diarization
	diarizationStartTime := time.Now()
	diarizationResult, err := vk.diarizationMgr.ProcessAudio(float32Data, inputConfig.SampleRate, sessionID)
	diarizationProcessTime := time.Since(diarizationStartTime)
	vk.metrics.RecordDiarization(diarizationProcessTime, err == nil)

	if err != nil {
		return nil, nil, err
	}

	return speakerResult, diarizationResult, nil
}

// NewSpeakerManager creates a standalone speaker recognition manager
func NewSpeakerManager(config *speaker.Config) (*speaker.Manager, error) {
	return speaker.NewManager(config)
}

// NewAudioConverter creates a standalone audio converter
func NewAudioConverter(config *audio.ConverterConfig) (*audio.Converter, error) {
	return audio.NewConverter(config)
}

// NewDiarizationManager creates a standalone diarization manager
func NewDiarizationManager(config *diarization.DiarizationConfig, speakerDB diarization.SpeakerDatabase) *diarization.Manager {
	return diarization.NewManager(config, speakerDB)
}

// NewAudioResampler creates a standalone audio resampler
func NewAudioResampler(config *audio.ResampleConfig) *audio.Resampler {
	return audio.NewResampler(config)
}

// NewDiarizationSegmenter creates a standalone diarization segmenter
func NewDiarizationSegmenter(config *diarization.DiarizationConfig) *diarization.Segmenter {
	return diarization.NewSegmenter(config)
}

// NewDiarizationIntegrator creates a standalone diarization integrator
func NewDiarizationIntegrator(config *diarization.DiarizationConfig, manager *diarization.Manager) *diarization.Integrator {
	return diarization.NewIntegrator(config, manager)
}

// NewSpeakerParser creates a standalone speaker audio parser
func NewSpeakerParser() *speaker.AudioParser {
	return speaker.NewAudioParser()
}

// speakerDatabaseAdapter adapts speaker.Manager to diarization.SpeakerDatabase interface
type speakerDatabaseAdapter struct {
	manager *speaker.Manager
}

func (a *speakerDatabaseAdapter) GetSpeakerEmbedding(speakerID string) ([]float32, error) {
	return a.manager.GetSpeakerEmbedding(speakerID)
}

func (a *speakerDatabaseAdapter) GetAllSpeakers() map[string][]float32 {
	return a.manager.GetAllSpeakersEmbeddings()
}

func (a *speakerDatabaseAdapter) CalculateSimilarity(embedding1, embedding2 []float32) float32 {
	return speaker.CosineSimilarity(embedding1, embedding2)
}

func (a *speakerDatabaseAdapter) RegisterSpeakerEmbedding(speakerID string, embedding []float32) error {
	return a.manager.RegisterSpeakerEmbedding(speakerID, embedding)
}

// mockSpeakerDatabase provides a mock implementation for diarization when no speaker manager is available
type mockSpeakerDatabase struct{}

func (m *mockSpeakerDatabase) GetSpeakerEmbedding(speakerID string) ([]float32, error) {
	return nil, ErrSpeakerNotFound
}

func (m *mockSpeakerDatabase) GetAllSpeakers() map[string][]float32 {
	return make(map[string][]float32)
}

func (m *mockSpeakerDatabase) CalculateSimilarity(embedding1, embedding2 []float32) float32 {
	// Simple mock similarity calculation
	if len(embedding1) == 0 || len(embedding2) == 0 {
		return 0.0
	}
	return 0.5 // Mock similarity
}

func (m *mockSpeakerDatabase) RegisterSpeakerEmbedding(speakerID string, embedding []float32) error {
	return nil
}