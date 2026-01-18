package asr

import (
	"voicekit/types"
)

// VADService provides voice activity detection
type VADService struct {
	config *VADConfig
}

// VADConfig holds VAD configuration
type VADConfig struct {
	Provider   string  `json:"provider"`
	Threshold  float64 `json:"threshold"`
	WindowSize int     `json:"window_size"`
	SampleRate int     `json:"sample_rate"`
}

// Use VADResult from types package

// NewVADService creates a new VAD service
func NewVADService(config *VADConfig) (*VADService, error) {
	if config == nil {
		config = &VADConfig{
			Provider:   "ten_vad",
			Threshold:  0.5,
			WindowSize: 512,
			SampleRate: 16000,
		}
	}

	return &VADService{
		config: config,
	}, nil
}

// Process processes audio for voice activity detection
func (v *VADService) Process(audio []float32, state interface{}) (*types.VADResult, error) {
	if len(audio) == 0 {
		return &types.VADResult{
			IsSpeech:   false,
			IsEndpoint: false,
			Confidence: 0.0,
			State:      state,
		}, nil
	}

	// Simple energy-based VAD (placeholder for actual VAD integration)
	result := &types.VADResult{
		IsSpeech:   v.detectSpeech(audio),
		IsEndpoint: false, // Would be determined by actual VAD
		Confidence: 0.8,
		State:      state,
	}

	return result, nil
}

// detectSpeech performs basic speech detection
func (v *VADService) detectSpeech(audio []float32) bool {
	if len(audio) == 0 {
		return false
	}

	// Calculate RMS energy
	var sum float32
	for _, sample := range audio {
		sum += sample * sample
	}
	rms := sum / float32(len(audio))

	return rms > float32(v.config.Threshold)
}

// Close closes the VAD service
func (v *VADService) Close() error {
	// Cleanup VAD resources
	return nil
}
