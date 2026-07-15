package evaluation

import (
	"sort"
	"time"
)

// DiarizationSegment is a speaker-attributed time interval used for
// diarization error rate scoring.
type DiarizationSegment struct {
	SpeakerID string
	Start     time.Duration
	End       time.Duration
}

// DiarizationOptions configures diarization error rate scoring.
type DiarizationOptions struct {
	// Collar excludes a no-score window of this duration on each side of
	// every reference-segment boundary, following NIST md-eval convention
	// (commonly 0.25s). Zero disables collar exclusion.
	Collar time.Duration
	// SkipOverlap excludes intervals where more than one reference speaker
	// is simultaneously active from scoring.
	SkipOverlap bool
}

// DiarizationReport summarizes NIST md-eval / pyannote.metrics-style
// diarization error rate (DER) results.
type DiarizationReport struct {
	ErrorRate      float64 `json:"error_rate"`
	MissedRate     float64 `json:"missed_rate"`
	FalseAlarmRate float64 `json:"false_alarm_rate"`
	ConfusionRate  float64 `json:"confusion_rate"`

	MissedDuration         time.Duration `json:"missed_duration"`
	FalseAlarmDuration     time.Duration `json:"false_alarm_duration"`
	ConfusionDuration      time.Duration `json:"confusion_duration"`
	TotalReferenceDuration time.Duration `json:"total_reference_duration"`
}

// EvaluateDiarization scores hypothesis diarization segments against
// reference segments:
//
//	DER = (missed + falseAlarm + confusion) / totalReferenceSpeakerTime
//
// An optimal 1:1 hypothesis-to-reference speaker mapping (maximizing total
// overlap) is computed once, before scoring, using the Hungarian algorithm.
// The recording is swept as a sequence of elementary intervals between
// segment boundaries; each interval contributes missed, false-alarm, and
// confusion time based on the count of active reference/hypothesis speakers
// and whether the mapped counterpart of each active reference speaker is
// also active.
func EvaluateDiarization(reference []DiarizationSegment, hypothesis []DiarizationSegment, opts DiarizationOptions) (DiarizationReport, error) {
	if len(reference) == 0 {
		return DiarizationReport{}, ErrReferenceRequired
	}

	refToHyp := invertSpeakerMapping(optimalSpeakerMapping(speakerOverlap(reference, hypothesis)))
	refBoundaries := segmentBoundaries(reference)
	boundaries := elementaryBoundaries(reference, hypothesis, opts.Collar)

	var missed, falseAlarm, confusion, totalRef time.Duration
	for i := 0; i+1 < len(boundaries); i++ {
		start, end := boundaries[i], boundaries[i+1]
		if end <= start {
			continue
		}
		if opts.Collar > 0 && withinCollar(start, end, refBoundaries, opts.Collar) {
			continue
		}

		refSpeakers := activeSpeakers(reference, start, end)
		if opts.SkipOverlap && len(refSpeakers) > 1 {
			continue
		}
		hypSpeakers := activeSpeakers(hypothesis, start, end)

		nRef := len(refSpeakers)
		nSys := len(hypSpeakers)
		nCorrect := 0
		for speaker := range refSpeakers {
			mappedHyp, mapped := refToHyp[speaker]
			if !mapped {
				continue
			}
			if _, active := hypSpeakers[mappedHyp]; active {
				nCorrect++
			}
		}

		d := end - start
		if nRef > nSys {
			missed += d * time.Duration(nRef-nSys)
		}
		if nSys > nRef {
			falseAlarm += d * time.Duration(nSys-nRef)
		}
		minRefSys := nRef
		if nSys < minRefSys {
			minRefSys = nSys
		}
		confusion += d * time.Duration(minRefSys-nCorrect)
		totalRef += d * time.Duration(nRef)
	}

	report := DiarizationReport{
		MissedDuration:         missed,
		FalseAlarmDuration:     falseAlarm,
		ConfusionDuration:      confusion,
		TotalReferenceDuration: totalRef,
	}
	if totalRef > 0 {
		report.ErrorRate = float64(missed+falseAlarm+confusion) / float64(totalRef)
		report.MissedRate = float64(missed) / float64(totalRef)
		report.FalseAlarmRate = float64(falseAlarm) / float64(totalRef)
		report.ConfusionRate = float64(confusion) / float64(totalRef)
	}
	return report, nil
}

// speakerOverlap sums, for every (hypothesis speaker, reference speaker)
// pair, the total time both were simultaneously active across all of their
// segments.
func speakerOverlap(reference []DiarizationSegment, hypothesis []DiarizationSegment) map[string]map[string]float64 {
	overlap := make(map[string]map[string]float64)
	for _, h := range hypothesis {
		for _, r := range reference {
			start := maxDuration(h.Start, r.Start)
			end := minDuration(h.End, r.End)
			if end <= start {
				continue
			}
			if overlap[h.SpeakerID] == nil {
				overlap[h.SpeakerID] = make(map[string]float64)
			}
			overlap[h.SpeakerID][r.SpeakerID] += float64(end - start)
		}
	}
	return overlap
}

// invertSpeakerMapping flips a hypothesis-to-reference speaker mapping into
// a reference-to-hypothesis mapping.
func invertSpeakerMapping(hypToRef map[string]string) map[string]string {
	refToHyp := make(map[string]string, len(hypToRef))
	for hyp, ref := range hypToRef {
		refToHyp[ref] = hyp
	}
	return refToHyp
}

// segmentBoundaries returns the distinct start/end points of segments, used
// as the centers of no-score collar windows.
func segmentBoundaries(segments []DiarizationSegment) []time.Duration {
	seen := make(map[time.Duration]struct{}, len(segments)*2)
	boundaries := make([]time.Duration, 0, len(segments)*2)
	add := func(d time.Duration) {
		if _, ok := seen[d]; ok {
			return
		}
		seen[d] = struct{}{}
		boundaries = append(boundaries, d)
	}
	for _, seg := range segments {
		add(seg.Start)
		add(seg.End)
	}
	return boundaries
}

// elementaryBoundaries returns the sorted, deduplicated sweep points for the
// elementary-interval scan: every reference and hypothesis segment
// boundary, plus (when collar is positive) the collar edges around every
// reference-segment boundary so no elementary interval straddles a
// collar/no-collar transition.
func elementaryBoundaries(reference []DiarizationSegment, hypothesis []DiarizationSegment, collar time.Duration) []time.Duration {
	seen := make(map[time.Duration]struct{})
	boundaries := make([]time.Duration, 0)
	add := func(d time.Duration) {
		if d < 0 {
			d = 0
		}
		if _, ok := seen[d]; ok {
			return
		}
		seen[d] = struct{}{}
		boundaries = append(boundaries, d)
	}

	for _, seg := range reference {
		add(seg.Start)
		add(seg.End)
		if collar > 0 {
			add(seg.Start - collar)
			add(seg.Start + collar)
			add(seg.End - collar)
			add(seg.End + collar)
		}
	}
	for _, seg := range hypothesis {
		add(seg.Start)
		add(seg.End)
	}

	sort.Slice(boundaries, func(i, j int) bool { return boundaries[i] < boundaries[j] })
	return boundaries
}

// withinCollar reports whether the elementary interval [start, end) lies
// entirely inside the no-score collar window around any reference-segment
// boundary. Because collar edges are always sweep boundaries (see
// elementaryBoundaries), an interval is guaranteed to be either fully
// inside or fully outside each window, never straddling it.
func withinCollar(start time.Duration, end time.Duration, boundaries []time.Duration, collar time.Duration) bool {
	for _, b := range boundaries {
		if start >= b-collar && end <= b+collar {
			return true
		}
	}
	return false
}

// activeSpeakers returns the set of distinct speaker IDs whose segment
// covers the whole elementary interval [start, end).
func activeSpeakers(segments []DiarizationSegment, start time.Duration, end time.Duration) map[string]struct{} {
	active := make(map[string]struct{})
	for _, seg := range segments {
		if seg.Start < seg.End && seg.Start <= start && seg.End >= end {
			active[seg.SpeakerID] = struct{}{}
		}
	}
	return active
}

func maxDuration(a time.Duration, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func minDuration(a time.Duration, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
