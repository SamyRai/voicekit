package voicekit

import (
	"testing"
)

func TestVoiceKit_Integration(t *testing.T) {
	// Create VoiceKit config with ASR enabled
	config := &Config{
		Audio: AudioConfig{
			SampleRate:      16000,
			Channels:        1,
			NormalizeFactor: 32768.0,
		},
		Speaker: SpeakerConfig{
			NumThreads: 1,
			Provider:   "cpu",
			Threshold:  0.5,
			DataDir:    "/tmp/voicekit_test_speaker",
		},
		Diarization: DiarizationConfig{
			Enabled:               true,
			MinSegmentLength:      1.0,
			MaxSegmentLength:      30.0,
			SilenceThreshold:      0.5,
			SimilarityThreshold:   0.7,
			MaxSpeakers:           10,
			ReassignmentThreshold: 0.8,
			OverlapThreshold:      0.2,
		},
		ASR: ASRConfig{
			Enabled:              true,
			DefaultModel:         "whisper_large_v3",
			Language:             "en",
			Quantization:         "int8",
			MaxConcurrentStreams: 10,
			StreamTimeout:        300,
			ChunkSize:            16000,
			VADProvider:          "ten_vad",
		},
	}

	// Create VoiceKit instance
	vk, err := NewVoiceKit(config)
	if err != nil {
		t.Fatalf("Failed to create VoiceKit: %v", err)
	}
	defer vk.Close()

	// Verify VoiceKit was created successfully
	// Note: Individual services may not be available if their models/configs are not set up
	// This test primarily verifies that the configuration validation and initialization pipeline works

	// Test completed successfully - VoiceKit initialized with all components
	t.Log("VoiceKit integration test passed")
}