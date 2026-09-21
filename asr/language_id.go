package asr

import (
	"context"
	"fmt"
	"os"
	"sync"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
	"go.glpx.pro/gdk/voice/types"
)

// LanguageIDConfig owns Sherpa spoken language identification model
// configuration. The Whisper-based language ID model only needs an encoder
// and decoder (unlike Whisper ASR, its native config has no tokens field:
// see SpokenLanguageIdentificationWhisperConfig in the vendored binding) —
// it predicts a language code directly rather than decoding text tokens.
type LanguageIDConfig struct {
	EncoderPath string `json:"encoder_path"`
	DecoderPath string `json:"decoder_path"`

	// TailPaddings controls how many tail padding samples are appended before
	// inference. -1 tells Sherpa to use its internal default.
	TailPaddings int `json:"tail_paddings,omitempty"`

	Provider   string `json:"provider"`
	NumThreads int    `json:"num_threads"`
	Debug      bool   `json:"debug"`
}

// DefaultLanguageIDConfig returns runtime defaults; EncoderPath/DecoderPath
// are left empty since they must be supplied by the caller.
func DefaultLanguageIDConfig() LanguageIDConfig {
	return LanguageIDConfig{
		Provider:     "cpu",
		NumThreads:   1,
		TailPaddings: -1,
	}
}

// ApplyDefaults merges zero-value runtime settings with conservative
// defaults.
func (c *LanguageIDConfig) ApplyDefaults() {
	defaults := DefaultLanguageIDConfig()
	if c.Provider == "" {
		c.Provider = defaults.Provider
	}
	if c.NumThreads <= 0 {
		c.NumThreads = defaults.NumThreads
	}
	if c.TailPaddings == 0 {
		c.TailPaddings = defaults.TailPaddings
	}
}

// Validate validates language ID configuration, including model path checks.
func (c *LanguageIDConfig) Validate() error {
	c.ApplyDefaults()

	var errs []error
	if c.NumThreads <= 0 {
		errs = append(errs, fmt.Errorf("num threads must be positive, got %d", c.NumThreads))
	}
	if c.EncoderPath == "" {
		errs = append(errs, fmt.Errorf("language ID encoder path is required"))
	} else if err := requireLanguageIDFile(c.EncoderPath); err != nil {
		errs = append(errs, fmt.Errorf("language ID encoder path: %w", err))
	}
	if c.DecoderPath == "" {
		errs = append(errs, fmt.Errorf("language ID decoder path is required"))
	} else if err := requireLanguageIDFile(c.DecoderPath); err != nil {
		errs = append(errs, fmt.Errorf("language ID decoder path: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("language ID config validation failed: %v", errs)
	}
	return nil
}

func requireLanguageIDFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, expected file", path)
	}
	return nil
}

// SherpaLanguageIdentifier implements types.LanguageIdentifier with Sherpa
// ONNX's Whisper-based spoken language identification model. It is a
// single-shot/batch identifier over a complete audio segment, mirroring
// Transcriber rather than the streaming-session-scoped ASR/keyword-spotter
// interfaces.
type SherpaLanguageIdentifier struct {
	config     *LanguageIDConfig
	identifier nativeLanguageIdentifier
	mu         sync.Mutex
	closed     bool
}

// nativeLanguageIdentifier is the mockable seam over the native Sherpa
// SpokenLanguageIdentification binding, so tests can inject a fake without
// loading real model weights.
type nativeLanguageIdentifier interface {
	CreateStream() (lidStream, error)
	Compute(stream lidStream) (string, error)
	Close() error
}

// lidStream is the mockable seam over the native Sherpa OfflineStream used
// to feed audio into the language identifier.
type lidStream interface {
	AcceptWaveform(sampleRate int, samples []float32) error
	Close() error
}

// NewSherpaLanguageIdentifier validates config and constructs a language
// identifier backed by a native Sherpa SpokenLanguageIdentification
// instance.
func NewSherpaLanguageIdentifier(config *LanguageIDConfig) (*SherpaLanguageIdentifier, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	config.ApplyDefaults()
	if err := config.Validate(); err != nil {
		return nil, err
	}

	nativeConfig := buildSpokenLanguageIdentificationConfig(config)
	native := sherpa.NewSpokenLanguageIdentification(nativeConfig)
	if native == nil {
		return nil, fmt.Errorf("failed to create Sherpa spoken language identification")
	}

	return &SherpaLanguageIdentifier{
		config:     config,
		identifier: &nativeSpokenLanguageIdentifier{slid: native},
	}, nil
}

func buildSpokenLanguageIdentificationConfig(config *LanguageIDConfig) *sherpa.SpokenLanguageIdentificationConfig {
	return &sherpa.SpokenLanguageIdentificationConfig{
		Whisper: sherpa.SpokenLanguageIdentificationWhisperConfig{
			Encoder:      config.EncoderPath,
			Decoder:      config.DecoderPath,
			TailPaddings: config.TailPaddings,
		},
		NumThreads: config.NumThreads,
		Debug:      boolToInt(config.Debug),
		Provider:   config.Provider,
	}
}

// Identify detects the spoken language of a complete audio segment.
func (l *SherpaLanguageIdentifier) Identify(ctx context.Context, audio []float32, sampleRate int) (result *types.LanguageResult, err error) {
	if l == nil {
		return nil, fmt.Errorf("language identifier cannot be nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("audio cannot be empty")
	}
	if sampleRate <= 0 {
		return nil, fmt.Errorf("sampleRate must be positive, got %d", sampleRate)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.identifier == nil {
		return nil, fmt.Errorf("sherpa language identifier is closed")
	}

	stream, err := l.identifier.CreateStream()
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := stream.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if err := stream.AcceptWaveform(sampleRate, audio); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	lang, err := l.identifier.Compute(stream)
	if err != nil {
		return nil, err
	}

	return &types.LanguageResult{
		Language: lang,
		// The vendored SpokenLanguageIdentificationResult only carries the
		// predicted language code, not a confidence score, so Score is left
		// at its zero value (matching Transcription.Confidence: 0 for the
		// offline ASR result, which has the same binding limitation).
		Score: 0,
	}, nil
}

// Close releases the native language identifier. It is safe to call more
// than once.
func (l *SherpaLanguageIdentifier) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	if l.identifier != nil {
		if err := l.identifier.Close(); err != nil {
			return err
		}
		l.identifier = nil
	}
	l.closed = true
	return nil
}

var _ types.LanguageIdentifier = (*SherpaLanguageIdentifier)(nil)

type nativeSpokenLanguageIdentifier struct {
	slid *sherpa.SpokenLanguageIdentification
}

func (n *nativeSpokenLanguageIdentifier) CreateStream() (lidStream, error) {
	if n == nil || n.slid == nil {
		return nil, fmt.Errorf("sherpa language identifier is closed")
	}
	stream := n.slid.CreateStream()
	if stream == nil {
		return nil, fmt.Errorf("failed to create Sherpa language ID stream")
	}
	return &nativeLIDStream{stream: stream}, nil
}

func (n *nativeSpokenLanguageIdentifier) Compute(stream lidStream) (string, error) {
	if n == nil || n.slid == nil {
		return "", fmt.Errorf("sherpa language identifier is closed")
	}
	native, ok := stream.(*nativeLIDStream)
	if !ok || native.stream == nil {
		return "", fmt.Errorf("unsupported language ID stream implementation")
	}
	result := n.slid.Compute(native.stream)
	if result == nil {
		return "", fmt.Errorf("sherpa language identification failed")
	}
	return result.Lang, nil
}

func (n *nativeSpokenLanguageIdentifier) Close() error {
	if n == nil || n.slid == nil {
		return nil
	}
	sherpa.DeleteSpokenLanguageIdentification(n.slid)
	n.slid = nil
	return nil
}

type nativeLIDStream struct {
	stream *sherpa.OfflineStream
}

func (s *nativeLIDStream) AcceptWaveform(sampleRate int, samples []float32) error {
	if s == nil || s.stream == nil {
		return fmt.Errorf("sherpa language ID stream is closed")
	}
	if sampleRate <= 0 {
		return fmt.Errorf("sampleRate must be positive, got %d", sampleRate)
	}
	if len(samples) == 0 {
		return fmt.Errorf("audio cannot be empty")
	}
	s.stream.AcceptWaveform(sampleRate, samples)
	return nil
}

func (s *nativeLIDStream) Close() error {
	if s == nil || s.stream == nil {
		return nil
	}
	sherpa.DeleteOfflineStream(s.stream)
	s.stream = nil
	return nil
}
