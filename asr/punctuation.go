package asr

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
	"go.glpx.pro/gdk/voice/types"
)

// PunctuationConfig owns Sherpa offline punctuation model configuration. The
// offline punctuation model (a ct-transformer ONNX model) is a single file,
// unlike the multi-file ASR/TTS model sets.
type PunctuationConfig struct {
	ModelPath  string `json:"model_path"`
	Provider   string `json:"provider"`
	NumThreads int    `json:"num_threads"`
	Debug      bool   `json:"debug"`
}

// DefaultPunctuationConfig returns runtime defaults; ModelPath is left empty
// since it must be supplied by the caller.
func DefaultPunctuationConfig() PunctuationConfig {
	return PunctuationConfig{
		Provider:   "cpu",
		NumThreads: 1,
	}
}

// ApplyDefaults merges zero-value runtime defaults.
func (c *PunctuationConfig) ApplyDefaults() {
	defaults := DefaultPunctuationConfig()
	if c.Provider == "" {
		c.Provider = defaults.Provider
	}
	if c.NumThreads <= 0 {
		c.NumThreads = defaults.NumThreads
	}
}

// Validate validates punctuation configuration, including model path checks.
func (c *PunctuationConfig) Validate() error {
	c.ApplyDefaults()

	var errs []error
	if c.NumThreads <= 0 {
		errs = append(errs, fmt.Errorf("num threads must be positive, got %d", c.NumThreads))
	}
	if c.ModelPath == "" {
		errs = append(errs, fmt.Errorf("punctuation model path is required"))
	} else if err := requirePunctuationFile(c.ModelPath); err != nil {
		errs = append(errs, fmt.Errorf("punctuation model path: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("punctuation config validation failed: %v", errs)
	}
	return nil
}

func requirePunctuationFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, expected file", path)
	}
	return nil
}

// SherpaPunctuation implements types.Punctuation with Sherpa ONNX's offline
// ct-transformer punctuation model, restoring punctuation and casing in raw
// ASR transcript text.
type SherpaPunctuation struct {
	config      *PunctuationConfig
	punctuation offlinePunctuation
	mu          sync.Mutex
	closed      bool
}

// offlinePunctuation is the mockable seam over the native Sherpa binding, so
// tests can inject a fake without loading real model weights.
type offlinePunctuation interface {
	AddPunct(text string) string
	Close() error
}

// NewSherpaPunctuation validates config and constructs a punctuation
// restorer backed by a native Sherpa OfflinePunctuation instance.
func NewSherpaPunctuation(config *PunctuationConfig) (*SherpaPunctuation, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	config.ApplyDefaults()
	if err := config.Validate(); err != nil {
		return nil, err
	}

	nativeConfig := buildOfflinePunctuationConfig(config)
	native := sherpa.NewOfflinePunctuation(nativeConfig)
	if native == nil {
		return nil, fmt.Errorf("failed to create Sherpa offline punctuation")
	}

	return &SherpaPunctuation{
		config:      config,
		punctuation: &nativeOfflinePunctuation{punc: native},
	}, nil
}

func buildOfflinePunctuationConfig(config *PunctuationConfig) *sherpa.OfflinePunctuationConfig {
	return &sherpa.OfflinePunctuationConfig{
		Model: sherpa.OfflinePunctuationModelConfig{
			CtTransformer: config.ModelPath,
			NumThreads:    config.NumThreads,
			Debug:         boolToInt(config.Debug),
			Provider:      config.Provider,
		},
	}
}

// Restore adds punctuation and casing to raw ASR transcript text.
func (p *SherpaPunctuation) Restore(ctx context.Context, text string) (string, error) {
	if p == nil {
		return "", fmt.Errorf("punctuation restorer cannot be nil")
	}
	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("text cannot be empty")
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.punctuation == nil {
		return "", fmt.Errorf("sherpa offline punctuation is closed")
	}

	restored := p.punctuation.AddPunct(text)

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	return restored, nil
}

// Close releases the native punctuation model. It is safe to call more than
// once.
func (p *SherpaPunctuation) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	if p.punctuation != nil {
		if err := p.punctuation.Close(); err != nil {
			return err
		}
		p.punctuation = nil
	}
	p.closed = true
	return nil
}

var _ types.Punctuation = (*SherpaPunctuation)(nil)

type nativeOfflinePunctuation struct {
	punc *sherpa.OfflinePunctuation
}

func (n *nativeOfflinePunctuation) AddPunct(text string) string {
	if n == nil || n.punc == nil {
		return ""
	}
	return n.punc.AddPunct(text)
}

func (n *nativeOfflinePunctuation) Close() error {
	if n == nil || n.punc == nil {
		return nil
	}
	sherpa.DeleteOfflinePunc(n.punc)
	n.punc = nil
	return nil
}
