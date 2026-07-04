# ASR Offline Example

This example demonstrates `VoiceKit.Transcriber()` with Sherpa offline ASR. It does not ship model files.

## Requirements

- CGO enabled.
- A mono WAV file that matches the configured sample rate.
- Sherpa offline model files for one supported family.

For a SenseVoice-style single-file model:

```bash
export VOICEKIT_OFFLINE_ASR_FAMILY=sense_voice
export VOICEKIT_OFFLINE_ASR_MODEL=/models/sherpa/sense-voice.onnx
export VOICEKIT_OFFLINE_ASR_WAV=/audio/example.wav
```

For Whisper:

```bash
export VOICEKIT_OFFLINE_ASR_FAMILY=whisper
export VOICEKIT_OFFLINE_ASR_ENCODER=/models/sherpa/whisper-encoder.onnx
export VOICEKIT_OFFLINE_ASR_DECODER=/models/sherpa/whisper-decoder.onnx
export VOICEKIT_OFFLINE_ASR_WAV=/audio/example.wav
```

For transducer models, also set `VOICEKIT_OFFLINE_ASR_TOKENS`, `VOICEKIT_OFFLINE_ASR_ENCODER`, `VOICEKIT_OFFLINE_ASR_DECODER`, and `VOICEKIT_OFFLINE_ASR_JOINER`.

## Running

```bash
go run ./examples/asr_offline
```

Optional:

```bash
export VOICEKIT_OFFLINE_ASR_SAMPLE_RATE=16000
```

If required variables are missing, the example exits cleanly and prints the required settings.
