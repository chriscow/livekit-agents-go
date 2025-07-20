package agents

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
	"livekit-agents-go/plugins"
	"livekit-agents-go/services/llm"
	"livekit-agents-go/services/stt"
	"livekit-agents-go/services/tts"
	"livekit-agents-go/services/vad"
)

// ConsoleAgent handles local audio I/O for console mode testing
type ConsoleAgent struct {
	opts *WorkerOptions
	
	// Local audio I/O
	audioIO *audio.LocalAudioIO
	
	// Services
	vadService vad.VAD
	sttService stt.STT
	llmService llm.LLM
	ttsService tts.TTS
	
	// Agent implementation
	agent Agent
	
	// Context and shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// NewConsoleAgent creates a new console agent with local audio I/O
func NewConsoleAgent(opts *WorkerOptions) *ConsoleAgent {
	return &ConsoleAgent{
		opts: opts,
	}
}

// Start begins the console agent with local audio pipeline
func (ca *ConsoleAgent) Start(ctx context.Context) error {
	ca.ctx, ca.cancel = context.WithCancel(ctx)
	defer ca.cancel()

	// Load environment variables first (critical for service initialization)
	log.Println("Console Agent: Loading environment variables...")
	if err := ca.loadEnvironmentVariables(); err != nil {
		log.Printf("⚠️ Warning: Failed to load .env file: %v", err)
	}

	log.Println("Console Agent: Initializing services...")

	// Initialize services using the plugin system
	services, err := plugins.CreateSmartServices()
	if err != nil {
		return fmt.Errorf("failed to create services: %w", err)
	}

	ca.vadService = services.VAD
	ca.sttService = services.STT
	ca.llmService = services.LLM
	ca.ttsService = services.TTS

	log.Printf("Console Agent: Services initialized:")
	if ca.sttService != nil {
		log.Printf("  STT: %s", ca.sttService.Name())
	}
	if ca.llmService != nil {
		log.Printf("  LLM: %s", ca.llmService.Name())
	}
	if ca.ttsService != nil {
		log.Printf("  TTS: %s", ca.ttsService.Name())
	}
	if ca.vadService != nil {
		log.Printf("  VAD: %s", ca.vadService.Name())
	}

	// Initialize AEC pipeline for echo cancellation with proper sample rate
	log.Println("Console Agent: Initializing AEC pipeline...")
	aecConfig := audio.DefaultAECConfig()
	aecConfig.SampleRate = 48000 // Use 48kHz to match audio pipeline and prevent sample rate conflicts
	aecPipeline, err := audio.NewAECPipeline(aecConfig)
	if err != nil {
		return fmt.Errorf("failed to create AEC pipeline: %w", err)
	}
	defer aecPipeline.Close()

	// Extract the underlying audio I/O for compatibility
	ca.audioIO = aecPipeline.GetAudioIO()
	
	log.Printf("Console Agent: AEC pipeline configured (Sample Rate: %d Hz, Echo Cancellation: %v)", 
		aecConfig.SampleRate, aecConfig.EnableEchoCancellation)

	// Show audio device info
	if err := ca.audioIO.GetDeviceInfo(); err != nil {
		log.Printf("Warning: Failed to get audio device info: %v", err)
	}

	// Start AEC pipeline (which starts the underlying audio I/O)
	log.Println("Console Agent: Starting AEC pipeline...")
	if err := aecPipeline.Start(ca.ctx); err != nil {
		return fmt.Errorf("failed to start AEC pipeline: %w", err)
	}
	defer aecPipeline.Stop()

	// Create and start agent if entrypoint is provided
	var session *AgentSession
	if ca.opts.EntrypointFunc != nil {
		// Create agent session and configure with existing services
		session = NewAgentSession(ca.ctx)
		session.STT = ca.sttService
		session.LLM = ca.llmService
		session.TTS = ca.ttsService
		session.VAD = ca.vadService

		// Create job context for console mode with configured session
		jobCtx := &JobContext{
			Context: ca.ctx,
			Session: session,
			// Room handling: nil for legacy mode, mock room for console mode
		}
		
		// If console mode is enabled, use ChatCLI but with AEC pipeline (not bypass)
		if ca.opts.ConsoleMode {
			log.Printf("Console mode: Creating mock room '%s'", ca.opts.RoomName)
			// TODO: Create proper mock room implementation
			// For now, leave Room as nil but log that we would create a mock room
			// jobCtx.Room = createMockRoom(ca.opts.RoomName)
			
			log.Println("Console Agent: Using ChatCLI with AEC pipeline for echo cancellation...")
			return ca.runChatCLIWithAEC(jobCtx, session, aecPipeline)
		}

		log.Println("Console Agent: Starting entrypoint...")
		
		// Start the entrypoint in a goroutine so it doesn't block the audio pipeline
		go func() {
			if err := ca.opts.EntrypointFunc(jobCtx); err != nil {
				log.Printf("❌ Entrypoint failed: %v", err)
			}
		}()

		// Give entrypoint a moment to initialize
		time.Sleep(100 * time.Millisecond)

		// Start integrated pipeline immediately
		log.Println("Console Agent: Using integrated agent session voice pipeline...")
		log.Println("Console Agent: Listening... (Press Ctrl+C to quit)")
		return ca.runIntegratedVoicePipeline(session)
	}

	// Fallback: Start the standalone audio processing loop
	log.Println("Console Agent: Starting standalone voice pipeline...")
	log.Println("Console Agent: Listening... (Press Ctrl+C to quit)")

	return ca.runVoicePipeline()
}

// runIntegratedVoicePipeline runs audio processing through AgentSession (Python framework pattern)
func (ca *ConsoleAgent) runIntegratedVoicePipeline(session *AgentSession) error {
	audioInput := ca.audioIO.InputChan()
	audioOutput := ca.audioIO.OutputChan()

	// Audio buffer for accumulating speech
	speechBuffer := make([]*media.AudioFrame, 0)
	isRecordingSpeech := false
	lastSpeechTime := time.Now()
	frameCount := 0

	log.Println("🔌 Integrated voice pipeline started - audio flows through AgentSession")

	for {
		select {
		case <-ca.ctx.Done():
			return ca.ctx.Err()

		case frame, ok := <-audioInput:
			if !ok {
				log.Println("Console Agent: Audio input channel closed")
				return nil
			}
			
			frameCount++
			if frameCount%50 == 0 {
				log.Printf("📥 Processed %d frames through AgentSession", frameCount)
			}

			// Process frame through agent session for VAD and agent notifications
			if err := session.ProcessAudioFrame(frame); err != nil {
				log.Printf("Frame processing error: %v", err)
			}

			// Simple energy-based detection for speech boundaries (fallback)
			energy := ca.calculateFrameEnergy(frame)
			speechDetected := energy > 0.0005

			if speechDetected {
				lastSpeechTime = time.Now()
			}

			// Manage speech recording state
			if speechDetected && !isRecordingSpeech {
				log.Println("🎤 Speech detected - recording...")
				isRecordingSpeech = true
				speechBuffer = speechBuffer[:0] // Clear buffer
			}

			// Collect speech frames
			if isRecordingSpeech {
				speechBuffer = append(speechBuffer, frame)
			}

			// End speech recording after silence
			if isRecordingSpeech && time.Since(lastSpeechTime) > 1*time.Second {
				log.Println("🔇 Speech ended - processing through AgentSession...")
				isRecordingSpeech = false

				// Process accumulated speech through agent session pipeline
				if len(speechBuffer) > 0 {
					go ca.processIntegratedSpeech(session, speechBuffer, audioOutput)
				}
			}
		}
	}
}

// processIntegratedSpeech handles speech processing through AgentSession
func (ca *ConsoleAgent) processIntegratedSpeech(session *AgentSession, speechFrames []*media.AudioFrame, audioOutput chan<- *media.AudioFrame) {
	log.Println("🧠 Processing speech through integrated AgentSession pipeline...")

	// Use AgentSession's integrated pipeline including function calling
	audioResponse, err := session.ProcessSpeechFrames(speechFrames)
	if err != nil {
		log.Printf("❌ Integrated speech processing error: %v", err)
		return
	}

	// Send TTS audio to output
	log.Println("🔊 Playing response from AgentSession...")
	log.Printf("📊 Audio response: %d bytes", len(audioResponse.Data))
	
	select {
	case audioOutput <- audioResponse:
		log.Println("✅ Response sent to audio output")
	case <-ca.ctx.Done():
		return
	case <-time.After(5 * time.Second):
		log.Println("⚠️  Audio output timeout - output channel may be blocked")
	}
}

// runVoicePipeline runs the main voice processing loop
func (ca *ConsoleAgent) runVoicePipeline() error {
	audioInput := ca.audioIO.InputChan()
	audioOutput := ca.audioIO.OutputChan()

	// Voice activity detection state - use proper VAD interface
	var vadStream vad.DetectionStream
	// Temporarily disable mock VAD to test fallback energy-based detection
	vadStream = nil
	log.Println("🔬 Using fallback energy-based VAD for better real-audio detection")
	// if ca.vadService != nil {
	//	vadStream, err = ca.vadService.DetectStream(ca.ctx, vad.DefaultStreamOptions())
	//	if err != nil {
	//		log.Printf("Warning: Failed to create VAD stream: %v", err)
	//	}
	// }

	// Audio buffer for accumulating speech
	speechBuffer := make([]*media.AudioFrame, 0)
	isRecordingSpeech := false
	lastSpeechTime := time.Now()
	frameCount := 0

	for {
		select {
		case <-ca.ctx.Done():
			return ca.ctx.Err()

		case frame, ok := <-audioInput:
			if !ok {
				log.Println("Console Agent: Audio input channel closed")
				return nil
			}
			
			// Debug: Log that we're receiving frames (every 50th frame to avoid spam)
			frameCount++
			if frameCount%50 == 0 {
				log.Printf("📥 Received %d frames, current frame: %d bytes", frameCount, len(frame.Data))
			}

			// Voice Activity Detection
			speechDetected := false
			if vadStream != nil {
				err := vadStream.SendAudio(frame)
				if err != nil {
					log.Printf("VAD send error: %v", err)
				} else {
					vadResult, err := vadStream.Recv()
					if err != nil {
						log.Printf("VAD recv error: %v", err)
					} else {
						speechDetected = vadResult.Probability > 0.4 // Lowered threshold for mock VAD testing
						// Only log when speech is detected or probability is high
						if speechDetected || vadResult.Probability > 0.5 {
							log.Printf("🔊 VAD result: probability=%.3f, speech=%v", vadResult.Probability, speechDetected)
						}
						if speechDetected {
							lastSpeechTime = time.Now()
						}
					}
				}
			} else {
				// Fallback: simple energy-based detection
				energy := ca.calculateFrameEnergy(frame)
				speechDetected = energy > 0.0005 // Adjusted threshold based on observed noise levels (~10x background)
				
				// Debug: Log energy levels every 100 frames to understand range
				if frameCount%100 == 0 {
					log.Printf("🔊 Energy VAD: energy=%.8f, speech=%v", energy, speechDetected)
				}
				
				if speechDetected {
					lastSpeechTime = time.Now()
				}
			}

			// Manage speech recording state
			if speechDetected && !isRecordingSpeech {
				log.Println("🎤 Speech detected - recording...")
				isRecordingSpeech = true
				speechBuffer = speechBuffer[:0] // Clear buffer
			}

			// Collect speech frames
			if isRecordingSpeech {
				speechBuffer = append(speechBuffer, frame)
			}

			// End speech recording after silence
			if isRecordingSpeech && time.Since(lastSpeechTime) > 1*time.Second {
				log.Println("🔇 Speech ended - processing...")
				isRecordingSpeech = false

				// Process accumulated speech
				if len(speechBuffer) > 0 {
					go ca.processSpeech(speechBuffer, audioOutput)
				}
			}
		}
	}
}

// processSpeech handles STT -> LLM -> TTS pipeline
func (ca *ConsoleAgent) processSpeech(speechFrames []*media.AudioFrame, audioOutput chan<- *media.AudioFrame) {
	// Combine audio frames for STT processing
	combinedAudio := ca.combineAudioFrames(speechFrames)
	if combinedAudio == nil {
		log.Println("❌ No audio data to process")
		return
	}

	// Speech-to-Text
	log.Println("🧠 Converting speech to text...")
	recognition, err := ca.sttService.Recognize(ca.ctx, combinedAudio)
	if err != nil {
		log.Printf("❌ STT error: %v", err)
		return
	}

	if recognition.Text == "" {
		log.Println("❌ No text recognized")
		return
	}

	log.Printf("📝 Recognized: \"%s\" (confidence: %.2f)", recognition.Text, recognition.Confidence)

	// LLM Processing
	log.Println("🤖 Generating response...")
	messages := []llm.Message{
		{
			Role: llm.RoleSystem,
			Content: "You are Kelly, a helpful voice assistant. Keep responses concise and conversational since you're speaking to the user.",
		},
		{
			Role:    llm.RoleUser,
			Content: recognition.Text,
		},
	}

	response, err := ca.llmService.Chat(ca.ctx, messages, nil)
	if err != nil {
		log.Printf("❌ LLM error: %v", err)
		return
	}

	responseText := response.Message.Content
	log.Printf("💬 Response: \"%s\"", responseText)

	// Text-to-Speech
	log.Println("🗣️  Converting text to speech...")
	audioResponse, err := ca.ttsService.Synthesize(ca.ctx, responseText, nil)
	if err != nil {
		log.Printf("❌ TTS error: %v", err)
		return
	}

	// Send TTS audio to output
	log.Println("🔊 Playing response...")
	log.Printf("📊 Audio response: %d bytes, format: %+v", len(audioResponse.Data), audioResponse.Format)
	
	// Try to send the audio frame
	select {
	case audioOutput <- audioResponse:
		log.Println("✅ Response sent to audio output")
	case <-ca.ctx.Done():
		return
	case <-time.After(5 * time.Second):
		log.Println("⚠️  Audio output timeout - output channel may be blocked")
	}
}

// combineAudioFrames combines multiple audio frames into a single frame
func (ca *ConsoleAgent) combineAudioFrames(frames []*media.AudioFrame) *media.AudioFrame {
	if len(frames) == 0 {
		return nil
	}

	// Calculate total data size
	totalSize := 0
	for _, frame := range frames {
		totalSize += len(frame.Data)
	}

	if totalSize == 0 {
		return nil
	}

	// Combine all frame data
	combinedData := make([]byte, 0, totalSize)
	for _, frame := range frames {
		combinedData = append(combinedData, frame.Data...)
	}

	// Create combined frame with same format as first frame
	return media.NewAudioFrame(combinedData, frames[0].Format)
}

// hasAudioEnergy performs simple energy-based voice detection as fallback
func (ca *ConsoleAgent) hasAudioEnergy(frame *media.AudioFrame) bool {
	energy := ca.calculateFrameEnergy(frame)
	// Threshold for speech detection (adjust as needed)
	return energy > 0.001
}

// calculateFrameEnergy calculates the normalized RMS energy of an audio frame
func (ca *ConsoleAgent) calculateFrameEnergy(frame *media.AudioFrame) float64 {
	if len(frame.Data) < 2 {
		return 0.0
	}

	// Calculate RMS energy
	var sum int64
	sampleCount := len(frame.Data) / 2 // 16-bit samples

	for i := 0; i < sampleCount; i++ {
		// Read 16-bit little-endian sample
		sample := int16(frame.Data[i*2]) | (int16(frame.Data[i*2+1]) << 8)
		sum += int64(sample) * int64(sample)
	}

	rms := float64(sum) / float64(sampleCount)
	energy := rms / (32767.0 * 32767.0) // Normalize to [0, 1]

	return energy
}

// loadEnvironmentVariables loads .env file (similar to CLI tools and basic-agent main)
func (ca *ConsoleAgent) loadEnvironmentVariables() error {
	// Try to load .env file from current directory first
	if err := godotenv.Load(".env"); err == nil {
		log.Println("✅ Loaded environment from: .env")
		return nil
	}
	
	// Try to find project root and load .env from there
	projectRoot := ca.findProjectRoot()
	if projectRoot != "" {
		envPath := filepath.Join(projectRoot, ".env")
		if err := godotenv.Load(envPath); err == nil {
			log.Printf("✅ Loaded environment from: %s", envPath)
			return nil
		}
	}
	
	log.Println("⚠️ No .env file found, using system environment variables")
	return nil
}

// findProjectRoot looks for the project root by finding go.mod
func (ca *ConsoleAgent) findProjectRoot() string {
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

// runChatCLI starts the ChatCLI for local audio bypass (Python console mode equivalent)
func (ca *ConsoleAgent) runChatCLI(jobCtx *JobContext, session *AgentSession) error {
	log.Println("🎯 Console Agent: Starting ChatCLI for local audio bypass...")
	
	// Create ChatCLI instance
	chatCLI := NewChatCLI(ca.ctx, ca.opts)
	
	// Configure ChatCLI with services
	chatCLI.SetServices(ca.vadService, ca.sttService, ca.llmService, ca.ttsService)
	
	// Start the entrypoint to create the agent instance
	log.Println("Console Agent: Starting entrypoint to initialize agent...")
	go func() {
		if err := ca.opts.EntrypointFunc(jobCtx); err != nil {
			log.Printf("❌ Entrypoint failed: %v", err)
		}
	}()
	
	// Give entrypoint time to initialize and create agent
	time.Sleep(200 * time.Millisecond)
	
	// Configure ChatCLI with agent for function calling
	if session.Agent != nil {
		log.Println("Console Agent: Configuring ChatCLI with agent for function calling...")
		chatCLI.SetAgent(session.Agent, session)
	} else {
		log.Println("⚠️ Console Agent: No agent available - ChatCLI will use direct LLM")
	}
	
	// Start ChatCLI - this will handle all local audio I/O and bypass room tracks
	log.Println("Console Agent: Starting ChatCLI local audio bypass...")
	return chatCLI.Start()
}

// runChatCLIWithAEC starts ChatCLI using an existing AEC pipeline for echo cancellation
func (ca *ConsoleAgent) runChatCLIWithAEC(jobCtx *JobContext, session *AgentSession, aecPipeline *audio.AECPipeline) error {
	log.Println("🎯 Console Agent: Starting ChatCLI with AEC pipeline...")
	
	// Create ChatCLI instance
	chatCLI := NewChatCLI(ca.ctx, ca.opts)
	
	// Configure ChatCLI with services
	chatCLI.SetServices(ca.vadService, ca.sttService, ca.llmService, ca.ttsService)
	
	// Start the entrypoint to create the agent instance
	log.Println("Console Agent: Starting entrypoint to initialize agent...")
	go func() {
		if err := ca.opts.EntrypointFunc(jobCtx); err != nil {
			log.Printf("❌ Entrypoint failed: %v", err)
		}
	}()
	
	// Give entrypoint time to initialize and create agent
	time.Sleep(200 * time.Millisecond)
	
	// Configure ChatCLI with agent for function calling
	if session.Agent != nil {
		log.Println("Console Agent: Configuring ChatCLI with agent for function calling...")
		chatCLI.SetAgent(session.Agent, session)
	} else {
		log.Println("⚠️ Console Agent: No agent available - ChatCLI will use direct LLM")
	}
	
	// Start ChatCLI with AEC pipeline for echo cancellation
	log.Println("Console Agent: Starting ChatCLI with AEC pipeline...")
	return chatCLI.StartWithAEC(aecPipeline)
}

// Stop gracefully shuts down the console agent
func (ca *ConsoleAgent) Stop() error {
	if ca.cancel != nil {
		ca.cancel()
	}
	return nil
}