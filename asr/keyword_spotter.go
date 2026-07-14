package asr

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/SamyRai/voicekit/types"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// KeywordSpotterConfig owns Sherpa streaming keyword-spotting model
// configuration. Keyword spotting models are streaming, like online ASR:
// audio is fed incrementally through a native stream kept per session.
//
// Keywords are supplied either as a file (KeywordsFile, one keyword phrase
// per line in the format the Sherpa model's tokenizer expects) or inline
// (Keywords). When both are set, KeywordsFile takes precedence.
type KeywordSpotterConfig struct {
	// Model paths. Either a transducer triple (EncoderPath/DecoderPath/
	// JoinerPath) or a single-file CTC model (ModelPath + ModelType),
	// mirroring OnlineConfig's shape.
	TokensPath  string `json:"tokens_path"`
	EncoderPath string `json:"encoder_path"`
	DecoderPath string `json:"decoder_path"`
	JoinerPath  string `json:"joiner_path"`
	ModelPath   string `json:"model_path"`
	ModelType   string `json:"model_type"`

	KeywordsFile      string   `json:"keywords_file,omitempty"`
	Keywords          []string `json:"keywords,omitempty"`
	KeywordsScore     float32  `json:"keywords_score"`
	KeywordsThreshold float32  `json:"keywords_threshold"`

	SampleRate     int `json:"sample_rate"`
	FeatureDim     int `json:"feature_dim"`
	MaxActivePaths int `json:"max_active_paths"`

	Provider   string `json:"provider"`
	NumThreads int    `json:"num_threads"`
	Debug      bool   `json:"debug"`
}

// DefaultKeywordSpotterConfig returns runtime defaults. Model and keyword
// paths are left empty since they must be supplied by the caller.
func DefaultKeywordSpotterConfig() KeywordSpotterConfig {
	return KeywordSpotterConfig{
		SampleRate:        16000,
		FeatureDim:        80,
		Provider:          "cpu",
		NumThreads:        1,
		MaxActivePaths:    4,
		KeywordsScore:     1.5,
		KeywordsThreshold: 0.25,
	}
}

// ApplyDefaults merges zero-value runtime settings with conservative
// defaults.
func (c *KeywordSpotterConfig) ApplyDefaults() {
	defaults := DefaultKeywordSpotterConfig()
	if c.SampleRate <= 0 {
		c.SampleRate = defaults.SampleRate
	}
	if c.FeatureDim <= 0 {
		c.FeatureDim = defaults.FeatureDim
	}
	if c.Provider == "" {
		c.Provider = defaults.Provider
	}
	if c.NumThreads <= 0 {
		c.NumThreads = defaults.NumThreads
	}
	if c.MaxActivePaths <= 0 {
		c.MaxActivePaths = defaults.MaxActivePaths
	}
	if c.KeywordsScore == 0 {
		c.KeywordsScore = defaults.KeywordsScore
	}
	if c.KeywordsThreshold == 0 {
		c.KeywordsThreshold = defaults.KeywordsThreshold
	}
}

// Validate validates keyword spotter configuration, including model and
// keyword file checks.
func (c *KeywordSpotterConfig) Validate() error {
	c.ApplyDefaults()

	var errs []error
	if c.TokensPath == "" {
		errs = append(errs, fmt.Errorf("tokens path is required"))
	} else if err := requireFile(c.TokensPath); err != nil {
		errs = append(errs, fmt.Errorf("tokens path: %w", err))
	}

	hasTransducer := c.EncoderPath != "" || c.DecoderPath != "" || c.JoinerPath != ""
	switch {
	case hasTransducer:
		errs = append(errs, requirePathSet("keyword spotter transducer", map[string]string{
			"encoder": c.EncoderPath,
			"decoder": c.DecoderPath,
			"joiner":  c.JoinerPath,
		})...)
	case c.ModelPath != "":
		if err := requireFile(c.ModelPath); err != nil {
			errs = append(errs, fmt.Errorf("model path: %w", err))
		}
	default:
		errs = append(errs, fmt.Errorf("keyword spotter requires transducer encoder/decoder/joiner paths or a single model path"))
	}

	if c.KeywordsFile == "" && len(c.Keywords) == 0 {
		errs = append(errs, fmt.Errorf("keyword spotter requires KeywordsFile or at least one inline keyword"))
	} else if c.KeywordsFile != "" {
		if err := requireFile(c.KeywordsFile); err != nil {
			errs = append(errs, fmt.Errorf("keywords file: %w", err))
		}
	}

	if c.SampleRate <= 0 {
		errs = append(errs, fmt.Errorf("sample rate must be positive, got %d", c.SampleRate))
	}
	if c.FeatureDim <= 0 {
		errs = append(errs, fmt.Errorf("feature dim must be positive, got %d", c.FeatureDim))
	}
	if c.NumThreads <= 0 {
		errs = append(errs, fmt.Errorf("num threads must be positive, got %d", c.NumThreads))
	}

	if len(errs) > 0 {
		return fmt.Errorf("keyword spotter config validation failed: %v", errs)
	}
	return nil
}

// SherpaKeywordSpotter implements types.KeywordSpotter with Sherpa ONNX's
// streaming KeywordSpotter, keeping a native decode stream per sessionID so
// a single spotter can serve concurrent streams (mirroring SherpaOnlineModel).
type SherpaKeywordSpotter struct {
	sampleRate int
	spotter    keywordRecognizer
	sessions   map[string]*keywordSession

	mu     sync.Mutex
	closed bool
}

// keywordRecognizer is the mockable seam over the native Sherpa binding, so
// tests can inject a fake without loading real model weights.
type keywordRecognizer interface {
	NewStream() (keywordStream, error)
	Decode(stream keywordStream) error
	IsReady(stream keywordStream) bool
	Reset(stream keywordStream) error
	Result(stream keywordStream) (*sherpa.KeywordSpotterResult, error)
	Close() error
}

type keywordStream interface {
	AcceptWaveform(sampleRate int, samples []float32) error
	Close() error
}

type keywordSession struct {
	stream keywordStream
	closed bool
}

func (s *keywordSession) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	if s.stream != nil {
		return s.stream.Close()
	}
	return nil
}

// NewSherpaKeywordSpotter validates config and constructs a keyword spotter
// backed by a native Sherpa KeywordSpotter instance.
func NewSherpaKeywordSpotter(config *KeywordSpotterConfig) (*SherpaKeywordSpotter, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	config.ApplyDefaults()
	if err := config.Validate(); err != nil {
		return nil, err
	}

	nativeConfig, err := buildKeywordSpotterConfig(config)
	if err != nil {
		return nil, err
	}
	native := sherpa.NewKeywordSpotter(nativeConfig)
	if native == nil {
		return nil, fmt.Errorf("failed to create Sherpa keyword spotter")
	}

	return &SherpaKeywordSpotter{
		sampleRate: config.SampleRate,
		spotter:    &sherpaKeywordRecognizer{spotter: native},
		sessions:   make(map[string]*keywordSession),
	}, nil
}

func buildKeywordSpotterConfig(config *KeywordSpotterConfig) (*sherpa.KeywordSpotterConfig, error) {
	modelConfig := sherpa.OnlineModelConfig{
		Tokens:     config.TokensPath,
		NumThreads: config.NumThreads,
		Provider:   config.Provider,
		Debug:      boolToInt(config.Debug),
		ModelType:  config.ModelType,
	}

	if config.EncoderPath != "" || config.DecoderPath != "" || config.JoinerPath != "" {
		if config.EncoderPath == "" || config.DecoderPath == "" || config.JoinerPath == "" {
			return nil, fmt.Errorf("transducer encoder, decoder, and joiner paths must be provided together")
		}
		modelConfig.Transducer = sherpa.OnlineTransducerModelConfig{
			Encoder: config.EncoderPath,
			Decoder: config.DecoderPath,
			Joiner:  config.JoinerPath,
		}
	} else {
		switch strings.ToLower(config.ModelType) {
		case "", "zipformer2_ctc", "zipformer_ctc", "ctc":
			modelConfig.Zipformer2Ctc.Model = config.ModelPath
		case "nemo_ctc", "nemo":
			modelConfig.NemoCtc.Model = config.ModelPath
		default:
			return nil, fmt.Errorf("unsupported single-file keyword spotting model type %q", config.ModelType)
		}
	}

	nativeConfig := &sherpa.KeywordSpotterConfig{
		FeatConfig: sherpa.FeatureConfig{
			SampleRate: config.SampleRate,
			FeatureDim: config.FeatureDim,
		},
		ModelConfig:       modelConfig,
		MaxActivePaths:    config.MaxActivePaths,
		KeywordsFile:      config.KeywordsFile,
		KeywordsScore:     config.KeywordsScore,
		KeywordsThreshold: config.KeywordsThreshold,
	}

	if config.KeywordsFile == "" && len(config.Keywords) > 0 {
		buf := strings.Join(config.Keywords, "\n")
		nativeConfig.KeywordsBuf = buf
		nativeConfig.KeywordsBufSize = len(buf)
	}

	return nativeConfig, nil
}

// Spot feeds an audio chunk for a session through its native keyword stream
// and returns a keyword detected on this step, or nil when none fired.
func (m *SherpaKeywordSpotter) Spot(ctx context.Context, sessionID string, audio []float32) (*types.KeywordMatch, error) {
	if m == nil {
		return nil, fmt.Errorf("keyword spotter cannot be nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("audio cannot be empty")
	}

	if err := contextError(ctx); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.spotter == nil {
		return nil, fmt.Errorf("sherpa keyword spotter is closed")
	}

	session, err := m.sessionForIDLocked(sessionID)
	if err != nil {
		return nil, err
	}

	if err := session.stream.AcceptWaveform(m.sampleRate, audio); err != nil {
		return nil, err
	}

	for m.spotter.IsReady(session.stream) {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		if err := m.spotter.Decode(session.stream); err != nil {
			return nil, err
		}
	}

	result, err := m.spotter.Result(session.stream)
	if err != nil {
		return nil, err
	}
	if result == nil || result.Keyword == "" {
		return nil, nil
	}

	// The Sherpa binding requires Reset immediately after a keyword fires so
	// the stream is ready to detect the next occurrence.
	if err := m.spotter.Reset(session.stream); err != nil {
		return nil, err
	}

	return &types.KeywordMatch{
		Keyword: result.Keyword,
		// Score is always 0: Sherpa's KeywordSpotterResult (v1.13.4) exposes
		// only the matched keyword text, not a calibrated confidence value.
		Score: 0,
	}, nil
}

func (m *SherpaKeywordSpotter) sessionForIDLocked(sessionID string) (*keywordSession, error) {
	if session, ok := m.sessions[sessionID]; ok && !session.closed {
		return session, nil
	}

	stream, err := m.spotter.NewStream()
	if err != nil {
		return nil, err
	}
	session := &keywordSession{stream: stream}
	if m.sessions == nil {
		m.sessions = make(map[string]*keywordSession)
	}
	m.sessions[sessionID] = session
	return session, nil
}

// EndSession closes and removes the native decode stream for a session so a
// caller that uses a fresh sessionID per utterance does not accumulate native
// streams. Ending an unknown session is a no-op; a later Spot with the same
// sessionID starts a fresh stream.
func (m *SherpaKeywordSpotter) EndSession(sessionID string) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[sessionID]
	if !ok {
		return nil
	}
	delete(m.sessions, sessionID)
	return session.Close()
}

// Close releases all per-session native streams and the native spotter. It
// is safe to call more than once.
func (m *SherpaKeywordSpotter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	for id, session := range m.sessions {
		if err := session.Close(); err != nil {
			return err
		}
		delete(m.sessions, id)
	}
	if m.spotter != nil {
		if err := m.spotter.Close(); err != nil {
			return err
		}
		m.spotter = nil
	}
	m.closed = true
	return nil
}

var _ types.KeywordSpotter = (*SherpaKeywordSpotter)(nil)

type sherpaKeywordRecognizer struct {
	spotter *sherpa.KeywordSpotter
}

func (r *sherpaKeywordRecognizer) NewStream() (keywordStream, error) {
	stream := sherpa.NewKeywordStream(r.spotter)
	if stream == nil {
		return nil, fmt.Errorf("failed to create sherpa keyword stream")
	}
	return &sherpaKeywordStream{stream: stream}, nil
}

func (r *sherpaKeywordRecognizer) Decode(stream keywordStream) error {
	sherpaStream, err := nativeKeywordStream(stream)
	if err != nil {
		return err
	}
	r.spotter.Decode(sherpaStream)
	return nil
}

func (r *sherpaKeywordRecognizer) IsReady(stream keywordStream) bool {
	sherpaStream, err := nativeKeywordStream(stream)
	if err != nil {
		return false
	}
	return r.spotter.IsReady(sherpaStream)
}

func (r *sherpaKeywordRecognizer) Reset(stream keywordStream) error {
	sherpaStream, err := nativeKeywordStream(stream)
	if err != nil {
		return err
	}
	r.spotter.Reset(sherpaStream)
	return nil
}

func (r *sherpaKeywordRecognizer) Result(stream keywordStream) (*sherpa.KeywordSpotterResult, error) {
	sherpaStream, err := nativeKeywordStream(stream)
	if err != nil {
		return nil, err
	}
	return r.spotter.GetResult(sherpaStream), nil
}

func (r *sherpaKeywordRecognizer) Close() error {
	if r.spotter != nil {
		sherpa.DeleteKeywordSpotter(r.spotter)
		r.spotter = nil
	}
	return nil
}

// sherpaKeywordStream wraps *sherpa.OnlineStream: the binding reuses the
// online ASR stream type for keyword-spotting streams (see
// sherpa.NewKeywordStream).
type sherpaKeywordStream struct {
	stream *sherpa.OnlineStream
}

func (s *sherpaKeywordStream) AcceptWaveform(sampleRate int, samples []float32) error {
	if s.stream == nil {
		return fmt.Errorf("sherpa keyword stream is closed")
	}
	s.stream.AcceptWaveform(sampleRate, samples)
	return nil
}

func (s *sherpaKeywordStream) Close() error {
	if s.stream != nil {
		sherpa.DeleteOnlineStream(s.stream)
		s.stream = nil
	}
	return nil
}

func nativeKeywordStream(stream keywordStream) (*sherpa.OnlineStream, error) {
	sherpaStream, ok := stream.(*sherpaKeywordStream)
	if !ok || sherpaStream.stream == nil {
		return nil, fmt.Errorf("unexpected keyword stream implementation")
	}
	return sherpaStream.stream, nil
}
