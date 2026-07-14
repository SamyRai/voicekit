package types

import (
	"context"
	"time"
)

// ASRService defines the interface for ASR (Automatic Speech Recognition) services
type ASRService interface {
	// ProcessAudioChunk processes a chunk of audio for streaming ASR
	ProcessAudioChunk(ctx context.Context, sessionID string, audio []float32) (*Transcription, error)

	// FinishStream finalizes the streaming session identified by sessionID,
	// flushing any buffered audio through a final decode and returning a
	// non-partial transcription. Callers invoke it at their own utterance
	// boundary instead of waiting for a VAD endpoint. Finalizing a session
	// that does not exist or has no buffered audio yields an empty,
	// non-partial transcription rather than an error.
	FinishStream(ctx context.Context, sessionID string) (*Transcription, error)

	// RegisterModel registers a new ASR model
	RegisterModel(model ASRModel) error

	// GetModel returns a model by name
	GetModel(name string) (ASRModel, bool)

	// SelectModel selects the best model for given requirements
	SelectModel(language string, requirements *ModelRequirements) (ASRModel, error)

	// Close releases all resources
	Close() error
}

// ASRModel represents an ASR model interface
type ASRModel interface {
	Name() string
	Language() string
	Quantization() string
	ProcessAudio(ctx context.Context, audio []float32, state *StreamingState) (*Transcription, error)
	SupportsLanguage(lang string) bool
	Latency() time.Duration
	Close() error
}

// Transcriber defines batch/offline transcription over complete audio inputs.
type Transcriber interface {
	Transcribe(ctx context.Context, audio []float32, sampleRate int) (*Transcription, error)
	Close() error
}

// Transcription represents a transcription result
type Transcription struct {
	Text       string        `json:"text"`
	IsPartial  bool          `json:"is_partial"`
	Confidence float64       `json:"confidence"`
	Language   string        `json:"language"`
	Emotion    string        `json:"emotion,omitempty"`
	Event      string        `json:"event,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
	Words      []Word        `json:"words,omitempty"`
	StartTime  time.Duration `json:"start_time,omitempty"`
	EndTime    time.Duration `json:"end_time,omitempty"`
}

// Word represents a word-level transcription with timing
type Word struct {
	Text       string        `json:"text"`
	StartTime  time.Duration `json:"start_time"`
	EndTime    time.Duration `json:"end_time"`
	Confidence float64       `json:"confidence"`
}

// StreamingState represents the state of a streaming session
type StreamingState struct {
	SessionID      string
	Language       string
	Buffer         AudioBufferInterface
	VADState       interface{}
	ASRState       interface{}
	LastActivity   time.Time
	PartialResults []Transcription
}

// AudioBufferInterface defines the interface for audio buffering
type AudioBufferInterface interface {
	Append(audio []float32) error
	GetRecentChunk() []float32
	ReturnBuffer(buf []float32)
	IsReady() bool
	Reset()
	Size() int
	Capacity() int
}

// StreamingConfig holds streaming configuration
type StreamingConfig struct {
	ChunkSize          int
	OverlapSize        int
	BufferSize         int
	SampleRate         int
	StreamTimeout      time.Duration
	IdleTimeout        time.Duration
	FlushInterval      time.Duration
	PartialResults     bool
	StabilityThreshold float64
	MinConfidence      float64
}

// ModelRequirements defines requirements for model selection
type ModelRequirements struct {
	MaxLatency     time.Duration
	MinAccuracy    float64
	PreferSpeed    bool
	PreferAccuracy bool
}

// VADConfig holds VAD configuration
type VADConfig struct {
	Provider   string  `json:"provider"`
	Threshold  float64 `json:"threshold"`
	WindowSize int     `json:"window_size"`
	SampleRate int     `json:"sample_rate"`
}

// VADResult represents VAD processing result
type VADResult struct {
	IsSpeech   bool
	IsEndpoint bool
	Confidence float64
	State      interface{}
}
