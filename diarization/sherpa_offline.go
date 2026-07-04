package diarization

import (
	"context"
	"fmt"
	"sync"
	"time"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

type sherpaOfflineBackend struct {
	config   *DiarizationConfig
	diarizer offlineSpeakerDiarizer
	mu       sync.Mutex
	closed   bool
}

type offlineSpeakerDiarizer interface {
	SampleRate() int
	Process(samples []float32) ([]sherpa.OfflineSpeakerDiarizationSegment, error)
	Close() error
}

func newSherpaOfflineBackend(config *DiarizationConfig) (*sherpaOfflineBackend, error) {
	nativeConfig := buildOfflineSpeakerDiarizationConfig(config)
	diarizer := sherpa.NewOfflineSpeakerDiarization(nativeConfig)
	if diarizer == nil {
		return nil, fmt.Errorf("failed to create Sherpa offline speaker diarization")
	}
	return &sherpaOfflineBackend{
		config:   config,
		diarizer: &nativeOfflineSpeakerDiarizer{diarizer: diarizer},
	}, nil
}

func buildOfflineSpeakerDiarizationConfig(config *DiarizationConfig) *sherpa.OfflineSpeakerDiarizationConfig {
	return &sherpa.OfflineSpeakerDiarizationConfig{
		Segmentation: sherpa.OfflineSpeakerSegmentationModelConfig{
			Pyannote: sherpa.OfflineSpeakerSegmentationPyannoteModelConfig{
				Model: config.SegmentationModelPath,
			},
			NumThreads: config.NumThreads,
			Debug:      boolToInt(config.Debug),
			Provider:   config.Provider,
		},
		Embedding: sherpa.SpeakerEmbeddingExtractorConfig{
			Model:      config.EmbeddingModelPath,
			NumThreads: config.NumThreads,
			Debug:      boolToInt(config.Debug),
			Provider:   config.Provider,
		},
		Clustering: sherpa.FastClusteringConfig{
			NumClusters: config.NumClusters,
			Threshold:   config.ClusteringThreshold,
		},
		MinDurationOn:  config.MinDurationOn,
		MinDurationOff: config.MinDurationOff,
	}
}

func (b *sherpaOfflineBackend) Process(ctx context.Context, request Request) (*DiarizationResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if len(request.Audio) == 0 {
		return nil, fmt.Errorf("audio cannot be empty")
	}
	if request.SampleRate <= 0 {
		return nil, fmt.Errorf("sampleRate must be positive, got %d", request.SampleRate)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.diarizer == nil {
		return nil, fmt.Errorf("sherpa offline diarization backend is closed")
	}
	expectedRate := b.diarizer.SampleRate()
	if expectedRate > 0 && request.SampleRate != expectedRate {
		return nil, fmt.Errorf("sample rate mismatch: diarization backend expects %d Hz, got %d Hz", expectedRate, request.SampleRate)
	}

	segments, err := b.diarizer.Process(request.Audio)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	processedAt := request.ProcessedAt
	if processedAt.IsZero() {
		processedAt = time.Now()
	}
	return mapSherpaDiarizationResult(request, segments, processedAt), nil
}

func (b *sherpaOfflineBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	if b.diarizer != nil {
		if err := b.diarizer.Close(); err != nil {
			return err
		}
		b.diarizer = nil
	}
	b.closed = true
	return nil
}

func mapSherpaDiarizationResult(request Request, segments []sherpa.OfflineSpeakerDiarizationSegment, processedAt time.Time) *DiarizationResult {
	mapped := make([]SpeakerSegment, 0, len(segments))
	for _, segment := range segments {
		start := float64(segment.Start)
		end := float64(segment.End)
		mapped = append(mapped, SpeakerSegment{
			SpeakerID:  fmt.Sprintf("speaker_%d", segment.Speaker),
			StartTime:  start,
			EndTime:    end,
			Duration:   end - start,
			Confidence: 0,
		})
	}

	totalDuration := float64(len(request.Audio)) / float64(request.SampleRate)
	for _, segment := range mapped {
		if segment.EndTime > totalDuration {
			totalDuration = segment.EndTime
		}
	}

	return &DiarizationResult{
		SessionID:     request.SessionID,
		Segments:      mapped,
		SpeakerCount:  countUniqueSpeakers(mapped),
		TotalDuration: totalDuration,
		ProcessedAt:   processedAt,
	}
}

type nativeOfflineSpeakerDiarizer struct {
	diarizer *sherpa.OfflineSpeakerDiarization
}

func (d *nativeOfflineSpeakerDiarizer) SampleRate() int {
	if d == nil || d.diarizer == nil {
		return 0
	}
	return d.diarizer.SampleRate()
}

func (d *nativeOfflineSpeakerDiarizer) Process(samples []float32) ([]sherpa.OfflineSpeakerDiarizationSegment, error) {
	if d == nil || d.diarizer == nil {
		return nil, fmt.Errorf("sherpa offline diarizer is closed")
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("audio cannot be empty")
	}
	return d.diarizer.Process(samples), nil
}

func (d *nativeOfflineSpeakerDiarizer) Close() error {
	if d == nil || d.diarizer == nil {
		return nil
	}
	sherpa.DeleteOfflineSpeakerDiarization(d.diarizer)
	d.diarizer = nil
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
