package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/SamyRai/voicekit/asr"
)

func main() {
	text := os.Getenv("VOICEKIT_PUNCT_TEXT")
	if text == "" {
		text = "voicekit restores punctuation and casing for raw asr transcripts"
	}

	config, ok := punctuationConfigFromEnv()
	if !ok {
		fmt.Println("Sherpa offline punctuation model path is not configured.")
		fmt.Println("Set VOICEKIT_PUNCT_MODEL to a ct-transformer punctuation ONNX model file.")
		return
	}

	restorer, err := asr.NewSherpaPunctuation(&config)
	if err != nil {
		log.Fatalf("failed to create punctuation restorer: %v", err)
	}
	defer restorer.Close()

	restored, err := restorer.Restore(context.Background(), text)
	if err != nil {
		log.Fatalf("punctuation restore failed: %v", err)
	}
	fmt.Println(restored)
}

func punctuationConfigFromEnv() (asr.PunctuationConfig, bool) {
	config := asr.DefaultPunctuationConfig()
	config.ModelPath = os.Getenv("VOICEKIT_PUNCT_MODEL")
	config.Provider = envString("VOICEKIT_PUNCT_PROVIDER", config.Provider)
	config.NumThreads = envInt("VOICEKIT_PUNCT_THREADS", config.NumThreads)
	return config, config.ModelPath != ""
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
