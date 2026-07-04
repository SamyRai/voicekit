package diarization

import "testing"

func TestSegmentBySilencePreservesTimingAndSpeechSamples(t *testing.T) {
	config := &DiarizationConfig{
		MinSegmentLength: 0.03,
		MaxSegmentLength: 1.0,
		SilenceThreshold: 0.02,
		OverlapThreshold: 0.2,
	}
	segmenter := NewSegmenter(config)

	audioData := []float32{
		0, 0,
		0.2, 0.3, 0.4,
		0,
		0.5, 0.6,
		0, 0,
	}
	segments, err := segmenter.SegmentBySilence(audioData, 100)
	if err != nil {
		t.Fatalf("SegmentBySilence returned error: %v", err)
	}
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}

	segment := segments[0]
	assertFloat64Equal(t, segment.StartTime, 0.02)
	assertFloat64Equal(t, segment.EndTime, 0.08)
	assertFloat32SliceEqual(t, segment.Samples, []float32{0.2, 0.3, 0.4, 0.5, 0.6})
}

func TestBasicBackendSegmentAudioPreservesTimingAndSpeechSamples(t *testing.T) {
	config := &DiarizationConfig{
		MinSegmentLength:    0.2,
		MaxSegmentLength:    1.0,
		SilenceThreshold:    0.1,
		SimilarityThreshold: 0.7,
	}
	backend := newBasicBackend(config, nil, nil)

	audioData := []float32{0, 0.2, 0.3, 0.4, 0, 0.5, 0.6, 0.7, 0}
	segments, err := backend.segmentAudio(audioData, 10)
	if err != nil {
		t.Fatalf("segmentAudio returned error: %v", err)
	}
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}

	assertFloat64Equal(t, segments[0].StartTime, 0)
	assertFloat64Equal(t, segments[0].EndTime, 0.4)
	assertFloat32SliceEqual(t, segments[0].Samples, []float32{0.2, 0.3, 0.4})

	assertFloat64Equal(t, segments[1].StartTime, 0.5)
	assertFloat64Equal(t, segments[1].EndTime, 0.8)
	assertFloat32SliceEqual(t, segments[1].Samples, []float32{0.5, 0.6, 0.7})
}

func assertFloat64Equal(t *testing.T, got float64, want float64) {
	t.Helper()
	if got != want {
		t.Fatalf("got %f want %f", got, want)
	}
}

func assertFloat32SliceEqual(t *testing.T, got []float32, want []float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d samples, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample %d: got %f want %f", i, got[i], want[i])
		}
	}
}
