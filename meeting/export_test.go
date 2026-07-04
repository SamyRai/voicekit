package meeting

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMeetingExportJSONUsesSecondsFields(t *testing.T) {
	meeting := sampleMeeting()

	data, err := meeting.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON returned error: %v", err)
	}

	var exported struct {
		ID         string `json:"id"`
		Transcript []struct {
			SpeakerLabel string  `json:"speaker_label"`
			StartSec     float64 `json:"start_sec"`
			EndSec       float64 `json:"end_sec"`
			Words        []struct {
				Text     string  `json:"text"`
				StartSec float64 `json:"start_sec"`
				EndSec   float64 `json:"end_sec"`
			} `json:"words"`
		} `json:"transcript"`
		Summary struct {
			Topics []struct {
				Title    string  `json:"title"`
				StartSec float64 `json:"start_sec"`
			} `json:"topics"`
			Decisions []struct {
				Text     string  `json:"text"`
				StartSec float64 `json:"start_sec"`
			} `json:"decisions"`
		} `json:"summary"`
		TalkTimeSec map[string]float64 `json:"talk_time_sec"`
	}
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatalf("failed to unmarshal export: %v\n%s", err, string(data))
	}

	if exported.ID != "meeting-1" {
		t.Fatalf("unexpected id %q", exported.ID)
	}
	if len(exported.Transcript) != 2 {
		t.Fatalf("expected 2 transcript turns, got %d", len(exported.Transcript))
	}
	if exported.Transcript[0].SpeakerLabel != "Ada" {
		t.Fatalf("expected participant label Ada, got %q", exported.Transcript[0].SpeakerLabel)
	}
	if exported.Transcript[0].EndSec != 1.5 {
		t.Fatalf("expected first turn end_sec 1.5, got %f", exported.Transcript[0].EndSec)
	}
	if exported.Transcript[0].Words[1].Text != "go" || exported.Transcript[0].Words[1].StartSec != 0.4 {
		t.Fatalf("unexpected word export: %+v", exported.Transcript[0].Words[1])
	}
	if exported.Summary.Topics[0].StartSec != 0.25 {
		t.Fatalf("expected topic start_sec 0.25, got %f", exported.Summary.Topics[0].StartSec)
	}
	if exported.Summary.Decisions[0].StartSec != 1.75 {
		t.Fatalf("expected decision start_sec 1.75, got %f", exported.Summary.Decisions[0].StartSec)
	}
	if exported.TalkTimeSec["speaker-1"] != 1.5 || exported.TalkTimeSec["speaker-2"] != 1.75 {
		t.Fatalf("unexpected talk_time_sec: %+v", exported.TalkTimeSec)
	}

	if strings.Contains(string(data), `"start_time"`) {
		t.Fatalf("JSON export should use seconds fields, got %s", string(data))
	}
}

func TestMeetingExportMarkdownRendersNotesAndTranscript(t *testing.T) {
	markdown, err := sampleMeeting().ExportMarkdown()
	if err != nil {
		t.Fatalf("ExportMarkdown returned error: %v", err)
	}

	wantFragments := []string{
		"# Product sync\n",
		"## Summary\n",
		"Finalize the export layer.",
		"### Action Items\n",
		"- [ ] Review exporter output - Ada (due 2026-07-05)",
		"## Transcript\n",
		"- `00:00` **Ada:** Ready to go",
		"- `00:01` **Grace:** Ship it now",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(markdown, fragment) {
			t.Fatalf("expected markdown to contain %q\n%s", fragment, markdown)
		}
	}
}

func TestMeetingExportSRTAndWebVTT(t *testing.T) {
	meeting := sampleMeeting()

	srt, err := meeting.ExportSRT()
	if err != nil {
		t.Fatalf("ExportSRT returned error: %v", err)
	}
	wantSRT := "1\n00:00:00,000 --> 00:00:01,500\nAda: Ready to go\n\n2\n00:00:01,500 --> 00:00:03,250\nGrace: Ship it now\n\n"
	if srt != wantSRT {
		t.Fatalf("unexpected SRT:\n%s", srt)
	}

	vtt, err := meeting.ExportWebVTT()
	if err != nil {
		t.Fatalf("ExportWebVTT returned error: %v", err)
	}
	wantVTT := "WEBVTT\n\nturn-1\n00:00:00.000 --> 00:00:01.500\nAda: Ready to go\n\nturn-2\n00:00:01.500 --> 00:00:03.250\nGrace: Ship it now\n\n"
	if vtt != wantVTT {
		t.Fatalf("unexpected WebVTT:\n%s", vtt)
	}
}

func TestCaptionExportsRequirePositiveTiming(t *testing.T) {
	meeting := Meeting{
		ID:        "meeting-1",
		CreatedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		Transcript: []TranscriptTurn{
			{ID: "turn-1", SpeakerID: "speaker-1", Text: "missing timing"},
		},
	}

	if _, err := meeting.ExportSRT(); !errors.Is(err, ErrCaptionTimingRequired) {
		t.Fatalf("expected ErrCaptionTimingRequired from SRT export, got %v", err)
	}
	if _, err := meeting.ExportWebVTT(); !errors.Is(err, ErrCaptionTimingRequired) {
		t.Fatalf("expected ErrCaptionTimingRequired from WebVTT export, got %v", err)
	}
}

func sampleMeeting() Meeting {
	return Meeting{
		ID:        "meeting-1",
		Title:     "Product sync",
		Language:  "en",
		Source:    Source{Kind: SourceMeetingPlatform, Provider: "google_meet"},
		CreatedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		Participants: []Participant{
			{ID: "p1", DisplayName: "Ada", SpeakerID: "speaker-1"},
			{ID: "p2", DisplayName: "Grace", SpeakerID: "speaker-2"},
		},
		Transcript: []TranscriptTurn{
			{
				ID:            "turn-1",
				SpeakerID:     "speaker-1",
				ParticipantID: "p1",
				SpeakerLabel:  "Ada",
				Text:          "Ready to go",
				StartTime:     0,
				EndTime:       1500 * time.Millisecond,
				Confidence:    0.91,
				Words: []Word{
					{Text: "Ready", StartTime: 0, EndTime: 400 * time.Millisecond, Confidence: 0.9},
					{Text: "go", StartTime: 400 * time.Millisecond, EndTime: 900 * time.Millisecond, Confidence: 0.92},
				},
			},
			{
				ID:            "turn-2",
				SpeakerID:     "speaker-2",
				ParticipantID: "p2",
				SpeakerLabel:  "Grace",
				Text:          "Ship it now",
				StartTime:     1500 * time.Millisecond,
				EndTime:       3250 * time.Millisecond,
				Confidence:    0.87,
			},
		},
		Summary: MeetingSummary{
			Overview: "Finalize the export layer.",
			Topics: []Topic{
				{ID: "topic-1", Title: "Exports", Summary: "Markdown, JSON, and captions.", StartTime: 250 * time.Millisecond, EndTime: 2 * time.Second},
			},
			Decisions: []Decision{
				{ID: "decision-1", Text: "Ship meeting exporters.", StartTime: 1750 * time.Millisecond},
			},
			ActionItems: []ActionItem{
				{
					ID:        "action-1",
					Text:      "Review exporter output",
					OwnerID:   "p1",
					OwnerName: "Ada",
					DueAt:     time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC),
					Status:    ActionItemOpen,
				},
			},
		},
	}
}
