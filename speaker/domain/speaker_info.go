package domain

import "time"

// SpeakerInfo represents read-only information about a speaker for listing purposes
type SpeakerInfo struct {
	id          SpeakerID
	name        SpeakerName
	sampleCount int
	createdAt   time.Time
	updatedAt   time.Time
}

// NewSpeakerInfo creates a new SpeakerInfo from a Speaker
func NewSpeakerInfo(speaker *Speaker) *SpeakerInfo {
	return &SpeakerInfo{
		id:          speaker.ID(),
		name:        speaker.Name(),
		sampleCount: speaker.SampleCount(),
		createdAt:   speaker.CreatedAt(),
		updatedAt:   speaker.UpdatedAt(),
	}
}

// ID returns the speaker's unique identifier
func (s *SpeakerInfo) ID() SpeakerID {
	return s.id
}

// Name returns the speaker's human-readable name
func (s *SpeakerInfo) Name() SpeakerName {
	return s.name
}

// SampleCount returns the number of samples for this speaker
func (s *SpeakerInfo) SampleCount() int {
	return s.sampleCount
}

// CreatedAt returns when the speaker was created
func (s *SpeakerInfo) CreatedAt() time.Time {
	return s.createdAt
}

// UpdatedAt returns when the speaker was last updated
func (s *SpeakerInfo) UpdatedAt() time.Time {
	return s.updatedAt
}