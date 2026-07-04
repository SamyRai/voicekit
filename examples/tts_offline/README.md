# TTS Offline Example

This example demonstrates `VoiceKit.Synthesizer()` with Sherpa offline TTS. It does not ship model files.

## Requirements

- CGO enabled.
- Sherpa offline TTS model files for one supported family.
- An `espeak-ng-data` directory for VITS, Matcha, Kokoro, and Kitten models.

For Kokoro:

```bash
export VOICEKIT_TTS_FAMILY=kokoro
export VOICEKIT_TTS_MODEL=/models/sherpa/kokoro/model.onnx
export VOICEKIT_TTS_VOICES=/models/sherpa/kokoro/voices.bin
export VOICEKIT_TTS_TOKENS=/models/sherpa/kokoro/tokens.txt
export VOICEKIT_TTS_DATA_DIR=/models/sherpa/kokoro/espeak-ng-data
export VOICEKIT_TTS_TEXT="VoiceKit can synthesize speech locally."
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
```

## Running

```bash
go run ./examples/tts_offline
```

If required variables are missing, the example exits cleanly and prints the required settings.
