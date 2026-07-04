# Voice Stack Research - July 2026

## Scope

This note records the July 4, 2026 research pass for VoiceKit's local audio inference direction. It focuses on ASR, VAD, speaker recognition, diarization, and TTS compatibility for a Go library that should remain useful as a local `go-agent` audio backend.

No model files are committed or required by this note.

## Executive Summary

Sherpa-ONNX should remain VoiceKit's primary native inference stack. The latest checked Go module is `github.com/k2-fsa/sherpa-onnx-go v1.13.3`, and the upstream `sherpa-onnx` release `v1.13.3` was published on June 15, 2026. The current Go API and platform packages expose enough typed surface for streaming ASR, offline ASR, VAD, speaker embedding extraction, offline speaker diarization, and offline TTS. VoiceKit now targets Go 1.26.4 for this foundation branch.

The main correction to the foundation sprint roadmap is that VoiceKit should not build much more custom diarization logic. Sherpa already exposes `OfflineSpeakerDiarization` and `SpeakerEmbeddingExtractor` in Go. VoiceKit now adapts `OfflineSpeakerDiarization` behind its own backend interface, keeps its typed result shapes, and confines the old silence-plus-clustering path to an explicit `basic` backend.

The second correction is ASR model coverage. VoiceKit now has a Sherpa offline ASR transcriber for whole-audio inputs while keeping Sherpa online ASR for streaming. The online path keeps a native Sherpa stream per VoiceKit session instead of creating one stream per chunk, and `InputFinished` is reserved for finalization. Mid-2026 Sherpa has a much broader offline ASR surface: Whisper, SenseVoice, Moonshine, Omnilingual, Cohere Transcribe, FunASR Nano, Qwen3-ASR, FireRedAsr, Dolphin, Canary, NeMo, Paraformer, Wenet CTC, Zipformer CTC, and related model configs. The first supported VoiceKit config families are transducer, Paraformer, Zipformer CTC, NeMo CTC, SenseVoice, and Whisper.

The third correction is that TTS no longer needs to remain purely future work. Sherpa Go exposes offline TTS model configs and `OfflineTts.Generate`, so VoiceKit now provides a separate opt-in `tts.SherpaOfflineSynthesizer` and root `VoiceKit.Synthesizer()` facet. This keeps synthesis out of ASR-only initialization while giving local agents a typed text-to-speech path.

## Current Upstream State

### Sherpa-ONNX

Checked sources:

- Release notes: https://github.com/k2-fsa/sherpa-onnx/releases
- Go API docs: https://k2-fsa.github.io/sherpa/onnx/go-api/index.html
- VAD docs: https://k2-fsa.github.io/sherpa/onnx/vad/index.html
- Speaker diarization docs: https://k2-fsa.github.io/sherpa/onnx/speaker-diarization/index.html
- Speaker identification docs: https://k2-fsa.github.io/sherpa/onnx/speaker-identification/index.html
- TTS docs: https://k2-fsa.github.io/sherpa/onnx/tts/index.html
- Local Go module source: `$GOMODCACHE/github.com/k2-fsa/sherpa-onnx-go-macos@v1.13.3/sherpa_onnx.go`

Findings:

- `sherpa-onnx-go v1.13.3` is the latest Go module version returned by `go list -m -versions`.
- `sherpa-onnx v1.13.3` release notes include mid-2026 work such as multilingual Nemotron-3.5 streaming ASR, Android QNN demo work, and an iOS ONNX Runtime 1.26.0 update.
- `v1.13.2` release notes are also relevant: NeMo Parakeet Unified streaming, KittenTTS v0.8, Supertonic3 TTS, WASM fixes, and model export work.
- Official Go docs state the Go API supports both streaming and non-streaming ASR, ships prebuilt platform libraries, and requires CGO.
- Official VAD docs list Silero VAD and TEN VAD. Silero is MIT licensed; TEN VAD uses a modified Apache 2.0 license, so production use needs license review.
- The Go source exposes richer APIs than VoiceKit currently uses:
  - `OnlineRecognizerConfig` for streaming ASR.
  - `OfflineRecognizerConfig` for batch/offline ASR.
  - `VadModelConfig` and `VoiceActivityDetector` for Silero/TEN VAD.
  - `SpeakerEmbeddingExtractorConfig` and `SpeakerEmbeddingExtractor`.
  - `OfflineSpeakerDiarizationConfig` and `OfflineSpeakerDiarization`.
  - `OfflineTtsConfig`, `OfflineTts`, `GeneratedAudio`, and family configs for VITS, Matcha, Kokoro, KittenTTS, ZipVoice, Pocket, and Supertonic.

Source-level ASR implications:

- Online model configs include transducer, paraformer, zipformer2 CTC, NeMo CTC, and Tone CTC.
- Offline model configs include the more modern/high-value model families: Whisper, SenseVoice, Moonshine, FireRedAsr, FunASR Nano, Dolphin, Zipformer CTC, Canary, Wenet CTC, Omnilingual ASR, MedASR, FireRedAsr CTC, Qwen3-ASR, and Cohere Transcribe.
- VoiceKit should add `asr.SherpaOfflineModel` rather than trying to stretch the online model to every use case.

Source-level diarization implications:

- Sherpa Go exposes first-class offline diarization using a segmentation model, speaker embedding extractor config, clustering config, and min-duration settings.
- VoiceKit should add a Sherpa-backed diarization adapter that maps Sherpa segments into `diarization.DiarizationResult`.
- The current custom diarization manager should become a basic/testing fallback or be narrowed to result integration only.

Source-level TTS implications:

- Sherpa Go exposes first-class offline TTS with `Generate(text, sid, speed)` and generated normalized float32 samples.
- VoiceKit should keep TTS as a separate package/config/facet rather than folding synthesis into ASR or audio conversion.
- VoiceKit should return typed generated audio samples and let callers decide whether to encode WAV/PCM containers.

### Silero VAD

Checked sources:

- https://github.com/snakers4/silero-vad

Findings:

- Latest checked tag from Git was `v6.2.1`.
- The README advertises ONNX Runtime support and notes 8 kHz and 16 kHz sampling support.
- Direct use is possible through ONNX Runtime, but Sherpa already wraps Silero for the use cases VoiceKit needs.

Recommendation:

- Keep Silero behind Sherpa for VoiceKit production use.
- Add direct ONNX Runtime Silero only if Sherpa's VAD API blocks a required feature.

### TEN VAD

Checked sources:

- https://github.com/TEN-framework/ten-vad

Findings:

- Latest checked tags include `v1.0` and `v1.0-ONNX`.
- The repository includes ONNX examples and model files.
- Sherpa supports TEN VAD in the same VAD API family as Silero.

Recommendation:

- Keep TEN VAD as a Sherpa provider option.
- Do a license review before making it the default in production bundles.

### ONNX Runtime Go

Checked sources:

- https://github.com/yalue/onnxruntime_go
- https://github.com/microsoft/onnxruntime
- https://onnxruntime.ai/docs/reference/releases-servicing.html

Findings:

- `github.com/yalue/onnxruntime_go v1.31.0` is the latest Go wrapper module returned by `go list -m -versions`.
- ONNX Runtime itself is the cross-platform engine underneath many of these stacks and commits to release-branch/backward-compatibility practices.
- Raw ONNX Runtime Go gives maximum control but also forces VoiceKit to own feature extraction, tokenization, decoding, timestamps, and post-processing.

Recommendation:

- Treat raw ONNX Runtime as an escape hatch, not the default VoiceKit stack.
- Use it for models Sherpa cannot load but only behind explicit interfaces and tests.

## Alternatives

| Stack | Latest checked version/state | Fit for VoiceKit | Tradeoffs |
| --- | --- | --- | --- |
| Sherpa-ONNX Go | `v1.13.3` Go module and upstream release | Best primary stack | CGO and model-path management, but typed Go bindings cover ASR, VAD, speaker embeddings, diarization, and TTS. |
| whisper.cpp Go bindings | upstream `v1.9.1` release; Go package exists under whisper.cpp bindings | Good ASR fallback for offline/batch transcription | Strong local Whisper ecosystem, but not a complete voice stack; streaming/diarization/VAD need extra components. |
| Vosk Go | Go module tags through `v0.3.50`; GitHub release page is older than module tags | Lightweight ASR fallback for constrained streaming | Mature offline ASR with small models and Go bindings, but lower modern-model ceiling than Sherpa/Whisper. |
| pyannote.audio | `4.0.7` latest checked tag; `community-1` open-source model launched with pyannote 4.0 | Best diarization benchmark/sidecar, not in-process Go | Python/PyTorch/Hugging Face oriented; useful as evaluator or optional sidecar, not the core Go library dependency. |
| Raw ONNX Runtime Go | `yalue/onnxruntime_go v1.31.0` | Escape hatch for unsupported ONNX models | High ownership burden for audio pre/post-processing. |
| asr_server sibling repo | Local branch `refactor-fix`; broad server/session app | Reference only | Carries server/web/session concerns and tracked model/binary assets. Useful ideas: VAD pooling, worker pool shape, demo UX. |

## VoiceKit Roadmap Implications

### Recommended Next Implementation Order

1. Evaluate Sherpa offline ASR models on target audio.
   - `asr.SherpaOfflineModel` is implemented as a batch `types.Transcriber`.
   - Config is explicit: `ASR.Online` and `ASR.Offline` replace the old flat model-path fields.
   - Env-gated integration tests run when model path variables are present.

2. Evaluate Sherpa offline diarization models on target audio.
   - `diarization.Backend` and the Sherpa offline backend are implemented.
   - Config covers segmentation model, embedding model, clustering threshold, cluster count, min on/off duration, provider, and threads.
   - VoiceKit normalizes sample rate at the audio boundary before diarization.

3. Revisit speaker recognition ownership.
   - VoiceKit speaker manager currently uses Sherpa embedding extraction already.
   - Speaker embedding streams are now single-use and deleted after extraction because Sherpa streams cannot be safely reused after `InputFinished`.
   - The same `SpeakerEmbeddingExtractor` should back speaker recognition and diarization when configured, but ownership must remain explicit so resources are closed once.

4. Add recognizer/model lifecycle pooling only after measurement.
   - `asr_server` shows a VAD pool and recognition worker pool shape.
   - Do not add pools blindly. Sherpa recognizers are expensive to initialize, but concurrency guarantees need targeted tests and benchmarks. Speaker stream pooling is explicitly disabled until reset/reuse support is proven.

5. Keep TTS separate.
   - `tts.SherpaOfflineSynthesizer` is implemented as a `types.SpeechSynthesizer`.
   - Config is explicit and family-specific: VITS, Matcha, Kokoro, KittenTTS, ZipVoice, Pocket, and Supertonic.
   - ASR-only users still do not pay TTS model/config complexity.

### Recommended Configuration Shape

Keep root config as adapters over package-owned configs:

- `asr.Config` now splits online and offline model config instead of one wide flat struct.
- `diarization.Config` now has a `Backend` field with values like `sherpa_offline` and `basic`.
- `tts.Config` now has a `Backend`, `ModelFamily`, and family-specific model path blocks.
- `speaker.Config` should expose whether the embedding extractor is owned by speaker or injected from a shared runtime.
- `vad.Config` should remain provider-oriented: `none`, `energy`, `silero_vad`, `ten_vad`.

### What Not To Do

- Do not add Whisper/Vosk/Pyannote as production dependencies inside VoiceKit yet.
- Do not commit model files.
- Do not copy server/session code from `asr_server` into VoiceKit.
- Do not claim diarization is production-grade until Sherpa diarization or a comparable real backend is wired and evaluated.
- Do not make TEN VAD the default before license review.

## Reference Commands

Commands run during this research pass:

```bash
go list -m -versions github.com/k2-fsa/sherpa-onnx-go github.com/viktordanov/go-hnswlib github.com/cockroachdb/pebble
go list -m -versions github.com/ggerganov/whisper.cpp/bindings/go github.com/yalue/onnxruntime_go github.com/alphacep/vosk-api/go
git ls-remote --tags https://github.com/k2-fsa/sherpa-onnx-go.git
git ls-remote --tags https://github.com/ggml-org/whisper.cpp.git
git ls-remote --tags https://github.com/alphacep/vosk-api.git
git ls-remote --tags https://github.com/pyannote/pyannote-audio.git
git ls-remote --tags https://github.com/microsoft/onnxruntime.git
git ls-remote --tags https://github.com/snakers4/silero-vad.git
git ls-remote --tags https://github.com/TEN-framework/ten-vad.git
```
