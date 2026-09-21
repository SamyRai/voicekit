package voicekit

import (
	"fmt"
	"os"
	"time"

	"go.glpx.pro/gdk/voice/asr"
	"go.glpx.pro/gdk/voice/denoise"
	"go.glpx.pro/gdk/voice/diarization"
	"go.glpx.pro/gdk/voice/speaker"
	"go.glpx.pro/gdk/voice/tts"
)

// Config represents the main configuration for voicekit
type Config struct {
	Audio       AudioConfig
	Speaker     SpeakerConfig
	Diarization DiarizationConfig
	ASR         ASRConfig
	TTS         TTSConfig
	// Denoiser is optional; when a GTCRN/DPDFNet model path is set, the full
	// ProcessAudio pipeline denoises samples before speaker/diarization.
	Denoiser DenoiserConfig
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
	// MaxSpeakers bounds the speaker database; >0 evicts on overflow by
	// RetentionPolicy. 0 (default) means unlimited.
	MaxSpeakers int `json:"max_speakers"`
	// RetentionPolicy selects the eviction order (empty = FIFO by CreatedAt;
	// LRU by LastUsedAt).
	RetentionPolicy speaker.RetentionPolicy `json:"retention_policy"`
	// MaxAge, when >0, evicts speakers idle longer than this (by LastUsedAt).
	MaxAge time.Duration `json:"max_age"`
	Logger Logger        `json:"-"`
}

// DenoiserConfig is the root-facing alias for the speech-denoiser configuration.
type DenoiserConfig = denoise.DenoiserConfig

// DiarizationConfig is the root-facing alias for diarization-owned runtime configuration.
type DiarizationConfig = diarization.DiarizationConfig

// ASRConfig is the root-facing alias for ASR-owned runtime configuration.
type ASRConfig = asr.Config

// OnlineConfig is the root-facing alias for Sherpa online ASR model paths.
type OnlineConfig = asr.OnlineConfig

// OfflineConfig is the root-facing alias for Sherpa offline ASR model paths.
type OfflineConfig = asr.OfflineConfig

// TTSConfig is the root-facing alias for TTS-owned runtime configuration.
type TTSConfig = tts.Config

// TTS model family configs are root-facing aliases for Sherpa offline TTS paths.
type TTSVitsConfig = tts.VitsConfig
type TTSMatchaConfig = tts.MatchaConfig
type TTSKokoroConfig = tts.KokoroConfig
type TTSKittenConfig = tts.KittenConfig
type TTSZipvoiceConfig = tts.ZipvoiceConfig
type TTSPocketConfig = tts.PocketConfig
type TTSSupertonicConfig = tts.SupertonicConfig

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
			DataDir:    ".voicekit/speakers",
			Logger:     DefaultLogger(),
		},
		Diarization: defaultRootDiarizationConfig(),
		ASR:         asr.DefaultConfig(),
		TTS:         tts.DefaultConfig(),
	}
}

func defaultRootDiarizationConfig() DiarizationConfig {
	config := *diarization.DefaultDiarizationConfig()
	config.Enabled = false
	config.Logger = DefaultLogger()
	return config
}

// ApplyDefaults merges zero-value runtime defaults while preserving explicit booleans.
func (c *Config) ApplyDefaults() {
	defaults := DefaultConfig()

	if c.Audio.SampleRate <= 0 {
		c.Audio.SampleRate = defaults.Audio.SampleRate
	}
	if c.Audio.Channels <= 0 {
		c.Audio.Channels = defaults.Audio.Channels
	}
	if c.Audio.NormalizeFactor <= 0 {
		c.Audio.NormalizeFactor = defaults.Audio.NormalizeFactor
	}

	if c.Speaker.NumThreads <= 0 {
		c.Speaker.NumThreads = defaults.Speaker.NumThreads
	}
	if c.Speaker.Provider == "" {
		c.Speaker.Provider = defaults.Speaker.Provider
	}
	if c.Speaker.Threshold <= 0 {
		c.Speaker.Threshold = defaults.Speaker.Threshold
	}
	if c.Speaker.ModelPath != "" && c.Speaker.DataDir == "" {
		c.Speaker.DataDir = defaults.Speaker.DataDir
	}
	if c.Speaker.Logger == nil {
		c.Speaker.Logger = defaults.Speaker.Logger
	}

	c.Diarization.ApplyDefaults()
	if c.Diarization.Backend == "" {
		c.Diarization.Backend = defaults.Diarization.Backend
	}
	if c.Diarization.Logger == nil {
		c.Diarization.Logger = defaults.Diarization.Logger
	}

	c.ASR.ApplyDefaults()
	if c.ASR.Logger == nil {
		c.ASR.Logger = DefaultLogger()
	}

	c.TTS.ApplyDefaults()
	if c.TTS.Logger == nil {
		c.TTS.Logger = DefaultLogger()
	}
}

// Validate validates the main configuration
func (c *Config) Validate() error {
	c.ApplyDefaults()
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

	// Validate TTS config
	if err := c.TTS.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("TTS config validation failed: %w", err))
	}

	// Validate the optional denoiser only when a model path is configured.
	if c.Denoiser.GtcrnModel != "" || c.Denoiser.DpdfNetModel != "" {
		if err := c.Denoiser.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("denoiser config validation failed: %w", err))
		}
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

	if s.ModelPath == "" {
		return nil
	}

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
