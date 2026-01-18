package voicekit

import (
	"fmt"
	"os"
)

// Config represents the main configuration for voicekit
type Config struct {
	Audio       AudioConfig
	Speaker     SpeakerConfig
	Diarization DiarizationConfig
	ASR         ASRConfig
}

// AudioConfig represents audio processing configuration
type AudioConfig struct {
	SampleRate      int     `json:"sample_rate"`
	Channels        int     `json:"channels"`
	NormalizeFactor float32 `json:"normalize_factor"`
}

// SpeakerConfig represents speaker recognition configuration
type SpeakerConfig struct {
	ModelPath  string  `json:"model_path"`
	NumThreads int     `json:"num_threads"`
	Provider   string  `json:"provider"`
	Threshold  float32 `json:"threshold"`
	DataDir    string  `json:"data_dir"`
	Logger     Logger  `json:"-"`
}

// DiarizationConfig represents diarization configuration
type DiarizationConfig struct {
	Enabled               bool    `json:"enabled"`
	MinSegmentLength      float64 `json:"min_segment_length"`
	MaxSegmentLength      float64 `json:"max_segment_length"`
	SilenceThreshold      float64 `json:"silence_threshold"`
	SimilarityThreshold   float64 `json:"similarity_threshold"`
	MaxSpeakers           int     `json:"max_speakers"`
	ReassignmentThreshold float64 `json:"reassignment_threshold"`
	OverlapThreshold      float64 `json:"overlap_threshold"`
	Logger                Logger  `json:"-"`
}

// ASRConfig represents ASR (Automatic Speech Recognition) configuration
type ASRConfig struct {
	Enabled         bool    `json:"enabled"`
	DefaultModel    string  `json:"default_model"`
	Language        string  `json:"language"`
	Quantization    string  `json:"quantization"` // "int8", "int4", "float16", "float32"
	MaxConcurrentStreams int    `json:"max_concurrent_streams"`
	StreamTimeout   int    `json:"stream_timeout"`   // seconds
	ChunkSize       int    `json:"chunk_size"`       // audio chunk size in samples
	VADProvider     string `json:"vad_provider"`     // "ten_vad", "silero_vad"
	Logger          Logger `json:"-"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Audio: AudioConfig{
			SampleRate:      16000,
			Channels:        1,
			NormalizeFactor: 32768.0,
		},
		Speaker: SpeakerConfig{
			NumThreads: 1,
			Provider:   "cpu",
			Threshold:  0.5,
			Logger:     DefaultLogger(),
		},
		Diarization: DiarizationConfig{
			Enabled:               true,
			MinSegmentLength:      1.0,
			MaxSegmentLength:      30.0,
			SilenceThreshold:      0.5,
			SimilarityThreshold:   0.7,
			MaxSpeakers:           10,
			ReassignmentThreshold: 0.8,
			OverlapThreshold:      0.2,
			Logger:                DefaultLogger(),
		},
		ASR: ASRConfig{
			Enabled:              true,
			DefaultModel:         "whisper_large_v3",
			Language:             "en",
			Quantization:         "int8",
			MaxConcurrentStreams: 10,
			StreamTimeout:        300,  // 5 minutes
			ChunkSize:            16000, // 1 second at 16kHz
			VADProvider:          "ten_vad",
			Logger:               DefaultLogger(),
		},
	}
}

// Validate validates the main configuration
func (c *Config) Validate() error {
	var errs []error

	// Validate audio config
	if err := c.Audio.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("audio config validation failed: %w", err))
	}

	// Validate speaker config
	if err := c.Speaker.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("speaker config validation failed: %w", err))
	}

	// Validate diarization config
	if err := c.Diarization.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("diarization config validation failed: %w", err))
	}

	// Validate ASR config
	if err := c.ASR.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("ASR config validation failed: %w", err))
	}

	// Cross-validation
	if c.Audio.SampleRate <= 0 {
		errs = append(errs, fmt.Errorf("audio sample rate must be positive, got %d", c.Audio.SampleRate))
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration validation failed with %d errors: %v", len(errs), errs)
	}

	return nil
}

// Validate validates audio configuration
func (a *AudioConfig) Validate() error {
	var errs []error

	if a.SampleRate <= 0 {
		errs = append(errs, fmt.Errorf("sample rate must be positive, got %d", a.SampleRate))
	}

	if a.SampleRate < 8000 || a.SampleRate > 192000 {
		errs = append(errs, fmt.Errorf("sample rate must be between 8000-192000 Hz, got %d", a.SampleRate))
	}

	if a.Channels <= 0 {
		errs = append(errs, fmt.Errorf("channels must be positive, got %d", a.Channels))
	}

	if a.Channels > 8 {
		errs = append(errs, fmt.Errorf("channels must not exceed 8, got %d", a.Channels))
	}

	if a.NormalizeFactor <= 0 {
		errs = append(errs, fmt.Errorf("normalize factor must be positive, got %f", a.NormalizeFactor))
	}

	if len(errs) > 0 {
		return fmt.Errorf("audio config validation failed: %v", errs)
	}

	return nil
}

// Validate validates speaker configuration
func (s *SpeakerConfig) Validate() error {
	var errs []error

	if s.NumThreads <= 0 {
		errs = append(errs, fmt.Errorf("num threads must be positive, got %d", s.NumThreads))
	}

	if s.NumThreads > 64 {
		errs = append(errs, fmt.Errorf("num threads must not exceed 64, got %d", s.NumThreads))
	}

	// Validate provider
	validProviders := map[string]bool{
		"cpu":      true,
		"cuda":     true,
		"directml": true,
		"coreml":   true,
		"tensorrt": true,
	}

	if !validProviders[s.Provider] {
		errs = append(errs, fmt.Errorf("invalid provider '%s', must be one of: cpu, cuda, directml, coreml, tensorrt", s.Provider))
	}

	if s.Threshold < 0.0 || s.Threshold > 1.0 {
		errs = append(errs, fmt.Errorf("threshold must be between 0.0-1.0, got %f", s.Threshold))
	}

	// Validate data directory
	if s.DataDir == "" {
		errs = append(errs, fmt.Errorf("data directory cannot be empty"))
	} else {
		// Check if directory exists or can be created
		if _, err := os.Stat(s.DataDir); os.IsNotExist(err) {
			// Try to create the directory to check permissions
			if err := os.MkdirAll(s.DataDir, 0755); err != nil {
				errs = append(errs, fmt.Errorf("cannot create data directory '%s': %v", s.DataDir, err))
			}
		} else if err != nil {
			errs = append(errs, fmt.Errorf("cannot access data directory '%s': %v", s.DataDir, err))
		}
	}

	// Validate model path if provided
	if s.ModelPath != "" {
		if _, err := os.Stat(s.ModelPath); os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("model file does not exist: %s", s.ModelPath))
		} else if err != nil {
			errs = append(errs, fmt.Errorf("cannot access model file '%s': %v", s.ModelPath, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("speaker config validation failed: %v", errs)
	}

	return nil
}

// Validate validates diarization configuration
func (d *DiarizationConfig) Validate() error {
	var errs []error

	if d.MinSegmentLength <= 0 {
		errs = append(errs, fmt.Errorf("min segment length must be positive, got %f", d.MinSegmentLength))
	}

	if d.MaxSegmentLength <= 0 {
		errs = append(errs, fmt.Errorf("max segment length must be positive, got %f", d.MaxSegmentLength))
	}

	if d.MinSegmentLength >= d.MaxSegmentLength {
		errs = append(errs, fmt.Errorf("min segment length (%f) must be less than max segment length (%f)",
			d.MinSegmentLength, d.MaxSegmentLength))
	}

	if d.SilenceThreshold <= 0 {
		errs = append(errs, fmt.Errorf("silence threshold must be positive, got %f", d.SilenceThreshold))
	}

	if d.SilenceThreshold > 10.0 {
		errs = append(errs, fmt.Errorf("silence threshold must not exceed 10.0 seconds, got %f", d.SilenceThreshold))
	}

	if d.SimilarityThreshold < 0.0 || d.SimilarityThreshold > 1.0 {
		errs = append(errs, fmt.Errorf("similarity threshold must be between 0.0-1.0, got %f", d.SimilarityThreshold))
	}

	if d.ReassignmentThreshold < 0.0 || d.ReassignmentThreshold > 1.0 {
		errs = append(errs, fmt.Errorf("reassignment threshold must be between 0.0-1.0, got %f", d.ReassignmentThreshold))
	}

	if d.OverlapThreshold < 0.0 || d.OverlapThreshold > 1.0 {
		errs = append(errs, fmt.Errorf("overlap threshold must be between 0.0-1.0, got %f", d.OverlapThreshold))
	}

	if d.MaxSpeakers <= 0 {
		errs = append(errs, fmt.Errorf("max speakers must be positive, got %d", d.MaxSpeakers))
	}

	if d.MaxSpeakers > 50 {
		errs = append(errs, fmt.Errorf("max speakers must not exceed 50, got %d", d.MaxSpeakers))
	}

	if len(errs) > 0 {
		return fmt.Errorf("diarization config validation failed: %v", errs)
	}

	return nil
}

// Validate validates ASR configuration
func (a *ASRConfig) Validate() error {
	var errs []error

	if a.Enabled {
		if a.DefaultModel == "" {
			errs = append(errs, fmt.Errorf("default model cannot be empty when ASR is enabled"))
		}

		if a.Language == "" {
			errs = append(errs, fmt.Errorf("language cannot be empty when ASR is enabled"))
		}

		// Validate quantization
		validQuantizations := map[string]bool{
			"int4": true, "int8": true, "float16": true, "float32": true,
		}
		if !validQuantizations[a.Quantization] {
			errs = append(errs, fmt.Errorf("invalid quantization: %s, must be one of: int4, int8, float16, float32", a.Quantization))
		}

		if a.MaxConcurrentStreams <= 0 {
			errs = append(errs, fmt.Errorf("max concurrent streams must be positive, got %d", a.MaxConcurrentStreams))
		}

		if a.StreamTimeout <= 0 {
			errs = append(errs, fmt.Errorf("stream timeout must be positive, got %d", a.StreamTimeout))
		}

		if a.ChunkSize <= 0 {
			errs = append(errs, fmt.Errorf("chunk size must be positive, got %d", a.ChunkSize))
		}

		// Validate VAD provider
		validVADProviders := map[string]bool{
			"ten_vad": true, "silero_vad": true, "webrtc_vad": true, "quail_vad": true,
		}
		if !validVADProviders[a.VADProvider] {
			errs = append(errs, fmt.Errorf("invalid VAD provider: %s, must be one of: ten_vad, silero_vad, webrtc_vad, quail_vad", a.VADProvider))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("ASR config validation failed: %v", errs)
	}

	return nil
}