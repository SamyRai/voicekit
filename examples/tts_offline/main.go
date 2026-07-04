package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/SamyRai/voicekit"
)

func main() {
	text := os.Getenv("VOICEKIT_TTS_TEXT")
	if text == "" {
		text = "VoiceKit offline text to speech is ready."
	}

	ttsConfig, ok := offlineTTSConfigFromEnv()
	if !ok {
		fmt.Println("Sherpa offline TTS model paths are not configured.")
		fmt.Println("Set VOICEKIT_TTS_FAMILY plus the required VOICEKIT_TTS_* model path variables.")
		return
	}

	config := &voicekit.Config{
		TTS: ttsConfig,
	}

	vk, err := voicekit.NewVoiceKit(config)
	if err != nil {
		log.Fatalf("failed to create VoiceKit: %v", err)
	}
	defer vk.Close()

	result, err := vk.Synthesizer().Synthesize(context.Background(), voicekit.SynthesisRequest{
		Text:      text,
		SpeakerID: envInt("VOICEKIT_TTS_SPEAKER_ID", ttsConfig.SpeakerID),
		Speed:     envFloat32("VOICEKIT_TTS_SPEED", ttsConfig.Speed),
	})
	if err != nil {
		log.Fatalf("offline TTS failed: %v", err)
	}
	fmt.Printf("Generated %d samples at %d Hz (%.2fs)\n",
		len(result.Samples),
		result.SampleRate,
		result.Duration.Seconds(),
	)
}

func offlineTTSConfigFromEnv() (voicekit.TTSConfig, bool) {
	config := voicekit.TTSConfig{
		Enabled:     true,
		Backend:     "sherpa_offline",
		ModelFamily: os.Getenv("VOICEKIT_TTS_FAMILY"),
		Provider:    envString("VOICEKIT_TTS_PROVIDER", "cpu"),
		NumThreads:  envInt("VOICEKIT_TTS_THREADS", 1),
		SpeakerID:   envInt("VOICEKIT_TTS_SPEAKER_ID", 0),
		Speed:       envFloat32("VOICEKIT_TTS_SPEED", 1),
	}
	if config.ModelFamily == "" {
		config.ModelFamily = "kokoro"
	}

	switch config.ModelFamily {
	case "vits":
		config.Vits = voicekit.TTSVitsConfig{
			Model:       os.Getenv("VOICEKIT_TTS_MODEL"),
			Lexicon:     os.Getenv("VOICEKIT_TTS_LEXICON"),
			Tokens:      os.Getenv("VOICEKIT_TTS_TOKENS"),
			DataDir:     os.Getenv("VOICEKIT_TTS_DATA_DIR"),
			NoiseScale:  0.667,
			NoiseScaleW: 0.8,
			LengthScale: 1,
		}
		return config, config.Vits.Model != "" && config.Vits.Tokens != "" && config.Vits.DataDir != ""
	case "matcha":
		config.Matcha = voicekit.TTSMatchaConfig{
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
	case "kokoro":
		config.Kokoro = voicekit.TTSKokoroConfig{
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
	case "kitten":
		config.Kitten = voicekit.TTSKittenConfig{
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
