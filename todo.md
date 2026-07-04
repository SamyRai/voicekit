# VoiceKit Foundation Sprint Todo

## Baseline and Reference

- [x] Create branch `dev/voicekit-foundation-sprint`.
- [x] Clone and inspect sibling `asr_server` as Sherpa/VAD reference material.
- [x] Fix baseline correctness: config defaulting, `NewVoiceKit(nil)`, vet failure.

## ASR and VAD

- [x] Move ASR-owned config, errors, logging, model lifecycle, and VAD seams into `asr`.
- [x] Wire root `voicekit` to the ASR package without an ASR -> root import.
- [x] Implement Sherpa online ASR model with explicit model path validation.
- [x] Replace default threshold VAD with explicit providers: Sherpa Silero/TEN or named energy/basic.
- [x] Add ASR/VAD tests and dependency boundary check.

## Diarization, Speaker, Audio

- [x] Add diarization embedding extractor seam and remove generated fake embeddings.
- [x] Add context-aware speaker APIs and exported embedding extraction for adapters.
- [x] Replace untyped diarization integration with typed input plus deprecated shim.
- [x] Fix PCM/WAV edge cases and add round-trip tests.
- [x] Remove stdout prints from library code.

## Dependencies, Docs, Verification

- [x] Upgrade Sherpa, HNSW, and Pebble dependencies after API smoke checks.
- [x] Update README, ASR streaming README, and performance/roadmap docs.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `go test -race ./...`.
- [x] Run `go test -cover ./...`.
- [x] Run audio/indexing/diarization benchmarks.

## July 2026 Research Follow-up

- [x] Check latest Sherpa-ONNX release notes, Go docs, and local Go module source.
- [x] Compare current alternatives: whisper.cpp, Vosk, pyannote.audio, Silero VAD, TEN VAD, and raw ONNX Runtime Go.
- [x] Revisit local `asr_server` as a reference for reusable lifecycle/pooling ideas.
- [x] Document recommendations in `VOICE_STACK_RESEARCH_2026.md`.

## Sherpa Native Stack Refactor

- [x] Replace flat ASR model paths with explicit online/offline sub-configs.
- [x] Add Sherpa offline ASR as a batch `types.Transcriber`.
- [x] Wire `VoiceKit.Transcriber()` without changing streaming `ASRService`.
- [x] Replace production diarization with a Sherpa offline backend.
- [x] Move the old silence/embedding clustering path behind an explicit basic backend.
- [x] Normalize sample rate at the VoiceKit audio boundary before speaker/diarization calls.
- [x] Add unit and env-gated integration tests for offline ASR and diarization.
- [x] Update README, examples, performance notes, and research notes.
- [x] Run full Go validation and focused ASR/diarization benchmarks.

## Sherpa Offline TTS

- [x] Check official Sherpa Go/TTS docs and local Go binding source.
- [x] Add `types.SpeechSynthesizer` and a separate `tts` package.
- [x] Implement Sherpa offline TTS config, model-family validation, and native lifecycle wrapper.
- [x] Wire disabled-by-default root `TTSConfig` and `VoiceKit.Synthesizer()`.
- [x] Add unit tests with a fake native synthesizer.
- [x] Add env-gated offline TTS example.
- [x] Update README, streaming example notes, performance notes, and research notes.

## Lead Dev Hardening Sprint

- [x] Move the module compatibility target to Go 1.26.4.
- [x] Replace chunk-local Sherpa online streams with session-owned stream state.
- [x] Flush online ASR streams only on finalization and close stream state on session cleanup/service close.
- [x] Remove unsafe speaker embedding stream reuse after `InputFinished`.
- [x] Harden HNSW missing-removal, empty-search, logical size, and closed-index behavior.
- [x] Add unit tests for ASR native stream lifecycle, speaker stream lifecycle, and HNSW edge cases.
- [x] Run full Go 1.26.4 validation gate and checkpoint the sprint.

## Product Intelligence Roadmap Sprint

- [x] Document the market review, top 10 product gaps, sequencing, and first implementation slice in `PRODUCT_ROADMAP_2026.md`.
- [x] Add a first-class `meeting` package for meeting artifacts, participants, transcript turns, summaries, decisions, action items, and follow-ups.
- [x] Map `diarization.IntegratedResult` into `meeting.Meeting` transcript turns without adding LLM/provider dependencies.
- [x] Add focused tests for meeting artifact validation, speaker/participant mapping, timing, and word preservation.
- [x] Update README package/API notes for the new meeting intelligence layer.
- [x] Run focused and repo-wide Go validation.

## Meeting Export Sprint

- [x] Add Markdown export for meeting metadata, summary sections, action items, and transcript turns.
- [x] Add product-facing JSON export with transcript and summary timing in seconds.
- [x] Add SRT and WebVTT caption exports with speaker labels and positive-timing validation.
- [x] Add focused exporter tests for JSON timing, Markdown output, captions, and invalid caption timing.
- [x] Run focused and repo-wide Go validation for the export slice.

## Meeting Analyzer Contract Sprint

- [x] Add provider-neutral `meeting.Analyzer` with context-aware analysis requests and structured results.
- [x] Add default analysis scopes for overview, topics, decisions, action items, follow-ups, open questions, and risks.
- [x] Add analyzer output normalization for generated text, stable artifact IDs, warnings, evidence IDs, and action item statuses.
- [x] Add validation for empty output, duplicate IDs, invalid timing, unsupported statuses, and transcript evidence references.
- [x] Add focused analyzer contract tests.
- [x] Run focused and repo-wide Go validation for the analyzer contract slice.

## Meeting Quality, Privacy, and Reference Analyzer Sprint

- [x] Checkpoint the existing meeting/export/analyzer foundation in a separate commit after clean focused and repo-wide validation.
- [x] Add top-level `evaluation` metrics for transcript WER/CER, turn-level speaker attribution, action-item precision/recall/F1, and real-time factor.
- [x] Add deterministic evaluation fixtures and edge-case tests for empty references and normalization.
- [x] Add meeting redaction contracts, built-in stdlib pattern redactor, redaction reports, and copy-based `Meeting.Redact`.
- [x] Add export options with `RequireRedaction` enforcement for JSON, Markdown, SRT, and WebVTT while preserving existing export methods.
- [x] Add `meeting.HeuristicAnalyzer` as a stdlib-only deterministic reference analyzer with evidence IDs and scope filtering.
- [x] Add `examples/meeting_intelligence` for analyze, redact, export, and evaluate flow without model files or provider SDKs.
- [x] Update README and product roadmap for the shipped APIs and remaining limits.
- [x] Run `go test ./meeting ./evaluation`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `go test -race ./...`.
- [x] Run `git diff --check`.

## Meeting Intelligence Documentation Sprint

- [x] Add `MEETING_INTELLIGENCE.md` as the next-developer guide for data flow, package ownership, analyzer extension, redaction/export safety, evaluation metrics, errors, and validation.
- [x] Link the guide from `README.md`.
- [x] Update `PRODUCT_ROADMAP_2026.md` implementation status with the guide.
- [x] Run `go test ./meeting ./evaluation`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `go test -race ./...`.
- [x] Run `git diff --check`.

## Performance Sprint

- [x] Capture Go 1.26.4/darwin-arm64 targeted baseline benchmarks in `/tmp/voicekit-benchmarks`.
- [x] Replace fake root benchmarks with real audio conversion work and heap-visible allocation comparisons.
- [x] Add meeting/evaluation benchmarks for construction, heuristic analysis, redaction, exports, and `EvaluateMeeting`.
- [x] Optimize WAV/PCM decode and encode paths without changing public audio APIs.
- [x] Optimize basic diarization silence segmentation and pipeline segment allocation paths with stable sample/timing tests.
- [x] Replace the small clustering precomputed-similarity path with the measured direct union-find path.
- [x] Document the benchstat workflow and current July 2026 benchmark results.
- [x] Document sprint learnings for the next performance developer in `PERFORMANCE_IMPROVEMENTS.md`.
- [x] Generalize reusable benchmark and hot-path lessons into the local `golang-performance` skill.
- [x] Run broad benchmark sweep: `go test -bench=. -benchmem -run=^$ -count=6 ./audio ./diarization ./speaker ./indexing ./asr ./meeting ./evaluation .`.
- [x] Run `go test ./audio ./diarization ./meeting ./evaluation`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `go test -race ./...`.
- [x] Run `git diff --check`.

## Repository Hygiene and CI Sprint

- [x] Inspect local common workflow repo and sibling Go CI/lint conventions.
- [x] Check current official docs for Go setup caching, Gitea Actions
  compatibility, and golangci-lint v2 config shape.
- [x] Add `.gitignore` for local env files, generated benchmark/profile output,
  local model/runtime assets, and accidental root binaries.
- [x] Remove tracked generated benchmark artifacts and the stray `final-test`
  Go archive from source control.
- [x] Add `scripts/check-forbidden-files.sh` and wire it into `make verify`.
- [x] Add `.golangci.yml` with the standard correctness gate plus misspell and
  unconvert checks.
- [x] Add `.github/workflows/ci.yml` that sets up Go from `go.mod` and runs
  `make verify`.
- [x] Update `Makefile` with local/CI verification targets and benchmark output
  paths under `/tmp/voicekit-benchmarks` and `/tmp/voicekit-profiles`.
- [x] Fix production lint findings for sync.Pool slice storage, dead speaker
  benchmark/code, cleanup error handling, and standard staticcheck messages.
- [x] Update README, performance notes, product roadmap, and todo with the
  shipped repo maintenance behavior.
