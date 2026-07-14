# Spoken Language Identification Example

This example demonstrates `asr.SherpaLanguageIdentifier` with Sherpa offline
spoken language identification (a Whisper-based ONNX model). It does not ship
model files.

Spoken language identification is not yet wired into the root
`voicekit.Config`/`VoiceKit`, so this example constructs
`asr.SherpaLanguageIdentifier` directly, and decodes the input WAV file with
`audio.NewConverter` directly (mirroring `examples/asr_offline`, which also
decodes audio outside the root facade).

## Requirements

- CGO enabled.
- A Sherpa spoken language identification Whisper encoder/decoder ONNX model
  pair.
- A mono WAV file to identify.

```bash
export VOICEKIT_LID_WAV=/path/to/audio.wav
export VOICEKIT_LID_ENCODER=/models/sherpa/lid/encoder.onnx
export VOICEKIT_LID_DECODER=/models/sherpa/lid/decoder.onnx
```

Optional:

```bash
export VOICEKIT_LID_PROVIDER=cpu
export VOICEKIT_LID_THREADS=1
export VOICEKIT_LID_SAMPLE_RATE=16000
```

## Running

```bash
go run ./examples/language_id
```

If `VOICEKIT_LID_WAV`, `VOICEKIT_LID_ENCODER`, or `VOICEKIT_LID_DECODER` is
not set, the example exits cleanly and prints the required setting.
