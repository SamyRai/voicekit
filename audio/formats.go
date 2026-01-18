package audio

import (
	"fmt"
	"strings"
)

// AudioFormat represents supported audio formats
type AudioFormat string

const (
	FormatWAV  AudioFormat = "wav"
	FormatFLAC AudioFormat = "flac"
	FormatMP3  AudioFormat = "mp3"
	FormatOGG  AudioFormat = "ogg"
	FormatM4A  AudioFormat = "m4a"
	FormatAAC  AudioFormat = "aac"
	FormatPCM  AudioFormat = "pcm"
)

// AudioConfig represents audio configuration
type AudioConfig struct {
	Format        AudioFormat `json:"format"`
	SampleRate    int         `json:"sample_rate"`
	Channels      int         `json:"channels"`
	BitsPerSample int         `json:"bits_per_sample"`
	Quality       int         `json:"quality,omitempty"` // For lossy formats (0-100)
	Bitrate       int         `json:"bitrate,omitempty"` // For compressed formats (kbps)
}

// DefaultWAVConfig returns default WAV configuration
func DefaultWAVConfig() *AudioConfig {
	return &AudioConfig{
		Format:        FormatWAV,
		SampleRate:    16000,
		Channels:      1,
		BitsPerSample: 16,
	}
}

// DefaultFLACConfig returns default FLAC configuration
func DefaultFLACConfig() *AudioConfig {
	return &AudioConfig{
		Format:        FormatFLAC,
		SampleRate:    16000,
		Channels:      1,
		BitsPerSample: 16,
	}
}

// DefaultMP3Config returns default MP3 configuration
func DefaultMP3Config() *AudioConfig {
	return &AudioConfig{
		Format:        FormatMP3,
		SampleRate:    16000,
		Channels:      1,
		BitsPerSample: 16,
		Quality:       80,
		Bitrate:       128,
	}
}

// DefaultOGGConfig returns default OGG configuration
func DefaultOGGConfig() *AudioConfig {
	return &AudioConfig{
		Format:        FormatOGG,
		SampleRate:    16000,
		Channels:      1,
		BitsPerSample: 16,
		Quality:       80,
	}
}

// DefaultM4AConfig returns default M4A configuration
func DefaultM4AConfig() *AudioConfig {
	return &AudioConfig{
		Format:        FormatM4A,
		SampleRate:    16000,
		Channels:      1,
		BitsPerSample: 16,
		Bitrate:       128,
	}
}

// SupportedFormats returns all supported audio formats
func SupportedFormats() []AudioFormat {
	return []AudioFormat{
		FormatWAV,
		FormatFLAC,
		FormatMP3,
		FormatOGG,
		FormatM4A,
		FormatAAC,
		FormatPCM,
	}
}

// IsSupportedFormat checks if a format is supported
func IsSupportedFormat(format AudioFormat) bool {
	for _, f := range SupportedFormats() {
		if f == format {
			return true
		}
	}
	return false
}

// GetFormatFromExtension returns format from file extension
func GetFormatFromExtension(filename string) (AudioFormat, error) {
	parts := strings.Split(strings.ToLower(filename), ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("no file extension found")
	}

	ext := parts[len(parts)-1]

	switch ext {
	case "wav":
		return FormatWAV, nil
	case "flac":
		return FormatFLAC, nil
	case "mp3":
		return FormatMP3, nil
	case "ogg":
		return FormatOGG, nil
	case "m4a":
		return FormatM4A, nil
	case "aac":
		return FormatAAC, nil
	case "pcm":
		return FormatPCM, nil
	default:
		return "", fmt.Errorf("unsupported format: %s", ext)
	}
}

// GetMimeType returns MIME type for audio format
func (f AudioFormat) GetMimeType() string {
	switch f {
	case FormatWAV:
		return "audio/wav"
	case FormatFLAC:
		return "audio/flac"
	case FormatMP3:
		return "audio/mpeg"
	case FormatOGG:
		return "audio/ogg"
	case FormatM4A:
		return "audio/mp4"
	case FormatAAC:
		return "audio/aac"
	case FormatPCM:
		return "audio/pcm"
	default:
		return "application/octet-stream"
	}
}

// GetFileExtension returns file extension for audio format
func (f AudioFormat) GetFileExtension() string {
	switch f {
	case FormatWAV:
		return ".wav"
	case FormatFLAC:
		return ".flac"
	case FormatMP3:
		return ".mp3"
	case FormatOGG:
		return ".ogg"
	case FormatM4A:
		return ".m4a"
	case FormatAAC:
		return ".aac"
	case FormatPCM:
		return ".pcm"
	default:
		return ".bin"
	}
}

// IsLossy returns true if the format is lossy
func (f AudioFormat) IsLossy() bool {
	switch f {
	case FormatMP3, FormatOGG, FormatAAC, FormatM4A:
		return true
	default:
		return false
	}
}

// IsCompressed returns true if the format uses compression
func (f AudioFormat) IsCompressed() bool {
	switch f {
	case FormatFLAC, FormatMP3, FormatOGG, FormatAAC, FormatM4A:
		return true
	default:
		return false
	}
}

// Validate validates audio configuration with comprehensive checks
func (c *AudioConfig) Validate() error {
	var errs []error

	if !IsSupportedFormat(c.Format) {
		errs = append(errs, fmt.Errorf("unsupported format: %s", c.Format))
	}

	if c.SampleRate <= 0 {
		errs = append(errs, fmt.Errorf("sample rate must be positive, got %d", c.SampleRate))
	}

	if c.SampleRate < 8000 {
		errs = append(errs, fmt.Errorf("sample rate must be at least 8000 Hz for audio quality, got %d", c.SampleRate))
	}

	if c.SampleRate > 192000 {
		errs = append(errs, fmt.Errorf("sample rate must not exceed 192000 Hz, got %d", c.SampleRate))
	}

	if c.Channels <= 0 {
		errs = append(errs, fmt.Errorf("channels must be positive, got %d", c.Channels))
	}

	if c.Channels > 8 {
		errs = append(errs, fmt.Errorf("channels must not exceed 8, got %d", c.Channels))
	}

	validBitsPerSample := map[int]bool{8: true, 16: true, 24: true, 32: true}
	if !validBitsPerSample[c.BitsPerSample] {
		errs = append(errs, fmt.Errorf("bits per sample must be 8, 16, 24, or 32, got %d", c.BitsPerSample))
	}

	if c.Format.IsLossy() {
		if c.Quality < 0 || c.Quality > 100 {
			errs = append(errs, fmt.Errorf("quality must be between 0-100 for lossy formats, got %d", c.Quality))
		}
		if c.Bitrate < 0 {
			errs = append(errs, fmt.Errorf("bitrate must be non-negative for lossy formats, got %d", c.Bitrate))
		}
		if c.Bitrate > 0 && c.Bitrate < 32 {
			errs = append(errs, fmt.Errorf("bitrate too low for quality, minimum 32 kbps recommended, got %d", c.Bitrate))
		}
		if c.Bitrate > 320 {
			errs = append(errs, fmt.Errorf("bitrate too high, maximum 320 kbps supported, got %d", c.Bitrate))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("audio config validation failed: %v", errs)
	}

	return nil
}

// ValidateConfig is an alias for Validate for backward compatibility
func (c *AudioConfig) ValidateConfig() error {
	return c.Validate()
}

// GetBytesPerSample returns bytes per sample
func (c *AudioConfig) GetBytesPerSample() int {
	return c.BitsPerSample / 8
}

// GetBytesPerSecond returns bytes per second
func (c *AudioConfig) GetBytesPerSecond() int {
	return c.SampleRate * c.Channels * c.GetBytesPerSample()
}

// EstimateFileSize estimates file size for given duration
func (c *AudioConfig) EstimateFileSize(durationSeconds float64) int64 {
	bytesPerSecond := float64(c.GetBytesPerSecond())

	// For compressed formats, use bitrate if available
	if c.Format.IsCompressed() && c.Bitrate > 0 {
		bytesPerSecond = float64(c.Bitrate*1000) / 8.0
	}

	return int64(bytesPerSecond * durationSeconds)
}