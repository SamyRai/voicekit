package tts

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
	"go.glpx.pro/gdk/voice/types"
)

// SherpaOfflineSynthesizer implements offline TTS with Sherpa ONNX.
type SherpaOfflineSynthesizer struct {
	config      *Config
	synthesizer offlineSynthesizer
	mu          sync.Mutex
	closed      bool
}

type offlineSynthesizer interface {
	Generate(text string, speakerID int, speed float32) (*generatedAudio, error)
	NumSpeakers() int
	SampleRate() int
	Close() error
}

type generatedAudio struct {
	Samples    []float32
	SampleRate int
}

func NewSherpaOfflineSynthesizer(config *Config) (*SherpaOfflineSynthesizer, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	config.ApplyDefaults()
	if config.Backend != BackendSherpaOffline {
		return nil, fmt.Errorf("sherpa offline TTS requires backend %q, got %q", BackendSherpaOffline, config.Backend)
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	nativeConfig, err := buildOfflineTTSConfig(config)
	if err != nil {
		return nil, err
	}
	native := sherpa.NewOfflineTts(nativeConfig)
	if native == nil {
		return nil, fmt.Errorf("failed to create Sherpa offline TTS")
	}

	if config.Logger != nil {
		config.Logger.Infof("TTS synthesizer initialized with family: %s, provider: %s", config.ModelFamily, config.Provider)
	}
	return &SherpaOfflineSynthesizer{
		config:      config,
		synthesizer: &nativeOfflineSynthesizer{tts: native},
	}, nil
}

func buildOfflineTTSConfig(config *Config) (*sherpa.OfflineTtsConfig, error) {
	model := sherpa.OfflineTtsModelConfig{
		NumThreads: config.NumThreads,
		Debug:      boolToInt(config.Debug),
		Provider:   config.Provider,
	}

	switch config.ModelFamily {
	case FamilyVits:
		model.Vits = sherpa.OfflineTtsVitsModelConfig{
			Model:       config.Vits.Model,
			Lexicon:     config.Vits.Lexicon,
			Tokens:      config.Vits.Tokens,
			DataDir:     config.Vits.DataDir,
			NoiseScale:  config.Vits.NoiseScale,
			NoiseScaleW: config.Vits.NoiseScaleW,
			LengthScale: config.Vits.LengthScale,
		}
	case FamilyMatcha:
		model.Matcha = sherpa.OfflineTtsMatchaModelConfig{
			AcousticModel: config.Matcha.AcousticModel,
			Vocoder:       config.Matcha.Vocoder,
			Lexicon:       config.Matcha.Lexicon,
			Tokens:        config.Matcha.Tokens,
			DataDir:       config.Matcha.DataDir,
			NoiseScale:    config.Matcha.NoiseScale,
			LengthScale:   config.Matcha.LengthScale,
		}
	case FamilyKokoro:
		model.Kokoro = sherpa.OfflineTtsKokoroModelConfig{
			Model:       config.Kokoro.Model,
			Voices:      config.Kokoro.Voices,
			Tokens:      config.Kokoro.Tokens,
			DataDir:     config.Kokoro.DataDir,
			Lexicon:     config.Kokoro.Lexicon,
			Lang:        config.Kokoro.Lang,
			LengthScale: config.Kokoro.LengthScale,
		}
	case FamilyKitten:
		model.Kitten = sherpa.OfflineTtsKittenModelConfig{
			Model:       config.Kitten.Model,
			Voices:      config.Kitten.Voices,
			Tokens:      config.Kitten.Tokens,
			DataDir:     config.Kitten.DataDir,
			LengthScale: config.Kitten.LengthScale,
		}
	case FamilyZipvoice:
		model.Zipvoice = sherpa.OfflineTtsZipvoiceModelConfig{
			Tokens:        config.Zipvoice.Tokens,
			Encoder:       config.Zipvoice.Encoder,
			Decoder:       config.Zipvoice.Decoder,
			DataDir:       config.Zipvoice.DataDir,
			Lexicon:       config.Zipvoice.Lexicon,
			Vocoder:       config.Zipvoice.Vocoder,
			FeatScale:     config.Zipvoice.FeatScale,
			TShift:        config.Zipvoice.TShift,
			TargetRms:     config.Zipvoice.TargetRms,
			GuidanceScale: config.Zipvoice.GuidanceScale,
		}
	case FamilyPocket:
		model.Pocket = sherpa.OfflineTtsPocketModelConfig{
			LmFlow:                      config.Pocket.LmFlow,
			LmMain:                      config.Pocket.LmMain,
			Encoder:                     config.Pocket.Encoder,
			Decoder:                     config.Pocket.Decoder,
			TextConditioner:             config.Pocket.TextConditioner,
			VocabJson:                   config.Pocket.VocabJSON,
			TokenScoresJson:             config.Pocket.TokenScoresJSON,
			VoiceEmbeddingCacheCapacity: config.Pocket.VoiceEmbeddingCacheCapacity,
		}
	case FamilySupertonic:
		model.Supertonic = sherpa.OfflineTtsSupertonicModelConfig{
			DurationPredictor: config.Supertonic.DurationPredictor,
			TextEncoder:       config.Supertonic.TextEncoder,
			VectorEstimator:   config.Supertonic.VectorEstimator,
			Vocoder:           config.Supertonic.Vocoder,
			TtsJson:           config.Supertonic.TtsJSON,
			UnicodeIndexer:    config.Supertonic.UnicodeIndexer,
			VoiceStyle:        config.Supertonic.VoiceStyle,
		}
	default:
		return nil, fmt.Errorf("unsupported TTS model family %q", config.ModelFamily)
	}

	return &sherpa.OfflineTtsConfig{
		Model:           model,
		RuleFsts:        config.RuleFsts,
		RuleFars:        config.RuleFars,
		MaxNumSentences: config.MaxNumSentences,
		SilenceScale:    config.SilenceScale,
	}, nil
}

func (s *SherpaOfflineSynthesizer) Synthesize(ctx context.Context, request types.SynthesisRequest) (*types.SynthesizedSpeech, error) {
	if s == nil {
		return nil, fmt.Errorf("TTS synthesizer cannot be nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	text := strings.TrimSpace(request.Text)
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}
	speakerID := request.SpeakerID
	if speakerID < 0 {
		return nil, fmt.Errorf("speaker ID cannot be negative, got %d", speakerID)
	}
	speed := request.Speed
	if speed == 0 && s.config != nil {
		speed = s.config.Speed
	}
	if speed <= 0 {
		return nil, fmt.Errorf("speed must be positive, got %f", speed)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.synthesizer == nil {
		return nil, fmt.Errorf("sherpa offline TTS is closed")
	}
	numSpeakers := s.synthesizer.NumSpeakers()
	if numSpeakers > 0 && speakerID >= numSpeakers {
		return nil, fmt.Errorf("speaker ID %d exceeds available speaker count %d", speakerID, numSpeakers)
	}

	audio, err := s.synthesizer.Generate(text, speakerID, speed)
	if err != nil {
		return nil, err
	}
	if audio == nil || len(audio.Samples) == 0 {
		return nil, fmt.Errorf("TTS generated empty audio")
	}
	if audio.SampleRate <= 0 {
		return nil, fmt.Errorf("TTS generated invalid sample rate %d", audio.SampleRate)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return &types.SynthesizedSpeech{
		Samples:    append([]float32(nil), audio.Samples...),
		SampleRate: audio.SampleRate,
		Duration:   time.Duration(len(audio.Samples)) * time.Second / time.Duration(audio.SampleRate),
		SpeakerID:  speakerID,
	}, nil
}

func (s *SherpaOfflineSynthesizer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if s.synthesizer != nil {
		if err := s.synthesizer.Close(); err != nil {
			return err
		}
		s.synthesizer = nil
	}
	s.closed = true
	return nil
}

type nativeOfflineSynthesizer struct {
	tts *sherpa.OfflineTts
}

func (s *nativeOfflineSynthesizer) Generate(text string, speakerID int, speed float32) (*generatedAudio, error) {
	if s == nil || s.tts == nil {
		return nil, fmt.Errorf("sherpa offline TTS is closed")
	}
	audio := s.tts.Generate(text, speakerID, speed)
	if audio == nil {
		return nil, fmt.Errorf("sherpa offline TTS generation failed")
	}
	return &generatedAudio{
		Samples:    audio.Samples,
		SampleRate: audio.SampleRate,
	}, nil
}

func (s *nativeOfflineSynthesizer) NumSpeakers() int {
	if s == nil || s.tts == nil {
		return 0
	}
	return s.tts.NumSpeakers()
}

func (s *nativeOfflineSynthesizer) SampleRate() int {
	if s == nil || s.tts == nil {
		return 0
	}
	return s.tts.SampleRate()
}

func (s *nativeOfflineSynthesizer) Close() error {
	if s == nil || s.tts == nil {
		return nil
	}
	sherpa.DeleteOfflineTts(s.tts)
	s.tts = nil
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
