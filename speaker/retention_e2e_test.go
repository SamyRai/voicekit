package speaker

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.glpx.pro/gdk/voice/speaker/application"
	"go.glpx.pro/gdk/voice/speaker/domain"
)

// newRetentionTestManager builds a fully wired *Manager over a hermetic,
// temp-dir Pebble database for retention end-to-end tests. It intentionally
// avoids NewManager (which requires a real native speaker embedding model
// via Config.ModelPath): the extractor and vector index are swapped for
// deterministic in-memory fakes, but persistence (ShardedSpeakerDatabase),
// the domain service, and the application use cases are the real production
// types, so retention/eviction runs through the real code path.
func newRetentionTestManager(t *testing.T, maxSpeakers int, policy RetentionPolicy, maxAge time.Duration) *Manager {
	t.Helper()

	shardedDB, err := NewShardedSpeakerDatabase(t.TempDir(), 2)
	if err != nil {
		t.Fatalf("failed to create sharded speaker database: %v", err)
	}
	t.Cleanup(func() {
		if err := shardedDB.Close(); err != nil {
			t.Errorf("failed to close speaker database: %v", err)
		}
	})

	repository := NewSpeakerDatabaseAdapter(shardedDB)
	extractor := &identityEmbeddingExtractor{}
	vectorIndex := newFakeVectorIndex()
	similarity := NewCosineSimilarityCalculator()
	threshold, err := domain.NewSimilarityScore(0.9)
	if err != nil {
		t.Fatalf("failed to build similarity threshold: %v", err)
	}

	speakerService := domain.NewSpeakerService(repository, extractor, vectorIndex, similarity, threshold)
	logger := &NoOpLogger{}
	metrics := &application.NoOpMetricsRecorder{}

	return &Manager{
		recognitionUseCase: application.NewSpeakerRecognitionUseCase(*speakerService, logger, metrics),
		managementUseCase:  application.NewSpeakerManagementUseCase(*speakerService, logger, metrics),
		database:           shardedDB,
		vectorIndex:        vectorIndex,
		threshold:          0.9,
		dataDir:            "",
		logger:             logger,
		maxSpeakers:        maxSpeakers,
		retentionPolicy:    policy,
		maxAge:             maxAge,
	}
}

// identityEmbeddingExtractor treats the supplied "audio" as the embedding
// itself, so tests can drive deterministic identify/verify matches without a
// native model.
type identityEmbeddingExtractor struct{}

func (e *identityEmbeddingExtractor) ExtractEmbedding(_ context.Context, audioData []float32, _ int) (domain.SpeakerEmbedding, error) {
	return domain.NewSpeakerEmbedding(audioData)
}
func (e *identityEmbeddingExtractor) Dimension() int { return 0 }
func (e *identityEmbeddingExtractor) IsReady() bool  { return true }
func (e *identityEmbeddingExtractor) Close() error   { return nil }

// fakeVectorIndex is a minimal in-memory domain.VectorIndex backed by exact
// cosine similarity, used so tests get deterministic identify/verify
// matches without depending on the HNSW approximate index.
type fakeVectorIndex struct {
	mu       sync.Mutex
	speakers map[domain.SpeakerID]*domain.Speaker
}

func newFakeVectorIndex() *fakeVectorIndex {
	return &fakeVectorIndex{speakers: make(map[domain.SpeakerID]*domain.Speaker)}
}

func (f *fakeVectorIndex) AddSpeaker(_ context.Context, speaker *domain.Speaker) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.speakers[speaker.ID()] = speaker
	return nil
}

func (f *fakeVectorIndex) RemoveSpeaker(_ context.Context, id domain.SpeakerID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.speakers, id)
	return nil
}

func (f *fakeVectorIndex) SearchSimilar(_ context.Context, embedding domain.SpeakerEmbedding, threshold domain.SimilarityScore) (domain.SpeakerID, domain.SimilarityScore, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	calc := NewCosineSimilarityCalculator()
	var bestID domain.SpeakerID
	best := domain.MinSimilarityScore
	for id, sp := range f.speakers {
		score := calc.CalculateSimilarity(embedding, sp.Embeddings())
		if score.Float32() > best.Float32() {
			best = score
			bestID = id
		}
	}
	if bestID == "" || !best.IsAboveThreshold(threshold) {
		return "", domain.MinSimilarityScore, nil
	}
	return bestID, best, nil
}

func (f *fakeVectorIndex) GetSpeaker(_ context.Context, id domain.SpeakerID) (*domain.Speaker, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	sp, ok := f.speakers[id]
	if !ok {
		return nil, fmt.Errorf("speaker %s not found in fake vector index", id)
	}
	return sp, nil
}

func (f *fakeVectorIndex) Size(_ context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.speakers), nil
}

func (f *fakeVectorIndex) Close() error { return nil }

// TestRetentionE2EFIFOEvictsOldestOnRegister proves that registering beyond
// MaxSpeakers evicts the oldest speaker (by CreatedAt) end-to-end: through
// RegisterSpeakerContext -> enforceRetention -> DeleteSpeakerContext, backed
// by a real Pebble-backed database.
func TestRetentionE2EFIFOEvictsOldestOnRegister(t *testing.T) {
	manager := newRetentionTestManager(t, 2, RetentionFIFO, 0)
	ctx := context.Background()

	mustRegister(t, manager, ctx, "a", []float32{1, 0, 0})
	time.Sleep(2 * time.Millisecond) // ensure distinct CreatedAt across registrations
	mustRegister(t, manager, ctx, "b", []float32{0, 1, 0})
	time.Sleep(2 * time.Millisecond)
	mustRegister(t, manager, ctx, "c", []float32{0, 0, 1}) // exceeds cap of 2, should evict "a"

	speakers := manager.GetAllSpeakersContext(ctx)
	if len(speakers) != 2 {
		t.Fatalf("expected 2 speakers after eviction, got %d: %+v", len(speakers), speakers)
	}
	ids := map[string]bool{}
	for _, sp := range speakers {
		ids[sp.ID] = true
	}
	if ids["a"] {
		t.Fatalf("expected oldest speaker 'a' to be evicted, got %v", ids)
	}
	if !ids["b"] || !ids["c"] {
		t.Fatalf("expected 'b' and 'c' to survive, got %v", ids)
	}

	if _, err := manager.database.GetSpeaker("a"); err == nil {
		t.Fatalf("expected evicted speaker 'a' to be removed from the database")
	}
}

// TestRetentionE2ELRUTouchChangesVictim proves that an IdentifySpeaker match
// under RetentionLRU refreshes LastUsedAt, which changes which speaker is
// evicted compared to plain CreatedAt (FIFO) order.
func TestRetentionE2ELRUTouchChangesVictim(t *testing.T) {
	manager := newRetentionTestManager(t, 2, RetentionLRU, 0)
	ctx := context.Background()

	aEmbedding := []float32{1, 0, 0}
	mustRegister(t, manager, ctx, "a", aEmbedding) // oldest CreatedAt
	time.Sleep(2 * time.Millisecond)
	mustRegister(t, manager, ctx, "b", []float32{0, 1, 0}) // at cap now (2), no eviction yet

	time.Sleep(2 * time.Millisecond)
	// Touch "a" via a successful identify so its LastUsedAt becomes the most
	// recent of {a, b}, even though "a" has the oldest CreatedAt.
	result, err := manager.IdentifySpeakerContext(ctx, aEmbedding, 16000)
	if err != nil {
		t.Fatalf("IdentifySpeakerContext failed: %v", err)
	}
	if !result.Identified || result.SpeakerID != "a" {
		t.Fatalf("expected to identify speaker 'a', got %+v", result)
	}

	time.Sleep(2 * time.Millisecond)
	mustRegister(t, manager, ctx, "c", []float32{0, 0, 1}) // exceeds cap of 2

	speakers := manager.GetAllSpeakersContext(ctx)
	ids := map[string]bool{}
	for _, sp := range speakers {
		ids[sp.ID] = true
	}

	// Under plain FIFO this would have evicted "a" (oldest CreatedAt). Since
	// "a" was just touched under RetentionLRU, "b" (never touched since
	// registration) is the least-recently-used and gets evicted instead.
	if ids["b"] {
		t.Fatalf("expected untouched speaker 'b' to be evicted under LRU, got %v", ids)
	}
	if !ids["a"] || !ids["c"] {
		t.Fatalf("expected 'a' (touched) and 'c' (newest) to survive, got %v", ids)
	}
}

func mustRegister(t *testing.T, manager *Manager, ctx context.Context, id string, embedding []float32) {
	t.Helper()
	if err := manager.RegisterSpeakerContext(ctx, id, id, embedding, 16000); err != nil {
		t.Fatalf("failed to register speaker %s: %v", id, err)
	}
}
