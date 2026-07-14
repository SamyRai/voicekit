package tts

import (
	"fmt"
	"os"
)

const (
	BackendSherpaOffline = "sherpa_offline"

	FamilyVits       = "vits"
	FamilyMatcha     = "matcha"
	FamilyKokoro     = "kokoro"
	FamilyKitten     = "kitten"
	FamilyZipvoice   = "zipvoice"
	FamilyPocket     = "pocket"
	FamilySupertonic = "supertonic"
)

// Logger is the logging contract used by TTS without importing the root package.
type Logger interface {
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// VitsConfig owns Sherpa VITS TTS model paths and generation defaults.
type VitsConfig struct {
	Model       string  `json:"model"`
	Lexicon     string  `json:"lexicon,omitempty"`
	Tokens      string  `json:"tokens"`
	DataDir     string  `json:"data_dir"`
	NoiseScale  float32 `json:"noise_scale,omitempty"`
	NoiseScaleW float32 `json:"noise_scale_w,omitempty"`
	LengthScale float32 `json:"length_scale,omitempty"`
}

// MatchaConfig owns Sherpa Matcha TTS model paths and generation defaults.
type MatchaConfig struct {
	AcousticModel string  `json:"acoustic_model"`
	Vocoder       string  `json:"vocoder"`
	Lexicon       string  `json:"lexicon,omitempty"`
	Tokens        string  `json:"tokens"`
	DataDir       string  `json:"data_dir"`
	NoiseScale    float32 `json:"noise_scale,omitempty"`
	LengthScale   float32 `json:"length_scale,omitempty"`
}

// KokoroConfig owns Sherpa Kokoro TTS model paths and generation defaults.
type KokoroConfig struct {
	Model       string  `json:"model"`
	Voices      string  `json:"voices"`
	Tokens      string  `json:"tokens"`
	DataDir     string  `json:"data_dir"`
	Lexicon     string  `json:"lexicon,omitempty"`
	Lang        string  `json:"lang,omitempty"`
	LengthScale float32 `json:"length_scale,omitempty"`
}

// KittenConfig owns Sherpa KittenTTS model paths and generation defaults.
type KittenConfig struct {
	Model       string  `json:"model"`
	Voices      string  `json:"voices"`
	Tokens      string  `json:"tokens"`
	DataDir     string  `json:"data_dir"`
	LengthScale float32 `json:"length_scale,omitempty"`
}

// ZipvoiceConfig owns Sherpa ZipVoice model paths and generation defaults.
type ZipvoiceConfig struct {
	Tokens        string  `json:"tokens"`
	Encoder       string  `json:"encoder"`
	Decoder       string  `json:"decoder"`
	DataDir       string  `json:"data_dir"`
	Lexicon       string  `json:"lexicon,omitempty"`
	Vocoder       string  `json:"vocoder"`
	FeatScale     float32 `json:"feat_scale,omitempty"`
	TShift        float32 `json:"t_shift,omitempty"`
	TargetRms     float32 `json:"target_rms,omitempty"`
	GuidanceScale float32 `json:"guidance_scale,omitempty"`
}

// PocketConfig owns Sherpa Pocket TTS model paths.
type PocketConfig struct {
	LmFlow                      string `json:"lm_flow"`
	LmMain                      string `json:"lm_main"`
	Encoder                     string `json:"encoder"`
	Decoder                     string `json:"decoder"`
	TextConditioner             string `json:"text_conditioner"`
	VocabJSON                   string `json:"vocab_json"`
	TokenScoresJSON             string `json:"token_scores_json"`
	VoiceEmbeddingCacheCapacity int    `json:"voice_embedding_cache_capacity,omitempty"`
}

// SupertonicConfig owns Sherpa Supertonic TTS model paths.
type SupertonicConfig struct {
	DurationPredictor string `json:"duration_predictor"`
	TextEncoder       string `json:"text_encoder"`
	VectorEstimator   string `json:"vector_estimator"`
	Vocoder           string `json:"vocoder"`
	TtsJSON           string `json:"tts_json"`
	UnicodeIndexer    string `json:"unicode_indexer"`
	VoiceStyle        string `json:"voice_style"`
}

// Config owns TTS runtime configuration.
type Config struct {
	Enabled bool `json:"enabled"`

	Backend     string `json:"backend"`
	ModelFamily string `json:"model_family"`
	Provider    string `json:"provider"`
	NumThreads  int    `json:"num_threads"`
	Debug       bool   `json:"debug"`

	RuleFsts        string  `json:"rule_fsts,omitempty"`
	RuleFars        string  `json:"rule_fars,omitempty"`
	MaxNumSentences int     `json:"max_num_sentences"`
	SilenceScale    float32 `json:"silence_scale"`
	SpeakerID       int     `json:"speaker_id"`
	Speed           float32 `json:"speed"`

	Vits       VitsConfig       `json:"vits"`
	Matcha     MatchaConfig     `json:"matcha"`
	Kokoro     KokoroConfig     `json:"kokoro"`
	Kitten     KittenConfig     `json:"kitten"`
	Zipvoice   ZipvoiceConfig   `json:"zipvoice"`
	Pocket     PocketConfig     `json:"pocket"`
	Supertonic SupertonicConfig `json:"supertonic"`

	Logger Logger `json:"-"`
}

// DefaultConfig returns disabled TTS defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:         false,
		Backend:         BackendSherpaOffline,
		ModelFamily:     FamilyKokoro,
		Provider:        "cpu",
		NumThreads:      1,
		MaxNumSentences: 1,
		SilenceScale:    0.2,
		SpeakerID:       0,
		Speed:           1,
		Vits: VitsConfig{
			NoiseScale:  0.667,
			NoiseScaleW: 0.8,
			LengthScale: 1,
		},
		Matcha: MatchaConfig{
			NoiseScale:  0.667,
			LengthScale: 1,
		},
		Kokoro: KokoroConfig{
			LengthScale: 1,
		},
		Kitten: KittenConfig{
			LengthScale: 1,
		},
		Zipvoice: ZipvoiceConfig{
			FeatScale:     0.1,
			TShift:        0.5,
			TargetRms:     0.1,
			GuidanceScale: 1,
		},
		Pocket: PocketConfig{
			VoiceEmbeddingCacheCapacity: 64,
		},
	}
}

// ApplyDefaults merges zero-value runtime defaults.
func (c *Config) ApplyDefaults() {
	defaults := DefaultConfig()
	if c.Backend == "" {
		c.Backend = defaults.Backend
	}
	if c.ModelFamily == "" {
		c.ModelFamily = defaults.ModelFamily
	}
	if c.Provider == "" {
		c.Provider = defaults.Provider
	}
	if c.NumThreads == 0 {
		c.NumThreads = defaults.NumThreads
	}
	if c.MaxNumSentences == 0 {
		c.MaxNumSentences = defaults.MaxNumSentences
	}
	if c.SilenceScale == 0 {
		c.SilenceScale = defaults.SilenceScale
	}
	if c.Speed == 0 {
		c.Speed = defaults.Speed
	}
	c.applyModelDefaults(defaults)
}

func (c *Config) applyModelDefaults(defaults Config) {
	if c.Vits.NoiseScale <= 0 {
		c.Vits.NoiseScale = defaults.Vits.NoiseScale
	}
	if c.Vits.NoiseScaleW <= 0 {
		c.Vits.NoiseScaleW = defaults.Vits.NoiseScaleW
	}
	if c.Vits.LengthScale <= 0 {
		c.Vits.LengthScale = defaults.Vits.LengthScale
	}
	if c.Matcha.NoiseScale <= 0 {
		c.Matcha.NoiseScale = defaults.Matcha.NoiseScale
	}
	if c.Matcha.LengthScale <= 0 {
		c.Matcha.LengthScale = defaults.Matcha.LengthScale
	}
	if c.Kokoro.LengthScale <= 0 {
		c.Kokoro.LengthScale = defaults.Kokoro.LengthScale
	}
	if c.Kitten.LengthScale <= 0 {
		c.Kitten.LengthScale = defaults.Kitten.LengthScale
	}
	if c.Zipvoice.FeatScale <= 0 {
		c.Zipvoice.FeatScale = defaults.Zipvoice.FeatScale
	}
	if c.Zipvoice.TShift <= 0 {
		c.Zipvoice.TShift = defaults.Zipvoice.TShift
	}
	if c.Zipvoice.TargetRms <= 0 {
		c.Zipvoice.TargetRms = defaults.Zipvoice.TargetRms
	}
	if c.Zipvoice.GuidanceScale <= 0 {
		c.Zipvoice.GuidanceScale = defaults.Zipvoice.GuidanceScale
	}
	if c.Pocket.VoiceEmbeddingCacheCapacity <= 0 {
		c.Pocket.VoiceEmbeddingCacheCapacity = defaults.Pocket.VoiceEmbeddingCacheCapacity
	}
}

// Validate validates TTS configuration. Disabled TTS skips model path checks.
func (c *Config) Validate() error {
	c.ApplyDefaults()
	if !c.Enabled {
		return nil
	}

	var errs []error
	if c.Backend != BackendSherpaOffline {
		errs = append(errs, fmt.Errorf("unsupported TTS backend %q", c.Backend))
	}
	if c.NumThreads <= 0 {
		errs = append(errs, fmt.Errorf("num threads must be positive, got %d", c.NumThreads))
	}
	if c.MaxNumSentences <= 0 {
		errs = append(errs, fmt.Errorf("max num sentences must be positive, got %d", c.MaxNumSentences))
	}
	if c.SilenceScale < 0 {
		errs = append(errs, fmt.Errorf("silence scale cannot be negative, got %f", c.SilenceScale))
	}
	if c.Speed <= 0 {
		errs = append(errs, fmt.Errorf("speed must be positive, got %f", c.Speed))
	}
	if c.SpeakerID < 0 {
		errs = append(errs, fmt.Errorf("speaker ID cannot be negative, got %d", c.SpeakerID))
	}
	errs = append(errs, c.validateModelFamily()...)

	if len(errs) > 0 {
		return fmt.Errorf("TTS config validation failed: %v", errs)
	}
	return nil
}

func (c *Config) validateModelFamily() []error {
	switch c.ModelFamily {
	case FamilyVits:
		return validateVits(c.Vits)
	case FamilyMatcha:
		return validateMatcha(c.Matcha)
	case FamilyKokoro:
		return validateKokoro(c.Kokoro)
	case FamilyKitten:
		return validateKitten(c.Kitten)
	case FamilyZipvoice:
		return validateZipvoice(c.Zipvoice)
	case FamilyPocket:
		return validatePocket(c.Pocket)
	case FamilySupertonic:
		return validateSupertonic(c.Supertonic)
	default:
		return []error{fmt.Errorf("unsupported TTS model family %q", c.ModelFamily)}
	}
}

func validateVits(config VitsConfig) []error {
	errs := requireFileSet("vits", map[string]string{
		"model":  config.Model,
		"tokens": config.Tokens,
	})
	errs = append(errs, requireDir("vits data dir", config.DataDir)...)
	errs = append(errs, validateOptionalFile("vits lexicon", config.Lexicon)...)
	return errs
}

func validateMatcha(config MatchaConfig) []error {
	errs := requireFileSet("matcha", map[string]string{
		"acoustic model": config.AcousticModel,
		"vocoder":        config.Vocoder,
		"tokens":         config.Tokens,
	})
	errs = append(errs, requireDir("matcha data dir", config.DataDir)...)
	errs = append(errs, validateOptionalFile("matcha lexicon", config.Lexicon)...)
	return errs
}

func validateKokoro(config KokoroConfig) []error {
	errs := requireFileSet("kokoro", map[string]string{
		"model":  config.Model,
		"voices": config.Voices,
		"tokens": config.Tokens,
	})
	errs = append(errs, requireDir("kokoro data dir", config.DataDir)...)
	errs = append(errs, validateOptionalFile("kokoro lexicon", config.Lexicon)...)

	// KokoroConfig (and the upstream sherpa-onnx OfflineTtsKokoroModelConfig it
	// maps to) has no explicit "multilingual voice pack" flag, so a Lexicon path
	// is used as the multilingual signal: Kokoro's stock monolingual English
	// pack works with both Lexicon and Lang empty, while multilingual packs
	// (Kokoro >= v1.0) ship a per-language lexicon and require an explicit Lang
	// (e.g. "es", "fr-fr") for correct phonemization — Kokoro rejects synthesis
	// without it. If this proxy ever misfires (e.g. a monolingual config that
	// still sets Lexicon), replace it with a real multilingual flag instead of
	// loosening this check.
	if config.Lexicon != "" && config.Lang == "" {
		errs = append(errs, fmt.Errorf("kokoro lang is required when lexicon is set (multilingual Kokoro voice packs require an explicit language, e.g. \"es\" or \"fr-fr\")"))
	}
	return errs
}

func validateKitten(config KittenConfig) []error {
	errs := requireFileSet("kitten", map[string]string{
		"model":  config.Model,
		"voices": config.Voices,
		"tokens": config.Tokens,
	})
	errs = append(errs, requireDir("kitten data dir", config.DataDir)...)
	return errs
}

func validateZipvoice(config ZipvoiceConfig) []error {
	errs := requireFileSet("zipvoice", map[string]string{
		"tokens":  config.Tokens,
		"encoder": config.Encoder,
		"decoder": config.Decoder,
		"vocoder": config.Vocoder,
	})
	errs = append(errs, requireDir("zipvoice data dir", config.DataDir)...)
	errs = append(errs, validateOptionalFile("zipvoice lexicon", config.Lexicon)...)
	return errs
}

func validatePocket(config PocketConfig) []error {
	return requireFileSet("pocket", map[string]string{
		"lm flow":           config.LmFlow,
		"lm main":           config.LmMain,
		"encoder":           config.Encoder,
		"decoder":           config.Decoder,
		"text conditioner":  config.TextConditioner,
		"vocab json":        config.VocabJSON,
		"token scores json": config.TokenScoresJSON,
	})
}

func validateSupertonic(config SupertonicConfig) []error {
	return requireFileSet("supertonic", map[string]string{
		"duration predictor": config.DurationPredictor,
		"text encoder":       config.TextEncoder,
		"vector estimator":   config.VectorEstimator,
		"vocoder":            config.Vocoder,
		"tts json":           config.TtsJSON,
		"unicode indexer":    config.UnicodeIndexer,
		"voice style":        config.VoiceStyle,
	})
}

func requireFileSet(label string, paths map[string]string) []error {
	errs := make([]error, 0, len(paths))
	for name, path := range paths {
		if path == "" {
			errs = append(errs, fmt.Errorf("%s %s path is required", label, name))
			continue
		}
		if err := requireFile(path); err != nil {
			errs = append(errs, fmt.Errorf("%s %s path: %w", label, name, err))
		}
	}
	return errs
}

func validateOptionalFile(label, path string) []error {
	if path == "" {
		return nil
	}
	if err := requireFile(path); err != nil {
		return []error{fmt.Errorf("%s path: %w", label, err)}
	}
	return nil
}

func requireFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, expected file", path)
	}
	return nil
}

func requireDir(label, path string) []error {
	if path == "" {
		return []error{fmt.Errorf("%s is required", label)}
	}
	info, err := os.Stat(path)
	if err != nil {
		return []error{fmt.Errorf("%s: %w", label, err)}
	}
	if !info.IsDir() {
		return []error{fmt.Errorf("%s %s is a file, expected directory", label, path)}
	}
	return nil
}
