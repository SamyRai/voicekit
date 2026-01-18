package asr

import (
	"context"
	"fmt"
	"time"

	"github.com/SamyRai/voicekit/types"
)

// WhisperModel implements the Model interface for Whisper
type WhisperModel struct {
	name         string
	language     string
	quantization string
	latency      time.Duration
}

// NewWhisperModel creates a new Whisper model
func NewWhisperModel(name, language, quantization string) *WhisperModel {
	return &WhisperModel{
		name:         name,
		language:     language,
		quantization: quantization,
		latency:      500 * time.Millisecond, // Typical latency
	}
}

// Name returns the model name
func (m *WhisperModel) Name() string {
	return m.name
}

// Language returns the supported language
func (m *WhisperModel) Language() string {
	return m.language
}

// Quantization returns the quantization level
func (m *WhisperModel) Quantization() string {
	return m.quantization
}

// ProcessAudio processes audio and returns transcription
func (m *WhisperModel) ProcessAudio(ctx context.Context, audio []float32, state *types.StreamingState) (*types.Transcription, error) {
	// Simulate processing time
	select {
	case <-time.After(m.latency):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Generate mock transcription based on audio length
	text := fmt.Sprintf("Transcribed %d samples in %s", len(audio), m.language)
	if len(audio) > 1000 {
		text = "This is a sample transcription of the audio input."
	}

	return &types.Transcription{
		Text:       text,
		IsPartial:  true, // For streaming, usually partial
		Confidence: 0.85,
		Language:   m.language,
		Timestamp:  time.Now(),
		StartTime:  0,
		EndTime:    time.Duration(len(audio)/16) * time.Millisecond, // Rough estimate
	}, nil
}

// SupportsLanguage checks if the model supports a language
func (m *WhisperModel) SupportsLanguage(lang string) bool {
	return m.language == lang || m.language == "all" || lang == "en"
}

// Latency returns expected latency for this model
func (m *WhisperModel) Latency() time.Duration {
	return m.latency
}

// Close releases model resources
func (m *WhisperModel) Close() error {
	// Cleanup model resources
	return nil
}