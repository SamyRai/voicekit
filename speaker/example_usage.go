// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	"github.com/SamyRai/voicekit/indexing"
)

// Example demonstrates how to use the generic HNSW vector indexer
func main() {
	// Create HNSW configuration
	config := &indexing.Config{
		Dimension:      128, // Embedding dimension
		MaxElements:    1000, // Maximum vectors
		M:              16,   // Number of connections per node
		EfConstruction: 200,  // Construction quality
		EfSearch:       100,  // Search quality
		DistanceMetric: indexing.Cosine,
	}

	// Create indexer
	indexer, err := indexing.NewHNSWIndex(config)
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
	}
	defer indexer.Close()

	fmt.Printf("Created HNSW indexer with dimension %d\n", config.Dimension)

	ctx := context.Background()

	// Add some vectors
	for i := 0; i < 10; i++ {
		vectorID := fmt.Sprintf("vector_%03d", i)

		// Generate random embedding (in real usage, this would come from ML models)
		embedding := make([]float32, 128)
		for j := range embedding {
			embedding[j] = rand.Float32()
		}

		err := indexer.Add(ctx, vectorID, embedding)
		if err != nil {
			log.Printf("Failed to add vector %s: %v", vectorID, err)
			continue
		}

		fmt.Printf("Added vector: %s\n", vectorID)
	}

	// Search for similar vectors
	queryVector := make([]float32, 128)
	for j := range queryVector {
		queryVector[j] = rand.Float32()
	}

	results, err := indexer.Search(ctx, queryVector, 5, 0.0) // Top 5, no threshold
	if err != nil {
		log.Printf("Search failed: %v", err)
	} else if len(results) == 0 {
		fmt.Println("No vectors found")
	} else {
		fmt.Printf("Found %d similar vectors:\n", len(results))
		for _, result := range results {
			fmt.Printf("  ID: %s, Similarity: %.3f, Distance: %.3f\n",
				result.ID, result.Similarity, result.Distance)
		}
	}

	// Show statistics
	size, err := indexer.Size(ctx)
	if err != nil {
		log.Printf("Failed to get size: %v", err)
	} else {
		fmt.Printf("Total vectors in index: %d\n", size)
	}

	// Demonstrate interface usage
	var indexerInterface indexing.VectorIndex = indexer
	fmt.Printf("Indexer implements VectorIndex interface: %T\n", indexerInterface)

	fmt.Println("Generic HNSW indexer example completed successfully!")
}