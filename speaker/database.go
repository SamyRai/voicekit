package speaker

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/SamyRai/voicekit/speaker/domain"
)

// GetSpeakerEmbedding returns the embedding for a specific speaker
func (m *Manager) GetSpeakerEmbedding(speakerID string) ([]float32, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.database.GetSpeakerEmbedding(speakerID)
}

// GetAllSpeakersEmbeddings returns all speakers with their embeddings
func (m *Manager) GetAllSpeakersEmbeddings() map[string][]float32 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	embeddings, err := m.database.GetAllSpeakers()
	if err != nil {
		m.logger.Errorf("Failed to get all speakers embeddings: %v", err)
		return make(map[string][]float32)
	}

	return embeddings
}

// RegisterSpeakerEmbedding registers a speaker embedding directly (used by diarization)
func (m *Manager) RegisterSpeakerEmbedding(speakerID string, embedding []float32) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	atomic.AddInt64(&m.registrationCount, 1)

	// For diarization purposes, we create a SpeakerData with a single embedding
	speakerData := &SpeakerData{
		ID:          speakerID,
		Name:        speakerID, // Use ID as name for diarization-generated speakers
		Embeddings:  [][]float32{embedding},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		SampleCount: 1,
	}

	// Register to Sherpa memory manager
	success := m.manager.RegisterV(speakerID, speakerData.Embeddings)
	if !success {
		atomic.AddInt64(&m.errorCount, 1)
		return fmt.Errorf("memory registration failed for speaker %s", speakerID)
	}

	// Create domain speaker for indexing
	domainID, _ := domain.NewSpeakerID(speakerID)
	domainName, _ := domain.NewSpeakerName(speakerID)

	// Convert embedding
	domainEmbedding, _ := domain.NewSpeakerEmbedding(embedding)
	speaker := domain.NewSpeaker(domainID, domainName, domainEmbedding)

	// Add to vector index
	err := m.vectorIndex.AddSpeaker(context.Background(), speaker)
	if err != nil {
		atomic.AddInt64(&m.errorCount, 1)
		return fmt.Errorf("vector indexing failed for speaker %s: %v", speakerID, err)
	}

	// Save to database
	if err := m.database.RegisterSpeaker(speakerID, speakerID, speakerData.Embeddings); err != nil {
		atomic.AddInt64(&m.errorCount, 1)
		return fmt.Errorf("database save failed for speaker %s: %v", speakerID, err)
	}

	m.logger.Infof("Successfully registered speaker %s with embedding via diarization", speakerID)
	return nil
}
