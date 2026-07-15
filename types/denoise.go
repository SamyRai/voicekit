package types

import "context"

// SpeechDenoiser suppresses noise in a complete audio buffer, returning denoised
// PCM samples. It is an optional preprocessing stage before VAD/ASR for offline
// or whole-utterance audio.
type SpeechDenoiser interface {
	Denoise(ctx context.Context, samples []float32, sampleRate int) ([]float32, error)
	Close() error
}

// StreamingDenoiser suppresses noise incrementally for realtime pipelines. The
// stream's sample rate is fixed by the model at construction. Accept feeds a
// chunk and returns whatever denoised samples are ready; Flush drains any
// remaining buffered audio at end of stream; Reset clears state to begin a new
// stream on the same instance.
type StreamingDenoiser interface {
	Accept(ctx context.Context, chunk []float32) ([]float32, error)
	Flush() ([]float32, error)
	Reset()
	Close() error
}
