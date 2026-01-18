package indexing

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// TestHNSWIndex_BasicOperations tests basic HNSW index operations
func TestHNSWIndex_BasicOperations(t *testing.T) {
	config := &Config{
		Dimension:      128, // Use smaller dimension for testing
		MaxElements:    100,
		M:              8,
		EfConstruction: 50,
		EfSearch:       50,
		DistanceMetric: Cosine,
	}

	index, err := NewHNSWIndex(config)
	if err != nil {
		t.Fatalf("Failed to create HNSW index: %v", err)
	}
	defer index.Close()

	ctx := context.Background()

	// Test adding vectors
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("vector_%d", i)
		vector := make([]float32, 128)
		for j := range vector {
			vector[j] = rand.Float32()
		}

		err := index.Add(ctx, id, vector)
		if err != nil {
			t.Fatalf("Failed to add vector %s: %v", id, err)
		}
	}

	// Test search
	queryVector := make([]float32, 128)
	for j := range queryVector {
		queryVector[j] = rand.Float32()
	}

	results, err := index.Search(ctx, queryVector, 5, 0.0)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if len(results) == 0 {
		t.Log("No results found (expected for random vectors)")
	} else {
		t.Logf("Found %d results", len(results))
		for _, result := range results {
			t.Logf("  ID: %s, Similarity: %.3f, Distance: %.3f",
				result.ID, result.Similarity, result.Distance)
		}
	}

	// Test size
	size, err := index.Size(ctx)
	if err != nil {
		t.Fatalf("Failed to get size: %v", err)
	}
	if size != 10 {
		t.Errorf("Expected size 10, got %d", size)
	}
}

// TestHNSWIndex_Remove tests vector removal
func TestHNSWIndex_Remove(t *testing.T) {
	config := &Config{
		Dimension:      64,
		MaxElements:    50,
		M:              8,
		EfConstruction: 50,
		EfSearch:       50,
		DistanceMetric: Cosine,
	}

	index, err := NewHNSWIndex(config)
	if err != nil {
		t.Fatalf("Failed to create HNSW index: %v", err)
	}
	defer index.Close()

	ctx := context.Background()

	// Add a vector
	id := "test_vector"
	vector := make([]float32, 64)
	for j := range vector {
		vector[j] = rand.Float32()
	}

	err = index.Add(ctx, id, vector)
	if err != nil {
		t.Fatalf("Failed to add vector: %v", err)
	}

	// Verify it's added
	size, err := index.Size(ctx)
	if err != nil {
		t.Fatalf("Failed to get size: %v", err)
	}
	if size != 1 {
		t.Errorf("Expected size 1 after add, got %d", size)
	}

	// Remove the vector
	err = index.Remove(ctx, id)
	if err != nil {
		t.Fatalf("Failed to remove vector: %v", err)
	}

	// Note: hnswlib marks as deleted, but ElementCount might still include deleted elements
	t.Logf("Vector removed successfully")
}

// TestConfig_Validation tests configuration validation
func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Dimension:      192,
				MaxElements:    1000,
				M:              16,
				EfConstruction: 200,
				EfSearch:       100,
				DistanceMetric: Cosine,
			},
			wantErr: false,
		},
		{
			name: "invalid dimension",
			config: &Config{
				Dimension:      0,
				MaxElements:    1000,
				M:              16,
				EfConstruction: 200,
				EfSearch:       100,
				DistanceMetric: Cosine,
			},
			wantErr: true,
		},
		{
			name: "invalid efConstruction",
			config: &Config{
				Dimension:      192,
				MaxElements:    1000,
				M:              16,
				EfConstruction: 8, // Less than M
				EfSearch:       100,
				DistanceMetric: Cosine,
			},
			wantErr: true,
		},
		{
			name: "invalid distance metric",
			config: &Config{
				Dimension:      192,
				MaxElements:    1000,
				M:              16,
				EfConstruction: 200,
				EfSearch:       100,
				DistanceMetric: DistanceMetric("invalid"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestIDMapper tests the ID mapping functionality
func TestIDMapper(t *testing.T) {
	mapper := NewIDMapper()

	// Test GetIntID
	id1 := mapper.GetIntID("test1")
	id2 := mapper.GetIntID("test2")
	id3 := mapper.GetIntID("test1") // Same ID should return same int

	if id1 == id2 {
		t.Errorf("Different strings should get different int IDs")
	}
	if id1 != id3 {
		t.Errorf("Same string should get same int ID")
	}

	// Test GetStringID
	str1, exists1 := mapper.GetStringID(id1)
	str2, exists2 := mapper.GetStringID(id2)

	if !exists1 || str1 != "test1" {
		t.Errorf("Expected to find 'test1', got %s (exists: %v)", str1, exists1)
	}
	if !exists2 || str2 != "test2" {
		t.Errorf("Expected to find 'test2', got %s (exists: %v)", str2, exists2)
	}

	// Test RemoveID
	mapper.RemoveID("test1")
	_, existsAfterRemove := mapper.GetStringID(id1)
	if existsAfterRemove {
		t.Errorf("ID should not exist after removal")
	}

	// Test size
	if mapper.Size() != 1 { // Only test2 should remain
		t.Errorf("Expected size 1 after removal, got %d", mapper.Size())
	}
}

// BenchmarkHNSWIndex_Add benchmarks adding vectors
func BenchmarkHNSWIndex_Add(b *testing.B) {
	config := &Config{
		Dimension:      128,
		MaxElements:    10000,
		M:              8,
		EfConstruction: 50,
		EfSearch:       50,
		DistanceMetric: Cosine,
	}

	index, err := NewHNSWIndex(config)
	if err != nil {
		b.Fatalf("Failed to create HNSW index: %v", err)
	}
	defer index.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("bench_vector_%d", i)
		vector := make([]float32, 128)
		for j := range vector {
			vector[j] = rand.Float32()
		}

		err := index.Add(ctx, id, vector)
		if err != nil {
			b.Fatalf("Failed to add vector: %v", err)
		}
	}
}

// BenchmarkHNSWIndex_Search benchmarks searching
func BenchmarkHNSWIndex_Search(b *testing.B) {
	config := &Config{
		Dimension:      128,
		MaxElements:    100,
		M:              8,
		EfConstruction: 50,
		EfSearch:       50,
		DistanceMetric: Cosine,
	}

	index, err := NewHNSWIndex(config)
	if err != nil {
		b.Fatalf("Failed to create HNSW index: %v", err)
	}
	defer index.Close()

	ctx := context.Background()

	// Add some vectors
	for i := 0; i < 50; i++ {
		id := fmt.Sprintf("bench_vector_%d", i)
		vector := make([]float32, 128)
		for j := range vector {
			vector[j] = rand.Float32()
		}

		err := index.Add(ctx, id, vector)
		if err != nil {
			b.Fatalf("Failed to add vector: %v", err)
		}
	}

	// Benchmark search
	queryVector := make([]float32, 128)
	for j := range queryVector {
		queryVector[j] = rand.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := index.Search(ctx, queryVector, 5, 0.0)
		if err != nil {
			b.Fatalf("Failed to search: %v", err)
		}
	}
}

// TestSimilarityFunctions tests similarity calculation functions
func TestSimilarityFunctions(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{4.0, 5.0, 6.0}

	cosine := CosineSimilarity(a, b)
	l2 := L2Distance(a, b)
	ip := DotProduct(a, b)

	if cosine <= 0 || cosine > 1 {
		t.Errorf("Cosine similarity should be between 0 and 1, got %f", cosine)
	}
	if l2 < 0 {
		t.Errorf("L2 distance should be non-negative, got %f", l2)
	}
	// Inner product can be any value, no specific test needed

	t.Logf("Cosine: %f, L2: %f, Inner Product: %f", cosine, l2, ip)
}

// TestNormalizeVector tests vector normalization
func TestNormalizeVector(t *testing.T) {
	vector := []float32{3.0, 4.0} // Should normalize to [0.6, 0.8]
	normalized := NormalizeVector(vector)

	// Check length is approximately 1
	var length float32
	for _, v := range normalized {
		length += v * v
	}
	length = float32(math.Sqrt(float64(length)))

	if math.Abs(float64(length-1.0)) > 0.001 {
		t.Errorf("Normalized vector should have length 1, got %f", length)
	}

	expected := []float32{0.6, 0.8}
	for i, v := range normalized {
		if math.Abs(float64(v-expected[i])) > 0.001 {
			t.Errorf("Expected %f at position %d, got %f", expected[i], i, v)
		}
	}
}