package asr

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/SamyRai/voicekit/types"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// SherpaOfflineModel implements batch ASR with Sherpa ONNX OfflineRecognizer.
type SherpaOfflineModel struct {
	name         string
	language     string
	quantization string
	recognizer   offlineRecognizer

	mu     sync.Mutex
	closed bool
}

type offlineRecognizer interface {
	NewStream() (offlineStream, error)
	Decode(stream offlineStream) error
	Close() error
}

type offlineStream interface {
	AcceptWaveform(sampleRate int, samples []float32) error
	Result() *sherpa.OfflineRecognizerResult
	Close() error
}

func NewSherpaOfflineModel(config *Config) (*SherpaOfflineModel, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	config.ApplyDefaults()
	if config.Backend != BackendSherpaOffline {
		return nil, fmt.Errorf("Sherpa offline recognizer requires backend %q, got %q", BackendSherpaOffline, config.Backend)
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	recognizerConfig, err := buildOfflineRecognizerConfig(config)
	if err != nil {
		return nil, err
	}
	recognizer := sherpa.NewOfflineRecognizer(recognizerConfig)
	if recognizer == nil {
		return nil, fmt.Errorf("failed to create Sherpa offline recognizer")
	}

	return &SherpaOfflineModel{
		name:         config.DefaultModel,
		language:     config.Offline.Language,
		quantization: config.Quantization,
		recognizer:   &nativeOfflineRecognizer{recognizer: recognizer},
	}, nil
}

func buildOfflineRecognizerConfig(config *Config) (*sherpa.OfflineRecognizerConfig, error) {
	offline := config.Offline
	modelType := offline.ModelType
	if modelType == "" {
		modelType = offline.ModelFamily
	}

	modelConfig := sherpa.OfflineModelConfig{
		Tokens:     offline.TokensPath,
		NumThreads: config.NumThreads,
		Debug:      boolToInt(config.Debug),
		Provider:   config.Provider,
		ModelType:  modelType,
	}

	switch offline.ModelFamily {
	case OfflineFamilyTransducer:
		modelConfig.Transducer = sherpa.OfflineTransducerModelConfig{
			Encoder: offline.EncoderPath,
			Decoder: offline.DecoderPath,
			Joiner:  offline.JoinerPath,
		}
	case OfflineFamilyParaformer:
		modelConfig.Paraformer = sherpa.OfflineParaformerModelConfig{Model: offline.ModelPath}
	case OfflineFamilyZipformerCTC:
		modelConfig.ZipformerCtc = sherpa.OfflineZipformerCtcModelConfig{Model: offline.ModelPath}
	case OfflineFamilyNemoCTC:
		modelConfig.NemoCTC = sherpa.OfflineNemoEncDecCtcModelConfig{Model: offline.ModelPath}
	case OfflineFamilySenseVoice:
		modelConfig.SenseVoice = sherpa.OfflineSenseVoiceModelConfig{
			Model:                       offline.ModelPath,
			Language:                    offline.Language,
			UseInverseTextNormalization: boolToInt(offline.UseInverseTextNormalization),
		}
	case OfflineFamilyWhisper:
		modelConfig.Whisper = sherpa.OfflineWhisperModelConfig{
			Encoder:                 offline.EncoderPath,
			Decoder:                 offline.DecoderPath,
			Language:                offline.Language,
			Task:                    offline.Task,
			TailPaddings:            offline.TailPaddings,
			EnableTokenTimestamps:   boolToInt(offline.EnableTokenTimestamps),
			EnableSegmentTimestamps: boolToInt(offline.EnableSegmentTimestamps),
		}
	default:
		return nil, fmt.Errorf("unsupported offline model family %q", offline.ModelFamily)
	}

	return &sherpa.OfflineRecognizerConfig{
		FeatConfig: sherpa.FeatureConfig{
			SampleRate: config.SampleRate,
			FeatureDim: config.FeatureDim,
		},
		ModelConfig:    modelConfig,
		DecodingMethod: config.DecodingMethod,
		MaxActivePaths: config.MaxActivePaths,
	}, nil
}

func (m *SherpaOfflineModel) Transcribe(ctx context.Context, audio []float32, sampleRate int) (*types.Transcription, error) {
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

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.recognizer == nil {
		return nil, fmt.Errorf("Sherpa offline recognizer is closed")
	}

	stream, err := m.recognizer.NewStream()
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	if err := stream.AcceptWaveform(sampleRate, audio); err != nil {
		return nil, err
	}
	if err := m.recognizer.Decode(stream); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result := stream.Result()
	return transcriptionFromOfflineResult(result, m.language, len(audio), sampleRate), nil
}

func (m *SherpaOfflineModel) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	if m.recognizer != nil {
		if err := m.recognizer.Close(); err != nil {
			return err
		}
		m.recognizer = nil
	}
	m.closed = true
	return nil
}

func transcriptionFromOfflineResult(result *sherpa.OfflineRecognizerResult, fallbackLanguage string, sampleCount, sampleRate int) *types.Transcription {
	language := fallbackLanguage
	text := ""
	emotion := ""
	event := ""
	if result != nil {
		text = result.Text
		emotion = result.Emotion
		event = result.Event
		if result.Lang != "" {
			language = result.Lang
		}
	}

	return &types.Transcription{
		Text:       text,
		IsPartial:  false,
		Confidence: 0,
		Language:   language,
		Emotion:    emotion,
		Event:      event,
		Timestamp:  time.Now(),
		StartTime:  0,
		EndTime:    time.Duration(sampleCount) * time.Second / time.Duration(sampleRate),
		Words:      wordsFromOfflineResult(result),
	}
}

func wordsFromOfflineResult(result *sherpa.OfflineRecognizerResult) []types.Word {
	if result == nil || len(result.Tokens) == 0 || len(result.Timestamps) == 0 {
		return nil
	}

	n := len(result.Tokens)
	if len(result.Timestamps) < n {
		n = len(result.Timestamps)
	}

	words := make([]types.Word, 0, n)
	for i := 0; i < n; i++ {
		start := time.Duration(result.Timestamps[i] * float32(time.Second))
		end := start
		if i < len(result.Durations) && result.Durations[i] > 0 {
			end = start + time.Duration(result.Durations[i]*float32(time.Second))
		} else if i+1 < len(result.Timestamps) {
			end = time.Duration(result.Timestamps[i+1] * float32(time.Second))
		}
		words = append(words, types.Word{
			Text:       strings.TrimSpace(result.Tokens[i]),
			StartTime:  start,
			EndTime:    end,
			Confidence: 0,
		})
	}
	return words
}

type nativeOfflineRecognizer struct {
	recognizer *sherpa.OfflineRecognizer
}

func (r *nativeOfflineRecognizer) NewStream() (offlineStream, error) {
	if r == nil || r.recognizer == nil {
		return nil, fmt.Errorf("Sherpa offline recognizer is closed")
	}
	stream := sherpa.NewOfflineStream(r.recognizer)
	if stream == nil {
		return nil, fmt.Errorf("failed to create Sherpa offline stream")
	}
	return &nativeOfflineStream{stream: stream}, nil
}

func (r *nativeOfflineRecognizer) Decode(stream offlineStream) error {
	if r == nil || r.recognizer == nil {
		return fmt.Errorf("Sherpa offline recognizer is closed")
	}
	native, ok := stream.(*nativeOfflineStream)
	if !ok || native.stream == nil {
		return fmt.Errorf("unsupported offline stream implementation")
	}
	r.recognizer.Decode(native.stream)
	return nil
}

func (r *nativeOfflineRecognizer) Close() error {
	if r == nil || r.recognizer == nil {
		return nil
	}
	sherpa.DeleteOfflineRecognizer(r.recognizer)
	r.recognizer = nil
	return nil
}

type nativeOfflineStream struct {
	stream *sherpa.OfflineStream
}

func (s *nativeOfflineStream) AcceptWaveform(sampleRate int, samples []float32) error {
	if s == nil || s.stream == nil {
		return fmt.Errorf("Sherpa offline stream is closed")
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

func (s *nativeOfflineStream) Result() *sherpa.OfflineRecognizerResult {
	if s == nil || s.stream == nil {
		return nil
	}
	return s.stream.GetResult()
}

func (s *nativeOfflineStream) Close() error {
	if s == nil || s.stream == nil {
		return nil
	}
	sherpa.DeleteOfflineStream(s.stream)
	s.stream = nil
	return nil
}
