package indexing

import "context"

// VectorIndex provides efficient similarity search for high-dimensional vectors
type VectorIndex interface {
	// Add adds a vector with an ID to the index
	Add(ctx context.Context, id string, vector []float32) error

	// Remove removes a vector from the index by ID
	Remove(ctx context.Context, id string) error

	// Search finds the k most similar vectors to the query vector
	// Returns results above the similarity threshold
	Search(ctx context.Context, query []float32, k int, threshold float32) ([]SearchResult, error)

	// Size returns the number of vectors currently in the index
	Size(ctx context.Context) (int, error)

	// Close releases resources used by the index
	Close() error
}

// SearchResult represents a single search result
type SearchResult struct {
	// ID is the identifier of the found vector
	ID string
	// Similarity is the similarity score (0.0 to 1.0, higher is more similar)
	Similarity float32
	// Distance is the distance metric (lower is more similar)
	Distance float32
}

// DistanceMetric defines supported distance/similarity metrics
type DistanceMetric string

const (
	// Cosine similarity (recommended for embeddings)
	Cosine DistanceMetric = "cosine"
	// L2 distance (Euclidean)
	L2 DistanceMetric = "l2"
	// Inner product
	InnerProduct DistanceMetric = "ip" // Renamed to avoid conflict
)