package diarization

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

type basicBackend struct {
	config             *DiarizationConfig
	speakerDB          SpeakerDatabase
	embeddingExtractor EmbeddingExtractor
	mu                 sync.Mutex

	scratchEmbeddings [][]float32
	ufParent          []int
	ufRank            []int
}

func newBasicBackend(config *DiarizationConfig, speakerDB SpeakerDatabase, extractor EmbeddingExtractor) *basicBackend {
	return &basicBackend{
		config:             config,
		speakerDB:          speakerDB,
		embeddingExtractor: extractor,
	}
}

func (b *basicBackend) Process(ctx context.Context, request Request) (*DiarizationResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	_ = b.speakerDB
	if b.config.Logger != nil {
		b.config.Logger.Infof("Starting basic diarization for session %s with %d samples", request.SessionID, len(request.Audio))
	}

	segments, err := b.segmentAudio(request.Audio, request.SampleRate)
	if err != nil {
		return nil, fmt.Errorf("failed to segment audio: %w", err)
	}
	processedAt := request.ProcessedAt
	if processedAt.IsZero() {
		processedAt = time.Now()
	}

	if len(segments) == 0 {
		return &DiarizationResult{
			SessionID:     request.SessionID,
			Segments:      []SpeakerSegment{},
			SpeakerCount:  0,
			TotalDuration: 0,
			ProcessedAt:   processedAt,
		}, nil
	}

	segmentEmbeddings, err := b.extractEmbeddings(ctx, segments, request.SampleRate)
	if err != nil {
		return nil, fmt.Errorf("failed to extract embeddings: %w", err)
	}

	speakerSegments, err := b.clusterAndAssignSpeakers(segmentEmbeddings, segments)
	if err != nil {
		return nil, fmt.Errorf("failed to cluster speakers: %w", err)
	}

	totalDuration := 0.0
	if len(segments) > 0 {
		totalDuration = segments[len(segments)-1].EndTime
	}

	result := &DiarizationResult{
		SessionID:     request.SessionID,
		Segments:      speakerSegments,
		SpeakerCount:  countUniqueSpeakers(speakerSegments),
		TotalDuration: totalDuration,
		ProcessedAt:   processedAt,
	}

	if b.config.Logger != nil {
		b.config.Logger.Infof("Basic diarization completed for session %s: %d segments, %d speakers, %.2f seconds",
			request.SessionID, len(speakerSegments), result.SpeakerCount, totalDuration)
	}
	return result, nil
}

func (b *basicBackend) Close() error {
	return nil
}

func (b *basicBackend) segmentAudio(audioData []float32, sampleRate int) ([]AudioSegment, error) {
	spans := []audioSegmentSpan{}
	minSegmentSamples := int(b.config.MinSegmentLength * float64(sampleRate))

	currentSegment := audioSegmentSpan{}
	currentSegment.setStartTime(0)
	activeStart := -1

	for i, sample := range audioData {
		currentTime := float64(i) / float64(sampleRate)
		isSilence := math.Abs(float64(sample)) < 0.01

		if isSilence && activeStart >= 0 {
			currentSegment.appendRange(activeStart, i, sampleRate)
			activeStart = -1
		}

		if isSilence && currentSegment.sampleCount > minSegmentSamples {
			currentSegment.endTime = currentTime
			spans = append(spans, currentSegment)
			currentSegment = audioSegmentSpan{}
			currentSegment.setStartTime(currentTime + b.config.SilenceThreshold)
		} else if !isSilence {
			if activeStart < 0 {
				activeStart = i
			}
		}
	}

	if activeStart >= 0 {
		currentSegment.appendRange(activeStart, len(audioData), sampleRate)
	}
	if currentSegment.sampleCount > minSegmentSamples {
		currentSegment.endTime = float64(len(audioData)) / float64(sampleRate)
		spans = append(spans, currentSegment)
	}
	segments := materializeAudioSegments(audioData, spans)
	return segments, nil
}

func (b *basicBackend) extractEmbeddings(ctx context.Context, segments []AudioSegment, sampleRate int) ([][]float32, error) {
	if b.embeddingExtractor == nil {
		return nil, ErrEmbeddingExtractorUnavailable
	}

	if cap(b.scratchEmbeddings) < len(segments) {
		b.scratchEmbeddings = make([][]float32, len(segments))
	} else {
		b.scratchEmbeddings = b.scratchEmbeddings[:len(segments)]
	}
	embeddings := b.scratchEmbeddings

	for i := range segments {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		embedding, err := b.embeddingExtractor.ExtractEmbedding(ctx, segments[i].Samples, sampleRate)
		if err != nil {
			return nil, fmt.Errorf("segment %d embedding extraction failed: %w", i, err)
		}
		if len(embedding) == 0 {
			return nil, fmt.Errorf("segment %d embedding extraction returned empty embedding", i)
		}
		embeddings[i] = embedding
	}

	result := make([][]float32, len(segments))
	for i, emb := range embeddings {
		result[i] = make([]float32, len(emb))
		copy(result[i], emb)
	}
	return result, nil
}

func (b *basicBackend) clusterAndAssignSpeakers(embeddings [][]float32, segments []AudioSegment) ([]SpeakerSegment, error) {
	if len(embeddings) == 0 {
		return []SpeakerSegment{}, nil
	}

	clusters := b.performClustering(embeddings)
	speakerSegments := make([]SpeakerSegment, len(segments))
	for i, segment := range segments {
		clusterID := clusters[i]
		speakerSegments[i] = SpeakerSegment{
			SpeakerID:  fmt.Sprintf("speaker_%d", clusterID+1),
			StartTime:  segment.StartTime,
			EndTime:    segment.EndTime,
			Duration:   segment.EndTime - segment.StartTime,
			Confidence: 0,
			Embedding:  embeddings[i],
		}
	}
	return speakerSegments, nil
}

func (b *basicBackend) performClustering(embeddings [][]float32) []int {
	n := len(embeddings)
	if n == 0 {
		return []int{}
	}

	b.initializeUnionFind(n)
	threshold := float32(b.config.SimilarityThreshold)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if b.find(i) == b.find(j) {
				continue
			}
			if calculateEmbeddingSimilarity(embeddings[i], embeddings[j]) > threshold {
				b.union(i, j)
			}
		}
	}
	return b.extractClusters(n)
}

func (b *basicBackend) initializeUnionFind(size int) {
	if cap(b.ufParent) < size {
		b.ufParent = make([]int, size)
		b.ufRank = make([]int, size)
	} else {
		b.ufParent = b.ufParent[:size]
		b.ufRank = b.ufRank[:size]
	}
	for i := 0; i < size; i++ {
		b.ufParent[i] = i
		b.ufRank[i] = 0
	}
}

// find returns the set representative for x, applying path compression. It is
// iterative (two passes) rather than recursive so that a pathological chain of
// segments cannot overflow the goroutine stack.
func (b *basicBackend) find(x int) int {
	root := x
	for b.ufParent[root] != root {
		root = b.ufParent[root]
	}
	for b.ufParent[x] != root {
		b.ufParent[x], x = root, b.ufParent[x]
	}
	return root
}

func (b *basicBackend) union(x, y int) {
	rootX := b.find(x)
	rootY := b.find(y)
	if rootX == rootY {
		return
	}
	if b.ufRank[rootX] < b.ufRank[rootY] {
		b.ufParent[rootX] = rootY
	} else if b.ufRank[rootX] > b.ufRank[rootY] {
		b.ufParent[rootY] = rootX
	} else {
		b.ufParent[rootY] = rootX
		b.ufRank[rootX]++
	}
}

func (b *basicBackend) extractClusters(n int) []int {
	clusters := make([]int, n)
	clusterMap := make(map[int]int)
	nextClusterID := 0

	for i := 0; i < n; i++ {
		root := b.find(i)
		if _, exists := clusterMap[root]; !exists {
			clusterMap[root] = nextClusterID
			nextClusterID++
		}
		clusters[i] = clusterMap[root]
	}
	return clusters
}

func calculateEmbeddingSimilarity(emb1, emb2 []float32) float32 {
	if len(emb1) != len(emb2) {
		return 0
	}

	var dotProduct, norm1, norm2 float32
	for i := range emb1 {
		dotProduct += emb1[i] * emb2[i]
		norm1 += emb1[i] * emb1[i]
		norm2 += emb2[i] * emb2[i]
	}
	if norm1 == 0 || norm2 == 0 {
		return 0
	}
	return dotProduct / (float32(math.Sqrt(float64(norm1))) * float32(math.Sqrt(float64(norm2))))
}

func countUniqueSpeakers(segments []SpeakerSegment) int {
	speakerMap := make(map[string]bool)
	for _, segment := range segments {
		speakerMap[segment.SpeakerID] = true
	}
	return len(speakerMap)
}
