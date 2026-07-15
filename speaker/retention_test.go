package speaker

import (
	"fmt"
	"testing"
	"time"
)

func TestSelectEvictionsFIFOChoosesOldestCreatedAtFirst(t *testing.T) {
	base := time.Now()
	speakers := []*SpeakerInfo{
		{ID: "c", CreatedAt: base.Add(2 * time.Hour)},
		{ID: "a", CreatedAt: base},
		{ID: "b", CreatedAt: base.Add(1 * time.Hour)},
	}
	got := selectEvictions(speakers, 2, RetentionFIFO, 0, base.Add(3*time.Hour)) // keep the 2 newest by CreatedAt
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("expected to evict the oldest [a], got %v", got)
	}
}

func TestSelectEvictionsFIFOEvictsMultipleOldest(t *testing.T) {
	base := time.Now()
	var speakers []*SpeakerInfo
	for i := 0; i < 5; i++ {
		speakers = append(speakers, &SpeakerInfo{
			ID:        fmt.Sprintf("s%d", i),
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}
	got := selectEvictions(speakers, 2, RetentionFIFO, 0, base.Add(time.Hour)) // evict the 3 oldest: s0, s1, s2
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
	now := time.Now()
	speakers := []*SpeakerInfo{{ID: "a"}, {ID: "b"}}
	if got := selectEvictions(speakers, 0, RetentionFIFO, 0, now); got != nil {
		t.Errorf("maxSpeakers=0 (unlimited), maxAge=0 (no TTL) should evict nothing, got %v", got)
	}
	if got := selectEvictions(speakers, 5, RetentionFIFO, 0, now); got != nil {
		t.Errorf("under the cap should evict nothing, got %v", got)
	}
	if got := selectEvictions(speakers, 2, RetentionFIFO, 0, now); got != nil {
		t.Errorf("exactly at the cap should evict nothing, got %v", got)
	}
	if got := selectEvictions(nil, 3, RetentionFIFO, 0, now); got != nil {
		t.Errorf("empty input should evict nothing, got %v", got)
	}
}

// TestSelectEvictionsFIFOVsLRUChooseDifferentVictims proves the policy
// switch actually changes behavior: build a case where the speaker with the
// oldest CreatedAt is NOT the least-recently-used speaker, so FIFO and LRU
// must evict different IDs.
func TestSelectEvictionsFIFOVsLRUChooseDifferentVictims(t *testing.T) {
	base := time.Now()
	speakers := []*SpeakerInfo{
		// "a" was created first (oldest CreatedAt) but was just used, so it
		// is NOT least-recently-used.
		{ID: "a", CreatedAt: base, LastUsedAt: base.Add(5 * time.Hour)},
		// "b" was created after "a" but has never been used since, so it is
		// the least-recently-used speaker.
		{ID: "b", CreatedAt: base.Add(1 * time.Hour), LastUsedAt: base.Add(1 * time.Hour)},
		// "c" is the newest by both CreatedAt and LastUsedAt.
		{ID: "c", CreatedAt: base.Add(2 * time.Hour), LastUsedAt: base.Add(2 * time.Hour)},
	}

	now := base.Add(6 * time.Hour)

	fifoEvicted := selectEvictions(speakers, 2, RetentionFIFO, 0, now)
	if len(fifoEvicted) != 1 || fifoEvicted[0] != "a" {
		t.Fatalf("FIFO expected to evict the oldest CreatedAt [a], got %v", fifoEvicted)
	}

	lruEvicted := selectEvictions(speakers, 2, RetentionLRU, 0, now)
	if len(lruEvicted) != 1 || lruEvicted[0] != "b" {
		t.Fatalf("LRU expected to evict the least-recently-used [b], got %v", lruEvicted)
	}

	if fifoEvicted[0] == lruEvicted[0] {
		t.Fatalf("expected FIFO and LRU to choose different victims, both chose %q", fifoEvicted[0])
	}
}

func TestSelectEvictionsLRUChoosesLeastRecentlyUsedFirst(t *testing.T) {
	base := time.Now()
	var speakers []*SpeakerInfo
	for i := 0; i < 5; i++ {
		speakers = append(speakers, &SpeakerInfo{
			ID:         fmt.Sprintf("s%d", i),
			CreatedAt:  base, // identical CreatedAt: FIFO order would be ambiguous/stable-sort only
			LastUsedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}
	got := selectEvictions(speakers, 2, RetentionLRU, 0, base.Add(time.Hour)) // evict the 3 least-recently-used: s0, s1, s2
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

// TestSelectEvictionsMaxAgeEvictsIdleSpeakersRegardlessOfCap proves the
// optional TTL evicts speakers idle longer than maxAge even when the
// capacity cap alone would not have evicted them.
func TestSelectEvictionsMaxAgeEvictsIdleSpeakersRegardlessOfCap(t *testing.T) {
	base := time.Now()
	speakers := []*SpeakerInfo{
		{ID: "stale", CreatedAt: base, LastUsedAt: base},                     // idle for the whole window
		{ID: "fresh", CreatedAt: base, LastUsedAt: base.Add(23 * time.Hour)}, // used recently
	}
	now := base.Add(24 * time.Hour)

	// maxSpeakers=0 (unlimited): only the TTL should evict anything.
	got := selectEvictions(speakers, 0, RetentionFIFO, time.Hour, now)
	if len(got) != 1 || got[0] != "stale" {
		t.Fatalf("expected maxAge to evict only [stale], got %v", got)
	}
}

func TestSelectEvictionsMaxAgeDisabledWhenZero(t *testing.T) {
	base := time.Now()
	speakers := []*SpeakerInfo{
		{ID: "a", CreatedAt: base, LastUsedAt: base},
	}
	if got := selectEvictions(speakers, 0, RetentionFIFO, 0, base.Add(365*24*time.Hour)); got != nil {
		t.Fatalf("maxAge=0 should disable TTL eviction, got %v", got)
	}
}

// TestSelectEvictionsMaxAgeAndCapCombine proves a speaker evicted by TTL is
// not double-counted against the capacity cap, and the cap still evicts
// additional survivors as needed.
func TestSelectEvictionsMaxAgeAndCapCombine(t *testing.T) {
	base := time.Now()
	speakers := []*SpeakerInfo{
		{ID: "stale", CreatedAt: base, LastUsedAt: base}, // evicted by TTL
		{ID: "a", CreatedAt: base.Add(1 * time.Hour), LastUsedAt: base.Add(23 * time.Hour)},
		{ID: "b", CreatedAt: base.Add(2 * time.Hour), LastUsedAt: base.Add(23 * time.Hour)},
	}
	now := base.Add(24 * time.Hour)

	// TTL evicts "stale"; cap of 1 over the 2 survivors (a, b) then evicts
	// the FIFO-oldest survivor "a", for a total of 2 evictions.
	got := selectEvictions(speakers, 1, RetentionFIFO, time.Hour, now)
	if len(got) != 2 {
		t.Fatalf("expected 2 evictions (stale by TTL, a by cap), got %v", got)
	}
	want := map[string]bool{"stale": true, "a": true}
	for _, id := range got {
		if !want[id] {
			t.Errorf("unexpected eviction %q", id)
		}
	}
}
