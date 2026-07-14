package speaker

// SpeakerEmbeddingExtractor defines the interface for speaker embedding extraction
type SpeakerEmbeddingExtractor interface {
	CreateStream() SpeakerStream
	IsReady(stream SpeakerStream) bool
	Compute(stream SpeakerStream) []float32
	Dim() int
	Delete()
}

// SpeakerEmbeddingManager defines the interface for speaker embedding management
type SpeakerEmbeddingManager interface {
	RegisterV(speakerID string, embeddings [][]float32) bool
	Search(embedding []float32, threshold float32) string
	Remove(speakerID string)
	Verify(speakerID string, embedding []float32, threshold float32) bool
	Contains(speakerID string) bool
	Delete()
}

// SpeakerStream defines the interface for speaker embedding streams
type SpeakerStream interface {
	AcceptWaveform(sampleRate int, samples []float32)
	InputFinished()
	Delete()
}

// VectorIndexer defines the interface for vector indexing operations
type VectorIndexer interface {
	// AddSpeaker adds a speaker to the index
	AddSpeaker(speakerID string, speakerData *SpeakerData) error

	// RemoveSpeaker removes a speaker from the index
	RemoveSpeaker(speakerID string) error

	// SearchSpeaker finds the most similar speaker to the given embedding
	// Returns speakerID, similarity score, and error
	SearchSpeaker(embedding []float32, threshold float32) (string, float32, error)

	// GetSpeakerCount returns the number of speakers in the index
	GetSpeakerCount() int

	// GetAllSpeakers returns all speakers in the index
	GetAllSpeakers() map[string]*SpeakerData

	// Close releases resources
	Close() error
}

// SpeakerDatabase defines the interface for speaker database operations
type SpeakerDatabase interface {
	// Speaker management
	RegisterSpeaker(speakerID, speakerName string, embeddings [][]float32) error
	GetSpeaker(speakerID string) (*SpeakerData, error)
	DeleteSpeaker(speakerID string) error
	ListSpeakers() ([]string, error)

	// Embedding operations
	GetSpeakerEmbedding(speakerID string) ([]float32, error)
	GetAllSpeakers() (map[string][]float32, error)

	// Statistics
	GetStats() (*DatabaseStats, error)

	// Lifecycle
	Close() error
}

// Logger interface for speaker operations
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// SimilarityCalculator defines the interface for similarity calculations
type SimilarityCalculator interface {
	CalculateSimilarity(queryEmbedding []float32, storedEmbeddings [][]float32) float32
	CosineSimilarity(a, b []float32) float32
}
