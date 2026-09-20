# Speech Denoiser Example

This example demonstrates `denoise.SherpaSpeechDenoiser` (offline,
whole-buffer) and `denoise.SherpaStreamingDenoiser` (online, streaming/
chunked), both backed by Sherpa ONNX's speech-enhancement models (GTCRN or
DPDFNet). It does not ship model files.

The root `voicekit.Config.Denoiser` can enable the offline denoiser in the full
`VoiceKit.ProcessAudio` pipeline. This example constructs both the offline and
streaming denoisers directly so it can demonstrate their individual lifecycle
and decodes the input WAV with `audio.NewConverter`.

## Requirements

- CGO enabled.
- A Sherpa speech-enhancement ONNX model: exactly one of a GTCRN model or a
  DPDFNet model (never both).
- A mono WAV file to denoise.

```bash
export VOICEKIT_DENOISE_WAV=/path/to/audio.wav
export VOICEKIT_DENOISE_GTCRN_MODEL=/models/sherpa/denoise/gtcrn.onnx
```

Or, for a DPDFNet model:

```bash
export VOICEKIT_DENOISE_WAV=/path/to/audio.wav
export VOICEKIT_DENOISE_DPDFNET_MODEL=/models/sherpa/denoise/dpdfnet.onnx
```

Optional:

```bash
export VOICEKIT_DENOISE_PROVIDER=cpu
export VOICEKIT_DENOISE_THREADS=1
export VOICEKIT_DENOISE_SAMPLE_RATE=16000
```

## Running

```bash
go run ./examples/denoise
```

If `VOICEKIT_DENOISE_WAV` is unset, or neither/both of
`VOICEKIT_DENOISE_GTCRN_MODEL`/`VOICEKIT_DENOISE_DPDFNET_MODEL` is set, the
example exits cleanly and prints what's missing.

## Notes

- The offline denoiser (`Denoise`) processes the whole input buffer in one
  call at the caller-supplied sample rate.
- The streaming denoiser (`Accept`/`Flush`/`Reset`) processes audio
  incrementally at a sample rate fixed by the model at construction time.
  This demo feeds fixed-size chunks and does not resample, so it is only
  representative when the WAV's decode sample rate matches the streaming
  model's rate.
- `Reset` clears stream state so a `SherpaStreamingDenoiser` instance can be
  reused for a new stream without reconstructing it; `Close` releases the
  native denoiser and cannot be undone.
