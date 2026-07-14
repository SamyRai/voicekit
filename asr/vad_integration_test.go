package asr

import (
	"os"
	"testing"
)

// TestVADServiceIntegration exercises a real Sherpa VAD backend (Silero or TEN
// VAD) against a real model file. It reads:
//
//	VOICEKIT_VAD_PROVIDER - silero_vad or ten_vad (defaults to silero_vad)
//	VOICEKIT_VAD_MODEL    - VAD model path (required)
func TestVADServiceIntegration(t *testing.T) {
	provider := os.Getenv("VOICEKIT_VAD_PROVIDER")
	if provider == "" {
		provider = VADProviderSilero
	}
	modelPath := os.Getenv("VOICEKIT_VAD_MODEL")
	if modelPath == "" {
		t.Skip("set VOICEKIT_VAD_MODEL (and optionally VOICEKIT_VAD_PROVIDER) to run Sherpa VAD integration")
	}

	config := &VADConfig{
		Provider:   provider,
		ModelPath:  modelPath,
		SampleRate: 16000,
	}

	service, err := NewVADService(config)
	if err != nil {
		t.Fatalf("failed to create VAD service: %v", err)
	}
	defer service.Close()

	audio := make([]float32, 512)
	result, err := service.Process(audio, nil)
	if err != nil {
		t.Fatalf("VAD processing failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected VAD result")
	}
}
