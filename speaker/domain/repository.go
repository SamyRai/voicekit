package domain

import "context"

// SpeakerRepository defines the interface for speaker data persistence
type SpeakerRepository interface {
	// Save stores a speaker in the repository
	Save(ctx context.Context, speaker *Speaker) error

	// FindByID retrieves a speaker by ID
	FindByID(ctx context.Context, id SpeakerID) (*Speaker, error)

	// FindAll retrieves all speakers
	FindAll(ctx context.Context) ([]*Speaker, error)

	// Delete removes a speaker from the repository
	Delete(ctx context.Context, id SpeakerID) error

	// Exists checks if a speaker exists
	Exists(ctx context.Context, id SpeakerID) (bool, error)

	// Count returns the total number of speakers
	Count(ctx context.Context) (int, error)
}

// EmbeddingExtractor defines the interface for extracting speaker embeddings from audio
type EmbeddingExtractor interface {
	// ExtractEmbedding extracts a speaker embedding from audio data
	ExtractEmbedding(ctx context.Context, audioData []float32, sampleRate int) (SpeakerEmbedding, error)

	// Dimension returns the dimensionality of the embeddings
	Dimension() int

	// IsReady checks if the extractor is ready for use
	IsReady() bool

	// Close releases resources used by the extractor
	Close() error
}

// SimilarityCalculator defines the interface for calculating similarity between embeddings
type SimilarityCalculator interface {
	// CalculateSimilarity computes similarity between query and stored embeddings
	// Returns the maximum similarity score across all stored embeddings
	CalculateSimilarity(queryEmbedding SpeakerEmbedding, storedEmbeddings []SpeakerEmbedding) SimilarityScore

	// CosineSimilarity computes cosine similarity between two embeddings
	CosineSimilarity(a, b SpeakerEmbedding) SimilarityScore
}

// VectorIndex defines the interface for efficient vector similarity search
type VectorIndex interface {
	// AddSpeaker adds a speaker to the vector index
	AddSpeaker(ctx context.Context, speaker *Speaker) error

	// RemoveSpeaker removes a speaker from the vector index
	RemoveSpeaker(ctx context.Context, speakerID SpeakerID) error

	// SearchSimilar finds the most similar speaker to the given embedding
	// Returns speakerID, similarity score, and error
	SearchSimilar(ctx context.Context, embedding SpeakerEmbedding, threshold SimilarityScore) (SpeakerID, SimilarityScore, error)

	// GetSpeaker retrieves speaker data from the index (for metadata access)
	GetSpeaker(ctx context.Context, speakerID SpeakerID) (*Speaker, error)

	// Size returns the number of speakers in the index
	Size(ctx context.Context) (int, error)

	// Close releases resources used by the index
	Close() error
}
