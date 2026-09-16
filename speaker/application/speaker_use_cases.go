package application

import (
	"context"
	"sync"
	"time"

	"go.glpx.pro/voicekit/speaker/domain"
)

// Logger interface for application layer logging
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// MetricsRecorder interface for recording application metrics
type MetricsRecorder interface {
	IncrementCounter(name string, value int64)
	RecordLatency(name string, duration time.Duration)
}

// SpeakerRecognitionUseCase handles speaker recognition operations
type SpeakerRecognitionUseCase struct {
	speakerService domain.SpeakerService
	logger         Logger
	metrics        MetricsRecorder
}

// NewSpeakerRecognitionUseCase creates a new speaker recognition use case
func NewSpeakerRecognitionUseCase(
	speakerService domain.SpeakerService,
	logger Logger,
	metrics MetricsRecorder,
) *SpeakerRecognitionUseCase {
	return &SpeakerRecognitionUseCase{
		speakerService: speakerService,
		logger:         logger,
		metrics:        metrics,
	}
}

// IdentifySpeaker identifies a speaker from audio data
func (uc *SpeakerRecognitionUseCase) IdentifySpeaker(
	ctx context.Context,
	audioData []float32,
	sampleRate int,
) (*domain.SpeakerIdentificationResult, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		uc.metrics.RecordLatency("speaker_identification_duration", duration)
	}()

	uc.logger.Infof("Starting speaker identification for %d samples at %d Hz", len(audioData), sampleRate)

	result, err := uc.speakerService.IdentifySpeaker(ctx, audioData, sampleRate)
	if err != nil {
		uc.metrics.IncrementCounter("speaker_identification_errors", 1)
		uc.logger.Errorf("Speaker identification failed: %v", err)
		return nil, err
	}

	uc.metrics.IncrementCounter("speaker_identification_requests", 1)
	if result.Identified() {
		uc.metrics.IncrementCounter("speaker_identification_successes", 1)
		uc.logger.Infof("Successfully identified speaker %s with confidence %.4f",
			result.SpeakerID(), result.Confidence().Float32())
	} else {
		uc.logger.Infof("Speaker not identified above threshold %.4f", result.Threshold().Float32())
	}

	return result, nil
}

// VerifySpeaker verifies a speaker against provided audio data
func (uc *SpeakerRecognitionUseCase) VerifySpeaker(
	ctx context.Context,
	speakerID domain.SpeakerID,
	audioData []float32,
	sampleRate int,
) (*domain.SpeakerVerificationResult, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		uc.metrics.RecordLatency("speaker_verification_duration", duration)
	}()

	uc.logger.Infof("Starting speaker verification for speaker %s with %d samples at %d Hz",
		speakerID, len(audioData), sampleRate)

	result, err := uc.speakerService.VerifySpeaker(ctx, speakerID, audioData, sampleRate)
	if err != nil {
		uc.metrics.IncrementCounter("speaker_verification_errors", 1)
		uc.logger.Errorf("Speaker verification failed for %s: %v", speakerID, err)
		return nil, err
	}

	uc.metrics.IncrementCounter("speaker_verification_requests", 1)
	if result.Verified() {
		uc.metrics.IncrementCounter("speaker_verification_successes", 1)
		uc.logger.Infof("Successfully verified speaker %s with confidence %.4f",
			result.SpeakerID(), result.Confidence().Float32())
	} else {
		uc.logger.Infof("Speaker verification failed for %s with confidence %.4f (threshold: %.4f)",
			result.SpeakerID(), result.Confidence().Float32(), result.Threshold().Float32())
	}

	return result, nil
}

// SpeakerManagementUseCase handles speaker management operations
type SpeakerManagementUseCase struct {
	speakerService domain.SpeakerService
	logger         Logger
	metrics        MetricsRecorder
}

// NewSpeakerManagementUseCase creates a new speaker management use case
func NewSpeakerManagementUseCase(
	speakerService domain.SpeakerService,
	logger Logger,
	metrics MetricsRecorder,
) *SpeakerManagementUseCase {
	return &SpeakerManagementUseCase{
		speakerService: speakerService,
		logger:         logger,
		metrics:        metrics,
	}
}

// RegisterSpeaker registers a new speaker
func (uc *SpeakerManagementUseCase) RegisterSpeaker(
	ctx context.Context,
	speakerID domain.SpeakerID,
	speakerName domain.SpeakerName,
	audioData []float32,
	sampleRate int,
) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		uc.metrics.RecordLatency("speaker_registration_duration", duration)
	}()

	uc.logger.Infof("Registering new speaker %s (%s) with %d samples at %d Hz",
		speakerID, speakerName, len(audioData), sampleRate)

	if err := uc.speakerService.RegisterSpeaker(ctx, speakerID, speakerName, audioData, sampleRate); err != nil {
		uc.metrics.IncrementCounter("speaker_registration_errors", 1)
		uc.logger.Errorf("Speaker registration failed for %s: %v", speakerID, err)
		return err
	}

	uc.metrics.IncrementCounter("speaker_registration_successes", 1)
	uc.logger.Infof("Successfully registered speaker %s (%s)", speakerID, speakerName)

	return nil
}

// GetSpeaker retrieves speaker information
func (uc *SpeakerManagementUseCase) GetSpeaker(
	ctx context.Context,
	speakerID domain.SpeakerID,
) (*domain.SpeakerInfo, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		uc.metrics.RecordLatency("speaker_get_duration", duration)
	}()

	speaker, err := uc.speakerService.GetSpeaker(ctx, speakerID)
	if err != nil {
		uc.metrics.IncrementCounter("speaker_get_errors", 1)
		uc.logger.Errorf("Failed to get speaker %s: %v", speakerID, err)
		return nil, err
	}

	uc.metrics.IncrementCounter("speaker_get_requests", 1)
	return domain.NewSpeakerInfo(speaker), nil
}

// ListSpeakers lists all speakers
func (uc *SpeakerManagementUseCase) ListSpeakers(ctx context.Context) ([]*domain.SpeakerInfo, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		uc.metrics.RecordLatency("speaker_list_duration", duration)
	}()

	speakers, err := uc.speakerService.ListSpeakers(ctx)
	if err != nil {
		uc.metrics.IncrementCounter("speaker_list_errors", 1)
		uc.logger.Errorf("Failed to list speakers: %v", err)
		return nil, err
	}

	uc.metrics.IncrementCounter("speaker_list_requests", 1)
	uc.logger.Infof("Retrieved %d speakers", len(speakers))

	return speakers, nil
}

// DeleteSpeaker deletes a speaker
func (uc *SpeakerManagementUseCase) DeleteSpeaker(ctx context.Context, speakerID domain.SpeakerID) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		uc.metrics.RecordLatency("speaker_deletion_duration", duration)
	}()

	uc.logger.Infof("Deleting speaker %s", speakerID)

	if err := uc.speakerService.DeleteSpeaker(ctx, speakerID); err != nil {
		uc.metrics.IncrementCounter("speaker_deletion_errors", 1)
		uc.logger.Errorf("Speaker deletion failed for %s: %v", speakerID, err)
		return err
	}

	uc.metrics.IncrementCounter("speaker_deletion_successes", 1)
	uc.logger.Infof("Successfully deleted speaker %s", speakerID)

	return nil
}

// UpdateSpeakerName updates a speaker's name
func (uc *SpeakerManagementUseCase) UpdateSpeakerName(
	ctx context.Context,
	speakerID domain.SpeakerID,
	newName domain.SpeakerName,
) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		uc.metrics.RecordLatency("speaker_update_duration", duration)
	}()

	uc.logger.Infof("Updating speaker %s name to %s", speakerID, newName)

	if err := uc.speakerService.UpdateSpeakerName(ctx, speakerID, newName); err != nil {
		uc.metrics.IncrementCounter("speaker_update_errors", 1)
		uc.logger.Errorf("Speaker name update failed for %s: %v", speakerID, err)
		return err
	}

	uc.metrics.IncrementCounter("speaker_update_successes", 1)
	uc.logger.Infof("Successfully updated speaker %s name to %s", speakerID, newName)

	return nil
}

// NoOpMetricsRecorder provides a no-op implementation of MetricsRecorder
type NoOpMetricsRecorder struct{}

func (m *NoOpMetricsRecorder) IncrementCounter(name string, value int64)         {}
func (m *NoOpMetricsRecorder) RecordLatency(name string, duration time.Duration) {}

// SimpleMetricsRecorder provides a simple in-memory metrics recorder
type SimpleMetricsRecorder struct {
	mu        sync.RWMutex
	counters  map[string]int64
	latencies map[string][]time.Duration
}

func NewSimpleMetricsRecorder() *SimpleMetricsRecorder {
	return &SimpleMetricsRecorder{
		counters:  make(map[string]int64),
		latencies: make(map[string][]time.Duration),
	}
}

func (m *SimpleMetricsRecorder) IncrementCounter(name string, value int64) {
	m.mu.Lock()
	m.counters[name] += value
	m.mu.Unlock()
}

func (m *SimpleMetricsRecorder) RecordLatency(name string, duration time.Duration) {
	m.mu.Lock()
	// In a real implementation, you might want to use a more sophisticated approach
	// For now, we'll just store the last N latencies or use percentiles
	m.latencies[name] = append(m.latencies[name], duration)
	// Keep only last 1000 measurements
	if len(m.latencies[name]) > 1000 {
		m.latencies[name] = m.latencies[name][1:]
	}
	m.mu.Unlock()
}

func (m *SimpleMetricsRecorder) GetCounter(name string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.counters[name]
}

func (m *SimpleMetricsRecorder) GetLatencies(name string) []time.Duration {
	m.mu.RLock()
	latencies := make([]time.Duration, len(m.latencies[name]))
	copy(latencies, m.latencies[name])
	m.mu.RUnlock()
	return latencies
}
