package asr

import (
	"context"
	"sync"
	"testing"

	"go.glpx.pro/gdk/voice/types"
)

// TestSherpaOnlineModelConcurrentProcessAndCloseIsRaceFree drives many concurrent
// ProcessAudio/FinishAudio calls against the online model while another goroutine
// closes it. Every native-handle interaction (stream creation, accept, decode,
// reset, close) happens under the model's mutex, so this must pass cleanly under
// `go test -race`. It fails if the lock guarding the recognizer/stream lifecycle
// is removed or narrowed — the exact regression that lets a session close race an
// in-flight native call.
func TestSherpaOnlineModelConcurrentProcessAndCloseIsRaceFree(t *testing.T) {
	recognizer := &fakeOnlineRecognizer{}
	model := newSherpaOnlineModelForTest(recognizer)

	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers + 1)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			// Each worker owns its own streaming state (its own native session).
			state := &types.StreamingState{Language: "en"}
			for j := 0; j < 25; j++ {
				// Errors once the model is closed are expected; we only assert the
				// absence of data races and crashes here.
				_, _ = model.ProcessAudio(context.Background(), []float32{0.1, 0.2}, state)
			}
			_, _ = model.FinishAudio(context.Background(), []float32{0.3}, state)
		}()
	}

	go func() {
		defer wg.Done()
		_ = model.Close()
	}()

	wg.Wait()

	// Close must stay idempotent after the concurrent churn.
	if err := model.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}
