package evaluation

import (
	"strings"

	"go.glpx.pro/gdk/voice/meeting"
)

// DiarizationSegmentsFromReferenceTurns adapts reviewed reference transcript
// turns into diarization segments for DER scoring. Turns without a speaker
// ID are excluded.
func DiarizationSegmentsFromReferenceTurns(turns []ReferenceTurn) []DiarizationSegment {
	segments := make([]DiarizationSegment, 0, len(turns))
	for _, turn := range turns {
		if strings.TrimSpace(turn.SpeakerID) == "" {
			continue
		}
		segments = append(segments, DiarizationSegment{
			SpeakerID: turn.SpeakerID,
			Start:     turn.StartTime,
			End:       turn.EndTime,
		})
	}
	return segments
}

// DiarizationSegmentsFromTranscriptTurns adapts predicted meeting transcript
// turns into diarization segments for DER scoring. Turns without a speaker
// ID are excluded.
func DiarizationSegmentsFromTranscriptTurns(turns []meeting.TranscriptTurn) []DiarizationSegment {
	segments := make([]DiarizationSegment, 0, len(turns))
	for _, turn := range turns {
		if strings.TrimSpace(turn.SpeakerID) == "" {
			continue
		}
		segments = append(segments, DiarizationSegment{
			SpeakerID: turn.SpeakerID,
			Start:     turn.StartTime,
			End:       turn.EndTime,
		})
	}
	return segments
}
