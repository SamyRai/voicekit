package tts

import (
	"context"
	"os"
	"testing"

	"go.glpx.pro/gdk/voice/types"
)

// TestSherpaOfflineSynthesizerIntegration exercises the Sherpa offline TTS
// backend against real model files. Env var shape mirrors
// examples/tts_offline/main.go:
//
//	VOICEKIT_TTS_FAMILY         - kokoro (default), vits, matcha, or kitten
//	VOICEKIT_TTS_MODEL          - model path (vits/kokoro/kitten)
//	VOICEKIT_TTS_VOICES         - voices path (kokoro/kitten)
//	VOICEKIT_TTS_TOKENS         - tokens path
//	VOICEKIT_TTS_DATA_DIR       - espeak-ng-data directory
//	VOICEKIT_TTS_LEXICON        - optional lexicon path (vits/matcha/kokoro)
//	VOICEKIT_TTS_LANG           - optional kokoro language
//	VOICEKIT_TTS_ACOUSTIC_MODEL - matcha acoustic model path
//	VOICEKIT_TTS_VOCODER        - matcha vocoder path
func TestSherpaOfflineSynthesizerIntegration(t *testing.T) {
	config, ok := offlineTTSIntegrationConfigFromEnv()
	if !ok {
		t.Skip("set VOICEKIT_TTS_FAMILY plus the required VOICEKIT_TTS_* model path env vars to run Sherpa offline TTS integration")
	}

	synth, err := NewSherpaOfflineSynthesizer(config)
	if err != nil {
		t.Fatalf("failed to create Sherpa offline synthesizer: %v", err)
	}
	defer synth.Close()

	result, err := synth.Synthesize(context.Background(), types.SynthesisRequest{
		Text:  "VoiceKit offline text to speech integration test.",
		Speed: 1,
	})
	if err != nil {
		t.Fatalf("offline TTS synthesis failed: %v", err)
	}
	if result == nil || len(result.Samples) == 0 {
		t.Fatal("expected synthesized audio samples")
	}
}

func offlineTTSIntegrationConfigFromEnv() (*Config, bool) {
	config := DefaultConfig()
	config.Enabled = true
	config.Backend = BackendSherpaOffline
	config.ModelFamily = os.Getenv("VOICEKIT_TTS_FAMILY")
	if config.ModelFamily == "" {
		config.ModelFamily = FamilyKokoro
	}

	switch config.ModelFamily {
	case FamilyVits:
		config.Vits.Model = os.Getenv("VOICEKIT_TTS_MODEL")
		config.Vits.Lexicon = os.Getenv("VOICEKIT_TTS_LEXICON")
		config.Vits.Tokens = os.Getenv("VOICEKIT_TTS_TOKENS")
		config.Vits.DataDir = os.Getenv("VOICEKIT_TTS_DATA_DIR")
		return &config, config.Vits.Model != "" && config.Vits.Tokens != "" && config.Vits.DataDir != ""
	case FamilyMatcha:
		config.Matcha.AcousticModel = os.Getenv("VOICEKIT_TTS_ACOUSTIC_MODEL")
		config.Matcha.Vocoder = os.Getenv("VOICEKIT_TTS_VOCODER")
		config.Matcha.Lexicon = os.Getenv("VOICEKIT_TTS_LEXICON")
		config.Matcha.Tokens = os.Getenv("VOICEKIT_TTS_TOKENS")
		config.Matcha.DataDir = os.Getenv("VOICEKIT_TTS_DATA_DIR")
		return &config, config.Matcha.AcousticModel != "" && config.Matcha.Vocoder != "" &&
			config.Matcha.Tokens != "" && config.Matcha.DataDir != ""
	case FamilyKokoro:
		config.Kokoro.Model = os.Getenv("VOICEKIT_TTS_MODEL")
		config.Kokoro.Voices = os.Getenv("VOICEKIT_TTS_VOICES")
		config.Kokoro.Tokens = os.Getenv("VOICEKIT_TTS_TOKENS")
		config.Kokoro.DataDir = os.Getenv("VOICEKIT_TTS_DATA_DIR")
		config.Kokoro.Lexicon = os.Getenv("VOICEKIT_TTS_LEXICON")
		config.Kokoro.Lang = os.Getenv("VOICEKIT_TTS_LANG")
		return &config, config.Kokoro.Model != "" && config.Kokoro.Voices != "" &&
			config.Kokoro.Tokens != "" && config.Kokoro.DataDir != ""
	case FamilyKitten:
		config.Kitten.Model = os.Getenv("VOICEKIT_TTS_MODEL")
		config.Kitten.Voices = os.Getenv("VOICEKIT_TTS_VOICES")
		config.Kitten.Tokens = os.Getenv("VOICEKIT_TTS_TOKENS")
		config.Kitten.DataDir = os.Getenv("VOICEKIT_TTS_DATA_DIR")
		return &config, config.Kitten.Model != "" && config.Kitten.Voices != "" &&
			config.Kitten.Tokens != "" && config.Kitten.DataDir != ""
	default:
		return &config, false
	}
}
