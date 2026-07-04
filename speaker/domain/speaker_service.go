package domain

import (
	"context"
	"fmt"
)

// SpeakerService handles core speaker recognition business logic
type SpeakerService struct {
	repository           SpeakerRepository
	embeddingExtractor   EmbeddingExtractor
	vectorIndex          VectorIndex
	similarityCalculator SimilarityCalculator
	similarityThreshold  SimilarityScore
}

// NewSpeakerService creates a new speaker service
func NewSpeakerService(
	repository SpeakerRepository,
	embeddingExtractor EmbeddingExtractor,
	vectorIndex VectorIndex,
	similarityCalculator SimilarityCalculator,
	similarityThreshold SimilarityScore,
) *SpeakerService {
	return &SpeakerService{
		repository:           repository,
		embeddingExtractor:   embeddingExtractor,
		vectorIndex:          vectorIndex,
		similarityCalculator: similarityCalculator,
		similarityThreshold:  similarityThreshold,
	}
}

// RegisterSpeaker registers a new speaker with audio data
func (s *SpeakerService) RegisterSpeaker(
	ctx context.Context,
	speakerID SpeakerID,
	speakerName SpeakerName,
	audioData []float32,
	sampleRate int,
) error {
	// Validate input
	if speakerID == "" {
		return ErrInvalidSpeakerID
	}
	if speakerName == "" {
		return ErrInvalidSpeakerName
	}
	if len(audioData) == 0 {
		return ErrInsufficientAudio
	}
	if sampleRate <= 0 {
		return fmt.Errorf("invalid sample rate: %d", sampleRate)
	}

	// Check if speaker already exists
	exists, err := s.repository.Exists(ctx, speakerID)
	if err != nil {
		return fmt.Errorf("failed to check speaker existence: %w", err)
	}
	if exists {
		return ErrSpeakerAlreadyExists
	}

	// Extract embedding from audio
	embedding, err := s.embeddingExtractor.ExtractEmbedding(ctx, audioData, sampleRate)
	if err != nil {
		return fmt.Errorf("embedding extraction failed: %w", err)
	}

	// Create new speaker
	speaker := NewSpeaker(speakerID, speakerName, embedding)

	// Save to repository
	if err := s.repository.Save(ctx, speaker); err != nil {
		return fmt.Errorf("failed to save speaker: %w", err)
	}

	// Add to vector index for fast search
	if err := s.vectorIndex.AddSpeaker(ctx, speaker); err != nil {
		// Note: We don't fail the registration if indexing fails,
		// but this should be logged as it affects search performance
		return fmt.Errorf("failed to index speaker: %w", err)
	}

	return nil
}

// IdentifySpeaker identifies the speaker in the provided audio
func (s *SpeakerService) IdentifySpeaker(
	ctx context.Context,
	audioData []float32,
	sampleRate int,
) (*SpeakerIdentificationResult, error) {
	// Validate input
	if len(audioData) == 0 {
		return nil, ErrInsufficientAudio
	}
	if sampleRate <= 0 {
		return nil, fmt.Errorf("invalid sample rate: %d", sampleRate)
	}

	// Extract embedding from audio
	embedding, err := s.embeddingExtractor.ExtractEmbedding(ctx, audioData, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("embedding extraction failed: %w", err)
	}

	// Search for similar speaker using vector index
	speakerID, confidence, err := s.vectorIndex.SearchSimilar(ctx, embedding, s.similarityThreshold)
	if err != nil {
		return nil, fmt.Errorf("similarity search failed: %w", err)
	}

	// If no speaker found above threshold
	if speakerID == "" {
		return NewSpeakerIdentificationResult(
			false,
			"",
			"",
			MinSimilarityScore,
			s.similarityThreshold,
		), nil
	}

	// Get speaker details from repository
	speaker, err := s.repository.FindByID(ctx, speakerID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve speaker details: %w", err)
	}

	return NewSpeakerIdentificationResult(
		true,
		speaker.ID(),
		speaker.Name(),
		confidence,
		s.similarityThreshold,
	), nil
}

// VerifySpeaker verifies if the audio belongs to the specified speaker
func (s *SpeakerService) VerifySpeaker(
	ctx context.Context,
	speakerID SpeakerID,
	audioData []float32,
	sampleRate int,
) (*SpeakerVerificationResult, error) {
	// Validate input
	if speakerID == "" {
		return nil, ErrInvalidSpeakerID
	}
	if len(audioData) == 0 {
		return nil, ErrInsufficientAudio
	}
	if sampleRate <= 0 {
		return nil, fmt.Errorf("invalid sample rate: %d", sampleRate)
	}

	// Check if speaker exists
	speaker, err := s.repository.FindByID(ctx, speakerID)
	if err != nil {
		return nil, fmt.Errorf("speaker lookup failed: %w", err)
	}

	// Extract embedding from audio
	embedding, err := s.embeddingExtractor.ExtractEmbedding(ctx, audioData, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("embedding extraction failed: %w", err)
	}

	// Calculate similarity with stored embeddings
	confidence := s.similarityCalculator.CalculateSimilarity(embedding, speaker.Embeddings())
	verified := confidence.IsAboveThreshold(s.similarityThreshold)

	return NewSpeakerVerificationResult(
		speaker.ID(),
		speaker.Name(),
		verified,
		confidence,
		s.similarityThreshold,
	), nil
}

// GetSpeaker retrieves speaker information
func (s *SpeakerService) GetSpeaker(ctx context.Context, speakerID SpeakerID) (*Speaker, error) {
	if speakerID == "" {
		return nil, ErrInvalidSpeakerID
	}

	speaker, err := s.repository.FindByID(ctx, speakerID)
	if err != nil {
		return nil, fmt.Errorf("speaker lookup failed: %w", err)
	}

	return speaker, nil
}

// ListSpeakers returns information about all speakers
func (s *SpeakerService) ListSpeakers(ctx context.Context) ([]*SpeakerInfo, error) {
	speakers, err := s.repository.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list speakers: %w", err)
	}

	speakerInfos := make([]*SpeakerInfo, len(speakers))
	for i, speaker := range speakers {
		speakerInfos[i] = NewSpeakerInfo(speaker)
	}

	return speakerInfos, nil
}

// DeleteSpeaker removes a speaker from the system
func (s *SpeakerService) DeleteSpeaker(ctx context.Context, speakerID SpeakerID) error {
	if speakerID == "" {
		return ErrInvalidSpeakerID
	}

	// Check if speaker exists
	exists, err := s.repository.Exists(ctx, speakerID)
	if err != nil {
		return fmt.Errorf("failed to check speaker existence: %w", err)
	}
	if !exists {
		return ErrSpeakerNotFound
	}

	var indexErr error
	if err := s.vectorIndex.RemoveSpeaker(ctx, speakerID); err != nil {
		indexErr = fmt.Errorf("failed to remove speaker from index: %w", err)
	}

	// Remove from repository
	if err := s.repository.Delete(ctx, speakerID); err != nil {
		return fmt.Errorf("failed to delete speaker: %w", err)
	}
	if indexErr != nil {
		return indexErr
	}

	return nil
}

// UpdateSpeakerName updates a speaker's name
func (s *SpeakerService) UpdateSpeakerName(
	ctx context.Context,
	speakerID SpeakerID,
	newName SpeakerName,
) error {
	if speakerID == "" {
		return ErrInvalidSpeakerID
	}
	if newName == "" {
		return ErrInvalidSpeakerName
	}

	// Get speaker
	speaker, err := s.repository.FindByID(ctx, speakerID)
	if err != nil {
		return fmt.Errorf("speaker lookup failed: %w", err)
	}

	// Update name
	speaker.UpdateName(newName)

	// Save changes
	if err := s.repository.Save(ctx, speaker); err != nil {
		return fmt.Errorf("failed to save speaker: %w", err)
	}

	return nil
}
