package speaker

import (
	"fmt"
	"testing"
	"time"
)

func TestSelectEvictionsChoosesOldestFirst(t *testing.T) {
	base := time.Now()
	speakers := []*SpeakerInfo{
		{ID: "c", CreatedAt: base.Add(2 * time.Hour)},
		{ID: "a", CreatedAt: base},
		{ID: "b", CreatedAt: base.Add(1 * time.Hour)},
	}
	got := selectEvictions(speakers, 2) // keep the 2 newest, evict the oldest
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("expected to evict the oldest [a], got %v", got)
	}
}

func TestSelectEvictionsEvictsMultipleOldest(t *testing.T) {
	base := time.Now()
	var speakers []*SpeakerInfo
	for i := 0; i < 5; i++ {
		speakers = append(speakers, &SpeakerInfo{
			ID:        fmt.Sprintf("s%d", i),
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}
	got := selectEvictions(speakers, 2) // evict the 3 oldest: s0, s1, s2
	if len(got) != 3 {
		t.Fatalf("expected 3 evictions, got %v", got)
	}
	want := map[string]bool{"s0": true, "s1": true, "s2": true}
	for _, id := range got {
		if !want[id] {
			t.Errorf("unexpected eviction %q", id)
		}
	}
}

func TestSelectEvictionsNoOpCases(t *testing.T) {
	speakers := []*SpeakerInfo{{ID: "a"}, {ID: "b"}}
	if got := selectEvictions(speakers, 0); got != nil {
		t.Errorf("maxSpeakers=0 (unlimited) should evict nothing, got %v", got)
	}
	if got := selectEvictions(speakers, 5); got != nil {
		t.Errorf("under the cap should evict nothing, got %v", got)
	}
	if got := selectEvictions(speakers, 2); got != nil {
		t.Errorf("exactly at the cap should evict nothing, got %v", got)
	}
	if got := selectEvictions(nil, 3); got != nil {
		t.Errorf("empty input should evict nothing, got %v", got)
	}
}
