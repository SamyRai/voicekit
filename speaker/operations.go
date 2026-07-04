package speaker

import (
	"context"
	"sync/atomic"

	"github.com/SamyRai/voicekit/speaker/domain"
)

// RegisterSpeaker registers a new speaker with audio sample data for recognition.
//
// The speaker is identified by speakerID (must be unique) and associated with
// a human-readable speakerName. The audioData should contain clear speech
// from the speaker for embedding extraction.
//
// Parameters:
//   - speakerID: unique identifier for the speaker (e.g., "john_doe", "speaker_001")
//   - speakerName: human-readable name for the speaker (e.g., "John Doe")
//   - audioData: float32 audio samples in range [-1, 1]
//   - sampleRate: sample rate of the audio in Hz (8000-192000)
//
// Returns an error if registration fails due to invalid parameters, embedding
// extraction failure, or database/storage issues.
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (m *Manager) RegisterSpeaker(speakerID, speakerName string, audioData []float32, sampleRate int) error {
	return m.RegisterSpeakerContext(context.Background(), speakerID, speakerName, audioData, sampleRate)
}

// RegisterSpeakerContext registers a new speaker with caller-controlled cancellation.
func (m *Manager) RegisterSpeakerContext(ctx context.Context, speakerID, speakerName string, audioData []float32, sampleRate int) error {
	if ctx == nil {
		return context.Canceled
	}
	atomic.AddInt64(&m.registrationCount, 1)

	// Use clean architecture: delegate to application use case
	err := m.managementUseCase.RegisterSpeaker(
		ctx,
		domain.SpeakerID(speakerID),
		domain.SpeakerName(speakerName),
		audioData,
		sampleRate,
	)

	if err != nil {
		atomic.AddInt64(&m.errorCount, 1)
		return err
	}

	return nil
}

// IdentifySpeaker identifies the speaker in the provided audio sample by comparing
// against all registered speakers using neural network embeddings.
//
// The function extracts an embedding from the input audio and performs approximate
// nearest neighbor search against the speaker database using HNSW indexing for
// efficient similarity computation.
//
// Parameters:
//   - audioData: float32 audio samples containing speech to identify
//   - sampleRate: sample rate of the audio in Hz (8000-192000)
//
// Returns:
//   - IdentifyResult containing identification results and confidence score
//   - Error if embedding extraction fails or search encounters issues
//
// The result includes:
//   - Identified: whether a matching speaker was found above threshold
//   - SpeakerID: unique identifier of the identified speaker (if found)
//   - SpeakerName: human-readable name of the identified speaker
//   - Confidence: similarity score between 0.0 and 1.0
//   - Threshold: the confidence threshold used for identification
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (m *Manager) IdentifySpeaker(audioData []float32, sampleRate int) (*IdentifyResult, error) {
	return m.IdentifySpeakerContext(context.Background(), audioData, sampleRate)
}

// IdentifySpeakerContext identifies a speaker with caller-controlled cancellation.
func (m *Manager) IdentifySpeakerContext(ctx context.Context, audioData []float32, sampleRate int) (*IdentifyResult, error) {
	if ctx == nil {
		return nil, context.Canceled
	}
	atomic.AddInt64(&m.identifyRequests, 1)

	// Use clean architecture: delegate to application use case
	result, err := m.recognitionUseCase.IdentifySpeaker(ctx, audioData, sampleRate)
	if err != nil {
		atomic.AddInt64(&m.errorCount, 1)
		return nil, err
	}

	// Convert domain result to API format
	return &IdentifyResult{
		Identified:  result.Identified(),
		SpeakerID:   result.SpeakerID().String(),
		SpeakerName: result.SpeakerName().String(),
		Confidence:  result.Confidence().Float32(),
		Threshold:   result.Threshold().Float32(),
	}, nil
}

// VerifySpeaker verifies whether the provided audio belongs to the specified speaker.
//
// This is a 1:N verification where the speaker identity is known in advance.
// The function compares the input audio embedding against the stored embedding
// for the given speaker ID.
//
// Parameters:
//   - speakerID: unique identifier of the speaker to verify against
//   - audioData: float32 audio samples containing speech to verify
//   - sampleRate: sample rate of the audio in Hz (8000-192000)
//
// Returns:
//   - VerifyResult containing verification results and confidence score
//   - Error if speaker not found, embedding extraction fails, or other issues
//
// The result includes:
//   - Verified: whether the audio matches the speaker above threshold
//   - Confidence: similarity score between 0.0 and 1.0
//   - Threshold: the similarity threshold used for verification
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (m *Manager) VerifySpeaker(speakerID string, audioData []float32, sampleRate int) (*VerifyResult, error) {
	return m.VerifySpeakerContext(context.Background(), speakerID, audioData, sampleRate)
}

// VerifySpeakerContext verifies a speaker with caller-controlled cancellation.
func (m *Manager) VerifySpeakerContext(ctx context.Context, speakerID string, audioData []float32, sampleRate int) (*VerifyResult, error) {
	if ctx == nil {
		return nil, context.Canceled
	}
	atomic.AddInt64(&m.verifyRequests, 1)

	// Use clean architecture: delegate to application use case
	result, err := m.recognitionUseCase.VerifySpeaker(
		ctx,
		domain.SpeakerID(speakerID),
		audioData,
		sampleRate,
	)

	if err != nil {
		atomic.AddInt64(&m.errorCount, 1)
		return nil, err
	}

	// Convert domain result to API format
	verifyResult := &VerifyResult{
		SpeakerID:   result.SpeakerID().String(),
		SpeakerName: result.SpeakerName().String(),
		Verified:    result.Verified(),
		Confidence:  result.Confidence().Float32(),
		Threshold:   result.Threshold().Float32(),
	}

	if verifyResult.Verified {
		atomic.AddInt64(&m.verifySuccesses, 1)
	}

	return verifyResult, nil
}

// GetAllSpeakers returns information about all registered speakers in the system.
//
// Returns a slice of SpeakerInfo structs containing metadata for each registered
// speaker including ID, name, sample count, and registration timestamps.
//
// Returns an empty slice if no speakers are registered or if there are
// database access errors (errors are logged internally).
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (m *Manager) GetAllSpeakers() []*SpeakerInfo {
	return m.GetAllSpeakersContext(context.Background())
}

// GetAllSpeakersContext returns all speakers with caller-controlled cancellation.
func (m *Manager) GetAllSpeakersContext(ctx context.Context) []*SpeakerInfo {
	if ctx == nil {
		return []*SpeakerInfo{}
	}
	// Use clean architecture: delegate to application use case
	speakers, err := m.managementUseCase.ListSpeakers(ctx)
	if err != nil {
		m.logger.Errorf("Failed to list speakers: %v", err)
		return []*SpeakerInfo{}
	}

	// Convert domain SpeakerInfo to API SpeakerInfo
	result := make([]*SpeakerInfo, len(speakers))
	for i, speaker := range speakers {
		result[i] = &SpeakerInfo{
			ID:          speaker.ID().String(),
			Name:        speaker.Name().String(),
			SampleCount: speaker.SampleCount(),
			CreatedAt:   speaker.CreatedAt(),
			UpdatedAt:   speaker.UpdatedAt(),
		}
	}

	return result
}

// DeleteSpeaker removes a registered speaker from the system.
//
// This permanently deletes the speaker's embeddings from both memory and
// persistent storage. The speaker will no longer be available for identification
// or verification operations.
//
// Parameters:
//   - speakerID: unique identifier of the speaker to delete
//
// Returns an error if the speaker doesn't exist or if deletion fails.
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (m *Manager) DeleteSpeaker(speakerID string) error {
	return m.DeleteSpeakerContext(context.Background(), speakerID)
}

// DeleteSpeakerContext removes a speaker with caller-controlled cancellation.
func (m *Manager) DeleteSpeakerContext(ctx context.Context, speakerID string) error {
	if ctx == nil {
		return context.Canceled
	}
	// Use clean architecture: delegate to application use case
	return m.managementUseCase.DeleteSpeaker(ctx, domain.SpeakerID(speakerID))
}
