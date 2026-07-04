# VoiceKit Meeting Intelligence

This guide documents the meeting-intelligence foundation added around the
`meeting` and `evaluation` packages. It is intended for the next developer who
needs to extend analyzer implementations, privacy controls, exports, or quality
metrics without re-reading every test first.

## Contents

- [Scope](#scope)
- [Source Map](#source-map)
- [Data Flow](#data-flow)
- [Meeting Aggregate](#meeting-aggregate)
- [Analyzer Contract](#analyzer-contract)
- [Heuristic Analyzer](#heuristic-analyzer)
- [Redaction](#redaction)
- [Export Safety](#export-safety)
- [Evaluation Package](#evaluation-package)
- [Error Reference](#error-reference)
- [Development Checklist](#development-checklist)

## Scope

Meeting intelligence is a product-domain layer on top of lower-level VoiceKit
ASR and diarization primitives. It owns:

- stable meeting artifact types in `meeting`
- deterministic mapping from diarized ASR output into transcript turns
- provider-neutral analyzer contracts and output validation
- a stdlib-only reference analyzer for local examples and baseline tests
- deterministic redaction before export
- Markdown, JSON, SRT, and WebVTT exports
- deterministic fixture metrics in `evaluation`

It does not own model inference, hosted LLM SDKs, capture adapters, retention
policy, consent management, external workflow integrations, or searchable
meeting memory.

## Source Map

| Area | Files | Responsibility |
| --- | --- | --- |
| Meeting aggregate | `meeting/meeting.go` | Public meeting, participant, transcript, summary, task, question, and risk types plus timing validation. |
| Diarized ASR mapping | `meeting/from_diarization.go` | Converts `diarization.IntegratedResult` into `meeting.Meeting` transcript turns. |
| Exports | `meeting/export.go` | Markdown, product JSON, SRT, and WebVTT export formats plus redaction enforcement. |
| Analyzer contract | `meeting/analyzer.go` | Provider-neutral analyzer interface, request scopes, normalization, and validation. |
| Reference analyzer | `meeting/heuristic_analyzer.go` | Deterministic local baseline analyzer with no provider or model dependency. |
| Redaction | `meeting/redaction.go` | Redactor interface, built-in pattern redactor, meeting redaction, and redaction reports. |
| Evaluation | `evaluation/evaluation.go` | Fixture-oriented WER/CER, speaker attribution, action-item, and real-time-factor metrics. |
| Example | `examples/meeting_intelligence` | End-to-end synthetic meeting flow for analyze, redact, export, and evaluate. |

Focused tests live next to each owner file:

- `meeting/from_diarization_test.go`
- `meeting/export_test.go`
- `meeting/analyzer_test.go`
- `meeting/heuristic_analyzer_test.go`
- `meeting/redaction_test.go`
- `evaluation/evaluation_test.go`

## Data Flow

The intended local flow is:

1. Produce ASR and diarization output with lower-level VoiceKit packages.
2. Build a `meeting.Meeting` with `meeting.FromIntegratedResult` or by
   constructing the meeting aggregate directly.
3. Run `Meeting.Analyze` with an implementation of `meeting.Analyzer`.
4. Redact the analyzed meeting with `Meeting.Redact` when exported artifacts
   must avoid deterministic sensitive patterns.
5. Export the redacted meeting with `RequireRedaction` enabled.
6. Evaluate the predicted meeting against deterministic fixture references with
   `evaluation.EvaluateMeeting`.

The example command exercises steps 2 through 6 without model files:

```bash
go run ./examples/meeting_intelligence
```

## Meeting Aggregate

`meeting.Meeting` is the product-level aggregate. It contains:

- stable identity and source metadata
- participants and speaker IDs
- speaker-attributed transcript turns
- optional generated or reviewed summary artifacts
- a `Redacted` flag used by export safety checks
- wall-clock and relative timing

`Meeting.Validate` enforces the structural contract before export, analysis, or
redaction. It checks required meeting IDs, participant uniqueness, wall-clock
ordering, transcript turn timing, and word timing. It does not require summary
data because a raw transcript-only meeting is valid.

`TranscriptTurn.DisplayLabel` chooses the visible speaker label in this order:

1. `SpeakerLabel`
2. `ParticipantID`
3. `SpeakerID`

That label is used by Markdown, SRT, and WebVTT exports.

## Analyzer Contract

`meeting.Analyzer` is the extension point for generating structured meeting
notes:

```go
type Analyzer interface {
    AnalyzeMeeting(ctx context.Context, meeting Meeting, request AnalysisRequest) (*AnalysisResult, error)
}
```

`Meeting.Analyze` owns the contract boundary around analyzer implementations:

1. Rejects a nil analyzer with `ErrAnalyzerRequired`.
2. Validates the input meeting.
3. Normalizes and validates the `AnalysisRequest`.
4. Calls the analyzer.
5. Rejects nil analyzer output with `ErrAnalysisOutputInvalid`.
6. Normalizes the analyzer result.
7. Validates generated artifacts against the meeting transcript.
8. Returns a copy of the meeting with `Summary` applied.

`AnalysisRequest` supports scoped output. Empty scopes expand to
`DefaultAnalysisScopes`, which currently includes overview, topics, decisions,
action items, follow-ups, open questions, and risks. Unsupported scopes return
`ErrAnalysisRequestInvalid`.

`AnalysisResult.Normalize` and `MeetingSummary.Normalize` trim generated text,
fill stable artifact IDs, default missing action item statuses to `proposed`,
and remove empty or duplicate evidence IDs. Validation then checks duplicate
artifact IDs, unsupported statuses, invalid timing, empty output, and references
to transcript evidence IDs that do not exist.

### Adding A Real Analyzer

Keep provider adapters behind `meeting.Analyzer`. A local LLM or hosted provider
adapter should:

- accept `context.Context` cancellation and deadlines
- avoid importing provider SDKs into `meeting`
- map provider output into `meeting.MeetingSummary`
- attach transcript turn IDs as `EvidenceIDs` when possible
- return provider metadata through `AnalysisMetadata`
- rely on `Meeting.Analyze` or `Meeting.WithAnalysis` for normalization and
  validation
- keep prompt templates, HTTP clients, keys, and model selection outside the
  core meeting schema

Do not bypass `Meeting.Analyze` unless tests explicitly cover equivalent
normalization and validation.

## Heuristic Analyzer

`meeting.HeuristicAnalyzer` is a deterministic reference baseline. It is useful
for examples, tests, and contract smoke checks. It is not intended to match LLM
quality.

Behavior:

- metadata is `Analyzer: "heuristic"`, `Provider: "voicekit"`, `Model: "rules-v1"`
- empty transcript returns the overview `No transcript content available.` and
  warning `transcript is empty`
- overview uses the first non-empty transcript turns, controlled by
  `OverviewMaxTurns`
- decisions are turns containing terms such as `decided`, `decision`,
  `agreed`, `approved`, `approve`, or `ship it`
- action items are turns containing phrases such as `will`, `need to`,
  `needs to`, `action`, or `todo`
- open questions are turns ending in `?` or starting with common question words
- risks are turns containing `risk`, `blocked`, `blocker`, `concern`, `delay`,
  or `dependency`
- risk severity is `high` for blocker language and `medium` for other detected
  risk terms
- generated artifacts use transcript turn IDs as `EvidenceIDs`

The analyzer honors `context.Context` before and during extraction loops.

## Redaction

Redaction is explicit and copy-based. Callers provide a `meeting.Redactor`:

```go
type Redactor interface {
    Redact(field string, text string) (string, []RedactionMatch, error)
}
```

`Meeting.Redact` validates the meeting, deep-copies mutable slices, redacts
supported string fields, sets `Meeting.Redacted = true`, and returns a
`RedactionReport`. The original meeting is not mutated.

The built-in `PatternRedactor` is configured with `RedactionPolicy`:

- `RedactEmails`
- `RedactPhones`
- `RedactURLs`
- `Terms`
- `Replacement`

`DefaultRedactionPolicy` enables email, phone-like number, and URL redaction
with `[REDACTED]` as the replacement. Literal terms are opt-in and matched
case-insensitively.

Redaction currently covers:

- meeting title
- source provider and reference
- participant display names and roles
- transcript speaker labels
- transcript turn text
- word text
- summary overview
- topic titles and summaries
- decision text
- action item text and owner name
- follow-up text
- open question text
- risk text

`RedactionMatch` intentionally does not keep the raw sensitive value. It records
only field path, kind, byte offsets, and replacement text.

### Redaction Limits

The built-in redactor is deterministic pattern redaction, not a complete privacy
or compliance system. It does not understand every locale, entity type, consent
state, retention rule, biometric policy, or free-form PII shape. For stricter
privacy requirements, add a custom `Redactor` implementation and keep tests for
the field paths that must be covered.

## Export Safety

The original export methods remain backward compatible:

- `ExportJSON`
- `ExportMarkdown`
- `ExportSRT`
- `ExportWebVTT`

Each method delegates to a `WithOptions` variant using zero options:

- `ExportJSONWithOptions`
- `ExportMarkdownWithOptions`
- `ExportSRTWithOptions`
- `ExportWebVTTWithOptions`

`ExportOptions.RequireRedaction` blocks export unless `Meeting.Redacted` is true.
Blocked exports return `ErrRedactionRequired`.

Caption exports still require positive turn timing. If a transcript turn has
`EndTime <= StartTime`, SRT and WebVTT export return
`ErrCaptionTimingRequired` with turn context.

Use this pattern for privacy-sensitive exports:

```go
redacted, report, err := analyzed.Redact(
    meeting.NewPatternRedactor(meeting.DefaultRedactionPolicy()),
)
if err != nil {
    return err
}
_ = report

markdown, err := redacted.ExportMarkdownWithOptions(meeting.ExportOptions{
    RequireRedaction: true,
})
if err != nil {
    return err
}
_ = markdown
```

## Evaluation Package

`evaluation` imports `meeting`, but `meeting` does not import `evaluation`.
This keeps product-quality scoring outside the core artifact model.

`EvaluateMeeting(reference MeetingReference, prediction meeting.Meeting)`:

1. Rejects an empty reference with `ErrReferenceRequired`.
2. Validates the predicted meeting.
3. Computes transcript WER and CER.
4. Computes turn-level speaker attribution accuracy.
5. Computes action-item precision, recall, and F1.

Transcript metrics concatenate non-empty turn text from the reference and
prediction. Normalization lowercases text, trims it, preserves letters and
numbers, and turns punctuation or whitespace into single spaces. WER is
Levenshtein edit distance over normalized words divided by reference word count.
CER uses the same approach over normalized runes.

Speaker attribution compares reference speaker IDs to predicted speaker IDs by
turn ID when available. If a reference turn ID is missing from the prediction,
it falls back to the same index. Reference turns without speaker IDs are skipped.

Action item scoring uses exact normalized text matching. It ignores owner,
status, due date, and evidence IDs for now.

`RealTimeFactor(processingDuration, audioDuration)` returns processing seconds
divided by audio seconds. It rejects negative processing duration and
non-positive audio duration with `ErrInvalidDuration`. Use
`EvaluateRealTimeFactor` to attach that number to an existing report.

### Evaluation Limits

The package is a deterministic fixture harness. It does not yet provide:

- diarization error rate
- overlap-aware speaker scoring
- semantic action-item matching
- latency distribution summaries
- memory metrics
- model-backed benchmark corpora
- hosted provider comparisons

Keep those as additive metrics in `evaluation`; do not push them into
`meeting`.

## Error Reference

| Error | Owner | Meaning |
| --- | --- | --- |
| `meeting.ErrAnalyzerRequired` | `meeting/analyzer.go` | `Meeting.Analyze` was called with nil analyzer. |
| `meeting.ErrAnalysisRequestInvalid` | `meeting/analyzer.go` | Analysis request contains unsupported scopes. |
| `meeting.ErrAnalysisOutputInvalid` | `meeting/analyzer.go` | Analyzer output is nil, empty, invalid, duplicated, or references unknown evidence. |
| `meeting.ErrRedactorRequired` | `meeting/redaction.go` | Redaction was requested without a redactor. |
| `meeting.ErrRedactionRequired` | `meeting/export.go` | Export required prior redaction but `Meeting.Redacted` is false. |
| `meeting.ErrCaptionTimingRequired` | `meeting/export.go` | Caption export saw a turn without positive duration. |
| `evaluation.ErrReferenceRequired` | `evaluation/evaluation.go` | Evaluation was requested without transcript or summary reference data. |
| `evaluation.ErrInvalidDuration` | `evaluation/evaluation.go` | Real-time-factor inputs are invalid. |

## Development Checklist

When changing this area:

1. Preserve existing export method compatibility.
2. Add focused tests next to the owner package.
3. Keep `meeting` provider-neutral and dependency-light.
4. Keep deterministic quality metrics in `evaluation`.
5. Keep privacy claims narrow and test field coverage.
6. Update `README.md`, `PRODUCT_ROADMAP_2026.md`, and `todo.md` when public
   behavior changes.
7. Run the focused and repo-wide validation commands.

Recommended validation:

```bash
go test ./meeting ./evaluation
go test ./...
go vet ./...
go test -race ./...
git diff --check
```
