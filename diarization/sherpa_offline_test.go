package diarization

import (
	"context"
	"testing"
	"time"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

func TestSherpaOfflineBackendMapsSegments(t *testing.T) {
	processedAt := time.Unix(10, 0)
	backend := &sherpaOfflineBackend{
		config: DefaultDiarizationConfig(),
		diarizer: &fakeDiarizer{
			sampleRate: 16000,
			segments: []sherpa.OfflineSpeakerDiarizationSegment{
				{Start: 0.25, End: 1.00, Speaker: 1},
				{Start: 1.10, End: 2.00, Speaker: 2},
			},
		},
	}

	result, err := backend.Process(context.Background(), Request{
		Audio:       make([]float32, 32000),
		SampleRate:  16000,
		SessionID:   "session",
		ProcessedAt: processedAt,
	})
	if err != nil {
		t.Fatalf("Sherpa backend failed: %v", err)
	}
	if result.SessionID != "session" {
		t.Fatalf("unexpected session ID %q", result.SessionID)
	}
	if result.ProcessedAt != processedAt {
		t.Fatalf("processed timestamp was not preserved")
	}
	if result.SpeakerCount != 2 {
		t.Fatalf("expected two speakers, got %d", result.SpeakerCount)
	}
	if len(result.Segments) != 2 {
		t.Fatalf("expected two segments, got %d", len(result.Segments))
	}
	if result.Segments[0].SpeakerID != "speaker_1" {
		t.Fatalf("unexpected speaker ID %q", result.Segments[0].SpeakerID)
	}
	if result.Segments[0].Duration != 0.75 {
		t.Fatalf("unexpected duration %.2f", result.Segments[0].Duration)
	}
	if result.TotalDuration != 2 {
		t.Fatalf("expected total duration 2, got %.2f", result.TotalDuration)
	}
}

func TestSherpaOfflineBackendRejectsSampleRateMismatch(t *testing.T) {
	backend := &sherpaOfflineBackend{
		config:   DefaultDiarizationConfig(),
		diarizer: &fakeDiarizer{sampleRate: 16000},
	}

	_, err := backend.Process(context.Background(), Request{
		Audio:      make([]float32, 8000),
		SampleRate: 8000,
		SessionID:  "session",
	})
	if err == nil {
		t.Fatal("sample-rate mismatch should fail")
	}
}

func TestSherpaOfflineBackendRejectsEmptyAudio(t *testing.T) {
	backend := &sherpaOfflineBackend{
		config:   DefaultDiarizationConfig(),
		diarizer: &fakeDiarizer{sampleRate: 16000},
	}

	if _, err := backend.Process(context.Background(), Request{SampleRate: 16000, SessionID: "session"}); err == nil {
		t.Fatal("empty audio should fail before native calls")
	}
}

func TestSherpaOfflineBackendCloseIdempotent(t *testing.T) {
	diarizer := &fakeDiarizer{sampleRate: 16000}
	backend := &sherpaOfflineBackend{config: DefaultDiarizationConfig(), diarizer: diarizer}

	if err := backend.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := backend.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
	if diarizer.closeCount != 1 {
		t.Fatalf("expected one native close, got %d", diarizer.closeCount)
	}
}

type fakeDiarizer struct {
	sampleRate int
	segments   []sherpa.OfflineSpeakerDiarizationSegment
	closeCount int
}

func (d *fakeDiarizer) SampleRate() int {
	return d.sampleRate
}

func (d *fakeDiarizer) Process(samples []float32) ([]sherpa.OfflineSpeakerDiarizationSegment, error) {
	return d.segments, nil
}

func (d *fakeDiarizer) Close() error {
	d.closeCount++
	return nil
}
