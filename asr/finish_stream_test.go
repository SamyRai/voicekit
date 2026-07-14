package asr

import (
	"context"
	"testing"

	"github.com/SamyRai/voicekit/types"
)

// These tests are hermetic: they exercise the public FinishStream entry point
// against fake models/streams with no native weights, proving the finalization
// path is reachable by sessionID (not only via a VAD endpoint) and that it
// flushes the underlying stream exactly once.

func TestFinishStreamFinalizesBufferedSession(t *testing.T) {
	service := newTestService()
	defer service.Close()

	model := &finalizableTestModel{
		testModel: testModel{name: "fake", language: "en", quantization: "int8"},
	}
	if err := service.RegisterModel(model); err != nil {
		t.Fatalf("failed to register model: %v", err)
	}

	ctx := context.Background()
	sessionID := "finish-session"

	// Feed a chunk so the session has buffered audio to finalize.
	audio := make([]float32, service.config.ChunkSize)
	if _, err := service.ProcessAudioChunk(ctx, sessionID, audio); err != nil {
		t.Fatalf("process audio chunk failed: %v", err)
	}

	final, err := service.FinishStream(ctx, sessionID)
	if err != nil {
		t.Fatalf("FinishStream failed: %v", err)
	}
	if final == nil {
		t.Fatal("expected a final transcription")
	}
	if final.IsPartial {
		t.Fatal("FinishStream must return a non-partial result")
	}
	if final.Text != "final transcript" {
		t.Fatalf("expected finalized text, got %q", final.Text)
	}
	if model.finishCalls != 1 {
		t.Fatalf("expected FinishAudio called exactly once, got %d", model.finishCalls)
	}
}

func TestFinishStreamFlushesNativeStreamOnce(t *testing.T) {
	recognizer := &fakeOnlineRecognizer{}
	model := newSherpaOnlineModelForTest(recognizer)

	service := newTestService()
	service.config.DefaultModel = model.Name()
	defer service.Close()
	if err := service.RegisterModel(model); err != nil {
		t.Fatalf("failed to register model: %v", err)
	}

	ctx := context.Background()
	sessionID := "native-session"

	audio := make([]float32, service.config.ChunkSize)
	if _, err := service.ProcessAudioChunk(ctx, sessionID, audio); err != nil {
		t.Fatalf("process audio chunk failed: %v", err)
	}

	final, err := service.FinishStream(ctx, sessionID)
	if err != nil {
		t.Fatalf("FinishStream failed: %v", err)
	}
	if final == nil || final.IsPartial {
		t.Fatal("expected a non-partial final transcription")
	}
	if final.Text == "" {
		t.Fatal("expected non-empty finalized text from the model")
	}

	if len(recognizer.streams) != 1 {
		t.Fatalf("expected a single native stream, got %d", len(recognizer.streams))
	}
	stream := recognizer.streams[0]
	if stream.inputFinishedCount != 1 {
		t.Fatalf("expected InputFinished called exactly once, got %d", stream.inputFinishedCount)
	}
}

func TestFinishStreamUnknownSessionReturnsEmptyFinal(t *testing.T) {
	service := newTestService()
	defer service.Close()

	final, err := service.FinishStream(context.Background(), "never-seen")
	if err != nil {
		t.Fatalf("finalizing an unknown session should not error: %v", err)
	}
	if final == nil {
		t.Fatal("expected an empty final transcription, got nil")
	}
	if final.IsPartial {
		t.Fatal("empty final must be non-partial")
	}
	if final.Text != "" {
		t.Fatalf("expected empty text for an unknown session, got %q", final.Text)
	}
}

func TestFinishStreamValidatesArguments(t *testing.T) {
	service := newTestService()
	defer service.Close()

	if _, err := service.FinishStream(nil, "session"); err == nil { //nolint:staticcheck // deliberately passing nil ctx to assert validation
		t.Fatal("expected error for nil context")
	}
	if _, err := service.FinishStream(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty sessionID")
	}
}

var _ types.ASRService = (*Service)(nil)
