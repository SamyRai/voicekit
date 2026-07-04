package diarization

import (
	"context"
	"math"
	"testing"
)

// BenchmarkClustering benchmarks speaker clustering algorithm
func BenchmarkClustering(b *testing.B) {
	benchmarkClustering(b, 10) // 10 segments
}

func BenchmarkClustering_50(b *testing.B) {
	benchmarkClustering(b, 50) // 50 segments
}

func BenchmarkClustering_100(b *testing.B) {
	benchmarkClustering(b, 100) // 100 segments
}

func benchmarkClustering(b *testing.B, numSegments int) {
	dim := 192

	// Generate test embeddings for segments
	embeddings := make([][]float32, numSegments)
	for i := 0; i < numSegments; i++ {
		embedding := make([]float32, dim)
		// Create embeddings that should cluster into ~3-5 speakers
		speakerGroup := i % 4 // 4 different speakers
		for j := 0; j < dim; j++ {
			baseValue := math.Sin(float64(j) * 0.05)
			// Add speaker-specific variation
			speakerVariation := math.Sin(float64(speakerGroup) * 0.5)
			// Add some noise
			noise := (math.Sin(float64(i*j)*0.01) * 0.1)
			embedding[j] = float32(baseValue + speakerVariation + noise)
		}
		embeddings[i] = embedding
	}

	config := &DiarizationConfig{
		Enabled:             true,
		Backend:             BackendBasic,
		SimilarityThreshold: 0.7,
		MaxSpeakers:         10,
	}
	config.ApplyDefaults()

	backend := newBasicBackend(config, nil, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = backend.performClustering(embeddings)
	}
}

// BenchmarkAudioSegmentation benchmarks audio segmentation
func BenchmarkAudioSegmentation(b *testing.B) {
	// Create test audio (5 seconds at 16kHz)
	sampleRate := 16000
	duration := 5.0
	numSamples := int(float64(sampleRate) * duration)

	audioData := make([]float32, numSamples)

	// Generate test audio with speech-like patterns
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)

		// Simulate speech segments with silence gaps
		segment := int(t) % 10 // 10-second cycle
		if segment < 3 {       // 3 seconds of "speech"
			// Speech-like signal
			freq := 100 + math.Sin(t*2)*50 // Varying frequency
			audioData[i] = float32(math.Sin(2*math.Pi*freq*t) * 0.3)
		} else {
			// Silence with some noise
			audioData[i] = float32((math.Sin(t*1000) * 0.01))
		}
	}

	config := &DiarizationConfig{
		Backend:             BackendBasic,
		MinSegmentLength:    1.0,
		MaxSegmentLength:    30.0,
		SilenceThreshold:    0.5,
		SimilarityThreshold: 0.7,
	}

	segmenter := NewSegmenter(config)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := segmenter.SegmentBySilence(audioData, sampleRate)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEmbeddingExtraction benchmarks fake extractor allocation patterns.
func BenchmarkEmbeddingExtraction(b *testing.B) {
	numSegments := 20
	dim := 192

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		embeddings := make([][]float32, numSegments)
		for j := 0; j < numSegments; j++ {
			embedding := make([]float32, dim)
			for k := 0; k < dim; k++ {
				embedding[k] = float32(j%5) * 0.2
			}
			embeddings[j] = embedding
		}
		_ = embeddings
	}
}

// mockSpeakerDatabase provides a mock implementation for benchmarking
type mockSpeakerDatabase struct{}

func (m *mockSpeakerDatabase) GetSpeakerEmbedding(speakerID string) ([]float32, error) {
	return nil, nil
}

func (m *mockSpeakerDatabase) GetAllSpeakers() map[string][]float32 {
	return make(map[string][]float32)
}

func (m *mockSpeakerDatabase) CalculateSimilarity(embedding1, embedding2 []float32) float32 {
	return 0.5 // Mock similarity
}

func (m *mockSpeakerDatabase) RegisterSpeakerEmbedding(speakerID string, embedding []float32) error {
	return nil
}

type benchmarkEmbeddingExtractor struct{}

func (benchmarkEmbeddingExtractor) ExtractEmbedding(ctx context.Context, audioData []float32, sampleRate int) ([]float32, error) {
	embedding := make([]float32, 192)
	for i := range embedding {
		embedding[i] = float32((len(audioData)+i)%17) / 17
	}
	return embedding, nil
}

// BenchmarkDiarizationPipeline benchmarks the full diarization pipeline
func BenchmarkDiarizationPipeline(b *testing.B) {
	// Create test audio (2 seconds at 16kHz for faster benchmarking)
	sampleRate := 16000
	duration := 2.0
	numSamples := int(float64(sampleRate) * duration)

	audioData := make([]float32, numSamples)

	// Generate test audio with multiple "speakers"
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		segment := int(t*2) % 4 // Switch every 0.5 seconds

		switch segment {
		case 0, 2: // "Speaker 1"
			audioData[i] = float32(math.Sin(2*math.Pi*120*t) * 0.3)
		case 1: // "Speaker 2"
			audioData[i] = float32(math.Sin(2*math.Pi*150*t) * 0.25)
		case 3: // Silence
			audioData[i] = float32(math.Sin(t*100) * 0.01) // Low noise
		}
	}

	config := &DiarizationConfig{
		Enabled:               true,
		Backend:               BackendBasic,
		MinSegmentLength:      0.5,
		MaxSegmentLength:      10.0,
		SilenceThreshold:      0.3,
		SimilarityThreshold:   0.6,
		MaxSpeakers:           5,
		ReassignmentThreshold: 0.8,
		OverlapThreshold:      0.2,
	}

	mockDB := &mockSpeakerDatabase{}
	manager, err := NewManagerWithBackend(config, mockDB, benchmarkEmbeddingExtractor{})
	if err != nil {
		b.Fatalf("failed to create manager: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := manager.ProcessAudio(audioData, sampleRate, "benchmark_session")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMemoryUsage benchmarks memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	// Test memory allocation patterns in diarization

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate creating and processing segments
		segments := make([]AudioSegment, 20)
		for j := 0; j < 20; j++ {
			segments[j] = AudioSegment{
				StartTime: float64(j) * 0.5,
				EndTime:   float64(j+1) * 0.5,
				Samples:   make([]float32, 8000), // 0.5s at 16kHz
			}

			// Fill with test data
			for k := 0; k < 8000; k++ {
				segments[j].Samples[k] = float32(math.Sin(float64(k+j) * 0.01))
			}
		}

		// Simulate processing
		for j := range segments {
			_ = len(segments[j].Samples)
		}
	}
}
