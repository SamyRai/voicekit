package asr

import "testing"

// These tests exercise VAD provider selection and the non-native detectors
// (passthrough, energy) plus the error paths for the native providers. They are
// hermetic: silero/ten fail on the missing model path before any native detector
// is constructed, so no model weights are required.

func TestNewVADServiceDefaultsToEnergy(t *testing.T) {
	svc, err := NewVADService(nil)
	if err != nil {
		t.Fatalf("NewVADService(nil): %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	// The energy detector treats silence as non-speech; a passthrough default
	// would instead report speech for any non-empty buffer.
	res, err := svc.Process([]float32{0, 0, 0, 0}, nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if res.IsSpeech {
		t.Error("default (energy) VAD should not flag silence as speech")
	}
}

func TestVADProviderNonePassesThrough(t *testing.T) {
	svc, err := NewVADService(&VADConfig{Provider: VADProviderNone})
	if err != nil {
		t.Fatalf("NewVADService: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	res, err := svc.Process([]float32{0, 0, 0}, nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !res.IsSpeech {
		t.Error("passthrough VAD should report speech for any non-empty audio")
	}

	empty, err := svc.Process(nil, nil)
	if err != nil {
		t.Fatalf("Process(nil): %v", err)
	}
	if empty.IsSpeech {
		t.Error("passthrough VAD should report no speech for empty audio")
	}
}

func TestVADProviderEnergyThreshold(t *testing.T) {
	svc, err := NewVADService(&VADConfig{Provider: VADProviderEnergy, Threshold: 0.1})
	if err != nil {
		t.Fatalf("NewVADService: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	// mean-square of 0.5 samples = 0.25 > 0.1 threshold -> speech
	loud, err := svc.Process([]float32{0.5, 0.5, 0.5, 0.5}, nil)
	if err != nil {
		t.Fatalf("Process(loud): %v", err)
	}
	if !loud.IsSpeech {
		t.Error("energy VAD should flag above-threshold audio as speech")
	}

	// mean-square of 0.01 samples = 1e-4 < 0.1 threshold -> not speech
	quiet, err := svc.Process([]float32{0.01, 0.01}, nil)
	if err != nil {
		t.Fatalf("Process(quiet): %v", err)
	}
	if quiet.IsSpeech {
		t.Error("energy VAD should not flag below-threshold audio as speech")
	}

	empty, err := svc.Process([]float32{}, nil)
	if err != nil {
		t.Fatalf("Process(empty): %v", err)
	}
	if empty.IsSpeech {
		t.Error("empty audio is not speech")
	}
}

func TestVADNativeProvidersRequireModelPath(t *testing.T) {
	for _, provider := range []string{VADProviderSilero, VADProviderTen} {
		if _, err := NewVADService(&VADConfig{Provider: provider}); err == nil {
			t.Errorf("provider %q without a model path should return an error", provider)
		}
	}
}

func TestVADUnknownProviderErrors(t *testing.T) {
	if _, err := NewVADService(&VADConfig{Provider: "bogus"}); err == nil {
		t.Error("unknown VAD provider should return an error")
	}
}

func TestVADNilServiceProcessErrors(t *testing.T) {
	var svc *VADService
	if _, err := svc.Process([]float32{0.1}, nil); err == nil {
		t.Error("Process on a nil VAD service should return an error")
	}
}
