package voicekit

import (
	"context"
	"testing"
)

// fakeDenoiser is a hermetic types.SpeechDenoiser that halves samples so a test
// can prove the denoise stage actually ran, and counts Close calls.
type fakeDenoiser struct {
	closeCount int
}

func (f *fakeDenoiser) Denoise(_ context.Context, samples []float32, _ int) ([]float32, error) {
	out := make([]float32, len(samples))
	for i, s := range samples {
		out[i] = s * 0.5
	}
	return out, nil
}

func (f *fakeDenoiser) Close() error {
	f.closeCount++
	return nil
}

func TestPrepareProcessingAudioAppliesDenoiser(t *testing.T) {
	fake := &fakeDenoiser{}
	vk := &VoiceKit{
		config:   &Config{Audio: AudioConfig{SampleRate: 16000}},
		denoiser: fake,
	}

	in := []float32{1, 2, 4}
	out, rate, err := vk.prepareProcessingAudio(in, 16000)
	if err != nil {
		t.Fatalf("prepareProcessingAudio failed: %v", err)
	}
	if rate != 16000 {
		t.Fatalf("expected rate 16000, got %d", rate)
	}
	for i := range in {
		if out[i] != in[i]*0.5 {
			t.Fatalf("expected denoised (halved) samples, got %v", out)
		}
	}
}

func TestPrepareProcessingAudioWithoutDenoiserIsPassthrough(t *testing.T) {
	vk := &VoiceKit{config: &Config{Audio: AudioConfig{SampleRate: 16000}}}

	in := []float32{1, 2, 3}
	out, rate, err := vk.prepareProcessingAudio(in, 16000)
	if err != nil {
		t.Fatalf("prepareProcessingAudio failed: %v", err)
	}
	if rate != 16000 || len(out) != len(in) {
		t.Fatalf("unexpected output rate=%d len=%d", rate, len(out))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Fatalf("expected passthrough samples, got %v", out)
		}
	}
}

func TestCloseClosesDenoiser(t *testing.T) {
	fake := &fakeDenoiser{}
	vk := &VoiceKit{denoiser: fake}

	if err := vk.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if fake.closeCount != 1 {
		t.Fatalf("expected denoiser closed exactly once, got %d", fake.closeCount)
	}
}
