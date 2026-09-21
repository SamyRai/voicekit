package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.glpx.pro/gdk/voice/evaluation"
	"go.glpx.pro/gdk/voice/meeting"
)

func main() {
	artifact := meeting.Meeting{
		ID:        "meeting-demo",
		Title:     "Private launch review",
		Language:  "en",
		Source:    meeting.Source{Kind: meeting.SourceUpload, Reference: "https://meet.example.com/demo"},
		CreatedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		Participants: []meeting.Participant{
			{ID: "p1", DisplayName: "Ada Example", SpeakerID: "speaker-1"},
			{ID: "p2", DisplayName: "Grace Example", SpeakerID: "speaker-2"},
		},
		Transcript: []meeting.TranscriptTurn{
			{
				ID:            "turn-1",
				SpeakerID:     "speaker-1",
				ParticipantID: "p1",
				SpeakerLabel:  "Ada Example",
				Text:          "We decided to ship the local analyzer baseline.",
				StartTime:     0,
				EndTime:       2 * time.Second,
			},
			{
				ID:            "turn-2",
				SpeakerID:     "speaker-2",
				ParticipantID: "p2",
				SpeakerLabel:  "Grace Example",
				Text:          "Grace will review ada@example.com before external sharing.",
				StartTime:     2 * time.Second,
				EndTime:       4 * time.Second,
			},
			{
				ID:            "turn-3",
				SpeakerID:     "speaker-1",
				ParticipantID: "p1",
				SpeakerLabel:  "Ada Example",
				Text:          "The main risk is leaking a private contact number +1 415-555-1212.",
				StartTime:     4 * time.Second,
				EndTime:       6 * time.Second,
			},
		},
	}

	analyzed, _, err := artifact.Analyze(context.Background(), meeting.HeuristicAnalyzer{}, meeting.AnalysisRequest{})
	if err != nil {
		log.Fatal(err)
	}

	redacted, redactionReport, err := analyzed.Redact(meeting.NewPatternRedactor(meeting.DefaultRedactionPolicy()))
	if err != nil {
		log.Fatal(err)
	}

	markdown, err := redacted.ExportMarkdownWithOptions(meeting.ExportOptions{RequireRedaction: true})
	if err != nil {
		log.Fatal(err)
	}
	jsonData, err := redacted.ExportJSONWithOptions(meeting.ExportOptions{RequireRedaction: true})
	if err != nil {
		log.Fatal(err)
	}

	report, err := evaluation.EvaluateMeeting(evaluation.MeetingReference{
		ID: "meeting-demo",
		Transcript: []evaluation.ReferenceTurn{
			{ID: "turn-1", SpeakerID: "speaker-1", Text: "We decided to ship the local analyzer baseline."},
			{ID: "turn-2", SpeakerID: "speaker-2", Text: "Grace will review ada@example.com before external sharing."},
			{ID: "turn-3", SpeakerID: "speaker-1", Text: "The main risk is leaking a private contact number +1 415-555-1212."},
		},
		Summary: meeting.MeetingSummary{
			ActionItems: []meeting.ActionItem{{Text: "Grace will review ada@example.com before external sharing."}},
		},
	}, analyzed)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Markdown export:\n%s\n", markdown)
	fmt.Printf("JSON bytes: %d\n", len(jsonData))
	fmt.Printf("Redactions: %d\n", len(redactionReport.Matches))
	fmt.Printf("WER: %.2f, speaker accuracy: %.2f, action precision: %.2f\n",
		report.Transcript.WordErrorRate,
		report.SpeakerAttribution.Accuracy,
		report.ActionItems.Precision,
	)
}
