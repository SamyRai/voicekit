package meeting

import (
	"context"
	"strings"
)

const (
	heuristicAnalyzerName  = "heuristic"
	heuristicProviderName  = "voicekit"
	heuristicModelName     = "rules-v1"
	defaultOverviewMaxTurn = 2
)

// HeuristicAnalyzer is a deterministic reference analyzer, not an LLM-quality summarizer.
type HeuristicAnalyzer struct {
	OverviewMaxTurns int
}

// AnalyzeMeeting extracts baseline structured notes from transcript text using deterministic rules.
func (a HeuristicAnalyzer) AnalyzeMeeting(ctx context.Context, meeting Meeting, request AnalysisRequest) (*AnalysisResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := meeting.Validate(); err != nil {
		return nil, err
	}
	request = request.Normalize()
	if err := request.Validate(); err != nil {
		return nil, err
	}

	scopes := analysisScopeSet(request.Scopes)
	turns := nonEmptyTranscriptTurns(meeting.Transcript)
	summary := MeetingSummary{}

	if hasAnalysisScope(scopes, AnalysisScopeOverview) {
		summary.Overview = a.overview(turns)
	}
	if hasAnalysisScope(scopes, AnalysisScopeDecisions) {
		summary.Decisions = a.decisions(ctx, turns)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if hasAnalysisScope(scopes, AnalysisScopeActionItems) {
		summary.ActionItems = a.actionItems(ctx, turns)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if hasAnalysisScope(scopes, AnalysisScopeOpenQuestions) {
		summary.OpenQuestions = a.questions(ctx, turns)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if hasAnalysisScope(scopes, AnalysisScopeRisks) {
		summary.Risks = a.risks(ctx, turns)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	warnings := []string(nil)
	if len(turns) == 0 {
		warnings = append(warnings, "transcript is empty")
	}

	return &AnalysisResult{
		Summary: summary,
		Metadata: AnalysisMetadata{
			Analyzer: heuristicAnalyzerName,
			Provider: heuristicProviderName,
			Model:    heuristicModelName,
		},
		Warnings: warnings,
	}, nil
}

func (a HeuristicAnalyzer) overview(turns []TranscriptTurn) string {
	if len(turns) == 0 {
		return "No transcript content available."
	}
	maxTurns := a.OverviewMaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultOverviewMaxTurn
	}
	if maxTurns > len(turns) {
		maxTurns = len(turns)
	}
	parts := make([]string, 0, maxTurns)
	for _, turn := range turns[:maxTurns] {
		parts = append(parts, normalizeGeneratedText(turn.Text))
	}
	return strings.Join(parts, " ")
}

func (a HeuristicAnalyzer) decisions(ctx context.Context, turns []TranscriptTurn) []Decision {
	decisions := make([]Decision, 0)
	for _, turn := range turns {
		if err := ctx.Err(); err != nil {
			return decisions
		}
		text := normalizeGeneratedText(turn.Text)
		lower := strings.ToLower(text)
		if containsAny(lower, "decided", "decision", "agreed", "approved", "approve", "ship it") {
			decisions = append(decisions, Decision{
				Text:        text,
				StartTime:   turn.StartTime,
				EvidenceIDs: evidenceIDsForTurn(turn),
			})
		}
	}
	return decisions
}

func (a HeuristicAnalyzer) actionItems(ctx context.Context, turns []TranscriptTurn) []ActionItem {
	items := make([]ActionItem, 0)
	for _, turn := range turns {
		if err := ctx.Err(); err != nil {
			return items
		}
		text := normalizeGeneratedText(turn.Text)
		lower := strings.ToLower(text)
		if !containsAny(lower, " will ", " need to ", " needs to ", " action ", " todo ") &&
			!strings.HasPrefix(lower, "will ") &&
			!strings.HasPrefix(lower, "need to ") {
			continue
		}

		item := ActionItem{
			Text:        text,
			Status:      ActionItemProposed,
			EvidenceIDs: evidenceIDsForTurn(turn),
		}
		if strings.TrimSpace(turn.ParticipantID) != "" {
			item.OwnerID = strings.TrimSpace(turn.ParticipantID)
		}
		if strings.TrimSpace(turn.DisplayLabel()) != "" {
			item.OwnerName = turn.DisplayLabel()
		}
		items = append(items, item)
	}
	return items
}

func (a HeuristicAnalyzer) questions(ctx context.Context, turns []TranscriptTurn) []Question {
	questions := make([]Question, 0)
	for _, turn := range turns {
		if err := ctx.Err(); err != nil {
			return questions
		}
		text := normalizeGeneratedText(turn.Text)
		if !isQuestionText(text) {
			continue
		}
		question := Question{
			Text:        text,
			EvidenceIDs: evidenceIDsForTurn(turn),
		}
		if strings.TrimSpace(turn.ParticipantID) != "" {
			question.OwnerID = strings.TrimSpace(turn.ParticipantID)
		}
		questions = append(questions, question)
	}
	return questions
}

func (a HeuristicAnalyzer) risks(ctx context.Context, turns []TranscriptTurn) []Risk {
	risks := make([]Risk, 0)
	for _, turn := range turns {
		if err := ctx.Err(); err != nil {
			return risks
		}
		text := normalizeGeneratedText(turn.Text)
		lower := strings.ToLower(text)
		if !containsAny(lower, "risk", "blocked", "blocker", "concern", "delay", "dependency") {
			continue
		}
		risks = append(risks, Risk{
			Text:        text,
			Severity:    inferRiskSeverity(lower),
			EvidenceIDs: evidenceIDsForTurn(turn),
		})
	}
	return risks
}

func nonEmptyTranscriptTurns(turns []TranscriptTurn) []TranscriptTurn {
	nonEmpty := make([]TranscriptTurn, 0, len(turns))
	for _, turn := range turns {
		if strings.TrimSpace(turn.Text) == "" {
			continue
		}
		nonEmpty = append(nonEmpty, turn)
	}
	return nonEmpty
}

func analysisScopeSet(scopes []AnalysisScope) map[AnalysisScope]struct{} {
	set := make(map[AnalysisScope]struct{}, len(scopes))
	for _, scope := range scopes {
		set[scope] = struct{}{}
	}
	return set
}

func hasAnalysisScope(scopes map[AnalysisScope]struct{}, scope AnalysisScope) bool {
	_, ok := scopes[scope]
	return ok
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func isQuestionText(text string) bool {
	trimmed := strings.TrimSpace(text)
	if strings.HasSuffix(trimmed, "?") {
		return true
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "can ") ||
		strings.HasPrefix(lower, "should ") ||
		strings.HasPrefix(lower, "what ") ||
		strings.HasPrefix(lower, "how ") ||
		strings.HasPrefix(lower, "why ") ||
		strings.HasPrefix(lower, "when ") ||
		strings.HasPrefix(lower, "who ") ||
		strings.HasPrefix(lower, "which ")
}

func inferRiskSeverity(lowerText string) string {
	switch {
	case containsAny(lowerText, "blocked", "blocker"):
		return "high"
	case containsAny(lowerText, "risk", "concern", "delay", "dependency"):
		return "medium"
	default:
		return ""
	}
}

func evidenceIDsForTurn(turn TranscriptTurn) []string {
	if strings.TrimSpace(turn.ID) == "" {
		return nil
	}
	return []string{strings.TrimSpace(turn.ID)}
}
