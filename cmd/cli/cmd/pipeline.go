package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// NewPipelineCmd creates the pipeline testing command
func NewPipelineCmd() *cobra.Command {
	var (
		services  string
		duration  time.Duration
		verbose   bool
		saveAudio bool
		streaming bool
	)

	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Test service pipelines (chains of VAD, STT, LLM, TTS)",
		Long: `Test chains of services to validate the complete voice pipeline.

This command tests how services work together in sequence, which is critical
for finding where the audio processing breaks in the agent pipeline.

Available services: vad, stt, llm, tts

Examples:
  pipeline-test pipeline --services vad,stt                # Test VAD → STT chain
  pipeline-test pipeline --services stt,llm               # Test STT → LLM chain  
  pipeline-test pipeline --services llm,tts               # Test LLM → TTS chain
  pipeline-test pipeline --services vad,stt,llm,tts       # Test full pipeline
  pipeline-test pipeline --services stt,llm,tts --streaming # Test streaming pipeline`,
		RunE: func(cmd *cobra.Command, args []string) error {
			serviceList := parseServices(services)
			if len(serviceList) == 0 {
				return fmt.Errorf("no services specified. Use --services flag with: vad, stt, llm, tts")
			}

			return runPipelineTest(serviceList, duration, verbose, saveAudio, streaming)
		},
	}

	cmd.Flags().StringVarP(&services, "services", "s", "", "comma-separated list of services (vad,stt,llm,tts)")
	cmd.Flags().DurationVarP(&duration, "duration", "d", 60*time.Second, "test duration")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose pipeline logging")
	cmd.Flags().BoolVar(&saveAudio, "save-audio", false, "save intermediate audio to files")
	cmd.Flags().BoolVar(&streaming, "streaming", false, "use streaming mode for applicable services")

	// Make services flag required
	cmd.MarkFlagRequired("services")

	return cmd
}

// parseServices parses comma-separated service names
func parseServices(servicesStr string) []string {
	if servicesStr == "" {
		return nil
	}

	services := strings.Split(servicesStr, ",")
	var validServices []string

	for _, service := range services {
		service = strings.TrimSpace(strings.ToLower(service))
		switch service {
		case "vad", "stt", "llm", "tts":
			validServices = append(validServices, service)
		default:
			fmt.Printf("⚠️  Unknown service: %s (ignoring)\n", service)
		}
	}

	return validServices
}

// runPipelineTest tests a chain of services
func runPipelineTest(services []string, duration time.Duration, verbose, saveAudio, streaming bool) error {
	fmt.Printf("🔗 Testing pipeline: %s\n", strings.Join(services, " → "))
	fmt.Printf("⏱️  Duration: %v\n", duration)
	fmt.Printf("📝 Verbose: %v\n", verbose)
	fmt.Printf("💾 Save audio: %v\n", saveAudio)
	fmt.Printf("🌊 Streaming: %v\n", streaming)
	fmt.Println()

	// Validate service chain
	if err := validateServiceChain(services); err != nil {
		return err
	}

	fmt.Println("🔧 Pipeline validation:")
	for i, service := range services {
		description := getServiceDescription(service)
		if streaming && isStreamingCapable(service) {
			description += " (STREAMING MODE)"
		}
		fmt.Printf("  %d. %s\n", i+1, description)
	}
	fmt.Println()

	// For now, run a simple demonstration of pipeline capability
	fmt.Printf("✅ Pipeline validation passed - %d services configured\n", len(services))
	
	if streaming {
		fmt.Println("🌊 Streaming mode enabled for applicable services:")
		for _, service := range services {
			if isStreamingCapable(service) {
				fmt.Printf("  ✅ %s: streaming supported\n", service)
			} else {
				fmt.Printf("  ⚠️  %s: batch mode only\n", service)
			}
		}
	}
	
	fmt.Println()
	fmt.Println("🎯 Pipeline testing framework ready!")
	fmt.Println("Note: Full pipeline implementation would chain these services")
	fmt.Println("      to test end-to-end audio processing flow.")
	
	return nil
}

// validateServiceChain validates that the service chain makes sense
func validateServiceChain(services []string) error {
	for i := 0; i < len(services)-1; i++ {
		current := services[i]
		next := services[i+1]
		
		// Check for logical flow
		switch {
		case current == "vad" && next != "stt":
			return fmt.Errorf("VAD should typically be followed by STT, not %s", next)
		case current == "stt" && next != "llm":
			return fmt.Errorf("STT should typically be followed by LLM, not %s", next)
		case current == "llm" && next != "tts":
			return fmt.Errorf("LLM should typically be followed by TTS, not %s", next)
		case current == "tts":
			return fmt.Errorf("TTS should be the final service in the chain")
		}
	}
	
	return nil
}

// getServiceDescription returns a description of what each service does
func getServiceDescription(service string) string {
	switch service {
	case "vad":
		return "VAD (Voice Activity Detection) - Detects when speech is present"
	case "stt":
		return "STT (Speech-to-Text) - Converts speech audio to text"
	case "llm":
		return "LLM (Large Language Model) - Generates responses to text input"
	case "tts":
		return "TTS (Text-to-Speech) - Converts text responses to audio"
	default:
		return "Unknown service"
	}
}

// isStreamingCapable checks if a service supports streaming mode
func isStreamingCapable(service string) bool {
	switch service {
	case "stt":
		return true  // Deepgram supports real-time streaming
	case "llm":
		return true  // OpenAI GPT supports streaming completion
	case "tts":
		return true  // OpenAI TTS supports streaming synthesis
	case "vad":
		return false // VAD is inherently real-time but not "streaming" in the same sense
	default:
		return false
	}
}