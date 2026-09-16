package meeting

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.glpx.pro/voicekit/diarization"
)

var ErrIntegratedResultRequired = errors.New("integrated diarization result is required")

// BuildOptions configures meeting construction from a diarized ASR result.
type BuildOptions struct {
	ID           string
	Title        string
	Source       Source
	Participants []Participant
	StartedAt    time.Time
	EndedAt      time.Time
	CreatedAt    time.Time
	ProcessedAt  time.Time
}

// FromIntegratedResult maps ASR plus diarization output into a meeting artifact.
func FromIntegratedResult(result *diarization.IntegratedResult, options BuildOptions) (*Meeting, error) {
	if result == nil {
		return nil, ErrIntegratedResultRequired
	}

	id := strings.TrimSpace(options.ID)
	if id == "" {
		id = strings.TrimSpace(result.SessionID)
	}
	if id == "" {
		return nil, ErrMeetingIDRequired
	}

	createdAt := options.CreatedAt
	if createdAt.IsZero() {
		createdAt = result.ProcessedAt
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	processedAt := options.ProcessedAt
	if processedAt.IsZero() {
		processedAt = result.ProcessedAt
	}

	participants := normalizeParticipants(options.Participants)
	participantBySpeaker := participantsBySpeakerID(participants)

	meeting := &Meeting{
		ID:           id,
		Title:        strings.TrimSpace(options.Title),
		Language:     strings.TrimSpace(result.Language),
		Source:       normalizeSource(options.Source),
		Participants: participants,
		Transcript:   transcriptTurnsFromSegments(id, result.SpeakerSegments, participantBySpeaker),
		StartedAt:    options.StartedAt,
		EndedAt:      options.EndedAt,
		CreatedAt:    createdAt,
		ProcessedAt:  processedAt,
	}

	if err := meeting.Validate(); err != nil {
		return nil, err
	}
	return meeting, nil
}

func transcriptTurnsFromSegments(
	meetingID string,
	segments []diarization.SpeakerTextSegment,
	participants map[string]Participant,
) []TranscriptTurn {
	turns := make([]TranscriptTurn, 0, len(segments))
	for index, segment := range segments {
		text := strings.TrimSpace(segment.Text)
		if text == "" && len(segment.Words) == 0 {
			continue
		}

		turn := TranscriptTurn{
			ID:         fmt.Sprintf("%s-turn-%03d", meetingID, index+1),
			SpeakerID:  strings.TrimSpace(segment.SpeakerID),
			Text:       text,
			StartTime:  secondsToDuration(segment.StartTime),
			EndTime:    secondsToDuration(segment.EndTime),
			Confidence: segment.Confidence,
			Words:      wordsFromSegment(segment.Words),
		}

		if participant, ok := participants[turn.SpeakerID]; ok {
			turn.ParticipantID = participant.ID
			turn.SpeakerLabel = participant.DisplayName
		}
		if turn.SpeakerLabel == "" {
			turn.SpeakerLabel = turn.SpeakerID
		}

		turns = append(turns, turn)
	}
	return turns
}

func wordsFromSegment(words []diarization.RecognitionWordInfo) []Word {
	if len(words) == 0 {
		return nil
	}
	mapped := make([]Word, 0, len(words))
	for _, word := range words {
		mapped = append(mapped, Word{
			Text:       strings.TrimSpace(word.Text),
			StartTime:  secondsToDuration(word.StartTime),
			EndTime:    secondsToDuration(word.EndTime),
			Confidence: word.Confidence,
		})
	}
	return mapped
}

func normalizeParticipants(participants []Participant) []Participant {
	if len(participants) == 0 {
		return nil
	}
	normalized := make([]Participant, 0, len(participants))
	for index, participant := range participants {
		participant.ID = strings.TrimSpace(participant.ID)
		participant.DisplayName = strings.TrimSpace(participant.DisplayName)
		participant.Role = strings.TrimSpace(participant.Role)
		participant.SpeakerID = strings.TrimSpace(participant.SpeakerID)

		if participant.ID == "" && participant.SpeakerID != "" {
			participant.ID = participant.SpeakerID
		}
		if participant.ID == "" {
			participant.ID = fmt.Sprintf("participant-%03d", index+1)
		}
		if participant.DisplayName == "" {
			participant.DisplayName = participant.ID
		}

		normalized = append(normalized, participant)
	}
	return normalized
}

func participantsBySpeakerID(participants []Participant) map[string]Participant {
	bySpeakerID := make(map[string]Participant, len(participants))
	for _, participant := range participants {
		if participant.SpeakerID == "" {
			continue
		}
		bySpeakerID[participant.SpeakerID] = participant
	}
	return bySpeakerID
}

func normalizeSource(source Source) Source {
	source.Provider = strings.TrimSpace(source.Provider)
	source.Reference = strings.TrimSpace(source.Reference)
	if source.Kind == "" {
		source.Kind = SourceUnknown
	}
	return source
}

func secondsToDuration(seconds float64) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}
