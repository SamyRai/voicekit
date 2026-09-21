package evaluation

import (
	"errors"
	"testing"
	"time"

	"go.glpx.pro/gdk/voice/meeting"
)

func TestEvaluateMeetingScoresTranscriptSpeakersAndActionItems(t *testing.T) {
	reference := MeetingReference{
		ID: "meeting-1",
		Transcript: []ReferenceTurn{
			{ID: "turn-1", SpeakerID: "speaker-1", Text: "We need to ship analyzer"},
			{ID: "turn-2", SpeakerID: "speaker-2", Text: "Ada will review the exporter"},
		},
		Summary: meeting.MeetingSummary{
			ActionItems: []meeting.ActionItem{
				{Text: "Ship analyzer"},
				{Text: "Review exporter"},
			},
		},
	}
	prediction := meeting.Meeting{
		ID:        "meeting-1",
		CreatedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		Transcript: []meeting.TranscriptTurn{
			{ID: "turn-1", SpeakerID: "speaker-1", Text: "we need ship analyzer"},
			{ID: "turn-2", SpeakerID: "speaker-9", Text: "Ada will review exporter"},
		},
		Summary: meeting.MeetingSummary{
			ActionItems: []meeting.ActionItem{
				{Text: "ship analyzer"},
				{Text: "Write a recap"},
			},
		},
	}

	report, err := EvaluateMeeting(reference, prediction)
	if err != nil {
		t.Fatalf("EvaluateMeeting returned error: %v", err)
	}

	if report.Transcript.WordEdits != 2 || report.Transcript.ReferenceWords != 10 {
		t.Fatalf("unexpected word report: %+v", report.Transcript)
	}
	if report.Transcript.WordErrorRate != 0.2 {
		t.Fatalf("expected WER 0.2, got %f", report.Transcript.WordErrorRate)
	}
	if report.Transcript.CharacterErrorRate <= 0 || report.Transcript.CharacterErrorRate >= 1 {
		t.Fatalf("expected bounded CER, got %+v", report.Transcript)
	}
	if report.SpeakerAttribution.CorrectTurns != 1 || report.SpeakerAttribution.ComparedTurns != 2 {
		t.Fatalf("unexpected speaker report: %+v", report.SpeakerAttribution)
	}
	if report.SpeakerAttribution.Accuracy != 0.5 {
		t.Fatalf("expected speaker accuracy 0.5, got %f", report.SpeakerAttribution.Accuracy)
	}
	if report.ActionItems.TruePositives != 1 || report.ActionItems.FalsePositives != 1 || report.ActionItems.FalseNegatives != 1 {
		t.Fatalf("unexpected action item report: %+v", report.ActionItems)
	}
	if report.ActionItems.Precision != 0.5 || report.ActionItems.Recall != 0.5 || report.ActionItems.F1 != 0.5 {
		t.Fatalf("unexpected classification ratios: %+v", report.ActionItems)
	}
}

func TestEvaluateMeetingRejectsEmptyReference(t *testing.T) {
	_, err := EvaluateMeeting(MeetingReference{}, meeting.Meeting{ID: "meeting-1"})
	if !errors.Is(err, ErrReferenceRequired) {
		t.Fatalf("expected ErrReferenceRequired, got %v", err)
	}
}

func TestEvaluateTranscriptTextHandlesEmptyReferences(t *testing.T) {
	report := EvaluateTranscriptText(nil, nil)
	if report.WordErrorRate != 0 || report.CharacterErrorRate != 0 {
		t.Fatalf("expected empty-vs-empty transcript to score zero errors, got %+v", report)
	}

	report = EvaluateTranscriptText(nil, []meeting.TranscriptTurn{{Text: "extra words"}})
	if report.WordErrorRate != 1 || report.CharacterErrorRate != 1 {
		t.Fatalf("expected empty reference with prediction to score one, got %+v", report)
	}
}

func TestEvaluateSpeakerAttributionFallsBackToIndex(t *testing.T) {
	report := EvaluateSpeakerAttribution(
		[]ReferenceTurn{{SpeakerID: "speaker-1"}, {SpeakerID: "speaker-2"}},
		[]meeting.TranscriptTurn{{SpeakerID: "speaker-1"}},
	)
	if report.CorrectTurns != 1 || report.MissingTurns != 1 || report.ComparedTurns != 2 {
		t.Fatalf("unexpected speaker attribution fallback report: %+v", report)
	}
}

func TestEvaluateActionItemsHandlesEmptySets(t *testing.T) {
	report := EvaluateActionItems(nil, nil)
	if report.Precision != 1 || report.Recall != 1 || report.F1 != 1 {
		t.Fatalf("expected empty action-item sets to score perfect, got %+v", report)
	}

	report = EvaluateActionItems(nil, []meeting.ActionItem{{Text: "extra"}})
	if report.Precision != 0 || report.Recall != 1 || report.F1 != 0 {
		t.Fatalf("unexpected empty-reference action-item report: %+v", report)
	}
}

func TestRealTimeFactor(t *testing.T) {
	factor, err := RealTimeFactor(5*time.Second, 20*time.Second)
	if err != nil {
		t.Fatalf("RealTimeFactor returned error: %v", err)
	}
	if factor != 0.25 {
		t.Fatalf("expected factor 0.25, got %f", factor)
	}

	if _, err := RealTimeFactor(time.Second, 0); !errors.Is(err, ErrInvalidDuration) {
		t.Fatalf("expected ErrInvalidDuration, got %v", err)
	}

	report, err := EvaluateRealTimeFactor(Report{}, 3*time.Second, 6*time.Second)
	if err != nil {
		t.Fatalf("EvaluateRealTimeFactor returned error: %v", err)
	}
	if report.RealTimeFactor != 0.5 {
		t.Fatalf("expected report factor 0.5, got %+v", report)
	}
}
