# ASR Streaming Example

This example demonstrates real-time ASR (Automatic Speech Recognition) streaming capabilities in VoiceKit.

## Features Demonstrated

- **Real-time Streaming**: Process audio chunks as they arrive
- **Voice Activity Detection**: Automatic speech/non-speech detection
- **Partial Results**: Streaming transcription with interim results
- **Session Management**: Handle multiple concurrent streaming sessions
- **Performance Monitoring**: Built-in latency and memory tracking

## Running the Example

```bash
# From the voicekit/examples/asr_streaming directory
go run main.go
```

## Expected Output

```
🎤 VoiceKit ASR Streaming Example
==================================
✅ VoiceKit initialized with ASR streaming support

🎵 Starting ASR streaming session...
Processing 5 audio chunks (1s each):

Chunk 1/5: No speech detected
Chunk 2/5: [Partial] "Transcribed 16000 samples in en" (confidence: 0.85, lang: en, 501ms)
Chunk 3/5: [Partial] "Transcribed 16000 samples in en" (confidence: 0.85, lang: en, 498ms)
Chunk 4/5: [Partial] "Transcribed 16000 samples in en" (confidence: 0.85, lang: en, 502ms)
Chunk 5/5: [Final] "Transcribed 16000 samples in en" (confidence: 0.85, lang: en, 499ms)

📊 ASR Streaming Performance Summary:
- Latency: ~500ms per 1-second chunk (benchmark result)
- Memory: ~100KB per operation
- Real-time factor: <0.5 (system keeps up with audio stream)
- VAD integration: Automatic speech/non-speech detection

🎉 ASR streaming demonstration completed!
```

## Configuration

The example uses the following ASR configuration:

```go
ASR: voicekit.ASRConfig{
    Enabled:              true,
    DefaultModel:         "whisper_large_v3",
    Language:             "en",
    Quantization:         "int8",
    MaxConcurrentStreams: 10,
    StreamTimeout:        300,    // 5 minutes
    ChunkSize:            16000,  // 1 second at 16kHz
    VADProvider:          "ten_vad",
}
```

## Synthetic Audio Generation

The example generates synthetic audio chunks to simulate:

1. **Silent chunks**: No speech detected
2. **Speech-like chunks**: Detected and transcribed
3. **Mixed content**: Speech with pauses
4. **Real-time streaming**: 100ms delays between chunks

## Performance Characteristics

- **Latency**: ~500ms per 1-second audio chunk
- **Memory**: ~100KB per operation
- **Allocations**: 14 allocations per chunk
- **Concurrent Sessions**: Support for 10+ simultaneous streams

## Integration with Real Audio

To use with real audio streams:

1. Replace `generateSyntheticAudioChunk()` with actual audio input
2. Handle audio format conversion (PCM 16-bit, 16kHz, mono)
3. Implement proper audio chunking from your audio source
4. Add error handling for network/audio issues

## Next Steps

- Integrate with WebRTC for browser-based streaming
- Add support for multiple languages and models
- Implement advanced VAD with noise reduction
- Add confidence-based result filtering
- Integrate with speaker diarization for multi-speaker scenarios