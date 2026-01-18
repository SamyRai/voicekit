# VoiceKit

A reusable Go library for voice processing operations including speaker recognition, diarization, and audio processing. Designed to be easily copied to other projects with minimal dependencies.

## Features

- **Real-Time ASR Streaming**: Streaming automatic speech recognition with <500ms latency
- **Speaker Recognition**: Register speakers, identify unknown speakers, and verify speaker identities
- **Audio Processing**: Resampling, format conversion, normalization, and channel conversion
- **Speaker Diarization**: Segment audio by speaker and integrate with ASR results
- **Voice Activity Detection**: VAD integration for intelligent speech processing
- **Modular Design**: Use individual components or the unified VoiceKit interface
- **Clean API**: Simple factory functions and consistent interfaces
- **Production Ready**: Comprehensive monitoring, error handling, and performance optimization

## Installation

Copy the `voicekit` directory to your project and add it to your Go module:

```bash
# Add to your project
cp -r voicekit /path/to/your/project/

# Update your go.mod if needed
go mod tidy
```

## Architecture Overview

VoiceKit implements a modular, layered architecture for voice processing operations:

### Core Architecture Layers

```
┌─────────────────────────────────────────────────────────────────┐
│                      VoiceKit (Unified API)                      │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                 Component Managers                       │    │
│  │  ┌─────────────┬─────────────┬─────────────┬───────┐   │    │
│  │  │  ASR        │ Audio       │ Speaker     │ Dia-  │   │    │
│  │  │ Streaming   │ Processing  │ Recognition │ riza- │   │    │
│  │  │             │             │             │ tion  │   │    │
│  │  └─────────────┴─────────────┴─────────────┴───────┘   │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
                                       │
                         ┌─────────────┼─────────────┐
                         │             │             │
                 ┌───────▼────┐ ┌──────▼────┐ ┌─────▼─────┐
                 │   ASR      │ │   Audio    │ │   Speaker  │
                 │ Streaming  │ │ Processing│ │ Recognition│
                 │ Engine     │ │ Primitives│ │ Engine     │
                 └────────────┘ └────────────┘ └────────────┘
                         │             │             │
                 ┌───────▼────┐ ┌──────▼────┐ ┌─────▼─────┐
                 │   VAD      │ │   Resam-  │ │   Sherpa-  │
                 │ Processing │ │   pling   │ │   ONNX     │
                 └────────────┘ └────────────┘ └────────────┘
```

### Component Responsibilities

- **VoiceKit (Top Level)**: Orchestrates component interactions, provides unified API
- **ASR Streaming**: Real-time speech recognition with streaming session management
- **Audio Processing**: Handles sample rate conversion, format transcoding, normalization
- **Speaker Recognition**: Manages speaker embeddings, similarity computation, database operations
- **Diarization**: Performs audio segmentation, speaker clustering, result integration
- **VAD Processing**: Voice activity detection for intelligent audio segmentation

### Technical Approaches

#### Audio Processing Architecture
- **Resampling**: Implements multiple interpolation algorithms (Linear, Cubic, Lanczos, Sinc)
- **Format Conversion**: Uses streaming approach with configurable buffer sizes
- **Channel Handling**: Supports mono↔stereo conversion with proper downmixing/upmixing
- **Memory Management**: Implements object pooling for float32 slices to reduce GC pressure

#### Speaker Recognition Architecture
- **Embedding Extraction**: Leverages Sherpa-ONNX neural network inference
- **Similarity Computation**: Uses cosine similarity on normalized embedding vectors
- **Database Design**: JSON-based persistence with in-memory caching for fast lookups
- **Thread Safety**: RWMutex-based concurrent access to speaker database

#### ASR Streaming Architecture
- **Real-Time Processing**: Streaming session management with <500ms end-to-end latency
- **Model Registry**: Dynamic model selection based on language and performance requirements
- **Audio Buffering**: Cache-aware audio buffering with overlap handling for continuous processing
- **VAD Integration**: Voice activity detection for intelligent speech/non-speech segmentation
- **Multi-Model Support**: Whisper, Nemotron, Parakeet, Moonshine with quantization options

#### Diarization Architecture
- **Segmentation Strategies**: Multiple algorithms (VAD-based, silence-based, fixed-length)
- **Clustering**: Agglomerative hierarchical clustering with similarity thresholds
- **Integration**: Time-based alignment between audio segments and ASR word timestamps
- **Speaker Database Abstraction**: Interface-based design for pluggable speaker backends

## Technical Implementation Details

### Memory Management Strategy

#### Object Pooling
```go
// Audio processing uses object pooling to reduce GC overhead
type float32Pool struct {
    pool sync.Pool
}

func (p *float32Pool) Get(size int) []float32 {
    // Returns pooled slice or allocates new one
}

func (p *float32Pool) Put(slice []float32) {
    // Returns slice to pool for reuse
}
```

#### Buffer Management
- **Streaming Processing**: Audio data processed in configurable chunks (default 8KB)
- **Zero-Copy Operations**: Where possible, slices reference original data
- **Pre-allocated Buffers**: Reuse buffers for resampling and conversion operations

### Performance Optimizations

#### Audio Processing
- **SIMD-Compatible**: Algorithms designed for potential SIMD vectorization
- **Cache-Friendly**: Sequential memory access patterns in resampling loops
- **Branch Prediction**: Algorithms structured to minimize branch mispredictions

#### Speaker Recognition
- **Batch Processing**: Sherpa-ONNX processes multiple audio segments efficiently
- **Embedding Caching**: Speaker embeddings cached in memory for fast similarity searches
- **Similarity Optimization**: Early termination in similarity searches using thresholds

#### Diarization
- **Incremental Processing**: Segments processed as they become available
- **Parallel Clustering**: Potential for parallel clustering of independent segments
- **Lazy Evaluation**: Results computed only when requested

### Threading and Concurrency

#### Synchronization Strategy
- **RWMutex**: Reader-writer locks for database operations (multiple readers, exclusive writer)
- **Atomic Operations**: Used for statistics counters in high-throughput scenarios
- **Channel-Based**: Internal communication between processing stages

#### Goroutine Safety
- **Stateless Processing**: Audio processing functions are stateless and goroutine-safe
- **Immutable Results**: Result structures are immutable after creation
- **Interface-Based**: Dependency injection allows for safe concurrent testing

### Error Handling Architecture

#### Error Types
```go
// Structured error types with context
type AudioError struct {
    Op   string    // Operation that failed
    Format string  // Audio format involved
    Err  error     // Underlying error
}

type SpeakerError struct {
    Op        string // Operation
    SpeakerID string // Speaker involved
    Err       error  // Underlying error
}
```

#### Error Propagation
- **Wrapped Errors**: All errors implement `Unwrap()` for error chain inspection
- **Context Preservation**: Errors maintain operation context for debugging
- **Graceful Degradation**: Components fail independently without cascading failures

### Integration Patterns

#### ASR Integration
- **Word-Level Alignment**: Uses timestamp information for precise speaker-text alignment
- **Confidence Propagation**: Combines audio and speaker recognition confidence scores
- **Metadata Preservation**: Maintains original ASR metadata alongside speaker information

#### WebSocket Streaming
- **Real-time Processing**: Designed for streaming audio with low latency
- **Stateful Sessions**: Maintains per-session state for continuous processing
- **Message Batching**: Groups related results for efficient network transmission

### Extension Points

#### Custom Speaker Backends
```go
type SpeakerDatabase interface {
    GetSpeakerEmbedding(speakerID string) ([]float32, error)
    CalculateSimilarity(emb1, emb2 []float32) float32
    RegisterSpeakerEmbedding(id string, emb []float32) error
}
```

#### Custom Audio Processors
- **Resampler Interface**: Pluggable resampling algorithms
- **Format Handlers**: Extensible audio format support
- **Normalizer Interface**: Custom normalization strategies

#### Logger Integration
```go
type Logger interface {
    Infof(format string, args ...interface{})
    Warnf(format string, args ...interface{})
    Errorf(format string, args ...interface{})
}
```

## Dependencies

### Core Dependencies

#### Sherpa-ONNX (github.com/k2-fsa/sherpa-onnx-go)
- **Role**: Neural network inference for speaker embedding extraction
- **Architecture**: ONNX Runtime integration with platform-specific binaries
- **Performance**: GPU acceleration support, optimized for real-time processing
- **Memory**: Models loaded once, shared across processing sessions
- **Threading**: Thread-safe inference with configurable thread pools

#### WAV Library (github.com/go-audio/wav)
- **Role**: WAV file parsing and PCM data extraction
- **Implementation**: Streaming decoder with minimal memory footprint
- **Compatibility**: Supports various WAV formats (mono/stereo, different bit depths)

### Internal Dependencies

#### Logger Interface
- **Purpose**: Decoupled logging allows integration with various logging frameworks
- **Default**: No-op implementation for minimal dependency footprint
- **Integration**: Compatible with logrus, zap, standard library log, etc.

#### Configuration System
- **Approach**: Struct-based configuration with validation
- **Defaults**: Sensible defaults for all parameters
- **Validation**: Compile-time and runtime configuration validation

## Performance Characteristics

### Latency Breakdown
- **Audio Conversion**: O(n) where n is sample count, dominated by resampling
- **Speaker Recognition**: O(m*k) where m is speakers, k is embedding dimension
- **Diarization**: O(s²) where s is number of segments (clustering complexity)
- **ASR Streaming**: ~500ms per 1-second chunk, <200ms first token (Whisper/Nemotron)

### Memory Usage
- **Audio Buffers**: Proportional to audio duration and sample rate
- **Speaker Database**: Scales with number of enrolled speakers
- **Embedding Cache**: Fixed size per speaker (typically 192 floats)
- **ASR Models**: 100KB-200KB per operation, varies by model/quantization

### Scalability Considerations
- **Horizontal**: Stateless audio processing scales linearly
- **Vertical**: Speaker database operations bottleneck at high concurrency
- **Memory**: Bounded by speaker database size and audio buffer pools

## Design Decisions and Trade-offs

### Modularity vs Performance
- **Decision**: Interface-based design prioritizes modularity over micro-optimizations
- **Trade-off**: Virtual function calls vs direct function calls
- **Mitigation**: Inlining and interface optimizations in Go compiler

### Memory vs Speed
- **Decision**: Object pooling reduces GC pressure but increases memory usage
- **Trade-off**: Higher baseline memory usage for lower latency
- **Rationale**: Voice processing often requires consistent low latency

### Simplicity vs Features
- **Decision**: Core functionality only, advanced features via extension
- **Trade-off**: Smaller API surface vs comprehensive feature set
- **Benefit**: Easier testing, maintenance, and integration

### Platform Compatibility
- **Decision**: Pure Go implementation where possible, CGO only for performance-critical code
- **Trade-off**: Development complexity vs runtime performance
- **Implementation**: Sherpa-ONNX provides cross-platform CGO bindings

## Limitations and Known Issues

### Audio Format Support
- **Compressed Formats**: MP3/FLAC/OGG encoding/decoding are placeholder implementations
- **Metadata**: Format-specific metadata (tags, etc.) not preserved
- **Sample Rates**: Limited testing on extreme sample rates (>192kHz, <8kHz)

### Speaker Recognition
- **Model Dependencies**: Requires specific Sherpa-ONNX model files
- **Embedding Quality**: Performance depends on training data quality
- **Real-time Constraints**: Batch processing may introduce latency

### Diarization
- **Accuracy**: Simple clustering algorithms may not match research implementations
- **Speaker Count**: Limited testing with large numbers of speakers (>20)
- **Language Dependency**: Performance may vary across languages

### ASR Streaming
- **Model Dependencies**: Requires specific Whisper/Nemotron/Parakeet model files
- **GPU Requirements**: High-performance models require GPU for optimal latency
- **Quantization Trade-offs**: INT4 quantization may reduce accuracy in noisy conditions
- **Language Support**: Best performance with English; multilingual support varies

### Memory Management
- **Large Files**: Memory usage scales linearly with audio duration
- **Long Sessions**: Speaker database grows indefinitely without cleanup
- **Goroutine Leaks**: Improper cleanup may leave goroutines running

## Future Enhancements

### Planned Improvements
- **GPU Acceleration**: Enhanced CUDA/OpenCL integration for neural network inference
- **Advanced Clustering**: Integration with scikit-learn style clustering algorithms
- **ASR Model Expansion**: Support for additional models (Distil-Whisper, Moonshine, Kroko)
- **Two-Pass Decoding**: CTC + attention for improved streaming accuracy
- **WebRTC Integration**: Native WebRTC support for ultra-low latency streaming
- **Model Quantization**: 8-bit quantization for reduced memory footprint

### Extension Points
- **Custom Models**: Support for different embedding model architectures
- **Plugin System**: Dynamic loading of audio processing plugins
- **Cloud Integration**: Backend support for cloud-based processing

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "log"

    "github.com/SamyRai/voicekit"
    "github.com/SamyRai/voicekit/audio"
    "github.com/SamyRai/voicekit/speaker"
)

func main() {
    // Create a simple logger
    logger := &simpleLogger{}

    // Configure VoiceKit
    config := &voicekit.Config{
        Audio: voicekit.AudioConfig{
            SampleRate:      16000,
            Channels:        1,
            NormalizeFactor: 32768.0,
        },
        Speaker: voicekit.SpeakerConfig{
            ModelPath:  "/path/to/speaker/model",
            DataDir:    "/path/to/speaker/data",
            Threshold:  0.5,
            Logger:     logger,
        },
        Diarization: voicekit.DiarizationConfig{
            Enabled: true,
            Logger:  logger,
        },
        ASR: voicekit.ASRConfig{
            Enabled:              true,
            DefaultModel:         "whisper_large_v3",
            Language:             "en",
            Quantization:         "int8",
            MaxConcurrentStreams: 10,
            StreamTimeout:        300,
            ChunkSize:            16000,
            VADProvider:          "ten_vad",
            Logger:               logger,
        },
    }

    // Create VoiceKit instance
    vk, err := voicekit.NewVoiceKit(config)
    if err != nil {
        log.Fatal(err)
    }
    defer vk.Close()

    // Load audio data (example: WAV file)
    audioData := loadAudioFile("example.wav")
    inputConfig := audio.DefaultWAVConfig()

    // Process audio through the full pipeline
    speakerResult, diarizationResult, err := vk.ProcessAudio(audioData, inputConfig, "session-123")
    if err != nil {
        log.Fatal(err)
    }

    // Print results
    if speakerResult != nil && speakerResult.Identified {
        fmt.Printf("Identified speaker: %s (%s)\n",
            speakerResult.SpeakerName, speakerResult.SpeakerID)
    }

    if diarizationResult != nil {
        fmt.Printf("Found %d speakers in %d segments\n",
            diarizationResult.SpeakerCount, len(diarizationResult.Segments))
    }
}
```

### ASR Streaming

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/SamyRai/voicekit"
)

func main() {
    // Configure VoiceKit with ASR enabled
    config := &voicekit.Config{
        ASR: voicekit.ASRConfig{
            Enabled:              true,
            DefaultModel:         "whisper_large_v3",
            Language:             "en",
            Quantization:         "int8",
            MaxConcurrentStreams: 10,
            StreamTimeout:        300,
            ChunkSize:            16000,
            VADProvider:          "ten_vad",
        },
    }

    // Create VoiceKit instance
    vk, err := voicekit.NewVoiceKit(config)
    if err != nil {
        log.Fatal(err)
    }
    defer vk.Close()

    // Get ASR service
    asrService := vk.ASR()

    // Simulate streaming audio chunks
    sessionID := "stream-session-1"
    ctx := context.Background()

    // Process audio chunks in a loop (simulate real-time streaming)
    for i := 0; i < 10; i++ {
        // Generate or load audio chunk (1 second at 16kHz)
        audioChunk := generateAudioChunk() // []float32 with 16000 samples

        // Process chunk through ASR
        transcription, err := asrService.ProcessAudioChunk(ctx, sessionID, audioChunk)
        if err != nil {
            log.Printf("ASR processing error: %v", err)
            continue
        }

        // Handle transcription results
        if transcription != nil {
            if transcription.IsPartial {
                fmt.Printf("Partial: %s\n", transcription.Text)
            } else {
                fmt.Printf("Final: %s (confidence: %.2f)\n",
                    transcription.Text, transcription.Confidence)
            }
        }

        // Simulate real-time delay
        time.Sleep(100 * time.Millisecond)
    }

    fmt.Println("ASR streaming session completed")
}

func generateAudioChunk() []float32 {
    // Generate a 1-second audio chunk (16kHz = 16000 samples)
    chunk := make([]float32, 16000)
    // Fill with actual audio data or synthesized speech
    return chunk
}
```

### Speaker Recognition Only

```go
package main

import (
    "github.com/SamyRai/voicekit/speaker"
)

func main() {
    // Create speaker manager
    config := &speaker.Config{
        ModelPath: "/path/to/speaker/model",
        DataDir:   "/path/to/speaker/data",
        Threshold: 0.5,
    }

    manager, err := speaker.NewManager(config)
    if err != nil {
        panic(err)
    }
    defer manager.Close()

    // Register a speaker
    audioData := loadAudioData() // []float32
    err = manager.RegisterSpeaker("john_doe", "John Doe", audioData, 16000)
    if err != nil {
        panic(err)
    }

    // Identify a speaker
    unknownAudio := loadUnknownAudio() // []float32
    result, err := manager.IdentifySpeaker(unknownAudio, 16000)
    if err != nil {
        panic(err)
    }

    if result.Identified {
        println("Identified:", result.SpeakerName)
    } else {
        println("Speaker not recognized")
    }
}
```

### Audio Processing Only

```go
package main

import (
    "github.com/SamyRai/voicekit/audio"
)

func main() {
    // Create audio converter
    config := audio.DefaultConverterConfig()
    converter := audio.NewConverter(config)

    // Convert audio format
    inputData := loadAudioBytes()
    inputConfig := audio.DefaultWAVConfig()
    outputConfig := audio.DefaultMP3Config()

    convertedData, err := converter.ConvertAudio(inputData, inputConfig, outputConfig)
    if err != nil {
        panic(err)
    }

    // Create resampler
    resampleConfig := &audio.ResampleConfig{
        Method: audio.MethodCubic,
        Quality: audio.QualityMedium,
    }
    resampler := audio.NewResampler(resampleConfig)

    // Resample audio
    audioSamples := loadFloat32Samples()
    resampled, err := resampler.Resample(audioSamples, 44100, 16000)
    if err != nil {
        panic(err)
    }

    println("Resampled from 44100Hz to 16000Hz")
}
```

### Diarization Only

```go
package main

import (
    "github.com/SamyRai/voicekit/diarization"
)

func main() {
    // Create diarization manager
    config := diarization.DefaultDiarizationConfig()
    config.Enabled = true

    // Use a mock speaker database (or implement your own)
    speakerDB := &mySpeakerDatabase{}

    manager := diarization.NewManager(config, speakerDB)

    // Process audio for diarization
    audioData := loadAudioSamples() // []float32
    result, err := manager.ProcessAudio(audioData, 16000, "session-123")
    if err != nil {
        panic(err)
    }

    println("Found", result.SpeakerCount, "speakers")
    for _, segment := range result.Segments {
        println("Speaker", segment.SpeakerID, "speaks from",
            segment.StartTime, "to", segment.EndTime)
    }
}
```

## API Reference

### VoiceKit (Unified Interface)

#### `NewVoiceKit(config *Config) (*VoiceKit, error)`
Creates a new VoiceKit instance with all components initialized.

#### `(*VoiceKit) Close()`
Releases all resources held by VoiceKit.

#### `(*VoiceKit) Speaker() *speaker.Manager`
Returns the speaker recognition manager.

#### `(*VoiceKit) Audio() *audio.Converter`
Returns the audio converter.

#### `(*VoiceKit) Diarization() *diarization.Manager`
Returns the diarization manager.

#### `(*VoiceKit) ASR() ASRService`
Returns the ASR service for real-time speech recognition.

#### `(*VoiceKit) ProcessAudio(audioData []byte, inputConfig *audio.AudioConfig, sessionID string) (*speaker.IdentifyResult, *diarization.DiarizationResult, error)`
Processes audio through the full pipeline: conversion, speaker identification, and diarization.

### Speaker Recognition

#### `speaker.NewManager(config *Config) (*Manager, error)`
Creates a new speaker recognition manager.

#### `(*Manager) RegisterSpeaker(speakerID, speakerName string, audioData []float32, sampleRate int) error`
Registers a new speaker with audio sample.

#### `(*Manager) IdentifySpeaker(audioData []float32, sampleRate int) (*IdentifyResult, error)`
Identifies the speaker in the provided audio.

#### `(*Manager) VerifySpeaker(speakerID string, audioData []float32, sampleRate int) (*VerifyResult, error)`
Verifies if the audio belongs to the specified speaker.

#### `(*Manager) GetAllSpeakers() []*SpeakerInfo`
Returns information about all registered speakers.

#### `(*Manager) DeleteSpeaker(speakerID string) error`
Deletes a speaker from the database.

### Audio Processing

#### `audio.NewConverter(config *ConverterConfig) *Converter`
Creates a new audio converter.

#### `(*Converter) ConvertAudio(inputData []byte, inputConfig, outputConfig *AudioConfig) ([]byte, error)`
Converts audio between formats.

#### `(*Converter) ConvertToFloat32(data []byte, config *AudioConfig) ([]float32, error)`
Converts audio data to float32 samples.

#### `audio.NewResampler(config *ResampleConfig) *Resampler`
Creates a new audio resampler.

#### `(*Resampler) Resample(audioData []float32, sourceRate, targetRate int) ([]float32, error)`
Resamples audio to a different sample rate.

### Diarization

#### `diarization.NewManager(config *DiarizationConfig, speakerDB SpeakerDatabase) *Manager`
Creates a new diarization manager.

#### `(*Manager) ProcessAudio(audioData []float32, sampleRate int, sessionID string) (*DiarizationResult, error)`
Processes audio for speaker diarization.

#### `diarization.NewSegmenter(config *DiarizationConfig) *Segmenter`
Creates a new audio segmenter.

#### `(*Segmenter) SegmentBySilence(audioData []float32, sampleRate int) ([]AudioSegment, error)`
Segments audio based on silence detection.

### ASR Streaming

#### `asr.NewService(config *voicekit.ASRConfig) (*Service, error)`
Creates a new ASR service for real-time speech recognition.

#### `(*Service) ProcessAudioChunk(ctx context.Context, sessionID string, audio []float32) (*types.Transcription, error)`
Processes a chunk of audio for streaming ASR, returning partial or final transcription results.

#### `(*Service) RegisterModel(model asr.Model) error`
Registers a new ASR model with the service.

#### `(*Service) GetModel(name string) (asr.Model, bool)`
Retrieves a registered ASR model by name.

#### `(*Service) SelectModel(language string, requirements *types.ModelRequirements) (asr.Model, error)`
Selects the best ASR model based on language and performance requirements.

#### `(*Service) Close() error`
Closes the ASR service and releases all resources.

## Configuration

### Main Config
```go
type Config struct {
    Audio       AudioConfig
    Speaker     SpeakerConfig
    Diarization DiarizationConfig
    ASR         ASRConfig
}
```

### Audio Config
```go
type AudioConfig struct {
    SampleRate      int     // Sample rate in Hz (e.g., 16000)
    Channels        int     // Number of channels (1=mono, 2=stereo)
    NormalizeFactor float32 // Normalization factor for audio levels
}
```

### Speaker Config
```go
type SpeakerConfig struct {
    ModelPath  string  // Path to Sherpa-ONNX speaker model
    NumThreads int     // Number of threads for inference
    Provider   string  // Compute provider ("cpu", "cuda", etc.)
    Threshold  float32 // Similarity threshold (0.0-1.0)
    DataDir    string  // Directory for speaker database
    Logger     Logger  // Optional logger
}
```

### Diarization Config
```go
type DiarizationConfig struct {
    Enabled               bool    // Whether diarization is enabled
    MinSegmentLength      float64 // Minimum segment length in seconds
    MaxSegmentLength      float64 // Maximum segment length in seconds
    SilenceThreshold      float64 // Silence threshold for segmentation
    SimilarityThreshold   float64 // Similarity threshold for clustering
    MaxSpeakers           int     // Maximum number of speakers to detect
    ReassignmentThreshold float64 // Threshold for speaker reassignment
    OverlapThreshold      float64 // Overlap threshold for speaker turns
    Logger                Logger  // Optional logger
}
```

### ASR Config
```go
type ASRConfig struct {
    Enabled              bool    // Whether ASR is enabled
    DefaultModel         string  // Default ASR model ("whisper_large_v3")
    Language             string  // Default language ("en")
    Quantization         string  // Quantization level ("int8", "int4", "float16", "float32")
    MaxConcurrentStreams int     // Maximum concurrent streaming sessions
    StreamTimeout        int     // Stream timeout in seconds
    ChunkSize            int     // Audio chunk size in samples (16000 = 1 second at 16kHz)
    VADProvider          string  // VAD provider ("ten_vad", "silero_vad", "webrtc_vad", "quail_vad")
    Logger               Logger  // Optional logger
}
```

## Error Handling

VoiceKit provides specific error types for different components:

- `AudioError`: Audio processing errors
- `SpeakerError`: Speaker recognition errors
- `DiarizationError`: Diarization errors
- `ASRError`: ASR streaming errors

All errors implement `Unwrap()` for compatibility with `errors.Is()` and `errors.As()`.

## Logger Interface

Implement this interface for custom logging:

```go
type Logger interface {
    Infof(format string, args ...interface{})
    Warnf(format string, args ...interface{})
    Errorf(format string, args ...interface{})
}
```

## Examples

See the `examples/` directory for complete working examples:
- `examples/basic_usage/` - Basic VoiceKit usage
- `examples/asr_streaming/` - Real-time ASR streaming
- `examples/speaker_only/` - Speaker recognition only
- `examples/audio_processing/` - Audio format conversion
- `examples/diarization/` - Speaker diarization

## Model Requirements

### ASR Models

VoiceKit supports multiple ASR models with different performance characteristics:

#### Whisper Models (Recommended for Multilingual)
- **Model**: OpenAI Whisper Large-v3
- **Languages**: 99+ languages
- **Performance**: ~500ms latency, high accuracy
- **Download**: Available via Hugging Face or direct integration

#### Nemotron Speech ASR (Recommended for English)
- **Model**: NVIDIA Nemotron Speech ASR 0.6B
- **Languages**: English
- **Performance**: 80-160ms latency, ultra-low latency
- **Requirements**: GPU recommended for optimal performance

#### Parakeet TDT (Recommended for High Throughput)
- **Model**: NVIDIA Parakeet TDT 0.6B
- **Languages**: English + multilingual variants
- **Performance**: High throughput, streaming optimized
- **Requirements**: GPU recommended

#### Moonshine Models (Recommended for Edge/CPU)
- **Model**: Moonshine (27M-120M variants)
- **Languages**: Multilingual or monolingual
- **Performance**: CPU-friendly, low resource usage
- **Requirements**: Works on CPU, edge devices

### Speaker Recognition Models
Download speaker embedding models from [Sherpa-ONNX Model Zoo](https://github.com/k2-fsa/sherpa-onnx):

```bash
# Example: 3D Speaker model
wget https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/3dspeaker_speech_eres2net_base_sv_zh-cn_3dspeaker_16k.onnx
```

### Audio Format Support
- **Input**: WAV, PCM, FLAC, MP3, OGG, M4A (limited compressed format support)
- **Output**: WAV, PCM (compressed formats are placeholders)

## Testing Strategy

### Unit Testing Architecture
- **Isolation**: Each component tested independently with mocked dependencies
- **Interface Mocking**: Dependency injection enables comprehensive mocking
- **Table-Driven Tests**: Go's testing framework used extensively for data-driven tests

### Integration Testing
- **Component Interaction**: Tests verify correct interaction between audio, speaker, and diarization components
- **End-to-End Pipelines**: Full voice processing pipelines tested with synthetic data
- **Performance Benchmarks**: `testing.B` benchmarks for performance regression detection

### Test Data Management
- **Synthetic Audio**: Generated test audio with known characteristics
- **Speaker Embeddings**: Pre-computed embeddings for consistent testing
- **Edge Cases**: Tests cover boundary conditions (empty audio, extreme sample rates, etc.)

### Coverage Goals
- **Core Logic**: >90% coverage for business logic functions
- **Error Paths**: All error conditions tested with appropriate assertions
- **Concurrency**: Race condition testing using `go test -race`

## Security Considerations

### Input Validation
- **Audio Format Sanitization**: Strict validation of audio file headers
- **Buffer Bounds Checking**: All array operations bounds-checked
- **Size Limits**: Configurable limits on audio file sizes and durations

### Memory Safety
- **No Unsafe Operations**: Pure Go implementation avoids unsafe pointers
- **Slice Bounds**: All slice operations use safe indexing
- **Resource Cleanup**: Proper cleanup of external library resources

### Data Privacy
- **Speaker Embeddings**: Sensitive biometric data stored securely
- **Temporary Files**: Audio processing uses memory buffers, not temporary files
- **Logging Sanitization**: Speaker IDs and sensitive data not logged

### Denial of Service Prevention
- **Resource Limits**: Configurable timeouts and memory limits
- **CPU Bounds**: Thread pool limits prevent resource exhaustion
- **Rate Limiting**: Integration-ready for rate limiting at higher layers

## Deployment and Operations

### Configuration Management
- **Environment Variables**: Support for 12-factor app configuration
- **Validation**: Configuration validated at startup with detailed error messages
- **Hot Reload**: Select components support configuration updates without restart

### Monitoring and Observability
- **Metrics Integration**: Compatible with Prometheus, statsd, etc.
- **Structured Logging**: Consistent log formats with contextual information
- **Health Checks**: Component-level health reporting for orchestration systems

### Performance Tuning
- **Buffer Sizes**: Tunable buffer sizes for memory/throughput trade-offs
- **Thread Pools**: Configurable thread counts for different workloads
- **Caching Strategies**: LRU caches for frequently accessed speaker data

### Production Checklist
- [ ] Configure appropriate log levels for production
- [ ] Set resource limits (memory, CPU, file descriptors)
- [ ] Enable metrics collection and monitoring
- [ ] Configure proper timeouts for all operations
- [ ] Set up proper speaker database backup/restore procedures
- [ ] Test with production-like audio data and speaker counts
- [ ] Verify CGO dependencies are properly installed
- [ ] Configure appropriate security policies for biometric data

## Code Organization and Development Practices

### Package Structure Rationale

```
voicekit/
├── voicekit.go      # Main API and orchestration
├── config.go        # Configuration types and validation
├── logger.go        # Logging interface and default implementation
├── errors.go        # Error types and handling utilities
├── asr/             # Real-time ASR streaming system
│   ├── service.go   # ASR orchestration and model management
│   ├── streaming.go # Session management and streaming state
│   ├── buffer.go    # Cache-aware audio buffering
│   ├── vad.go       # Voice activity detection
│   ├── whisper.go   # Whisper model implementation
│   ├── *.go         # Additional model implementations
│   └── *_test.go    # Comprehensive test coverage
├── audio/           # Audio processing primitives
│   ├── resampler.go # Sample rate conversion algorithms
│   ├── converter.go # Format conversion and I/O
│   ├── formats.go   # Audio format definitions and utilities
│   └── types.go     # Audio-specific type definitions
├── speaker/         # Speaker recognition system
│   ├── manager.go   # Core speaker operations (register/identify/verify)
│   ├── embedding.go # Neural network integration and similarity computation
│   ├── database.go  # Persistence layer and caching
│   ├── parser.go    # Audio parsing utilities
│   └── types.go     # Speaker domain types and interfaces
└── diarization/     # Speaker diarization system
    ├── manager.go   # Diarization orchestration and clustering
    ├── segmenter.go # Audio segmentation algorithms
    ├── integrator.go# ASR integration and result alignment
    └── types.go     # Diarization domain types
```

### Development Principles

#### Single Responsibility Principle
- **Audio Package**: Only audio format conversion and processing
- **Speaker Package**: Only speaker recognition operations
- **Diarization Package**: Only speaker diarization and segmentation
- **Main Package**: Only orchestration and API provision

#### Dependency Injection
- **Interface-Based**: All external dependencies injected as interfaces
- **Testability**: Easy mocking for comprehensive unit testing
- **Flexibility**: Runtime configuration of behavior

#### Error Handling Strategy
- **Structured Errors**: Custom error types with context
- **Error Wrapping**: Preserves error chains for debugging
- **Graceful Degradation**: Components fail independently

#### Memory Management
- **Object Pooling**: Reduces garbage collection pressure
- **Streaming Processing**: Minimizes memory footprint for large files
- **Resource Cleanup**: Proper cleanup of external resources

### Code Quality Standards

#### Naming Conventions
- **Exported Functions**: PascalCase, descriptive names (e.g., `NewVoiceKit`, `ProcessAudio`)
- **Internal Functions**: camelCase, implementation details (e.g., `extractEmbedding`)
- **Variables**: camelCase, descriptive and concise
- **Types**: PascalCase for exported, meaningful names

#### Documentation Standards
- **Package Comments**: Every package has a package comment explaining its purpose
- **Function Comments**: All exported functions documented with purpose and parameters
- **Code Comments**: Complex algorithms explained with comments
- **Example Code**: README contains working code examples

#### Testing Standards
- **Unit Tests**: Every exported function has corresponding unit tests
- **Integration Tests**: Component interaction tested
- **Benchmark Tests**: Performance-critical code benchmarked
- **Edge Cases**: Error conditions and boundary cases tested

### Performance Considerations

#### Algorithm Complexity
- **Audio Resampling**: O(n) linear time complexity
- **Speaker Search**: O(m*k) where m=speakers, k=embedding dimension
- **Diarization Clustering**: O(s²) where s=segments (acceptable for typical use cases)

#### Memory Efficiency
- **Buffer Reuse**: Object pools prevent unnecessary allocations
- **Streaming I/O**: Large files processed in chunks
- **Copy Minimization**: Zero-copy operations where safe

#### Concurrency Design
- **Goroutine Safety**: All public APIs goroutine-safe
- **Lock Granularity**: Fine-grained locking to minimize contention
- **Channel Communication**: Internal coordination uses channels

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

### Code Review Guidelines

- **Architecture**: Ensure changes align with layered architecture
- **Dependencies**: No new external dependencies without justification
- **Performance**: Consider algorithmic complexity and memory usage
- **Testing**: All changes include appropriate test coverage
- **Documentation**: Update README and code comments as needed
- **Security**: Consider security implications of changes

## Algorithms and Mathematical Foundations

### Audio Processing Algorithms

#### Resampling Mathematics
The resampling implementation uses interpolation theory:

**Linear Interpolation:**
```
y(x) = y₁ + (y₂ - y₁) * (x - x₁)/(x₂ - x₁)
```
- **Pros**: Fast, simple, low latency
- **Cons**: Aliasing artifacts, poor high-frequency response
- **Use Case**: Real-time applications prioritizing speed

**Cubic Interpolation:**
```
y(x) = a*x³ + b*x² + c*x + d
```
Where coefficients are solved from 4 neighboring points using:
```
[a,b,c,d] = [-0.5, 1.5, -1.5, 0.5] * [y₀, y₁, y₂, y₃]ᵀ
```
- **Pros**: Better frequency response than linear
- **Cons**: Higher computational cost
- **Use Case**: Balanced quality/performance applications

**Lanczos Interpolation:**
```
L(x) = sinc(x) * sinc(x/a) for |x| < a, 0 otherwise
```
- **Pros**: Excellent anti-aliasing properties
- **Cons**: Highest computational cost
- **Use Case**: High-quality offline processing

#### Audio Normalization
**RMS-Based Normalization:**
```
RMS = √(Σ(xᵢ²)/n)
scale_factor = target_rms / current_rms
normalized = input * scale_factor
```
- **Purpose**: Consistent audio levels for downstream processing
- **Range**: Typically normalizes to -20dB to -30dB RMS
- **Clamping**: Prevents over-amplification (>4x gain limit)

### Speaker Recognition Mathematics

#### Cosine Similarity
Speaker comparison uses cosine similarity on embedding vectors:
```
cosine_similarity(a,b) = (a·b) / (||a||₂ * ||b||₂)
```
- **Range**: [-1, 1] where 1.0 = identical, 0.0 = orthogonal, -1.0 = opposite
- **Properties**: Independent of vector magnitude, rotationally invariant
- **Advantages**: Computationally efficient, works well for high-dimensional embeddings

#### Embedding Space
- **Dimensionality**: Typically 192-512 dimensions from neural networks
- **Normalization**: Embeddings usually L2-normalized before similarity computation
- **Clustering**: Speaker embeddings form clusters in vector space

### Diarization Algorithms

#### Agglomerative Clustering
Speaker clustering uses bottom-up hierarchical clustering:

1. **Initialization**: Each segment starts as its own cluster
2. **Similarity Matrix**: Compute pairwise similarities between all clusters
3. **Merge Step**: Find most similar pair, merge them
4. **Iteration**: Repeat until similarity threshold reached
5. **Labeling**: Assign speaker IDs to final clusters

**Complexity**: O(n³) worst case, but practical for typical speaker counts

#### Audio Segmentation

**Silence-Based Segmentation:**
- **Energy Threshold**: Detect silence using RMS energy below threshold
- **Hysteresis**: Require consecutive silence frames to avoid noise triggers
- **Gap Filling**: Merge segments separated by short silence periods

**VAD-Based Segmentation:**
- **Voice Activity Detection**: Use external VAD to identify speech regions
- **Boundary Refinement**: Adjust boundaries based on energy analysis
- **Overlap Handling**: Merge overlapping speech segments

### Numerical Stability Considerations

#### Floating Point Precision
- **Accumulation**: Use Kahan summation for precise RMS calculations
- **Normalization**: Careful handling of near-zero values in similarity computations
- **Clamping**: Prevent numerical instabilities in interpolation

#### Memory Layout
- **Cache Alignment**: Struct fields ordered for optimal memory access
- **SIMD Opportunities**: Algorithms structured for potential vectorization
- **Memory Coherence**: Sequential access patterns minimize cache misses

### Performance Optimizations

#### Algorithmic Optimizations
- **Early Termination**: Similarity searches stop when threshold exceeded
- **Approximate Methods**: Use approximate nearest neighbor for large speaker databases
- **Incremental Updates**: Update clustering incrementally rather than recomputing

#### Memory Optimizations
- **Buffer Pools**: Reuse allocated buffers to reduce GC pressure
- **Streaming Processing**: Process audio in chunks to bound memory usage
- **Lazy Evaluation**: Compute results only when requested

#### Parallelization Opportunities
- **Independent Segments**: Diarization segments can be processed in parallel
- **Batch Processing**: Multiple audio files processed concurrently
- **GPU Acceleration**: Neural network inference can use GPU acceleration

## License

This library is released under the MIT License. See LICENSE file for details.

## Migration from Internal Packages

When migrating from the original ASR server's internal packages:

1. Replace `internal/speaker` imports with `github.com/SamyRai/voicekit/speaker`
2. Replace `internal/audio` imports with `github.com/SamyRai/voicekit/audio`
3. Replace `internal/diarization` imports with `github.com/SamyRai/voicekit/diarization`
4. Update configuration structures to use `voicekit.Config`
5. Remove dependencies on `internal/logger` and `internal/config`

Example migration:

```go
// Before
import "asr_server/internal/speaker"

// After
import "github.com/SamyRai/voicekit/speaker"
```