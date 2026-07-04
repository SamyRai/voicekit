package domain

import (
	"time"
)

// Speaker represents a speaker in the domain
type Speaker struct {
	id          SpeakerID
	name        SpeakerName
	embeddings  []SpeakerEmbedding
	sampleCount int
	createdAt   time.Time
	updatedAt   time.Time
}

// SpeakerID represents a unique speaker identifier
type SpeakerID string

func NewSpeakerID(id string) (SpeakerID, error) {
	if id == "" {
		return "", ErrInvalidSpeakerID
	}
	return SpeakerID(id), nil
}

func (id SpeakerID) String() string {
	return string(id)
}

// SpeakerName represents a human-readable speaker name
type SpeakerName string

func NewSpeakerName(name string) (SpeakerName, error) {
	if name == "" {
		return "", ErrInvalidSpeakerName
	}
	return SpeakerName(name), nil
}

func (name SpeakerName) String() string {
	return string(name)
}

// SpeakerEmbedding represents a speaker embedding vector
type SpeakerEmbedding struct {
	vector []float32
}

func NewSpeakerEmbedding(vector []float32) (SpeakerEmbedding, error) {
	if len(vector) == 0 {
		return SpeakerEmbedding{}, ErrInvalidEmbedding
	}
	// Create a copy to prevent external modifications
	embedding := make([]float32, len(vector))
	copy(embedding, vector)
	return SpeakerEmbedding{vector: embedding}, nil
}

func (e SpeakerEmbedding) Vector() []float32 {
	// Return a copy to prevent external modifications
	result := make([]float32, len(e.vector))
	copy(result, e.vector)
	return result
}

func (e SpeakerEmbedding) Dimension() int {
	return len(e.vector)
}

// NewSpeaker creates a new speaker with initial embedding
func NewSpeaker(id SpeakerID, name SpeakerName, initialEmbedding SpeakerEmbedding) *Speaker {
	now := time.Now()
	return &Speaker{
		id:          id,
		name:        name,
		embeddings:  []SpeakerEmbedding{initialEmbedding},
		sampleCount: 1,
		createdAt:   now,
		updatedAt:   now,
	}
}

// ID returns the speaker's unique identifier
func (s *Speaker) ID() SpeakerID {
	return s.id
}

// Name returns the speaker's human-readable name
func (s *Speaker) Name() SpeakerName {
	return s.name
}

// Embeddings returns all embeddings for this speaker
func (s *Speaker) Embeddings() []SpeakerEmbedding {
	// Return a copy to prevent external modifications
	result := make([]SpeakerEmbedding, len(s.embeddings))
	copy(result, s.embeddings)
	return result
}

// SampleCount returns the number of samples for this speaker
func (s *Speaker) SampleCount() int {
	return s.sampleCount
}

// CreatedAt returns when the speaker was created
func (s *Speaker) CreatedAt() time.Time {
	return s.createdAt
}

// UpdatedAt returns when the speaker was last updated
func (s *Speaker) UpdatedAt() time.Time {
	return s.updatedAt
}

// AddEmbedding adds a new embedding to the speaker
func (s *Speaker) AddEmbedding(embedding SpeakerEmbedding) {
	s.embeddings = append(s.embeddings, embedding)
	s.sampleCount++
	s.updatedAt = time.Now()
}

// UpdateName updates the speaker's name
func (s *Speaker) UpdateName(name SpeakerName) {
	s.name = name
	s.updatedAt = time.Now()
}
