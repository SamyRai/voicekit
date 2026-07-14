package types

import "context"

// Punctuation restores punctuation and casing in raw ASR transcript text. It is
// a post-ASR normalizer kept separate from ASRService so callers can compose it
// only when they need it.
type Punctuation interface {
	Restore(ctx context.Context, text string) (string, error)
	Close() error
}

// KeywordSpotter detects configured keywords or wake-words in streaming audio.
// A sessionID scopes the per-stream detection state, mirroring ASRService so a
// single spotter can serve concurrent streams.
type KeywordSpotter interface {
	// Spot feeds an audio chunk for a session and returns a keyword detected on
	// this step, or nil when none fired.
	Spot(ctx context.Context, sessionID string, audio []float32) (*KeywordMatch, error)
	Close() error
}

// KeywordMatch is a keyword detected in a stream.
type KeywordMatch struct {
	Keyword string  `json:"keyword"`
	Score   float64 `json:"score"`
}

// LanguageIdentifier identifies the spoken language of a complete audio segment.
type LanguageIdentifier interface {
	Identify(ctx context.Context, audio []float32, sampleRate int) (*LanguageResult, error)
	Close() error
}

// LanguageResult is the detected spoken language of an audio segment.
type LanguageResult struct {
	Language string  `json:"language"`
	Score    float64 `json:"score"`
}
