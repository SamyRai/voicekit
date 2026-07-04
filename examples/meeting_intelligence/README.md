# Meeting Intelligence Example

This example demonstrates the stdlib-only meeting intelligence foundation:

- build a synthetic `meeting.Meeting`
- run `meeting.HeuristicAnalyzer`
- redact sensitive text with the built-in pattern redactor
- export Markdown and JSON with redaction required
- compute deterministic evaluation metrics

It does not use model files, hosted APIs, or provider SDKs.

## Running

```bash
go run ./examples/meeting_intelligence
```
