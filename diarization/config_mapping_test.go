package diarization

import "testing"

// TestBuildOfflineSpeakerDiarizationConfigMapsFields proves the sherpa offline
// diarization backend receives each configuration value on the correct native
// field. It is hermetic: it builds the native config struct only, without
// constructing a native diarizer, so no model weights are required.
func TestBuildOfflineSpeakerDiarizationConfigMapsFields(t *testing.T) {
	cfg := &DiarizationConfig{
		SegmentationModelPath: "seg.onnx",
		EmbeddingModelPath:    "emb.onnx",
		Provider:              "cpu",
		NumThreads:            3,
		Debug:                 true,
		ClusteringThreshold:   0.65,
		NumClusters:           4,
		MinDurationOn:         0.35,
		MinDurationOff:        0.55,
	}

	got := buildOfflineSpeakerDiarizationConfig(cfg)

	if got.Segmentation.Pyannote.Model != "seg.onnx" {
		t.Errorf("Segmentation.Pyannote.Model = %q, want seg.onnx", got.Segmentation.Pyannote.Model)
	}
	if got.Segmentation.NumThreads != 3 || got.Segmentation.Provider != "cpu" || got.Segmentation.Debug != 1 {
		t.Errorf("segmentation runtime fields not mapped: %+v", got.Segmentation)
	}
	if got.Embedding.Model != "emb.onnx" {
		t.Errorf("Embedding.Model = %q, want emb.onnx", got.Embedding.Model)
	}
	if got.Embedding.NumThreads != 3 || got.Embedding.Provider != "cpu" || got.Embedding.Debug != 1 {
		t.Errorf("embedding runtime fields not mapped: %+v", got.Embedding)
	}
	if got.Clustering.NumClusters != 4 || got.Clustering.Threshold != 0.65 {
		t.Errorf("clustering fields not mapped: %+v", got.Clustering)
	}
	if got.MinDurationOn != 0.35 || got.MinDurationOff != 0.55 {
		t.Errorf("min-duration fields not mapped: on=%v off=%v", got.MinDurationOn, got.MinDurationOff)
	}
}
