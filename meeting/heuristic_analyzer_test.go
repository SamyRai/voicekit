package meeting

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHeuristicAnalyzerProducesScopedArtifacts(t *testing.T) {
	meeting := heuristicMeeting()
	analyzed, result, err := meeting.Analyze(context.Background(), HeuristicAnalyzer{}, AnalysisRequest{
		Scopes: []AnalysisScope{
			AnalysisScopeDecisions,
			AnalysisScopeActionItems,
			AnalysisScopeOpenQuestions,
			AnalysisScopeRisks,
		},
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if analyzed.Summary.Overview != "" {
		t.Fatalf("overview should be omitted by scope filter, got %q", analyzed.Summary.Overview)
	}
	if len(analyzed.Summary.Decisions) != 1 || analyzed.Summary.Decisions[0].EvidenceIDs[0] != "turn-1" {
		t.Fatalf("unexpected decisions: %+v", analyzed.Summary.Decisions)
	}
	if len(analyzed.Summary.ActionItems) != 1 {
		t.Fatalf("expected one action item, got %+v", analyzed.Summary.ActionItems)
	}
	item := analyzed.Summary.ActionItems[0]
	if item.OwnerID != "p1" || item.OwnerName != "Ada" || item.EvidenceIDs[0] != "turn-2" || item.Status != ActionItemProposed {
		t.Fatalf("unexpected action item: %+v", item)
	}
	if len(analyzed.Summary.OpenQuestions) != 1 || analyzed.Summary.OpenQuestions[0].EvidenceIDs[0] != "turn-3" {
		t.Fatalf("unexpected open questions: %+v", analyzed.Summary.OpenQuestions)
	}
	if len(analyzed.Summary.Risks) != 1 || analyzed.Summary.Risks[0].Severity != "medium" || analyzed.Summary.Risks[0].EvidenceIDs[0] != "turn-4" {
		t.Fatalf("unexpected risks: %+v", analyzed.Summary.Risks)
	}
	if result.Metadata.Analyzer != heuristicAnalyzerName || result.Metadata.Provider != heuristicProviderName || result.Metadata.Model != heuristicModelName {
		t.Fatalf("unexpected metadata: %+v", result.Metadata)
	}
}

func TestHeuristicAnalyzerScopeFiltering(t *testing.T) {
	analyzed, _, err := heuristicMeeting().Analyze(context.Background(), HeuristicAnalyzer{}, AnalysisRequest{
		Scopes: []AnalysisScope{AnalysisScopeRisks},
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(analyzed.Summary.Risks) != 1 {
		t.Fatalf("expected one risk, got %+v", analyzed.Summary.Risks)
	}
	if len(analyzed.Summary.Decisions) != 0 || len(analyzed.Summary.ActionItems) != 0 || len(analyzed.Summary.OpenQuestions) != 0 {
		t.Fatalf("scope filter leaked artifacts: %+v", analyzed.Summary)
	}
}

func TestHeuristicAnalyzerEmptyTranscriptProducesOverviewAndWarning(t *testing.T) {
	analyzed, result, err := (Meeting{
		ID:        "empty",
		CreatedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
	}).Analyze(context.Background(), HeuristicAnalyzer{}, AnalysisRequest{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if analyzed.Summary.Overview != "No transcript content available." {
		t.Fatalf("unexpected empty overview: %+v", analyzed.Summary)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "transcript is empty" {
		t.Fatalf("expected empty transcript warning, got %+v", result.Warnings)
	}
}

func TestHeuristicAnalyzerRejectsInvalidRequest(t *testing.T) {
	_, err := HeuristicAnalyzer{}.AnalyzeMeeting(context.Background(), heuristicMeeting(), AnalysisRequest{
		Scopes: []AnalysisScope{"unsupported"},
	})
	if !errors.Is(err, ErrAnalysisRequestInvalid) {
		t.Fatalf("expected ErrAnalysisRequestInvalid, got %v", err)
	}
}

func TestHeuristicAnalyzerHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := HeuristicAnalyzer{}.AnalyzeMeeting(ctx, heuristicMeeting(), AnalysisRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func heuristicMeeting() Meeting {
	return Meeting{
		ID:        "meeting-heuristic",
		Language:  "en",
		CreatedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		Transcript: []TranscriptTurn{
			{ID: "turn-1", SpeakerID: "speaker-1", ParticipantID: "p1", SpeakerLabel: "Ada", Text: "We decided to ship the export guard.", StartTime: 0, EndTime: time.Second},
			{ID: "turn-2", SpeakerID: "speaker-1", ParticipantID: "p1", SpeakerLabel: "Ada", Text: "Ada will review the redaction report.", StartTime: time.Second, EndTime: 2 * time.Second},
			{ID: "turn-3", SpeakerID: "speaker-2", ParticipantID: "p2", SpeakerLabel: "Grace", Text: "Can we share this externally?", StartTime: 2 * time.Second, EndTime: 3 * time.Second},
			{ID: "turn-4", SpeakerID: "speaker-2", ParticipantID: "p2", SpeakerLabel: "Grace", Text: "The main risk is leaking phone numbers.", StartTime: 3 * time.Second, EndTime: 4 * time.Second},
		},
	}
}
