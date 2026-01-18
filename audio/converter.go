package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/go-audio/wav"
)

// Converter handles audio format conversion
type Converter struct {
	resampler *Resampler
	config    *ConverterConfig
}

// ConverterConfig represents converter configuration
type ConverterConfig struct {
	EnableResampling    bool    `json:"enable_resampling"`
	EnableNormalization bool    `json:"enable_normalization"`
	TargetRMS           float32 `json:"target_rms"`
	TempBufferSize      int     `json:"temp_buffer_size"`
	NormalizeFactor     float32 `json:"normalize_factor"`
}

// DefaultConverterConfig returns default converter configuration
func DefaultConverterConfig() *ConverterConfig {
	return &ConverterConfig{
		EnableResampling:    true,
		EnableNormalization: true,
		TargetRMS:           0.1, // Standard normalization level
		TempBufferSize:      8192,
		NormalizeFactor:     32768.0,
	}
}

// Validate validates converter configuration
func (c *ConverterConfig) Validate() error {
	var errs []error

	if c.TargetRMS <= 0 {
		errs = append(errs, fmt.Errorf("target RMS must be positive, got %f", c.TargetRMS))
	}

	if c.TargetRMS > 1.0 {
		errs = append(errs, fmt.Errorf("target RMS must not exceed 1.0, got %f", c.TargetRMS))
	}

	if c.TempBufferSize <= 0 {
		errs = append(errs, fmt.Errorf("temp buffer size must be positive, got %d", c.TempBufferSize))
	}

	if c.TempBufferSize > 1048576 { // 1MB limit
		errs = append(errs, fmt.Errorf("temp buffer size must not exceed 1MB, got %d", c.TempBufferSize))
	}

	if c.NormalizeFactor <= 0 {
		errs = append(errs, fmt.Errorf("normalize factor must be positive, got %f", c.NormalizeFactor))
	}

	if len(errs) > 0 {
		return fmt.Errorf("converter config validation failed: %v", errs)
	}

	return nil
}

// NewConverter creates a new audio converter
func NewConverter(config *ConverterConfig) (*Converter, error) {
	if config == nil {
		config = DefaultConverterConfig()
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid converter configuration: %w", err)
	}

	resampleConfig := &ResampleConfig{
		Method:       MethodCubic,
		Quality:      QualityMedium,
		FilterLength: 16,
		UseSIMD:      false, // SIMD integration deferred
	}

	return &Converter{
		resampler: NewResampler(resampleConfig),
		config:    config,
	}, nil
}

// ConvertAudio converts audio data between different formats and configurations.
//
// This function performs format conversion, sample rate resampling, and channel
// conversion as needed. It supports WAV, PCM, and other uncompressed formats.
// Compressed formats (FLAC, MP3, OGG, M4A) are not supported due to CGO dependencies.
//
// Supported conversions:
//   - Format conversion: WAV ↔ PCM
//   - Sample rate conversion: any rate to any rate using high-quality resampling
//   - Channel conversion: mono ↔ stereo with proper downmixing/upmixing
//   - Normalization: optional audio level normalization
//
// Parameters:
//   - inputData: raw audio data in the input format
//   - inputConfig: configuration describing the input audio format
//   - outputConfig: configuration describing the desired output format
//
// Returns converted audio data or an error if conversion fails.
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (c *Converter) ConvertAudio(inputData []byte, inputConfig, outputConfig *AudioConfig) ([]byte, error) {
	if len(inputData) == 0 {
		return nil, fmt.Errorf("input data is empty")
	}

	// Validate configurations
	if err := inputConfig.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid input config: %v", err)
	}
	if err := outputConfig.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid output config: %v", err)
	}

	// Parse input format
	samples, err := c.decodeAudio(inputData, inputConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode input: %v", err)
	}

	// Apply processing
	if c.config.EnableResampling {
		// Resample if needed
		if inputConfig.SampleRate != outputConfig.SampleRate {
			samples, err = c.resampler.Resample(samples, inputConfig.SampleRate, outputConfig.SampleRate)
			if err != nil {
				return nil, fmt.Errorf("failed to resample: %v", err)
			}
		}

		// Convert channels if needed
		if inputConfig.Channels != outputConfig.Channels {
			samples, err = c.resampler.ConvertChannels(samples, inputConfig.Channels, outputConfig.Channels)
			if err != nil {
				return nil, fmt.Errorf("failed to convert channels: %v", err)
			}
		}
	}

	// Normalize if enabled
	if c.config.EnableNormalization {
		samples = c.resampler.NormalizeAudio(samples, c.config.TargetRMS)
	}

	// Encode to output format
	outputData, err := c.encodeAudio(samples, outputConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to encode output: %v", err)
	}

	return outputData, nil
}

// decodeAudio decodes audio data to float32 samples
func (c *Converter) decodeAudio(data []byte, config *AudioConfig) ([]float32, error) {
	switch config.Format {
	case FormatWAV:
		return c.decodeWAV(data, config)
	case FormatPCM:
		return c.decodePCM(data, config)
	case FormatFLAC:
		return c.decodeFLAC(data, config)
	case FormatMP3:
		return c.decodeMP3(data, config)
	case FormatOGG:
		return c.decodeOGG(data, config)
	case FormatM4A:
		return c.decodeM4A(data, config)
	default:
		return nil, fmt.Errorf("unsupported input format: %s", config.Format)
	}
}

// encodeAudio encodes float32 samples to audio data
func (c *Converter) encodeAudio(samples []float32, config *AudioConfig) ([]byte, error) {
	switch config.Format {
	case FormatWAV:
		return c.encodeWAV(samples, config)
	case FormatPCM:
		return c.encodePCM(samples, config)
	case FormatFLAC:
		return c.encodeFLAC(samples, config)
	case FormatMP3:
		return c.encodeMP3(samples, config)
	case FormatOGG:
		return c.encodeOGG(samples, config)
	case FormatM4A:
		return c.encodeM4A(samples, config)
	default:
		return nil, fmt.Errorf("unsupported output format: %s", config.Format)
	}
}

// decodeWAV decodes WAV format (simplified implementation)
func (c *Converter) decodeWAV(data []byte, config *AudioConfig) ([]float32, error) {
	if len(data) < 44 {
		return nil, fmt.Errorf("WAV data too short")
	}

	// Skip WAV header (44 bytes) and decode PCM data
	pcmData := data[44:]
	return c.decodePCM(pcmData, config)
}

// encodeWAV encodes to WAV format (simplified implementation)
func (c *Converter) encodeWAV(samples []float32, config *AudioConfig) ([]byte, error) {
	pcmData, err := c.encodePCM(samples, config)
	if err != nil {
		return nil, err
	}

	// Create WAV header
	header := c.createWAVHeader(len(pcmData), config)
	return append(header, pcmData...), nil
}

// createWAVHeader creates a WAV file header
func (c *Converter) createWAVHeader(dataSize int, config *AudioConfig) []byte {
	header := make([]byte, 44)

	// RIFF header
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(36+dataSize)) // File size
	copy(header[8:12], "WAVE")

	// Format chunk
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)                                                                   // Chunk size
	binary.LittleEndian.PutUint16(header[20:22], 1)                                                                    // PCM format
	binary.LittleEndian.PutUint16(header[22:24], uint16(config.Channels))                                              // Channels
	binary.LittleEndian.PutUint32(header[24:28], uint32(config.SampleRate))                                            // Sample rate
	binary.LittleEndian.PutUint32(header[28:32], uint32(config.SampleRate*config.Channels*config.GetBytesPerSample())) // Byte rate
	binary.LittleEndian.PutUint16(header[32:34], uint16(config.Channels*config.GetBytesPerSample()))                   // Block align
	binary.LittleEndian.PutUint16(header[34:36], uint16(config.BitsPerSample))                                         // Bits per sample

	// Data chunk
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(dataSize)) // Data size

	return header
}

// decodePCM decodes raw PCM data
func (c *Converter) decodePCM(data []byte, config *AudioConfig) ([]float32, error) {
	bytesPerSample := config.GetBytesPerSample()
	if len(data)%bytesPerSample != 0 {
		return nil, fmt.Errorf("PCM data size not aligned with sample size")
	}

	sampleCount := len(data) / bytesPerSample
	samples := DefaultAudioBufferPool.Get(sampleCount)

	buf := bytes.NewReader(data)

	for i := 0; i < sampleCount; i++ {
		switch config.BitsPerSample {
		case 8:
			var val uint8
			if err := binary.Read(buf, binary.LittleEndian, &val); err != nil {
				DefaultAudioBufferPool.Put(samples)
				return nil, err
			}
			samples[i] = float32(val-128) / 127.0 // Convert to -1..1 range

		case 16:
			var val int16
			if err := binary.Read(buf, binary.LittleEndian, &val); err != nil {
				DefaultAudioBufferPool.Put(samples)
				return nil, err
			}
			samples[i] = float32(val) / c.config.NormalizeFactor

		case 24:
			// Read 3 bytes as 24-bit signed integer
			var val int32
			temp := BytePool.Get(4)
			if _, err := io.ReadFull(buf, temp[:3]); err != nil {
				BytePool.Put(temp)
				DefaultAudioBufferPool.Put(samples)
				return nil, err
			}
			if temp[2]&0x80 != 0 { // Sign extension
				temp[3] = 0xFF
			}
			val = int32(binary.LittleEndian.Uint32(temp))
			BytePool.Put(temp)
			samples[i] = float32(val>>8) / 8388608.0

		case 32:
			var val int32
			if err := binary.Read(buf, binary.LittleEndian, &val); err != nil {
				DefaultAudioBufferPool.Put(samples)
				return nil, err
			}
			samples[i] = float32(val) / 2147483648.0

		default:
			DefaultAudioBufferPool.Put(samples)
			return nil, fmt.Errorf("unsupported bits per sample: %d", config.BitsPerSample)
		}
	}

	return samples, nil
}

// encodePCM encodes to raw PCM data
func (c *Converter) encodePCM(samples []float32, config *AudioConfig) ([]byte, error) {
	// Estimate output size and get buffer from pool
	bytesPerSample := config.GetBytesPerSample()
	outputSize := len(samples) * bytesPerSample
	output := BytePool.Get(outputSize)
	buf := bytes.NewBuffer(output[:0]) // Use pooled buffer as backing

	for _, sample := range samples {
		switch config.BitsPerSample {
		case 8:
			val := uint8((sample * 127.0) + 128.0) // Convert from -1..1 to 0..255
			binary.Write(buf, binary.LittleEndian, val)

		case 16:
			val := int16(sample * c.config.NormalizeFactor) // Convert to -32768..32767 range
			binary.Write(buf, binary.LittleEndian, val)

		case 24:
			val := int32(sample * 8388607.0) // Convert to 24-bit range
			temp := BytePool.Get(3)
			binary.LittleEndian.PutUint32(temp, uint32(val<<8))
			buf.Write(temp[:3])
			BytePool.Put(temp)

		case 32:
			val := int32(sample * 2147483647.0) // Convert to 32-bit range
			binary.Write(buf, binary.LittleEndian, val)

		default:
			BytePool.Put(output)
			return nil, fmt.Errorf("unsupported bits per sample: %d", config.BitsPerSample)
		}
	}

	// Return the actual bytes written
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	BytePool.Put(output)

	return result, nil
}

// Compressed audio format support is not implemented
// These formats require external CGO libraries (libflac, libmp3lame, libvorbis, etc.)
// which introduce deployment complexity and platform dependencies.
//
// For production use, consider:
// 1. Pre-converting compressed audio to WAV/PCM before processing
// 2. Using external services for format conversion
// 3. Adding CGO dependencies if deployment constraints allow
//
// Currently supported formats: WAV, PCM (uncompressed only)

func (c *Converter) decodeFLAC(data []byte, config *AudioConfig) ([]float32, error) {
	return nil, fmt.Errorf("FLAC format not supported - requires libFLAC CGO library. Use WAV or PCM format instead")
}

func (c *Converter) encodeFLAC(samples []float32, config *AudioConfig) ([]byte, error) {
	return nil, fmt.Errorf("FLAC encoding not supported - requires libFLAC CGO library. Use WAV format instead")
}

func (c *Converter) decodeMP3(data []byte, config *AudioConfig) ([]float32, error) {
	return nil, fmt.Errorf("MP3 format not supported - requires libmp3lame CGO library. Use WAV or PCM format instead")
}

func (c *Converter) encodeMP3(samples []float32, config *AudioConfig) ([]byte, error) {
	return nil, fmt.Errorf("MP3 encoding not supported - requires libmp3lame CGO library. Use WAV format instead")
}

func (c *Converter) decodeOGG(data []byte, config *AudioConfig) ([]float32, error) {
	return nil, fmt.Errorf("OGG format not supported - requires libvorbis CGO library. Use WAV or PCM format instead")
}

func (c *Converter) encodeOGG(samples []float32, config *AudioConfig) ([]byte, error) {
	return nil, fmt.Errorf("OGG encoding not supported - requires libvorbis CGO library. Use WAV format instead")
}

func (c *Converter) decodeM4A(data []byte, config *AudioConfig) ([]float32, error) {
	return nil, fmt.Errorf("M4A format not supported - requires Core Audio or ffmpeg CGO library. Use WAV or PCM format instead")
}

func (c *Converter) encodeM4A(samples []float32, config *AudioConfig) ([]byte, error) {
	return nil, fmt.Errorf("M4A encoding not supported - requires Core Audio or ffmpeg CGO library. Use WAV format instead")
}

// ConvertToFloat32 converts compressed or encoded audio data to float32 samples.
//
// This is the primary entry point for converting audio data into the internal
// float32 format used by VoiceKit for processing. The samples are normalized
// to the range [-1, 1] for consistent processing across different input formats.
//
// Supported input formats:
//   - WAV (various bit depths and channel configurations)
//   - PCM (raw linear PCM data)
//   - Compressed formats: NOT SUPPORTED (returns informative error)
//
// Parameters:
//   - data: raw audio data bytes
//   - config: audio configuration describing the input format
//
// Returns float32 samples in range [-1, 1] or an error if conversion fails.
//
// Thread-safe: can be called concurrently from multiple goroutines.
func (c *Converter) ConvertToFloat32(data []byte, config *AudioConfig) ([]float32, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("input data cannot be empty")
	}
	if config == nil {
		return nil, fmt.Errorf("audio config cannot be nil")
	}
	if err := config.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid audio config: %v", err)
	}
	return c.decodeAudio(data, config)
}

// ConvertFromFloat32 converts float32 samples to specified audio format
func (c *Converter) ConvertFromFloat32(samples []float32, config *AudioConfig) ([]byte, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("audio samples cannot be empty")
	}
	if config == nil {
		return nil, fmt.Errorf("audio config cannot be nil")
	}
	if err := config.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid audio config: %v", err)
	}
	return c.encodeAudio(samples, config)
}

// GetSupportedConversions returns supported format conversions
// Note: Only uncompressed formats (WAV, PCM) are fully supported.
// Compressed formats (FLAC, MP3, OGG, M4A) require external CGO libraries.
func (c *Converter) GetSupportedConversions() map[string][]string {
	return map[string][]string{
		"wav": {"pcm"}, // WAV ↔ PCM conversion supported
		"pcm": {"wav"}, // PCM ↔ WAV conversion supported
		// Compressed formats not supported without CGO dependencies
	}
}

// ValidateConversion checks if a conversion is supported
func (c *Converter) ValidateConversion(from, to AudioFormat) bool {
	supported, exists := c.GetSupportedConversions()[string(from)]
	if !exists {
		return false
	}

	for _, format := range supported {
		if format == string(to) {
			return true
		}
	}
	return false
}

// ParseWAVFile parses WAV file and returns audio data
func (c *Converter) ParseWAVFile(data []byte) ([]float32, int, error) {
	// Read WAV file
	reader := bytes.NewReader(data)
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
	samples := DefaultAudioBufferPool.Get(len(buffer.Data))
	for i, sample := range buffer.Data {
		// Convert int to float32, range [-1.0, 1.0]
		samples[i] = float32(sample) / c.config.NormalizeFactor
	}

	// If stereo, convert to mono (take average)
	if numChannels == 2 {
		monoSamples := DefaultAudioBufferPool.Get(len(samples) / 2)
		for i := 0; i < len(monoSamples); i++ {
			monoSamples[i] = (samples[i*2] + samples[i*2+1]) / 2.0
		}
		// Return stereo samples to pool
		DefaultAudioBufferPool.Put(samples)
		samples = monoSamples
	}

	return samples, sampleRate, nil
}
