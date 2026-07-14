package tts

import (
	"strings"
	"testing"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// These tests prove that every supported TTS model family maps its configuration
// onto the correct Sherpa native config fields, and that the required-path
// validation matrix is enforced per family. They are hermetic: no model weights
// and no native synthesizer construction, so they run everywhere. Kokoro mapping
// and validation are already covered in sherpa_offline_test.go.

func ttsBase(family string) Config {
	c := DefaultConfig()
	c.Enabled = true
	c.ModelFamily = family
	c.NumThreads = 2
	c.Provider = "cpu"
	return c
}

func assertTTSCommon(t *testing.T, got *sherpa.OfflineTtsConfig, c *Config) {
	t.Helper()
	if got.Model.NumThreads != c.NumThreads {
		t.Errorf("Model.NumThreads = %d, want %d", got.Model.NumThreads, c.NumThreads)
	}
	if got.Model.Provider != c.Provider {
		t.Errorf("Model.Provider = %q, want %q", got.Model.Provider, c.Provider)
	}
}

func TestBuildOfflineTTSConfigMapsVits(t *testing.T) {
	c := ttsBase(FamilyVits)
	c.Vits = VitsConfig{Model: "m", Tokens: "tok", DataDir: "dd", Lexicon: "lex", NoiseScale: 0.5, NoiseScaleW: 0.6, LengthScale: 1.2}
	got, err := buildOfflineTTSConfig(&c)
	if err != nil {
		t.Fatalf("buildOfflineTTSConfig: %v", err)
	}
	v := got.Model.Vits
	if v.Model != "m" || v.Tokens != "tok" || v.DataDir != "dd" || v.Lexicon != "lex" {
		t.Errorf("vits paths not mapped: %+v", v)
	}
	if v.NoiseScale != 0.5 || v.NoiseScaleW != 0.6 || v.LengthScale != 1.2 {
		t.Errorf("vits scales not mapped: %+v", v)
	}
	if got.Model.Matcha.AcousticModel != "" || got.Model.Kokoro.Model != "" {
		t.Error("unexpected cross-family field population for vits")
	}
	assertTTSCommon(t, got, &c)
}

func TestBuildOfflineTTSConfigMapsMatcha(t *testing.T) {
	c := ttsBase(FamilyMatcha)
	c.Matcha = MatchaConfig{AcousticModel: "am", Vocoder: "voc", Tokens: "tok", DataDir: "dd", NoiseScale: 0.7, LengthScale: 1.1}
	got, err := buildOfflineTTSConfig(&c)
	if err != nil {
		t.Fatalf("buildOfflineTTSConfig: %v", err)
	}
	m := got.Model.Matcha
	if m.AcousticModel != "am" || m.Vocoder != "voc" || m.Tokens != "tok" || m.DataDir != "dd" {
		t.Errorf("matcha paths not mapped: %+v", m)
	}
	if m.NoiseScale != 0.7 || m.LengthScale != 1.1 {
		t.Errorf("matcha scales not mapped: %+v", m)
	}
	if got.Model.Vits.Model != "" || got.Model.Kokoro.Model != "" {
		t.Error("unexpected cross-family field population for matcha")
	}
	assertTTSCommon(t, got, &c)
}

func TestBuildOfflineTTSConfigMapsKitten(t *testing.T) {
	c := ttsBase(FamilyKitten)
	c.Kitten = KittenConfig{Model: "m", Voices: "v", Tokens: "tok", DataDir: "dd", LengthScale: 0.9}
	got, err := buildOfflineTTSConfig(&c)
	if err != nil {
		t.Fatalf("buildOfflineTTSConfig: %v", err)
	}
	k := got.Model.Kitten
	if k.Model != "m" || k.Voices != "v" || k.Tokens != "tok" || k.DataDir != "dd" || k.LengthScale != 0.9 {
		t.Errorf("kitten not mapped: %+v", k)
	}
	if got.Model.Kokoro.Model != "" {
		t.Error("unexpected cross-family field population for kitten")
	}
	assertTTSCommon(t, got, &c)
}

func TestBuildOfflineTTSConfigMapsZipvoice(t *testing.T) {
	c := ttsBase(FamilyZipvoice)
	c.Zipvoice = ZipvoiceConfig{
		Tokens: "tok", Encoder: "enc", Decoder: "dec", Vocoder: "voc", DataDir: "dd", Lexicon: "lex",
		FeatScale: 0.2, TShift: 0.4, TargetRms: 0.15, GuidanceScale: 1.3,
	}
	got, err := buildOfflineTTSConfig(&c)
	if err != nil {
		t.Fatalf("buildOfflineTTSConfig: %v", err)
	}
	z := got.Model.Zipvoice
	if z.Tokens != "tok" || z.Encoder != "enc" || z.Decoder != "dec" || z.Vocoder != "voc" || z.DataDir != "dd" || z.Lexicon != "lex" {
		t.Errorf("zipvoice paths not mapped: %+v", z)
	}
	if z.FeatScale != 0.2 || z.TShift != 0.4 || z.TargetRms != 0.15 || z.GuidanceScale != 1.3 {
		t.Errorf("zipvoice scales not mapped: %+v", z)
	}
	if got.Model.Vits.Model != "" {
		t.Error("unexpected cross-family field population for zipvoice")
	}
	assertTTSCommon(t, got, &c)
}

func TestBuildOfflineTTSConfigMapsPocket(t *testing.T) {
	c := ttsBase(FamilyPocket)
	c.Pocket = PocketConfig{
		LmFlow: "lf", LmMain: "lm", Encoder: "enc", Decoder: "dec", TextConditioner: "tc",
		VocabJSON: "vj", TokenScoresJSON: "ts", VoiceEmbeddingCacheCapacity: 32,
	}
	got, err := buildOfflineTTSConfig(&c)
	if err != nil {
		t.Fatalf("buildOfflineTTSConfig: %v", err)
	}
	p := got.Model.Pocket
	if p.LmFlow != "lf" || p.LmMain != "lm" || p.Encoder != "enc" || p.Decoder != "dec" || p.TextConditioner != "tc" {
		t.Errorf("pocket paths not mapped: %+v", p)
	}
	if p.VocabJson != "vj" || p.TokenScoresJson != "ts" || p.VoiceEmbeddingCacheCapacity != 32 {
		t.Errorf("pocket json/capacity not mapped: %+v", p)
	}
	if got.Model.Supertonic.TextEncoder != "" {
		t.Error("unexpected cross-family field population for pocket")
	}
	assertTTSCommon(t, got, &c)
}

func TestBuildOfflineTTSConfigMapsSupertonic(t *testing.T) {
	c := ttsBase(FamilySupertonic)
	c.Supertonic = SupertonicConfig{
		DurationPredictor: "dp", TextEncoder: "te", VectorEstimator: "ve", Vocoder: "voc",
		TtsJSON: "tj", UnicodeIndexer: "ui", VoiceStyle: "vs",
	}
	got, err := buildOfflineTTSConfig(&c)
	if err != nil {
		t.Fatalf("buildOfflineTTSConfig: %v", err)
	}
	s := got.Model.Supertonic
	if s.DurationPredictor != "dp" || s.TextEncoder != "te" || s.VectorEstimator != "ve" || s.Vocoder != "voc" {
		t.Errorf("supertonic paths not mapped: %+v", s)
	}
	if s.TtsJson != "tj" || s.UnicodeIndexer != "ui" || s.VoiceStyle != "vs" {
		t.Errorf("supertonic json/style not mapped: %+v", s)
	}
	if got.Model.Pocket.LmFlow != "" {
		t.Error("unexpected cross-family field population for supertonic")
	}
	assertTTSCommon(t, got, &c)
}

func TestBuildOfflineTTSConfigRejectsUnknownFamily(t *testing.T) {
	c := ttsBase("does_not_exist")
	if _, err := buildOfflineTTSConfig(&c); err == nil {
		t.Fatal("expected error for unknown TTS family, got nil")
	}
}

// TestValidateTTSRequiredPathMatrix asserts each family reports its
// required-but-missing paths. Empty paths mean no filesystem access occurs.
func TestValidateTTSRequiredPathMatrix(t *testing.T) {
	cases := []struct {
		family   string
		wantSubs []string
	}{
		{FamilyVits, []string{"vits", "model", "tokens", "data dir"}},
		{FamilyMatcha, []string{"matcha", "acoustic model", "vocoder", "data dir"}},
		{FamilyKokoro, []string{"kokoro", "voices", "tokens", "data dir"}},
		{FamilyKitten, []string{"kitten", "voices", "data dir"}},
		{FamilyZipvoice, []string{"zipvoice", "encoder", "decoder", "vocoder"}},
		{FamilyPocket, []string{"pocket", "lm flow", "encoder", "vocab json"}},
		{FamilySupertonic, []string{"supertonic", "duration predictor", "vocoder", "voice style"}},
	}
	for _, tc := range cases {
		t.Run(tc.family, func(t *testing.T) {
			c := ttsBase(tc.family)
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

func TestValidateTTSRejectsUnknownFamily(t *testing.T) {
	c := ttsBase("nope")
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "unsupported TTS model family") {
		t.Fatalf("expected unsupported-family error, got: %v", err)
	}
}
