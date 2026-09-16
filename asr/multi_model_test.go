package asr

import (
	"context"
	"strings"
	"testing"

	"go.glpx.pro/voicekit/types"
)

// These tests cover Wave 3d: hosting multiple online models in one Service
// and routing sessions to the right one by language.
//
// Routing tests (TestServiceRoutesSessionsByLanguage) are hermetic: they
// register fake types.ASRModel implementations directly via
// Service.RegisterModel, so no native Sherpa recognizer construction is
// involved. Config-resolution and validation tests
// (resolvedOnlineModelConfigs, Config.Validate) are likewise hermetic: they
// only touch os.Stat via requireFile, never the sherpa native library.

// countingModel wraps testModel to record how many times ProcessAudio was
// invoked, so routing tests can assert which registered model actually
// handled a session's audio.
type countingModel struct {
	testModel
	calls int
}

func (m *countingModel) ProcessAudio(ctx context.Context, audio []float32, state *types.StreamingState) (*types.Transcription, error) {
	m.calls++
	return m.testModel.ProcessAudio(ctx, audio, state)
}

// TestServiceRoutesSessionsByLanguage proves that a Service hosting multiple
// online models (registered directly here, standing in for
// NewService-created SherpaOnlineModels) routes ProcessAudioChunk and
// FinishStream calls to the model matching the session's language: "en" by
// default, or whatever SetSessionLanguage last set.
func TestServiceRoutesSessionsByLanguage(t *testing.T) {
	service := newTestService()
	defer service.Close()

	enModel := &countingModel{testModel: testModel{name: "en_model", language: "en", quantization: "int8"}}
	esModel := &countingModel{testModel: testModel{name: "es_model", language: "es", quantization: "int8"}}
	if err := service.RegisterModel(enModel); err != nil {
		t.Fatalf("failed to register en model: %v", err)
	}
	if err := service.RegisterModel(esModel); err != nil {
		t.Fatalf("failed to register es model: %v", err)
	}

	audio := make([]float32, 16000)
	ctx := context.Background()

	t.Run("default session language routes to en", func(t *testing.T) {
		if _, err := service.ProcessAudioChunk(ctx, "session-default", audio); err != nil {
			t.Fatalf("ProcessAudioChunk failed: %v", err)
		}
		if _, err := service.FinishStream(ctx, "session-default"); err != nil {
			t.Fatalf("FinishStream failed: %v", err)
		}

		if enModel.calls != 2 {
			t.Errorf("expected en model to handle process+finalize (2 calls), got %d", enModel.calls)
		}
		if esModel.calls != 0 {
			t.Errorf("expected es model untouched by default-language session, got %d calls", esModel.calls)
		}
	})

	t.Run("SetSessionLanguage routes to matching model", func(t *testing.T) {
		if err := service.SetSessionLanguage("session-es", "es"); err != nil {
			t.Fatalf("SetSessionLanguage failed: %v", err)
		}
		if _, err := service.ProcessAudioChunk(ctx, "session-es", audio); err != nil {
			t.Fatalf("ProcessAudioChunk failed: %v", err)
		}
		if _, err := service.FinishStream(ctx, "session-es"); err != nil {
			t.Fatalf("FinishStream failed: %v", err)
		}

		if esModel.calls != 2 {
			t.Errorf("expected es model to handle process+finalize (2 calls), got %d", esModel.calls)
		}
		// enModel must not have picked up any of the es-routed session's calls
		// on top of the 2 calls it already recorded for session-default.
		if enModel.calls != 2 {
			t.Errorf("expected en model call count unchanged by es-routed session, got %d", enModel.calls)
		}
	})
}

// TestSetSessionLanguageRejectsEmptyArgs proves the exported entry point
// validates its inputs like the rest of the Service API instead of silently
// creating a malformed session.
func TestSetSessionLanguageRejectsEmptyArgs(t *testing.T) {
	service := newTestService()
	defer service.Close()

	if err := service.SetSessionLanguage("", "es"); err == nil {
		t.Fatal("expected error for empty sessionID")
	}
	if err := service.SetSessionLanguage("session", ""); err == nil {
		t.Fatal("expected error for empty language")
	}
}

// onlineModelsBaseConfig returns a hermetic (no real model weights, backend
// sherpa_online) base config for OnlineModels validation/resolution tests.
func onlineModelsBaseConfig() Config {
	c := DefaultConfig()
	c.Enabled = true
	c.Backend = BackendSherpaOnline
	c.Language = "en"
	c.SampleRate = 16000
	c.FeatureDim = 80
	c.NumThreads = 1
	c.Provider = "cpu"
	return c
}

// TestResolvedOnlineModelConfigsSingleOnlineBackCompat proves that with
// OnlineModels empty, NewService's model-building loop still expands to
// exactly one entry derived from the legacy Config.Online field, with
// Name/Language defaulted from DefaultModel/Language exactly as
// NewSherpaOnlineModel derives them today.
func TestResolvedOnlineModelConfigsSingleOnlineBackCompat(t *testing.T) {
	config := onlineModelsBaseConfig()
	config.DefaultModel = "legacy-online"
	config.Online = OnlineConfig{
		TokensPath: tempModelFile(t, "tokens.txt"),
		ModelPath:  tempModelFile(t, "model.onnx"),
	}
	config.ApplyDefaults()

	entries := config.resolvedOnlineModelConfigs()
	if len(entries) != 1 {
		t.Fatalf("expected exactly one online model config for the legacy Online path, got %d", len(entries))
	}
	if entries[0].Name != "legacy-online" {
		t.Errorf("expected name defaulted from DefaultModel, got %q", entries[0].Name)
	}
	if entries[0].Language != "en" {
		t.Errorf("expected language defaulted from Config.Language, got %q", entries[0].Language)
	}
}

// TestResolvedOnlineModelConfigsUsesOnlineModelsWhenSet proves that a
// non-empty OnlineModels takes over from the legacy Online field entirely,
// preserving each entry's explicit Name/Language.
func TestResolvedOnlineModelConfigsUsesOnlineModelsWhenSet(t *testing.T) {
	config := onlineModelsBaseConfig()
	config.OnlineModels = []OnlineConfig{
		{Name: "en-model", Language: "en", TokensPath: tempModelFile(t, "en-tokens.txt"), ModelPath: tempModelFile(t, "en-model.onnx")},
		{Name: "es-model", Language: "es", TokensPath: tempModelFile(t, "es-tokens.txt"), ModelPath: tempModelFile(t, "es-model.onnx")},
	}
	config.ApplyDefaults()

	entries := config.resolvedOnlineModelConfigs()
	if len(entries) != 2 {
		t.Fatalf("expected both OnlineModels entries, got %d", len(entries))
	}
	if entries[0].Name != "en-model" || entries[0].Language != "en" {
		t.Errorf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].Name != "es-model" || entries[1].Language != "es" {
		t.Errorf("unexpected second entry: %+v", entries[1])
	}
}

// TestValidateOnlineModelsRequiresDistinctNonEmptyNames proves Config.Validate
// rejects duplicate and empty names across OnlineModels entries.
func TestValidateOnlineModelsRequiresDistinctNonEmptyNames(t *testing.T) {
	t.Run("duplicate names", func(t *testing.T) {
		config := onlineModelsBaseConfig()
		config.OnlineModels = []OnlineConfig{
			{Name: "dup", TokensPath: tempModelFile(t, "a-tokens.txt"), ModelPath: tempModelFile(t, "a-model.onnx")},
			{Name: "dup", TokensPath: tempModelFile(t, "b-tokens.txt"), ModelPath: tempModelFile(t, "b-model.onnx")},
		}
		err := config.Validate()
		if err == nil {
			t.Fatal("expected validation error for duplicate online model names")
		}
		if !strings.Contains(err.Error(), "duplicate model name") {
			t.Errorf("expected duplicate-name error, got: %v", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		config := onlineModelsBaseConfig()
		config.OnlineModels = []OnlineConfig{
			{TokensPath: tempModelFile(t, "tokens.txt"), ModelPath: tempModelFile(t, "model.onnx")},
		}
		err := config.Validate()
		if err == nil {
			t.Fatal("expected validation error for empty online model name")
		}
		if !strings.Contains(err.Error(), "name is required") {
			t.Errorf("expected name-required error, got: %v", err)
		}
	})

	t.Run("valid distinct names pass", func(t *testing.T) {
		config := onlineModelsBaseConfig()
		config.OnlineModels = []OnlineConfig{
			{Name: "en-model", TokensPath: tempModelFile(t, "en-tokens.txt"), ModelPath: tempModelFile(t, "en-model.onnx")},
			{Name: "es-model", TokensPath: tempModelFile(t, "es-tokens.txt"), ModelPath: tempModelFile(t, "es-model.onnx")},
		}
		if err := config.Validate(); err != nil {
			t.Fatalf("expected valid OnlineModels config to pass validation: %v", err)
		}
	})
}

// TestValidateOnlineModelsRequiresPaths proves each OnlineModels entry is
// held to the same required-path shape as the legacy single Online config.
func TestValidateOnlineModelsRequiresPaths(t *testing.T) {
	config := onlineModelsBaseConfig()
	config.OnlineModels = []OnlineConfig{{Name: "no-paths"}}

	err := config.Validate()
	if err == nil {
		t.Fatal("expected validation error for online model with no paths")
	}
	if !strings.Contains(err.Error(), `online model "no-paths"`) || !strings.Contains(err.Error(), "tokens") {
		t.Errorf("expected per-model tokens-path error, got: %v", err)
	}
}

// TestNewServiceRejectsDuplicateOnlineModelNames proves NewService's config
// validation gate (which runs before any native Sherpa recognizer
// construction) rejects duplicate OnlineModels names, so the failure is
// hermetic and does not require real model weights.
func TestNewServiceRejectsDuplicateOnlineModelNames(t *testing.T) {
	config := onlineModelsBaseConfig()
	config.OnlineModels = []OnlineConfig{
		{Name: "dup", TokensPath: tempModelFile(t, "a-tokens.txt"), ModelPath: tempModelFile(t, "a-model.onnx")},
		{Name: "dup", TokensPath: tempModelFile(t, "b-tokens.txt"), ModelPath: tempModelFile(t, "b-model.onnx")},
	}

	if _, err := NewService(&config); err == nil {
		t.Fatal("expected NewService to reject duplicate online model names before building any model")
	}
}
