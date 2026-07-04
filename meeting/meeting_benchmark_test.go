package meeting

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/SamyRai/voicekit/diarization"
)

var benchmarkMeetingSink Meeting
var benchmarkAnalysisSink AnalysisResult
var benchmarkRedactionReportSink RedactionReport
var benchmarkBytesSink []byte
var benchmarkStringSink string

func BenchmarkFromIntegratedResult(b *testing.B) {
	result := benchmarkIntegratedResult(24)
	options := BuildOptions{
		Title: "Performance sync",
		Source: Source{
			Kind:     SourceMeetingPlatform,
			Provider: "local",
		},
		Participants: []Participant{
			{ID: "p1", DisplayName: "Ada", SpeakerID: "speaker-1"},
			{ID: "p2", DisplayName: "Grace", SpeakerID: "speaker-2"},
		},
		CreatedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		meeting, err := FromIntegratedResult(result, options)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkMeetingSink = *meeting
	}
}

func BenchmarkMeetingAnalyzeHeuristicAnalyzer(b *testing.B) {
	meeting := benchmarkMeeting(24)
	analyzer := HeuristicAnalyzer{}
	request := AnalysisRequest{}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		analyzed, result, err := meeting.Analyze(context.Background(), analyzer, request)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkMeetingSink = analyzed
		benchmarkAnalysisSink = result
	}
}

func BenchmarkMeetingRedact(b *testing.B) {
	meeting := benchmarkMeeting(24)
	policy := DefaultRedactionPolicy()
	policy.Terms = []string{"Project Atlas"}
	redactor := NewPatternRedactor(policy)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		redacted, report, err := meeting.Redact(redactor)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkMeetingSink = redacted
		benchmarkRedactionReportSink = report
	}
}

func BenchmarkMeetingExports(b *testing.B) {
	meeting := benchmarkMeeting(24)

	b.Run("JSON", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data, err := meeting.ExportJSON()
			if err != nil {
				b.Fatal(err)
			}
			benchmarkBytesSink = data
		}
	})

	b.Run("Markdown", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			markdown, err := meeting.ExportMarkdown()
			if err != nil {
				b.Fatal(err)
			}
			benchmarkStringSink = markdown
		}
	})

	b.Run("SRT", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			srt, err := meeting.ExportSRT()
			if err != nil {
				b.Fatal(err)
			}
			benchmarkStringSink = srt
		}
	})

	b.Run("WebVTT", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			vtt, err := meeting.ExportWebVTT()
			if err != nil {
				b.Fatal(err)
			}
			benchmarkStringSink = vtt
		}
	})
}

func benchmarkMeeting(turnCount int) Meeting {
	turns := make([]TranscriptTurn, 0, turnCount)
	for i := 0; i < turnCount; i++ {
		start := time.Duration(i) * 1500 * time.Millisecond
		end := start + 1250*time.Millisecond
		speakerID := "speaker-1"
		participantID := "p1"
		speakerLabel := "Ada"
		if i%2 == 1 {
			speakerID = "speaker-2"
			participantID = "p2"
			speakerLabel = "Grace"
		}

		text := fmt.Sprintf("Project Atlas turn %02d will review export performance at https://example.com/%02d", i+1, i+1)
		switch i % 5 {
		case 1:
			text = fmt.Sprintf("We decided to keep local processing for turn %02d", i+1)
		case 2:
			text = fmt.Sprintf("Can %s verify the action item before Friday?", speakerLabel)
		case 3:
			text = fmt.Sprintf("The main risk is delay from dependency %02d", i+1)
		case 4:
			text = fmt.Sprintf("Contact owner%02d@example.com or +1 555 010 %04d", i+1, i+1)
		}

		turns = append(turns, TranscriptTurn{
			ID:            fmt.Sprintf("turn-%03d", i+1),
			SpeakerID:     speakerID,
			ParticipantID: participantID,
			SpeakerLabel:  speakerLabel,
			Text:          text,
			StartTime:     start,
			EndTime:       end,
			Confidence:    0.9,
			Words: []Word{
				{Text: "Project", StartTime: start, EndTime: start + 200*time.Millisecond, Confidence: 0.9},
				{Text: "Atlas", StartTime: start + 200*time.Millisecond, EndTime: start + 400*time.Millisecond, Confidence: 0.9},
			},
		})
	}

	return Meeting{
		ID:        "benchmark-meeting",
		Title:     "Project Atlas performance sync",
		Language:  "en",
		Source:    Source{Kind: SourceMeetingPlatform, Provider: "local", Reference: "benchmark://meeting"},
		CreatedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		Participants: []Participant{
			{ID: "p1", DisplayName: "Ada", SpeakerID: "speaker-1"},
			{ID: "p2", DisplayName: "Grace", SpeakerID: "speaker-2"},
		},
		Transcript: turns,
		Summary: MeetingSummary{
			Overview: "Project Atlas local performance review.",
			Topics: []Topic{
				{ID: "topic-1", Title: "Performance", Summary: "Audio and diarization allocations.", StartTime: 0, EndTime: 10 * time.Second},
			},
			Decisions: []Decision{
				{ID: "decision-1", Text: "Keep local processing as the default.", StartTime: 1500 * time.Millisecond, EvidenceIDs: []string{"turn-002"}},
			},
			ActionItems: []ActionItem{
				{ID: "action-1", Text: "Review export performance", OwnerID: "p1", OwnerName: "Ada", Status: ActionItemOpen, EvidenceIDs: []string{"turn-001"}},
			},
			OpenQuestions: []Question{
				{ID: "question-1", Text: "Can Grace verify the action item before Friday?", OwnerID: "p2", EvidenceIDs: []string{"turn-003"}},
			},
			Risks: []Risk{
				{ID: "risk-1", Text: "Delay from dependency.", Severity: "medium", EvidenceIDs: []string{"turn-004"}},
			},
		},
	}
}

func benchmarkIntegratedResult(segmentCount int) *diarization.IntegratedResult {
	segments := make([]diarization.SpeakerTextSegment, 0, segmentCount)
	for i := 0; i < segmentCount; i++ {
		start := float64(i) * 1.5
		end := start + 1.25
		speakerID := "speaker-1"
		if i%2 == 1 {
			speakerID = "speaker-2"
		}
		segments = append(segments, diarization.SpeakerTextSegment{
			SpeakerID:  speakerID,
			StartTime:  start,
			EndTime:    end,
			Duration:   end - start,
			Text:       fmt.Sprintf("Turn %02d will review the local analyzer", i+1),
			Confidence: 0.9,
			Words: []diarization.RecognitionWordInfo{
				{Text: "Turn", StartTime: start, EndTime: start + 0.2, Confidence: 0.9},
				{Text: fmt.Sprintf("%02d", i+1), StartTime: start + 0.2, EndTime: start + 0.4, Confidence: 0.9},
			},
		})
	}
	return &diarization.IntegratedResult{
		SessionID:       "benchmark-meeting",
		Language:        "en",
		SpeakerSegments: segments,
		ProcessedAt:     time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
	}
}
