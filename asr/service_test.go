package asr

import (
	"context"
	"testing"
	"time"

	"github.com/SamyRai/voicekit"
	"github.com/SamyRai/voicekit/types"
)

func TestASRService_Basic(t *testing.T) {
	// Create ASR config
	config := &voicekit.ASRConfig{
		Enabled:              true,
		DefaultModel:         "whisper_large_v3",
		Language:             "en",
		Quantization:         "int8",
		MaxConcurrentStreams: 10,
		StreamTimeout:        300,
		ChunkSize:            16000,
		VADProvider:          "ten_vad",
		Logger:               voicekit.DefaultLogger(),
	}

	// Create ASR service
	service, err := NewService(config)
	if err != nil {
		t.Fatalf("Failed to create ASR service: %v", err)
	}
	defer service.Close()

	// Test basic audio processing
	ctx := context.Background()
	sessionID := "test-session-1"

	// Generate test audio (1 second at 16kHz)
	audio := make([]float32, 16000)
	for i := range audio {
		// Simple sine wave for testing
		audio[i] = float32(i%100) / 100.0
	}

	// Process audio chunk
	transcription, err := service.ProcessAudioChunk(ctx, sessionID, audio)
	if err != nil {
		t.Fatalf("Failed to process audio chunk: %v", err)
	}

	// Verify transcription
	if transcription == nil {
		t.Fatal("Expected transcription result, got nil")
	}

	if transcription.Text == "" {
		t.Log("Transcription text is empty (expected for mock implementation)")
	}

	if transcription.Language != "en" {
		t.Errorf("Expected language 'en', got '%s'", transcription.Language)
	}

	if transcription.IsPartial {
		t.Log("Result is partial (expected for streaming)")
	}
}

func TestASRService_ModelRegistration(t *testing.T) {
	config := &voicekit.ASRConfig{
		Enabled:      true,
		DefaultModel: "whisper_large_v3",
		Language:     "en",
		Quantization: "int8",
		Logger:       voicekit.DefaultLogger(),
	}

	service, err := NewService(config)
	if err != nil {
		t.Fatalf("Failed to create ASR service: %v", err)
	}
	defer service.Close()

	// Test model registration
	model := NewWhisperModel("test_model", "en", "int8")
	err = service.RegisterModel(model)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	// Test model retrieval
	retrieved, exists := service.GetModel("test_model")
	if !exists {
		t.Fatal("Model should exist after registration")
	}

	if retrieved.Name() != "test_model" {
		t.Errorf("Expected model name 'test_model', got '%s'", retrieved.Name())
	}
}

func TestASRService_ModelSelection(t *testing.T) {
	config := &voicekit.ASRConfig{
		Enabled:      true,
		DefaultModel: "whisper_large_v3",
		Language:     "en",
		Quantization: "int8",
		Logger:       voicekit.DefaultLogger(),
	}

	service, err := NewService(config)
	if err != nil {
		t.Fatalf("Failed to create ASR service: %v", err)
	}
	defer service.Close()

	// Register test models
	fastModel := NewWhisperModel("fast_model", "en", "int8")
	fastModel.latency = 100 * time.Millisecond

	accurateModel := NewWhisperModel("accurate_model", "en", "float32")
	accurateModel.latency = 500 * time.Millisecond

	service.RegisterModel(fastModel)
	service.RegisterModel(accurateModel)

	// Test model selection for speed
	selected, err := service.SelectModel("en", &types.ModelRequirements{
		PreferSpeed: true,
	})
	if err != nil {
		t.Fatalf("Failed to select model: %v", err)
	}

	if selected.Name() != "fast_model" {
		t.Errorf("Expected fast model selection, got '%s'", selected.Name())
	}
}

func BenchmarkASRService_ProcessAudioChunk(b *testing.B) {
	config := &voicekit.ASRConfig{
		Enabled:              true,
		DefaultModel:         "whisper_large_v3",
		Language:             "en",
		Quantization:         "int8",
		MaxConcurrentStreams: 10,
		StreamTimeout:        300,
		ChunkSize:            16000,
		VADProvider:          "ten_vad",
		Logger:               voicekit.DefaultLogger(),
	}

	service, err := NewService(config)
	if err != nil {
		b.Fatalf("Failed to create ASR service: %v", err)
	}
	defer service.Close()

	// Generate test audio
	audio := make([]float32, 16000)
	for i := range audio {
		audio[i] = float32(i%100) / 100.0
	}

	ctx := context.Background()
	sessionID := "bench-session"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ProcessAudioChunk(ctx, sessionID, audio)
		if err != nil {
			b.Fatalf("Failed to process audio chunk: %v", err)
		}
	}
}