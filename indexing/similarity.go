package indexing

import "math"

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

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// L2Distance calculates L2 (Euclidean) distance between two vectors
func L2Distance(a, b []float32) float32 {
	if len(a) != len(b) {
		return float32(math.Inf(1)) // Return infinity for dimension mismatch
	}

	var sum float32
	for i := 0; i < len(a); i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return float32(math.Sqrt(float64(sum)))
}

// DotProduct calculates the inner product (dot product) of two vectors
func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0.0
	}

	var sum float32
	for i := 0; i < len(a); i++ {
		sum += a[i] * b[i]
	}

	return sum
}

// NormalizeVector normalizes a vector to unit length (L2 normalization)
func NormalizeVector(v []float32) []float32 {
	if len(v) == 0 {
		return v
	}

	var norm float32
	for _, val := range v {
		norm += val * val
	}

	if norm == 0 {
		return v // Can't normalize zero vector
	}

	norm = float32(math.Sqrt(float64(norm)))

	result := make([]float32, len(v))
	for i, val := range v {
		result[i] = val / norm
	}

	return result
}

// SimilarityToDistance converts similarity score to distance based on metric
func SimilarityToDistance(similarity float32, metric DistanceMetric) float32 {
	switch metric {
	case Cosine:
		// Cosine similarity to distance: distance = 1 - similarity
		return 1.0 - similarity
	case L2:
		// For L2, we can't directly convert similarity back to distance
		// This is a rough approximation
		if similarity <= 0 {
			return 1.0
		}
		return 1.0 / similarity
	case InnerProduct:
		// Inner product similarity is already in distance-like range
		return 1.0 - similarity
	default:
		return 1.0 - similarity
	}
}
