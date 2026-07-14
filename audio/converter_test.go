package audio

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestPCM8BitRoundTrip(t *testing.T) {
	converter := newTestConverter(t)
	config := &AudioConfig{Format: FormatPCM, SampleRate: 16000, Channels: 1, BitsPerSample: 8}

	encoded, err := converter.ConvertFromFloat32([]float32{-1, 0, 1}, config)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if got, want := encoded, []byte{0, 128, 255}; string(got) != string(want) {
		t.Fatalf("unexpected 8-bit PCM bytes: got %v want %v", got, want)
	}

	decoded, err := converter.ConvertToFloat32(encoded, config)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	assertNear(t, decoded[0], -1, 0.001)
	assertNear(t, decoded[1], 0, 0.001)
	assertNear(t, decoded[2], 0.9921875, 0.001)
}

func TestPCM24BitRoundTrip(t *testing.T) {
	converter := newTestConverter(t)
	config := &AudioConfig{Format: FormatPCM, SampleRate: 16000, Channels: 1, BitsPerSample: 24}

	input := []float32{-1, -0.5, 0, 0.5, 1}
	encoded, err := converter.ConvertFromFloat32(input, config)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	decoded, err := converter.ConvertToFloat32(encoded, config)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for i := range input {
		assertNear(t, decoded[i], input[i], 0.000001)
	}
}

func TestPCMRoundTripSupportedBitDepths(t *testing.T) {
	converter := newTestConverter(t)
	tests := []struct {
		name      string
		bitDepth  int
		input     []float32
		tolerance float32
	}{
		{
			name:      "16-bit mono",
			bitDepth:  16,
			input:     []float32{-1, -0.25, 0, 0.25, 1},
			tolerance: 0.0001,
		},
		{
			name:      "32-bit stereo",
			bitDepth:  32,
			input:     []float32{-1, 0.5, -0.5, 1},
			tolerance: 0.000001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channels := 1
			if tt.name == "32-bit stereo" {
				channels = 2
			}
			config := &AudioConfig{Format: FormatPCM, SampleRate: 16000, Channels: channels, BitsPerSample: tt.bitDepth}
			encoded, err := converter.ConvertFromFloat32(tt.input, config)
			if err != nil {
				t.Fatalf("encode failed: %v", err)
			}
			decoded, err := converter.ConvertToFloat32(encoded, config)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			assertSliceNear(t, decoded, tt.input, tt.tolerance)
		})
	}
}

func TestWAVDecodeWithExtraChunk(t *testing.T) {
	converter := newTestConverter(t)
	config := &AudioConfig{Format: FormatWAV, SampleRate: 16000, Channels: 1, BitsPerSample: 16}

	data := wavWithJunkChunk([]int16{-32768, 0, 32767}, config)
	decoded, err := converter.ConvertToFloat32(data, config)
	if err != nil {
		t.Fatalf("decode WAV with extra chunk failed: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(decoded))
	}
	assertNear(t, decoded[0], -1, 0.001)
	assertNear(t, decoded[1], 0, 0.001)
	assertNear(t, decoded[2], 0.9999695, 0.001)
}

func TestParseWAVFileWithExtraChunkAndStereoDownmix(t *testing.T) {
	converter := newTestConverter(t)
	config := &AudioConfig{Format: FormatWAV, SampleRate: 16000, Channels: 2, BitsPerSample: 16}

	data := wavWithJunkChunk([]int16{10000, -10000, 20000, 10000}, config)
	decoded, sampleRate, err := converter.ParseWAVFile(data)
	if err != nil {
		t.Fatalf("ParseWAVFile failed: %v", err)
	}
	if sampleRate != 16000 {
		t.Fatalf("expected sample rate 16000, got %d", sampleRate)
	}
	if len(decoded) != 2 {
		t.Fatalf("expected 2 mono samples, got %d", len(decoded))
	}
	assertNear(t, decoded[0], 0, 0.0001)
	assertNear(t, decoded[1], float32(15000.0/32768.0), 0.0001)
}

func TestSupportedFormatsAreActuallyImplemented(t *testing.T) {
	// Every advertised format must actually decode; formats without a decoder
	// (AAC/M4A) must not be advertised.
	for _, f := range []AudioFormat{FormatWAV, FormatPCM, FormatFLAC, FormatMP3, FormatOGG} {
		if !IsSupportedFormat(f) {
			t.Errorf("%s should be advertised as supported", f)
		}
	}
	for _, f := range []AudioFormat{FormatAAC, FormatM4A} {
		if IsSupportedFormat(f) {
			t.Errorf("%s should not be advertised as supported without a decoder", f)
		}
	}
}

func newTestConverter(t *testing.T) *Converter {
	t.Helper()
	converter, err := NewConverter(&ConverterConfig{
		EnableResampling:    false,
		EnableNormalization: false,
		TargetRMS:           0.1,
		TempBufferSize:      8192,
		NormalizeFactor:     32768,
	})
	if err != nil {
		t.Fatalf("failed to create converter: %v", err)
	}
	return converter
}

func wavWithJunkChunk(samples []int16, config *AudioConfig) []byte {
	pcmSize := len(samples) * 2
	junkSize := 4
	riffSize := 4 + (8 + 16) + (8 + junkSize) + (8 + pcmSize)
	data := make([]byte, 8+riffSize)

	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(riffSize))
	copy(data[8:12], "WAVE")

	offset := 12
	copy(data[offset:offset+4], "fmt ")
	binary.LittleEndian.PutUint32(data[offset+4:offset+8], 16)
	binary.LittleEndian.PutUint16(data[offset+8:offset+10], 1)
	binary.LittleEndian.PutUint16(data[offset+10:offset+12], uint16(config.Channels))
	binary.LittleEndian.PutUint32(data[offset+12:offset+16], uint32(config.SampleRate))
	binary.LittleEndian.PutUint32(data[offset+16:offset+20], uint32(config.GetBytesPerSecond()))
	binary.LittleEndian.PutUint16(data[offset+20:offset+22], uint16(config.Channels*config.GetBytesPerSample()))
	binary.LittleEndian.PutUint16(data[offset+22:offset+24], uint16(config.BitsPerSample))
	offset += 24

	copy(data[offset:offset+4], "JUNK")
	binary.LittleEndian.PutUint32(data[offset+4:offset+8], uint32(junkSize))
	copy(data[offset+8:offset+12], []byte{1, 2, 3, 4})
	offset += 12

	copy(data[offset:offset+4], "data")
	binary.LittleEndian.PutUint32(data[offset+4:offset+8], uint32(pcmSize))
	offset += 8
	for _, sample := range samples {
		binary.LittleEndian.PutUint16(data[offset:offset+2], uint16(sample))
		offset += 2
	}

	return data
}

func assertNear(t *testing.T, got, want, tolerance float32) {
	t.Helper()
	if math.Abs(float64(got-want)) > float64(tolerance) {
		t.Fatalf("got %f want %f (tolerance %f)", got, want, tolerance)
	}
}

func assertSliceNear(t *testing.T, got, want []float32, tolerance float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d samples, want %d", len(got), len(want))
	}
	for i := range want {
		if math.Abs(float64(got[i]-want[i])) > float64(tolerance) {
			t.Fatalf("sample %d: got %f want %f (tolerance %f)", i, got[i], want[i], tolerance)
		}
	}
}
