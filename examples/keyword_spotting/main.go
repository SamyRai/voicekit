// Command keyword_spotting demonstrates streaming wake-word/hotword
// detection with asr.SherpaKeywordSpotter. It requires real Sherpa ONNX
// keyword-spotting model files, so it is gated on environment variables and
// prints setup instructions instead of failing when they are unset.
package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"go.glpx.pro/voicekit/asr"
)

func main() {
	fmt.Println("VoiceKit Keyword Spotting Example")
	fmt.Println("==================================")

	config, ok := keywordSpotterConfigFromEnv()
	if !ok {
		fmt.Println("Sherpa keyword spotting model paths are not configured.")
		fmt.Println("Set VOICEKIT_KWS_TOKENS plus either:")
		fmt.Println("  - VOICEKIT_KWS_ENCODER, VOICEKIT_KWS_DECODER, VOICEKIT_KWS_JOINER (transducer), or")
		fmt.Println("  - VOICEKIT_KWS_MODEL (+ optional VOICEKIT_KWS_MODEL_TYPE) for a single-file model")
		fmt.Println("and set either VOICEKIT_KWS_KEYWORDS_FILE or VOICEKIT_KWS_KEYWORDS (comma-separated).")
		return
	}

	spotter, err := asr.NewSherpaKeywordSpotter(&config)
	if err != nil {
		log.Fatalf("failed to create keyword spotter: %v", err)
	}
	defer spotter.Close()

	fmt.Println("Keyword spotter initialized.")
	fmt.Println()

	demonstrateKeywordSpotting(spotter, config.SampleRate)

	fmt.Println()
	fmt.Println("Keyword spotting demonstration completed.")
}

func demonstrateKeywordSpotting(spotter *asr.SherpaKeywordSpotter, sampleRate int) {
	sessionID := "demo-session-1"
	ctx := context.Background()

	totalChunks := 5
	fmt.Printf("Processing %d audio chunks (1s each):\n", totalChunks)
	fmt.Println()

	for i := 0; i < totalChunks; i++ {
		audioChunk := generateSyntheticAudioChunk(i, sampleRate)

		startTime := time.Now()
		match, err := spotter.Spot(ctx, sessionID, audioChunk)
		processTime := time.Since(startTime)

		if err != nil {
			log.Printf("keyword spotting error on chunk %d: %v", i+1, err)
			continue
		}

		fmt.Printf("Chunk %d/%d: ", i+1, totalChunks)
		if match == nil {
			fmt.Printf("no keyword detected (%v)\n", processTime.Round(time.Millisecond))
		} else {
			fmt.Printf("keyword %q detected (%v)\n", match.Keyword, processTime.Round(time.Millisecond))
		}

		time.Sleep(100 * time.Millisecond)
	}

	// End the session to release its native decode stream. Callers that use a
	// fresh sessionID per utterance should EndSession when done so streams do
	// not accumulate for the life of the spotter.
	if err := spotter.EndSession(sessionID); err != nil {
		log.Printf("end session error: %v", err)
	}

	fmt.Println()
	fmt.Println("Notes:")
	fmt.Println("- Detection depends entirely on the configured Sherpa model and keyword list")
	fmt.Println("- Score is always 0: the Sherpa KeywordSpotterResult binding exposes only the matched text")
	fmt.Println("- Call EndSession(sessionID) to free a session's native stream; Close() frees all")
}

func keywordSpotterConfigFromEnv() (asr.KeywordSpotterConfig, bool) {
	config := asr.DefaultKeywordSpotterConfig()
	config.TokensPath = os.Getenv("VOICEKIT_KWS_TOKENS")
	config.EncoderPath = os.Getenv("VOICEKIT_KWS_ENCODER")
	config.DecoderPath = os.Getenv("VOICEKIT_KWS_DECODER")
	config.JoinerPath = os.Getenv("VOICEKIT_KWS_JOINER")
	config.ModelPath = os.Getenv("VOICEKIT_KWS_MODEL")
	config.ModelType = os.Getenv("VOICEKIT_KWS_MODEL_TYPE")
	config.KeywordsFile = os.Getenv("VOICEKIT_KWS_KEYWORDS_FILE")
	config.Provider = envString("VOICEKIT_KWS_PROVIDER", config.Provider)
	config.NumThreads = envInt("VOICEKIT_KWS_THREADS", config.NumThreads)

	if raw := os.Getenv("VOICEKIT_KWS_KEYWORDS"); raw != "" {
		for _, keyword := range strings.Split(raw, ",") {
			keyword = strings.TrimSpace(keyword)
			if keyword != "" {
				config.Keywords = append(config.Keywords, keyword)
			}
		}
	}

	hasTokens := config.TokensPath != ""
	hasModel := (config.EncoderPath != "" && config.DecoderPath != "" && config.JoinerPath != "") || config.ModelPath != ""
	hasKeywords := config.KeywordsFile != "" || len(config.Keywords) > 0

	return config, hasTokens && hasModel && hasKeywords
}

func generateSyntheticAudioChunk(chunkIndex int, sampleRate int) []float32 {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	chunk := make([]float32, sampleRate)

	switch chunkIndex {
	case 0:
		// Silent chunk (no speech).
	case 1:
		generateSpeechLikeAudio(chunk, sampleRate, 0.3)
	case 2:
		generateSpeechLikeAudio(chunk, sampleRate, 0.7)
	case 3:
		half := len(chunk) / 2
		generateSpeechLikeAudio(chunk[:half], sampleRate, 0.6)
	default:
		generateSpeechLikeAudio(chunk, sampleRate, 0.5)
	}

	return chunk
}

func generateSpeechLikeAudio(audio []float32, sampleRate int, energy float32) {
	frequencies := []float64{85, 170, 255, 340}

	for i := range audio {
		t := float64(i) / float64(sampleRate)
		sample := 0.0
		for _, freq := range frequencies {
			sample += math.Sin(2 * math.Pi * freq * t)
		}
		sample /= float64(len(frequencies))
		audio[i] = float32(sample) * energy
	}
}

func envString(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
