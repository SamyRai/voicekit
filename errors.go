package voicekit

import (
	"errors"
	"fmt"
)

// Common errors
var (
	ErrSpeakerNotFound            = errors.New("speaker not found")
	ErrInvalidAudioFormat         = errors.New("invalid audio format")
	ErrAudioProcessing            = errors.New("audio processing failed")
	ErrEmbeddingExtraction        = errors.New("embedding extraction failed")
	ErrDiarizationDisabled        = errors.New("diarization is disabled")
	ErrInvalidConfiguration       = errors.New("invalid configuration")
	ErrUnsupportedFormat          = errors.New("unsupported audio format")
	ErrInvalidSampleRate          = errors.New("invalid sample rate")
	ErrInvalidChannelCount        = errors.New("invalid channel count")
	ErrResamplingFailed           = errors.New("audio resampling failed")
	ErrChannelConversion          = errors.New("channel conversion failed")
	ErrNormalizationFailed        = errors.New("audio normalization failed")
	ErrInvalidSimilarityThreshold = errors.New("invalid similarity threshold")
	ErrDatabaseOperation          = errors.New("database operation failed")
)

// AudioError represents audio processing errors
type AudioError struct {
	Op     string // Operation that failed
	Format string // Audio format involved
	Err    error  // Underlying error
}

func (e *AudioError) Error() string {
	if e.Format != "" {
		return fmt.Sprintf("audio %s failed for format %s: %v", e.Op, e.Format, e.Err)
	}
	return fmt.Sprintf("audio %s failed: %v", e.Op, e.Err)
}

func (e *AudioError) Unwrap() error {
	return e.Err
}

// SpeakerError represents speaker recognition errors
type SpeakerError struct {
	Op        string // Operation that failed
	SpeakerID string // Speaker ID involved
	Err       error  // Underlying error
}

func (e *SpeakerError) Error() string {
	if e.SpeakerID != "" {
		return fmt.Sprintf("speaker %s failed for speaker %s: %v", e.Op, e.SpeakerID, e.Err)
	}
	return fmt.Sprintf("speaker %s failed: %v", e.Op, e.Err)
}

func (e *SpeakerError) Unwrap() error {
	return e.Err
}

// DiarizationError represents diarization errors
type DiarizationError struct {
	Op        string // Operation that failed
	SessionID string // Session ID involved
	Err       error  // Underlying error
}

func (e *DiarizationError) Error() string {
	if e.SessionID != "" {
		return fmt.Sprintf("diarization %s failed for session %s: %v", e.Op, e.SessionID, e.Err)
	}
	return fmt.Sprintf("diarization %s failed: %v", e.Op, e.Err)
}

func (e *DiarizationError) Unwrap() error {
	return e.Err
}

// ASRError represents ASR processing errors
type ASRError struct {
	Op        string // Operation that failed
	SessionID string // Session ID involved
	Err       error  // Underlying error
}

func (e *ASRError) Error() string {
	if e.SessionID != "" {
		return fmt.Sprintf("ASR %s failed for session %s: %v", e.Op, e.SessionID, e.Err)
	}
	return fmt.Sprintf("ASR %s failed: %v", e.Op, e.Err)
}

func (e *ASRError) Unwrap() error {
	return e.Err
}

// NewAudioError creates a new audio error
func NewAudioError(op string, err error) *AudioError {
	return &AudioError{Op: op, Err: err}
}

// NewAudioErrorWithFormat creates a new audio error with format information
func NewAudioErrorWithFormat(op, format string, err error) *AudioError {
	return &AudioError{Op: op, Format: format, Err: err}
}

// NewSpeakerError creates a new speaker error
func NewSpeakerError(op string, err error) *SpeakerError {
	return &SpeakerError{Op: op, Err: err}
}

// NewSpeakerErrorWithID creates a new speaker error with speaker ID
func NewSpeakerErrorWithID(op, speakerID string, err error) *SpeakerError {
	return &SpeakerError{Op: op, SpeakerID: speakerID, Err: err}
}

// NewDiarizationError creates a new diarization error
func NewDiarizationError(op string, err error) *DiarizationError {
	return &DiarizationError{Op: op, Err: err}
}

// NewDiarizationErrorWithSession creates a new diarization error with session ID
func NewDiarizationErrorWithSession(op, sessionID string, err error) *DiarizationError {
	return &DiarizationError{Op: op, SessionID: sessionID, Err: err}
}

// NewASRError creates a new ASR error
func NewASRError(op string, err error) *ASRError {
	return &ASRError{Op: op, Err: err}
}

// NewASRErrorWithSession creates a new ASR error with session ID
func NewASRErrorWithSession(op, sessionID string, err error) *ASRError {
	return &ASRError{Op: op, SessionID: sessionID, Err: err}
}
