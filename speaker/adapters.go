package speaker

import (
	"github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// SherpaExtractorAdapter adapts sherpa_onnx.SpeakerEmbeddingExtractor to SpeakerEmbeddingExtractor interface
type SherpaExtractorAdapter struct {
	extractor *sherpa_onnx.SpeakerEmbeddingExtractor
}

func NewSherpaExtractorAdapter(config *sherpa_onnx.SpeakerEmbeddingExtractorConfig) *SherpaExtractorAdapter {
	extractor := sherpa_onnx.NewSpeakerEmbeddingExtractor(config)
	if extractor == nil {
		return nil
	}
	return &SherpaExtractorAdapter{extractor: extractor}
}

func (s *SherpaExtractorAdapter) CreateStream() SpeakerStream {
	stream := s.extractor.CreateStream()
	if stream == nil {
		return nil
	}
	return &SherpaStreamAdapter{stream: stream}
}

func (s *SherpaExtractorAdapter) IsReady(stream SpeakerStream) bool {
	if adapter, ok := stream.(*SherpaStreamAdapter); ok {
		return s.extractor.IsReady(adapter.stream)
	}
	return false
}

func (s *SherpaExtractorAdapter) Compute(stream SpeakerStream) []float32 {
	if adapter, ok := stream.(*SherpaStreamAdapter); ok {
		return s.extractor.Compute(adapter.stream)
	}
	return nil
}

func (s *SherpaExtractorAdapter) Dim() int {
	return s.extractor.Dim()
}

func (s *SherpaExtractorAdapter) Delete() {
	if s.extractor != nil {
		sherpa_onnx.DeleteSpeakerEmbeddingExtractor(s.extractor)
	}
}

// SherpaManagerAdapter adapts sherpa_onnx.SpeakerEmbeddingManager to SpeakerEmbeddingManager interface
type SherpaManagerAdapter struct {
	manager *sherpa_onnx.SpeakerEmbeddingManager
}

func NewSherpaManagerAdapter(dim int) *SherpaManagerAdapter {
	manager := sherpa_onnx.NewSpeakerEmbeddingManager(dim)
	if manager == nil {
		return nil
	}
	return &SherpaManagerAdapter{manager: manager}
}

func (s *SherpaManagerAdapter) RegisterV(speakerID string, embeddings [][]float32) bool {
	return s.manager.RegisterV(speakerID, embeddings)
}

func (s *SherpaManagerAdapter) Search(embedding []float32, threshold float32) string {
	return s.manager.Search(embedding, threshold)
}

func (s *SherpaManagerAdapter) Remove(speakerID string) {
	s.manager.Remove(speakerID)
}

func (s *SherpaManagerAdapter) Verify(speakerID string, embedding []float32, threshold float32) bool {
	return s.manager.Verify(speakerID, embedding, threshold)
}

func (s *SherpaManagerAdapter) Contains(speakerID string) bool {
	return s.manager.Contains(speakerID)
}

func (s *SherpaManagerAdapter) Delete() {
	if s.manager != nil {
		sherpa_onnx.DeleteSpeakerEmbeddingManager(s.manager)
	}
}

// SherpaStreamAdapter adapts sherpa_onnx stream to SpeakerStream interface
type SherpaStreamAdapter struct {
	stream *sherpa_onnx.OnlineStream
}

func (s *SherpaStreamAdapter) AcceptWaveform(sampleRate int, samples []float32) {
	s.stream.AcceptWaveform(sampleRate, samples)
}

func (s *SherpaStreamAdapter) InputFinished() {
	s.stream.InputFinished()
}

func (s *SherpaStreamAdapter) Delete() {
	if s.stream != nil {
		sherpa_onnx.DeleteOnlineStream(s.stream)
	}
}