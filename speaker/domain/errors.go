package domain

import "errors"

// Domain errors
var (
	ErrInvalidSpeakerID     = errors.New("invalid speaker ID: cannot be empty")
	ErrInvalidSpeakerName   = errors.New("invalid speaker name: cannot be empty")
	ErrInvalidEmbedding     = errors.New("invalid embedding: cannot be empty")
	ErrSpeakerNotFound      = errors.New("speaker not found")
	ErrSpeakerAlreadyExists = errors.New("speaker already exists")
	ErrInsufficientAudio    = errors.New("insufficient audio data for embedding extraction")
	ErrEmbeddingExtraction  = errors.New("embedding extraction failed")
)