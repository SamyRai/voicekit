package voicekit

import (
	"testing"
	"time"
)

// TestGetMetricsReturnsIndependentSnapshots guards the invariant that every
// GetMetrics call returns a freshly allocated snapshot the caller fully owns.
// It fails if GetMetrics is ever changed back to hand out a shared/pooled
// instance, which would alias an earlier caller's snapshot to later state.
func TestGetMetricsReturnsIndependentSnapshots(t *testing.T) {
	c := NewMetricsCollector()

	c.RecordAudioProcessing(time.Millisecond, true)
	first := c.GetMetrics()
	if first.AudioProcessCount != 1 {
		t.Fatalf("first snapshot AudioProcessCount = %d, want 1", first.AudioProcessCount)
	}

	c.RecordAudioProcessing(time.Millisecond, true)
	second := c.GetMetrics()

	if first == second {
		t.Fatal("GetMetrics returned the same snapshot instance for two calls")
	}
	if first.AudioProcessCount != 1 {
		t.Fatalf("earlier snapshot was mutated by a later GetMetrics: AudioProcessCount = %d, want 1", first.AudioProcessCount)
	}
	if second.AudioProcessCount != 2 {
		t.Fatalf("second snapshot AudioProcessCount = %d, want 2", second.AudioProcessCount)
	}
}
