package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTranscriptionJSONSeparatesTokensFromWords(t *testing.T) {
	transcription := Transcription{
		Text: "hello",
		Tokens: []Token{
			{Text: "▁hel", HasTiming: true, StartTime: 0},
			{Text: "lo", HasTiming: true, StartTime: 500 * time.Millisecond},
		},
	}

	data, err := json.Marshal(transcription)
	if err != nil {
		t.Fatalf("marshal transcription: %v", err)
	}

	var encoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &encoded); err != nil {
		t.Fatalf("decode transcription object: %v", err)
	}
	if _, ok := encoded["tokens"]; !ok {
		t.Fatalf("token contract missing from JSON: %s", data)
	}
	if _, ok := encoded["words"]; ok {
		t.Fatalf("empty word segmentation must be omitted from JSON: %s", data)
	}

	var tokens []Token
	if err := json.Unmarshal(encoded["tokens"], &tokens); err != nil {
		t.Fatalf("decode tokens: %v", err)
	}
	if len(tokens) != 2 || !tokens[0].HasTiming || tokens[0].StartTime != 0 || tokens[1].StartTime != 500*time.Millisecond {
		t.Fatalf("unexpected token round trip: %+v", tokens)
	}
}
