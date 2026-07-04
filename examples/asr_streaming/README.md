# ASR Streaming Example

This example demonstrates VoiceKit's Sherpa online ASR wiring. It does not ship model files and does not use the old fake transcription path.

## Requirements

- CGO enabled.
- Go 1.26.4 or newer 1.26.x.
- Sherpa online transducer model files.
- `tokens.txt` for the same model.

Set these environment variables before running:

```bash
export VOICEKIT_ASR_TOKENS=/models/sherpa/tokens.txt
export VOICEKIT_ASR_ENCODER=/models/sherpa/encoder.onnx
export VOICEKIT_ASR_DECODER=/models/sherpa/decoder.onnx
export VOICEKIT_ASR_JOINER=/models/sherpa/joiner.onnx
```

## Running

```bash
go run ./examples/asr_streaming
```

If the variables are missing, the example exits cleanly and prints the required settings. This is intentional: ASR enabled without model paths is a configuration error in VoiceKit.

## Configuration

The example uses:

```go
ASR: voicekit.ASRConfig{
    Enabled:              true,
    Backend:              "sherpa_online",
    DefaultModel:         "sherpa_online",
    Language:             "en",
    Quantization:         "float32",
    MaxConcurrentStreams: 10,
    StreamTimeout:        300,
    ChunkSize:            16000,
    Online: voicekit.OnlineConfig{
        TokensPath:  os.Getenv("VOICEKIT_ASR_TOKENS"),
        EncoderPath: os.Getenv("VOICEKIT_ASR_ENCODER"),
        DecoderPath: os.Getenv("VOICEKIT_ASR_DECODER"),
        JoinerPath:  os.Getenv("VOICEKIT_ASR_JOINER"),
    },
    VADProvider: "none",
}
```

To use VAD, configure `VADProvider` as `energy`, `silero_vad`, or `ten_vad`. Sherpa VAD providers also require `VADModelPath`.

## Notes

- Output quality and latency depend on the model and runtime provider.
- VoiceKit keeps one Sherpa online stream per `sessionID` across chunks. Endpoint/finalization clears that stream so the next utterance starts fresh.
- Confidence is `0` when the Sherpa binding does not expose calibrated confidence.
- TTS is not part of this example; see `examples/tts_offline/` for offline speech synthesis.
