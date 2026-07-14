# TTS Streaming Example

This example demonstrates `tts.SherpaStreamingSynthesizer` with Sherpa offline TTS models. Unlike `examples/tts_offline`, which waits for the whole utterance before returning, this example receives audio chunks as they are produced via a sink callback, printing each chunk's size and progress as it arrives. It does not ship model files.

## Requirements

- CGO enabled.
- Sherpa offline TTS model files for one supported family (`vits`, `matcha`, `kokoro`, or `kitten`).
- An `espeak-ng-data` directory for VITS, Matcha, Kokoro, and Kitten models.

For Kokoro:

```bash
export VOICEKIT_TTS_FAMILY=kokoro
export VOICEKIT_TTS_MODEL=/models/sherpa/kokoro/model.onnx
export VOICEKIT_TTS_VOICES=/models/sherpa/kokoro/voices.bin
export VOICEKIT_TTS_TOKENS=/models/sherpa/kokoro/tokens.txt
export VOICEKIT_TTS_DATA_DIR=/models/sherpa/kokoro/espeak-ng-data
export VOICEKIT_TTS_TEXT="VoiceKit can stream synthesized speech chunk by chunk."
```

For KittenTTS:

```bash
export VOICEKIT_TTS_FAMILY=kitten
export VOICEKIT_TTS_MODEL=/models/sherpa/kitten/model.onnx
export VOICEKIT_TTS_VOICES=/models/sherpa/kitten/voices.bin
export VOICEKIT_TTS_TOKENS=/models/sherpa/kitten/tokens.txt
export VOICEKIT_TTS_DATA_DIR=/models/sherpa/kitten/espeak-ng-data
```

Optional:

```bash
export VOICEKIT_TTS_PROVIDER=cpu
export VOICEKIT_TTS_THREADS=1
export VOICEKIT_TTS_SPEAKER_ID=0
export VOICEKIT_TTS_SPEED=1
# Stop early after N chunks to simulate barge-in (0 = deliver every chunk):
export VOICEKIT_TTS_MAX_CHUNKS=0
```

## Running

```bash
go run ./examples/tts_streaming
```

If required variables are missing, the example exits cleanly and prints the required settings.

## What it shows

- `tts.NewSherpaStreamingSynthesizer` construction from a `tts.Config`.
- `SynthesizeStream` delivering `types.AudioChunk` values to a sink callback in order, each with its own `Progress` (0..1).
- Returning `false` from the sink (via `VOICEKIT_TTS_MAX_CHUNKS`) stops synthesis early, the same mechanism a caller would use for playback interruption / barge-in.
- The method still returns the fully assembled `*types.SynthesizedSpeech` for the portion generated, in addition to the streamed chunks.
