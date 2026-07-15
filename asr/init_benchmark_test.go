package asr

import (
	"context"
	"os"
	"testing"
)

// These benchmarks quantify the one-time cost of loading a native online
// recognizer, to decide (per the golang-performance measure-first rule) whether
// recognizer warmup/pooling is worth adding. They need real model files and
// skip otherwise, using the same VOICEKIT_ONLINE_ASR_* env vars as
// TestSherpaOnlineModelIntegration.
//
// Run with real models:
//
//	VOICEKIT_ONLINE_ASR_TOKENS=... VOICEKIT_ONLINE_ASR_MODEL=... \
//	  go test -bench='NewSherpaOnlineModel|FirstChunk' -benchmem -run=^$ -count=6 ./asr | tee /tmp/asr-init.txt
//	benchstat /tmp/asr-init.txt
func benchOnlineConfigFromEnv() Config {
	config := DefaultConfig()
	config.Enabled = true
	config.Backend = BackendSherpaOnline
	config.DefaultModel = "bench_sherpa_online"
	config.Online.TokensPath = os.Getenv("VOICEKIT_ONLINE_ASR_TOKENS")
	config.Online.EncoderPath = os.Getenv("VOICEKIT_ONLINE_ASR_ENCODER")
	config.Online.DecoderPath = os.Getenv("VOICEKIT_ONLINE_ASR_DECODER")
	config.Online.JoinerPath = os.Getenv("VOICEKIT_ONLINE_ASR_JOINER")
	config.Online.ModelPath = os.Getenv("VOICEKIT_ONLINE_ASR_MODEL")
	config.Online.ModelType = os.Getenv("VOICEKIT_ONLINE_ASR_MODEL_TYPE")
	config.Language = "en"
	return config
}

// BenchmarkNewSherpaOnlineModel measures the wall-clock + allocations of loading
// a native online recognizer (sherpa.NewOnlineRecognizer). If this dominates,
// warmup/pooling of recognizers is justified.
func BenchmarkNewSherpaOnlineModel(b *testing.B) {
	config := benchOnlineConfigFromEnv()
	if !onlineIntegrationConfigured(config.Online) {
		b.Skip("set VOICEKIT_ONLINE_ASR_* model path env vars to benchmark recognizer init cost")
	}

	for b.Loop() {
		model, err := NewSherpaOnlineModel(&config)
		if err != nil {
			b.Fatalf("failed to create model: %v", err)
		}
		if err := model.Close(); err != nil {
			b.Fatalf("failed to close model: %v", err)
		}
	}
}

// BenchmarkSherpaOnlineModelFirstChunk measures the latency of the first decode
// on a freshly created model (cold path). Comparing this to steady-state
// per-chunk latency quantifies the first-chunk penalty a warmup decode would
// hide.
func BenchmarkSherpaOnlineModelFirstChunk(b *testing.B) {
	config := benchOnlineConfigFromEnv()
	if !onlineIntegrationConfigured(config.Online) {
		b.Skip("set VOICEKIT_ONLINE_ASR_* model path env vars to benchmark first-chunk latency")
	}

	ctx := context.Background()
	audio := make([]float32, config.ChunkSize)

	for b.Loop() {
		b.StopTimer()
		model, err := NewSherpaOnlineModel(&config)
		if err != nil {
			b.Fatalf("failed to create model: %v", err)
		}
		b.StartTimer()

		if _, err := model.ProcessAudio(ctx, audio, nil); err != nil {
			b.Fatalf("first-chunk decode failed: %v", err)
		}

		b.StopTimer()
		_ = model.Close()
		b.StartTimer()
	}
}
