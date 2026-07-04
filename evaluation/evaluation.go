// Package evaluation provides deterministic product-quality metrics for meetings.
package evaluation

import (
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/SamyRai/voicekit/meeting"
)

var (
	ErrReferenceRequired = errors.New("evaluation reference is required")
	ErrInvalidDuration   = errors.New("duration must be positive")
)

// MeetingReference is the reviewed ground truth used for deterministic scoring.
type MeetingReference struct {
	ID         string                 `json:"id,omitempty"`
	Transcript []ReferenceTurn        `json:"transcript,omitempty"`
	Summary    meeting.MeetingSummary `json:"summary,omitempty"`
	Duration   time.Duration          `json:"duration,omitempty"`
}

// ReferenceTurn is a reviewed transcript turn in the evaluation reference.
type ReferenceTurn struct {
	ID        string        `json:"id,omitempty"`
	SpeakerID string        `json:"speaker_id,omitempty"`
	Text      string        `json:"text"`
	StartTime time.Duration `json:"start_time,omitempty"`
	EndTime   time.Duration `json:"end_time,omitempty"`
}

// Report summarizes deterministic product-quality metrics for a prediction.
type Report struct {
	Transcript         TextReport               `json:"transcript"`
	SpeakerAttribution SpeakerAttributionReport `json:"speaker_attribution"`
	ActionItems        ClassificationReport     `json:"action_items"`
	RealTimeFactor     float64                  `json:"real_time_factor,omitempty"`
}

// TextReport contains edit-distance metrics for transcript text.
type TextReport struct {
	WordErrorRate      float64 `json:"word_error_rate"`
	CharacterErrorRate float64 `json:"character_error_rate"`
	WordEdits          int     `json:"word_edits"`
	ReferenceWords     int     `json:"reference_words"`
	CharacterEdits     int     `json:"character_edits"`
	ReferenceChars     int     `json:"reference_chars"`
}

// SpeakerAttributionReport scores turn-level speaker assignment.
type SpeakerAttributionReport struct {
	Accuracy      float64 `json:"accuracy"`
	CorrectTurns  int     `json:"correct_turns"`
	ComparedTurns int     `json:"compared_turns"`
	MissingTurns  int     `json:"missing_turns"`
}

// ClassificationReport scores extracted artifacts using exact normalized matching.
type ClassificationReport struct {
	Precision      float64 `json:"precision"`
	Recall         float64 `json:"recall"`
	F1             float64 `json:"f1"`
	TruePositives  int     `json:"true_positives"`
	FalsePositives int     `json:"false_positives"`
	FalseNegatives int     `json:"false_negatives"`
}

// EvaluateMeeting compares a predicted meeting against a reviewed reference.
func EvaluateMeeting(reference MeetingReference, prediction meeting.Meeting) (Report, error) {
	if len(reference.Transcript) == 0 && isZeroSummary(reference.Summary) {
		return Report{}, ErrReferenceRequired
	}
	if err := prediction.Validate(); err != nil {
		return Report{}, err
	}

	report := Report{
		Transcript:         EvaluateTranscriptText(reference.Transcript, prediction.Transcript),
		SpeakerAttribution: EvaluateSpeakerAttribution(reference.Transcript, prediction.Transcript),
		ActionItems:        EvaluateActionItems(reference.Summary.ActionItems, prediction.Summary.ActionItems),
	}
	return report, nil
}

// EvaluateTranscriptText scores predicted transcript text against reference text.
func EvaluateTranscriptText(reference []ReferenceTurn, prediction []meeting.TranscriptTurn) TextReport {
	referenceText := referenceTranscriptText(reference)
	predictionText := predictionTranscriptText(prediction)

	referenceWords := normalizeWords(referenceText)
	predictionWords := normalizeWords(predictionText)
	wordEdits := levenshteinStrings(referenceWords, predictionWords)

	referenceChars := normalizeRunes(referenceText)
	predictionChars := normalizeRunes(predictionText)
	characterEdits := levenshteinRunes(referenceChars, predictionChars)

	return TextReport{
		WordErrorRate:      rate(wordEdits, len(referenceWords), len(predictionWords)),
		CharacterErrorRate: rate(characterEdits, len(referenceChars), len(predictionChars)),
		WordEdits:          wordEdits,
		ReferenceWords:     len(referenceWords),
		CharacterEdits:     characterEdits,
		ReferenceChars:     len(referenceChars),
	}
}

// EvaluateSpeakerAttribution scores turn-level speaker assignment.
func EvaluateSpeakerAttribution(reference []ReferenceTurn, prediction []meeting.TranscriptTurn) SpeakerAttributionReport {
	predictedByID := make(map[string]meeting.TranscriptTurn, len(prediction))
	for _, turn := range prediction {
		if strings.TrimSpace(turn.ID) == "" {
			continue
		}
		predictedByID[turn.ID] = turn
	}

	var report SpeakerAttributionReport
	for index, ref := range reference {
		if strings.TrimSpace(ref.SpeakerID) == "" {
			continue
		}
		report.ComparedTurns++

		predicted, ok := predictedByID[strings.TrimSpace(ref.ID)]
		if !ok && index < len(prediction) {
			predicted = prediction[index]
			ok = true
		}
		if !ok {
			report.MissingTurns++
			continue
		}
		if normalizeToken(predicted.SpeakerID) == normalizeToken(ref.SpeakerID) {
			report.CorrectTurns++
		}
	}
	report.Accuracy = ratio(report.CorrectTurns, report.ComparedTurns)
	return report
}

// EvaluateActionItems scores predicted action item text using exact normalized matching.
func EvaluateActionItems(reference []meeting.ActionItem, prediction []meeting.ActionItem) ClassificationReport {
	referenceItems := actionItemSet(reference)
	predictedItems := actionItemSet(prediction)

	var report ClassificationReport
	for item := range predictedItems {
		if _, ok := referenceItems[item]; ok {
			report.TruePositives++
		} else {
			report.FalsePositives++
		}
	}
	for item := range referenceItems {
		if _, ok := predictedItems[item]; !ok {
			report.FalseNegatives++
		}
	}
	report.Precision = classificationRatio(report.TruePositives, report.TruePositives+report.FalsePositives)
	report.Recall = classificationRatio(report.TruePositives, report.TruePositives+report.FalseNegatives)
	report.F1 = f1(report.Precision, report.Recall)
	return report
}

// RealTimeFactor returns processing duration divided by audio duration.
func RealTimeFactor(processingDuration time.Duration, audioDuration time.Duration) (float64, error) {
	if processingDuration < 0 || audioDuration <= 0 {
		return 0, ErrInvalidDuration
	}
	return processingDuration.Seconds() / audioDuration.Seconds(), nil
}

// EvaluateRealTimeFactor stores a real-time factor in an existing report.
func EvaluateRealTimeFactor(report Report, processingDuration time.Duration, audioDuration time.Duration) (Report, error) {
	factor, err := RealTimeFactor(processingDuration, audioDuration)
	if err != nil {
		return Report{}, err
	}
	report.RealTimeFactor = factor
	return report, nil
}

func referenceTranscriptText(turns []ReferenceTurn) string {
	parts := make([]string, 0, len(turns))
	for _, turn := range turns {
		if text := strings.TrimSpace(turn.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

func predictionTranscriptText(turns []meeting.TranscriptTurn) string {
	parts := make([]string, 0, len(turns))
	for _, turn := range turns {
		if text := strings.TrimSpace(turn.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

func actionItemSet(items []meeting.ActionItem) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		text := normalizeComparableText(item.Text)
		if text == "" {
			continue
		}
		set[text] = struct{}{}
	}
	return set
}

func normalizeWords(text string) []string {
	text = normalizeComparableText(text)
	if text == "" {
		return nil
	}
	return strings.Fields(text)
}

func normalizeRunes(text string) []rune {
	text = normalizeComparableText(text)
	if text == "" {
		return nil
	}
	return []rune(text)
}

func normalizeComparableText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	lastSpace := true
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(r)
			lastSpace = false
		case unicode.IsSpace(r):
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		default:
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func normalizeToken(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

func levenshteinStrings(reference []string, prediction []string) int {
	return levenshtein(len(reference), len(prediction), func(i int, j int) bool {
		return reference[i] == prediction[j]
	})
}

func levenshteinRunes(reference []rune, prediction []rune) int {
	return levenshtein(len(reference), len(prediction), func(i int, j int) bool {
		return reference[i] == prediction[j]
	})
}

func levenshtein(referenceLen int, predictionLen int, equal func(i int, j int) bool) int {
	if referenceLen == 0 {
		return predictionLen
	}
	if predictionLen == 0 {
		return referenceLen
	}

	previous := make([]int, predictionLen+1)
	current := make([]int, predictionLen+1)
	for j := 0; j <= predictionLen; j++ {
		previous[j] = j
	}
	for i := 1; i <= referenceLen; i++ {
		current[0] = i
		for j := 1; j <= predictionLen; j++ {
			cost := 1
			if equal(i-1, j-1) {
				cost = 0
			}
			current[j] = minInt(
				previous[j]+1,
				current[j-1]+1,
				previous[j-1]+cost,
			)
		}
		previous, current = current, previous
	}
	return previous[predictionLen]
}

func rate(edits int, referenceLen int, predictionLen int) float64 {
	if referenceLen == 0 {
		if predictionLen == 0 {
			return 0
		}
		return 1
	}
	return float64(edits) / float64(referenceLen)
}

func ratio(numerator int, denominator int) float64 {
	if denominator == 0 {
		return 1
	}
	return float64(numerator) / float64(denominator)
}

func classificationRatio(numerator int, denominator int) float64 {
	if denominator == 0 {
		return 1
	}
	return float64(numerator) / float64(denominator)
}

func f1(precision float64, recall float64) float64 {
	if precision == 0 && recall == 0 {
		return 0
	}
	return 2 * precision * recall / (precision + recall)
}

func minInt(values ...int) int {
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func isZeroSummary(s meeting.MeetingSummary) bool {
	return strings.TrimSpace(s.Overview) == "" &&
		len(s.Topics) == 0 &&
		len(s.Decisions) == 0 &&
		len(s.ActionItems) == 0 &&
		len(s.FollowUps) == 0 &&
		len(s.OpenQuestions) == 0 &&
		len(s.Risks) == 0
}
