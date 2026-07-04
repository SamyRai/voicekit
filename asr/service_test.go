package asr

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SamyRai/voicekit/types"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

func TestConfigValidateDisabledSkipsModelPaths(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = false
	config.Offline.ModelPath = ""

	if err := config.Validate(); err != nil {
		t.Fatalf("disabled ASR should skip model path validation: %v", err)
	}
}

func TestConfigValidateEnabledRequiresSherpaPaths(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true

	if err := config.Validate(); err == nil {
		t.Fatal("enabled ASR without Sherpa model paths should fail validation")
	}
}

func TestConfigValidateOfflineTransducerRequiresAllPaths(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Backend = BackendSherpaOffline
	config.Offline = OfflineConfig{
		ModelFamily: OfflineFamilyTransducer,
		EncoderPath: tempModelFile(t, "encoder.onnx"),
		DecoderPath: tempModelFile(t, "decoder.onnx"),
	}

	if err := config.Validate(); err == nil {
		t.Fatal("offline transducer without tokens and joiner should fail")
	}
}

func TestBuildOfflineRecognizerConfigMapsWhisper(t *testing.T) {
	config := DefaultConfig()
	config.Enabled = true
	config.Backend = BackendSherpaOffline
	config.SampleRate = 16000
	config.FeatureDim = 80
	config.Provider = "cpu"
	config.NumThreads = 2
	config.Offline = OfflineConfig{
		ModelFamily:             OfflineFamilyWhisper,
		EncoderPath:             tempModelFile(t, "whisper-encoder.onnx"),
		DecoderPath:             tempModelFile(t, "whisper-decoder.onnx"),
		Language:                "en",
		Task:                    "translate",
		TailPaddings:            4,
		EnableTokenTimestamps:   true,
		EnableSegmentTimestamps: true,
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("whisper config should validate: %v", err)
	}
	mapped, err := buildOfflineRecognizerConfig(&config)
	if err != nil {
		t.Fatalf("failed to build offline recognizer config: %v", err)
	}
	if mapped.ModelConfig.Whisper.Encoder != config.Offline.EncoderPath {
		t.Fatalf("encoder path not mapped")
	}
	if mapped.ModelConfig.Whisper.Decoder != config.Offline.DecoderPath {
		t.Fatalf("decoder path not mapped")
	}
	if mapped.ModelConfig.Whisper.Task != "translate" {
		t.Fatalf("expected translate task, got %q", mapped.ModelConfig.Whisper.Task)
	}
	if mapped.ModelConfig.Whisper.EnableTokenTimestamps != 1 {
		t.Fatalf("expected token timestamps enabled")
	}
	if mapped.ModelConfig.NumThreads != 2 {
		t.Fatalf("expected two threads, got %d", mapped.ModelConfig.NumThreads)
	}
}

func TestSherpaOfflineModelTranscribeMapsResult(t *testing.T) {
	stream := &fakeOfflineStream{
		result: &sherpa.OfflineRecognizerResult{
			Text:       "hello world",
			Tokens:     []string{"hello", "world"},
			Timestamps: []float32{0, 0.5},
			Durations:  []float32{0.4, 0.3},
			Lang:       "en",
			Emotion:    "neutral",
			Event:      "speech",
		},
	}
	model := &SherpaOfflineModel{
		name:       "fake",
		language:   "auto",
		recognizer: &fakeOfflineRecognizer{stream: stream},
	}

	result, err := model.Transcribe(context.Background(), []float32{0.1, 0.2, 0.3, 0.4}, 2)
	if err != nil {
		t.Fatalf("transcription failed: %v", err)
	}
	if result.Text != "hello world" {
		t.Fatalf("unexpected text %q", result.Text)
	}
	if result.Language != "en" {
		t.Fatalf("unexpected language %q", result.Language)
	}
	if result.Emotion != "neutral" || result.Event != "speech" {
		t.Fatalf("expected emotion/event metadata to be mapped")
	}
	if result.EndTime != 2*time.Second {
		t.Fatalf("unexpected end time %s", result.EndTime)
	}
	if len(result.Words) != 2 {
		t.Fatalf("expected two words, got %d", len(result.Words))
	}
	if stream.acceptedRate != 2 || len(stream.acceptedSamples) != 4 {
		t.Fatalf("offline stream did not receive audio")
	}
}

func TestSherpaOfflineModelRejectsEmptyAudio(t *testing.T) {
	model := &SherpaOfflineModel{recognizer: &fakeOfflineRecognizer{stream: &fakeOfflineStream{}}}
	if _, err := model.Transcribe(context.Background(), nil, 16000); err == nil {
		t.Fatal("empty audio should fail before native calls")
	}
}

func TestSherpaOfflineModelCloseIdempotent(t *testing.T) {
	recognizer := &fakeOfflineRecognizer{stream: &fakeOfflineStream{}}
	model := &SherpaOfflineModel{recognizer: recognizer}

	if err := model.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := model.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
	if recognizer.closeCount != 1 {
		t.Fatalf("expected one native close, got %d", recognizer.closeCount)
	}
}

func TestVADProviderSelectionEnergy(t *testing.T) {
	service, err := NewVADService(&VADConfig{
		Provider:   VADProviderEnergy,
		Threshold:  0.01,
		SampleRate: 16000,
	})
	if err != nil {
		t.Fatalf("failed to create energy VAD: %v", err)
	}
	defer service.Close()

	result, err := service.Process([]float32{0.5, -0.5, 0.25}, nil)
	if err != nil {
		t.Fatalf("energy VAD failed: %v", err)
	}
	if !result.IsSpeech {
		t.Fatal("expected energy VAD to detect speech")
	}
}

func TestVADProviderSelectionSherpaRequiresModelPath(t *testing.T) {
	_, err := NewVADService(&VADConfig{
		Provider:   VADProviderSilero,
		SampleRate: 16000,
	})
	if err == nil {
		t.Fatal("Sherpa VAD without model path should fail")
	}
}

func TestASRServiceModelRegistrationAndSelection(t *testing.T) {
	service := newTestService()
	defer service.Close()

	fastModel := &testModel{name: "fast_model", language: "en", quantization: "int8", latency: 50 * time.Millisecond}
	accurateModel := &testModel{name: "accurate_model", language: "en", quantization: "float32", latency: 500 * time.Millisecond}

	if err := service.RegisterModel(fastModel); err != nil {
		t.Fatalf("failed to register fast model: %v", err)
	}
	if err := service.RegisterModel(accurateModel); err != nil {
		t.Fatalf("failed to register accurate model: %v", err)
	}

	selected, err := service.SelectModel("en", &types.ModelRequirements{PreferSpeed: true})
	if err != nil {
		t.Fatalf("failed to select model: %v", err)
	}
	if selected.Name() != "fast_model" {
		t.Fatalf("expected fast model, got %s", selected.Name())
	}
}

func TestASRServiceProcessAudioWithExplicitFakeModel(t *testing.T) {
	service := newTestService()
	defer service.Close()

	model := &testModel{name: "fake", language: "en", quantization: "int8", latency: 0}
	if err := service.RegisterModel(model); err != nil {
		t.Fatalf("failed to register fake model: %v", err)
	}

	audio := make([]float32, 16000)
	result, err := service.ProcessAudioChunk(context.Background(), "test-session", audio)
	if err != nil {
		t.Fatalf("failed to process audio chunk: %v", err)
	}
	if result == nil {
		t.Fatal("expected transcription result")
	}
	if result.Text != "test transcript" {
		t.Fatalf("expected fake transcript, got %q", result.Text)
	}
}

func TestSherpaOnlineModelReusesNativeStreamAcrossChunks(t *testing.T) {
	recognizer := &fakeOnlineRecognizer{}
	model := newSherpaOnlineModelForTest(recognizer)
	defer model.Close()

	state := &types.StreamingState{Language: "en"}
	if _, err := model.ProcessAudio(context.Background(), []float32{0.1, 0.2}, state); err != nil {
		t.Fatalf("first chunk failed: %v", err)
	}
	if _, err := model.ProcessAudio(context.Background(), []float32{0.3, 0.4}, state); err != nil {
		t.Fatalf("second chunk failed: %v", err)
	}

	if recognizer.newStreamCount != 1 {
		t.Fatalf("expected one native stream, got %d", recognizer.newStreamCount)
	}
	stream := recognizer.streams[0]
	if stream.acceptCount != 2 {
		t.Fatalf("expected two accepted chunks, got %d", stream.acceptCount)
	}
	if stream.inputFinishedCount != 0 {
		t.Fatalf("ordinary chunks must not finish the stream")
	}
	if stream.closeCount != 0 {
		t.Fatalf("stream should stay open across chunks")
	}
}

func TestSherpaOnlineModelFinishAudioClosesNativeStream(t *testing.T) {
	recognizer := &fakeOnlineRecognizer{}
	model := newSherpaOnlineModelForTest(recognizer)
	defer model.Close()

	state := &types.StreamingState{Language: "en"}
	if _, err := model.ProcessAudio(context.Background(), []float32{0.1}, state); err != nil {
		t.Fatalf("first chunk failed: %v", err)
	}
	result, err := model.FinishAudio(context.Background(), []float32{0.2}, state)
	if err != nil {
		t.Fatalf("finish failed: %v", err)
	}
	if result.IsPartial {
		t.Fatalf("finished result should be final")
	}
	if state.ASRState != nil {
		t.Fatalf("finished stream should be removed from state")
	}

	stream := recognizer.streams[0]
	if stream.inputFinishedCount != 1 {
		t.Fatalf("expected one InputFinished call, got %d", stream.inputFinishedCount)
	}
	if stream.closeCount != 1 {
		t.Fatalf("expected stream close after finish, got %d", stream.closeCount)
	}
}

func TestSherpaOnlineModelEndpointResetsWithoutFinishing(t *testing.T) {
	recognizer := &fakeOnlineRecognizer{endpoint: true}
	model := newSherpaOnlineModelForTest(recognizer)
	defer model.Close()

	result, err := model.ProcessAudio(context.Background(), []float32{0.1}, &types.StreamingState{Language: "en"})
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if result.IsPartial {
		t.Fatalf("endpoint result should be final")
	}
	if recognizer.resetCount != 1 {
		t.Fatalf("expected one endpoint reset, got %d", recognizer.resetCount)
	}
	stream := recognizer.streams[0]
	if stream.inputFinishedCount != 0 {
		t.Fatalf("endpoint reset must not call InputFinished")
	}
	if stream.closeCount != 0 {
		t.Fatalf("endpoint reset should keep the stream open")
	}
}

func TestProcessFinalAudioPrefersFinalizableModel(t *testing.T) {
	model := &finalizableTestModel{
		testModel: testModel{name: "finalizable", language: "en", quantization: "int8"},
	}

	result, err := processFinalAudio(context.Background(), model, []float32{0.1}, &types.StreamingState{Language: "en"})
	if err != nil {
		t.Fatalf("final processing failed: %v", err)
	}
	if result.IsPartial {
		t.Fatalf("expected final result")
	}
	if model.finishCalls != 1 || model.processCalls != 0 {
		t.Fatalf("expected FinishAudio only, got finish=%d process=%d", model.finishCalls, model.processCalls)
	}
}

func TestStreamingManagerClosesASRStateOnSessionRemoval(t *testing.T) {
	manager := NewStreamingManager(nil)
	defer manager.Close()

	state, err := manager.GetState("session")
	if err != nil {
		t.Fatalf("failed to get state: %v", err)
	}
	closer := &countingCloser{}
	state.ASRState = closer

	if err := manager.RemoveSession("session"); err != nil {
		t.Fatalf("failed to remove session: %v", err)
	}
	if closer.closeCount != 1 {
		t.Fatalf("expected ASR state close on removal, got %d", closer.closeCount)
	}
	if state.ASRState != nil {
		t.Fatalf("ASR state should be cleared after removal")
	}
}

func TestStreamingManagerCloseClosesASRStates(t *testing.T) {
	manager := NewStreamingManager(nil)
	state, err := manager.GetState("session")
	if err != nil {
		t.Fatalf("failed to get state: %v", err)
	}
	closer := &countingCloser{}
	state.ASRState = closer

	if err := manager.Close(); err != nil {
		t.Fatalf("failed to close manager: %v", err)
	}
	if closer.closeCount != 1 {
		t.Fatalf("expected ASR state close, got %d", closer.closeCount)
	}
	if state.ASRState != nil {
		t.Fatalf("ASR state should be cleared after close")
	}
}

func BenchmarkASRService_ProcessAudioChunk(b *testing.B) {
	service := newTestService()
	defer service.Close()

	if err := service.RegisterModel(&testModel{name: "fake", language: "en", quantization: "int8"}); err != nil {
		b.Fatalf("failed to register fake model: %v", err)
	}

	audio := make([]float32, 16000)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ProcessAudioChunk(ctx, "bench-session", audio)
		if err != nil {
			b.Fatalf("failed to process audio chunk: %v", err)
		}
	}
}

func newTestService() *Service {
	config := DefaultConfig()
	config.Enabled = true
	config.Backend = BackendSherpaOnline
	config.DefaultModel = "fake"
	config.VADProvider = VADProviderNone

	streamingConfig := &types.StreamingConfig{
		ChunkSize:          config.ChunkSize,
		OverlapSize:        config.ChunkSize / 10,
		BufferSize:         config.ChunkSize * 3,
		SampleRate:         config.SampleRate,
		StreamTimeout:      time.Duration(config.StreamTimeout) * time.Second,
		IdleTimeout:        30 * time.Second,
		FlushInterval:      100 * time.Millisecond,
		PartialResults:     true,
		StabilityThreshold: 0.8,
		MinConfidence:      0.5,
	}

	return &Service{
		config:    &config,
		models:    make(map[string]Model),
		streaming: NewStreamingManager(streamingConfig),
		metrics:   &ASRMetrics{},
	}
}

func tempModelFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create temp model file: %v", err)
	}
	return path
}

type fakeOfflineRecognizer struct {
	stream     *fakeOfflineStream
	closeCount int
}

func (r *fakeOfflineRecognizer) NewStream() (offlineStream, error) {
	return r.stream, nil
}

func (r *fakeOfflineRecognizer) Decode(stream offlineStream) error {
	return nil
}

func (r *fakeOfflineRecognizer) Close() error {
	r.closeCount++
	return nil
}

type fakeOfflineStream struct {
	result          *sherpa.OfflineRecognizerResult
	acceptedRate    int
	acceptedSamples []float32
	closeCount      int
}

func (s *fakeOfflineStream) AcceptWaveform(sampleRate int, samples []float32) error {
	s.acceptedRate = sampleRate
	s.acceptedSamples = append([]float32(nil), samples...)
	return nil
}

func (s *fakeOfflineStream) Result() *sherpa.OfflineRecognizerResult {
	return s.result
}

func (s *fakeOfflineStream) Close() error {
	s.closeCount++
	return nil
}

type testModel struct {
	name         string
	language     string
	quantization string
	latency      time.Duration
}

func (m *testModel) Name() string         { return m.name }
func (m *testModel) Language() string     { return m.language }
func (m *testModel) Quantization() string { return m.quantization }
func (m *testModel) SupportsLanguage(lang string) bool {
	return lang == "" || m.language == lang
}
func (m *testModel) Latency() time.Duration { return m.latency }
func (m *testModel) Close() error           { return nil }
func (m *testModel) ProcessAudio(ctx context.Context, audio []float32, state *types.StreamingState) (*types.Transcription, error) {
	return &types.Transcription{
		Text:      "test transcript",
		IsPartial: true,
		Language:  m.language,
		Timestamp: time.Now(),
	}, nil
}

type finalizableTestModel struct {
	testModel
	processCalls int
	finishCalls  int
}

func (m *finalizableTestModel) ProcessAudio(ctx context.Context, audio []float32, state *types.StreamingState) (*types.Transcription, error) {
	m.processCalls++
	return m.testModel.ProcessAudio(ctx, audio, state)
}

func (m *finalizableTestModel) FinishAudio(ctx context.Context, audio []float32, state *types.StreamingState) (*types.Transcription, error) {
	m.finishCalls++
	return &types.Transcription{
		Text:      "final transcript",
		IsPartial: false,
		Language:  m.language,
		Timestamp: time.Now(),
	}, nil
}

type countingCloser struct {
	closeCount int
}

func (c *countingCloser) Close() error {
	c.closeCount++
	return nil
}

func newSherpaOnlineModelForTest(recognizer *fakeOnlineRecognizer) *SherpaOnlineModel {
	return &SherpaOnlineModel{
		name:         "fake-sherpa-online",
		language:     "en",
		quantization: "int8",
		sampleRate:   16000,
		recognizer:   recognizer,
		sessions:     make(map[*onlineSession]struct{}),
	}
}

type fakeOnlineRecognizer struct {
	streams        []*fakeOnlineStream
	newStreamCount int
	decodeCount    int
	resetCount     int
	closeCount     int
	endpoint       bool
	result         *sherpa.OnlineRecognizerResult
}

func (r *fakeOnlineRecognizer) NewStream() (onlineStream, error) {
	r.newStreamCount++
	stream := &fakeOnlineStream{}
	r.streams = append(r.streams, stream)
	return stream, nil
}

func (r *fakeOnlineRecognizer) Decode(stream onlineStream) error {
	r.decodeCount++
	return nil
}

func (r *fakeOnlineRecognizer) IsReady(stream onlineStream) bool {
	return false
}

func (r *fakeOnlineRecognizer) IsEndpoint(stream onlineStream) bool {
	return r.endpoint
}

func (r *fakeOnlineRecognizer) Reset(stream onlineStream) error {
	r.resetCount++
	r.endpoint = false
	return nil
}

func (r *fakeOnlineRecognizer) Result(stream onlineStream) (*sherpa.OnlineRecognizerResult, error) {
	if r.result != nil {
		return r.result, nil
	}
	return &sherpa.OnlineRecognizerResult{Text: "stream result"}, nil
}

func (r *fakeOnlineRecognizer) Close() error {
	r.closeCount++
	return nil
}

type fakeOnlineStream struct {
	acceptCount        int
	inputFinishedCount int
	closeCount         int
	sampleRate         int
	samples            []float32
}

func (s *fakeOnlineStream) AcceptWaveform(sampleRate int, samples []float32) error {
	if s.inputFinishedCount > 0 {
		return nil
	}
	s.acceptCount++
	s.sampleRate = sampleRate
	s.samples = append(s.samples, samples...)
	return nil
}

func (s *fakeOnlineStream) InputFinished() error {
	s.inputFinishedCount++
	return nil
}

func (s *fakeOnlineStream) Close() error {
	s.closeCount++
	return nil
}
