# Keyword Spotting Example

This example demonstrates `asr.SherpaKeywordSpotter`, streaming wake-word/hotword detection backed by Sherpa ONNX's `KeywordSpotter`. It does not ship model files.

Keyword spotting is not yet wired into the root `voicekit.Config`/`VoiceKit`, so this example constructs `asr.SherpaKeywordSpotter` directly.

## Requirements

- CGO enabled.
- A Sherpa keyword-spotting model: `tokens.txt` plus either a transducer triple (encoder/decoder/joiner) or a single-file CTC model.
- A keyword list: a keywords file (Sherpa's tokenized keyword-phrase format) or an inline comma-separated list of keyword phrases.

```bash
export VOICEKIT_KWS_TOKENS=/models/sherpa/kws/tokens.txt
export VOICEKIT_KWS_ENCODER=/models/sherpa/kws/encoder.onnx
export VOICEKIT_KWS_DECODER=/models/sherpa/kws/decoder.onnx
export VOICEKIT_KWS_JOINER=/models/sherpa/kws/joiner.onnx
export VOICEKIT_KWS_KEYWORDS="hey computer,stop listening"
```

Or, for a single-file model plus a keywords file:

```bash
export VOICEKIT_KWS_TOKENS=/models/sherpa/kws/tokens.txt
export VOICEKIT_KWS_MODEL=/models/sherpa/kws/model.onnx
export VOICEKIT_KWS_MODEL_TYPE=zipformer2_ctc
export VOICEKIT_KWS_KEYWORDS_FILE=/models/sherpa/kws/keywords.txt
```

Optional:

```bash
export VOICEKIT_KWS_PROVIDER=cpu
export VOICEKIT_KWS_THREADS=1
```

## Running

```bash
go run ./examples/keyword_spotting
```

If the required model/keyword environment variables are not set, the example exits cleanly and prints what's missing.

## Notes

- Each `sessionID` passed to `Spot` keeps its own native decode stream. Call `EndSession(sessionID)` to release a single session's stream when you are done with it (important if you use a fresh `sessionID` per utterance), or `Close()` to release all streams and the spotter.
- `KeywordMatch.Score` is always `0`: the Sherpa v1.13.8 binding's `KeywordSpotterResult` exposes only the matched keyword text, not a calibrated confidence value.
