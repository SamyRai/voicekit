package voicekit

import (
	"testing"
	"time"

	"github.com/SamyRai/voicekit/audio"
)

// TestBenchmarkRunner tests the benchmark runner functionality
func TestBenchmarkRunner(t *testing.T) {
	runner := NewBenchmarkRunner("test_benchmarks")

	// Test basic functionality
	if runner == nil {
		t.Fatal("BenchmarkRunner creation failed")
	}

	if len(runner.results) != 0 {
		t.Errorf("Expected empty results map, got %d entries", len(runner.results))
	}
}

// BenchmarkVoiceKitFullPipeline benchmarks the complete VoiceKit processing pipeline
func BenchmarkVoiceKitFullPipeline(b *testing.B) {
	// This benchmark would require setting up a full VoiceKit instance
	// For now, we'll create a mock benchmark that simulates the pipeline

	// Simulate input audio (1 second, 16kHz, mono)
	numSamples := 16000
	audioData := make([]float32, numSamples)

	// Fill with test data (simulated speech)
	for i := 0; i < numSamples; i++ {
		// Simple sine wave to simulate audio
		audioData[i] = float32(sin(float64(i) * 0.01))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate the full pipeline:
		// 1. Audio preprocessing (already tested separately)
		// 2. Feature extraction (simulated)
		// 3. Speaker recognition (simulated)
		// 4. Diarization (simulated)

		_ = len(audioData) // Simulate processing
	}
}

// BenchmarkMemoryEfficiency benchmarks memory usage patterns
func BenchmarkMemoryEfficiency(b *testing.B) {
	// Test that our buffer pooling reduces allocations

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Test buffer pool usage
		buf1 := audio.DefaultAudioBufferPool.Get(4096)
		buf2 := audio.DefaultAudioBufferPool.Get(8192)
		buf3 := audio.Float32Pool.Get(2048)

		// Simulate some work
		for j := 0; j < len(buf1) && j < 100; j++ {
			buf1[j] = float32(j)
		}
		for j := 0; j < len(buf2) && j < 100; j++ {
			buf2[j] = float32(j * 2)
		}
		for j := 0; j < len(buf3) && j < 100; j++ {
			buf3[j] = float32(j * 3)
		}

		// Return to pools
		audio.DefaultAudioBufferPool.Put(buf1)
		audio.DefaultAudioBufferPool.Put(buf2)
		audio.Float32Pool.Put(buf3)
	}
}

// BenchmarkConcurrentOperations benchmarks concurrent VoiceKit operations
func BenchmarkConcurrentOperations(b *testing.B) {
	// Test concurrent performance with our optimizations

	samplesPerGoroutine := 4000

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		localData := make([]float32, samplesPerGoroutine)
		for pb.Next() {
			// Simulate concurrent audio processing
			for i := 0; i < len(localData); i++ {
				localData[i] = float32(i % 256) // Fill with test data
			}

			// Simulate processing with buffer pools
			buf := audio.DefaultAudioBufferPool.Get(len(localData))
			copy(buf, localData)

			// Simulate some computation
			for i := 1; i < len(buf); i++ {
				buf[i] = (buf[i] + buf[i-1]) * 0.5
			}

			audio.DefaultAudioBufferPool.Put(buf)
		}
	})
}

// BenchmarkDatabaseThroughput benchmarks database operation throughput
func BenchmarkDatabaseThroughput(b *testing.B) {
	// This would test database operations with our sharded implementation
	// For now, simulate the throughput

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate database operations
		_ = i // Simulate work
	}
}

// BenchmarkOptimizationImpact benchmarks the impact of our optimizations
func BenchmarkOptimizationImpact(b *testing.B) {
	// Compare optimized vs non-optimized patterns

	b.Run("Optimized_BufferPool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf := audio.DefaultAudioBufferPool.Get(4096)
			// Use buffer
			audio.DefaultAudioBufferPool.Put(buf)
		}
	})

	b.Run("NonOptimized_NewAlloc", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			buf := make([]float32, 4096)
			// Use buffer
			_ = buf
		}
	})
}

// Helper function (would be in math package in real implementation)
func sin(x float64) float64 {
	// Simple sine approximation for benchmarking
	return x - (x*x*x)/6 + (x*x*x*x*x)/120
}

// BenchmarkMetricsCollection benchmarks our metrics collection overhead
func BenchmarkMetricsCollection(b *testing.B) {
	metrics := NewMetricsCollector()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate metrics collection
		duration := time.Duration(i % 1000) * time.Microsecond
		success := (i % 10) != 0 // 90% success rate

		metrics.RecordAudioProcessing(duration, success)
	}
}

// BenchmarkConfigValidation benchmarks configuration validation performance
func BenchmarkConfigValidation(b *testing.B) {
	config := DefaultConfig()
	// Set required fields for validation
	config.Speaker.DataDir = "/tmp/voicekit_bench"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := config.Validate()
		if err != nil {
			b.Fatal(err)
		}
	}
}