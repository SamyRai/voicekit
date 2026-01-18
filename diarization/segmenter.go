package diarization

import (
	"math"
	"sort"
)

// Segmenter handles audio segmentation for diarization
type Segmenter struct {
	config *DiarizationConfig
}

// NewSegmenter creates a new audio segmenter
func NewSegmenter(config *DiarizationConfig) *Segmenter {
	if config == nil {
		config = DefaultDiarizationConfig()
	}
	return &Segmenter{config: config}
}

// SegmentByVAD segments audio using Voice Activity Detection results
func (s *Segmenter) SegmentByVAD(audioData []float32, sampleRate int, vadResults []VADResult) ([]AudioSegment, error) {
	if len(vadResults) == 0 {
		return []AudioSegment{}, nil
	}

	segments := []AudioSegment{}

	for _, vadResult := range vadResults {
		if vadResult.IsSpeech {
			segment := AudioSegment{
				StartTime: vadResult.StartTime,
				EndTime:   vadResult.EndTime,
			}

			// Extract audio samples for this segment
			startSample := int(vadResult.StartTime * float64(sampleRate))
			endSample := int(vadResult.EndTime * float64(sampleRate))

			if startSample < 0 {
				startSample = 0
			}
			if endSample > len(audioData) {
				endSample = len(audioData)
			}

			if startSample < endSample {
				segment.Samples = make([]float32, endSample-startSample)
				copy(segment.Samples, audioData[startSample:endSample])
				segments = append(segments, segment)
			}
		}
	}

	// Merge overlapping or adjacent segments
	segments = s.mergeAdjacentSegments(segments)

	// Filter segments by minimum length
	segments = s.filterSegmentsByLength(segments)

	if s.config.Logger != nil {
		s.config.Logger.Infof("Segmented audio into %d speech segments", len(segments))
	}
	return segments, nil
	return segments, nil
}

// SegmentBySilence segments audio based on silence detection
func (s *Segmenter) SegmentBySilence(audioData []float32, sampleRate int) ([]AudioSegment, error) {
	// Simple energy-based silence detection
	segments := []AudioSegment{}

	frameSize := sampleRate / 100 // 10ms frames
	minSegmentFrames := int(s.config.MinSegmentLength * float64(sampleRate) / float64(frameSize))
	silenceFrames := int(s.config.SilenceThreshold * float64(sampleRate) / float64(frameSize))

	currentSegment := AudioSegment{
		StartTime: -1,
		Samples:   []float32{},
	}
	consecutiveSilence := 0

	for i := 0; i < len(audioData); i += frameSize {
		endIndex := i + frameSize
		if endIndex > len(audioData) {
			endIndex = len(audioData)
		}

		frame := audioData[i:endIndex]
		currentTime := float64(i) / float64(sampleRate)

		// Calculate RMS energy for the frame
		energy := s.calculateRMSEnergy(frame)

		// Check if this is silence
		isSilence := energy < 0.01 // Threshold for silence

		if !isSilence {
			// Speech detected
			if currentSegment.StartTime < 0 {
				// Start new segment
				currentSegment.StartTime = currentTime
			}
			currentSegment.EndTime = currentTime + float64(len(frame))/float64(sampleRate)
			currentSegment.Samples = append(currentSegment.Samples, frame...)
			consecutiveSilence = 0
		} else {
			// Silence detected
			consecutiveSilence++

			// If we have a current segment and enough silence, end it
			if currentSegment.StartTime >= 0 && consecutiveSilence >= silenceFrames {
				if len(currentSegment.Samples) >= minSegmentFrames*frameSize {
					segments = append(segments, currentSegment)
				}

				// Reset for next segment
				currentSegment = AudioSegment{
					StartTime: -1,
					Samples:   []float32{},
				}
				consecutiveSilence = 0
			}
		}
	}

	// Add final segment if it exists
	if currentSegment.StartTime >= 0 && len(currentSegment.Samples) >= minSegmentFrames*frameSize {
		segments = append(segments, currentSegment)
	}

	if s.config.Logger != nil {
		s.config.Logger.Infof("Segmented audio by silence into %d segments", len(segments))
	}
	return segments, nil
}

// SegmentByFixedLength segments audio into fixed-length chunks
func (s *Segmenter) SegmentByFixedLength(audioData []float32, sampleRate int, chunkDuration float64) ([]AudioSegment, error) {
	segments := []AudioSegment{}

	chunkSamples := int(chunkDuration * float64(sampleRate))
	overlapSamples := int(0.1 * float64(chunkSamples)) // 10% overlap

	for startSample := 0; startSample < len(audioData); startSample += chunkSamples - overlapSamples {
		endSample := startSample + chunkSamples
		if endSample > len(audioData) {
			endSample = len(audioData)
		}

		if endSample-startSample < chunkSamples/4 {
			// Skip very short final segments
			break
		}

		segment := AudioSegment{
			StartTime: float64(startSample) / float64(sampleRate),
			EndTime:   float64(endSample) / float64(sampleRate),
			Samples:   make([]float32, endSample-startSample),
		}
		copy(segment.Samples, audioData[startSample:endSample])

		segments = append(segments, segment)
	}

	if s.config.Logger != nil {
		s.config.Logger.Infof("Segmented audio into %d fixed-length chunks", len(segments))
	}
	return segments, nil
}

// mergeAdjacentSegments merges overlapping or adjacent segments
func (s *Segmenter) mergeAdjacentSegments(segments []AudioSegment) []AudioSegment {
	if len(segments) <= 1 {
		return segments
	}

	// Sort segments by start time
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartTime < segments[j].StartTime
	})

	merged := []AudioSegment{segments[0]}

	for _, segment := range segments[1:] {
		last := &merged[len(merged)-1]

		// Check if segments overlap or are very close
		if segment.StartTime <= last.EndTime+s.config.OverlapThreshold {
			// Merge segments
			last.EndTime = math.Max(last.EndTime, segment.EndTime)
			last.Samples = append(last.Samples, segment.Samples...)
		} else {
			// Add as separate segment
			merged = append(merged, segment)
		}
	}

	return merged
}

// filterSegmentsByLength filters out segments that are too short or too long
func (s *Segmenter) filterSegmentsByLength(segments []AudioSegment) []AudioSegment {
	filtered := []AudioSegment{}

	for _, segment := range segments {
		duration := segment.EndTime - segment.StartTime

		if duration >= s.config.MinSegmentLength && duration <= s.config.MaxSegmentLength {
			filtered = append(filtered, segment)
		}
	}

	return filtered
}

// calculateRMSEnergy calculates RMS energy of an audio frame
func (s *Segmenter) calculateRMSEnergy(frame []float32) float64 {
	if len(frame) == 0 {
		return 0
	}

	var sum float64
	for _, sample := range frame {
		sum += float64(sample * sample)
	}

	return math.Sqrt(sum / float64(len(frame)))
}

// VADResult represents Voice Activity Detection result
type VADResult struct {
	StartTime  float64 `json:"start_time"`
	EndTime    float64 `json:"end_time"`
	IsSpeech   bool    `json:"is_speech"`
	Confidence float32 `json:"confidence"`
}

// DetectSpeechSegments uses VAD to detect speech segments
func (s *Segmenter) DetectSpeechSegments(audioData []float32, sampleRate int) ([]VADResult, error) {
	// This is a placeholder implementation
	// In a real system, this would integrate with actual VAD like Silero VAD

	results := []VADResult{}

	// Simple energy-based VAD simulation
	frameSize := sampleRate / 100 // 10ms frames
	minSpeechFrames := 5          // Minimum 5 frames (50ms) of speech

	currentSpeechStart := -1

	for i := 0; i < len(audioData); i += frameSize {
		endIndex := i + frameSize
		if endIndex > len(audioData) {
			endIndex = len(audioData)
		}

		frame := audioData[i:endIndex]
		currentTime := float64(i) / float64(sampleRate)

		energy := s.calculateRMSEnergy(frame)
		isSpeech := energy > 0.02 // Simple threshold

		if isSpeech {
			if currentSpeechStart < 0 {
				currentSpeechStart = i
			}
		} else {
			if currentSpeechStart >= 0 {
				speechFrames := (i - currentSpeechStart) / frameSize
				if speechFrames >= minSpeechFrames {
					results = append(results, VADResult{
						StartTime:  float64(currentSpeechStart) / float64(sampleRate),
						EndTime:    currentTime,
						IsSpeech:   true,
						Confidence: 0.8,
					})
				}
				currentSpeechStart = -1
			}
		}
	}

	// Handle final speech segment
	if currentSpeechStart >= 0 {
		speechFrames := (len(audioData) - currentSpeechStart) / frameSize
		if speechFrames >= minSpeechFrames {
			results = append(results, VADResult{
				StartTime:  float64(currentSpeechStart) / float64(sampleRate),
				EndTime:    float64(len(audioData)) / float64(sampleRate),
				IsSpeech:   true,
				Confidence: 0.8,
			})
		}
	}

	return results, nil
}