package meeting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrAnalyzerRequired       = errors.New("meeting analyzer is required")
	ErrAnalysisRequestInvalid = errors.New("meeting analysis request is invalid")
	ErrAnalysisOutputInvalid  = errors.New("meeting analysis output is invalid")
)

// Analyzer generates provider-neutral structured meeting notes.
type Analyzer interface {
	AnalyzeMeeting(ctx context.Context, meeting Meeting, request AnalysisRequest) (*AnalysisResult, error)
}

// AnalysisScope names one structured artifact family requested from an analyzer.
type AnalysisScope string

const (
	AnalysisScopeOverview      AnalysisScope = "overview"
	AnalysisScopeTopics        AnalysisScope = "topics"
	AnalysisScopeDecisions     AnalysisScope = "decisions"
	AnalysisScopeActionItems   AnalysisScope = "action_items"
	AnalysisScopeFollowUps     AnalysisScope = "follow_ups"
	AnalysisScopeOpenQuestions AnalysisScope = "open_questions"
	AnalysisScopeRisks         AnalysisScope = "risks"
)

// AnalysisRequest configures an analyzer run without binding VoiceKit to a provider.
type AnalysisRequest struct {
	Scopes        []AnalysisScope `json:"scopes,omitempty"`
	Language      string          `json:"language,omitempty"`
	Instructions  string          `json:"instructions,omitempty"`
	ReferenceTime time.Time       `json:"reference_time,omitempty"`
}

// AnalysisResult is the provider-neutral structured output returned by an analyzer.
type AnalysisResult struct {
	Summary  MeetingSummary   `json:"summary"`
	Metadata AnalysisMetadata `json:"metadata,omitempty"`
	Warnings []string         `json:"warnings,omitempty"`
}

// AnalysisMetadata captures optional provenance without coupling to provider SDKs.
type AnalysisMetadata struct {
	Analyzer    string    `json:"analyzer,omitempty"`
	Provider    string    `json:"provider,omitempty"`
	Model       string    `json:"model,omitempty"`
	GeneratedAt time.Time `json:"generated_at,omitempty"`
}

// DefaultAnalysisScopes returns the full artifact set requested by default.
func DefaultAnalysisScopes() []AnalysisScope {
	return []AnalysisScope{
		AnalysisScopeOverview,
		AnalysisScopeTopics,
		AnalysisScopeDecisions,
		AnalysisScopeActionItems,
		AnalysisScopeFollowUps,
		AnalysisScopeOpenQuestions,
		AnalysisScopeRisks,
	}
}

// Normalize trims request fields, deduplicates scopes, and expands an empty scope list to the default artifact set.
func (r AnalysisRequest) Normalize() AnalysisRequest {
	r.Language = strings.TrimSpace(r.Language)
	r.Instructions = strings.TrimSpace(r.Instructions)
	if len(r.Scopes) == 0 {
		r.Scopes = DefaultAnalysisScopes()
		return r
	}

	scopes := make([]AnalysisScope, 0, len(r.Scopes))
	seen := make(map[AnalysisScope]struct{}, len(r.Scopes))
	for _, scope := range r.Scopes {
		scope = AnalysisScope(strings.TrimSpace(string(scope)))
		if scope == "" {
			continue
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	if len(scopes) == 0 {
		scopes = DefaultAnalysisScopes()
	}
	r.Scopes = scopes
	return r
}

// Validate checks the request scope set.
func (r AnalysisRequest) Validate() error {
	for index, scope := range r.Normalize().Scopes {
		if !isSupportedAnalysisScope(scope) {
			return fmt.Errorf("%w: scope %d %q is not supported", ErrAnalysisRequestInvalid, index, scope)
		}
	}
	return nil
}

// Analyze runs analyzer against the meeting and returns a copy with the validated summary applied.
func (m Meeting) Analyze(ctx context.Context, analyzer Analyzer, request AnalysisRequest) (Meeting, AnalysisResult, error) {
	if analyzer == nil {
		return Meeting{}, AnalysisResult{}, ErrAnalyzerRequired
	}
	if err := m.Validate(); err != nil {
		return Meeting{}, AnalysisResult{}, err
	}

	request = request.Normalize()
	if err := request.Validate(); err != nil {
		return Meeting{}, AnalysisResult{}, err
	}

	result, err := analyzer.AnalyzeMeeting(ctx, m, request)
	if err != nil {
		return Meeting{}, AnalysisResult{}, err
	}
	if result == nil {
		return Meeting{}, AnalysisResult{}, fmt.Errorf("%w: analyzer returned nil result", ErrAnalysisOutputInvalid)
	}

	next, normalized, err := m.WithAnalysis(*result)
	if err != nil {
		return Meeting{}, AnalysisResult{}, err
	}
	return next, normalized, nil
}

// WithAnalysis returns a copy of the meeting with normalized analyzer output applied.
func (m Meeting) WithAnalysis(result AnalysisResult) (Meeting, AnalysisResult, error) {
	if err := m.Validate(); err != nil {
		return Meeting{}, AnalysisResult{}, err
	}

	normalized := result.Normalize()
	if err := normalized.ValidateForMeeting(m); err != nil {
		return Meeting{}, AnalysisResult{}, err
	}

	m.Summary = normalized.Summary
	return m, normalized, nil
}

// Normalize trims text, fills stable item IDs, and removes empty evidence/warning entries.
func (r AnalysisResult) Normalize() AnalysisResult {
	r.Summary = r.Summary.Normalize()
	r.Metadata.Analyzer = strings.TrimSpace(r.Metadata.Analyzer)
	r.Metadata.Provider = strings.TrimSpace(r.Metadata.Provider)
	r.Metadata.Model = strings.TrimSpace(r.Metadata.Model)
	r.Warnings = cleanStringList(r.Warnings)
	return r
}

// ValidateForMeeting checks analyzer output against the meeting transcript contract.
func (r AnalysisResult) ValidateForMeeting(meeting Meeting) error {
	if r.Summary.isZero() {
		return fmt.Errorf("%w: summary is empty", ErrAnalysisOutputInvalid)
	}
	return r.Summary.ValidateForMeeting(meeting)
}

// Normalize trims summary text and fills stable IDs for generated artifacts.
func (s MeetingSummary) Normalize() MeetingSummary {
	s.Overview = normalizeGeneratedText(s.Overview)
	s.Topics = normalizeTopics(s.Topics)
	s.Decisions = normalizeDecisions(s.Decisions)
	s.ActionItems = normalizeActionItems(s.ActionItems)
	s.FollowUps = normalizeFollowUps(s.FollowUps)
	s.OpenQuestions = normalizeQuestions(s.OpenQuestions)
	s.Risks = normalizeRisks(s.Risks)
	return s
}

// ValidateForMeeting checks summary artifacts and their transcript evidence references.
func (s MeetingSummary) ValidateForMeeting(meeting Meeting) error {
	if s.isZero() {
		return fmt.Errorf("%w: summary is empty", ErrAnalysisOutputInvalid)
	}

	evidenceIDs := transcriptTurnIDs(meeting.Transcript)
	if err := validateTopics(s.Topics); err != nil {
		return err
	}
	if err := validateDecisions(s.Decisions, evidenceIDs); err != nil {
		return err
	}
	if err := validateActionItems(s.ActionItems, evidenceIDs); err != nil {
		return err
	}
	if err := validateFollowUps(s.FollowUps, evidenceIDs); err != nil {
		return err
	}
	if err := validateQuestions(s.OpenQuestions, evidenceIDs); err != nil {
		return err
	}
	if err := validateRisks(s.Risks, evidenceIDs); err != nil {
		return err
	}
	return nil
}

func isSupportedAnalysisScope(scope AnalysisScope) bool {
	switch scope {
	case AnalysisScopeOverview,
		AnalysisScopeTopics,
		AnalysisScopeDecisions,
		AnalysisScopeActionItems,
		AnalysisScopeFollowUps,
		AnalysisScopeOpenQuestions,
		AnalysisScopeRisks:
		return true
	default:
		return false
	}
}

func normalizeTopics(topics []Topic) []Topic {
	if len(topics) == 0 {
		return nil
	}
	normalized := make([]Topic, 0, len(topics))
	for index, topic := range topics {
		topic.ID = defaultArtifactID(topic.ID, "topic", index)
		topic.Title = normalizeGeneratedText(topic.Title)
		topic.Summary = normalizeGeneratedText(topic.Summary)
		normalized = append(normalized, topic)
	}
	return normalized
}

func normalizeDecisions(decisions []Decision) []Decision {
	if len(decisions) == 0 {
		return nil
	}
	normalized := make([]Decision, 0, len(decisions))
	for index, decision := range decisions {
		decision.ID = defaultArtifactID(decision.ID, "decision", index)
		decision.Text = normalizeGeneratedText(decision.Text)
		decision.OwnerID = strings.TrimSpace(decision.OwnerID)
		decision.EvidenceIDs = cleanStringList(decision.EvidenceIDs)
		normalized = append(normalized, decision)
	}
	return normalized
}

func normalizeActionItems(actionItems []ActionItem) []ActionItem {
	if len(actionItems) == 0 {
		return nil
	}
	normalized := make([]ActionItem, 0, len(actionItems))
	for index, item := range actionItems {
		item.ID = defaultArtifactID(item.ID, "action", index)
		item.Text = normalizeGeneratedText(item.Text)
		item.OwnerID = strings.TrimSpace(item.OwnerID)
		item.OwnerName = normalizeGeneratedText(item.OwnerName)
		item.Status = ActionItemStatus(strings.TrimSpace(string(item.Status)))
		if item.Status == "" {
			item.Status = ActionItemProposed
		}
		item.EvidenceIDs = cleanStringList(item.EvidenceIDs)
		normalized = append(normalized, item)
	}
	return normalized
}

func normalizeFollowUps(followUps []FollowUp) []FollowUp {
	if len(followUps) == 0 {
		return nil
	}
	normalized := make([]FollowUp, 0, len(followUps))
	for index, followUp := range followUps {
		followUp.ID = defaultArtifactID(followUp.ID, "follow-up", index)
		followUp.Text = normalizeGeneratedText(followUp.Text)
		followUp.RecipientID = strings.TrimSpace(followUp.RecipientID)
		followUp.Channel = strings.TrimSpace(followUp.Channel)
		followUp.EvidenceIDs = cleanStringList(followUp.EvidenceIDs)
		normalized = append(normalized, followUp)
	}
	return normalized
}

func normalizeQuestions(questions []Question) []Question {
	if len(questions) == 0 {
		return nil
	}
	normalized := make([]Question, 0, len(questions))
	for index, question := range questions {
		question.ID = defaultArtifactID(question.ID, "question", index)
		question.Text = normalizeGeneratedText(question.Text)
		question.OwnerID = strings.TrimSpace(question.OwnerID)
		question.EvidenceIDs = cleanStringList(question.EvidenceIDs)
		normalized = append(normalized, question)
	}
	return normalized
}

func normalizeRisks(risks []Risk) []Risk {
	if len(risks) == 0 {
		return nil
	}
	normalized := make([]Risk, 0, len(risks))
	for index, risk := range risks {
		risk.ID = defaultArtifactID(risk.ID, "risk", index)
		risk.Text = normalizeGeneratedText(risk.Text)
		risk.Severity = strings.TrimSpace(risk.Severity)
		risk.EvidenceIDs = cleanStringList(risk.EvidenceIDs)
		normalized = append(normalized, risk)
	}
	return normalized
}

func validateTopics(topics []Topic) error {
	ids := make(map[string]struct{}, len(topics))
	for index, topic := range topics {
		if strings.TrimSpace(topic.ID) != "" {
			if err := validateUniqueArtifactID(ids, "topic", index, topic.ID); err != nil {
				return err
			}
		}
		if strings.TrimSpace(topic.Title) == "" {
			return fmt.Errorf("%w: topic %d title is required", ErrAnalysisOutputInvalid, index)
		}
		if topic.StartTime < 0 || topic.EndTime < 0 || topic.EndTime < topic.StartTime {
			return fmt.Errorf("%w: topic %d: %w", ErrAnalysisOutputInvalid, index, ErrInvalidTiming)
		}
	}
	return nil
}

func validateDecisions(decisions []Decision, evidenceIDs map[string]struct{}) error {
	ids := make(map[string]struct{}, len(decisions))
	for index, decision := range decisions {
		if strings.TrimSpace(decision.ID) != "" {
			if err := validateUniqueArtifactID(ids, "decision", index, decision.ID); err != nil {
				return err
			}
		}
		if strings.TrimSpace(decision.Text) == "" {
			return fmt.Errorf("%w: decision %d text is required", ErrAnalysisOutputInvalid, index)
		}
		if decision.StartTime < 0 {
			return fmt.Errorf("%w: decision %d: %w", ErrAnalysisOutputInvalid, index, ErrInvalidTiming)
		}
		if err := validateEvidenceIDs("decision", index, decision.EvidenceIDs, evidenceIDs); err != nil {
			return err
		}
	}
	return nil
}

func validateActionItems(actionItems []ActionItem, evidenceIDs map[string]struct{}) error {
	ids := make(map[string]struct{}, len(actionItems))
	for index, item := range actionItems {
		if strings.TrimSpace(item.ID) != "" {
			if err := validateUniqueArtifactID(ids, "action item", index, item.ID); err != nil {
				return err
			}
		}
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("%w: action item %d text is required", ErrAnalysisOutputInvalid, index)
		}
		if !isValidActionItemStatus(item.Status) {
			return fmt.Errorf("%w: action item %d status %q is not supported", ErrAnalysisOutputInvalid, index, item.Status)
		}
		if err := validateEvidenceIDs("action item", index, item.EvidenceIDs, evidenceIDs); err != nil {
			return err
		}
	}
	return nil
}

func validateFollowUps(followUps []FollowUp, evidenceIDs map[string]struct{}) error {
	ids := make(map[string]struct{}, len(followUps))
	for index, followUp := range followUps {
		if strings.TrimSpace(followUp.ID) != "" {
			if err := validateUniqueArtifactID(ids, "follow up", index, followUp.ID); err != nil {
				return err
			}
		}
		if strings.TrimSpace(followUp.Text) == "" {
			return fmt.Errorf("%w: follow up %d text is required", ErrAnalysisOutputInvalid, index)
		}
		if err := validateEvidenceIDs("follow up", index, followUp.EvidenceIDs, evidenceIDs); err != nil {
			return err
		}
	}
	return nil
}

func validateQuestions(questions []Question, evidenceIDs map[string]struct{}) error {
	ids := make(map[string]struct{}, len(questions))
	for index, question := range questions {
		if strings.TrimSpace(question.ID) != "" {
			if err := validateUniqueArtifactID(ids, "question", index, question.ID); err != nil {
				return err
			}
		}
		if strings.TrimSpace(question.Text) == "" {
			return fmt.Errorf("%w: question %d text is required", ErrAnalysisOutputInvalid, index)
		}
		if err := validateEvidenceIDs("question", index, question.EvidenceIDs, evidenceIDs); err != nil {
			return err
		}
	}
	return nil
}

func validateRisks(risks []Risk, evidenceIDs map[string]struct{}) error {
	ids := make(map[string]struct{}, len(risks))
	for index, risk := range risks {
		if strings.TrimSpace(risk.ID) != "" {
			if err := validateUniqueArtifactID(ids, "risk", index, risk.ID); err != nil {
				return err
			}
		}
		if strings.TrimSpace(risk.Text) == "" {
			return fmt.Errorf("%w: risk %d text is required", ErrAnalysisOutputInvalid, index)
		}
		if err := validateEvidenceIDs("risk", index, risk.EvidenceIDs, evidenceIDs); err != nil {
			return err
		}
	}
	return nil
}

func isValidActionItemStatus(status ActionItemStatus) bool {
	switch status {
	case "", ActionItemProposed, ActionItemOpen, ActionItemDone, ActionItemDismissed:
		return true
	default:
		return false
	}
}

func validateEvidenceIDs(kind string, index int, ids []string, known map[string]struct{}) error {
	for _, id := range ids {
		if _, exists := known[id]; !exists {
			return fmt.Errorf("%w: %s %d references unknown evidence id %q", ErrAnalysisOutputInvalid, kind, index, id)
		}
	}
	return nil
}

func validateUniqueArtifactID(ids map[string]struct{}, kind string, index int, id string) error {
	if _, exists := ids[id]; exists {
		return fmt.Errorf("%w: duplicate %s id %q at index %d", ErrAnalysisOutputInvalid, kind, id, index)
	}
	ids[id] = struct{}{}
	return nil
}

func transcriptTurnIDs(turns []TranscriptTurn) map[string]struct{} {
	ids := make(map[string]struct{}, len(turns))
	for _, turn := range turns {
		id := strings.TrimSpace(turn.ID)
		if id == "" {
			continue
		}
		ids[id] = struct{}{}
	}
	return ids
}

func defaultArtifactID(id string, prefix string, index int) string {
	id = strings.TrimSpace(id)
	if id != "" {
		return id
	}
	return fmt.Sprintf("%s-%03d", prefix, index+1)
}

func normalizeGeneratedText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func cleanStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cleaned := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	return cleaned
}
