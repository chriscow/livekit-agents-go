package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"livekit-agents-go/audio"
	"livekit-agents-go/examples/audio-test/tests"

	// Import plugins for testing
	_ "livekit-agents-go/plugins/openai"
)

const (
	AppName    = "LiveKit Audio Test Suite"
	AppVersion = "1.0.0"
)

type TestResult struct {
	Name     string
	Passed   bool
	Duration time.Duration
	Details  string
	Error    error
}

type TestSuite struct {
	results []TestResult
	scanner *bufio.Scanner
}

func main() {
	// Load environment variables from .env file (project root)
	envPath := filepath.Join("..", "..", ".env")
	if err := godotenv.Load(envPath); err != nil {
		// .env file is optional, continue without it
		fmt.Printf("💡 Note: .env file not found at %s (continuing with system env vars)\n", envPath)
	} else {
		fmt.Printf("✅ Loaded environment variables from %s\n", envPath)
	}
	
	fmt.Printf("=== %s v%s ===\n\n", AppName, AppVersion)
	
	suite := &TestSuite{
		results: make([]TestResult, 0),
		scanner: bufio.NewScanner(os.Stdin),
	}

	// Initialize PortAudio
	fmt.Println("🎵 Initializing PortAudio...")
	audioConfig := audio.DefaultConfig()
	audioIO, err := audio.NewLocalAudioIO(audioConfig)
	if err != nil {
		log.Fatalf("❌ Failed to initialize audio I/O: %v", err)
	}
	defer audioIO.Close()

	for {
		suite.showMainMenu()
		choice := suite.getUserInput("Select test (1-7, or 0 to exit): ")
		
		switch choice {
		case "0":
			suite.showSummary()
			fmt.Println("👋 Goodbye!")
			return
		case "1":
			suite.runDeviceDiscoveryTest(audioIO)
		case "2":
			suite.runMicrophoneTest(audioIO)
		case "3":
			suite.runSpeakerTest(audioIO)
		case "4":
			suite.runLoopbackTest(audioIO)
		case "5":
			suite.runPipelineTest()
		case "6":
			suite.runAllTests(audioIO)
		case "7":
			suite.showSummary()
		default:
			fmt.Println("❌ Invalid choice. Please try again.")
		}
		
		if choice != "0" && choice != "7" {
			suite.waitForKeypress()
		}
	}
}

func (ts *TestSuite) showMainMenu() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎧 AUDIO TEST SUITE")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("1. 🔍 Device Discovery Test")
	fmt.Println("2. 🎤 Microphone Test")
	fmt.Println("3. 🔊 Speaker Test")
	fmt.Println("4. 🔄 Loopback Test (Mic → Speaker)")
	fmt.Println("5. 🤖 Pipeline Integration Test")
	fmt.Println("6. 🚀 Run All Tests")
	fmt.Println("7. 📊 Show Test Summary")
	fmt.Println("0. 👋 Exit")
	fmt.Println(strings.Repeat("-", 60))
}

func (ts *TestSuite) getUserInput(prompt string) string {
	fmt.Print(prompt)
	if ts.scanner.Scan() {
		return strings.TrimSpace(ts.scanner.Text())
	}
	return ""
}

func (ts *TestSuite) waitForKeypress() {
	fmt.Print("\n⏸️  Press Enter to continue...")
	ts.scanner.Scan()
}

func (ts *TestSuite) recordResult(name string, passed bool, duration time.Duration, details string, err error) {
	result := TestResult{
		Name:     name,
		Passed:   passed,
		Duration: duration,
		Details:  details,
		Error:    err,
	}
	ts.results = append(ts.results, result)
	
	status := "✅ PASS"
	if !passed {
		status = "❌ FAIL"
	}
	
	fmt.Printf("%s - %s (%v)\n", status, name, duration.Round(time.Millisecond))
	if details != "" {
		fmt.Printf("   📝 %s\n", details)
	}
	if err != nil {
		fmt.Printf("   ⚠️  Error: %v\n", err)
	}
}

func (ts *TestSuite) runDeviceDiscoveryTest(audioIO *audio.LocalAudioIO) {
	fmt.Println("\n🔍 Running Device Discovery Test...")
	
	start := time.Now()
	err := tests.TestDeviceDiscovery(audioIO)
	duration := time.Since(start)
	
	passed := err == nil
	details := "Discovered and validated audio devices"
	
	ts.recordResult("Device Discovery", passed, duration, details, err)
}

func (ts *TestSuite) runMicrophoneTest(audioIO *audio.LocalAudioIO) {
	fmt.Println("\n🎤 Running Microphone Test...")
	fmt.Println("This test will capture audio from your microphone for 5 seconds.")
	fmt.Println("Please make some noise (speak, tap, etc.) during the test.")
	
	if ts.getUserInput("Ready to start? (y/N): ") != "y" {
		fmt.Println("Microphone test skipped.")
		return
	}
	
	start := time.Now()
	audioData, err := tests.TestMicrophoneCapture(audioIO, 5*time.Second)
	duration := time.Since(start)
	
	passed := err == nil && len(audioData) > 0
	details := fmt.Sprintf("Captured %d audio frames", len(audioData))
	
	ts.recordResult("Microphone Capture", passed, duration, details, err)
	
	if passed {
		// Test audio analysis
		energy := tests.CalculateAudioEnergy(audioData)
		fmt.Printf("   📊 Average audio energy: %.2f\n", energy)
		
		if energy > 0.001 {
			fmt.Println("   ✅ Audio energy detected - microphone working correctly")
		} else {
			fmt.Println("   ⚠️  Low audio energy - check microphone or make more noise")
		}
	}
}

func (ts *TestSuite) runSpeakerTest(audioIO *audio.LocalAudioIO) {
	fmt.Println("\n🔊 Running Speaker Test...")
	fmt.Println("This test will play test tones through your speakers.")
	
	if ts.getUserInput("Ready to start? (y/N): ") != "y" {
		fmt.Println("Speaker test skipped.")
		return
	}
	
	start := time.Now()
	err := tests.TestSpeakerPlayback(audioIO)
	duration := time.Since(start)
	
	// Ask user for verification
	fmt.Println("\nDid you hear the test tones clearly?")
	userResponse := ts.getUserInput("(y/N): ")
	passed := err == nil && userResponse == "y"
	
	details := "Played test tones at different frequencies"
	
	ts.recordResult("Speaker Playback", passed, duration, details, err)
}

func (ts *TestSuite) runLoopbackTest(audioIO *audio.LocalAudioIO) {
	fmt.Println("\n🔄 Running Loopback Test...")
	fmt.Println("This test captures audio from microphone and plays it back through speakers.")
	fmt.Println("You should hear your own voice with a slight delay.")
	
	if ts.getUserInput("Ready to start 10-second loopback test? (y/N): ") != "y" {
		fmt.Println("Loopback test skipped.")
		return
	}
	
	start := time.Now()
	err := tests.TestLoopback(audioIO, 10*time.Second)
	duration := time.Since(start)
	
	// Ask user for verification
	fmt.Println("\nDid you hear your voice played back through the speakers?")
	userResponse := ts.getUserInput("(y/N): ")
	passed := err == nil && userResponse == "y"
	
	details := "Captured microphone input and played through speakers"
	
	ts.recordResult("Audio Loopback", passed, duration, details, err)
}

func (ts *TestSuite) runPipelineTest() {
	fmt.Println("\n🤖 Running Pipeline Integration Test...")
	fmt.Println("Testing VAD → STT → LLM → TTS pipeline with mock services...")
	
	start := time.Now()
	err := tests.TestPipelineIntegration()
	duration := time.Since(start)
	
	passed := err == nil
	details := "Tested full voice processing pipeline"
	
	ts.recordResult("Pipeline Integration", passed, duration, details, err)
}

func (ts *TestSuite) runAllTests(audioIO *audio.LocalAudioIO) {
	fmt.Println("\n🚀 Running All Tests...")
	fmt.Println("This will run the complete test suite.")
	
	if ts.getUserInput("Continue with all tests? (y/N): ") != "y" {
		fmt.Println("All tests cancelled.")
		return
	}
	
	// Run all tests in sequence
	ts.runDeviceDiscoveryTest(audioIO)
	ts.runMicrophoneTest(audioIO)
	ts.runSpeakerTest(audioIO)
	ts.runLoopbackTest(audioIO)
	ts.runPipelineTest()
	
	fmt.Println("\n✅ All tests completed!")
}

func (ts *TestSuite) showSummary() {
	fmt.Println("\n📊 TEST SUMMARY")
	fmt.Println(strings.Repeat("=", 50))
	
	if len(ts.results) == 0 {
		fmt.Println("No tests have been run yet.")
		return
	}
	
	passed := 0
	failed := 0
	totalDuration := time.Duration(0)
	
	for _, result := range ts.results {
		status := "✅ PASS"
		if !result.Passed {
			status = "❌ FAIL"
			failed++
		} else {
			passed++
		}
		
		fmt.Printf("%s - %s (%v)\n", status, result.Name, result.Duration.Round(time.Millisecond))
		if result.Details != "" {
			fmt.Printf("        %s\n", result.Details)
		}
		if result.Error != nil {
			fmt.Printf("        Error: %v\n", result.Error)
		}
		
		totalDuration += result.Duration
	}
	
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("📈 Results: %d passed, %d failed, %d total\n", passed, failed, len(ts.results))
	fmt.Printf("⏱️  Total time: %v\n", totalDuration.Round(time.Millisecond))
	
	if failed == 0 && passed > 0 {
		fmt.Println("🎉 All tests passed! Your audio system is working correctly.")
	} else if failed > 0 {
		fmt.Printf("⚠️  %d test(s) failed. Please check the errors above.\n", failed)
	}
}