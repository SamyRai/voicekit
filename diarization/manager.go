package diarization

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

const (
	BackendSherpaOffline = "sherpa_offline"
	BackendBasic         = "basic"
)

var ErrEmbeddingExtractorUnavailable = errors.New("diarization embedding extractor is unavailable")

// DiarizationConfig represents configuration for speaker diarization.
type DiarizationConfig struct {
	Enabled bool   `json:"enabled"`
	Backend string `json:"backend"`

	MinSegmentLength      float64 `json:"min_segment_length"`
	MaxSegmentLength      float64 `json:"max_segment_length"`
	SilenceThreshold      float64 `json:"silence_threshold"`
	SimilarityThreshold   float64 `json:"similarity_threshold"`
	MaxSpeakers           int     `json:"max_speakers"`
	ReassignmentThreshold float64 `json:"reassignment_threshold"`
	OverlapThreshold      float64 `json:"overlap_threshold"`

	// SegmentationModelPath and EmbeddingModelPath supply the sherpa offline
	// diarization backend. Recommended (July 2026): pyannote-segmentation-3.0 (or
	// the newer pyannote.audio 4.0 "community-1" pipeline) for segmentation, and a
	// 3D-Speaker CAM++ embedding extractor. Speaker IDs are per-request; to track
	// the same person across recordings, pair diarization with the speaker package
	// and maintain an embedding store.
	SegmentationModelPath string  `json:"segmentation_model_path"`
	EmbeddingModelPath    string  `json:"embedding_model_path"`
	Provider              string  `json:"provider"`
	NumThreads            int     `json:"num_threads"`
	Debug                 bool    `json:"debug"`
	ClusteringThreshold   float32 `json:"clustering_threshold"`
	NumClusters           int     `json:"num_clusters"`
	MinDurationOn         float32 `json:"min_duration_on"`
	MinDurationOff        float32 `json:"min_duration_off"`

	Logger Logger `json:"-"`
}

// DefaultDiarizationConfig returns default diarization configuration.
func DefaultDiarizationConfig() *DiarizationConfig {
	return &DiarizationConfig{
		Enabled:               true,
		Backend:               BackendSherpaOffline,
		MinSegmentLength:      1.0,
		MaxSegmentLength:      30.0,
		SilenceThreshold:      0.5,
		SimilarityThreshold:   0.7,
		MaxSpeakers:           10,
		ReassignmentThreshold: 0.8,
		OverlapThreshold:      0.2,
		Provider:              "cpu",
		NumThreads:            1,
		ClusteringThreshold:   0.7,
		MinDurationOn:         0.3,
		MinDurationOff:        0.5,
		Logger:                &NoOpLogger{},
	}
}

// ApplyDefaults merges zero-value settings with production defaults.
func (c *DiarizationConfig) ApplyDefaults() {
	defaults := DefaultDiarizationConfig()
	c.applyBackendDefaults(defaults)
	c.applySegmentationDefaults(defaults)
	c.applyThresholdDefaults(defaults)
	if c.Logger == nil {
		c.Logger = defaults.Logger
	}
}

func (c *DiarizationConfig) applyBackendDefaults(defaults *DiarizationConfig) {
	if c.Backend == "" {
		c.Backend = defaults.Backend
	}
	if c.Provider == "" {
		c.Provider = defaults.Provider
	}
	if c.NumThreads <= 0 {
		c.NumThreads = defaults.NumThreads
	}
	if c.ClusteringThreshold <= 0 {
		c.ClusteringThreshold = float32(c.SimilarityThreshold)
	}
	if c.MinDurationOn <= 0 {
		c.MinDurationOn = defaults.MinDurationOn
	}
	if c.MinDurationOff <= 0 {
		c.MinDurationOff = defaults.MinDurationOff
	}
}

func (c *DiarizationConfig) applySegmentationDefaults(defaults *DiarizationConfig) {
	if c.MinSegmentLength <= 0 {
		c.MinSegmentLength = defaults.MinSegmentLength
	}
	if c.MaxSegmentLength <= 0 {
		c.MaxSegmentLength = defaults.MaxSegmentLength
	}
	if c.SilenceThreshold <= 0 {
		c.SilenceThreshold = defaults.SilenceThreshold
	}
}

func (c *DiarizationConfig) applyThresholdDefaults(defaults *DiarizationConfig) {
	if c.SimilarityThreshold <= 0 {
		c.SimilarityThreshold = defaults.SimilarityThreshold
	}
	if c.MaxSpeakers <= 0 {
		c.MaxSpeakers = defaults.MaxSpeakers
	}
	if c.ReassignmentThreshold <= 0 {
		c.ReassignmentThreshold = defaults.ReassignmentThreshold
	}
	if c.OverlapThreshold <= 0 {
		c.OverlapThreshold = defaults.OverlapThreshold
	}
}

// Validate validates diarization configuration.
func (c *DiarizationConfig) Validate() error {
	c.ApplyDefaults()
	if !c.Enabled {
		return nil
	}

	var errs []error
	errs = append(errs, c.validateSegmentDurations()...)
	errs = append(errs, c.validateThresholds()...)
	errs = append(errs, c.validateSpeakerLimits()...)
	errs = append(errs, c.validateBackend()...)

	if len(errs) > 0 {
		return fmt.Errorf("diarization config validation failed: %v", errs)
	}
	return nil
}

func (c *DiarizationConfig) validateSegmentDurations() []error {
	var errs []error
	if c.MinSegmentLength <= 0 {
		errs = append(errs, fmt.Errorf("min segment length must be positive, got %f", c.MinSegmentLength))
	}
	if c.MaxSegmentLength <= 0 {
		errs = append(errs, fmt.Errorf("max segment length must be positive, got %f", c.MaxSegmentLength))
	}
	if c.MinSegmentLength >= c.MaxSegmentLength {
		errs = append(errs, fmt.Errorf("min segment length (%f) must be less than max segment length (%f)",
			c.MinSegmentLength, c.MaxSegmentLength))
	}
	if c.SilenceThreshold <= 0 {
		errs = append(errs, fmt.Errorf("silence threshold must be positive, got %f", c.SilenceThreshold))
	}
	if c.SilenceThreshold > 10.0 {
		errs = append(errs, fmt.Errorf("silence threshold must not exceed 10.0 seconds, got %f", c.SilenceThreshold))
	}
	return errs
}

func (c *DiarizationConfig) validateThresholds() []error {
	var errs []error
	if c.SimilarityThreshold < 0.0 || c.SimilarityThreshold > 1.0 {
		errs = append(errs, fmt.Errorf("similarity threshold must be between 0.0-1.0, got %f", c.SimilarityThreshold))
	}
	if c.ReassignmentThreshold < 0.0 || c.ReassignmentThreshold > 1.0 {
		errs = append(errs, fmt.Errorf("reassignment threshold must be between 0.0-1.0, got %f", c.ReassignmentThreshold))
	}
	if c.OverlapThreshold < 0.0 || c.OverlapThreshold > 1.0 {
		errs = append(errs, fmt.Errorf("overlap threshold must be between 0.0-1.0, got %f", c.OverlapThreshold))
	}
	if c.ClusteringThreshold < 0 || c.ClusteringThreshold > 1 {
		errs = append(errs, fmt.Errorf("clustering threshold must be between 0.0-1.0, got %f", c.ClusteringThreshold))
	}
	return errs
}

func (c *DiarizationConfig) validateSpeakerLimits() []error {
	var errs []error
	if c.MaxSpeakers <= 0 {
		errs = append(errs, fmt.Errorf("max speakers must be positive, got %d", c.MaxSpeakers))
	}
	if c.MaxSpeakers > 50 {
		errs = append(errs, fmt.Errorf("max speakers must not exceed 50, got %d", c.MaxSpeakers))
	}
	if c.NumThreads <= 0 {
		errs = append(errs, fmt.Errorf("num threads must be positive, got %d", c.NumThreads))
	}
	if c.NumClusters < 0 {
		errs = append(errs, fmt.Errorf("num clusters cannot be negative, got %d", c.NumClusters))
	}
	return errs
}

func (c *DiarizationConfig) validateBackend() []error {
	var errs []error
	switch c.Backend {
	case BackendSherpaOffline:
		errs = append(errs, validateRequiredFile("segmentation model", c.SegmentationModelPath)...)
		errs = append(errs, validateRequiredFile("embedding model", c.EmbeddingModelPath)...)
	case BackendBasic:
	default:
		errs = append(errs, fmt.Errorf("unsupported diarization backend %q", c.Backend))
	}
	return errs
}

func validateRequiredFile(label, path string) []error {
	if path == "" {
		return []error{fmt.Errorf("%s path is required", label)}
	}
	info, err := os.Stat(path)
	if err != nil {
		return []error{fmt.Errorf("%s path: %w", label, err)}
	}
	if info.IsDir() {
		return []error{fmt.Errorf("%s path %s is a directory, expected file", label, path)}
	}
	return nil
}

// SpeakerSegment represents a segment of audio attributed to a specific speaker.
type SpeakerSegment struct {
	SpeakerID  string    `json:"speaker_id"`
	StartTime  float64   `json:"start_time"`
	EndTime    float64   `json:"end_time"`
	Duration   float64   `json:"duration"`
	Confidence float32   `json:"confidence"`
	Text       string    `json:"text,omitempty"`
	Embedding  []float32 `json:"-"`
}

// DiarizationResult represents the complete diarization result for an audio session.
type DiarizationResult struct {
	SessionID     string           `json:"session_id"`
	Segments      []SpeakerSegment `json:"segments"`
	SpeakerCount  int              `json:"speaker_count"`
	TotalDuration float64          `json:"total_duration"`
	ProcessedAt   time.Time        `json:"processed_at"`
}

// SpeakerTurn represents a speaker turn with timing information.
type SpeakerTurn struct {
	SpeakerID  string  `json:"speaker_id"`
	StartTime  float64 `json:"start_time"`
	EndTime    float64 `json:"end_time"`
	Confidence float32 `json:"confidence"`
}

// AudioSegment represents a segment of audio data.
type AudioSegment struct {
	StartTime float64   `json:"start_time"`
	EndTime   float64   `json:"end_time"`
	Samples   []float32 `json:"-"`
}

// Request is the input contract for diarization backends.
type Request struct {
	Audio       []float32
	SampleRate  int
	SessionID   string
	ProcessedAt time.Time
}

// Backend is the diarization processing boundary.
type Backend interface {
	Process(ctx context.Context, request Request) (*DiarizationResult, error)
	Close() error
}

// Manager handles speaker diarization operations.
type Manager struct {
	config  *DiarizationConfig
	mu      sync.Mutex
	backend Backend
	closed  bool
}

// SpeakerDatabase interface for accessing speaker embeddings.
type SpeakerDatabase interface {
	GetSpeakerEmbedding(speakerID string) ([]float32, error)
	GetAllSpeakers() map[string][]float32
	CalculateSimilarity(embedding1, embedding2 []float32) float32
	RegisterSpeakerEmbedding(speakerID string, embedding []float32) error
}

// EmbeddingExtractor extracts speaker embeddings from audio segments.
type EmbeddingExtractor interface {
	ExtractEmbedding(ctx context.Context, audioData []float32, sampleRate int) ([]float32, error)
}

// Logger interface for diarization operations.
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// NoOpLogger provides a no-op implementation of Logger.
type NoOpLogger struct{}

func (l *NoOpLogger) Infof(format string, args ...any)  {}
func (l *NoOpLogger) Warnf(format string, args ...any)  {}
func (l *NoOpLogger) Errorf(format string, args ...any) {}

// NewManager creates a diarization manager from the configured backend.
func NewManager(config *DiarizationConfig, speakerDB SpeakerDatabase) (*Manager, error) {
	return NewManagerWithBackend(config, speakerDB, nil)
}

// NewManagerWithBackend creates a diarization manager with an optional basic-backend extractor.
func NewManagerWithBackend(config *DiarizationConfig, speakerDB SpeakerDatabase, extractor EmbeddingExtractor) (*Manager, error) {
	if config == nil {
		config = DefaultDiarizationConfig()
	}
	config.ApplyDefaults()
	if err := config.Validate(); err != nil {
		return nil, err
	}

	backend, err := createBackend(config, speakerDB, extractor)
	if err != nil {
		return nil, err
	}
	return &Manager{config: config, backend: backend}, nil
}

func createBackend(config *DiarizationConfig, speakerDB SpeakerDatabase, extractor EmbeddingExtractor) (Backend, error) {
	switch config.Backend {
	case BackendSherpaOffline:
		return newSherpaOfflineBackend(config)
	case BackendBasic:
		return newBasicBackend(config, speakerDB, extractor), nil
	default:
		return nil, fmt.Errorf("unsupported diarization backend %q", config.Backend)
	}
}

// ProcessAudio performs speaker diarization on audio data to identify speaker turns.
func (m *Manager) ProcessAudio(audioData []float32, sampleRate int, sessionID string) (*DiarizationResult, error) {
	return m.ProcessAudioContext(context.Background(), audioData, sampleRate, sessionID)
}

// ProcessAudioContext performs speaker diarization with context cancellation support.
func (m *Manager) ProcessAudioContext(ctx context.Context, audioData []float32, sampleRate int, sessionID string) (*DiarizationResult, error) {
	if m == nil {
		return nil, fmt.Errorf("diarization manager cannot be nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID cannot be empty")
	}
	if len(audioData) == 0 {
		return nil, fmt.Errorf("audioData cannot be empty")
	}
	if sampleRate <= 0 {
		return nil, fmt.Errorf("sampleRate must be positive, got %d", sampleRate)
	}
	if sampleRate < 8000 || sampleRate > 192000 {
		return nil, fmt.Errorf("sampleRate must be between 8000-192000 Hz, got %d", sampleRate)
	}
	if !m.config.Enabled {
		return nil, fmt.Errorf("diarization is disabled")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	m.mu.Lock()
	if m.closed || m.backend == nil {
		m.mu.Unlock()
		return nil, fmt.Errorf("diarization manager is closed")
	}
	backend := m.backend
	m.mu.Unlock()

	return backend.Process(ctx, Request{
		Audio:       audioData,
		SampleRate:  sampleRate,
		SessionID:   sessionID,
		ProcessedAt: time.Now(),
	})
}

// Close releases backend resources.
func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	backend := m.backend
	m.backend = nil
	m.closed = true
	m.mu.Unlock()

	if backend == nil {
		return nil
	}
	return backend.Close()
}

// GetSpeakerTimeline returns speaker turns for timeline visualization.
func (m *Manager) GetSpeakerTimeline(result *DiarizationResult) []SpeakerTurn {
	if result == nil {
		return nil
	}

	turns := make([]SpeakerTurn, len(result.Segments))
	for i, segment := range result.Segments {
		turns[i] = SpeakerTurn{
			SpeakerID:  segment.SpeakerID,
			StartTime:  segment.StartTime,
			EndTime:    segment.EndTime,
			Confidence: segment.Confidence,
		}
	}

	sort.Slice(turns, func(i, j int) bool {
		return turns[i].StartTime < turns[j].StartTime
	})
	return turns
}
