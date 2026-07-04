package types

import (
	"context"
	"time"
)

// SpeechSynthesizer defines text-to-speech synthesis over complete text inputs.
type SpeechSynthesizer interface {
	Synthesize(ctx context.Context, request SynthesisRequest) (*SynthesizedSpeech, error)
	Close() error
}

// SynthesisRequest represents one text-to-speech generation request.
type SynthesisRequest struct {
	Text      string  `json:"text"`
	SpeakerID int     `json:"speaker_id,omitempty"`
	Speed     float32 `json:"speed,omitempty"`
}

// SynthesizedSpeech contains generated normalized PCM samples.
type SynthesizedSpeech struct {
	Samples    []float32     `json:"-"`
	SampleRate int           `json:"sample_rate"`
	Duration   time.Duration `json:"duration"`
	SpeakerID  int           `json:"speaker_id"`
}
