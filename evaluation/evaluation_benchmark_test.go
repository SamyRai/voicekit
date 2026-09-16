package evaluation

import (
	"fmt"
	"testing"
	"time"

	"go.glpx.pro/voicekit/meeting"
)

var benchmarkReportSink Report

func BenchmarkEvaluateMeeting(b *testing.B) {
	reference, prediction := benchmarkEvaluationFixture(32)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		report, err := EvaluateMeeting(reference, prediction)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkReportSink = report
	}
}

func benchmarkEvaluationFixture(turnCount int) (MeetingReference, meeting.Meeting) {
	referenceTurns := make([]ReferenceTurn, 0, turnCount)
	predictionTurns := make([]meeting.TranscriptTurn, 0, turnCount)
	for i := 0; i < turnCount; i++ {
		id := fmt.Sprintf("turn-%03d", i+1)
		speakerID := "speaker-1"
		if i%2 == 1 {
			speakerID = "speaker-2"
		}
		referenceText := fmt.Sprintf("Project Atlas turn %02d will review the export benchmark", i+1)
		predictionText := referenceText
		if i%7 == 0 {
			predictionText = fmt.Sprintf("Project Atlas turn %02d will review export benchmark", i+1)
		}

		start := time.Duration(i) * 1200 * time.Millisecond
		end := start + time.Second
		referenceTurns = append(referenceTurns, ReferenceTurn{
			ID:        id,
			SpeakerID: speakerID,
			Text:      referenceText,
			StartTime: start,
			EndTime:   end,
		})
		predictionTurns = append(predictionTurns, meeting.TranscriptTurn{
			ID:         id,
			SpeakerID:  speakerID,
			Text:       predictionText,
			StartTime:  start,
			EndTime:    end,
			Confidence: 0.9,
		})
	}

	reference := MeetingReference{
		ID:         "benchmark-meeting",
		Transcript: referenceTurns,
		Summary: meeting.MeetingSummary{
			ActionItems: []meeting.ActionItem{
				{Text: "Review the export benchmark"},
				{Text: "Compare diarization allocations"},
			},
		},
	}
	prediction := meeting.Meeting{
		ID:         "benchmark-meeting",
		CreatedAt:  time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		Transcript: predictionTurns,
		Summary: meeting.MeetingSummary{
			ActionItems: []meeting.ActionItem{
				{Text: "Review export benchmark"},
				{Text: "Compare diarization allocations"},
				{Text: "Write performance summary"},
			},
		},
	}
	return reference, prediction
}
