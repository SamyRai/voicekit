package domain

import "fmt"

// SimilarityScore represents a similarity score between 0.0 and 1.0
type SimilarityScore float32

const (
	// MinSimilarityScore represents the minimum possible similarity score
	MinSimilarityScore SimilarityScore = 0.0
	// MaxSimilarityScore represents the maximum possible similarity score
	MaxSimilarityScore SimilarityScore = 1.0
)

// NewSimilarityScore creates a new similarity score with validation
func NewSimilarityScore(score float32) (SimilarityScore, error) {
	if score < 0.0 || score > 1.0 {
		return 0, fmt.Errorf("similarity score must be between 0.0 and 1.0, got %f", score)
	}
	return SimilarityScore(score), nil
}

// Float32 returns the similarity score as a float32
func (s SimilarityScore) Float32() float32 {
	return float32(s)
}

// IsAboveThreshold checks if this score is above the given threshold
func (s SimilarityScore) IsAboveThreshold(threshold SimilarityScore) bool {
	return s >= threshold
}

// String returns a string representation of the similarity score
func (s SimilarityScore) String() string {
	return fmt.Sprintf("%.4f", float32(s))
}