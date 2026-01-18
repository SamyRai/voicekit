package speaker

import (
	"context"
	"fmt"

	"github.com/SamyRai/voicekit/speaker/domain"
)

// SherpaEmbeddingExtractorAdapter adapts the existing Sherpa embedding extractor to the domain interface
type SherpaEmbeddingExtractorAdapter struct {
	extractor SpeakerEmbeddingExtractor // This will be the existing interface
	streamPool *StreamPool
}

// Interfaces are defined in interfaces.go

// NewSherpaEmbeddingExtractorAdapter creates a new adapter for the Sherpa embedding extractor
func NewSherpaEmbeddingExtractorAdapter(extractor SpeakerEmbeddingExtractor) *SherpaEmbeddingExtractorAdapter {
	return &SherpaEmbeddingExtractorAdapter{
		extractor:  extractor,
		streamPool: NewStreamPool(extractor),
	}
}

// ExtractEmbedding extracts a speaker embedding from audio data
func (a *SherpaEmbeddingExtractorAdapter) ExtractEmbedding(
	ctx context.Context,
	audioData []float32,
	sampleRate int,
) (domain.SpeakerEmbedding, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return domain.SpeakerEmbedding{}, ctx.Err()
	default:
	}

	// Get a stream from the pool
	stream := a.streamPool.Get()
	defer a.streamPool.Put(stream)

	// Accept audio data
	stream.AcceptWaveform(sampleRate, audioData)
	stream.InputFinished()

	// Check if ready for extraction
	if !a.extractor.IsReady(stream) {
		return domain.SpeakerEmbedding{}, domain.ErrInsufficientAudio
	}

	// Extract embedding
	embeddingVector := a.extractor.Compute(stream)
	if len(embeddingVector) == 0 {
		return domain.SpeakerEmbedding{}, domain.ErrEmbeddingExtraction
	}

	// Create domain embedding
	embedding, err := domain.NewSpeakerEmbedding(embeddingVector)
	if err != nil {
		return domain.SpeakerEmbedding{}, fmt.Errorf("failed to create domain embedding: %w", err)
	}

	return embedding, nil
}

// Dimension returns the dimensionality of the embeddings
func (a *SherpaEmbeddingExtractorAdapter) Dimension() int {
	return a.extractor.Dim()
}

// IsReady checks if the extractor is ready for use
func (a *SherpaEmbeddingExtractorAdapter) IsReady() bool {
	// For Sherpa, we assume it's ready if it was successfully created
	return a.extractor != nil
}

// Close releases resources used by the extractor
func (a *SherpaEmbeddingExtractorAdapter) Close() error {
	if a.extractor != nil {
		a.extractor.Delete()
	}
	if a.streamPool != nil {
		a.streamPool.Close()
	}
	return nil
}

// StreamPool manages a pool of reusable speaker streams for better performance
type StreamPool struct {
	extractor SpeakerEmbeddingExtractor
	pool      chan SpeakerStream
	maxSize   int
}

// NewStreamPool creates a new stream pool
func NewStreamPool(extractor SpeakerEmbeddingExtractor) *StreamPool {
	maxSize := 10 // Configurable pool size
	pool := make(chan SpeakerStream, maxSize)

	// Pre-populate the pool
	for i := 0; i < maxSize; i++ {
		pool <- extractor.CreateStream()
	}

	return &StreamPool{
		extractor: extractor,
		pool:      pool,
		maxSize:   maxSize,
	}
}

// Get retrieves a stream from the pool
func (p *StreamPool) Get() SpeakerStream {
	select {
	case stream := <-p.pool:
		return stream
	default:
		// Pool is empty, create a new stream
		return p.extractor.CreateStream()
	}
}

// Put returns a stream to the pool
func (p *StreamPool) Put(stream SpeakerStream) {
	select {
	case p.pool <- stream:
		// Successfully returned to pool
	default:
		// Pool is full, delete the stream
		stream.Delete()
	}
}

// Close releases all streams in the pool
func (p *StreamPool) Close() {
	close(p.pool)
	for stream := range p.pool {
		stream.Delete()
	}
}