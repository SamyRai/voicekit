package asr

import (
	"context"
	"sync"
	"testing"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
	"go.glpx.pro/gdk/voice/types"
)

func TestKeywordSpotterConfigValidateRequiresTokensAndKeywords(t *testing.T) {
	config := DefaultKeywordSpotterConfig()
	if err := config.Validate(); err == nil {
		t.Fatal("expected validation error without tokens/model/keywords")
	}
}

func TestKeywordSpotterConfigValidateAcceptsInlineKeywords(t *testing.T) {
	config := DefaultKeywordSpotterConfig()
	config.TokensPath = tempModelFile(t, "tokens.txt")
	config.ModelPath = tempModelFile(t, "model.onnx")
	config.Keywords = []string{"hey computer"}

	if err := config.Validate(); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
}

func TestKeywordSpotterConfigValidateRequiresTransducerTriple(t *testing.T) {
	config := DefaultKeywordSpotterConfig()
	config.TokensPath = tempModelFile(t, "tokens.txt")
	config.EncoderPath = tempModelFile(t, "encoder.onnx")
	config.Keywords = []string{"hey computer"}

	if err := config.Validate(); err == nil {
		t.Fatal("expected validation error for incomplete transducer path set")
	}
}

func TestBuildKeywordSpotterConfigMapsInlineKeywords(t *testing.T) {
	config := DefaultKeywordSpotterConfig()
	config.TokensPath = tempModelFile(t, "tokens.txt")
	config.EncoderPath = tempModelFile(t, "encoder.onnx")
	config.DecoderPath = tempModelFile(t, "decoder.onnx")
	config.JoinerPath = tempModelFile(t, "joiner.onnx")
	config.Keywords = []string{"hey computer", "stop listening"}

	if err := config.Validate(); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}

	native, err := buildKeywordSpotterConfig(&config)
	if err != nil {
		t.Fatalf("failed to build native config: %v", err)
	}
	if native.ModelConfig.Transducer.Encoder != config.EncoderPath {
		t.Fatalf("encoder path not mapped")
	}
	wantBuf := "hey computer\nstop listening"
	if native.KeywordsBuf != wantBuf {
		t.Fatalf("expected keywords buf %q, got %q", wantBuf, native.KeywordsBuf)
	}
	if native.KeywordsBufSize != len(wantBuf) {
		t.Fatalf("expected keywords buf size %d, got %d", len(wantBuf), native.KeywordsBufSize)
	}
}

func TestSherpaKeywordSpotterReportsKeywordOnFire(t *testing.T) {
	recognizer := &fakeKeywordRecognizer{}
	spotter := newSherpaKeywordSpotterForTest(recognizer)
	defer spotter.Close()

	// First chunk creates the session's native stream with no result queued.
	if _, err := spotter.Spot(context.Background(), "session-a", []float32{0.1, 0.2}); err != nil {
		t.Fatalf("first spot failed: %v", err)
	}
	if recognizer.newStreamCount != 1 {
		t.Fatalf("expected one native stream, got %d", recognizer.newStreamCount)
	}

	// Queue a keyword on that session's stream, then feed the next chunk.
	recognizer.streams[0].nextKeyword = "hey computer"
	match, err := spotter.Spot(context.Background(), "session-a", []float32{0.3, 0.4})
	if err != nil {
		t.Fatalf("second spot failed: %v", err)
	}
	if match == nil {
		t.Fatal("expected a keyword match")
	}
	if match.Keyword != "hey computer" {
		t.Fatalf("unexpected keyword %q", match.Keyword)
	}
	if recognizer.newStreamCount != 1 {
		t.Fatalf("expected the session to reuse its native stream, got %d streams", recognizer.newStreamCount)
	}
	if recognizer.streams[0].resetCount != 1 {
		t.Fatalf("expected reset after detection, got %d", recognizer.streams[0].resetCount)
	}
}

func TestSherpaKeywordSpotterReturnsNilWhenNoKeywordFires(t *testing.T) {
	recognizer := &fakeKeywordRecognizer{}
	spotter := newSherpaKeywordSpotterForTest(recognizer)
	defer spotter.Close()

	match, err := spotter.Spot(context.Background(), "session-a", []float32{0.1, 0.2})
	if err != nil {
		t.Fatalf("spot failed: %v", err)
	}
	if match != nil {
		t.Fatalf("expected nil match, got %+v", match)
	}
	if recognizer.streams[0].resetCount != 0 {
		t.Fatalf("reset must not be called without a detection, got %d", recognizer.streams[0].resetCount)
	}
}

func TestSherpaKeywordSpotterPerSessionIsolation(t *testing.T) {
	recognizer := &fakeKeywordRecognizer{}
	spotter := newSherpaKeywordSpotterForTest(recognizer)
	defer spotter.Close()

	if _, err := spotter.Spot(context.Background(), "session-a", []float32{0.1}); err != nil {
		t.Fatalf("session-a spot failed: %v", err)
	}
	if _, err := spotter.Spot(context.Background(), "session-b", []float32{0.2}); err != nil {
		t.Fatalf("session-b spot failed: %v", err)
	}
	if _, err := spotter.Spot(context.Background(), "session-a", []float32{0.3}); err != nil {
		t.Fatalf("session-a second spot failed: %v", err)
	}

	if recognizer.newStreamCount != 2 {
		t.Fatalf("expected two native streams (one per session), got %d", recognizer.newStreamCount)
	}
	// Streams are created in first-seen order: index 0 is session-a, index 1
	// is session-b.
	if recognizer.streams[0].acceptCount != 2 {
		t.Fatalf("expected session-a stream to receive two chunks, got %d", recognizer.streams[0].acceptCount)
	}
	if recognizer.streams[1].acceptCount != 1 {
		t.Fatalf("expected session-b stream to receive one chunk, got %d", recognizer.streams[1].acceptCount)
	}
}

func TestSherpaKeywordSpotterCloseClosesAllSessionStreams(t *testing.T) {
	recognizer := &fakeKeywordRecognizer{}
	spotter := newSherpaKeywordSpotterForTest(recognizer)

	if _, err := spotter.Spot(context.Background(), "session-a", []float32{0.1}); err != nil {
		t.Fatalf("session-a spot failed: %v", err)
	}
	if _, err := spotter.Spot(context.Background(), "session-b", []float32{0.2}); err != nil {
		t.Fatalf("session-b spot failed: %v", err)
	}

	if err := spotter.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if recognizer.streams[0].closeCount != 1 {
		t.Fatal("expected session-a stream closed on spotter Close")
	}
	if recognizer.streams[1].closeCount != 1 {
		t.Fatal("expected session-b stream closed on spotter Close")
	}
	if recognizer.closeCount != 1 {
		t.Fatalf("expected native spotter closed once, got %d", recognizer.closeCount)
	}

	// Close is idempotent.
	if err := spotter.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
	if recognizer.closeCount != 1 {
		t.Fatalf("expected native spotter close count to stay 1, got %d", recognizer.closeCount)
	}
}

func TestSherpaKeywordSpotterEndSession(t *testing.T) {
	recognizer := &fakeKeywordRecognizer{}
	spotter := newSherpaKeywordSpotterForTest(recognizer)
	defer spotter.Close()

	if _, err := spotter.Spot(context.Background(), "session-a", []float32{0.1}); err != nil {
		t.Fatalf("session-a spot failed: %v", err)
	}

	// Ending the session closes and removes its native stream.
	if err := spotter.EndSession("session-a"); err != nil {
		t.Fatalf("EndSession failed: %v", err)
	}
	if recognizer.streams[0].closeCount != 1 {
		t.Fatalf("expected session-a stream closed on EndSession, got %d", recognizer.streams[0].closeCount)
	}

	// A later Spot with the same sessionID starts a fresh stream (no leak of the
	// old one; not a reuse of the closed one).
	if _, err := spotter.Spot(context.Background(), "session-a", []float32{0.2}); err != nil {
		t.Fatalf("session-a re-spot failed: %v", err)
	}
	if recognizer.newStreamCount != 2 {
		t.Fatalf("expected a fresh stream after EndSession, got %d total", recognizer.newStreamCount)
	}

	// Ending an unknown session is a no-op.
	if err := spotter.EndSession("never-seen"); err != nil {
		t.Fatalf("EndSession on unknown session should be a no-op, got %v", err)
	}
}

func TestSherpaKeywordSpotterRejectsEmptyAudioAndSessionID(t *testing.T) {
	recognizer := &fakeKeywordRecognizer{}
	spotter := newSherpaKeywordSpotterForTest(recognizer)
	defer spotter.Close()

	if _, err := spotter.Spot(context.Background(), "session-a", nil); err == nil {
		t.Fatal("expected error for empty audio")
	}
	if _, err := spotter.Spot(context.Background(), "", []float32{0.1}); err == nil {
		t.Fatal("expected error for empty session ID")
	}
}

func TestSherpaKeywordSpotterSpotAfterCloseFails(t *testing.T) {
	recognizer := &fakeKeywordRecognizer{}
	spotter := newSherpaKeywordSpotterForTest(recognizer)

	if err := spotter.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if _, err := spotter.Spot(context.Background(), "session-a", []float32{0.1}); err == nil {
		t.Fatal("expected error spotting on closed spotter")
	}
}

// TestSherpaKeywordSpotterConcurrentSessionsRace exercises concurrent Spot
// calls across distinct sessions plus a subsequent Close, guarding against
// data races on the shared sessions map and native seam (run with -race).
func TestSherpaKeywordSpotterConcurrentSessionsRace(t *testing.T) {
	recognizer := &fakeKeywordRecognizer{}
	spotter := newSherpaKeywordSpotterForTest(recognizer)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sessionID := "session-" + string(rune('a'+n))
			for j := 0; j < 20; j++ {
				_, _ = spotter.Spot(context.Background(), sessionID, []float32{0.1, 0.2})
			}
		}(i)
	}

	wg.Wait()
	if err := spotter.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

var _ types.KeywordSpotter = (*SherpaKeywordSpotter)(nil)

func newSherpaKeywordSpotterForTest(recognizer *fakeKeywordRecognizer) *SherpaKeywordSpotter {
	return &SherpaKeywordSpotter{
		sampleRate: 16000,
		spotter:    recognizer,
		sessions:   make(map[string]*keywordSession),
	}
}

// fakeKeywordRecognizer is a hermetic stand-in for the native Sherpa
// KeywordSpotter, mirroring fakeOnlineRecognizer's shape in service_test.go.
// Streams are recorded in creation order in streams, so a test can index
// into streams[0], streams[1], ... in the order sessions are first seen by
// the spotter under test.
type fakeKeywordRecognizer struct {
	mu             sync.Mutex
	streams        []*fakeKeywordStream
	newStreamCount int
	closeCount     int
}

func (r *fakeKeywordRecognizer) NewStream() (keywordStream, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.newStreamCount++
	stream := &fakeKeywordStream{}
	r.streams = append(r.streams, stream)
	return stream, nil
}

func (r *fakeKeywordRecognizer) Decode(stream keywordStream) error {
	stream.(*fakeKeywordStream).decodeCount++
	return nil
}

func (r *fakeKeywordRecognizer) IsReady(stream keywordStream) bool {
	fs := stream.(*fakeKeywordStream)
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.readyOnce {
		return false
	}
	fs.readyOnce = true
	return true
}

func (r *fakeKeywordRecognizer) Reset(stream keywordStream) error {
	fs := stream.(*fakeKeywordStream)
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.resetCount++
	fs.readyOnce = false
	return nil
}

func (r *fakeKeywordRecognizer) Result(stream keywordStream) (*sherpa.KeywordSpotterResult, error) {
	fs := stream.(*fakeKeywordStream)
	fs.mu.Lock()
	defer fs.mu.Unlock()
	keyword := fs.nextKeyword
	// A real keyword-spotter result reflects state "since the last Reset",
	// so a queued keyword only fires once per Reset.
	fs.nextKeyword = ""
	return &sherpa.KeywordSpotterResult{Keyword: keyword}, nil
}

func (r *fakeKeywordRecognizer) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeCount++
	return nil
}

type fakeKeywordStream struct {
	mu          sync.Mutex
	acceptCount int
	decodeCount int
	resetCount  int
	closeCount  int
	readyOnce   bool
	nextKeyword string
}

func (s *fakeKeywordStream) AcceptWaveform(sampleRate int, samples []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acceptCount++
	return nil
}

func (s *fakeKeywordStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeCount++
	return nil
}
