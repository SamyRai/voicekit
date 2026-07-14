# Punctuation Restoration Example

This example demonstrates `asr.SherpaPunctuation` with Sherpa offline punctuation restoration (a ct-transformer ONNX model). It does not ship model files.

Punctuation restoration is not yet wired into the root `voicekit.Config`/`VoiceKit`, so this example constructs `asr.SherpaPunctuation` directly.

## Requirements

- CGO enabled.
- A Sherpa offline punctuation ct-transformer ONNX model file.

```bash
export VOICEKIT_PUNCT_MODEL=/models/sherpa/punctuation/model.onnx
export VOICEKIT_PUNCT_TEXT="voicekit restores punctuation and casing for raw asr transcripts"
```

Optional:

```bash
export VOICEKIT_PUNCT_PROVIDER=cpu
export VOICEKIT_PUNCT_THREADS=1
```

## Running

```bash
go run ./examples/punctuation
```

If `VOICEKIT_PUNCT_MODEL` is not set, the example exits cleanly and prints the required setting.
