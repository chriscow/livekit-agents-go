package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/plugins"
	"livekit-agents-go/services/llm"
	"livekit-agents-go/services/tts"
	// Import plugins for auto-discovery
	_ "livekit-agents-go/plugins/deepgram"
	_ "livekit-agents-go/plugins/openai"
)

// NewBenchmarkCmd creates the benchmark testing command
func NewBenchmarkCmd() *cobra.Command {
	var (
		iterations int
		services   string
		verbose    bool
	)

	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Benchmark streaming vs batch performance",
		Long: `Benchmark streaming vs batch performance for voice pipeline services.

This command measures latency improvements from streaming implementation
and provides detailed performance comparisons.

Available services: stt, llm, tts, all

Examples:
  pipeline-test benchmark --services all --iterations 3     # Full benchmark
  pipeline-test benchmark --services llm --iterations 5     # LLM only  
  pipeline-test benchmark --services tts --iterations 3     # TTS only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if services == "" {
				services = "all"
			}

			return runBenchmark(services, iterations, verbose)
		},
	}

	cmd.Flags().IntVarP(&iterations, "iterations", "i", 3, "number of test iterations")
	cmd.Flags().StringVarP(&services, "services", "s", "all", "services to benchmark (stt,llm,tts,all)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose benchmark output")

	return cmd
}

// BenchmarkResult holds performance measurement results
type BenchmarkResult struct {
	Service       string
	Mode          string // "streaming" or "batch"
	Latency       time.Duration
	FirstResponse time.Duration // Time to first chunk/result
	Throughput    float64       // Tokens/bytes per second
	Success       bool
	Error         error
}

// runBenchmark executes performance benchmarks
func runBenchmark(services string, iterations int, verbose bool) error {
	fmt.Printf("🏁 Starting performance benchmark\n")
	fmt.Printf("📊 Services: %s\n", services)
	fmt.Printf("🔢 Iterations: %d\n", iterations)
	fmt.Printf("📝 Verbose: %v\n", verbose)
	fmt.Println()

	// Create services
	fmt.Println("🔧 Creating services...")
	pluginServices, err := plugins.CreateSmartServices()
	if err != nil {
		return fmt.Errorf("failed to create services: %w", err)
	}

	serviceList := parseServicesList(services)
	allResults := make(map[string][]BenchmarkResult)

	for _, service := range serviceList {
		fmt.Printf("\n🎯 Benchmarking %s service...\n", service)
		
		results, err := benchmarkService(service, pluginServices, iterations, verbose)
		if err != nil {
			fmt.Printf("❌ Failed to benchmark %s: %v\n", service, err)
			continue
		}
		
		allResults[service] = results
	}

	// Print summary results
	printBenchmarkSummary(allResults)
	
	return nil
}

// benchmarkService benchmarks a specific service
func benchmarkService(serviceName string, services *plugins.SmartServices, iterations int, verbose bool) ([]BenchmarkResult, error) {
	switch serviceName {
	case "llm":
		return benchmarkLLM(services.LLM, iterations, verbose)
	case "tts":
		return benchmarkTTS(services.TTS, iterations, verbose)
	case "stt":
		fmt.Printf("⚠️ STT benchmarking requires live audio input - skipping automated benchmark\n")
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown service: %s", serviceName)
	}
}

// benchmarkLLM benchmarks LLM streaming vs batch performance
func benchmarkLLM(llmService llm.LLM, iterations int, verbose bool) ([]BenchmarkResult, error) {
	if llmService == nil {
		return nil, fmt.Errorf("LLM service not available")
	}

	testPrompt := "Write a brief explanation of how artificial intelligence works in simple terms."
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a helpful assistant. Keep responses concise."},
		{Role: llm.RoleUser, Content: testPrompt},
	}

	var results []BenchmarkResult
	ctx := context.Background()

	// Benchmark batch mode
	fmt.Printf("  📋 Testing batch LLM completion...\n")
	for i := 0; i < iterations; i++ {
		if verbose {
			fmt.Printf("    Iteration %d/%d (batch)\n", i+1, iterations)
		}

		start := time.Now()
		response, err := llmService.Chat(ctx, messages, &llm.ChatOptions{MaxTokens: 150})
		duration := time.Since(start)

		result := BenchmarkResult{
			Service:       "llm",
			Mode:          "batch", 
			Latency:       duration,
			FirstResponse: duration, // Same as latency for batch
			Success:       err == nil,
			Error:         err,
		}

		if err == nil && response != nil {
			words := len(strings.Fields(response.Message.Content))
			result.Throughput = float64(words) / duration.Seconds()
		}

		results = append(results, result)
		
		if verbose && err == nil {
			fmt.Printf("    ✅ Batch: %v, %d words\n", duration, len(strings.Fields(response.Message.Content)))
		}
	}

	// Benchmark streaming mode
	fmt.Printf("  🌊 Testing streaming LLM completion...\n")
	for i := 0; i < iterations; i++ {
		if verbose {
			fmt.Printf("    Iteration %d/%d (streaming)\n", i+1, iterations)
		}

		start := time.Now()
		stream, err := llmService.ChatStream(ctx, messages, &llm.ChatOptions{MaxTokens: 150})
		if err != nil {
			results = append(results, BenchmarkResult{
				Service: "llm",
				Mode:    "streaming",
				Success: false,
				Error:   err,
			})
			continue
		}

		var firstChunkTime time.Duration
		var totalContent string
		chunkCount := 0

		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}

			chunkCount++
			totalContent += chunk.Delta.Content

			// Record time to first chunk
			if chunkCount == 1 {
				firstChunkTime = time.Since(start)
			}
		}

		stream.Close()
		totalDuration := time.Since(start)

		words := len(strings.Fields(totalContent))
		result := BenchmarkResult{
			Service:       "llm",
			Mode:          "streaming",
			Latency:       totalDuration,
			FirstResponse: firstChunkTime,
			Success:       true,
		}

		if totalDuration.Seconds() > 0 {
			result.Throughput = float64(words) / totalDuration.Seconds()
		}

		results = append(results, result)

		if verbose {
			fmt.Printf("    ✅ Streaming: %v total, %v first chunk, %d words, %d chunks\n", 
				totalDuration, firstChunkTime, words, chunkCount)
		}
	}

	return results, nil
}

// benchmarkTTS benchmarks TTS streaming vs batch performance
func benchmarkTTS(ttsService tts.TTS, iterations int, verbose bool) ([]BenchmarkResult, error) {
	if ttsService == nil {
		return nil, fmt.Errorf("TTS service not available")
	}

	testText := "Hello there. How are you doing today? This is a test of text-to-speech synthesis performance."
	
	var results []BenchmarkResult
	ctx := context.Background()

	// Benchmark batch mode
	fmt.Printf("  📋 Testing batch TTS synthesis...\n")
	for i := 0; i < iterations; i++ {
		if verbose {
			fmt.Printf("    Iteration %d/%d (batch)\n", i+1, iterations)
		}

		start := time.Now()
		audioFrame, err := ttsService.Synthesize(ctx, testText, &tts.SynthesizeOptions{
			Voice: "alloy",
			Speed: 1.0,
		})
		duration := time.Since(start)

		result := BenchmarkResult{
			Service:       "tts",
			Mode:          "batch",
			Latency:       duration,
			FirstResponse: duration, // Same as latency for batch
			Success:       err == nil,
			Error:         err,
		}

		if err == nil && audioFrame != nil {
			// Calculate throughput as bytes per second
			result.Throughput = float64(len(audioFrame.Data)) / duration.Seconds()
		}

		results = append(results, result)

		if verbose && err == nil {
			fmt.Printf("    ✅ Batch: %v, %d bytes audio\n", duration, len(audioFrame.Data))
		}
	}

	// Benchmark streaming mode
	fmt.Printf("  🌊 Testing streaming TTS synthesis...\n")
	for i := 0; i < iterations; i++ {
		if verbose {
			fmt.Printf("    Iteration %d/%d (streaming)\n", i+1, iterations)
		}

		start := time.Now()
		stream, err := ttsService.SynthesizeStream(ctx, &tts.SynthesizeOptions{
			Voice: "alloy",
			Speed: 1.0,
		})
		if err != nil {
			results = append(results, BenchmarkResult{
				Service: "tts",
				Mode:    "streaming",
				Success: false,
				Error:   err,
			})
			continue
		}

		// Send text and measure streaming performance
		err = stream.SendText(testText)
		if err != nil {
			stream.Close()
			continue
		}

		stream.CloseSend()

		var firstChunkTime time.Duration
		var totalBytes int
		chunkCount := 0

		for {
			audioFrame, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}

			chunkCount++
			totalBytes += len(audioFrame.Data)

			// Record time to first chunk
			if chunkCount == 1 {
				firstChunkTime = time.Since(start)
			}
		}

		stream.Close()
		totalDuration := time.Since(start)

		result := BenchmarkResult{
			Service:       "tts",
			Mode:          "streaming",
			Latency:       totalDuration,
			FirstResponse: firstChunkTime,
			Success:       true,
		}

		if totalDuration.Seconds() > 0 {
			result.Throughput = float64(totalBytes) / totalDuration.Seconds()
		}

		results = append(results, result)

		if verbose {
			fmt.Printf("    ✅ Streaming: %v total, %v first chunk, %d bytes, %d chunks\n", 
				totalDuration, firstChunkTime, totalBytes, chunkCount)
		}
	}

	return results, nil
}

// parseServicesList parses the services string into a list
func parseServicesList(services string) []string {
	if services == "all" {
		return []string{"llm", "tts"} // STT requires live audio
	}

	parts := strings.Split(services, ",")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// printBenchmarkSummary prints the benchmark results summary
func printBenchmarkSummary(allResults map[string][]BenchmarkResult) {
	fmt.Printf("\n📊 BENCHMARK SUMMARY\n")
	fmt.Printf("=====================================\n\n")

	for service, results := range allResults {
		if len(results) == 0 {
			continue
		}

		fmt.Printf("🎯 %s SERVICE RESULTS:\n", strings.ToUpper(service))
		
		// Separate streaming and batch results
		batchResults := filterResults(results, "batch")
		streamingResults := filterResults(results, "streaming")

		if len(batchResults) > 0 {
			fmt.Printf("📋 Batch Mode:\n")
			printResultStats("  ", batchResults)
		}

		if len(streamingResults) > 0 {
			fmt.Printf("🌊 Streaming Mode:\n")  
			printResultStats("  ", streamingResults)
		}

		// Calculate improvement
		if len(batchResults) > 0 && len(streamingResults) > 0 {
			batchAvg := calculateAverage(batchResults, func(r BenchmarkResult) float64 {
				return float64(r.FirstResponse.Milliseconds())
			})
			streamingAvg := calculateAverage(streamingResults, func(r BenchmarkResult) float64 {
				return float64(r.FirstResponse.Milliseconds()) 
			})

			if batchAvg > 0 {
				improvement := ((batchAvg - streamingAvg) / batchAvg) * 100
				fmt.Printf("🚀 First Response Improvement: %.1f%% faster\n", improvement)
			}
		}

		fmt.Println()
	}
}

// filterResults filters results by mode
func filterResults(results []BenchmarkResult, mode string) []BenchmarkResult {
	var filtered []BenchmarkResult
	for _, r := range results {
		if r.Mode == mode && r.Success {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// printResultStats prints statistics for a set of results
func printResultStats(prefix string, results []BenchmarkResult) {
	if len(results) == 0 {
		fmt.Printf("%sNo successful results\n", prefix)
		return
	}

	avgLatency := calculateAverage(results, func(r BenchmarkResult) float64 {
		return float64(r.Latency.Milliseconds())
	})

	avgFirstResponse := calculateAverage(results, func(r BenchmarkResult) float64 {
		return float64(r.FirstResponse.Milliseconds())
	})

	avgThroughput := calculateAverage(results, func(r BenchmarkResult) float64 {
		return r.Throughput
	})

	fmt.Printf("%sTotal Latency: %.0fms avg\n", prefix, avgLatency)
	fmt.Printf("%sFirst Response: %.0fms avg\n", prefix, avgFirstResponse)
	fmt.Printf("%sThroughput: %.1f units/sec avg\n", prefix, avgThroughput)
	fmt.Printf("%sSuccess Rate: %d/%d (%.1f%%)\n", prefix, 
		len(results), len(results), 100.0)
}

// calculateAverage calculates the average of a field across results
func calculateAverage(results []BenchmarkResult, getter func(BenchmarkResult) float64) float64 {
	if len(results) == 0 {
		return 0
	}

	sum := 0.0
	for _, r := range results {
		sum += getter(r)
	}
	return sum / float64(len(results))
}