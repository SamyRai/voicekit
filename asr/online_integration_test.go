package asr

import (
	"context"
	"os"
	"testing"
)

// TestSherpaOnlineModelIntegration exercises the streaming Sherpa ASR backend
// against real model files. It reads:
//
//	VOICEKIT_ONLINE_ASR_TOKENS     - tokens.txt path (required)
//	VOICEKIT_ONLINE_ASR_ENCODER    - transducer encoder path
//	VOICEKIT_ONLINE_ASR_DECODER    - transducer decoder path
//	VOICEKIT_ONLINE_ASR_JOINER     - transducer joiner path
//	VOICEKIT_ONLINE_ASR_MODEL      - single-file online model path (zipformer2_ctc/nemo_ctc/tone_ctc)
//	VOICEKIT_ONLINE_ASR_MODEL_TYPE - single-file model type, defaults to zipformer2_ctc
//
// Set VOICEKIT_ONLINE_ASR_TOKENS plus either the transducer trio
// (encoder+decoder+joiner) or a single-file model to run.
func TestSherpaOnlineModelIntegration(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Backend = BackendSherpaOnline
	config.DefaultModel = "integration_sherpa_online"
	config.Online.TokensPath = os.Getenv("VOICEKIT_ONLINE_ASR_TOKENS")
	config.Online.EncoderPath = os.Getenv("VOICEKIT_ONLINE_ASR_ENCODER")
	config.Online.DecoderPath = os.Getenv("VOICEKIT_ONLINE_ASR_DECODER")
	config.Online.JoinerPath = os.Getenv("VOICEKIT_ONLINE_ASR_JOINER")
	config.Online.ModelPath = os.Getenv("VOICEKIT_ONLINE_ASR_MODEL")
	config.Online.ModelType = os.Getenv("VOICEKIT_ONLINE_ASR_MODEL_TYPE")
	config.Language = "en"

	if !onlineIntegrationConfigured(config.Online) {
		t.Skip("set VOICEKIT_ONLINE_ASR_* model path env vars to run Sherpa online ASR integration")
	}

	model, err := NewSherpaOnlineModel(&config)
	if err != nil {
		t.Fatalf("failed to create Sherpa online model: %v", err)
	}
	defer model.Close()

	audio := make([]float32, 16000)
	transcription, err := model.FinishAudio(context.Background(), audio, nil)
	if err != nil {
		t.Fatalf("online transcription failed: %v", err)
	}
	if transcription == nil {
		t.Fatal("expected transcription result")
	}
}

func onlineIntegrationConfigured(config OnlineConfig) bool {
	if config.TokensPath == "" {
		return false
	}
	hasTransducer := config.EncoderPath != "" && config.DecoderPath != "" && config.JoinerPath != ""
	hasSingleFile := config.ModelPath != ""
	return hasTransducer || hasSingleFile
}
