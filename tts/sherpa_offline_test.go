package tts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.glpx.pro/gdk/voice/types"
)

func TestConfigValidateDisabledSkipsModelPaths(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false
	config.Kokoro.Model = ""

	if err := config.Validate(); err != nil {
		t.Fatalf("disabled TTS should skip model path validation: %v", err)
	}
}

func TestConfigValidateKokoroRequiresModelPaths(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.ModelFamily = FamilyKokoro

	if err := config.Validate(); err == nil {
		t.Fatal("enabled Kokoro TTS without model paths should fail")
	}
}

// TestConfigValidateKokoroMultilingualRequiresLang proves that a Kokoro
// config with a Lexicon set (the multilingual-pack signal; see the comment
// on validateKokoro) fails validation when Lang is empty, and passes once
// Lang is provided.
func TestConfigValidateKokoroMultilingualRequiresLang(t *testing.T) {
	config := validKokoroConfig(t)
	config.Kokoro.Lexicon = tempFile(t, "lexicon.txt")
	config.Kokoro.Lang = ""

	err := config.Validate()
	if err == nil {
		t.Fatal("multilingual Kokoro config (lexicon set) without lang should fail validation")
	}
	if !strings.Contains(err.Error(), "kokoro lang is required") {
		t.Errorf("expected kokoro lang requirement error; got: %s", err.Error())
	}
}

func TestConfigValidateKokoroMultilingualWithLangPasses(t *testing.T) {
	config := validKokoroConfig(t)
	config.Kokoro.Lexicon = tempFile(t, "lexicon.txt")
	config.Kokoro.Lang = "es"

	if err := config.Validate(); err != nil {
		t.Fatalf("multilingual Kokoro config with lang set should validate: %v", err)
	}
}

func TestBuildOfflineTTSConfigMapsKokoro(t *testing.T) {
	config := validKokoroConfig(t)
	mapped, err := buildOfflineTTSConfig(&config)
	if err != nil {
		t.Fatalf("failed to build TTS config: %v", err)
	}
	if mapped.Model.Kokoro.Model != config.Kokoro.Model {
		t.Fatalf("model path was not mapped")
	}
	if mapped.Model.Kokoro.Voices != config.Kokoro.Voices {
		t.Fatalf("voices path was not mapped")
	}
	if mapped.Model.Kokoro.LengthScale != 1 {
		t.Fatalf("unexpected length scale %f", mapped.Model.Kokoro.LengthScale)
	}
	if mapped.Model.NumThreads != 2 {
		t.Fatalf("expected two threads, got %d", mapped.Model.NumThreads)
	}
}

func TestSherpaOfflineSynthesizerSynthesizeMapsAudio(t *testing.T) {
	synth := &SherpaOfflineSynthesizer{
		config: &Config{Speed: 1},
		synthesizer: &fakeSynthesizer{
			numSpeakers: 2,
			audio: &generatedAudio{
				Samples:    []float32{0.1, -0.1, 0.2, -0.2},
				SampleRate: 2,
			},
		},
	}

	result, err := synth.Synthesize(context.Background(), types.SynthesisRequest{
		Text:      "hello",
		SpeakerID: 1,
		Speed:     1.25,
	})
	if err != nil {
		t.Fatalf("synthesis failed: %v", err)
	}
	if result.SampleRate != 2 {
		t.Fatalf("unexpected sample rate %d", result.SampleRate)
	}
	if result.Duration != 2*time.Second {
		t.Fatalf("unexpected duration %s", result.Duration)
	}
	if result.SpeakerID != 1 {
		t.Fatalf("unexpected speaker ID %d", result.SpeakerID)
	}
	if len(result.Samples) != 4 {
		t.Fatalf("expected four samples, got %d", len(result.Samples))
	}
}

func TestSherpaOfflineSynthesizerRejectsEmptyText(t *testing.T) {
	synth := &SherpaOfflineSynthesizer{config: &Config{Speed: 1}, synthesizer: &fakeSynthesizer{}}
	if _, err := synth.Synthesize(context.Background(), types.SynthesisRequest{Text: "   "}); err == nil {
		t.Fatal("empty text should fail before native calls")
	}
}

func TestSherpaOfflineSynthesizerRejectsUnknownSpeaker(t *testing.T) {
	synth := &SherpaOfflineSynthesizer{
		config:      &Config{Speed: 1},
		synthesizer: &fakeSynthesizer{numSpeakers: 1},
	}
	if _, err := synth.Synthesize(context.Background(), types.SynthesisRequest{Text: "hello", SpeakerID: 1}); err == nil {
		t.Fatal("speaker ID beyond native speaker count should fail")
	}
}

func TestSherpaOfflineSynthesizerCloseIdempotent(t *testing.T) {
	native := &fakeSynthesizer{}
	synth := &SherpaOfflineSynthesizer{config: &Config{Speed: 1}, synthesizer: native}

	if err := synth.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := synth.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
	if native.closeCount != 1 {
		t.Fatalf("expected one native close, got %d", native.closeCount)
	}
}

func validKokoroConfig(t *testing.T) Config {
	t.Helper()
	dataDir := t.TempDir()
	config := DefaultConfig()
	config.Enabled = true
	config.ModelFamily = FamilyKokoro
	config.NumThreads = 2
	config.Kokoro = KokoroConfig{
		Model:       tempFile(t, "kokoro.onnx"),
		Voices:      tempFile(t, "voices.bin"),
		Tokens:      tempFile(t, "tokens.txt"),
		DataDir:     dataDir,
		LengthScale: 1,
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("valid Kokoro config should validate: %v", err)
	}
	return config
}

func tempFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return path
}

type fakeSynthesizer struct {
	audio       *generatedAudio
	numSpeakers int
	closeCount  int
}

func (s *fakeSynthesizer) Generate(text string, speakerID int, speed float32) (*generatedAudio, error) {
	return s.audio, nil
}

func (s *fakeSynthesizer) NumSpeakers() int {
	return s.numSpeakers
}

func (s *fakeSynthesizer) SampleRate() int {
	if s.audio == nil {
		return 0
	}
	return s.audio.SampleRate
}

func (s *fakeSynthesizer) Close() error {
	s.closeCount++
	return nil
}
