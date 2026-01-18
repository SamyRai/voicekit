package speaker

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

// BenchmarkSpeakerSimilarity benchmarks cosine similarity calculation
func BenchmarkSpeakerSimilarity(b *testing.B) {
	// Create test embeddings (typical 192-dimension speaker embeddings)
	dim := 192
	embedding1 := make([]float32, dim)
	embedding2 := make([]float32, dim)

	// Fill with test data
	for i := 0; i < dim; i++ {
		embedding1[i] = float32(math.Sin(float64(i) * 0.1))
		embedding2[i] = float32(math.Cos(float64(i) * 0.1))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = CosineSimilarity(embedding1, embedding2)
	}
}

// BenchmarkSpeakerSimilarityOptimized benchmarks optimized similarity calculation
func BenchmarkSpeakerSimilarityOptimized(b *testing.B) {
	dim := 192
	embedding1 := make([]float32, dim)
	embedding2 := make([]float32, dim)

	for i := 0; i < dim; i++ {
		embedding1[i] = float32(math.Sin(float64(i) * 0.1))
		embedding2[i] = float32(math.Cos(float64(i) * 0.1))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = CosineSimilarity(embedding1, embedding2) // Uses optimized version
	}
}

// BenchmarkSpeakerSearch_10 benchmarks speaker search with 10 speakers
func BenchmarkSpeakerSearch_10(b *testing.B) {
	benchmarkSpeakerSearch(b, 10)
}

// BenchmarkSpeakerSearch_100 benchmarks speaker search with 100 speakers
func BenchmarkSpeakerSearch_100(b *testing.B) {
	benchmarkSpeakerSearch(b, 100)
}

// BenchmarkSpeakerSearch_1000 benchmarks speaker search with 1000 speakers
func BenchmarkSpeakerSearch_1000(b *testing.B) {
	benchmarkSpeakerSearch(b, 1000)
}

func benchmarkSpeakerSearch(b *testing.B, numSpeakers int) {
	// This would require a mock manager for benchmarking
	// For now, we'll create a simplified benchmark that tests the similarity calculation
	// that would be done during speaker search

	dim := 192
	queryEmbedding := make([]float32, dim)
	speakerEmbeddings := make([][]float32, numSpeakers)

	// Generate query embedding
	for i := 0; i < dim; i++ {
		queryEmbedding[i] = float32(math.Sin(float64(i) * 0.05))
	}

	// Generate speaker embeddings (slightly different from query)
	for s := 0; s < numSpeakers; s++ {
		embedding := make([]float32, dim)
		for i := 0; i < dim; i++ {
			// Add some variation based on speaker index
			embedding[i] = float32(math.Sin(float64(i)*0.05 + float64(s)*0.1))
		}
		speakerEmbeddings[s] = embedding
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate speaker search: calculate similarity with all speakers
		maxSimilarity := float32(0.0)
		for _, speakerEmb := range speakerEmbeddings {
			similarity := CosineSimilarity(queryEmbedding, speakerEmb)
			if similarity > maxSimilarity {
				maxSimilarity = similarity
			}
		}
		_ = maxSimilarity
	}
}

// BenchmarkBatchSpeakerIdentify benchmarks batch speaker identification
func BenchmarkBatchSpeakerIdentify(b *testing.B) {
	// This would require a full manager setup with database
	// For now, we'll benchmark the batch processing logic

	dim := 192
	batchSize := 8
	queryEmbeddings := make([][]float32, batchSize)

	// Generate batch of query embeddings
	for batch := 0; batch < batchSize; batch++ {
		embedding := make([]float32, dim)
		for i := 0; i < dim; i++ {
			embedding[i] = float32(math.Sin(float64(i)*0.05 + float64(batch)*0.2))
		}
		queryEmbeddings[batch] = embedding
	}

	// Simulate database with 100 speakers
	numSpeakers := 100
	speakerEmbeddings := make([][]float32, numSpeakers)
	for s := 0; s < numSpeakers; s++ {
		embedding := make([]float32, dim)
		for i := 0; i < dim; i++ {
			embedding[i] = float32(math.Sin(float64(i)*0.05 + float64(s)*0.15))
		}
		speakerEmbeddings[s] = embedding
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate batch identification
		results := make([]float32, batchSize)
		for batch := 0; batch < batchSize; batch++ {
			queryEmb := queryEmbeddings[batch]
			maxSimilarity := float32(0.0)

			// Find best match for this query
			for _, speakerEmb := range speakerEmbeddings {
				similarity := CosineSimilarity(queryEmb, speakerEmb)
				if similarity > maxSimilarity {
					maxSimilarity = similarity
				}
			}
			results[batch] = maxSimilarity
		}
		_ = results
	}
}

// BenchmarkDatabaseOperations benchmarks database read/write operations
func BenchmarkDatabaseOperations(b *testing.B) {
	// This would require setting up a test database
	// For now, we'll benchmark the data structures and serialization

	dim := 192
	speakerData := &SpeakerData{
		ID:          "test_speaker",
		Name:        "Test Speaker",
		Embeddings:  make([][]float32, 3), // 3 enrollment samples
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		SampleCount: 3,
	}

	// Generate embeddings
	for e := 0; e < 3; e++ {
		embedding := make([]float32, dim)
		for i := 0; i < dim; i++ {
			embedding[i] = float32(math.Sin(float64(i)*0.05 + float64(e)*0.3))
		}
		speakerData.Embeddings[e] = embedding
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate JSON marshaling (what database operations would do)
		_, err := json.Marshal(speakerData)
		if err != nil {
			b.Fatal(err)
		}
	}
}