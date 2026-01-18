package speaker

import (
	"context"
	"fmt"
	"math"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// extractEmbedding extracts speaker embedding from audio data
func (m *Manager) extractEmbedding(ctx context.Context, audioData []float32, sampleRate int) ([]float32, error) {
	// Check for context cancellation before expensive operation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Create audio stream
	stream := m.extractor.CreateStream()
	if stream == nil {
		return nil, fmt.Errorf("failed to create audio stream")
	}
	defer stream.Delete()

	// Accept audio data
	stream.AcceptWaveform(sampleRate, audioData)
	stream.InputFinished()

	// Check if ready
	if !m.extractor.IsReady(stream) {
		return nil, fmt.Errorf("insufficient audio data for embedding extraction")
	}

	// Extract embedding
	embedding := m.extractor.Compute(stream)
	if len(embedding) == 0 {
		return nil, fmt.Errorf("failed to extract embedding")
	}

	return embedding, nil
}

// calculateSimilarity calculates speaker embedding similarity
func (m *Manager) calculateSimilarity(queryEmbedding []float32, storedEmbeddings [][]float32) float32 {
	if len(storedEmbeddings) == 0 {
		return 0.0
	}

	maxSimilarity := float32(0.0)

	// Calculate cosine similarity with all stored embeddings, take maximum
	for _, embedding := range storedEmbeddings {
		similarity := CosineSimilarity(queryEmbedding, embedding)
		if similarity > maxSimilarity {
			maxSimilarity = similarity
		}
	}

	return maxSimilarity
}

// CosineSimilarity calculates cosine similarity between two vectors
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float32

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	similarity := dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
	return similarity
}

// cosineSimilarityOptimized uses optimized calculation with better numerical stability
func cosineSimilarityOptimized(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0.0
	}

	// Use Kahan summation for better numerical stability
	var dotProduct, normA, normB float32
	var dotCompensate, normACompensate, normBCompensate float32

	for i := 0; i < len(a); i++ {
		// Dot product with Kahan summation
		y := a[i]*b[i] - dotCompensate
		t := dotProduct + y
		dotCompensate = (t - dotProduct) - y
		dotProduct = t

		// Norm A with Kahan summation
		y = a[i]*a[i] - normACompensate
		t = normA + y
		normACompensate = (t - normA) - y
		normA = t

		// Norm B with Kahan summation
		y = b[i]*b[i] - normBCompensate
		t = normB + y
		normBCompensate = (t - normB) - y
		normB = t
	}

	if normA <= 0 || normB <= 0 {
		return 0.0
	}

	normA = float32(math.Sqrt(float64(normA)))
	normB = float32(math.Sqrt(float64(normB)))

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (normA * normB)
}

// NoOpLogger provides a no-op implementation of Logger
type NoOpLogger struct{}

func (l *NoOpLogger) Infof(format string, args ...interface{})  {}
func (l *NoOpLogger) Warnf(format string, args ...interface{})  {}
func (l *NoOpLogger) Errorf(format string, args ...interface{}) {}
