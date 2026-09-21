package speaker

import (
	"context"
	"fmt"

	"go.glpx.pro/gdk/voice/indexing"
	"go.glpx.pro/gdk/voice/speaker/domain"
)

// VectorIndexAdapter adapts the generic indexing.VectorIndex to the domain.VectorIndex interface
type VectorIndexAdapter struct {
	index indexing.VectorIndex // Generic index from indexing package
}

// NewVectorIndexAdapter creates a new vector index adapter
func NewVectorIndexAdapter(index indexing.VectorIndex) *VectorIndexAdapter {
	return &VectorIndexAdapter{
		index: index,
	}
}

// AddSpeaker adds a speaker to the vector index
func (a *VectorIndexAdapter) AddSpeaker(ctx context.Context, speaker *domain.Speaker) error {
	// Use the first embedding as representative for indexing
	if len(speaker.Embeddings()) == 0 {
		return fmt.Errorf("speaker has no embeddings")
	}

	embedding := speaker.Embeddings()[0].Vector()
	return a.index.Add(ctx, speaker.ID().String(), embedding)
}

// RemoveSpeaker removes a speaker from the vector index
func (a *VectorIndexAdapter) RemoveSpeaker(ctx context.Context, speakerID domain.SpeakerID) error {
	return a.index.Remove(ctx, speakerID.String())
}

// SearchSimilar finds the most similar speaker to the given embedding
func (a *VectorIndexAdapter) SearchSimilar(
	ctx context.Context,
	embedding domain.SpeakerEmbedding,
	threshold domain.SimilarityScore,
) (domain.SpeakerID, domain.SimilarityScore, error) {
	// Search for top 1 result
	results, err := a.index.Search(ctx, embedding.Vector(), 1, threshold.Float32())
	if err != nil {
		return "", domain.MinSimilarityScore, err
	}

	if len(results) == 0 {
		return "", domain.MinSimilarityScore, nil
	}

	// Get the best result
	result := results[0]

	speakerID, err := domain.NewSpeakerID(result.ID)
	if err != nil {
		return "", domain.MinSimilarityScore, fmt.Errorf("invalid speaker ID from index: %w", err)
	}

	similarityScore, err := domain.NewSimilarityScore(result.Similarity)
	if err != nil {
		return "", domain.MinSimilarityScore, fmt.Errorf("invalid similarity score from index: %w", err)
	}

	return speakerID, similarityScore, nil
}

// GetSpeaker retrieves speaker data from the index (not supported by generic index)
func (a *VectorIndexAdapter) GetSpeaker(ctx context.Context, speakerID domain.SpeakerID) (*domain.Speaker, error) {
	// The generic indexing.VectorIndex doesn't provide access to full speaker data
	// This would need to be implemented differently - perhaps by storing a reference
	// or using a separate lookup mechanism
	return nil, fmt.Errorf("GetSpeaker not supported by generic vector index - use repository instead")
}

// Size returns the number of speakers in the index
func (a *VectorIndexAdapter) Size(ctx context.Context) (int, error) {
	return a.index.Size(ctx)
}

// Close releases resources used by the index
func (a *VectorIndexAdapter) Close() error {
	return a.index.Close()
}
