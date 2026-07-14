package speaker

import (
	"context"
	"os"
	"testing"
)

// TestManagerExtractEmbeddingIntegration exercises real speaker embedding
// extraction via the Sherpa speaker embedding extractor. It reads:
//
//	VOICEKIT_SPEAKER_MODEL - speaker embedding model path (required, e.g. 3D-Speaker CAM++)
func TestManagerExtractEmbeddingIntegration(t *testing.T) {
	modelPath := os.Getenv("VOICEKIT_SPEAKER_MODEL")
	if modelPath == "" {
		t.Skip("set VOICEKIT_SPEAKER_MODEL to run speaker embedding extraction integration")
	}

	config := &Config{
		ModelPath:  modelPath,
		NumThreads: 1,
		Provider:   "cpu",
		Threshold:  0.5,
		DataDir:    t.TempDir(),
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("failed to create speaker manager: %v", err)
	}
	defer manager.Close()

	audio := make([]float32, 16000)
	embedding, err := manager.ExtractEmbedding(context.Background(), audio, 16000)
	if err != nil {
		t.Fatalf("speaker embedding extraction failed: %v", err)
	}
	if len(embedding) == 0 {
		t.Fatal("expected non-empty speaker embedding")
	}
}
