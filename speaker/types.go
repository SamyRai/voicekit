package speaker

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/SamyRai/voicekit/speaker/application"
	"github.com/SamyRai/voicekit/speaker/domain"
)

// SpeakerData represents speaker data structure
type SpeakerData struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Embeddings [][]float32 `json:"embeddings"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	// LastUsedAt is refreshed on every successful IdentifySpeaker/VerifySpeaker
	// match. It is initialized to CreatedAt at registration and is the sort
	// key for RetentionLRU.
	LastUsedAt  time.Time `json:"last_used_at"`
	SampleCount int       `json:"sample_count"`
}

// SpeakerDatabaseData represents speaker database structure for JSON serialization
type SpeakerDatabaseData struct {
	Speakers  map[string]*SpeakerData `json:"speakers"`
	Version   string                  `json:"version"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// Manager represents speaker recognition manager with clean architecture
type Manager struct {
	// Application layer use cases
	recognitionUseCase *application.SpeakerRecognitionUseCase
	managementUseCase  *application.SpeakerManagementUseCase

	// Legacy infrastructure (for backward compatibility)
	extractor   SpeakerEmbeddingExtractor
	manager     SpeakerEmbeddingManager
	database    SpeakerDatabase
	vectorIndex domain.VectorIndex // Generic vector index for fast ANN search

	// Configuration
	threshold    float32
	embeddingDim int
	dataDir      string
	logger       Logger
	maxSpeakers  int // 0 = unlimited; otherwise the retention cap enforced on registration
	// retentionPolicy selects which speaker is evicted first once maxSpeakers
	// is exceeded. Defaults to RetentionFIFO (see NewManager).
	retentionPolicy RetentionPolicy
	// maxAge, when >0, additionally evicts speakers idle longer than this
	// (by LastUsedAt), independent of maxSpeakers/retentionPolicy.
	maxAge time.Duration

	// retentionMu serializes eviction so concurrent registrations do not over-evict.
	retentionMu sync.Mutex

	// Atomic counters for statistics (lock-free)
	identifyRequests  int64 // Total identification requests
	identifySuccesses int64 // Successful identifications
	verifyRequests    int64 // Total verification requests
	verifySuccesses   int64 // Successful verifications
	registrationCount int64 // Total speaker registrations
	errorCount        int64 // Total errors

	// Mutex for legacy operations
	mutex sync.RWMutex
}

// Config represents speaker recognition configuration
type Config struct {
	ModelPath  string  `json:"model_path"`
	NumThreads int     `json:"num_threads"`
	Provider   string  `json:"provider"`
	Threshold  float32 `json:"threshold"`
	DataDir    string  `json:"data_dir"`
	// MaxSpeakers bounds the database size. When >0, registering a new speaker
	// beyond the cap evicts speakers chosen by RetentionPolicy. 0 means unlimited.
	MaxSpeakers int `json:"max_speakers"`
	// RetentionPolicy selects which speaker is evicted first once MaxSpeakers
	// is exceeded. Empty defaults to RetentionFIFO (oldest CreatedAt first),
	// matching the behavior shipped before RetentionPolicy existed.
	// RetentionLRU evicts the least-recently-used speaker (by LastUsedAt,
	// refreshed on every successful identify/verify match) first.
	RetentionPolicy RetentionPolicy `json:"retention_policy"`
	// MaxAge, when >0, additionally evicts any speaker idle longer than this
	// (by LastUsedAt), independent of MaxSpeakers/RetentionPolicy. 0 (the
	// default) disables TTL-based eviction.
	MaxAge time.Duration `json:"max_age"`
	Logger Logger        `json:"-"`
}

// Validate validates speaker configuration
func (c *Config) Validate() error {
	if c.NumThreads <= 0 {
		return fmt.Errorf("num threads must be positive, got %d", c.NumThreads)
	}
	if c.NumThreads > 64 {
		return fmt.Errorf("num threads must not exceed 64, got %d", c.NumThreads)
	}

	// Validate provider
	validProviders := map[string]bool{
		"cpu": true, "cuda": true, "directml": true, "coreml": true, "tensorrt": true,
	}
	if !validProviders[c.Provider] {
		return fmt.Errorf("invalid provider '%s', must be one of: cpu, cuda, directml, coreml, tensorrt", c.Provider)
	}

	if c.Threshold < 0.0 || c.Threshold > 1.0 {
		return fmt.Errorf("threshold must be between 0.0-1.0, got %f", c.Threshold)
	}

	if c.MaxSpeakers < 0 {
		return fmt.Errorf("max speakers cannot be negative, got %d", c.MaxSpeakers)
	}

	switch c.RetentionPolicy {
	case "", RetentionFIFO, RetentionLRU:
		// valid
	default:
		return fmt.Errorf("invalid retention policy '%s', must be one of: %s, %s", c.RetentionPolicy, RetentionFIFO, RetentionLRU)
	}

	if c.MaxAge < 0 {
		return fmt.Errorf("max age cannot be negative, got %s", c.MaxAge)
	}

	// Validate data directory
	if c.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}

	// Validate model path if provided
	if c.ModelPath != "" {
		if _, err := os.Stat(c.ModelPath); os.IsNotExist(err) {
			return fmt.Errorf("model file does not exist: %s", c.ModelPath)
		}
	}

	return nil
}

// IdentifyResult represents speaker identification result
type IdentifyResult struct {
	Identified  bool    `json:"identified"`
	SpeakerID   string  `json:"speaker_id"`
	SpeakerName string  `json:"speaker_name"`
	Confidence  float32 `json:"confidence"`
	Threshold   float32 `json:"threshold"`
}

// VerifyResult represents speaker verification result
type VerifyResult struct {
	SpeakerID   string  `json:"speaker_id"`
	SpeakerName string  `json:"speaker_name"`
	Verified    bool    `json:"verified"`
	Confidence  float32 `json:"confidence"`
	Threshold   float32 `json:"threshold"`
}

// SpeakerInfo represents speaker information
type SpeakerInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	SampleCount int       `json:"sample_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// LastUsedAt is the timestamp of the most recent successful identify or
	// verify match against this speaker; it is the sort key RetentionLRU
	// uses to pick eviction victims.
	LastUsedAt time.Time `json:"last_used_at"`
}

// DatabaseStats represents database statistics
type DatabaseStats struct {
	TotalSpeakers int       `json:"total_speakers"`
	TotalSamples  int       `json:"total_samples"`
	EmbeddingDim  int       `json:"embedding_dim"`
	Threshold     float32   `json:"threshold"`
	Version       string    `json:"version"`
	UpdatedAt     time.Time `json:"updated_at"`
}
