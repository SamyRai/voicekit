package speaker

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/go-audio/wav"
)

// AudioParser handles audio file parsing and processing
type AudioParser struct{}

// NewAudioParser creates a new audio parser
func NewAudioParser() *AudioParser {
	return &AudioParser{}
}

// ParseAudioFile parses audio file from multipart form
func (p *AudioParser) ParseAudioFile(file []byte, filename string) ([]float32, int, error) {
	// Check file type
	filename = strings.ToLower(filename)
	if !strings.HasSuffix(filename, ".wav") {
		return nil, 0, fmt.Errorf("only WAV files are supported")
	}

	return p.parseWAVFile(file)
}

// ParseWAVFile parses WAV file and returns audio data
func (p *AudioParser) ParseWAVFile(file []byte) ([]float32, int, error) {
	return p.parseWAVFile(file)
}

// parseWAVFile internal WAV parsing logic
func (p *AudioParser) parseWAVFile(file []byte) ([]float32, int, error) {
	// Read WAV file
	reader := bytes.NewReader(file)
	decoder := wav.NewDecoder(reader)
	if !decoder.IsValidFile() {
		return nil, 0, fmt.Errorf("invalid WAV file")
	}

	// Get audio format information
	sampleRate := int(decoder.SampleRate)
	numChannels := int(decoder.NumChans)

	// Only support mono or stereo
	if numChannels > 2 {
		return nil, 0, fmt.Errorf("unsupported number of channels: %d", numChannels)
	}

	// Read audio data
	buffer, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decode audio: %v", err)
	}

	// Convert to float32 format
	samples := make([]float32, len(buffer.Data))
	for i, sample := range buffer.Data {
		// Convert int to float32, range [-1.0, 1.0]
		samples[i] = float32(sample) / 32768.0
	}

	// If stereo, convert to mono (take average)
	if numChannels == 2 {
		monoSamples := make([]float32, len(samples)/2)
		for i := 0; i < len(monoSamples); i++ {
			monoSamples[i] = (samples[i*2] + samples[i*2+1]) / 2.0
		}
		samples = monoSamples
	}

	return samples, sampleRate, nil
}

// ParseBase64Audio parses Base64 encoded audio data
func (p *AudioParser) ParseBase64Audio(audioData string, sampleRate int) ([]float32, error) {
	// Decode Base64
	audioBytes, err := base64.StdEncoding.DecodeString(audioData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Base64 audio data: %v", err)
	}

	// For now, assume it's WAV data in Base64
	// In future, could add format detection
	reader := strings.NewReader(string(audioBytes))

	// Parse WAV data from reader
	samples, parsedSampleRate, err := p.parseWAVFileFromReader(reader)
	if err != nil {
		return nil, err
	}

	// Verify sample rate matches if provided
	if sampleRate > 0 && parsedSampleRate != sampleRate {
		return nil, fmt.Errorf("sample rate mismatch: expected %d, got %d", sampleRate, parsedSampleRate)
	}

	return samples, nil
}

// parseWAVFileFromReader parses WAV file from io.Reader
func (p *AudioParser) parseWAVFileFromReader(reader *strings.Reader) ([]float32, int, error) {
	// Read WAV file
	decoder := wav.NewDecoder(reader)
	if !decoder.IsValidFile() {
		return nil, 0, fmt.Errorf("invalid WAV file")
	}

	// Get audio format information
	sampleRate := int(decoder.SampleRate)
	numChannels := int(decoder.NumChans)

	// Only support mono or stereo
	if numChannels > 2 {
		return nil, 0, fmt.Errorf("unsupported number of channels: %d", numChannels)
	}

	// Read audio data
	buffer, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decode audio: %v", err)
	}

	// Convert to float32 format
	samples := make([]float32, len(buffer.Data))
	for i, sample := range buffer.Data {
		// Convert int to float32, range [-1.0, 1.0]
		samples[i] = float32(sample) / 32768.0
	}

	// If stereo, convert to mono (take average)
	if numChannels == 2 {
		monoSamples := make([]float32, len(samples)/2)
		for i := 0; i < len(monoSamples); i++ {
			monoSamples[i] = (samples[i*2] + samples[i*2+1]) / 2.0
		}
		samples = monoSamples
	}

	return samples, sampleRate, nil
}

// ValidateAudioFormat validates if audio format is supported
func (p *AudioParser) ValidateAudioFormat(filename string) error {
	if !strings.HasSuffix(strings.ToLower(filename), ".wav") {
		return fmt.Errorf("only WAV files are supported")
	}
	return nil
}

// GetSupportedFormats returns supported audio formats
func (p *AudioParser) GetSupportedFormats() []string {
	return []string{".wav"}
}