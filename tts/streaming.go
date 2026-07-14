package tts

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/SamyRai/voicekit/types"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// SherpaStreamingSynthesizer implements incremental TTS with Sherpa ONNX,
// delivering audio chunks to a sink as they are produced.
type SherpaStreamingSynthesizer struct {
	config      *Config
	synthesizer streamingSynthesizer
	mu          sync.Mutex
	closed      bool
}

var _ types.StreamingSynthesizer = (*SherpaStreamingSynthesizer)(nil)

// streamingSynthesizer is the mockable native seam for streaming generation.
type streamingSynthesizer interface {
	GenerateWithCallback(text string, speakerID int, speed float32, cb chunkCallback) (*generatedAudio, error)
	NumSpeakers() int
	SampleRate() int
	Close() error
}

// chunkCallback receives one generated audio chunk plus the fraction of the
// utterance synthesized so far. Returning false stops synthesis early.
type chunkCallback func(samples []float32, progress float32) bool

// NewSherpaStreamingSynthesizer constructs a streaming Sherpa offline TTS
// synthesizer. It reuses the same tts.Config validation and native config
// construction as SherpaOfflineSynthesizer.
func NewSherpaStreamingSynthesizer(config *Config) (*SherpaStreamingSynthesizer, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	config.ApplyDefaults()
	if config.Backend != BackendSherpaOffline {
		return nil, fmt.Errorf("sherpa streaming TTS requires backend %q, got %q", BackendSherpaOffline, config.Backend)
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
		config.Logger.Infof("Streaming TTS synthesizer initialized with family: %s, provider: %s", config.ModelFamily, config.Provider)
	}
	return &SherpaStreamingSynthesizer{
		config:      config,
		synthesizer: &nativeStreamingSynthesizer{tts: native},
	}, nil
}

// SynthesizeStream generates speech for request, delivering each produced
// chunk to sink in order. Synthesis stops early if ctx is canceled or sink
// returns false. It always returns the speech assembled up to the stopping
// point (nil on error).
func (s *SherpaStreamingSynthesizer) SynthesizeStream(ctx context.Context, request types.SynthesisRequest, sink types.AudioChunkFunc) (*types.SynthesizedSpeech, error) {
	if s == nil {
		return nil, fmt.Errorf("TTS synthesizer cannot be nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if sink == nil {
		return nil, fmt.Errorf("sink cannot be nil")
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
		return nil, fmt.Errorf("sherpa streaming TTS is closed")
	}
	numSpeakers := s.synthesizer.NumSpeakers()
	if numSpeakers > 0 && speakerID >= numSpeakers {
		return nil, fmt.Errorf("speaker ID %d exceeds available speaker count %d", speakerID, numSpeakers)
	}
	sampleRate := s.synthesizer.SampleRate()

	audio, err := s.synthesizer.GenerateWithCallback(text, speakerID, speed, func(samples []float32, progress float32) bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		if len(samples) == 0 {
			return true
		}
		return sink(types.AudioChunk{
			Samples:    append([]float32(nil), samples...),
			SampleRate: sampleRate,
			Progress:   progress,
		})
	})
	if err != nil {
		return nil, err
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if audio == nil || len(audio.Samples) == 0 {
		return nil, fmt.Errorf("TTS generated empty audio")
	}
	resultRate := audio.SampleRate
	if resultRate <= 0 {
		resultRate = sampleRate
	}
	if resultRate <= 0 {
		return nil, fmt.Errorf("TTS generated invalid sample rate %d", resultRate)
	}

	return &types.SynthesizedSpeech{
		Samples:    append([]float32(nil), audio.Samples...),
		SampleRate: resultRate,
		Duration:   time.Duration(len(audio.Samples)) * time.Second / time.Duration(resultRate),
		SpeakerID:  speakerID,
	}, nil
}

// Close releases the underlying native synthesizer. Safe to call more than
// once.
func (s *SherpaStreamingSynthesizer) Close() error {
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

// nativeStreamingSynthesizer adapts sherpa.OfflineTts's per-chunk progress
// callback API (GenerateWithProgressCallback) to the streamingSynthesizer
// seam.
type nativeStreamingSynthesizer struct {
	tts *sherpa.OfflineTts
}

func (s *nativeStreamingSynthesizer) GenerateWithCallback(text string, speakerID int, speed float32, cb chunkCallback) (*generatedAudio, error) {
	if s == nil || s.tts == nil {
		return nil, fmt.Errorf("sherpa streaming TTS is closed")
	}
	audio := s.tts.GenerateWithProgressCallback(text, speakerID, speed, func(samples []float32, progress float32) bool {
		return cb(samples, progress)
	})
	if audio == nil {
		return nil, fmt.Errorf("sherpa streaming TTS generation failed")
	}
	return &generatedAudio{
		Samples:    audio.Samples,
		SampleRate: audio.SampleRate,
	}, nil
}

func (s *nativeStreamingSynthesizer) NumSpeakers() int {
	if s == nil || s.tts == nil {
		return 0
	}
	return s.tts.NumSpeakers()
}

func (s *nativeStreamingSynthesizer) SampleRate() int {
	if s == nil || s.tts == nil {
		return 0
	}
	return s.tts.SampleRate()
}

func (s *nativeStreamingSynthesizer) Close() error {
	if s == nil || s.tts == nil {
		return nil
	}
	sherpa.DeleteOfflineTts(s.tts)
	s.tts = nil
	return nil
}
