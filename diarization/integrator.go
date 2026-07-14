package diarization

import (
	"fmt"
	"time"
)

// Integrator integrates diarization results with ASR recognition results
type Integrator struct {
	config  *DiarizationConfig
	manager *Manager
}

// NewIntegrator creates a new diarization integrator
func NewIntegrator(config *DiarizationConfig, manager *Manager) *Integrator {
	if config == nil {
		config = DefaultDiarizationConfig()
	}
	return &Integrator{
		config:  config,
		manager: manager,
	}
}

// IntegratedResult represents the integrated result of ASR and diarization
type IntegratedResult struct {
	SessionID         string               `json:"session_id"`
	FullText          string               `json:"full_text"`
	Language          string               `json:"language,omitempty"`
	Confidence        float32              `json:"confidence"`
	DiarizationResult *DiarizationResult   `json:"diarization"`
	SpeakerSegments   []SpeakerTextSegment `json:"speaker_segments"`
	Translations      map[string]string    `json:"translations,omitempty"`
	ProcessedAt       time.Time            `json:"processed_at"`
}

// SpeakerTextSegment represents a segment of text attributed to a specific speaker
type SpeakerTextSegment struct {
	SpeakerID  string                `json:"speaker_id"`
	StartTime  float64               `json:"start_time"`
	EndTime    float64               `json:"end_time"`
	Duration   float64               `json:"duration"`
	Text       string                `json:"text"`
	Confidence float32               `json:"confidence"`
	Words      []RecognitionWordInfo `json:"words,omitempty"`
}

// RecognitionWordInfo represents word-level information from ASR.
type RecognitionWordInfo struct {
	Text       string  `json:"text"`
	StartTime  float64 `json:"start_time"`
	EndTime    float64 `json:"end_time"`
	Confidence float32 `json:"confidence"`
}

// RecognitionResult represents typed ASR recognition data for diarization integration.
type RecognitionResult struct {
	SessionID      string                `json:"session_id"`
	Text           string                `json:"text"`
	Language       string                `json:"language,omitempty"`
	LanguageProb   float32               `json:"language_prob,omitempty"`
	Confidence     float32               `json:"confidence"`
	Words          []RecognitionWordInfo `json:"words,omitempty"`
	Timestamp      time.Time             `json:"timestamp"`
	Duration       float64               `json:"duration"`
	ProcessingTime time.Duration         `json:"processing_time"`
	Translations   map[string]string     `json:"translations,omitempty"`
}

type recognitionResult = RecognitionResult
type recognitionWordInfo = RecognitionWordInfo

// IntegrateRecognition integrates typed recognition results with diarization results.
func (i *Integrator) IntegrateRecognition(
	result RecognitionResult,
	diarizationResult *DiarizationResult,
) (*IntegratedResult, error) {
	integrated := &IntegratedResult{
		SessionID:         result.SessionID,
		FullText:          result.Text,
		Language:          result.Language,
		Confidence:        result.Confidence,
		DiarizationResult: diarizationResult,
		SpeakerSegments:   []SpeakerTextSegment{},
		ProcessedAt:       time.Now(),
	}

	if diarizationResult == nil || len(diarizationResult.Segments) == 0 {
		if i.config.Logger != nil {
			i.config.Logger.Warnf("No diarization results available for session %s", result.SessionID)
		}
		return integrated, nil
	}

	// Align recognition text with speaker segments
	speakerSegments, err := i.alignTextWithSpeakers(&result, diarizationResult)
	if err != nil {
		return nil, fmt.Errorf("failed to align text with speakers: %v", err)
	}

	integrated.SpeakerSegments = speakerSegments

	if i.config.Logger != nil {
		i.config.Logger.Infof("Integrated recognition with diarization for session %s: %d speaker segments",
			result.SessionID, len(speakerSegments))
	}

	return integrated, nil
}

// alignTextWithSpeakers aligns recognized text with speaker segments
func (i *Integrator) alignTextWithSpeakers(
	recognitionResult *recognitionResult,
	diarizationResult *DiarizationResult,
) ([]SpeakerTextSegment, error) {
	// If we have word-level timestamps, use them for precise alignment
	if len(recognitionResult.Words) > 0 {
		return i.alignWithWordTimestamps(recognitionResult, diarizationResult), nil
	}

	// Fallback to time-based proportional alignment
	return i.alignWithTimeProportions(recognitionResult, diarizationResult), nil
}

// alignWithWordTimestamps aligns using word-level timestamps
func (i *Integrator) alignWithWordTimestamps(
	result *recognitionResult,
	diarizationResult *DiarizationResult,
) []SpeakerTextSegment {
	segments := []SpeakerTextSegment{}

	for _, speakerSegment := range diarizationResult.Segments {
		segment := SpeakerTextSegment{
			SpeakerID:  speakerSegment.SpeakerID,
			StartTime:  speakerSegment.StartTime,
			EndTime:    speakerSegment.EndTime,
			Duration:   speakerSegment.Duration,
			Confidence: speakerSegment.Confidence,
			Words:      []recognitionWordInfo{},
		}

		// Find words that fall within this speaker segment
		for _, word := range result.Words {
			if word.StartTime >= speakerSegment.StartTime && word.EndTime <= speakerSegment.EndTime {
				segment.Words = append(segment.Words, word)
				segment.Text += word.Text + " "
			}
		}

		// Trim trailing space
		if len(segment.Text) > 0 {
			segment.Text = segment.Text[:len(segment.Text)-1]
		}

		// Calculate segment confidence based on word confidences
		if len(segment.Words) > 0 {
			totalConfidence := float32(0)
			for _, word := range segment.Words {
				totalConfidence += word.Confidence
			}
			segment.Confidence = totalConfidence / float32(len(segment.Words))
		}

		if len(segment.Words) > 0 {
			segments = append(segments, segment)
		}
	}

	return segments
}

// alignWithTimeProportions aligns using time proportions when word timestamps aren't available
func (i *Integrator) alignWithTimeProportions(
	result *recognitionResult,
	diarizationResult *DiarizationResult,
) []SpeakerTextSegment {
	segments := []SpeakerTextSegment{}
	words := result.Words

	// If no words available, split text proportionally
	if len(words) == 0 {
		totalTextLength := len(result.Text)
		totalAudioDuration := diarizationResult.TotalDuration

		for _, speakerSegment := range diarizationResult.Segments {
			proportion := speakerSegment.Duration / totalAudioDuration
			_ = int(float64(totalTextLength) * proportion) // Text splitting would need more sophisticated implementation

			segment := SpeakerTextSegment{
				SpeakerID:  speakerSegment.SpeakerID,
				StartTime:  speakerSegment.StartTime,
				EndTime:    speakerSegment.EndTime,
				Duration:   speakerSegment.Duration,
				Confidence: speakerSegment.Confidence,
				Text:       "", // Would need more sophisticated text splitting
			}

			segments = append(segments, segment)
		}

		return segments
	}

	// Align words with speaker segments
	wordIndex := 0
	for _, speakerSegment := range diarizationResult.Segments {
		segment := SpeakerTextSegment{
			SpeakerID:  speakerSegment.SpeakerID,
			StartTime:  speakerSegment.StartTime,
			EndTime:    speakerSegment.EndTime,
			Duration:   speakerSegment.Duration,
			Confidence: speakerSegment.Confidence,
			Words:      []recognitionWordInfo{},
		}

		// Collect words that belong to this speaker segment
		for wordIndex < len(words) {
			word := words[wordIndex]

			if word.StartTime >= speakerSegment.StartTime && word.StartTime < speakerSegment.EndTime {
				segment.Words = append(segment.Words, word)
				segment.Text += word.Text + " "
				wordIndex++
			} else if word.StartTime >= speakerSegment.EndTime {
				break
			} else {
				wordIndex++
			}
		}

		// Trim trailing space
		if len(segment.Text) > 0 {
			segment.Text = segment.Text[:len(segment.Text)-1]
		}

		// Calculate average confidence
		if len(segment.Words) > 0 {
			totalConfidence := float32(0)
			for _, word := range segment.Words {
				totalConfidence += word.Confidence
			}
			segment.Confidence = totalConfidence / float32(len(segment.Words))
		}

		segments = append(segments, segment)
	}

	return segments
}

// GetSpeakerSummary returns a summary of speakers and their speaking time
func (i *Integrator) GetSpeakerSummary(result *IntegratedResult) map[string]SpeakerSummary {
	summary := make(map[string]SpeakerSummary)

	if result == nil {
		return summary
	}

	for _, segment := range result.SpeakerSegments {
		if _, exists := summary[segment.SpeakerID]; !exists {
			summary[segment.SpeakerID] = SpeakerSummary{
				SpeakerID:    segment.SpeakerID,
				TotalTime:    0,
				SegmentCount: 0,
				WordCount:    0,
				Text:         "",
			}
		}

		s := summary[segment.SpeakerID]
		s.TotalTime += segment.Duration
		s.SegmentCount++
		s.WordCount += len(segment.Words)
		s.Text += segment.Text + " "
		summary[segment.SpeakerID] = s
	}

	// Trim trailing spaces
	for speakerID, s := range summary {
		if len(s.Text) > 0 {
			s.Text = s.Text[:len(s.Text)-1]
		}
		summary[speakerID] = s
	}

	return summary
}

// SpeakerSummary provides a summary of a speaker's contribution
type SpeakerSummary struct {
	SpeakerID    string  `json:"speaker_id"`
	TotalTime    float64 `json:"total_time"`    // Total speaking time in seconds
	SegmentCount int     `json:"segment_count"` // Number of speaking segments
	WordCount    int     `json:"word_count"`    // Number of words spoken
	Text         string  `json:"text"`          // All text spoken by this speaker
}

// ExportToWebSocketMessage converts integrated result to WebSocket message format
func (i *Integrator) ExportToWebSocketMessage(result *IntegratedResult) map[string]any {
	if result == nil {
		return nil
	}

	message := map[string]any{
		"type":         "diarization_result",
		"session_id":   result.SessionID,
		"full_text":    result.FullText,
		"confidence":   result.Confidence,
		"processed_at": result.ProcessedAt.UnixMilli(),
	}

	if result.Language != "" {
		message["language"] = result.Language
	}

	if result.DiarizationResult != nil {
		message["speaker_count"] = result.DiarizationResult.SpeakerCount
		message["total_duration"] = result.DiarizationResult.TotalDuration
	}

	if len(result.SpeakerSegments) > 0 {
		segments := make([]map[string]any, len(result.SpeakerSegments))
		for i, segment := range result.SpeakerSegments {
			segments[i] = map[string]any{
				"speaker_id": segment.SpeakerID,
				"start_time": segment.StartTime,
				"end_time":   segment.EndTime,
				"duration":   segment.Duration,
				"text":       segment.Text,
				"confidence": segment.Confidence,
			}
			if len(segment.Words) > 0 {
				segments[i]["words"] = segment.Words
			}
		}
		message["speaker_segments"] = segments
	}

	if len(result.Translations) > 0 {
		message["translations"] = result.Translations
	}

	return message
}
