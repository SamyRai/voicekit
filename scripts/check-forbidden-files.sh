#!/usr/bin/env sh
set -eu

bad=0

check_tracked() {
	pattern=$1
	message=$2
	matches=$(git ls-files | grep -E "$pattern" || true)
	if [ -n "$matches" ]; then
		printf '%s\n%s\n' "$message" "$matches" >&2
		bad=1
	fi
}

check_tracked '(^|/)final-test$' 'Forbidden tracked build artifact:'
check_tracked '(^|/)test_benchmarks(/|$)' 'Forbidden tracked test benchmark directory:'
check_tracked '(^|/)profiles?(/|$)' 'Forbidden tracked profile output directory:'
check_tracked '(^|/)models?(/|$)|\.(onnx|gguf|safetensors|pt|pth|tflite|mlmodel)$' 'Forbidden tracked model/runtime artifact:'

env_matches=$(git ls-files | grep -E '(^|/)\.env(\.|$)' | grep -Ev '(^|/)\.env\.example$' || true)
if [ -n "$env_matches" ]; then
	printf '%s\n%s\n' 'Forbidden tracked local environment file:' "$env_matches" >&2
	bad=1
fi

exit "$bad"
