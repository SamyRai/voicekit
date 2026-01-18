package speaker

import (
	"math"

	"github.com/SamyRai/voicekit/speaker/domain"
)

// CosineSimilarityCalculator implements the domain similarity calculator using cosine similarity
type CosineSimilarityCalculator struct{}

// NewCosineSimilarityCalculator creates a new cosine similarity calculator
func NewCosineSimilarityCalculator() *CosineSimilarityCalculator {
	return &CosineSimilarityCalculator{}
}

// CalculateSimilarity computes similarity between query and stored embeddings
// Returns the maximum similarity score across all stored embeddings
func (c *CosineSimilarityCalculator) CalculateSimilarity(
	queryEmbedding domain.SpeakerEmbedding,
	storedEmbeddings []domain.SpeakerEmbedding,
) domain.SimilarityScore {
	if len(storedEmbeddings) == 0 {
		return domain.MinSimilarityScore
	}

	maxSimilarity := domain.MinSimilarityScore

	// Calculate cosine similarity with all stored embeddings, take maximum
	for _, storedEmbedding := range storedEmbeddings {
		similarity := c.CosineSimilarity(queryEmbedding, storedEmbedding)
		if similarity.Float32() > maxSimilarity.Float32() {
			maxSimilarity = similarity
		}
	}

	return maxSimilarity
}

// CosineSimilarity computes cosine similarity between two embeddings
func (c *CosineSimilarityCalculator) CosineSimilarity(a, b domain.SpeakerEmbedding) domain.SimilarityScore {
	aVec := a.Vector()
	bVec := b.Vector()

	if len(aVec) != len(bVec) {
		return domain.MinSimilarityScore
	}

	var dotProduct, normA, normB float32

	for i := 0; i < len(aVec); i++ {
		dotProduct += aVec[i] * bVec[i]
		normA += aVec[i] * aVec[i]
		normB += bVec[i] * bVec[i]
	}

	if normA == 0 || normB == 0 {
		return domain.MinSimilarityScore
	}

	similarity := dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))

	// Clamp to valid range (cosine similarity should be between -1 and 1, but due to floating point
	// precision issues, we clamp to [0, 1] for our use case)
	if similarity < 0 {
		similarity = 0
	}
	if similarity > 1 {
		similarity = 1
	}

	score, _ := domain.NewSimilarityScore(similarity)
	return score
}