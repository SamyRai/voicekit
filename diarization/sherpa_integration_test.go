package diarization

import (
	"os"
	"testing"
)

func TestSherpaOfflineDiarizationIntegration(t *testing.T) {
	segmentationModel := os.Getenv("VOICEKIT_SHERPA_DIARIZATION_SEGMENTATION")
	embeddingModel := os.Getenv("VOICEKIT_SHERPA_DIARIZATION_EMBEDDING")
	if segmentationModel == "" || embeddingModel == "" {
		t.Skip("set VOICEKIT_SHERPA_DIARIZATION_SEGMENTATION and VOICEKIT_SHERPA_DIARIZATION_EMBEDDING to run Sherpa diarization integration")
	}

	config := DefaultDiarizationConfig()
	config.Backend = BackendSherpaOffline
	config.SegmentationModelPath = segmentationModel
	config.EmbeddingModelPath = embeddingModel

	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("failed to create Sherpa diarization manager: %v", err)
	}
	defer manager.Close()

	result, err := manager.ProcessAudio(make([]float32, 16000), 16000, "integration")
	if err != nil {
		t.Fatalf("Sherpa diarization failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected diarization result")
	}
}
