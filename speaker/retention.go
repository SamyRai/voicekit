package speaker

import (
	"context"
	"sort"
	"time"
)

// RetentionPolicy selects which speaker is evicted first when the database
// exceeds Config.MaxSpeakers.
type RetentionPolicy string

const (
	// RetentionFIFO evicts the oldest speakers first, by CreatedAt. This is
	// the default and matches the retention behavior shipped before
	// RetentionPolicy existed.
	RetentionFIFO RetentionPolicy = "fifo"
	// RetentionLRU evicts the least-recently-used speakers first, by
	// LastUsedAt (refreshed on every successful identify/verify match).
	RetentionLRU RetentionPolicy = "lru"
)

// selectEvictions returns the IDs of speakers to evict so the database
// honors maxSpeakers (capacity) and maxAge (TTL).
//
// maxSpeakers <= 0 means no capacity cap; maxAge <= 0 means no TTL. Speakers
// idle longer than maxAge (by LastUsedAt) are evicted first regardless of
// policy; the capacity cap is then enforced over the remaining survivors,
// sorted oldest-first by CreatedAt (RetentionFIFO) or least-recently-used
// first by LastUsedAt (RetentionLRU). now is passed in explicitly to keep
// the function pure and deterministic for tests.
func selectEvictions(speakers []*SpeakerInfo, maxSpeakers int, policy RetentionPolicy, maxAge time.Duration, now time.Time) []string {
	if len(speakers) == 0 {
		return nil
	}

	evicted := make(map[string]struct{}, len(speakers))
	var ids []string

	if maxAge > 0 {
		for _, sp := range speakers {
			if now.Sub(sp.LastUsedAt) > maxAge {
				evicted[sp.ID] = struct{}{}
				ids = append(ids, sp.ID)
			}
		}
	}

	if maxSpeakers > 0 {
		survivors := make([]*SpeakerInfo, 0, len(speakers))
		for _, sp := range speakers {
			if _, gone := evicted[sp.ID]; !gone {
				survivors = append(survivors, sp)
			}
		}

		if len(survivors) > maxSpeakers {
			sorted := make([]*SpeakerInfo, len(survivors))
			copy(sorted, survivors)
			sort.SliceStable(sorted, func(i, j int) bool {
				return retentionKey(sorted[i], policy).Before(retentionKey(sorted[j], policy))
			})

			evictCount := len(sorted) - maxSpeakers
			for i := 0; i < evictCount; i++ {
				evicted[sorted[i].ID] = struct{}{}
				ids = append(ids, sorted[i].ID)
			}
		}
	}

	return ids
}

// retentionKey returns the timestamp selectEvictions sorts on for policy.
func retentionKey(s *SpeakerInfo, policy RetentionPolicy) time.Time {
	if policy == RetentionLRU {
		return s.LastUsedAt
	}
	return s.CreatedAt
}

// enforceRetention evicts speakers until the database honors the configured
// maxSpeakers cap and maxAge TTL. It is a no-op when neither is configured,
// and is called after a successful registration. Eviction is serialized so
// concurrent registrations cannot over-evict.
func (m *Manager) enforceRetention(ctx context.Context) {
	if m.maxSpeakers <= 0 && m.maxAge <= 0 {
		return
	}
	m.retentionMu.Lock()
	defer m.retentionMu.Unlock()

	policy := m.retentionPolicy
	if policy == "" {
		policy = RetentionFIFO
	}

	for _, id := range selectEvictions(m.GetAllSpeakersContext(ctx), m.maxSpeakers, policy, m.maxAge, time.Now()) {
		if err := m.DeleteSpeakerContext(ctx, id); err != nil && m.logger != nil {
			m.logger.Warnf("speaker retention: failed to evict %s: %v", id, err)
		}
	}
}

// speakerUpdater is satisfied by database backends that support in-place
// metadata updates (e.g. *PebbleSpeakerDatabase, *ShardedSpeakerDatabase).
// It is declared locally, rather than widening the public SpeakerDatabase
// interface, because LRU touch tracking is an internal retention concern.
type speakerUpdater interface {
	UpdateSpeaker(speakerID string, data *SpeakerData) error
}

// touchLastUsed records that speakerID was just matched by a successful
// identify or verify, which the RetentionLRU policy uses to pick eviction
// victims. Failures only degrade LRU accuracy, so they are logged and
// otherwise ignored -- a missed touch must never fail the caller's request.
func (m *Manager) touchLastUsed(speakerID string) {
	updater, ok := m.database.(speakerUpdater)
	if !ok {
		return
	}

	// Serialize the read-modify-write with enforceRetention (same retentionMu)
	// so an eviction cannot delete this speaker between the GetSpeaker read and
	// the UpdateSpeaker write below, which would otherwise resurrect a ghost
	// record that is gone from the vector index and memory manager.
	m.retentionMu.Lock()
	defer m.retentionMu.Unlock()

	data, err := m.database.GetSpeaker(speakerID)
	if err != nil {
		if m.logger != nil {
			m.logger.Warnf("speaker retention: failed to load %s for LRU touch: %v", speakerID, err)
		}
		return
	}

	data.LastUsedAt = time.Now()
	if err := updater.UpdateSpeaker(speakerID, data); err != nil && m.logger != nil {
		m.logger.Warnf("speaker retention: failed to update LRU timestamp for %s: %v", speakerID, err)
	}
}
