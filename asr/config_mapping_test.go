package asr

import (
	"strings"
	"testing"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// These tests prove that every supported ASR model family maps its configuration
// onto the correct Sherpa native config fields, and that the required-path
// validation matrix is enforced per family. They are hermetic: no model weights
// and no native recognizer construction are involved, so they run everywhere.

func offlineBaseConfig() Config {
	c := DefaultConfig()
	c.Enabled = true
	c.Backend = BackendSherpaOffline
	c.SampleRate = 16000
	c.FeatureDim = 80
	c.NumThreads = 2
	c.Provider = "cpu"
	c.DecodingMethod = "greedy_search"
	c.MaxActivePaths = 4
	return c
}

func assertCommonOffline(t *testing.T, got *sherpa.OfflineRecognizerConfig, c *Config) {
	t.Helper()
	if got.FeatConfig.SampleRate != c.SampleRate {
		t.Errorf("FeatConfig.SampleRate = %d, want %d", got.FeatConfig.SampleRate, c.SampleRate)
	}
	if got.FeatConfig.FeatureDim != c.FeatureDim {
		t.Errorf("FeatConfig.FeatureDim = %d, want %d", got.FeatConfig.FeatureDim, c.FeatureDim)
	}
	if got.ModelConfig.NumThreads != c.NumThreads {
		t.Errorf("ModelConfig.NumThreads = %d, want %d", got.ModelConfig.NumThreads, c.NumThreads)
	}
	if got.ModelConfig.Provider != c.Provider {
		t.Errorf("ModelConfig.Provider = %q, want %q", got.ModelConfig.Provider, c.Provider)
	}
	if got.DecodingMethod != c.DecodingMethod {
		t.Errorf("DecodingMethod = %q, want %q", got.DecodingMethod, c.DecodingMethod)
	}
	if got.MaxActivePaths != c.MaxActivePaths {
		t.Errorf("MaxActivePaths = %d, want %d", got.MaxActivePaths, c.MaxActivePaths)
	}
}

func TestBuildOfflineRecognizerConfigMapsTransducer(t *testing.T) {
	c := offlineBaseConfig()
	c.Offline = OfflineConfig{
		ModelFamily: OfflineFamilyTransducer,
		TokensPath:  "tok", EncoderPath: "enc", DecoderPath: "dec", JoinerPath: "join",
	}
	got, err := buildOfflineRecognizerConfig(&c)
	if err != nil {
		t.Fatalf("buildOfflineRecognizerConfig: %v", err)
	}
	tr := got.ModelConfig.Transducer
	if tr.Encoder != "enc" || tr.Decoder != "dec" || tr.Joiner != "join" {
		t.Errorf("transducer paths not mapped: %+v", tr)
	}
	if got.ModelConfig.Tokens != "tok" {
		t.Errorf("Tokens = %q, want tok", got.ModelConfig.Tokens)
	}
	if got.ModelConfig.SenseVoice.Model != "" || got.ModelConfig.Paraformer.Model != "" || got.ModelConfig.Whisper.Encoder != "" {
		t.Error("unexpected cross-family field population for transducer")
	}
	assertCommonOffline(t, got, &c)
}

func TestBuildOfflineRecognizerConfigMapsParaformer(t *testing.T) {
	c := offlineBaseConfig()
	c.Offline = OfflineConfig{ModelFamily: OfflineFamilyParaformer, TokensPath: "tok", ModelPath: "model"}
	got, err := buildOfflineRecognizerConfig(&c)
	if err != nil {
		t.Fatalf("buildOfflineRecognizerConfig: %v", err)
	}
	if got.ModelConfig.Paraformer.Model != "model" {
		t.Errorf("Paraformer.Model = %q, want model", got.ModelConfig.Paraformer.Model)
	}
	if got.ModelConfig.Transducer.Encoder != "" || got.ModelConfig.ZipformerCtc.Model != "" {
		t.Error("unexpected cross-family field population for paraformer")
	}
	assertCommonOffline(t, got, &c)
}

func TestBuildOfflineRecognizerConfigMapsZipformerCTC(t *testing.T) {
	c := offlineBaseConfig()
	c.Offline = OfflineConfig{ModelFamily: OfflineFamilyZipformerCTC, TokensPath: "tok", ModelPath: "model"}
	got, err := buildOfflineRecognizerConfig(&c)
	if err != nil {
		t.Fatalf("buildOfflineRecognizerConfig: %v", err)
	}
	if got.ModelConfig.ZipformerCtc.Model != "model" {
		t.Errorf("ZipformerCtc.Model = %q, want model", got.ModelConfig.ZipformerCtc.Model)
	}
	if got.ModelConfig.Paraformer.Model != "" || got.ModelConfig.NemoCTC.Model != "" {
		t.Error("unexpected cross-family field population for zipformer_ctc")
	}
	assertCommonOffline(t, got, &c)
}

func TestBuildOfflineRecognizerConfigMapsNemoCTC(t *testing.T) {
	c := offlineBaseConfig()
	c.Offline = OfflineConfig{ModelFamily: OfflineFamilyNemoCTC, TokensPath: "tok", ModelPath: "model"}
	got, err := buildOfflineRecognizerConfig(&c)
	if err != nil {
		t.Fatalf("buildOfflineRecognizerConfig: %v", err)
	}
	if got.ModelConfig.NemoCTC.Model != "model" {
		t.Errorf("NemoCTC.Model = %q, want model", got.ModelConfig.NemoCTC.Model)
	}
	if got.ModelConfig.ZipformerCtc.Model != "" || got.ModelConfig.SenseVoice.Model != "" {
		t.Error("unexpected cross-family field population for nemo_ctc")
	}
	assertCommonOffline(t, got, &c)
}

func TestBuildOfflineRecognizerConfigMapsSenseVoice(t *testing.T) {
	c := offlineBaseConfig()
	c.Offline = OfflineConfig{
		ModelFamily: OfflineFamilySenseVoice, ModelPath: "model",
		Language: "zh", UseInverseTextNormalization: true,
	}
	got, err := buildOfflineRecognizerConfig(&c)
	if err != nil {
		t.Fatalf("buildOfflineRecognizerConfig: %v", err)
	}
	sv := got.ModelConfig.SenseVoice
	if sv.Model != "model" || sv.Language != "zh" || sv.UseInverseTextNormalization != 1 {
		t.Errorf("sense_voice not mapped: %+v", sv)
	}
	if got.ModelConfig.Whisper.Encoder != "" || got.ModelConfig.Transducer.Encoder != "" {
		t.Error("unexpected cross-family field population for sense_voice")
	}
	assertCommonOffline(t, got, &c)
}

// Whisper offline mapping is covered by TestBuildOfflineRecognizerConfigMapsWhisper
// in service_test.go; the families below are the previously-untested ones.

func TestBuildOfflineRecognizerConfigRejectsUnknownFamily(t *testing.T) {
	c := offlineBaseConfig()
	c.Offline = OfflineConfig{ModelFamily: "does_not_exist", ModelPath: "model"}
	if _, err := buildOfflineRecognizerConfig(&c); err == nil {
		t.Fatal("expected error for unknown offline family, got nil")
	}
}

func onlineBaseConfig() Config {
	c := DefaultConfig()
	c.Enabled = true
	c.Backend = BackendSherpaOnline
	c.SampleRate = 16000
	c.FeatureDim = 80
	c.NumThreads = 2
	c.Provider = "cpu"
	return c
}

func TestBuildOnlineRecognizerConfigMapsTransducer(t *testing.T) {
	c := onlineBaseConfig()
	c.Online = OnlineConfig{TokensPath: "tok", EncoderPath: "enc", DecoderPath: "dec", JoinerPath: "join"}
	got, err := buildOnlineRecognizerConfig(&c)
	if err != nil {
		t.Fatalf("buildOnlineRecognizerConfig: %v", err)
	}
	tr := got.ModelConfig.Transducer
	if tr.Encoder != "enc" || tr.Decoder != "dec" || tr.Joiner != "join" {
		t.Errorf("online transducer paths not mapped: %+v", tr)
	}
	if got.ModelConfig.Zipformer2Ctc.Model != "" || got.ModelConfig.NemoCtc.Model != "" {
		t.Error("unexpected CTC field population for online transducer")
	}
	if got.FeatConfig.SampleRate != c.SampleRate || got.ModelConfig.NumThreads != c.NumThreads {
		t.Error("online common fields not mapped")
	}
}

func TestBuildOnlineRecognizerConfigMapsSingleFileCTC(t *testing.T) {
	cases := []struct {
		modelType string
		field     func(*sherpa.OnlineModelConfig) string
		other     func(*sherpa.OnlineModelConfig) bool
	}{
		{"zipformer2_ctc", func(m *sherpa.OnlineModelConfig) string { return m.Zipformer2Ctc.Model }, func(m *sherpa.OnlineModelConfig) bool { return m.NemoCtc.Model != "" || m.ToneCtc.Model != "" }},
		{"nemo_ctc", func(m *sherpa.OnlineModelConfig) string { return m.NemoCtc.Model }, func(m *sherpa.OnlineModelConfig) bool { return m.Zipformer2Ctc.Model != "" || m.ToneCtc.Model != "" }},
		{"tone_ctc", func(m *sherpa.OnlineModelConfig) string { return m.ToneCtc.Model }, func(m *sherpa.OnlineModelConfig) bool { return m.Zipformer2Ctc.Model != "" || m.NemoCtc.Model != "" }},
	}
	for _, tc := range cases {
		t.Run(tc.modelType, func(t *testing.T) {
			c := onlineBaseConfig()
			c.Online = OnlineConfig{TokensPath: "tok", ModelPath: "model", ModelType: tc.modelType}
			got, err := buildOnlineRecognizerConfig(&c)
			if err != nil {
				t.Fatalf("buildOnlineRecognizerConfig: %v", err)
			}
			if tc.field(&got.ModelConfig) != "model" {
				t.Errorf("%s model path not mapped", tc.modelType)
			}
			if tc.other(&got.ModelConfig) {
				t.Errorf("%s populated a sibling CTC field", tc.modelType)
			}
			if got.ModelConfig.Transducer.Encoder != "" {
				t.Errorf("%s populated the transducer field", tc.modelType)
			}
		})
	}
}

func TestBuildOnlineRecognizerConfigRejectsPartialTransducer(t *testing.T) {
	c := onlineBaseConfig()
	c.Online = OnlineConfig{TokensPath: "tok", EncoderPath: "enc"} // decoder+joiner missing
	if _, err := buildOnlineRecognizerConfig(&c); err == nil {
		t.Fatal("expected error for partial online transducer paths, got nil")
	}
}

func TestBuildOnlineRecognizerConfigRejectsUnknownModelType(t *testing.T) {
	c := onlineBaseConfig()
	c.Online = OnlineConfig{TokensPath: "tok", ModelPath: "model", ModelType: "not_a_real_type"}
	if _, err := buildOnlineRecognizerConfig(&c); err == nil {
		t.Fatal("expected error for unknown online model type, got nil")
	}
}

// TestValidateOfflineRequiredPathMatrix asserts that each offline family reports
// its required-but-missing paths. Uses empty paths so no filesystem access occurs.
func TestValidateOfflineRequiredPathMatrix(t *testing.T) {
	cases := []struct {
		family   string
		wantSubs []string
	}{
		{OfflineFamilyTransducer, []string{"tokens", "encoder", "decoder", "joiner"}},
		{OfflineFamilyParaformer, []string{"tokens", "model"}},
		{OfflineFamilyZipformerCTC, []string{"tokens", "model"}},
		{OfflineFamilyNemoCTC, []string{"tokens", "model"}},
		{OfflineFamilySenseVoice, []string{"model"}},
		{OfflineFamilyWhisper, []string{"encoder", "decoder"}},
	}
	for _, tc := range cases {
		t.Run(tc.family, func(t *testing.T) {
			c := offlineBaseConfig()
			c.Offline = OfflineConfig{ModelFamily: tc.family}
			err := c.Validate()
			if err == nil {
				t.Fatalf("expected validation errors for %s with no paths", tc.family)
			}
			msg := err.Error()
			for _, sub := range tc.wantSubs {
				if !strings.Contains(msg, sub) {
					t.Errorf("validation error for %s missing mention of %q; got: %s", tc.family, sub, msg)
				}
			}
		})
	}
}

// TestValidateOnlineRequiresTokensAndModel asserts the online backend rejects a
// config with neither transducer paths nor a single model path.
func TestValidateOnlineRequiresModel(t *testing.T) {
	c := onlineBaseConfig()
	c.Online = OnlineConfig{} // nothing set
	err := c.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty online config")
	}
	if !strings.Contains(err.Error(), "tokens") {
		t.Errorf("expected online tokens requirement; got: %s", err.Error())
	}
}
