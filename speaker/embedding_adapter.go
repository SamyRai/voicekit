package speaker

import (
	"context"
	"fmt"

	"github.com/SamyRai/voicekit/speaker/domain"
)

// SherpaEmbeddingExtractorAdapter adapts the existing Sherpa embedding extractor to the domain interface
type SherpaEmbeddingExtractorAdapter struct {
	extractor  SpeakerEmbeddingExtractor // This will be the existing interface
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
	if ctx == nil {
		return domain.SpeakerEmbedding{}, context.Canceled
	}
	if a == nil || a.extractor == nil || a.streamPool == nil {
		return domain.SpeakerEmbedding{}, domain.ErrEmbeddingExtraction
	}
	if len(audioData) == 0 {
		return domain.SpeakerEmbedding{}, domain.ErrInsufficientAudio
	}
	if sampleRate <= 0 {
		return domain.SpeakerEmbedding{}, fmt.Errorf("invalid sample rate: %d", sampleRate)
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return domain.SpeakerEmbedding{}, ctx.Err()
	default:
	}

	// Get a stream from the pool
	stream := a.streamPool.Get()
	if stream == nil {
		return domain.SpeakerEmbedding{}, domain.ErrEmbeddingExtraction
	}
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
	if a == nil || a.extractor == nil {
		return 0
	}
	return a.extractor.Dim()
}

// IsReady checks if the extractor is ready for use
func (a *SherpaEmbeddingExtractorAdapter) IsReady() bool {
	// For Sherpa, we assume it's ready if it was successfully created
	return a != nil && a.extractor != nil
}

// Close releases resources used by the extractor
func (a *SherpaEmbeddingExtractorAdapter) Close() error {
	if a.streamPool != nil {
		a.streamPool.Close()
		a.streamPool = nil
	}
	if a.extractor != nil {
		a.extractor.Delete()
		a.extractor = nil
	}
	return nil
}

// StreamPool owns speaker streams created for extraction.
//
// Sherpa streams cannot be reused after InputFinished, so Get creates a fresh
// stream and Put deletes it. The type is kept to preserve the existing adapter
// boundary while making native stream ownership explicit.
type StreamPool struct {
	extractor SpeakerEmbeddingExtractor
}

// NewStreamPool creates a new stream pool
func NewStreamPool(extractor SpeakerEmbeddingExtractor) *StreamPool {
	return &StreamPool{
		extractor: extractor,
	}
}

// Get retrieves a stream from the pool
func (p *StreamPool) Get() SpeakerStream {
	if p == nil || p.extractor == nil {
		return nil
	}
	return p.extractor.CreateStream()
}

// Put releases a stream after extraction.
func (p *StreamPool) Put(stream SpeakerStream) {
	if stream != nil {
		stream.Delete()
	}
}

// Close releases all streams in the pool
func (p *StreamPool) Close() {
}
