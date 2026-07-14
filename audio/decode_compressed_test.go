package audio

import (
	"os"
	"path/filepath"
	"testing"
)

// Fixtures in testdata/ are 0.2s 440 Hz tones (16 kHz) produced with ffmpeg:
//   tone.flac  16-bit mono
//   tone.mp3   MPEG-2 layer III mono (go-mp3 decodes to 16-bit stereo)
//   tone.ogg   Vorbis stereo (the native encoder requires 2 channels)

func decodeFixture(t *testing.T, name string, cfg *AudioConfig) []float32 {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	conv, err := NewConverter(DefaultConverterConfig())
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	samples, err := conv.ConvertToFloat32(data, cfg)
	if err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return samples
}

func assertPlausibleAudio(t *testing.T, samples []float32, minLen int) {
	t.Helper()
	if len(samples) < minLen {
		t.Fatalf("decoded %d samples, want at least %d", len(samples), minLen)
	}
	nonZero := false
	for _, s := range samples {
		// A normalization bug would leave raw int16 magnitudes (~±32768) here;
		// the generous bound still catches that while tolerating lossy overshoot.
		if s < -1.5 || s > 1.5 {
			t.Fatalf("sample %f outside plausible normalized range", s)
		}
		if s != 0 {
			nonZero = true
		}
	}
	if !nonZero {
		t.Fatal("decoded audio is entirely silent")
	}
}

func TestDecodeFLAC(t *testing.T) {
	samples := decodeFixture(t, "tone.flac", &AudioConfig{Format: FormatFLAC, SampleRate: 16000, Channels: 1})
	assertPlausibleAudio(t, samples, 3000) // ~3200 mono samples
}

func TestDecodeMP3(t *testing.T) {
	// go-mp3 always emits 16-bit stereo, so expect ~2x interleaved samples.
	samples := decodeFixture(t, "tone.mp3", &AudioConfig{Format: FormatMP3, SampleRate: 16000, Channels: 2})
	assertPlausibleAudio(t, samples, 3000)
}

func TestDecodeOGG(t *testing.T) {
	samples := decodeFixture(t, "tone.ogg", &AudioConfig{Format: FormatOGG, SampleRate: 16000, Channels: 2})
	assertPlausibleAudio(t, samples, 3000)
}

func TestDecodeCompressedRejectsFormatMismatch(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "tone.flac"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	conv, err := NewConverter(DefaultConverterConfig())
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	// The fixture is 16 kHz; asking for 8 kHz must be rejected, not silently wrong.
	if _, err := conv.ConvertToFloat32(data, &AudioConfig{Format: FormatFLAC, SampleRate: 8000, Channels: 1}); err == nil {
		t.Error("expected a sample-rate mismatch error, got nil")
	}
}

func TestDecodeM4AReturnsClearUnsupportedError(t *testing.T) {
	conv, err := NewConverter(DefaultConverterConfig())
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	_, err = conv.ConvertToFloat32([]byte("not-audio"), &AudioConfig{Format: FormatM4A, SampleRate: 16000, Channels: 1})
	if err == nil {
		t.Fatal("expected an unsupported-format error for M4A")
	}
}
