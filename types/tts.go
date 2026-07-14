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

// StreamingSynthesizer synthesizes speech incrementally, delivering audio in
// chunks as they are produced so callers can begin playback before synthesis
// completes. It also returns the fully assembled speech. Synthesis stops early
// if the context is cancelled or the sink returns false (barge-in).
type StreamingSynthesizer interface {
	SynthesizeStream(ctx context.Context, request SynthesisRequest, sink AudioChunkFunc) (*SynthesizedSpeech, error)
	Close() error
}

// AudioChunkFunc receives generated audio chunks in order. Returning false
// stops synthesis early, allowing interruption/barge-in.
type AudioChunkFunc func(chunk AudioChunk) bool

// AudioChunk is a contiguous span of freshly generated PCM audio.
type AudioChunk struct {
	Samples    []float32 `json:"-"`
	SampleRate int       `json:"sample_rate"`
	// Progress is the fraction of the utterance synthesized so far (0..1) when
	// the backend reports it, and 0 otherwise.
	Progress float32 `json:"progress,omitempty"`
}
