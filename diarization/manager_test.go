package diarization

import (
	"context"
	"errors"
	"testing"
)

func TestManagerReturnsUnavailableWhenExtractorMissing(t *testing.T) {
	config := DefaultDiarizationConfig()
	config.Backend = BackendBasic
	config.MinSegmentLength = 0.01
	manager, err := NewManager(config, nil)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	audio := make([]float32, 1000)
	for i := range audio {
		audio[i] = 0.5
	}

	_, err = manager.ProcessAudio(audio, 8000, "session")
	if !errors.Is(err, ErrEmbeddingExtractorUnavailable) {
		t.Fatalf("expected unavailable extractor error, got %v", err)
	}
}

func TestManagerUsesEmbeddingExtractor(t *testing.T) {
	config := DefaultDiarizationConfig()
	config.Backend = BackendBasic
	config.MinSegmentLength = 0.01
	config.SimilarityThreshold = 0.9
	manager, err := NewManagerWithBackend(config, nil, fakeExtractor{})
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	audio := make([]float32, 1000)
	for i := range audio {
		audio[i] = 0.5
	}

	result, err := manager.ProcessAudio(audio, 8000, "session")
	if err != nil {
		t.Fatalf("unexpected diarization error: %v", err)
	}
	if len(result.Segments) != 1 {
		t.Fatalf("expected one segment, got %d", len(result.Segments))
	}
	if result.Segments[0].Confidence != 0 {
		t.Fatalf("expected unknown confidence 0, got %f", result.Segments[0].Confidence)
	}
}

func TestNewManagerSherpaRequiresModelPaths(t *testing.T) {
	config := DefaultDiarizationConfig()
	config.Backend = BackendSherpaOffline
	if _, err := NewManager(config, nil); err == nil {
		t.Fatal("Sherpa diarization without model paths should fail")
	}
}

func TestManagerCloseIdempotent(t *testing.T) {
	backend := &fakeBackend{}
	manager := &Manager{config: DefaultDiarizationConfig(), backend: backend}

	if err := manager.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
	if backend.closeCount != 1 {
		t.Fatalf("expected one backend close, got %d", backend.closeCount)
	}
}

func TestIntegratorTypedRecognition(t *testing.T) {
	config := DefaultDiarizationConfig()
	integrator := NewIntegrator(config, nil)

	result, err := integrator.IntegrateRecognition(RecognitionResult{
		SessionID:  "session",
		Text:       "hello world",
		Language:   "en",
		Confidence: 0.8,
		Words: []RecognitionWordInfo{
			{Text: "hello", StartTime: 0, EndTime: 0.5, Confidence: 0.9},
			{Text: "world", StartTime: 0.6, EndTime: 1.0, Confidence: 0.7},
		},
	}, &DiarizationResult{
		SessionID: "session",
		Segments: []SpeakerSegment{
			{SpeakerID: "speaker_1", StartTime: 0, EndTime: 1, Duration: 1, Confidence: 0.5},
		},
		TotalDuration: 1,
	})
	if err != nil {
		t.Fatalf("unexpected integration error: %v", err)
	}
	if len(result.SpeakerSegments) != 1 {
		t.Fatalf("expected one speaker text segment, got %d", len(result.SpeakerSegments))
	}
	if result.SpeakerSegments[0].Text != "hello world" {
		t.Fatalf("unexpected segment text: %q", result.SpeakerSegments[0].Text)
	}
}

type fakeExtractor struct{}

func (fakeExtractor) ExtractEmbedding(ctx context.Context, audioData []float32, sampleRate int) ([]float32, error) {
	return []float32{1, 0, 0}, nil
}

type fakeBackend struct {
	closeCount int
}

func (b *fakeBackend) Process(ctx context.Context, request Request) (*DiarizationResult, error) {
	return &DiarizationResult{SessionID: request.SessionID}, nil
}

func (b *fakeBackend) Close() error {
	b.closeCount++
	return nil
}
