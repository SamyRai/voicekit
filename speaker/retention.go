package speaker

import (
	"context"
	"sort"
)

// selectEvictions returns the IDs of speakers to evict so the database honors
// maxSpeakers, choosing the oldest speakers by CreatedAt first. A non-positive
// maxSpeakers means unlimited (no evictions).
func selectEvictions(speakers []*SpeakerInfo, maxSpeakers int) []string {
	if maxSpeakers <= 0 || len(speakers) <= maxSpeakers {
		return nil
	}
	sorted := make([]*SpeakerInfo, len(speakers))
	copy(sorted, speakers)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})
	evictCount := len(sorted) - maxSpeakers
	ids := make([]string, 0, evictCount)
	for i := 0; i < evictCount; i++ {
		ids = append(ids, sorted[i].ID)
	}
	return ids
}

// enforceRetention evicts the oldest speakers until the database honors the
// configured maxSpeakers cap. It is a no-op when maxSpeakers is 0 (unlimited),
// and is called after a successful registration. Eviction is serialized so
// concurrent registrations cannot over-evict.
func (m *Manager) enforceRetention(ctx context.Context) {
	if m.maxSpeakers <= 0 {
		return
	}
	m.retentionMu.Lock()
	defer m.retentionMu.Unlock()

	for _, id := range selectEvictions(m.GetAllSpeakersContext(ctx), m.maxSpeakers) {
		if err := m.DeleteSpeakerContext(ctx, id); err != nil && m.logger != nil {
			m.logger.Warnf("speaker retention: failed to evict %s: %v", id, err)
		}
	}
}
