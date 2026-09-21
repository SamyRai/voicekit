package meeting

import (
	"errors"
	"testing"
	"time"

	"go.glpx.pro/gdk/voice/diarization"
)

func TestFromIntegratedResultMapsSpeakerSegments(t *testing.T) {
	processedAt := time.Date(2026, 7, 4, 12, 30, 0, 0, time.UTC)
	result := &diarization.IntegratedResult{
		SessionID:   "session-123",
		Language:    "en",
		Confidence:  0.91,
		ProcessedAt: processedAt,
		SpeakerSegments: []diarization.SpeakerTextSegment{
			{
				SpeakerID:  "speaker-1",
				StartTime:  0,
				EndTime:    1.5,
				Duration:   1.5,
				Text:       "We should ship the roadmap",
				Confidence: 0.8,
				Words: []diarization.RecognitionWordInfo{
					{Text: "We", StartTime: 0, EndTime: 0.2, Confidence: 0.7},
					{Text: "should", StartTime: 0.2, EndTime: 0.5, Confidence: 0.8},
				},
			},
			{
				SpeakerID:  "speaker-2",
				StartTime:  1.5,
				EndTime:    3,
				Duration:   1.5,
				Text:       "I will review it",
				Confidence: 0.9,
			},
			{
				SpeakerID: "speaker-3",
				StartTime: 3,
				EndTime:   4,
			},
		},
	}

	meeting, err := FromIntegratedResult(result, BuildOptions{
		Title:  "Planning sync",
		Source: Source{Kind: SourceMeetingPlatform, Provider: "google_meet", Reference: "meet-abc"},
		Participants: []Participant{
			{ID: "p1", DisplayName: "Ada", SpeakerID: "speaker-1"},
			{ID: "p2", DisplayName: "Grace", SpeakerID: "speaker-2"},
		},
	})
	if err != nil {
		t.Fatalf("FromIntegratedResult returned error: %v", err)
	}

	if meeting.ID != "session-123" {
		t.Fatalf("expected meeting id from session, got %q", meeting.ID)
	}
	if meeting.CreatedAt != processedAt {
		t.Fatalf("expected created_at from processed_at, got %v", meeting.CreatedAt)
	}
	if meeting.Source.Kind != SourceMeetingPlatform || meeting.Source.Provider != "google_meet" {
		t.Fatalf("unexpected source: %+v", meeting.Source)
	}
	if len(meeting.Transcript) != 2 {
		t.Fatalf("expected empty segment to be skipped, got %d transcript turns", len(meeting.Transcript))
	}

	first := meeting.Transcript[0]
	if first.ParticipantID != "p1" || first.SpeakerLabel != "Ada" {
		t.Fatalf("expected participant mapping for first turn, got %+v", first)
	}
	if first.StartTime != 0 || first.EndTime != 1500*time.Millisecond {
		t.Fatalf("unexpected first turn timing: %s-%s", first.StartTime, first.EndTime)
	}
	if len(first.Words) != 2 {
		t.Fatalf("expected word timings to be preserved, got %d words", len(first.Words))
	}
	if first.Words[1].Text != "should" || first.Words[1].EndTime != 500*time.Millisecond {
		t.Fatalf("unexpected second word: %+v", first.Words[1])
	}

	second := meeting.Transcript[1]
	if second.ParticipantID != "p2" || second.SpeakerLabel != "Grace" {
		t.Fatalf("expected participant mapping for second turn, got %+v", second)
	}
	if got := meeting.Duration(); got != 3*time.Second {
		t.Fatalf("expected duration from transcript turns, got %s", got)
	}

	talkTime := meeting.SpeakerTalkTime()
	if talkTime["speaker-1"] != 1500*time.Millisecond || talkTime["speaker-2"] != 1500*time.Millisecond {
		t.Fatalf("unexpected speaker talk time: %+v", talkTime)
	}

	participant, ok := meeting.ParticipantForSpeaker("speaker-1")
	if !ok || participant.DisplayName != "Ada" {
		t.Fatalf("expected participant lookup for speaker-1, got %+v ok=%v", participant, ok)
	}
}

func TestFromIntegratedResultRequiresInputAndID(t *testing.T) {
	if _, err := FromIntegratedResult(nil, BuildOptions{}); !errors.Is(err, ErrIntegratedResultRequired) {
		t.Fatalf("expected ErrIntegratedResultRequired, got %v", err)
	}

	if _, err := FromIntegratedResult(&diarization.IntegratedResult{}, BuildOptions{}); !errors.Is(err, ErrMeetingIDRequired) {
		t.Fatalf("expected ErrMeetingIDRequired, got %v", err)
	}
}

func TestMeetingValidateRejectsInvalidArtifacts(t *testing.T) {
	tests := []struct {
		name    string
		meeting Meeting
		wantErr error
	}{
		{
			name:    "missing meeting id",
			meeting: Meeting{},
			wantErr: ErrMeetingIDRequired,
		},
		{
			name: "invalid meeting wall clock timing",
			meeting: Meeting{
				ID:        "m1",
				StartedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2026, 7, 4, 11, 0, 0, 0, time.UTC),
			},
			wantErr: ErrInvalidTiming,
		},
		{
			name: "duplicate participant id",
			meeting: Meeting{
				ID: "m1",
				Participants: []Participant{
					{ID: "p1", SpeakerID: "speaker-1"},
					{ID: "p1", SpeakerID: "speaker-2"},
				},
			},
		},
		{
			name: "invalid turn timing",
			meeting: Meeting{
				ID: "m1",
				Transcript: []TranscriptTurn{
					{SpeakerID: "speaker-1", StartTime: 2 * time.Second, EndTime: time.Second},
				},
			},
			wantErr: ErrInvalidTiming,
		},
		{
			name: "invalid word timing",
			meeting: Meeting{
				ID: "m1",
				Transcript: []TranscriptTurn{
					{
						SpeakerID: "speaker-1",
						StartTime: 0,
						EndTime:   2 * time.Second,
						Words: []Word{
							{Text: "hello", StartTime: time.Second, EndTime: 500 * time.Millisecond},
						},
					},
				},
			},
			wantErr: ErrInvalidTiming,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.meeting.Validate()
			if err == nil {
				t.Fatal("expected validation error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
