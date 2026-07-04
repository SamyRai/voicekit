package meeting

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMeetingAnalyzeAppliesNormalizedAnalyzerOutput(t *testing.T) {
	analyzer := &recordingAnalyzer{
		result: &AnalysisResult{
			Summary: MeetingSummary{
				Overview: "  Ship the analyzer contract.  ",
				Decisions: []Decision{
					{Text: "  Keep provider wiring outside VoiceKit.  ", EvidenceIDs: []string{" turn-1 ", "", "turn-1"}},
				},
				ActionItems: []ActionItem{
					{Text: "  Implement the first hosted analyzer adapter later.  ", OwnerName: " Ada ", EvidenceIDs: []string{"turn-2"}},
				},
				OpenQuestions: []Question{
					{Text: "  Which local model should be evaluated first?  ", EvidenceIDs: []string{"turn-2"}},
				},
				Risks: []Risk{
					{Text: "  Provider output may omit citations.  ", Severity: " medium ", EvidenceIDs: []string{"turn-1"}},
				},
			},
			Metadata: AnalysisMetadata{
				Analyzer: " fake-analyzer ",
				Provider: " local ",
				Model:    " test-model ",
			},
			Warnings: []string{" review before sending ", "", "review before sending"},
		},
	}

	meeting := analysisTestMeeting()
	analyzed, result, err := meeting.Analyze(context.Background(), analyzer, AnalysisRequest{
		Scopes:   []AnalysisScope{AnalysisScopeOverview, AnalysisScopeRisks, AnalysisScopeOverview},
		Language: " en ",
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if !reflect.DeepEqual(analyzer.request.Scopes, []AnalysisScope{AnalysisScopeOverview, AnalysisScopeRisks}) {
		t.Fatalf("expected normalized scopes, got %#v", analyzer.request.Scopes)
	}
	if analyzer.request.Language != "en" {
		t.Fatalf("expected normalized language, got %q", analyzer.request.Language)
	}
	if analyzed.Summary.Overview != "Ship the analyzer contract." {
		t.Fatalf("summary was not normalized: %+v", analyzed.Summary)
	}
	decision := analyzed.Summary.Decisions[0]
	if decision.ID != "decision-001" || decision.EvidenceIDs[0] != "turn-1" || len(decision.EvidenceIDs) != 1 {
		t.Fatalf("decision cleanup failed: %+v", decision)
	}
	item := analyzed.Summary.ActionItems[0]
	if item.ID != "action-001" || item.Status != ActionItemProposed || item.OwnerName != "Ada" {
		t.Fatalf("action item cleanup failed: %+v", item)
	}
	risk := analyzed.Summary.Risks[0]
	if risk.ID != "risk-001" || risk.Severity != "medium" {
		t.Fatalf("risk cleanup failed: %+v", risk)
	}
	if result.Metadata.Analyzer != "fake-analyzer" || result.Metadata.Provider != "local" || result.Metadata.Model != "test-model" {
		t.Fatalf("metadata cleanup failed: %+v", result.Metadata)
	}
	if !reflect.DeepEqual(result.Warnings, []string{"review before sending"}) {
		t.Fatalf("warning cleanup failed: %+v", result.Warnings)
	}
}

func TestMeetingAnalyzeRejectsInvalidSetupAndRequest(t *testing.T) {
	meeting := analysisTestMeeting()

	if _, _, err := meeting.Analyze(context.Background(), nil, AnalysisRequest{}); !errors.Is(err, ErrAnalyzerRequired) {
		t.Fatalf("expected ErrAnalyzerRequired, got %v", err)
	}

	analyzer := &recordingAnalyzer{result: &AnalysisResult{Summary: MeetingSummary{Overview: "ok"}}}
	_, _, err := meeting.Analyze(context.Background(), analyzer, AnalysisRequest{Scopes: []AnalysisScope{"unsupported"}})
	if !errors.Is(err, ErrAnalysisRequestInvalid) {
		t.Fatalf("expected ErrAnalysisRequestInvalid, got %v", err)
	}

	nilAnalyzer := &recordingAnalyzer{}
	_, _, err = meeting.Analyze(context.Background(), nilAnalyzer, AnalysisRequest{})
	if !errors.Is(err, ErrAnalysisOutputInvalid) {
		t.Fatalf("expected ErrAnalysisOutputInvalid, got %v", err)
	}
}

func TestAnalysisRequestDefaultScopes(t *testing.T) {
	request := AnalysisRequest{}.Normalize()
	if !reflect.DeepEqual(request.Scopes, DefaultAnalysisScopes()) {
		t.Fatalf("expected default scopes, got %#v", request.Scopes)
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("default request should validate: %v", err)
	}
}

func TestAnalysisResultValidationRejectsInvalidOutput(t *testing.T) {
	meeting := analysisTestMeeting()
	tests := []struct {
		name    string
		result  AnalysisResult
		wantErr error
	}{
		{
			name:    "empty summary",
			result:  AnalysisResult{},
			wantErr: ErrAnalysisOutputInvalid,
		},
		{
			name: "missing action text",
			result: AnalysisResult{Summary: MeetingSummary{
				ActionItems: []ActionItem{{ID: "action-1"}},
			}},
			wantErr: ErrAnalysisOutputInvalid,
		},
		{
			name: "unsupported action status",
			result: AnalysisResult{Summary: MeetingSummary{
				ActionItems: []ActionItem{{Text: "Review notes", Status: ActionItemStatus("blocked")}},
			}},
			wantErr: ErrAnalysisOutputInvalid,
		},
		{
			name: "unknown evidence id",
			result: AnalysisResult{Summary: MeetingSummary{
				Decisions: []Decision{{Text: "Ship", EvidenceIDs: []string{"missing-turn"}}},
			}},
			wantErr: ErrAnalysisOutputInvalid,
		},
		{
			name: "invalid topic timing",
			result: AnalysisResult{Summary: MeetingSummary{
				Topics: []Topic{{Title: "Timing", StartTime: 2 * time.Second, EndTime: time.Second}},
			}},
			wantErr: ErrInvalidTiming,
		},
		{
			name: "duplicate ids",
			result: AnalysisResult{Summary: MeetingSummary{
				Risks: []Risk{{ID: "risk-1", Text: "First"}, {ID: "risk-1", Text: "Second"}},
			}},
			wantErr: ErrAnalysisOutputInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.result.Normalize().ValidateForMeeting(meeting)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestAnalysisResultJSONContract(t *testing.T) {
	data, err := json.Marshal(AnalysisResult{
		Summary: MeetingSummary{
			Overview: "Contract",
			OpenQuestions: []Question{
				{ID: "question-1", Text: "What remains?"},
			},
			Risks: []Risk{
				{ID: "risk-1", Text: "Missing evidence", Severity: "low"},
			},
		},
		Metadata: AnalysisMetadata{Analyzer: "unit-test", Provider: "fake", Model: "fixture"},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	payload := string(data)
	for _, field := range []string{`"summary"`, `"open_questions"`, `"risks"`, `"metadata"`, `"provider"`} {
		if !strings.Contains(payload, field) {
			t.Fatalf("expected JSON payload to contain %s: %s", field, payload)
		}
	}
}

type recordingAnalyzer struct {
	request AnalysisRequest
	result  *AnalysisResult
}

func (a *recordingAnalyzer) AnalyzeMeeting(_ context.Context, _ Meeting, request AnalysisRequest) (*AnalysisResult, error) {
	a.request = request
	return a.result, nil
}

func analysisTestMeeting() Meeting {
	return Meeting{
		ID:        "meeting-1",
		Language:  "en",
		CreatedAt: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
		Transcript: []TranscriptTurn{
			{ID: "turn-1", SpeakerID: "speaker-1", SpeakerLabel: "Ada", Text: "We need provider neutral analysis.", StartTime: 0, EndTime: time.Second},
			{ID: "turn-2", SpeakerID: "speaker-2", SpeakerLabel: "Grace", Text: "Local adapters can come later.", StartTime: time.Second, EndTime: 2 * time.Second},
		},
	}
}
