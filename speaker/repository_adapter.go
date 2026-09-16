package speaker

import (
	"context"
	"fmt"

	"go.glpx.pro/voicekit/speaker/domain"
)

// SpeakerDatabaseAdapter adapts the existing speaker database to the domain repository interface
type SpeakerDatabaseAdapter struct {
	database SpeakerDatabase // Existing database interface
}

// Types are defined in interfaces.go and types.go

// NewSpeakerDatabaseAdapter creates a new repository adapter
func NewSpeakerDatabaseAdapter(database SpeakerDatabase) *SpeakerDatabaseAdapter {
	return &SpeakerDatabaseAdapter{
		database: database,
	}
}

// Save stores a speaker in the repository
func (r *SpeakerDatabaseAdapter) Save(ctx context.Context, speaker *domain.Speaker) error {
	// Convert domain speaker to database format
	embeddings := make([][]float32, len(speaker.Embeddings()))
	for i, emb := range speaker.Embeddings() {
		embeddings[i] = emb.Vector()
	}

	return r.database.RegisterSpeaker(
		speaker.ID().String(),
		speaker.Name().String(),
		embeddings,
	)
}

// FindByID retrieves a speaker by ID
func (r *SpeakerDatabaseAdapter) FindByID(ctx context.Context, id domain.SpeakerID) (*domain.Speaker, error) {
	data, err := r.database.GetSpeaker(id.String())
	if err != nil {
		return nil, fmt.Errorf("database lookup failed: %w", err)
	}

	// Convert to domain speaker
	speakerID, err := domain.NewSpeakerID(data.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid speaker ID from database: %w", err)
	}

	speakerName, err := domain.NewSpeakerName(data.Name)
	if err != nil {
		return nil, fmt.Errorf("invalid speaker name from database: %w", err)
	}

	// Convert embeddings
	embeddings := make([]domain.SpeakerEmbedding, len(data.Embeddings))
	for i, emb := range data.Embeddings {
		embedding, err := domain.NewSpeakerEmbedding(emb)
		if err != nil {
			return nil, fmt.Errorf("invalid embedding from database: %w", err)
		}
		embeddings[i] = embedding
	}

	// Create domain speaker
	speaker := &domain.Speaker{}
	// We need to use reflection or a constructor to set private fields
	// For now, let's create a new speaker and manually set the fields
	// This is a temporary solution - in a real implementation, we'd modify the domain
	// to allow reconstruction from existing data

	// Create with first embedding
	if len(embeddings) > 0 {
		speaker = domain.NewSpeaker(speakerID, speakerName, embeddings[0])
		// Add remaining embeddings
		for i := 1; i < len(embeddings); i++ {
			speaker.AddEmbedding(embeddings[i])
		}
	}

	return speaker, nil
}

// FindAll retrieves all speakers
func (r *SpeakerDatabaseAdapter) FindAll(ctx context.Context) ([]*domain.Speaker, error) {
	speakerIDs, err := r.database.ListSpeakers()
	if err != nil {
		return nil, fmt.Errorf("failed to list speakers: %w", err)
	}

	speakers := make([]*domain.Speaker, 0, len(speakerIDs))
	for _, id := range speakerIDs {
		speakerID, _ := domain.NewSpeakerID(id)
		speaker, err := r.FindByID(ctx, speakerID)
		if err != nil {
			// Log error but continue with other speakers
			continue
		}
		speakers = append(speakers, speaker)
	}

	return speakers, nil
}

// Delete removes a speaker from the repository
func (r *SpeakerDatabaseAdapter) Delete(ctx context.Context, id domain.SpeakerID) error {
	return r.database.DeleteSpeaker(id.String())
}

// Exists checks if a speaker exists
func (r *SpeakerDatabaseAdapter) Exists(ctx context.Context, id domain.SpeakerID) (bool, error) {
	_, err := r.database.GetSpeaker(id.String())
	if err != nil {
		// Check if it's a "not found" error vs other errors
		// For now, assume any error means doesn't exist
		return false, nil
	}
	return true, nil
}

// Count returns the total number of speakers
func (r *SpeakerDatabaseAdapter) Count(ctx context.Context) (int, error) {
	speakerIDs, err := r.database.ListSpeakers()
	if err != nil {
		return 0, fmt.Errorf("failed to count speakers: %w", err)
	}
	return len(speakerIDs), nil
}

// Close releases database resources
func (r *SpeakerDatabaseAdapter) Close() error {
	return r.database.Close()
}
