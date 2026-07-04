package asr

import (
	"context"
	"time"

	"github.com/SamyRai/voicekit/types"
)

// FakeModel implements Model for tests and demos. It is never registered by default.
type FakeModel struct {
	name         string
	language     string
	quantization string
}

// NewFakeModel creates an explicit fake model for tests and demos.
func NewFakeModel(name, language, quantization string) *FakeModel {
	return &FakeModel{
		name:         name,
		language:     language,
		quantization: quantization,
	}
}

func (m *FakeModel) Name() string {
	return m.name
}

func (m *FakeModel) Language() string {
	return m.language
}

func (m *FakeModel) Quantization() string {
	return m.quantization
}

func (m *FakeModel) ProcessAudio(ctx context.Context, audio []float32, state *types.StreamingState) (*types.Transcription, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return &types.Transcription{
		Text:       "",
		IsPartial:  true,
		Confidence: 0,
		Language:   m.language,
		Timestamp:  time.Now(),
		StartTime:  0,
		EndTime:    0,
	}, nil
}

func (m *FakeModel) SupportsLanguage(lang string) bool {
	return m.language == lang || m.language == "all" || lang == "en"
}

func (m *FakeModel) Latency() time.Duration {
	return 0
}

func (m *FakeModel) Close() error {
	return nil
}
