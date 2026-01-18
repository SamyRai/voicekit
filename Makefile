# VoiceKit Benchmarking Makefile

.PHONY: bench bench-all bench-report bench-memory bench-load bench-compare

# Run all benchmarks
bench-all:
	@echo "🚀 Running Complete VoiceKit Benchmark Suite..."
	@mkdir -p benchmarks
	@go test -bench=. -benchmem -run=^$ ./audio ./speaker ./diarization . | tee benchmarks/raw_results_$(shell date +%Y%m%d_%H%M%S).txt
	@echo "✅ Benchmark Suite Complete"

# Run audio benchmarks only
bench-audio:
	@echo "🎵 Running Audio Processing Benchmarks..."
	@go test -bench=. -benchmem -run=^$ ./audio

# Run speaker benchmarks only
bench-speaker:
	@echo "🎤 Running Speaker Recognition Benchmarks..."
	@go test -bench=. -benchmem -run=^$ ./speaker

# Run diarization benchmarks only
bench-diarization:
	@echo "🎭 Running Diarization Benchmarks..."
	@go test -bench=. -benchmem -run=^$ ./diarization

# Run memory-specific benchmarks
bench-memory:
	@echo "🧠 Running Memory Benchmarks..."
	@go test -bench=BufferPool -benchmem -run=^$ ./audio

# Run load testing simulation
bench-load:
	@echo "🔥 Running Load Test Simulation..."
	@go test -bench=Concurrent -benchmem -run=^$ .

# Generate benchmark report
bench-report:
	@echo "📊 Generating Benchmark Report..."
	@mkdir -p benchmarks
	@go run benchmark_runner.go -report > benchmarks/benchmark_report_$(shell date +%Y%m%d_%H%M%S).md
	@echo "✅ Report Generated"

# Compare with baseline (requires baseline results)
bench-compare:
	@echo "📈 Running Performance Comparison..."
	@mkdir -p benchmarks
	@go run benchmark_runner.go -compare > benchmarks/comparison_report_$(shell date +%Y%m%d_%H%M%S).md
	@echo "✅ Comparison Report Generated"

# Run benchmarks with CPU profiling
bench-profile:
	@echo "🔍 Running Benchmarks with CPU Profiling..."
	@mkdir -p profiles
	@go test -bench=. -benchmem -cpuprofile=profiles/cpu.prof -run=^$ ./audio ./speaker ./diarization .
	@echo "✅ CPU Profile Generated: profiles/cpu.prof"

# Run benchmarks with memory profiling
bench-memprofile:
	@echo "🔍 Running Benchmarks with Memory Profiling..."
	@mkdir -p profiles
	@go test -bench=. -benchmem -memprofile=profiles/mem.prof -run=^$ ./audio ./speaker ./diarization .
	@echo "✅ Memory Profile Generated: profiles/mem.prof"

# Clean benchmark artifacts
bench-clean:
	@echo "🧹 Cleaning Benchmark Artifacts..."
	@rm -rf benchmarks/
	@rm -rf profiles/
	@echo "✅ Cleaned"

# Run comprehensive benchmarking suite
bench-comprehensive: bench-clean
	@echo "🎯 Running Comprehensive Benchmark Suite..."
	@mkdir -p benchmarks profiles
	@echo "Step 1: Audio Benchmarks..."
	@go test -bench=. -benchmem -run=^$ ./audio > benchmarks/audio_results.txt
	@echo "Step 2: Speaker Benchmarks..."
	@go test -bench=. -benchmem -run=^$ ./speaker > benchmarks/speaker_results.txt
	@echo "Step 3: Diarization Benchmarks..."
	@go test -bench=. -benchmem -run=^$ ./diarization > benchmarks/diarization_results.txt
	@echo "Step 4: Memory Benchmarks..."
	@go test -bench=BufferPool -benchmem -run=^$ ./audio > benchmarks/memory_results.txt
	@echo "Step 5: Concurrent Benchmarks..."
	@go test -bench=Concurrent -benchmem -run=^$ . > benchmarks/concurrent_results.txt
	@echo "Step 6: Generating Summary Report..."
	@echo "# VoiceKit Performance Benchmark Results" > benchmarks/COMPREHENSIVE_REPORT.md
	@echo "Generated: $$(date)" >> benchmarks/COMPREHENSIVE_REPORT.md
	@echo "" >> benchmarks/COMPREHENSIVE_REPORT.md
	@echo "## Audio Processing Results" >> benchmarks/COMPREHENSIVE_REPORT.md
	@cat benchmarks/audio_results.txt >> benchmarks/COMPREHENSIVE_REPORT.md
	@echo "" >> benchmarks/COMPREHENSIVE_REPORT.md
	@echo "## Speaker Recognition Results" >> benchmarks/COMPREHENSIVE_REPORT.md
	@cat benchmarks/speaker_results.txt >> benchmarks/COMPREHENSIVE_REPORT.md
	@echo "" >> benchmarks/COMPREHENSIVE_REPORT.md
	@echo "## Diarization Results" >> benchmarks/COMPREHENSIVE_REPORT.md
	@cat benchmarks/diarization_results.txt >> benchmarks/COMPREHENSIVE_REPORT.md
	@echo "" >> benchmarks/COMPREHENSIVE_REPORT.md
	@echo "## Memory Performance" >> benchmarks/COMPREHENSIVE_REPORT.md
	@cat benchmarks/memory_results.txt >> benchmarks/COMPREHENSIVE_REPORT.md
	@echo "" >> benchmarks/COMPREHENSIVE_REPORT.md
	@echo "## Concurrent Performance" >> benchmarks/COMPREHENSIVE_REPORT.md
	@cat benchmarks/concurrent_results.txt >> benchmarks/COMPREHENSIVE_REPORT.md
	@echo "✅ Comprehensive Benchmark Suite Complete"
	@echo "📊 Results saved to: benchmarks/COMPREHENSIVE_REPORT.md"

# Help target
bench-help:
	@echo "VoiceKit Benchmarking Targets:"
	@echo "  bench-all          - Run all benchmarks"
	@echo "  bench-audio        - Run audio processing benchmarks only"
	@echo "  bench-speaker      - Run speaker recognition benchmarks only"
	@echo "  bench-diarization  - Run diarization benchmarks only"
	@echo "  bench-memory       - Run memory-specific benchmarks"
	@echo "  bench-load         - Run load testing simulation"
	@echo "  bench-profile      - Run benchmarks with CPU profiling"
	@echo "  bench-memprofile   - Run benchmarks with memory profiling"
	@echo "  bench-report       - Generate benchmark report"
	@echo "  bench-compare      - Compare with baseline results"
	@echo "  bench-comprehensive- Run full benchmarking suite with reports"
	@echo "  bench-clean        - Clean benchmark artifacts"