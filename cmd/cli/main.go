package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"livekit-agents-go/cmd/cli/cmd"
)

var (
	verbose   bool
	envFile   string
	outputDir string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pipeline-test",
	Short: "LiveKit Agents Go Pipeline Testing Tool",
	Long: `A comprehensive testing tool for LiveKit Agents Go pipeline components.

This tool allows you to test individual services (VAD, STT, LLM, TTS), 
service pipelines, and full agent integration in isolation to debug
and validate the audio processing pipeline.

Examples:
  pipeline-test                                    # Interactive menu mode
  pipeline-test audio --duration 10s --loopback   # Test audio I/O
  pipeline-test vad --duration 30s                # Test VAD speech detection
  pipeline-test pipeline --services vad,stt       # Test service chain`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand provided, show interactive menu
		if len(args) == 0 {
			showInteractiveMenu()
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&envFile, "env", ".env", "environment file to load")
	rootCmd.PersistentFlags().StringVar(&outputDir, "output-dir", "./test-results", "directory to save test outputs")

	// Add subcommands
	rootCmd.AddCommand(cmd.NewAudioCmd())
	rootCmd.AddCommand(cmd.NewAECTestCmd())
	rootCmd.AddCommand(cmd.NewAECPipelineCmd())
	rootCmd.AddCommand(cmd.NewAECTTSTestCmd())
	rootCmd.AddCommand(cmd.NewSTTDiagnosticCmd())
	rootCmd.AddCommand(cmd.NewAECAudioDiagnosticCmd())
	rootCmd.AddCommand(cmd.NewAECEffectivenessTestCmd())
	rootCmd.AddCommand(cmd.NewChatCLIAudioDebugCmd())
	rootCmd.AddCommand(cmd.NewEchoMeasurementCmd())
	rootCmd.AddCommand(cmd.NewDelayMeasurementCmd())
	rootCmd.AddCommand(cmd.NewDelayMeasurementV2Cmd())
	rootCmd.AddCommand(cmd.NewVADCmd())
	rootCmd.AddCommand(cmd.NewSTTCmd())
	rootCmd.AddCommand(cmd.NewLLMCmd())
	rootCmd.AddCommand(cmd.NewTTSCmd())
	rootCmd.AddCommand(cmd.NewPipelineCmd())
	rootCmd.AddCommand(cmd.NewBenchmarkCmd())
	rootCmd.AddCommand(cmd.NewAgentCmd())
}

// initConfig reads in config file and ENV variables if set
func initConfig() {
	// Load environment file
	if envFile != "" {
		// Try relative to current directory first
		if err := godotenv.Load(envFile); err != nil {
			// Try relative to project root
			projectRoot := findProjectRoot()
			if projectRoot != "" {
				envPath := filepath.Join(projectRoot, envFile)
				if err := godotenv.Load(envPath); err != nil {
					if verbose {
						fmt.Printf("Warning: Could not load env file %s: %v\n", envFile, err)
					}
				} else if verbose {
					fmt.Printf("Loaded environment from: %s\n", envPath)
				}
			}
		} else if verbose {
			fmt.Printf("Loaded environment from: %s\n", envFile)
		}
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil && verbose {
		fmt.Printf("Warning: Could not create output directory %s: %v\n", outputDir, err)
	}
}

// findProjectRoot looks for the project root by finding go.mod
func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached root
		}
		dir = parent
	}
	
	return ""
}

// showInteractiveMenu displays the interactive menu when no arguments are provided
func showInteractiveMenu() {
	fmt.Println(`
=== LiveKit Agents Go Pipeline Testing Tool ===

🔊 AUDIO TESTS
1. audio loopback     - Test microphone → speaker loopback
2. audio devices      - Show available audio devices
3. audio basic        - Basic audio I/O test

🎤 SERVICE TESTS
4. vad               - Test VAD (Silero) speech detection  
5. stt               - Test STT (OpenAI Whisper) transcription
6. llm               - Test LLM (OpenAI GPT) responses
7. tts               - Test TTS (OpenAI TTS) synthesis

🔗 PIPELINE TESTS
8. pipeline vad-stt  - Test VAD → STT chain
9. pipeline stt-llm  - Test STT → LLM chain  
10. pipeline llm-tts - Test LLM → TTS chain
11. pipeline full    - Test complete VAD → STT → LLM → TTS

🤖 AGENT TESTS
12. agent basic      - Test basic agent integration
13. agent functions  - Test agent with function calling

Enter test number or use commands like:
  pipeline-test audio --help
  pipeline-test vad --duration 30s
  pipeline-test pipeline --services vad,stt,llm,tts

`)

	// TODO: Implement interactive selection
	fmt.Println("Interactive mode not yet implemented. Use subcommands for now.")
	fmt.Println("Example: pipeline-test audio --help")
}

func main() {
	Execute()
}