package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"go.glpx.pro/gdk/voice"
)

func main() {
	fmt.Println("🎤 VoiceKit ASR Streaming Example")
	fmt.Println("==================================")

	tokensPath := os.Getenv("VOICEKIT_ASR_TOKENS")
	encoderPath := os.Getenv("VOICEKIT_ASR_ENCODER")
	decoderPath := os.Getenv("VOICEKIT_ASR_DECODER")
	joinerPath := os.Getenv("VOICEKIT_ASR_JOINER")
	if tokensPath == "" || encoderPath == "" || decoderPath == "" || joinerPath == "" {
		fmt.Println("Sherpa ASR model paths are not configured.")
		fmt.Println("Set VOICEKIT_ASR_TOKENS, VOICEKIT_ASR_ENCODER, VOICEKIT_ASR_DECODER, and VOICEKIT_ASR_JOINER to run this example.")
		return
	}

	config := &voicekit.Config{
		Audio: voicekit.AudioConfig{
			SampleRate:      16000,
			Channels:        1,
			NormalizeFactor: 32768.0,
		},
		ASR: voicekit.ASRConfig{
			Enabled:              true,
			Backend:              "sherpa_online",
			DefaultModel:         "sherpa_online",
			Language:             "en",
			Quantization:         "float32",
			MaxConcurrentStreams: 10,
			StreamTimeout:        300,   // 5 minutes
			ChunkSize:            16000, // 1 second chunks
			Online: voicekit.OnlineConfig{
				TokensPath:  tokensPath,
				EncoderPath: encoderPath,
				DecoderPath: decoderPath,
				JoinerPath:  joinerPath,
			},
			VADProvider: "none",
		},
	}

	// Create VoiceKit instance
	vk, err := voicekit.NewVoiceKit(config)
	if err != nil {
		log.Fatalf("Failed to create VoiceKit: %v", err)
	}
	defer vk.Close()

	// Get ASR service
	asrService := vk.ASR()
	if asrService == nil {
		log.Fatal("ASR service not available")
	}

	fmt.Println("✅ VoiceKit initialized with ASR streaming support")
	fmt.Println()

	// Demonstrate ASR streaming
	demonstrateASRStreaming(asrService)

	fmt.Println()
	fmt.Println("🎉 ASR streaming demonstration completed!")
}

func demonstrateASRStreaming(asrService voicekit.ASRService) {
	fmt.Println("🎵 Starting ASR streaming session...")

	sessionID := "demo-session-1"
	ctx := context.Background()

	// Simulate streaming audio chunks (representing ~5 seconds of audio)
	totalChunks := 5
	chunkDuration := time.Second

	fmt.Printf("Processing %d audio chunks (%v each):\n", totalChunks, chunkDuration)
	fmt.Println()

	for i := 0; i < totalChunks; i++ {
		// Generate synthetic audio chunk (simulate speech)
		audioChunk := generateSyntheticAudioChunk(i)

		// Process chunk through ASR
		startTime := time.Now()
		transcription, err := asrService.ProcessAudioChunk(ctx, sessionID, audioChunk)
		processTime := time.Since(startTime)

		if err != nil {
			log.Printf("❌ ASR processing error on chunk %d: %v", i+1, err)
			continue
		}

		// Display results
		fmt.Printf("Chunk %d/%d: ", i+1, totalChunks)

		if transcription == nil {
			fmt.Printf("No speech detected\n")
		} else {
			status := "Partial"
			if !transcription.IsPartial {
				status = "Final"
			}

			fmt.Printf("[%s] \"%s\" (confidence: %.2f, lang: %s, %v)\n",
				status,
				transcription.Text,
				transcription.Confidence,
				transcription.Language,
				processTime.Round(time.Millisecond))
		}

		// Simulate real-time streaming delay
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println()
	fmt.Println("📊 ASR Streaming Performance Summary:")
	fmt.Println("- Results depend on the configured Sherpa model and runtime provider")
	fmt.Println("- Confidence is 0 when the Sherpa binding does not expose calibrated confidence")
	fmt.Println("- VAD is disabled in this example unless configured separately")
}

func generateSyntheticAudioChunk(chunkIndex int) []float32 {
	// Generate a 1-second audio chunk (16kHz = 16000 samples)
	// This simulates different types of audio content

	const sampleRate = 16000
	chunk := make([]float32, sampleRate)

	switch chunkIndex {
	case 0:
		// Silent chunk (no speech)
		// All samples remain 0

	case 1:
		// Speech-like chunk with some silence
		generateSpeechLikeAudio(chunk, 0.3)

	case 2:
		// Clear speech chunk
		generateSpeechLikeAudio(chunk, 0.7)

	case 3:
		// Mixed speech with pauses
		half := len(chunk) / 2
		generateSpeechLikeAudio(chunk[:half], 0.6)
		// Second half remains silent

	case 4:
		// Final chunk with speech
		generateSpeechLikeAudio(chunk, 0.8)

	default:
		// Fallback: some speech
		generateSpeechLikeAudio(chunk, 0.5)
	}

	return chunk
}

func generateSpeechLikeAudio(audio []float32, energy float32) {
	// Generate speech-like audio using multiple frequency components
	// This creates a signal that VAD should detect as speech

	frequencies := []float64{85, 170, 255, 340} // Fundamental + harmonics
	phase := 0.0

	for i := range audio {
		t := float64(i) / 16000.0 // Time in seconds

		// Sum multiple sine waves for speech-like spectrum
		sample := 0.0
		for _, freq := range frequencies {
			sample += math.Sin(2*math.Pi*freq*t + phase)
		}
		sample /= float64(len(frequencies)) // Normalize

		// Apply energy level and add some noise
		audio[i] = float32(sample)*energy + (float32(sample) * 0.1 * energy)
	}
}
