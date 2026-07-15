package evaluation

import (
	"errors"
	"testing"
	"time"

	"github.com/SamyRai/voicekit/meeting"
)

func TestEvaluateDiarizationRecoversMappingAndScoresBoundaryMiss(t *testing.T) {
	reference := []DiarizationSegment{
		{SpeakerID: "A", Start: 0, End: 10 * time.Second},
		{SpeakerID: "B", Start: 10 * time.Second, End: 20 * time.Second},
	}
	hypothesis := []DiarizationSegment{
		{SpeakerID: "X", Start: 0, End: 9 * time.Second},
		{SpeakerID: "Y", Start: 11 * time.Second, End: 20 * time.Second},
	}

	report, err := EvaluateDiarization(reference, hypothesis, DiarizationOptions{})
	if err != nil {
		t.Fatalf("EvaluateDiarization returned error: %v", err)
	}

	if report.ErrorRate != 0.10 {
		t.Fatalf("expected DER 0.10, got %f (report: %+v)", report.ErrorRate, report)
	}
	if report.MissedDuration != 2*time.Second {
		t.Fatalf("expected missed 2s, got %v", report.MissedDuration)
	}
	if report.FalseAlarmDuration != 0 {
		t.Fatalf("expected false alarm 0, got %v", report.FalseAlarmDuration)
	}
	if report.ConfusionDuration != 0 {
		t.Fatalf("expected confusion 0, got %v", report.ConfusionDuration)
	}
	if report.TotalReferenceDuration != 20*time.Second {
		t.Fatalf("expected total reference duration 20s, got %v", report.TotalReferenceDuration)
	}
}

func TestEvaluateDiarizationScoresConfusionWithAmbiguousMapping(t *testing.T) {
	reference := []DiarizationSegment{
		{SpeakerID: "A", Start: 0, End: 10 * time.Second},
		{SpeakerID: "B", Start: 10 * time.Second, End: 20 * time.Second},
	}
	hypothesis := []DiarizationSegment{
		{SpeakerID: "X", Start: 0, End: 20 * time.Second},
	}

	report, err := EvaluateDiarization(reference, hypothesis, DiarizationOptions{})
	if err != nil {
		t.Fatalf("EvaluateDiarization returned error: %v", err)
	}

	if report.ErrorRate != 0.50 {
		t.Fatalf("expected DER 0.50, got %f (report: %+v)", report.ErrorRate, report)
	}
	if report.ConfusionDuration != 10*time.Second {
		t.Fatalf("expected confusion 10s, got %v", report.ConfusionDuration)
	}
	if report.MissedDuration != 0 || report.FalseAlarmDuration != 0 {
		t.Fatalf("expected zero missed/false-alarm, got missed=%v falseAlarm=%v", report.MissedDuration, report.FalseAlarmDuration)
	}
}

func TestEvaluateDiarizationScoresFalseAlarm(t *testing.T) {
	reference := []DiarizationSegment{
		{SpeakerID: "A", Start: 0, End: 10 * time.Second},
		{SpeakerID: "B", Start: 10 * time.Second, End: 20 * time.Second},
	}
	hypothesis := []DiarizationSegment{
		{SpeakerID: "X", Start: 0, End: 10 * time.Second},
		{SpeakerID: "Y", Start: 10 * time.Second, End: 25 * time.Second},
	}

	report, err := EvaluateDiarization(reference, hypothesis, DiarizationOptions{})
	if err != nil {
		t.Fatalf("EvaluateDiarization returned error: %v", err)
	}

	if report.ErrorRate != 0.25 {
		t.Fatalf("expected DER 0.25, got %f (report: %+v)", report.ErrorRate, report)
	}
	if report.FalseAlarmDuration != 5*time.Second {
		t.Fatalf("expected false alarm 5s, got %v", report.FalseAlarmDuration)
	}
	if report.MissedDuration != 0 || report.ConfusionDuration != 0 {
		t.Fatalf("expected zero missed/confusion, got missed=%v confusion=%v", report.MissedDuration, report.ConfusionDuration)
	}
}

func TestEvaluateDiarizationCollarExcludesBoundaryMiss(t *testing.T) {
	reference := []DiarizationSegment{
		{SpeakerID: "A", Start: 0, End: 10 * time.Second},
		{SpeakerID: "B", Start: 10 * time.Second, End: 20 * time.Second},
	}
	hypothesis := []DiarizationSegment{
		{SpeakerID: "X", Start: 0, End: 9 * time.Second},
		{SpeakerID: "Y", Start: 11 * time.Second, End: 20 * time.Second},
	}

	report, err := EvaluateDiarization(reference, hypothesis, DiarizationOptions{Collar: time.Second})
	if err != nil {
		t.Fatalf("EvaluateDiarization returned error: %v", err)
	}

	if report.ErrorRate != 0.00 {
		t.Fatalf("expected DER 0.00 with 1s collar, got %f (report: %+v)", report.ErrorRate, report)
	}
}

func TestEvaluateDiarizationRejectsEmptyReference(t *testing.T) {
	_, err := EvaluateDiarization(nil, []DiarizationSegment{{SpeakerID: "X", Start: 0, End: time.Second}}, DiarizationOptions{})
	if !errors.Is(err, ErrReferenceRequired) {
		t.Fatalf("expected ErrReferenceRequired, got %v", err)
	}
}

func TestEvaluateDiarizationSkipOverlapExcludesOverlappedReferenceTime(t *testing.T) {
	reference := []DiarizationSegment{
		{SpeakerID: "A", Start: 0, End: 10 * time.Second},
		{SpeakerID: "B", Start: 5 * time.Second, End: 15 * time.Second},
	}
	hypothesis := []DiarizationSegment{
		{SpeakerID: "X", Start: 0, End: 10 * time.Second},
		{SpeakerID: "Y", Start: 5 * time.Second, End: 15 * time.Second},
	}

	withOverlap, err := EvaluateDiarization(reference, hypothesis, DiarizationOptions{})
	if err != nil {
		t.Fatalf("EvaluateDiarization returned error: %v", err)
	}
	skipped, err := EvaluateDiarization(reference, hypothesis, DiarizationOptions{SkipOverlap: true})
	if err != nil {
		t.Fatalf("EvaluateDiarization returned error: %v", err)
	}

	// [0,10) has A active alone [0,5) and A+B overlapping [5,10); with
	// SkipOverlap the overlapped [5,10)/[5,15) region (Nref=2) is excluded,
	// shrinking total reference time relative to the unfiltered score.
	if skipped.TotalReferenceDuration >= withOverlap.TotalReferenceDuration {
		t.Fatalf("expected SkipOverlap to reduce total reference duration: skipped=%v withOverlap=%v", skipped.TotalReferenceDuration, withOverlap.TotalReferenceDuration)
	}
}

func TestDiarizationSegmentsFromReferenceTurns(t *testing.T) {
	turns := []ReferenceTurn{
		{SpeakerID: "speaker-1", StartTime: 0, EndTime: time.Second},
		{SpeakerID: "", StartTime: time.Second, EndTime: 2 * time.Second},
	}

	segments := DiarizationSegmentsFromReferenceTurns(turns)
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment (empty speaker excluded), got %d: %+v", len(segments), segments)
	}
	if segments[0].SpeakerID != "speaker-1" || segments[0].Start != 0 || segments[0].End != time.Second {
		t.Fatalf("unexpected segment: %+v", segments[0])
	}
}

func TestEvaluateMeetingPopulatesDiarizationReport(t *testing.T) {
	reference := MeetingReference{
		ID: "meeting-1",
		Transcript: []ReferenceTurn{
			{ID: "turn-1", SpeakerID: "speaker-1", Text: "hello", StartTime: 0, EndTime: 10 * time.Second},
			{ID: "turn-2", SpeakerID: "speaker-2", Text: "world", StartTime: 10 * time.Second, EndTime: 20 * time.Second},
		},
	}
	prediction := meeting.Meeting{
		ID:        "meeting-1",
		CreatedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		Transcript: []meeting.TranscriptTurn{
			{ID: "turn-1", SpeakerID: "speaker-1", Text: "hello", StartTime: 0, EndTime: 9 * time.Second},
			{ID: "turn-2", SpeakerID: "speaker-2", Text: "world", StartTime: 11 * time.Second, EndTime: 20 * time.Second},
		},
	}

	report, err := EvaluateMeeting(reference, prediction)
	if err != nil {
		t.Fatalf("EvaluateMeeting returned error: %v", err)
	}

	if report.Diarization == nil {
		t.Fatalf("expected Diarization report to be populated")
	}
	if report.Diarization.ErrorRate != 0.10 {
		t.Fatalf("expected DER 0.10, got %f (report: %+v)", report.Diarization.ErrorRate, report.Diarization)
	}
}

func TestEvaluateMeetingOmitsDiarizationWithoutSpeakerLabels(t *testing.T) {
	reference := MeetingReference{
		ID: "meeting-1",
		Summary: meeting.MeetingSummary{
			ActionItems: []meeting.ActionItem{{Text: "Ship it"}},
		},
	}
	prediction := meeting.Meeting{ID: "meeting-1", CreatedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)}

	report, err := EvaluateMeeting(reference, prediction)
	if err != nil {
		t.Fatalf("EvaluateMeeting returned error: %v", err)
	}
	if report.Diarization != nil {
		t.Fatalf("expected no Diarization report without speaker-labeled reference turns, got %+v", report.Diarization)
	}
}
