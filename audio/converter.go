package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/go-audio/wav"
	"github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
	"github.com/mewkiz/flac"
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

// decodeWAV decodes WAV format using the RIFF/WAV parser instead of assuming a 44-byte header.
func (c *Converter) decodeWAV(data []byte, config *AudioConfig) ([]float32, error) {
	reader := bytes.NewReader(data)
	decoder := wav.NewDecoder(reader)
	if !decoder.IsValidFile() {
		return nil, fmt.Errorf("invalid WAV file")
	}
	if decoder.WavAudioFormat != 1 {
		return nil, fmt.Errorf("unsupported WAV audio format %d: only PCM is supported", decoder.WavAudioFormat)
	}
	if int(decoder.NumChans) != config.Channels {
		return nil, fmt.Errorf("WAV channel count %d does not match config channels %d", decoder.NumChans, config.Channels)
	}
	if int(decoder.SampleRate) != config.SampleRate {
		return nil, fmt.Errorf("WAV sample rate %d does not match config sample rate %d", decoder.SampleRate, config.SampleRate)
	}
	if int(decoder.BitDepth) != config.BitsPerSample {
		return nil, fmt.Errorf("WAV bit depth %d does not match config bits per sample %d", decoder.BitDepth, config.BitsPerSample)
	}

	return c.decodeWAVPCM(decoder, config)
}

func (c *Converter) decodeWAVPCM(decoder *wav.Decoder, config *AudioConfig) ([]float32, error) {
	if err := decoder.FwdToPCM(); err != nil {
		return nil, fmt.Errorf("failed to locate WAV PCM data: %w", err)
	}
	if decoder.PCMChunk == nil {
		return nil, fmt.Errorf("WAV PCM chunk not found")
	}

	bytesPerSample := config.GetBytesPerSample()
	if decoder.PCMSize%bytesPerSample != 0 {
		return nil, fmt.Errorf("WAV PCM data size not aligned with sample size")
	}

	sampleCount := decoder.PCMSize / bytesPerSample
	samples := DefaultAudioBufferPool.Get(sampleCount)
	if sampleCount == 0 {
		return samples, nil
	}

	chunkSamples := c.config.TempBufferSize
	if chunkSamples > sampleCount {
		chunkSamples = sampleCount
	}
	chunkBytes := chunkSamples * bytesPerSample
	buffer := BytePool.Get(chunkBytes)
	defer BytePool.Put(buffer)

	written := 0
	for written < sampleCount {
		remainingSamples := sampleCount - written
		samplesThisChunk := chunkSamples
		if remainingSamples < samplesThisChunk {
			samplesThisChunk = remainingSamples
		}
		bytesThisChunk := samplesThisChunk * bytesPerSample

		if _, err := io.ReadFull(decoder.PCMChunk.R, buffer[:bytesThisChunk]); err != nil {
			DefaultAudioBufferPool.Put(samples)
			return nil, fmt.Errorf("failed to decode WAV PCM data: %w", err)
		}
		if err := c.decodePCMBytesInto(samples[written:written+samplesThisChunk], buffer[:bytesThisChunk], config); err != nil {
			DefaultAudioBufferPool.Put(samples)
			return nil, err
		}
		written += samplesThisChunk
	}

	return samples, nil
}

func (c *Converter) decodePCMBytesInto(samples []float32, data []byte, config *AudioConfig) error {
	bytesPerSample := config.GetBytesPerSample()
	if len(data) != len(samples)*bytesPerSample {
		return fmt.Errorf("PCM data size not aligned with sample size")
	}

	switch config.BitsPerSample {
	case 8:
		for i, val := range data {
			samples[i] = float32(int(val)-128) / 128.0
		}
	case 16:
		for i := range samples {
			offset := i * 2
			val := int16(binary.LittleEndian.Uint16(data[offset : offset+2]))
			samples[i] = float32(val) / c.config.NormalizeFactor
		}
	case 24:
		for i := range samples {
			offset := i * 3
			val := int32(data[offset]) | int32(data[offset+1])<<8 | int32(data[offset+2])<<16
			if val&0x800000 != 0 {
				val |= ^int32(0xFFFFFF)
			}
			samples[i] = float32(val) / 8388608.0
		}
	case 32:
		for i := range samples {
			offset := i * 4
			val := int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
			samples[i] = float32(val) / 2147483648.0
		}
	default:
		return fmt.Errorf("unsupported bits per sample: %d", config.BitsPerSample)
	}

	return nil
}

func (c *Converter) decodePCM(data []byte, config *AudioConfig) ([]float32, error) {
	bytesPerSample := config.GetBytesPerSample()
	if len(data)%bytesPerSample != 0 {
		return nil, fmt.Errorf("PCM data size not aligned with sample size")
	}

	sampleCount := len(data) / bytesPerSample
	samples := DefaultAudioBufferPool.Get(sampleCount)
	if err := c.decodePCMBytesInto(samples, data, config); err != nil {
		DefaultAudioBufferPool.Put(samples)
		return nil, err
	}
	return samples, nil
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

// encodePCM encodes to raw PCM data
func (c *Converter) encodePCM(samples []float32, config *AudioConfig) ([]byte, error) {
	bytesPerSample := config.GetBytesPerSample()
	outputSize := len(samples) * bytesPerSample
	output := make([]byte, outputSize)

	for i, sample := range samples {
		sample = clampSample(sample)
		switch config.BitsPerSample {
		case 8:
			output[i] = uint8(math.Round(float64((sample + 1.0) * 127.5)))
		case 16:
			offset := i * bytesPerSample
			binary.LittleEndian.PutUint16(output[offset:offset+2], uint16(floatToInt16(sample)))
		case 24:
			offset := i * bytesPerSample
			val := floatToInt24(sample)
			output[offset] = byte(val)
			output[offset+1] = byte(val >> 8)
			output[offset+2] = byte(val >> 16)
		case 32:
			offset := i * bytesPerSample
			binary.LittleEndian.PutUint32(output[offset:offset+4], uint32(floatToInt32(sample)))
		default:
			return nil, fmt.Errorf("unsupported bits per sample: %d", config.BitsPerSample)
		}
	}

	return output, nil
}

func clampSample(sample float32) float32 {
	if sample > 1 {
		return 1
	}
	if sample < -1 {
		return -1
	}
	return sample
}

func floatToInt16(sample float32) int16 {
	if sample <= -1 {
		return -32768
	}
	return int16(math.Round(float64(sample * 32767)))
}

func floatToInt24(sample float32) int32 {
	if sample <= -1 {
		return -8388608
	}
	return int32(math.Round(float64(sample * 8388607)))
}

func floatToInt32(sample float32) int32 {
	if sample <= -1 {
		return -2147483648
	}
	return int32(math.Round(float64(sample * 2147483647)))
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

// validateDecodedFormat checks a decoded stream's native rate/channels against
// the caller's expectations. A zero config value means "accept whatever the file
// declares", matching how the compressed formats carry their own rate/channels.
func validateDecodedFormat(config *AudioConfig, sampleRate, channels int) error {
	if config.SampleRate != 0 && config.SampleRate != sampleRate {
		return fmt.Errorf("decoded sample rate %d does not match config sample rate %d", sampleRate, config.SampleRate)
	}
	if config.Channels != 0 && config.Channels != channels {
		return fmt.Errorf("decoded channel count %d does not match config channels %d", channels, config.Channels)
	}
	return nil
}

// decodeFLAC decodes a FLAC stream (pure Go, no CGO) into interleaved float32
// samples in [-1, 1].
func (c *Converter) decodeFLAC(data []byte, config *AudioConfig) ([]float32, error) {
	stream, err := flac.New(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize FLAC decoder: %w", err)
	}
	defer stream.Close()

	info := stream.Info
	if err := validateDecodedFormat(config, int(info.SampleRate), int(info.NChannels)); err != nil {
		return nil, err
	}
	if info.BitsPerSample == 0 || info.BitsPerSample > 32 {
		return nil, fmt.Errorf("unsupported FLAC bits per sample: %d", info.BitsPerSample)
	}
	scale := float32(int64(1) << (info.BitsPerSample - 1))

	samples := make([]float32, 0, info.NSamples*uint64(info.NChannels))
	for {
		frame, err := stream.ParseNext()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode FLAC frame: %w", err)
		}
		n := len(frame.Subframes[0].Samples)
		for i := 0; i < n; i++ {
			for _, subframe := range frame.Subframes {
				samples = append(samples, float32(subframe.Samples[i])/scale)
			}
		}
	}
	return samples, nil
}

func (c *Converter) encodeFLAC(samples []float32, config *AudioConfig) ([]byte, error) {
	return nil, fmt.Errorf("FLAC encoding not supported - requires libFLAC CGO library. Use WAV format instead")
}

// decodeMP3 decodes an MP3 stream (pure Go, no CGO) into interleaved float32
// samples in [-1, 1]. go-mp3 always emits 16-bit little-endian stereo.
func (c *Converter) decodeMP3(data []byte, config *AudioConfig) ([]float32, error) {
	decoder, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MP3 decoder: %w", err)
	}
	if err := validateDecodedFormat(config, decoder.SampleRate(), 2); err != nil {
		return nil, err
	}

	raw, err := io.ReadAll(decoder)
	if err != nil {
		return nil, fmt.Errorf("failed to decode MP3 stream: %w", err)
	}
	if len(raw)%2 != 0 {
		return nil, fmt.Errorf("MP3 PCM data is not 16-bit aligned")
	}
	n := len(raw) / 2
	samples := make([]float32, n)
	for i := 0; i < n; i++ {
		v := int16(binary.LittleEndian.Uint16(raw[i*2:]))
		samples[i] = float32(v) / c.config.NormalizeFactor
	}
	return samples, nil
}

func (c *Converter) encodeMP3(samples []float32, config *AudioConfig) ([]byte, error) {
	return nil, fmt.Errorf("MP3 encoding not supported - requires libmp3lame CGO library. Use WAV format instead")
}

// decodeOGG decodes an Ogg/Vorbis stream (pure Go, no CGO) into interleaved
// float32 samples in [-1, 1].
func (c *Converter) decodeOGG(data []byte, config *AudioConfig) ([]float32, error) {
	samples, format, err := oggvorbis.ReadAll(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode Ogg/Vorbis stream: %w", err)
	}
	if err := validateDecodedFormat(config, format.SampleRate, format.Channels); err != nil {
		return nil, err
	}
	return samples, nil
}

func (c *Converter) encodeOGG(samples []float32, config *AudioConfig) ([]byte, error) {
	return nil, fmt.Errorf("OGG encoding not supported - requires libvorbis CGO library. Use WAV format instead")
}

// decodeM4A remains unsupported: there is no maintained pure-Go AAC/M4A decoder,
// and VoiceKit avoids CGO for audio ingestion.
func (c *Converter) decodeM4A(data []byte, config *AudioConfig) ([]float32, error) {
	return nil, fmt.Errorf("M4A/AAC decoding is not supported (no pure-Go decoder); transcode to WAV, FLAC, MP3, or Ogg first")
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
	if decoder.WavAudioFormat != 1 {
		return nil, 0, fmt.Errorf("unsupported WAV audio format %d: only PCM is supported", decoder.WavAudioFormat)
	}
	config := &AudioConfig{
		Format:        FormatWAV,
		SampleRate:    sampleRate,
		Channels:      numChannels,
		BitsPerSample: int(decoder.BitDepth),
	}
	samples, err := c.decodeWAVPCM(decoder, config)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decode audio: %v", err)
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
