package agents

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"livekit-agents-go/audio"
	"livekit-agents-go/media"
	"livekit-agents-go/services/llm"
	"livekit-agents-go/services/stt"
	"livekit-agents-go/services/tts"
	"livekit-agents-go/services/vad"
)

// AudioGate prevents audio feedback using sophisticated timing and content analysis
type AudioGate struct {
	mu                   sync.RWMutex
	playingTTS           bool
	allowInterrupts      bool
	ttsEndTime           time.Time
	lastTTSContent       string
	gateExtensionPeriod  time.Duration
	recentTTSResponses   []string
}

// SetTTSPlaying updates the TTS playback state with enhanced timing
func (g *AudioGate) SetTTSPlaying(playing bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.playingTTS = playing
	if playing {
		log.Println("🔇 AudioGate: TTS playback started - gating microphone input")
	} else {
		// Set end time and extend gating period to prevent immediate feedback
		g.ttsEndTime = time.Now()
		log.Printf("🔇 AudioGate: TTS playback ended - extending gate for %v", g.gateExtensionPeriod)
	}
}

// SetTTSContent stores the content being played for feedback detection
func (g *AudioGate) SetTTSContent(content string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastTTSContent = content
	
	// Keep track of recent TTS responses for content-based filtering
	g.recentTTSResponses = append(g.recentTTSResponses, content)
	if len(g.recentTTSResponses) > 5 {
		g.recentTTSResponses = g.recentTTSResponses[1:] // Keep last 5 responses
	}
	
	log.Printf("🔇 AudioGate: Stored TTS content for feedback detection: \"%s\"", content)
}

// ShouldDiscardAudio returns true if audio input should be discarded to prevent feedback
func (g *AudioGate) ShouldDiscardAudio() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	// Primary gating: during active TTS playback
	if g.playingTTS && !g.allowInterrupts {
		return true
	}
	
	// Extended gating: for a period after TTS ends to prevent immediate echo
	if !g.ttsEndTime.IsZero() && time.Since(g.ttsEndTime) < g.gateExtensionPeriod {
		return true
	}
	
	return false
}

// IsRecentTTSContent checks if recognized text matches recent TTS output (feedback detection)
func (g *AudioGate) IsRecentTTSContent(recognizedText string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	// Check if the recognized text closely matches any recent TTS response
	for _, ttsContent := range g.recentTTSResponses {
		if g.isContentSimilar(recognizedText, ttsContent) {
			log.Printf("🚫 AudioGate: Detected TTS feedback - discarding: \"%s\"", recognizedText)
			return true
		}
	}
	
	return false
}

// isContentSimilar checks if two strings are similar enough to be considered the same
func (g *AudioGate) isContentSimilar(text1, text2 string) bool {
	// Simple similarity check - exact match or very close
	if strings.TrimSpace(strings.ToLower(text1)) == strings.TrimSpace(strings.ToLower(text2)) {
		return true
	}
	
	// Check if one is a subset of the other (for partial recognition)
	clean1 := strings.TrimSpace(strings.ToLower(text1))
	clean2 := strings.TrimSpace(strings.ToLower(text2))
	
	if len(clean1) > 10 && len(clean2) > 10 {
		return strings.Contains(clean1, clean2) || strings.Contains(clean2, clean1)
	}
	
	return false
}

// SetAllowInterrupts configures whether user can interrupt during TTS
func (g *AudioGate) SetAllowInterrupts(allow bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.allowInterrupts = allow
}

// EnableMicrophone explicitly enables microphone after ensuring no feedback risk
func (g *AudioGate) EnableMicrophone() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.ttsEndTime = time.Time{} // Clear the end time to stop extended gating
	log.Println("🎤 AudioGate: Microphone explicitly enabled")
}

// ChatCLI provides local audio I/O bypass for console mode (Python ChatCLI equivalent)
// This routes TTS audio directly to speakers instead of through LiveKit room tracks
type ChatCLI struct {
	// AEC Pipeline with audio I/O
	aecPipeline *audio.AECPipeline
	audioIO     *audio.LocalAudioIO
	
	// Services
	vadService vad.VAD
	sttService stt.STT
	llmService llm.LLM
	ttsService tts.TTS
	
	// Agent integration
	agent Agent
	session *AgentSession
	
	// Chat context for conversation
	chatContext *llm.ChatContext
	
	// Audio feedback prevention
	audioGate *AudioGate
	
	// Greeting state
	greetingGenerated bool
	greetingMu sync.Mutex
	
	// State management
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	
	// Configuration
	opts *WorkerOptions
}

// NewChatCLI creates a new ChatCLI for local audio bypass
func NewChatCLI(ctx context.Context, opts *WorkerOptions) *ChatCLI {
	chatCtx, cancel := context.WithCancel(ctx)
	
	return &ChatCLI{
		ctx:         chatCtx,
		cancel:      cancel,
		opts:        opts,
		chatContext: llm.NewChatContext(),
		audioGate: &AudioGate{
			allowInterrupts:      true,                  // Allow interrupts by default (Python pattern)
			gateExtensionPeriod:  3 * time.Second,      // Extended gating period after TTS ends
			recentTTSResponses:   make([]string, 0, 5), // Buffer for recent TTS content
		},
	}
}

// StartWithAEC begins the ChatCLI using an existing AEC pipeline for echo cancellation
func (c *ChatCLI) StartWithAEC(aecPipeline *audio.AECPipeline) error {
	log.Println("=== ChatCLI: Starting with AEC pipeline for echo cancellation ===")
	
	// Use the provided AEC pipeline's audio I/O instead of creating our own
	c.audioIO = aecPipeline.GetAudioIO()
	
	log.Println("ChatCLI: Using AEC pipeline audio I/O (Sample Rate: 48000 Hz)")
	
	// Set up TTS callback now that AEC audio I/O is ready
	c.mu.Lock()
	c.setupTTSCallback()
	c.mu.Unlock()
	log.Println("ChatCLI: AEC pipeline active - TTS will route through echo cancellation")
	log.Println("ChatCLI: Listening... (Press Ctrl+C to quit)")
	
	// Start the local pipeline processing
	return c.runLocalPipeline()
}

// Start begins the ChatCLI with local audio pipeline bypass
func (c *ChatCLI) Start() error {
	log.Println("=== ChatCLI: Starting local audio bypass mode (AEC temporarily disabled) ===")
	
	// TEMPORARY FIX: Use direct audio I/O instead of AEC pipeline to resolve transcription issues
	// TODO: Re-enable AEC once configuration issues are resolved
	log.Println("ChatCLI: Initializing direct audio I/O (AEC disabled for stability)...")
	
	audioConfig := audio.Config{
		SampleRate:      48000, // Use standard sample rate for better Deepgram compatibility
		Channels:        1,
		BitDepth:        16,
		FramesPerBuffer: 1024,
		EnableAECProcessing: false, // Disable AEC processing
	}
	
	var err error
	c.audioIO, err = audio.NewLocalAudioIO(audioConfig)
	if err != nil {
		return fmt.Errorf("failed to create direct audio I/O: %w", err)
	}
	defer c.audioIO.Close()

	// Show audio device info
	if err := c.audioIO.GetDeviceInfo(); err != nil {
		log.Printf("Warning: Failed to get audio device info: %v", err)
	}

	// Start direct audio I/O
	log.Println("ChatCLI: Starting direct audio I/O...")
	if err := c.audioIO.Start(c.ctx); err != nil {
		return fmt.Errorf("failed to start direct audio I/O: %w", err)
	}
	defer c.audioIO.Stop()
	
	log.Printf("ChatCLI: Direct audio I/O active - No echo cancellation (Sample Rate: %d Hz)", 
		audioConfig.SampleRate)

	// Set up TTS callback now that direct audio I/O is ready
	c.mu.Lock()
	c.setupTTSCallback()
	c.mu.Unlock()

	log.Println("ChatCLI: Local audio bypass active - TTS will route directly to speakers")
	log.Println("ChatCLI: Listening... (Press Ctrl+C to quit)")
	
	// Run the local audio pipeline
	return c.runLocalPipeline()
}

// SetServices configures the ChatCLI with AI services
func (c *ChatCLI) SetServices(vad vad.VAD, stt stt.STT, llm llm.LLM, tts tts.TTS) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.vadService = vad
	c.sttService = stt
	c.llmService = llm
	c.ttsService = tts
	
	log.Printf("ChatCLI: Services configured - VAD:%v, STT:%v, LLM:%v, TTS:%v",
		vad != nil, stt != nil, llm != nil, tts != nil)
}

// SetAgent configures the ChatCLI with an agent for function calling
func (c *ChatCLI) SetAgent(agent Agent, session *AgentSession) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.agent = agent
	c.session = session
	
	// Set up TTS output callback for console mode
	c.setupTTSCallback()
	
	// Add agent's system instructions to chat context
	if agent != nil && c.chatContext != nil {
		// Try to cast agent to BaseAgent to access GetInstructions
		if baseAgent, ok := agent.(*BaseAgent); ok {
			instructions := baseAgent.GetInstructions()
			if instructions != "" {
				c.chatContext.AddMessage(llm.RoleSystem, instructions)
				log.Printf("ChatCLI: Agent configured with instructions: %s", instructions)
			}
		}
	}
}

// setupTTSCallback sets up the TTS output callback if both session and audio I/O are available
func (c *ChatCLI) setupTTSCallback() {
	if c.session != nil && c.audioIO != nil {
		outputChan := c.audioIO.OutputChan()
		c.session.SetTTSOutputCallback(func(audioFrame *media.AudioFrame) {
			log.Println("🔊 [AUDIO] Sending TTS audio to local speakers via callback...")
			
			// Mark greeting as generated when first TTS audio is sent
			c.greetingMu.Lock()
			c.greetingGenerated = true
			c.greetingMu.Unlock()
			
			select {
			case outputChan <- audioFrame:
				log.Println("✅ [AUDIO] TTS audio sent to speakers successfully")
			case <-time.After(5 * time.Second):
				log.Println("⚠️  [AUDIO] TTS output timeout - speakers may be blocked")
			}
		})
		log.Println("🎵 [AUDIO] TTS output callback configured for console mode (direct audio)")
	}
}

// runLocalPipeline processes audio locally without room tracks
func (c *ChatCLI) runLocalPipeline() error {
	audioInput := c.audioIO.InputChan()
	audioOutput := c.audioIO.OutputChan()

	// Speech detection state
	speechBuffer := make([]*media.AudioFrame, 0)
	isRecordingSpeech := false
	lastSpeechTime := time.Now()
	frameCount := 0

	log.Println("🔌 ChatCLI: Local pipeline started - bypassing room tracks")

	// Generate initial greeting if agent is configured
	if c.agent != nil {
		go c.generateInitialGreeting(audioOutput)
	}

	for {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()

		case frame, ok := <-audioInput:
			if !ok {
				log.Println("ChatCLI: Audio input channel closed")
				return nil
			}
			
			frameCount++
			if frameCount%50 == 0 {
				log.Printf("📥 ChatCLI: Processed %d frames locally", frameCount)
			}

			// Audio feedback prevention - discard input during TTS playback
			if c.audioGate.ShouldDiscardAudio() {
				// Skip processing this frame to prevent feedback loops
				continue
			}

			// Simple energy-based speech detection (can be enhanced with VAD service)
			energy := c.calculateFrameEnergy(frame)
			speechDetected := energy > 0.0005

			if speechDetected {
				lastSpeechTime = time.Now()
			}

			// Manage speech recording state
			if speechDetected && !isRecordingSpeech {
				log.Println("🎤 ChatCLI: Speech detected - recording...")
				isRecordingSpeech = true
				speechBuffer = speechBuffer[:0] // Clear buffer
			}

			// Collect speech frames
			if isRecordingSpeech {
				speechBuffer = append(speechBuffer, frame)
			}

			// End speech recording after silence
			if isRecordingSpeech && time.Since(lastSpeechTime) > 1*time.Second {
				log.Println("🔇 ChatCLI: Speech ended - processing locally...")
				isRecordingSpeech = false

				// Process accumulated speech locally
				if len(speechBuffer) > 0 {
					go c.processLocalSpeech(speechBuffer, audioOutput)
				}
			}
		}
	}
}

// generateInitialGreeting creates an initial greeting via TTS (Python pattern)
func (c *ChatCLI) generateInitialGreeting(audioOutput chan<- *media.AudioFrame) {
	time.Sleep(500 * time.Millisecond) // Brief delay for initialization
	
	// Check if greeting was already generated by session callback
	c.greetingMu.Lock()
	alreadyGenerated := c.greetingGenerated
	c.greetingMu.Unlock()
	
	if alreadyGenerated {
		log.Println("ChatCLI: Greeting already generated by session - skipping")
		return
	}
	
	log.Println("ChatCLI: Generating initial greeting...")
	
	// Generate greeting through agent if available, otherwise use default
	var greeting string
	if c.agent != nil && c.session != nil {
		// Use agent session to generate contextual greeting
		log.Println("ChatCLI: Requesting agent to generate initial greeting...")
		if err := c.session.GenerateReply(); err != nil {
			log.Printf("ChatCLI: Agent greeting failed, using default: %v", err)
			greeting = "Hello! I'm Kelly, your voice assistant. How can I help you today?"
		} else {
			// Agent session generated reply - it should be captured by normal pipeline
			log.Println("ChatCLI: Agent generated initial greeting")
			return
		}
	} else {
		greeting = "Hello! I'm Kelly, your voice assistant. How can I help you today?"
	}
	
	// Convert greeting to speech and play locally
	if c.ttsService != nil {
		c.synthesizeAndPlayLocal(greeting, audioOutput)
	}
}

// processLocalSpeech handles STT -> LLM -> TTS pipeline locally
func (c *ChatCLI) processLocalSpeech(speechFrames []*media.AudioFrame, audioOutput chan<- *media.AudioFrame) {
	log.Println("🧠 ChatCLI: Processing speech through local pipeline...")

	// Combine audio frames for STT processing
	combinedAudio := c.combineAudioFrames(speechFrames)
	if combinedAudio == nil {
		log.Println("❌ ChatCLI: No audio data to process")
		return
	}

	// Speech-to-Text
	log.Println("👂 ChatCLI: Converting speech to text...")
	if c.sttService == nil {
		log.Println("❌ ChatCLI: STT service not available")
		return
	}

	recognition, err := c.sttService.Recognize(c.ctx, combinedAudio)
	if err != nil {
		log.Printf("❌ ChatCLI: STT error: %v", err)
		return
	}

	if recognition.Text == "" {
		log.Println("❌ ChatCLI: No text recognized")
		return
	}

	log.Printf("📝 ChatCLI: Recognized: \"%s\" (confidence: %.2f)", recognition.Text, recognition.Confidence)

	// Content-based feedback detection - check if this matches recent TTS output
	if c.audioGate.IsRecentTTSContent(recognition.Text) {
		log.Printf("🚫 ChatCLI: Discarding recognized text as TTS feedback")
		return
	}

	// Add user message to chat context
	userMessage := &llm.ChatMessage{
		Role:    llm.RoleUser,
		Content: recognition.Text,
	}
	c.chatContext.AddMessage(llm.RoleUser, recognition.Text)

	// Check for function calling through agent if available
	var responseText string
	if c.agent != nil && c.session != nil {
		// Try to use agent's function calling capabilities
		log.Println("🔧 ChatCLI: Checking for function calls through agent...")
		
		// Trigger agent's user turn completion callback
		if err := c.agent.OnUserTurnCompleted(c.ctx, c.chatContext, userMessage); err != nil {
			log.Printf("⚠️ ChatCLI: Agent user turn callback failed: %v", err)
		}
		
		// Process through agent session for function calling
		response, err := c.session.ProcessUserMessageWithResponse(userMessage)
		if err != nil {
			log.Printf("⚠️ ChatCLI: Agent processing failed, falling back to direct LLM: %v", err)
			responseText = c.generateDirectLLMResponse(recognition.Text)
		} else {
			responseText = response.Content
			log.Printf("✅ ChatCLI: Agent response (with function calling): \"%s\"", responseText)
		}
	} else {
		// Direct LLM processing without function calling
		responseText = c.generateDirectLLMResponse(recognition.Text)
	}

	if responseText == "" {
		log.Println("❌ ChatCLI: No response generated")
		return
	}

	// Add assistant response to chat context
	c.chatContext.AddMessage(llm.RoleAssistant, responseText)

	// Convert to speech and play locally
	c.synthesizeAndPlayLocal(responseText, audioOutput)
}

// generateDirectLLMResponse generates a response using direct LLM call
func (c *ChatCLI) generateDirectLLMResponse(userText string) string {
	log.Println("🤖 ChatCLI: Generating response through direct LLM...")
	
	if c.llmService == nil {
		log.Println("❌ ChatCLI: LLM service not available")
		return ""
	}

	// Use chat context for conversation continuity
	response, err := c.llmService.Chat(c.ctx, c.chatContext.GetMessages(), nil)
	if err != nil {
		log.Printf("❌ ChatCLI: LLM error: %v", err)
		return ""
	}

	responseText := response.Message.Content
	log.Printf("💬 ChatCLI: LLM response: \"%s\"", responseText)
	return responseText
}

// synthesizeAndPlayLocal converts text to speech and plays it locally
func (c *ChatCLI) synthesizeAndPlayLocal(text string, audioOutput chan<- *media.AudioFrame) {
	log.Printf("🗣️ ChatCLI: Converting to speech: \"%s\"", text)
	
	if c.ttsService == nil {
		log.Println("❌ ChatCLI: TTS service not available")
		return
	}

	// Store TTS content for feedback detection
	c.audioGate.SetTTSContent(text)
	
	// Mark greeting as generated
	c.greetingMu.Lock()
	c.greetingGenerated = true
	c.greetingMu.Unlock()

	// Gate microphone input during TTS synthesis and playback
	c.audioGate.SetTTSPlaying(true)
	defer func() {
		c.audioGate.SetTTSPlaying(false)
		// Additional safety delay before allowing microphone input
		go func() {
			time.Sleep(2 * time.Second) // Extra delay for audio system to settle
			c.audioGate.EnableMicrophone()
		}()
	}()

	audioResponse, err := c.ttsService.Synthesize(c.ctx, text, nil)
	if err != nil {
		log.Printf("❌ ChatCLI: TTS error: %v", err)
		return
	}

	// Calculate estimated playback duration for better timing
	estimatedDuration := c.estimatePlaybackDuration(audioResponse)
	log.Printf("🕒 ChatCLI: Estimated playback duration: %v", estimatedDuration)

	// Play TTS audio directly to local speakers (bypassing room tracks)
	log.Println("🔊 ChatCLI: Playing response directly to local speakers...")
	log.Printf("📊 ChatCLI: Audio response: %d bytes, format: %+v", len(audioResponse.Data), audioResponse.Format)
	
	// Send TTS audio to local output (NOT to room tracks)
	select {
	case audioOutput <- audioResponse:
		log.Println("✅ ChatCLI: Response sent to local speakers")
		// Wait for estimated playback duration plus buffer time
		time.Sleep(estimatedDuration + 500*time.Millisecond)
		log.Printf("⏰ ChatCLI: Playback complete after %v", estimatedDuration + 500*time.Millisecond)
	case <-c.ctx.Done():
		return
	case <-time.After(10 * time.Second):
		log.Println("⚠️ ChatCLI: Local audio output timeout - output channel may be blocked")
	}
}

// estimatePlaybackDuration estimates how long the audio will take to play
func (c *ChatCLI) estimatePlaybackDuration(frame *media.AudioFrame) time.Duration {
	// Calculate duration based on sample rate and data size
	sampleRate := frame.Format.SampleRate
	channels := frame.Format.Channels
	bytesPerSample := 2 // Assuming 16-bit audio
	
	if sampleRate > 0 && channels > 0 {
		totalSamples := len(frame.Data) / (channels * bytesPerSample)
		duration := time.Duration(float64(totalSamples)/float64(sampleRate)*1000) * time.Millisecond
		return duration
	}
	
	// Fallback estimation based on data size
	return time.Duration(len(frame.Data)/3000) * time.Millisecond // Rough approximation
}

// combineAudioFrames combines multiple audio frames into a single frame
func (c *ChatCLI) combineAudioFrames(frames []*media.AudioFrame) *media.AudioFrame {
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

// calculateFrameEnergy calculates the normalized RMS energy of an audio frame
func (c *ChatCLI) calculateFrameEnergy(frame *media.AudioFrame) float64 {
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

// Stop gracefully shuts down the ChatCLI
func (c *ChatCLI) Stop() error {
	log.Println("ChatCLI: Stopping local audio bypass...")
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

// SetAECEnabled enables or disables echo cancellation
func (c *ChatCLI) SetAECEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.aecPipeline != nil {
		// TODO: Add dynamic AEC enable/disable to AECPipeline
		log.Printf("ChatCLI: AEC enable/disable requested: %v (dynamic control not yet implemented)", enabled)
	}
}

// GetAECStats returns current AEC performance statistics
func (c *ChatCLI) GetAECStats() audio.PipelineStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.aecPipeline != nil {
		return c.aecPipeline.GetStats()
	}
	return audio.PipelineStats{}
}

// SetAECDelay updates the estimated delay for AEC processing
func (c *ChatCLI) SetAECDelay(delay time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.aecPipeline != nil {
		log.Printf("ChatCLI: Updating AEC delay to %v", delay)
		return c.aecPipeline.SetDelay(delay)
	}
	return fmt.Errorf("AEC pipeline not initialized")
}

// PrintAECStats prints current AEC statistics
func (c *ChatCLI) PrintAECStats() {
	if c.aecPipeline != nil {
		c.aecPipeline.PrintStats()
	}
}

// CalibrateAECDelay automatically calibrates the AEC delay
func (c *ChatCLI) CalibrateAECDelay(duration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.aecPipeline != nil {
		log.Printf("ChatCLI: Starting automatic AEC delay calibration (%v)", duration)
		return c.aecPipeline.CalibrateDelay(c.ctx, duration)
	}
	return fmt.Errorf("AEC pipeline not initialized")
}