package voicekit

import (
	"testing"

	"go.glpx.pro/voicekit/audio"
)

func TestVoiceKitNilConfigIsAudioOnly(t *testing.T) {
	vk, err := NewVoiceKit(nil)
	if err != nil {
		t.Fatalf("NewVoiceKit(nil) failed: %v", err)
	}
	defer vk.Close()

	if vk.Audio() == nil {
		t.Fatal("expected audio converter")
	}
	if vk.Speaker() != nil {
		t.Fatal("speaker manager should be disabled without a model path")
	}
	if vk.Diarization() != nil {
		t.Fatal("diarization should be disabled by default")
	}
	if vk.ASR() != nil {
		t.Fatal("ASR should be disabled by default")
	}
	if vk.Transcriber() != nil {
		t.Fatal("transcriber should be disabled by default")
	}
	if vk.Synthesizer() != nil {
		t.Fatal("TTS synthesizer should be disabled by default")
	}
}

func TestVoiceKitASREnabledRequiresModelPaths(t *testing.T) {
	config := DefaultConfig()
	config.ASR.Enabled = true

	if _, err := NewVoiceKit(config); err == nil {
		t.Fatal("enabled ASR without Sherpa model paths should fail")
	}
}

func TestVoiceKitTTSEnabledRequiresModelPaths(t *testing.T) {
	config := DefaultConfig()
	config.TTS.Enabled = true

	if _, err := NewVoiceKit(config); err == nil {
		t.Fatal("enabled TTS without Sherpa model paths should fail")
	}
}

func TestConfigPartialDefaultsPreserveDisabledComponents(t *testing.T) {
	config := &Config{}
	if err := config.Validate(); err != nil {
		t.Fatalf("empty partial config should default to audio-only: %v", err)
	}

	if config.Audio.SampleRate != 16000 {
		t.Fatalf("expected default sample rate, got %d", config.Audio.SampleRate)
	}
	if config.ASR.Enabled {
		t.Fatal("ASR enabled boolean should not be defaulted to true")
	}
	if config.Diarization.Enabled {
		t.Fatal("diarization enabled boolean should not be defaulted to true")
	}
	if config.TTS.Enabled {
		t.Fatal("TTS enabled boolean should not be defaulted to true")
	}
}

func TestVoiceKitPrepareProcessingAudioResamplesToConfiguredRate(t *testing.T) {
	config := DefaultConfig()
	config.Audio.SampleRate = 16000
	vk := &VoiceKit{
		config:         config,
		audioResampler: audio.NewResampler(nil),
	}

	input := make([]float32, 8000)
	for i := range input {
		input[i] = 0.5
	}
	output, rate, err := vk.prepareProcessingAudio(input, 8000)
	if err != nil {
		t.Fatalf("sample-rate normalization failed: %v", err)
	}
	if rate != 16000 {
		t.Fatalf("expected 16kHz output rate, got %d", rate)
	}
	if len(output) != 16000 {
		t.Fatalf("expected resampled output length 16000, got %d", len(output))
	}
}
