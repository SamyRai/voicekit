// Command tts_streaming demonstrates tts.SherpaStreamingSynthesizer:
// incremental TTS that delivers audio chunks to a sink as they are produced,
// instead of waiting for the whole utterance to finish synthesizing.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/SamyRai/voicekit/tts"
	"github.com/SamyRai/voicekit/types"
)

func main() {
	text := os.Getenv("VOICEKIT_TTS_TEXT")
	if text == "" {
		text = "VoiceKit streaming text to speech delivers audio chunk by chunk."
	}

	config, ok := streamingTTSConfigFromEnv()
	if !ok {
		fmt.Println("Sherpa offline TTS model paths are not configured.")
		fmt.Println("Set VOICEKIT_TTS_FAMILY plus the required VOICEKIT_TTS_* model path variables.")
		return
	}

	synth, err := tts.NewSherpaStreamingSynthesizer(config)
	if err != nil {
		log.Fatalf("failed to create streaming TTS synthesizer: %v", err)
	}
	defer func() {
		if err := synth.Close(); err != nil {
			log.Printf("failed to close streaming TTS synthesizer: %v", err)
		}
	}()

	// maxChunks demonstrates barge-in: returning false from the sink stops
	// synthesis early. 0 (the default) means "no limit" - deliver every chunk.
	maxChunks := envInt("VOICEKIT_TTS_MAX_CHUNKS", 0)

	chunkCount := 0
	sampleCount := 0
	result, err := synth.SynthesizeStream(context.Background(), types.SynthesisRequest{
		Text:      text,
		SpeakerID: envInt("VOICEKIT_TTS_SPEAKER_ID", config.SpeakerID),
		Speed:     envFloat32("VOICEKIT_TTS_SPEED", config.Speed),
	}, func(chunk types.AudioChunk) bool {
		chunkCount++
		sampleCount += len(chunk.Samples)
		fmt.Printf("chunk %d: %d samples at %d Hz (progress %.0f%%)\n",
			chunkCount, len(chunk.Samples), chunk.SampleRate, chunk.Progress*100)
		if maxChunks > 0 && chunkCount >= maxChunks {
			fmt.Println("stopping early (VOICEKIT_TTS_MAX_CHUNKS reached, simulating barge-in)")
			return false
		}
		return true
	})
	if err != nil {
		log.Fatalf("streaming TTS failed: %v", err)
	}

	fmt.Printf("Delivered %d chunks (%d samples streamed)\n", chunkCount, sampleCount)
	fmt.Printf("Assembled %d samples at %d Hz (%.2fs)\n",
		len(result.Samples),
		result.SampleRate,
		result.Duration.Seconds(),
	)
}

func streamingTTSConfigFromEnv() (*tts.Config, bool) {
	config := &tts.Config{
		Enabled:     true,
		Backend:     tts.BackendSherpaOffline,
		ModelFamily: os.Getenv("VOICEKIT_TTS_FAMILY"),
		Provider:    envString("VOICEKIT_TTS_PROVIDER", "cpu"),
		NumThreads:  envInt("VOICEKIT_TTS_THREADS", 1),
		SpeakerID:   envInt("VOICEKIT_TTS_SPEAKER_ID", 0),
		Speed:       envFloat32("VOICEKIT_TTS_SPEED", 1),
	}
	if config.ModelFamily == "" {
		config.ModelFamily = tts.FamilyKokoro
	}

	switch config.ModelFamily {
	case tts.FamilyVits:
		config.Vits = tts.VitsConfig{
			Model:       os.Getenv("VOICEKIT_TTS_MODEL"),
			Lexicon:     os.Getenv("VOICEKIT_TTS_LEXICON"),
			Tokens:      os.Getenv("VOICEKIT_TTS_TOKENS"),
			DataDir:     os.Getenv("VOICEKIT_TTS_DATA_DIR"),
			NoiseScale:  0.667,
			NoiseScaleW: 0.8,
			LengthScale: 1,
		}
		return config, config.Vits.Model != "" && config.Vits.Tokens != "" && config.Vits.DataDir != ""
	case tts.FamilyMatcha:
		config.Matcha = tts.MatchaConfig{
			AcousticModel: os.Getenv("VOICEKIT_TTS_ACOUSTIC_MODEL"),
			Vocoder:       os.Getenv("VOICEKIT_TTS_VOCODER"),
			Lexicon:       os.Getenv("VOICEKIT_TTS_LEXICON"),
			Tokens:        os.Getenv("VOICEKIT_TTS_TOKENS"),
			DataDir:       os.Getenv("VOICEKIT_TTS_DATA_DIR"),
			NoiseScale:    0.667,
			LengthScale:   1,
		}
		return config, config.Matcha.AcousticModel != "" && config.Matcha.Vocoder != "" &&
			config.Matcha.Tokens != "" && config.Matcha.DataDir != ""
	case tts.FamilyKokoro:
		config.Kokoro = tts.KokoroConfig{
			Model:       os.Getenv("VOICEKIT_TTS_MODEL"),
			Voices:      os.Getenv("VOICEKIT_TTS_VOICES"),
			Tokens:      os.Getenv("VOICEKIT_TTS_TOKENS"),
			DataDir:     os.Getenv("VOICEKIT_TTS_DATA_DIR"),
			Lexicon:     os.Getenv("VOICEKIT_TTS_LEXICON"),
			Lang:        os.Getenv("VOICEKIT_TTS_LANG"),
			LengthScale: 1,
		}
		return config, config.Kokoro.Model != "" && config.Kokoro.Voices != "" &&
			config.Kokoro.Tokens != "" && config.Kokoro.DataDir != ""
	case tts.FamilyKitten:
		config.Kitten = tts.KittenConfig{
			Model:       os.Getenv("VOICEKIT_TTS_MODEL"),
			Voices:      os.Getenv("VOICEKIT_TTS_VOICES"),
			Tokens:      os.Getenv("VOICEKIT_TTS_TOKENS"),
			DataDir:     os.Getenv("VOICEKIT_TTS_DATA_DIR"),
			LengthScale: 1,
		}
		return config, config.Kitten.Model != "" && config.Kitten.Voices != "" &&
			config.Kitten.Tokens != "" && config.Kitten.DataDir != ""
	default:
		return config, false
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

func envFloat32(name string, fallback float32) float32 {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return fallback
	}
	return float32(parsed)
}
