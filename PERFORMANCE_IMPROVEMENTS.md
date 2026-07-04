# VoiceKit Performance Improvements & Optimization Recommendations

**Document Version:** 1.1
**Date:** July 2026
**Based on:** Research of latest best practices for audio processing, speaker recognition, diarization, Sherpa-ONNX, and Go 1.26.4 performance behavior

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Measured July 2026 Performance Sprint](#measured-july-2026-performance-sprint)
3. [Audio Processing Optimizations](#audio-processing-optimizations)
4. [Speaker Recognition Optimizations](#speaker-recognition-optimizations)
5. [Diarization Algorithm Improvements](#diarization-algorithm-improvements)
6. [Memory Management & GC Optimization](#memory-management--gc-optimization)
7. [Concurrency & Locking Improvements](#concurrency--locking-improvements)
8. [Database & Storage Optimizations](#database--storage-optimizations)
9. [Sherpa-ONNX Integration Improvements](#sherpa-onnx-integration-improvements)
10. [Real-Time Streaming Optimizations](#real-time-streaming-optimizations)
11. [Code Quality & Architecture Improvements](#code-quality--architecture-improvements)
12. [Implementation Priority & Roadmap](#implementation-priority--roadmap)

---

## Executive Summary

This document provides comprehensive recommendations for improving VoiceKit's performance, scalability, and maintainability. The recommendations are based on:

- Latest research in audio processing, speaker recognition, and diarization (2024-2025)
- Go language best practices and performance optimization techniques
- Production-grade patterns for high-throughput systems
- Modern database and storage solutions for embedding databases

**Key Impact Areas:**
- **Audio Processing**: 4-8× speedup potential with SIMD optimization
- **Speaker Recognition**: 10-100× faster searches with ANN indexing
- **Diarization**: 20-30% DER improvement with modern clustering algorithms
- **Memory**: 50-85% reduction in GC pressure with proper pooling
- **Database**: 2-5× faster writes with modern embedded databases

---

## Measured July 2026 Performance Sprint

**Environment:** Go 1.26.4, darwin/arm64, Apple M2, local CPU benchmarks. No
model files, hosted providers, new production dependencies, secrets, auth
settings, or production data were used.

The July 2026 performance sprint focused on local SDK overhead before native
model RTF work. The high-confidence hot paths were WAV/PCM conversion and the
basic diarization fallback. Sherpa-backed runtime performance remains
env-gated because model paths are not committed.

### Benchmark Workflow

Benchmark artifacts should be written outside tracked source by default:

```bash
mkdir -p /tmp/voicekit-benchmarks

go test -bench='Benchmark(AudioProcessingPipeline|AudioSegmentation|DiarizationPipeline|Clustering|Clustering_50|Clustering_100)$' \
  -benchmem -run=^$ -count=6 ./audio ./diarization \
  | tee /tmp/voicekit-benchmarks/voicekit-perf-before.txt

go test -bench='Benchmark(AudioProcessingPipeline|AudioSegmentation|DiarizationPipeline|Clustering|Clustering_50|Clustering_100)$' \
  -benchmem -run=^$ -count=6 ./audio ./diarization \
  | tee /tmp/voicekit-benchmarks/voicekit-perf-after.txt

benchstat /tmp/voicekit-benchmarks/voicekit-perf-before.txt \
  /tmp/voicekit-benchmarks/voicekit-perf-after.txt
```

The Makefile wraps this convention:

```bash
make bench-all
make bench-profile
make bench-memprofile
make bench-compare BEFORE=/tmp/voicekit-benchmarks/before.txt AFTER=/tmp/voicekit-benchmarks/after.txt
```

Broader benchmark sweep:

```bash
go test -bench=. -benchmem -run=^$ -count=6 \
  ./audio ./diarization ./speaker ./indexing ./asr ./meeting ./evaluation .
```

### July 2026 Results

The sprint exceeded the allocation targets without changing public APIs.

| Benchmark | Before | After | Change |
| --- | ---: | ---: | ---: |
| `BenchmarkAudioProcessingPipeline` time | 90.734 us/op | 9.461 us/op | -89.57% |
| `BenchmarkAudioProcessingPipeline` memory | 328.60 KiB/op | 64.58 KiB/op | -80.35% |
| `BenchmarkAudioSegmentation` time | 117.39 us/op | 69.36 us/op | -40.91% |
| `BenchmarkAudioSegmentation` memory | 984.5 KiB/op | 192.1 KiB/op | -80.49% |
| `BenchmarkDiarizationPipeline` time | 124.47 us/op | 53.31 us/op | -57.17% |
| `BenchmarkDiarizationPipeline` memory | 417.41 KiB/op | 83.72 KiB/op | -79.94% |
| `BenchmarkClustering_50` time | 242.49 us/op | 14.80 us/op | -93.90% |
| `BenchmarkClustering_50` memory | 10,656 B/op | 416 B/op | -96.10% |

Implementation notes:

- `audio.(*Converter).decodeWAV` and `ParseWAVFile` now use `go-audio/wav` for
  RIFF/header validation but decode PCM chunks directly into a preallocated
  `[]float32`.
- Raw PCM encode/decode uses direct little-endian indexing instead of
  per-sample `binary.Read`, `binary.Write`, or temporary 24-bit buffers.
- Basic silence segmentation collects speech sample ranges first and
  materializes final `AudioSegment.Samples` once per segment.
- The small clustering path now uses the direct union-find path because the
  precomputed similarity matrix was slower and allocated more for current
  benchmark sizes.
- Root fake benchmarks were removed or fixed; meeting/evaluation benchmarks now
  cover `meeting.FromIntegratedResult`, `Meeting.Analyze` with
  `HeuristicAnalyzer`, `Meeting.Redact`, all export formats, and
  `evaluation.EvaluateMeeting`.

Repository hygiene note: raw benchmark outputs, CPU profiles, and memory
profiles are generated outside tracked source by default and ignored if created
locally. Keep durable conclusions in this document instead of committing raw
one-off benchmark artifacts.

Dependency note: `go-audio-resampler` and `encoding/json/v2` were not adopted.
Candidate libraries should stay behind scratch benchmarks and dependency review
until they show a measured hot-path payoff and receive explicit approval.

### Sprint Learnings for the Next Developer

Use these notes before starting the next performance slice:

- Read dependency source around the suspected hot path. In this sprint,
  `go-audio/wav.FullPCMBuffer` was the allocation source, not VoiceKit's header
  validation. Keeping `go-audio/wav` for RIFF parsing while bypassing
  `FullPCMBuffer` produced the win without weakening format correctness.
- Prefer one final owned allocation over pool-plus-copy when returning data to
  callers. Pools are still useful for temporary chunk buffers and internal
  scratch space, but returned `[]byte` or `[]float32` values usually need stable
  caller ownership.
- Segmenting by appending samples during detection creates hidden allocation
  pressure. Collect timing/sample ranges first, then copy the final payload once
  after the segment is known.
- Preserve quality semantics explicitly. The optimized diarization path keeps
  the old "speech samples only" behavior by storing speech ranges instead of
  widening each segment to include silent gaps.
- Do not assume precomputation is faster. The small clustering similarity matrix
  was both slower and more memory-heavy than direct pairwise similarity at the
  current segment counts.
- Audit benchmarks for impossible results before acting on them. Sub-nanosecond
  or zero-work benchmarks must be replaced with real public operations or
  guarded with sinks/escapes that prevent compiler elimination.
- Keep model-backed claims separate from deterministic SDK overhead. Sherpa RTF,
  DER, and accuracy comparisons require explicit local model paths and fixture
  audio; they are not proven by unit tests or synthetic benchmarks alone.

Measured follow-up candidates:

- `BenchmarkMeetingExports/Markdown` now has a baseline and allocates
  significantly more than caption exports. Optimize only if meeting export
  volume makes it a real hot path.
- `BenchmarkEvaluateMeeting` is intentionally deterministic but relatively
  expensive because edit-distance work scales with transcript length. Optimize
  after larger fixture coverage defines realistic meeting sizes.

---

## Audio Processing Optimizations

### 1. SIMD Vectorization for Resampling

**Current State:** Pure Go implementation with scalar loops  
**Recommendation:** Implement SIMD-accelerated resampling using AVX2/SSE/NEON

**Implementation Strategy:**

1. **Use `tphakala/go-audio-resampler` or similar library**
   - Pure Go with SIMD support via `tphakala/simd`
   - Runtime CPU feature detection (AVX2, SSE, NEON)
   - Zero-allocation operations
   - Thread-safe

2. **Hand-optimize critical inner loops**
   ```go
   // Example structure for SIMD-optimized FIR filter
   type SIMDResampler struct {
       isAVX2 bool
       isSSE   bool
       isNEON  bool
       // ... filter coefficients
   }
   
   func (r *SIMDResampler) resampleSIMD(input []float32, ratio float64) []float32 {
       if r.isAVX2 {
           return r.resampleAVX2(input, ratio)
       } else if r.isSSE {
           return r.resampleSSE(input, ratio)
       }
       return r.resampleScalar(input, ratio) // Fallback
   }
   ```

3. **Multi-stage pipeline architecture**
   - Frequency-domain stage (FFT/DFT) for coarse filtering
   - Polyphase FIR filtering for fine resampling
   - Windowed FIR filters (Kaiser window) for anti-aliasing

**Expected Performance Gains:**
- 4-8× speedup for moderate filter lengths with AVX2
- 2-3× speedup for multichannel audio (stereo, 5.1, 7.1)
- Better quality with longer filters while maintaining performance

**Build Configuration:**
```bash
# Enable AVX2 support
GOAMD64=v3 go build

# For ARM64 (NEON)
GOOS=linux GOARCH=arm64 go build
```

**References:**
- `github.com/tphakala/go-audio-resampler` - Production-ready SIMD resampler
- `github.com/tphakala/simd` - SIMD primitives library

---

### 2. Precision Optimization (Float32 vs Float64)

**Current State:** Mixed precision usage  
**Recommendation:** Standardize on float32 for audio pipeline

**Benefits:**
- 2× SIMD throughput (8 floats per AVX2 register vs 4 doubles)
- 50% memory reduction
- Better cache utilization

**Implementation:**
- Ensure all audio processing uses `float32`
- Only use `float64` where numerical precision is critical (e.g., filter coefficient calculations)

---

### 3. Buffer Pooling for Audio Processing

**Current State:** Allocations per operation  
**Recommendation:** Implement size-classed buffer pools

**Implementation:**
```go
type AudioBufferPool struct {
    small  sync.Pool  // 1KB - 8KB
    medium sync.Pool  // 8KB - 64KB
    large  sync.Pool  // 64KB - 512KB
}

func (p *AudioBufferPool) Get(size int) []float32 {
    var pool *sync.Pool
    switch {
    case size <= 8192:
        pool = &p.small
    case size <= 65536:
        pool = &p.medium
    default:
        pool = &p.large
    }
    
    buf := pool.Get()
    if buf == nil {
        return make([]float32, size)
    }
    slice := buf.([]float32)
    if cap(slice) < size {
        return make([]float32, size)
    }
    return slice[:size]
}

func (p *AudioBufferPool) Put(buf []float32) {
    // Reset length but keep capacity
    buf = buf[:0]
    
    var pool *sync.Pool
    switch {
    case cap(buf) <= 8192:
        pool = &p.small
    case cap(buf) <= 65536:
        pool = &p.medium
    default:
        pool = &p.large
    }
    pool.Put(buf)
}
```

**Best Practices:**
- Always reset buffers before returning to pool (`slice[:0]`)
- Use separate pools for different size classes
- Monitor pool hit/miss rates

---

### 4. Streaming Processing Optimization

**Current State:** Full buffer processing  
**Recommendation:** Implement chunked/streaming processing with overlap-add

**Benefits:**
- Lower memory footprint for long audio files
- Better cache locality
- Reduced latency for real-time processing

**Implementation Pattern:**
```go
type StreamingResampler struct {
    buffer    []float32
    overlap   []float32
    chunkSize int
}

func (r *StreamingResampler) ProcessChunk(input []float32) []float32 {
    // Process with overlap-add
    // Maintain state between chunks
}
```

---

## Speaker Recognition Optimizations

### 1. Approximate Nearest Neighbor (ANN) Indexing

**Current State:** Linear search through all speaker embeddings  
**Recommendation:** Implement HNSW (Hierarchical Navigable Small World) index

**Why HNSW:**
- O(log n) query time vs O(n) linear search
- 10-100× faster for large speaker databases (1000+ speakers)
- High recall (95%+ with proper tuning)
- Supports dynamic updates (insertions/deletions)

**Implementation Options:**

1. **Use existing library:**
   - `github.com/weaviate/weaviate` (includes HNSW)
   - `github.com/milvus-io/milvus` (full vector database)
   - `github.com/go-skynet/LocalAI` (includes FAISS/HNSW bindings)

2. **Custom implementation with optimizations:**
   ```go
   type HNSWIndex struct {
       // Dual-branch HNSW with LID-based insertion
       // Skip bridges for redundant level avoidance
       // MN-RU algorithm for real-time updates
   }
   ```

**Recent Advances (2024-2025):**
- **Dual-branch HNSW**: +18-30% recall improvement
- **LID-based insertion**: Order insertions by Local Intrinsic Dimensionality
- **MN-RU algorithm**: Reduces unreachable points during updates
- **HENN (Hierarchical ε-Net Navigation)**: Polylogarithmic worst-case guarantees

**Configuration:**
```go
type HNSWConfig struct {
    M              int     // Max neighbors per node (16-64 typical)
    EfConstruction int     // Search width during construction (200-400)
    EfSearch       int     // Search width during queries (50-200)
    // Adaptive ef based on query characteristics
}
```

**Expected Performance:**
- 1000 speakers: 10-50× faster
- 10000 speakers: 50-200× faster
- 100000+ speakers: 200-1000× faster

---

### 2. Embedding Compression & Quantization

**Current State:** Full float32 embeddings (192-512 dimensions)  
**Recommendation:** Implement product quantization for large-scale storage

**Benefits:**
- 4-16× memory reduction
- Faster similarity computation (integer operations)
- Better cache utilization

**Implementation:**
```go
type QuantizedEmbedding struct {
    // Product quantization: split into sub-vectors
    // Each sub-vector quantized to 8-bit codebook
    codes []uint8
}

func (q *QuantizedEmbedding) Similarity(other *QuantizedEmbedding) float32 {
    // Fast integer-based distance computation
}
```

**Trade-offs:**
- Slight accuracy loss (<1% typically)
- More complex implementation
- Best for large databases (>10K speakers)

---

### 3. Batch Processing for Embedding Extraction

**Current State:** Single embedding extraction per call  
**Recommendation:** Batch multiple audio segments together

**Benefits:**
- Better GPU utilization (if using GPU)
- Reduced overhead per embedding
- 2-4× throughput improvement

**Implementation:**
```go
func (m *Manager) ExtractEmbeddingsBatch(audioSegments [][]float32, sampleRate int) ([][]float32, error) {
    // Batch process through Sherpa-ONNX
    // Process multiple segments in parallel
}
```

---

### 4. Embedding Caching Strategy

**Current State:** All embeddings loaded in memory  
**Recommendation:** Implement LRU cache with disk-backed storage

**Benefits:**
- Reduced memory usage
- Fast access to frequently used speakers
- Support for very large databases

**Implementation:**
```go
type EmbeddingCache struct {
    lru    *lru.Cache  // In-memory LRU
    db     *pebble.DB  // Disk storage
    maxMem int         // Max memory for cache
}
```

---

## Diarization Algorithm Improvements

### 1. Modern Clustering Algorithms

**Current State:** Production diarization now routes through Sherpa offline diarization. The old silence-plus-embedding clustering implementation remains only as an explicit `basic` backend for tests and local experiments.

**Recommendation:** Benchmark Sherpa segmentation/embedding model combinations before adding another clustering implementation.

**Option 1: VBx (Variational Bayesian HMM Clustering)**
- Better DER (Diarization Error Rate) than AHC
- Handles speaker identity over time
- Models uncertainty in assignments
- **Expected improvement:** 15-25% DER reduction

**Option 2: E-SHARC (End-to-End Supervised Hierarchical Graph Clustering)**
- Graph Neural Network (GNN) clustering
- Handles overlapping speech
- Joint training of embedding + clustering
- **Expected improvement:** 20-30% DER reduction on overlap-heavy datasets

**Option 3: MS-VBx (Multi-Stream VBx)**
- Extends VBx for multiple embedding streams
- Better speaker counting accuracy
- **Expected improvement:** 10-20% DER reduction, better speaker counting

**Option 4: SC-pNA (Self-Tuning Spectral Clustering)**
- Auto-tunes neighbor selection
- Sparsifies affinity matrix (top p% similarities)
- **Expected improvement:** 10-15% DER reduction, better efficiency

**Implementation Priority:**
1. Measure Sherpa offline diarization throughput and DER on target audio.
2. Tune `ClusteringThreshold`, `NumClusters`, `MinDurationOn`, and `MinDurationOff`.
3. Consider VBx or overlap handling only if Sherpa-backed results show a concrete gap.

---

### 2. Overlap Detection & Handling

**Current State:** No explicit overlap handling  
**Recommendation:** Add overlap detection and frame-wise embedding support

**Benefits:**
- Better accuracy in meeting scenarios
- Handles simultaneous speakers
- Reduces false speaker assignments

**Implementation:**
```go
type OverlapDetector interface {
    DetectOverlap(audio []float32, sampleRate int) []OverlapRegion
}

type FrameWiseEmbedding struct {
    // Frame-level embeddings with geodesic interpolation
    // Embeddings of overlapping regions lie on shortest path
    // between pure-speaker embeddings
}
```

**Research Reference:**
- Frame-wise embeddings with geodesic distance loss (2024)
- Outperforms segment-level clustering in overlap scenarios

---

### 3. Short Segment Filtering

**Current State:** All segments processed equally  
**Recommendation:** Filter unreliable short segments

**Implementation:**
```go
func (m *Manager) filterUnreliableSegments(segments []AudioSegment) []AudioSegment {
    filtered := make([]AudioSegment, 0)
    for _, seg := range segments {
        duration := seg.EndTime - seg.StartTime
        if duration >= m.config.MinSegmentLength {
            // Additional quality checks
            if m.isReliableSegment(seg) {
                filtered = append(filtered, seg)
            }
        }
    }
    return filtered
}
```

**Benefits:**
- Improved speaker counting accuracy
- Reduced false positives
- Better DER on challenging datasets

---

### 4. Adaptive Affinity Matrix Sparsification

**Current State:** Full similarity matrix computation  
**Recommendation:** Sparsify to top-k neighbors per segment

**Benefits:**
- Reduced computational cost (O(n²) → O(n log n))
- Less memory usage
- Auto-tuning reduces parameter sensitivity

**Implementation:**
```go
func (m *Manager) buildSparseAffinityMatrix(embeddings [][]float32) *SparseMatrix {
    // For each embedding, keep only top-k most similar
    // Use heap or partial sort
    // Auto-select k based on data distribution
}
```

---

## Memory Management & GC Optimization

### 1. Comprehensive Object Pooling Strategy

**Current State:** Limited pooling  
**Recommendation:** Implement size-classed pools for all temporary objects

**Areas to Pool:**
- Audio buffers (float32 slices)
- Embedding vectors
- Similarity computation scratch space
- Clustering intermediate data structures
- JSON encoding/decoding buffers

**Best Practices:**
```go
// Always reset before Put
func (p *Pool) Put(buf []float32) {
    buf = buf[:0]  // Reset length, keep capacity
    p.pool.Put(buf)
}

// Use New function
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]float32, 0, 8192)
    },
}
```

**Expected Impact:**
- 50-85% reduction in GC pressure
- Lower latency spikes
- More predictable performance

---

### 2. Slice Reuse Patterns

**Current State:** New slices allocated frequently  
**Recommendation:** Reuse slice capacity

**Pattern:**
```go
type Processor struct {
    scratch []float32  // Reused across calls
}

func (p *Processor) Process(data []float32) {
    // Reuse capacity
    p.scratch = p.scratch[:0]
    p.scratch = append(p.scratch, data...)
    // Process...
}
```

---

### 3. Pre-allocation Strategies

**Recommendation:** Pre-allocate slices with expected capacity

```go
// Instead of:
segments := []AudioSegment{}

// Use:
segments := make([]AudioSegment, 0, estimatedCount)
```

**Benefits:**
- Fewer reallocations
- Better memory locality
- Reduced GC churn

---

## Concurrency & Locking Improvements

### 1. Sharded Speaker Database

**Current State:** Single RWMutex for entire database  
**Recommendation:** Shard database into multiple partitions

**Implementation:**
```go
type ShardedSpeakerDB struct {
    shards []*SpeakerShard
    shardCount int
}

type SpeakerShard struct {
    mu   sync.RWMutex
    data map[string]*SpeakerData
}

func (db *ShardedSpeakerDB) getShard(speakerID string) *SpeakerShard {
    h := hash(speakerID)
    return db.shards[h % db.shardCount]
}
```

**Benefits:**
- Reduced lock contention
- Better scalability
- 2-5× throughput improvement under high concurrency

**Shard Count:** Typically 16-64 shards (power of 2)

---

### 2. Lock-Free Counters & Statistics

**Current State:** Mutex-protected counters  
**Recommendation:** Use atomic operations for simple counters

**Implementation:**
```go
type Stats struct {
    totalRequests int64
    totalErrors   int64
}

func (s *Stats) IncRequests() {
    atomic.AddInt64(&s.totalRequests, 1)
}

func (s *Stats) GetRequests() int64 {
    return atomic.LoadInt64(&s.totalRequests)
}
```

**Use Cases:**
- Request counters
- Error counts
- Performance metrics
- Cache hit/miss statistics

---

### 3. Consider sync.Map for Read-Heavy Workloads

**Current State:** map + RWMutex  
**Recommendation:** Evaluate sync.Map for stable key sets

**When to Use:**
- Read-heavy workloads (reads >> writes)
- Stable key set (few insertions/deletions)
- Type safety not critical

**Trade-offs:**
- Excellent read performance (lock-free fast path)
- Higher memory usage
- Slower writes
- Less type-safe (interface{})

**Implementation:**
```go
var speakerCache sync.Map  // key: string, value: *SpeakerData

func GetSpeaker(id string) (*SpeakerData, bool) {
    if v, ok := speakerCache.Load(id); ok {
        return v.(*SpeakerData), true
    }
    return nil, false
}
```

---

### 4. Minimize Critical Sections

**Current State:** Some operations hold locks too long  
**Recommendation:** Minimize code under locks

**Anti-pattern:**
```go
// BAD: I/O under lock
m.mutex.Lock()
data, err := ioutil.ReadFile(path)  // I/O!
m.database = parse(data)
m.mutex.Unlock()
```

**Better:**
```go
// GOOD: I/O outside lock
data, err := ioutil.ReadFile(path)
m.mutex.Lock()
m.database = parse(data)
m.mutex.Unlock()
```

---

## Database & Storage Optimizations

### 1. Replace JSON File with Embedded Database

**Current State:** JSON file for speaker database  
**Recommendation:** Migrate to embedded key-value database

**Options Comparison:**

| Database | Write Performance | Read Performance | Memory Usage | Best For |
|----------|-------------------|------------------|--------------|----------|
| **Pebble** | Excellent | Excellent | Moderate | Write-heavy, large datasets |
| **NutsDB** | Excellent (2-5× faster than bbolt) | Good | Low (with hint files) | Fast startup, moderate size |
| **bbolt** | Limited (single writer) | Excellent | Low | Read-heavy, simple needs |
| **Badger** | Excellent | Good | Higher | Large datasets, LSM benefits |

**Recommendation: Use Pebble**

**Why Pebble:**
- Pure Go (no CGO)
- Excellent write performance (comparable to RocksDB)
- Good read performance
- Mature and stable (used by CockroachDB)
- Better than Badger for most workloads
- SSD-optimized

**Migration Path:**
```go
import "github.com/cockroachdb/pebble"

type SpeakerDB struct {
    db *pebble.DB
}

func (db *SpeakerDB) SaveSpeaker(id string, data *SpeakerData) error {
    key := []byte("speaker:" + id)
    value, err := json.Marshal(data)
    if err != nil {
        return err
    }
    return db.db.Set(key, value, pebble.Sync)
}
```

**Expected Performance:**
- 2-5× faster writes vs JSON file
- 10-100× faster for large databases
- Atomic updates
- Better concurrency support

---

### 2. Hybrid Storage: Memory + Disk

**Recommendation:** Implement two-tier storage

**Architecture:**
- Hot data (frequently accessed speakers) in memory
- Cold data on disk (Pebble)
- LRU eviction policy

**Implementation:**
```go
type HybridSpeakerDB struct {
    cache *lru.Cache      // In-memory LRU
    disk  *pebble.DB      // Disk storage
}

func (db *HybridSpeakerDB) GetSpeaker(id string) (*SpeakerData, error) {
    // Try cache first
    if data, ok := db.cache.Get(id); ok {
        return data.(*SpeakerData), nil
    }
    
    // Fallback to disk
    data, err := db.loadFromDisk(id)
    if err != nil {
        return nil, err
    }
    
    // Populate cache
    db.cache.Add(id, data)
    return data, nil
}
```

---

### 3. Batch Writes

**Current State:** Individual writes per operation  
**Recommendation:** Batch multiple writes

**Benefits:**
- Better write throughput
- Reduced I/O operations
- Atomic batch commits

**Implementation:**
```go
type WriteBatch struct {
    batch *pebble.Batch
}

func (b *WriteBatch) AddSpeaker(id string, data *SpeakerData) {
    key := []byte("speaker:" + id)
    value, _ := json.Marshal(data)
    b.batch.Set(key, value, nil)
}

func (b *WriteBatch) Commit() error {
    return b.batch.Commit(pebble.Sync)
}
```

---

## Sherpa-ONNX Integration Improvements

### 1. GPU Acceleration Support

**Current State:** CPU-only inference  
**Recommendation:** Add GPU support with proper configuration

**Implementation:**
```go
type SpeakerConfig struct {
    Provider   string  // "cpu", "cuda", "directml"
    NumThreads int
    // GPU-specific options
    GPUDeviceID int    // For multi-GPU systems
    TensorRT    bool    // Use TensorRT optimization
}

// Configure for CUDA
extractorConfig := &sherpa_onnx.SpeakerEmbeddingExtractorConfig{
    Model:      config.ModelPath,
    Provider:   "cuda",  // Enable GPU
    NumThreads: 1,       // GPU doesn't need multiple threads
    DeviceID:   config.GPUDeviceID,
}
```

**Expected Performance:**
- 2-5× speedup with GPU
- 5-10× speedup with TensorRT
- Better batch processing throughput

**Build Requirements:**
- CUDA 11.8+ or 12.x
- cuDNN 8+ or 9
- Proper ONNX Runtime GPU build

---

### 2. Batch Processing for Embeddings

**Current State:** Single embedding extraction  
**Recommendation:** Batch multiple audio segments

**Implementation:**
```go
func (m *Manager) ExtractEmbeddingsBatch(
    audioSegments [][]float32,
    sampleRate int,
) ([][]float32, error) {
    // Group segments by length to minimize padding
    // Process in batches of 8-32
    // Use GPU if available
}
```

**Benefits:**
- Better GPU utilization
- Reduced overhead per embedding
- 2-4× throughput improvement

---

### 3. Model Quantization

**Current State:** FP32 models  
**Recommendation:** Support INT8 quantized models

**Benefits:**
- 2-4× speedup
- 75% memory reduction
- Minimal accuracy loss (<1% typically)

**Trade-offs:**
- Slight accuracy degradation
- Requires quantization-aware training or post-training quantization
- May be slower on some hardware (test first)

**Implementation:**
- Use ONNX Runtime quantization tools
- Test accuracy on your dataset
- Provide both FP32 and INT8 model options

---

### 4. Warmup & Model Caching

**Current State:** Cold start on first inference  
**Recommendation:** Implement warmup and model caching

**Implementation:**
```go
func (m *Manager) Warmup() error {
    // Run dummy inference to initialize
    dummyAudio := make([]float32, 16000) // 1 second at 16kHz
    _, err := m.extractEmbedding(dummyAudio, 16000)
    return err
}
```

**Benefits:**
- Eliminates first-request latency spike
- Better performance predictability
- Warms up GPU if available

---

## Real-Time Streaming Optimizations

### 1. Adaptive Buffer Management

**Current State:** Fixed buffer sizes  
**Recommendation:** Implement adaptive buffering

**Strategy:**
- Start with small buffers (10-20ms)
- Monitor network jitter and CPU load
- Increase buffer size if needed
- Decrease when conditions improve

**Implementation:**
```go
type AdaptiveBuffer struct {
    minSize    int
    maxSize    int
    currentSize int
    jitter     time.Duration
}

func (b *AdaptiveBuffer) Adjust(metrics *BufferMetrics) {
    if metrics.Jitter > threshold {
        b.currentSize = min(b.currentSize*2, b.maxSize)
    } else if metrics.Jitter < lowThreshold {
        b.currentSize = max(b.currentSize/2, b.minSize)
    }
}
```

---

### 2. Chunked Processing

**Current State:** Full audio processing  
**Recommendation:** Process in small chunks with overlap

**Benefits:**
- Lower latency
- Better real-time performance
- Reduced memory usage

**Implementation:**
```go
type StreamingProcessor struct {
    chunkSize   int
    overlap     int
    buffer      []float32
}

func (p *StreamingProcessor) ProcessChunk(chunk []float32) []float32 {
    // Add to buffer with overlap
    // Process chunk
    // Return result
}
```

---

### 3. Low-Latency Codec Support

**Recommendation:** Support low-latency codecs for streaming

**Options:**
- Opus with 10ms frames
- G.722 for telephony
- PCM for minimal latency

**Configuration:**
```go
type StreamingConfig struct {
    Codec        string
    FrameSize    time.Duration  // 10-20ms typical
    BufferSize   time.Duration  // Adaptive
}
```

---

## Code Quality & Architecture Improvements

### 1. Metrics & Observability

**Recommendation:** Add comprehensive metrics

**Metrics to Track:**
- Processing latency (p50, p95, p99)
- Throughput (requests/second)
- Error rates
- Memory usage
- GC pause times
- Cache hit/miss rates
- Database operation latencies

**Implementation:**
```go
type Metrics struct {
    ProcessingLatency prometheus.Histogram
    ErrorRate        prometheus.Counter
    Throughput       prometheus.Counter
    MemoryUsage      prometheus.Gauge
    GCPauseTime      prometheus.Histogram
}
```

**Integration:**
- Prometheus for metrics
- Structured logging (zap, logrus)
- Distributed tracing (OpenTelemetry)

---

### 2. Context-Based Cancellation

**Current State:** No cancellation support  
**Recommendation:** Add context support throughout

**Implementation:**
```go
func (m *Manager) IdentifySpeaker(
    ctx context.Context,
    audioData []float32,
    sampleRate int,
) (*IdentifyResult, error) {
    // Check context cancellation
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    // Process with periodic cancellation checks
    // ...
}
```

**Benefits:**
- Better resource cleanup
- Timeout support
- Graceful shutdown

---

### 3. Error Wrapping & Structured Errors

**Current State:** Basic error types  
**Recommendation:** Enhanced error context

**Implementation:**
```go
type AudioError struct {
    Op     string
    Format string
    Err    error
    // Additional context
    SampleRate int
    Duration   time.Duration
}

func (e *AudioError) Error() string {
    return fmt.Sprintf("audio error in %s (format: %s): %v", e.Op, e.Format, e.Err)
}

func (e *AudioError) Unwrap() error {
    return e.Err
}
```

---

### 4. Configuration Validation

**Recommendation:** Comprehensive config validation

**Implementation:**
```go
func (c *Config) Validate() error {
    var errs []error
    
    if c.Audio.SampleRate <= 0 {
        errs = append(errs, fmt.Errorf("invalid sample rate: %d", c.Audio.SampleRate))
    }
    
    if c.Speaker.Threshold < 0 || c.Speaker.Threshold > 1 {
        errs = append(errs, fmt.Errorf("threshold must be 0-1: %f", c.Speaker.Threshold))
    }
    
    // ... more validations
    
    if len(errs) > 0 {
        return fmt.Errorf("config validation failed: %v", errs)
    }
    
    return nil
}
```

---

### 5. Benchmark Suite

**Recommendation:** Comprehensive benchmark suite

**Benchmarks Needed:**
- Audio resampling (different methods, sizes)
- Speaker identification (various database sizes)
- Diarization (different segment counts)
- Memory allocation patterns
- Concurrent access patterns

**Implementation:**
```go
func BenchmarkResampleLinear(b *testing.B) {
    // ...
}

func BenchmarkSpeakerSearch_1000(b *testing.B) {
    // Test with 1000 speakers
}

func BenchmarkSpeakerSearch_10000(b *testing.B) {
    // Test with 10000 speakers
}
```

---

## Implementation Priority & Roadmap

### Phase 1: High Impact, Low Effort (Quick Wins)

**Timeline: 2-4 weeks**

1. ✅ **Buffer Pooling** - Implement size-classed pools
2. ✅ **Slice Reuse** - Optimize existing allocations
3. ✅ **Atomic Counters** - Replace mutex-protected counters
4. ✅ **Config Validation** - Add comprehensive validation
5. ✅ **Metrics** - Add basic metrics

**Expected Impact:**
- 30-50% reduction in GC pressure
- Better observability
- More predictable performance

---

### Phase 2: High Impact, Medium Effort

**Timeline: 1-2 months**

1. ✅ **Database Migration** - Move from JSON to Pebble
2. ✅ **Sharded Database** - Implement sharding for speaker DB
3. ✅ **SIMD Resampling** - Integrate SIMD-optimized resampler
4. ✅ **Batch Processing** - Add batch embedding extraction
5. ✅ **ANN Indexing** - Implement HNSW for speaker search

**Expected Impact:**
- 10-100× faster speaker searches
- 2-5× faster database operations
- 4-8× faster resampling

---

### Phase 3: High Impact, High Effort

**Timeline: 2-3 months**

1. ✅ **Modern Clustering** - Implement VBx or E-SHARC
2. ✅ **GPU Support** - Add CUDA/TensorRT support
3. ✅ **Overlap Detection** - Add overlap handling
4. ✅ **Streaming Optimizations** - Adaptive buffering, chunked processing
5. ✅ **Quantization** - Support INT8 models

**Expected Impact:**
- 20-30% DER improvement
- 2-5× GPU speedup
- Better real-time performance

---

### Phase 4: Advanced Optimizations

**Timeline: 3-6 months**

1. ✅ **Custom SIMD** - Hand-optimize critical paths
2. ✅ **Advanced ANN** - Dual-branch HNSW, LID-based insertion
3. ✅ **Frame-wise Embeddings** - Geodesic interpolation
4. ✅ **Hybrid Storage** - Multi-tier caching
5. ✅ **Distributed Processing** - Scale across nodes

---

## Measurement & Validation

### Before Optimization

**Baseline Metrics:**
- Processing latency: p50, p95, p99
- Throughput: requests/second
- Memory usage: peak, average
- GC pause times
- CPU utilization
- Error rates

### After Each Phase

**Validation:**
- Compare metrics before/after
- Run regression tests
- Performance benchmarks
- Load testing

### Tools

- **Profiling:** `go tool pprof`
- **Tracing:** `go tool trace`
- **Benchmarks:** `go test -bench`
- **Memory:** `go tool pprof -alloc_space`
- **CPU:** `go tool pprof -cpu`

---

## References & Further Reading

### Audio Processing
- [go-audio-resampler](https://pkg.go.dev/github.com/tphakala/go-audio-resampler) - SIMD-optimized resampler
- [simd](https://pkg.go.dev/github.com/tphakala/simd) - SIMD primitives
- [Efficient FIR Filter Implementation with SIMD](https://thewolfsound.com/fir-filter-with-simd/)

### Speaker Recognition & ANN
- [HNSW Paper](https://arxiv.org/abs/1603.09320) - Hierarchical Navigable Small World
- [Dual-Branch HNSW](https://arxiv.org/abs/2501.13992) - 2025 improvements
- [MN-RU Algorithm](https://www.catalyzex.com/paper/enhancing-hnsw-index-for-real-time-updates) - Real-time updates

### Diarization
- [E-SHARC](https://arxiv.org/abs/2401.12850) - End-to-End Supervised Hierarchical Clustering
- [MS-VBx](https://arxiv.org/abs/2305.13580) - Multi-Stream VBx
- [SC-pNA](https://arxiv.org/abs/2410.00023) - Self-Tuning Spectral Clustering
- [Frame-wise Embeddings](https://arxiv.org/abs/2401.03963) - Geodesic interpolation

### Go Performance
- [Go Memory Optimization](https://dev.to/nithinbharathwaj/go-memory-optimization-strategies)
- [sync.Pool Best Practices](https://leapcell.io/blog/unlocking-efficiency-demystifying-go-s-sync-pool)
- [Concurrency Patterns](https://jsschools.com/golang/optimizing-go-concurrency-practical-techniques)

### Databases
- [Pebble](https://github.com/cockroachdb/pebble) - Embedded key-value store
- [NutsDB](https://github.com/nutsdb/nutsdb) - Fast embedded database
- [Pebble vs RocksDB](https://www.cockroachlabs.com/blog/pebble-rocksdb-kv-store/)

### Sherpa-ONNX
- [Sherpa-ONNX Performance](https://k2-fsa.github.io/sherpa/triton/trt/index.html)
- [TensorRT Optimization](https://k2-fsa.github.io/sherpa/triton/trt/index.html)

---

## ASR Streaming Optimizations

### 1. Real-Time Streaming Architecture

**Current State:** Sherpa offline ASR is implemented as `types.Transcriber`, and Sherpa online ASR remains available as the streaming `ASRService`. Streaming now keeps a native Sherpa online stream per VoiceKit session and calls `InputFinished` only during endpoint/finalization cleanup. Latency depends on the configured Sherpa model, runtime provider, hardware, and chunking strategy.

**Current Characteristics:**
- **Runtime**: Sherpa ONNX offline and online recognizers through CGO bindings
- **Default state**: ASR disabled unless backend-specific model paths are configured
- **Architecture**: Offline transcription via `Transcriber`; streaming via cache-aware session buffering and explicit model registry
- **Lifecycle**: Session-owned native streams are closed on finalization, explicit removal, timeout cleanup, and service close
- **VAD**: Explicit `none`, `energy`, Sherpa Silero, and Sherpa TEN providers

**Key Optimizations:**
- **Streaming Session Management**: Efficient state management for multiple concurrent streams
- **Audio Buffer Pooling**: Zero-allocation audio buffering with sync.Pool
- **VAD Integration**: Provider seam for energy and Sherpa-backed VAD
- **Model Selection**: Dynamic model selection based on language and requirements

### 2. Model Performance Comparison

**Implemented Runtime:**
- **Sherpa Online Transducer**: `tokens.txt`, encoder, decoder, and joiner paths
- **Sherpa Online CTC**: `tokens.txt`, single model path, and model type (`zipformer2_ctc`, `nemo_ctc`, or `tone_ctc`)
- **Sherpa Offline**: transducer, Paraformer, Zipformer CTC, NeMo CTC, SenseVoice, and Whisper config families
- **Fake Model**: Explicit test/demo fake only; never registered by default

**Performance Work Still Needed:**
- Benchmark real Sherpa models on target CPU/GPU/CoreML providers
- Measure first-token and chunk latency per model family
- Add model-specific throughput tests and realistic audio fixtures
- Define an accuracy benchmark before publishing WER claims
- Keep `DecodeStreams` and recognizer pools out of production paths until benchmarks prove they improve throughput without lifecycle regressions

## TTS Synthesis Optimizations

### 1. Offline Synthesis Architecture

**Current State:** Sherpa offline TTS is implemented as `types.SpeechSynthesizer` through `tts.SherpaOfflineSynthesizer`. TTS is disabled by default and only initializes when a family-specific model config is provided.

**Current Characteristics:**
- **Runtime**: Sherpa ONNX offline TTS through CGO bindings
- **Default state**: TTS disabled unless required model paths and data directories are configured
- **Architecture**: Family-specific config blocks for VITS, Matcha, Kokoro, KittenTTS, ZipVoice, Pocket, and Supertonic
- **Output**: Normalized float32 PCM samples plus sample rate and duration metadata

**Performance Work Still Needed:**
- Benchmark synthesis latency by model family, speaker ID, provider, and text length
- Measure memory pressure for longer text inputs and multi-sentence generation
- Decide whether WAV encoding belongs in VoiceKit or in caller-owned application code
- Add real-model integration tests when local TTS model paths are available

### 3. Runtime Optimizations

**Scalability Features To Harden:**
- **Session Management**: Automatic cleanup of idle/expired sessions
- **Resource Pooling**: Evaluate shared VAD/model pools after native model benchmarks; speaker embedding streams remain single-use after `InputFinished`
- **Load Balancing**: Out of scope for this library; belongs in service integration
- **Monitoring**: Metrics interfaces exist but need production collector wiring

**Expected Impact:**
- **Latency**: Measurable real-time transcription once models and hardware are fixed
- **Scalability**: Bounded by native model memory, provider, and stream concurrency
- **Accuracy**: Must be benchmarked per model and language
- **Reliability**: Requires integration-level lifecycle, monitoring, and deployment work

### 4. Go 1.26.4 Runtime Baseline

VoiceKit now targets Go 1.26.4. Go 1.26's runtime improvements are relevant to this codebase because Sherpa-backed paths use CGO and the audio/speaker/indexing paths allocate heavily enough for GC behavior to matter.

**Measurement Policy:**
- Treat Go 1.26.4 as the baseline for new benchmark reports.
- Do not rewrite benchmark expectations from old Go 1.25 results without rerunning them.
- Keep model-backed benchmark claims gated on explicit local model/audio assets.

---

## Conclusion

This document provides a comprehensive roadmap for optimizing VoiceKit. The recommendations are based on:

1. **Latest checked research** for audio processing, speaker recognition, diarization, and Sherpa-backed voice inference
2. **Production best practices** for Go 1.26.4 performance optimization
3. **Modern database solutions** for embedding storage
4. **Real-world performance data** from similar systems

**Key Takeaways:**
- Start with Phase 1 (quick wins) for immediate impact
- Prioritize based on your specific use case and constraints
- Measure before and after each optimization
- Consider trade-offs (complexity vs performance vs accuracy)

**Expected Overall Impact:**
- **Performance:** 5-50× improvement in critical paths
- **Scalability:** Support for 10-100× more speakers
- **Latency:** 50-80% reduction in processing time
- **Memory:** 50-85% reduction in GC pressure
- **Accuracy:** 15-30% improvement in diarization DER

For questions or clarifications, refer to the specific sections or the referenced research papers and libraries.
