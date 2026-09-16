package asr

import (
	"context"
	"testing"

	"go.glpx.pro/voicekit/types"
)

// asrStateModel is a fake model that installs a closeable value into
// state.ASRState the way a real SherpaOnlineModel installs a native stream, so a
// test can assert that switching a session's model closes the previous model's
// stream instead of handing it to another recognizer.
type asrStateModel struct {
	testModel
	stream       *countingCloser
	processCalls int
}

func (m *asrStateModel) SupportsLanguage(lang string) bool { return m.language == lang }

func (m *asrStateModel) ProcessAudio(ctx context.Context, audio []float32, state *types.StreamingState) (*types.Transcription, error) {
	m.processCalls++
	if state != nil && state.ASRState == nil {
		state.ASRState = m.stream
	}
	return m.testModel.ProcessAudio(ctx, audio, state)
}

// TestSessionModelBindingResetsNativeStreamOnLanguageSwitch proves that when a
// session is re-routed to a different model (via SetSessionLanguage), the
// previous model's native stream is closed and cleared rather than reused by the
// new recognizer — the cross-recognizer native-stream hazard multi-model hosting
// would otherwise introduce.
func TestSessionModelBindingResetsNativeStreamOnLanguageSwitch(t *testing.T) {
	service := newTestService()
	defer service.Close()

	streamA := &countingCloser{}
	modelA := &asrStateModel{testModel: testModel{name: "en_model", language: "en"}, stream: streamA}
	modelB := &asrStateModel{testModel: testModel{name: "es_model", language: "es"}, stream: &countingCloser{}}
	service.config.DefaultModel = "en_model"
	if err := service.RegisterModel(modelA); err != nil {
		t.Fatalf("register en model: %v", err)
	}
	if err := service.RegisterModel(modelB); err != nil {
		t.Fatalf("register es model: %v", err)
	}

	ctx := context.Background()
	audio := make([]float32, service.config.ChunkSize)
	sid := "switch-session"

	// First chunk routes to the en model, which installs its native stream.
	if _, err := service.ProcessAudioChunk(ctx, sid, audio); err != nil {
		t.Fatalf("first chunk failed: %v", err)
	}
	if modelA.processCalls == 0 {
		t.Fatal("expected en model to process the first chunk")
	}

	// Switch language: the next chunk must close the en model's stream and route
	// to the es model with a fresh stream.
	if err := service.SetSessionLanguage(sid, "es"); err != nil {
		t.Fatalf("set session language: %v", err)
	}
	if _, err := service.ProcessAudioChunk(ctx, sid, audio); err != nil {
		t.Fatalf("second chunk failed: %v", err)
	}

	if streamA.closeCount != 1 {
		t.Fatalf("expected en model's native stream closed exactly once on switch, got %d", streamA.closeCount)
	}
	if modelB.processCalls == 0 {
		t.Fatal("expected es model to process after the language switch")
	}
}
