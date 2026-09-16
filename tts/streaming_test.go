package tts

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.glpx.pro/voicekit/types"
)

// fakeStreamingSynthesizer is a hermetic stand-in for the native Sherpa
// binding. It emits chunkSamples in order via cb, honoring the callback's
// stop signal exactly like the real GenerateWithProgressCallback would (a
// false return truncates the generated audio to what was already produced).
type fakeStreamingSynthesizer struct {
	mu           sync.Mutex
	chunkSamples [][]float32
	sampleRate   int
	numSpeakers  int
	closeCount   int
	genErr       error

	// delayBeforeChunk lets a test synchronize with an external cancel.
	delayBeforeChunk time.Duration
}

func (f *fakeStreamingSynthesizer) GenerateWithCallback(text string, speakerID int, speed float32, cb chunkCallback) (*generatedAudio, error) {
	if f.genErr != nil {
		return nil, f.genErr
	}
	var all []float32
	for i, chunk := range f.chunkSamples {
		// The delay never applies to the first chunk, so tests can prove a
		// mid-stream cancellation lands after at least one chunk was
		// already delivered rather than before synthesis even starts.
		if i > 0 && f.delayBeforeChunk > 0 {
			time.Sleep(f.delayBeforeChunk)
		}
		progress := float32(len(all)+len(chunk)) / float32(totalLen(f.chunkSamples))
		if !cb(chunk, progress) {
			// Mirrors the native binding: a false return from the callback
			// truncates generation to what was produced so far.
			all = append(all, chunk...)
			return &generatedAudio{Samples: all, SampleRate: f.sampleRate}, nil
		}
		all = append(all, chunk...)
	}
	return &generatedAudio{Samples: all, SampleRate: f.sampleRate}, nil
}

func (f *fakeStreamingSynthesizer) NumSpeakers() int { return f.numSpeakers }
func (f *fakeStreamingSynthesizer) SampleRate() int  { return f.sampleRate }
func (f *fakeStreamingSynthesizer) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeCount++
	return nil
}

func totalLen(chunks [][]float32) int {
	n := 0
	for _, c := range chunks {
		n += len(c)
	}
	return n
}

func TestSherpaStreamingSynthesizerDeliversChunksInOrder(t *testing.T) {
	fake := &fakeStreamingSynthesizer{
		chunkSamples: [][]float32{
			{0.1, 0.2},
			{0.3, 0.4},
			{0.5},
		},
		sampleRate:  16000,
		numSpeakers: 1,
	}
	synth := &SherpaStreamingSynthesizer{
		config:      &Config{Speed: 1},
		synthesizer: fake,
	}

	var received [][]float32
	var progresses []float32
	result, err := synth.SynthesizeStream(context.Background(), types.SynthesisRequest{Text: "hello"}, func(chunk types.AudioChunk) bool {
		received = append(received, chunk.Samples)
		progresses = append(progresses, chunk.Progress)
		if chunk.SampleRate != 16000 {
			t.Errorf("unexpected chunk sample rate %d", chunk.SampleRate)
		}
		return true
	})
	if err != nil {
		t.Fatalf("SynthesizeStream failed: %v", err)
	}
	if len(received) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(received))
	}
	for i, want := range fake.chunkSamples {
		if len(received[i]) != len(want) {
			t.Fatalf("chunk %d length mismatch: got %d want %d", i, len(received[i]), len(want))
		}
		for j := range want {
			if received[i][j] != want[j] {
				t.Fatalf("chunk %d sample %d mismatch: got %f want %f", i, j, received[i][j], want[j])
			}
		}
	}
	// Progress must be monotonically non-decreasing and reach 1.0 on the
	// last chunk.
	for i := 1; i < len(progresses); i++ {
		if progresses[i] < progresses[i-1] {
			t.Fatalf("progress decreased at chunk %d: %v", i, progresses)
		}
	}
	if progresses[len(progresses)-1] != 1 {
		t.Fatalf("expected final progress of 1, got %f", progresses[len(progresses)-1])
	}

	if result == nil {
		t.Fatal("expected assembled speech result")
	}
	if len(result.Samples) != 5 {
		t.Fatalf("expected 5 assembled samples, got %d", len(result.Samples))
	}
	if result.SampleRate != 16000 {
		t.Fatalf("unexpected result sample rate %d", result.SampleRate)
	}
}

func TestSherpaStreamingSynthesizerSinkFalseStopsEarly(t *testing.T) {
	fake := &fakeStreamingSynthesizer{
		chunkSamples: [][]float32{
			{0.1, 0.2},
			{0.3, 0.4},
			{0.5},
		},
		sampleRate: 8000,
	}
	synth := &SherpaStreamingSynthesizer{
		config:      &Config{Speed: 1},
		synthesizer: fake,
	}

	var received int
	result, err := synth.SynthesizeStream(context.Background(), types.SynthesisRequest{Text: "hello"}, func(chunk types.AudioChunk) bool {
		received++
		return received < 2 // stop after the second chunk
	})
	if err != nil {
		t.Fatalf("expected no error on sink-initiated stop, got: %v", err)
	}
	if received != 2 {
		t.Fatalf("expected exactly 2 chunks delivered before stop, got %d", received)
	}
	if result == nil {
		t.Fatal("expected partial assembled speech result")
	}
	if len(result.Samples) != 4 {
		t.Fatalf("expected 4 assembled samples (first two chunks), got %d", len(result.Samples))
	}
}

func TestSherpaStreamingSynthesizerCancelledContextAborts(t *testing.T) {
	fake := &fakeStreamingSynthesizer{
		chunkSamples: [][]float32{
			{0.1, 0.2},
			{0.3, 0.4},
			{0.5},
		},
		sampleRate:       8000,
		delayBeforeChunk: 20 * time.Millisecond,
	}
	synth := &SherpaStreamingSynthesizer{
		config:      &Config{Speed: 1},
		synthesizer: fake,
	}

	ctx, cancel := context.WithCancel(context.Background())
	var received int
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	result, err := synth.SynthesizeStream(ctx, types.SynthesisRequest{Text: "hello"}, func(chunk types.AudioChunk) bool {
		received++
		return true
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result on cancellation, got: %+v", result)
	}
	if received == 0 {
		t.Fatal("expected at least one chunk delivered before cancellation landed")
	}
	if received >= len(fake.chunkSamples) {
		t.Fatalf("expected synthesis to stop before all chunks were delivered, got %d", received)
	}
}

func TestSherpaStreamingSynthesizerRejectsAlreadyCancelledContext(t *testing.T) {
	fake := &fakeStreamingSynthesizer{
		chunkSamples: [][]float32{{0.1}},
		sampleRate:   8000,
	}
	synth := &SherpaStreamingSynthesizer{
		config:      &Config{Speed: 1},
		synthesizer: fake,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	_, err := synth.SynthesizeStream(ctx, types.SynthesisRequest{Text: "hello"}, func(chunk types.AudioChunk) bool {
		called = true
		return true
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if called {
		t.Fatal("sink should not be invoked for an already-canceled context")
	}
}

func TestSherpaStreamingSynthesizerRejectsNilSink(t *testing.T) {
	synth := &SherpaStreamingSynthesizer{
		config:      &Config{Speed: 1},
		synthesizer: &fakeStreamingSynthesizer{chunkSamples: [][]float32{{0.1}}, sampleRate: 8000},
	}
	if _, err := synth.SynthesizeStream(context.Background(), types.SynthesisRequest{Text: "hello"}, nil); err == nil {
		t.Fatal("nil sink should fail before native calls")
	}
}

func TestSherpaStreamingSynthesizerRejectsEmptyText(t *testing.T) {
	synth := &SherpaStreamingSynthesizer{
		config:      &Config{Speed: 1},
		synthesizer: &fakeStreamingSynthesizer{chunkSamples: [][]float32{{0.1}}, sampleRate: 8000},
	}
	if _, err := synth.SynthesizeStream(context.Background(), types.SynthesisRequest{Text: "   "}, func(types.AudioChunk) bool { return true }); err == nil {
		t.Fatal("empty text should fail before native calls")
	}
}

func TestSherpaStreamingSynthesizerRejectsUnknownSpeaker(t *testing.T) {
	synth := &SherpaStreamingSynthesizer{
		config:      &Config{Speed: 1},
		synthesizer: &fakeStreamingSynthesizer{chunkSamples: [][]float32{{0.1}}, sampleRate: 8000, numSpeakers: 1},
	}
	if _, err := synth.SynthesizeStream(context.Background(), types.SynthesisRequest{Text: "hello", SpeakerID: 1}, func(types.AudioChunk) bool { return true }); err == nil {
		t.Fatal("speaker ID beyond native speaker count should fail")
	}
}

func TestSherpaStreamingSynthesizerCloseIdempotent(t *testing.T) {
	fake := &fakeStreamingSynthesizer{}
	synth := &SherpaStreamingSynthesizer{config: &Config{Speed: 1}, synthesizer: fake}

	if err := synth.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := synth.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
	if fake.closeCount != 1 {
		t.Fatalf("expected one native close, got %d", fake.closeCount)
	}
}

func TestSherpaStreamingSynthesizerClosedRejectsCalls(t *testing.T) {
	synth := &SherpaStreamingSynthesizer{
		config:      &Config{Speed: 1},
		synthesizer: &fakeStreamingSynthesizer{chunkSamples: [][]float32{{0.1}}, sampleRate: 8000},
	}
	if err := synth.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if _, err := synth.SynthesizeStream(context.Background(), types.SynthesisRequest{Text: "hello"}, func(types.AudioChunk) bool { return true }); err == nil {
		t.Fatal("expected error after close")
	}
}
