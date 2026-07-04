// Package indexing provides HNSW-based vector indexing functionality
package indexing

import "fmt"

// This file contains HNSW implementation details and utilities
// It's separated from hnsw.go to keep the main interface clean

// Note: The actual HNSW implementation is in hnsw.go
// This file can contain additional HNSW-specific utilities or optimizations

// HNSWIndexImpl provides additional HNSW-specific functionality
// This is an internal implementation detail
type HNSWIndexImpl struct {
	// This could contain additional HNSW-specific optimizations
	// For now, it's empty as the main functionality is in HNSWIndex
}

// ValidateHNSWParameters validates HNSW parameters for optimal performance
func ValidateHNSWParameters(dim, maxElements, m, efConstruction, efSearch int) error {
	if dim <= 0 {
		return fmt.Errorf("dimension must be positive")
	}
	if maxElements <= 0 {
		return fmt.Errorf("maxElements must be positive")
	}
	if m <= 0 || m > 100 {
		return fmt.Errorf("m must be between 1 and 100")
	}
	if efConstruction < m {
		return fmt.Errorf("efConstruction must be >= M")
	}
	if efSearch <= 0 {
		return fmt.Errorf("efSearch must be positive")
	}

	// Additional performance recommendations
	if m > 64 {
		return fmt.Errorf("m > 64 may cause excessive memory usage")
	}
	if efConstruction > 800 {
		return fmt.Errorf("efConstruction > 800 may cause excessive build time")
	}
	if efSearch > 400 {
		return fmt.Errorf("efSearch > 400 may cause slow queries")
	}

	return nil
}

// GetRecommendedHNSWConfig returns recommended HNSW configuration for given scale
func GetRecommendedHNSWConfig(scale string) *Config {
	switch scale {
	case "small": // < 1K vectors
		return &Config{
			Dimension:      192,
			MaxElements:    1000,
			M:              8,
			EfConstruction: 100,
			EfSearch:       50,
			DistanceMetric: Cosine,
		}
	case "medium": // 1K - 50K vectors
		return &Config{
			Dimension:      192,
			MaxElements:    50000,
			M:              16,
			EfConstruction: 200,
			EfSearch:       100,
			DistanceMetric: Cosine,
		}
	case "large": // 50K - 1M vectors
		return &Config{
			Dimension:      192,
			MaxElements:    1000000,
			M:              32,
			EfConstruction: 400,
			EfSearch:       200,
			DistanceMetric: Cosine,
		}
	default:
		return DefaultConfig()
	}
}
