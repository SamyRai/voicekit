package speaker

import (
	"sync/atomic"
	"time"
)

// GetDatabaseStats returns database statistics
func (m *Manager) GetDatabaseStats() *DatabaseStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats, err := m.database.GetStats()
	if err != nil {
		m.logger.Errorf("Failed to get database stats: %v", err)
		return &DatabaseStats{
			TotalSpeakers: 0,
			TotalSamples:  0,
			EmbeddingDim:  m.embeddingDim,
			Threshold:     m.threshold,
			Version:       "unknown",
			UpdatedAt:     time.Now(),
		}
	}

	return stats
}

// GetStats returns statistics (for main service monitoring)
func (m *Manager) GetStats() map[string]any {
	stats := m.GetDatabaseStats()
	return map[string]any{
		"speaker_count":      stats.TotalSpeakers,
		"total_samples":      stats.TotalSamples,
		"embedding_dim":      stats.EmbeddingDim,
		"threshold":          stats.Threshold,
		"version":            stats.Version,
		"last_updated":       stats.UpdatedAt.Format(time.RFC3339),
		"identify_requests":  atomic.LoadInt64(&m.identifyRequests),
		"identify_successes": atomic.LoadInt64(&m.identifySuccesses),
		"verify_requests":    atomic.LoadInt64(&m.verifyRequests),
		"verify_successes":   atomic.LoadInt64(&m.verifySuccesses),
		"registration_count": atomic.LoadInt64(&m.registrationCount),
		"error_count":        atomic.LoadInt64(&m.errorCount),
	}
}
