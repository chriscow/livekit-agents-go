package agents

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hraban/opus"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/livekit/protocol/livekit"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	webrtcmedia "github.com/pion/webrtc/v4/pkg/media"

	"livekit-agents-go/media"
	"livekit-agents-go/plugins/openai"
	"livekit-agents-go/services/llm"
	"livekit-agents-go/services/tts"
	"livekit-agents-go/services/vad"
)

// ConversationContext tracks conversation state for a participant
type ConversationContext struct {
	ParticipantID       string
	ConversationHistory []ConversationTurn
	LastActivity        time.Time
	mu                  sync.RWMutex
}

// ConversationTurn represents a single turn in the conversation
type ConversationTurn struct {
	Timestamp  time.Time
	Speaker    string // "user" or "assistant"
	Text       string
	Confidence float64 // For STT results
}

// Worker manages agent jobs and lifecycle (equivalent to Python Worker)
type Worker struct {
	opts      *WorkerOptions
	scheduler *JobScheduler
	registry  *Registry
	mu        sync.RWMutex
	running   bool
	jobs      map[string]*JobContext

	// Audio processing
	audioBuffers    map[string]*AudioBuffer      // participant ID -> audio buffer
	audioStreams    map[string]*AudioByteStream  // participant ID -> audio byte stream
	opusDecoders    map[string]*opus.Decoder     // participant ID -> opus decoder
	vadStreams      map[string]vad.VADStream     // participant ID -> VAD stream
	sttService      *openai.WhisperSTT
	vadService      vad.SileroVAD // VAD service for voice activity detection

	// Conversation processing
	conversations map[string]*ConversationContext // participant ID -> conversation context
	llmService    *openai.GPTLLM                  // LLM service for intelligent responses
	ttsService    *openai.OpenAITTS               // TTS service for speech synthesis
	room          *lksdk.Room                     // LiveKit room for publishing responses

	// Audio publishing
	audioTrack        *lksdk.LocalTrackPublication // Published audio track for assistant voice
	audioProvider     *AudioSampleProvider         // Sample provider for streaming audio (simplified approach)
	isPublishingAudio bool                         // Track if we're currently publishing audio
}

// AudioBuffer accumulates audio frames for STT processing
type AudioBuffer struct {
	frames    []*media.AudioFrame
	maxFrames int
	lastSTT   time.Time
	mu        sync.Mutex
}

// AudioByteStream handles audio frame buffering and conversion (similar to LiveKit JS pattern)
type AudioByteStream struct {
	buffer        []byte
	targetFormat  media.AudioFormat
	partialFrame  []byte
	mu            sync.Mutex
}

// NewAudioByteStream creates a new audio byte stream for frame conversion
func NewAudioByteStream(sampleRate, channels, bitsPerSample int) *AudioByteStream {
	return &AudioByteStream{
		buffer: make([]byte, 0, 9600), // Buffer for 2 full frames (200ms at 24kHz)
		targetFormat: media.AudioFormat{
			SampleRate:    sampleRate,
			Channels:      channels,
			BitsPerSample: bitsPerSample,
			Format:        media.AudioFormatPCM,
		},
		partialFrame: make([]byte, 0),
	}
}

// WriteBytes adds raw audio bytes and returns complete audio frames
func (abs *AudioByteStream) WriteBytes(data []byte) []*media.AudioFrame {
	abs.mu.Lock()
	defer abs.mu.Unlock()
	
	// Add new data to buffer
	abs.buffer = append(abs.buffer, data...)
	
	// Calculate frame size (100ms at target sample rate)
	bytesPerSample := abs.targetFormat.BitsPerSample / 8
	samplesPerFrame := abs.targetFormat.SampleRate / 10 // 100ms = 1/10 second
	frameSize := samplesPerFrame * abs.targetFormat.Channels * bytesPerSample
	
	var frames []*media.AudioFrame
	
	// Extract complete frames from buffer
	for len(abs.buffer) >= frameSize {
		frameData := make([]byte, frameSize)
		copy(frameData, abs.buffer[:frameSize])
		
		// Create audio frame
		frame := media.NewAudioFrame(frameData, abs.targetFormat)
		frames = append(frames, frame)
		
		// Remove processed data from buffer
		abs.buffer = abs.buffer[frameSize:]
	}
	
	return frames
}

// Flush returns any remaining buffered data as a final frame
func (abs *AudioByteStream) Flush() *media.AudioFrame {
	abs.mu.Lock()
	defer abs.mu.Unlock()
	
	if len(abs.buffer) == 0 {
		return nil
	}
	
	// Create frame with remaining data (may be shorter than full frame)
	frameData := make([]byte, len(abs.buffer))
	copy(frameData, abs.buffer)
	
	frame := media.NewAudioFrame(frameData, abs.targetFormat)
	abs.buffer = abs.buffer[:0] // Clear buffer
	
	return frame
}

type WorkerOptions struct {
	// Main entrypoint function for agent creation
	EntrypointFunc func(ctx *JobContext) error

	// Optional prewarm function for optimization
	PrewarmFunc func(proc *JobProcess) error

	// Execution model (thread vs process)
	ExecutorType JobExecutorType

	// Worker configuration
	Host      string
	Port      int
	APIKey    string
	APISecret string

	// LiveKit connection
	LiveKitURL string

	// Agent configuration
	AgentName string
	Metadata  map[string]string
}

// JobScheduler manages job execution
type JobScheduler struct {
	executorType JobExecutorType
	jobQueue     chan *JobContext
	workers      []*jobWorker
	mu           sync.RWMutex
	wg           sync.WaitGroup
}

type jobWorker struct {
	id       int
	jobs     chan *JobContext
	quit     chan bool
	executor JobExecutorType
}

// AudioSampleProvider implements the SampleProvider interface for streaming TTS audio
type AudioSampleProvider struct {
	sampleQueue chan []byte
	closed      bool
	mu          sync.Mutex
}

// NewAudioSampleProvider creates a new audio sample provider for TTS streaming
func NewAudioSampleProvider() *AudioSampleProvider {
	return &AudioSampleProvider{
		sampleQueue: make(chan []byte, 100), // Buffer up to 100 audio chunks
		closed:      false,
	}
}

// NextSample returns the next audio sample for the LiveKit track
func (asp *AudioSampleProvider) NextSample(ctx context.Context) (webrtcmedia.Sample, error) {
	select {
	case <-ctx.Done():
		return webrtcmedia.Sample{}, ctx.Err()
	case sampleData, ok := <-asp.sampleQueue:
		if !ok {
			// Channel closed, return silence or EOF
			return webrtcmedia.Sample{}, io.EOF
		}

		// Convert PCM bytes to webrtc media.Sample
		// Calculate duration for this sample (using 48kHz to match Opus codec rate)
		// Note: We'll convert 24kHz data to 48kHz when queuing audio
		samplesCount := len(sampleData) / 2 // 16-bit samples
		durationMs := time.Duration(samplesCount*1000/48000) * time.Millisecond

		return webrtcmedia.Sample{
			Data:     sampleData,
			Duration: durationMs,
		}, nil
	}
}

// OnBind is called when the provider is bound to a track
func (asp *AudioSampleProvider) OnBind() error {
	log.Printf("🔗 Audio sample provider bound to track")
	return nil
}

// OnUnbind is called when the provider is unbound from a track
func (asp *AudioSampleProvider) OnUnbind() error {
	log.Printf("🔓 Audio sample provider unbound from track")
	return nil
}

// Close closes the sample provider
func (asp *AudioSampleProvider) Close() error {
	asp.mu.Lock()
	defer asp.mu.Unlock()

	if !asp.closed {
		asp.closed = true
		close(asp.sampleQueue)
		log.Printf("🔒 Audio sample provider closed")
	}
	return nil
}

// QueueAudio queues audio data for streaming
func (asp *AudioSampleProvider) QueueAudio(audioData []byte) error {
	asp.mu.Lock()
	defer asp.mu.Unlock()

	if asp.closed {
		return fmt.Errorf("sample provider is closed")
	}

	select {
	case asp.sampleQueue <- audioData:
		return nil
	default:
		// Queue full, drop the sample or return error
		log.Printf("⚠️ Audio sample queue full, dropping sample")
		return fmt.Errorf("sample queue full")
	}
}

// NewWorker creates a new worker instance
func NewWorker(opts *WorkerOptions) *Worker {
	if opts.Metadata == nil {
		opts.Metadata = make(map[string]string)
	}

	// Initialize STT service if OpenAI API key is available
	var sttService *openai.WhisperSTT
	if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey != "" {
		sttService = openai.NewWhisperSTT(openaiKey)
		log.Printf("🎙️ Whisper STT service initialized")
	}

	// Initialize VAD service with default options
	vadService := vad.NewEnergyVAD(vad.DefaultVADOptions())
	log.Printf("🎤 Energy VAD service initialized")

	// Initialize LLM service if OpenAI API key is available
	var llmService *openai.GPTLLM
	if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey != "" {
		llmService = openai.NewGPTLLM(openaiKey, "gpt-3.5-turbo")
		log.Printf("🧠 GPT LLM service initialized")
	}

	// Initialize TTS service if OpenAI API key is available
	var ttsService *openai.OpenAITTS
	if openaiKey := os.Getenv("OPENAI_API_KEY"); openaiKey != "" {
		ttsService = openai.NewOpenAITTS(openaiKey)
		log.Printf("🎤 OpenAI TTS service initialized")
	}

	return &Worker{
		opts:          opts,
		scheduler:     NewJobScheduler(opts.ExecutorType),
		registry:      GlobalRegistry(),
		jobs:          make(map[string]*JobContext),
		audioBuffers:  make(map[string]*AudioBuffer),
		audioStreams:  make(map[string]*AudioByteStream),
		opusDecoders:  make(map[string]*opus.Decoder),
		vadStreams:    make(map[string]vad.VADStream),
		sttService:    sttService,
		vadService:    vadService,
		conversations: make(map[string]*ConversationContext),
		llmService:    llmService,
		ttsService:    ttsService,
		room:          nil,
	}
}

// NewJobScheduler creates a new job scheduler
func NewJobScheduler(executorType JobExecutorType) *JobScheduler {
	return &JobScheduler{
		executorType: executorType,
		jobQueue:     make(chan *JobContext, 100),
		workers:      make([]*jobWorker, 0),
	}
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return ErrAgentAlreadyStarted
	}
	w.running = true
	w.mu.Unlock()

	log.Printf("Starting worker with agent: %s", w.opts.AgentName)

	// Start job scheduler
	if err := w.scheduler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start job scheduler: %w", err)
	}

	// TODO: Start LiveKit connection and room management
	// This would include connecting to LiveKit server and handling room events

	// Try to connect to a test room
	var testRoom *lksdk.Room
	if w.opts.LiveKitURL != "" && w.opts.APIKey != "" && w.opts.APISecret != "" {
		log.Printf("Attempting to connect to LiveKit room...")
		room, err := w.connectToTestRoom(ctx)
		if err != nil {
			log.Printf("Failed to connect to LiveKit room: %v", err)
			// Continue without room connection for now
		} else {
			log.Printf("Successfully connected to LiveKit room: %s", room.Name())
			testRoom = room
		}
	} else {
		log.Printf("LiveKit credentials not provided, skipping room connection")
	}

	// Submit a test job with the room (if connected)
	if w.opts.EntrypointFunc != nil {
		log.Printf("Submitting test job with entrypoint function")

		// Create a test job context
		jobCtx := NewJobContext(ctx)
		jobCtx.EntrypointFunc = w.opts.EntrypointFunc
		jobCtx.Room = testRoom // This might be nil, which is fine for testing

		// Submit the job
		if err := w.SubmitJob(jobCtx); err != nil {
			log.Printf("Failed to submit test job: %v", err)
		}
	} else {
		log.Printf("No entrypoint function provided in worker options")
	}

	// Wait for context cancellation
	<-ctx.Done()

	return w.Stop()
}

// connectToTestRoom creates a simple connection to a test room
func (w *Worker) connectToTestRoom(ctx context.Context) (*lksdk.Room, error) {
	// For Step 2, just connect to a test room named "test-room"
	roomName := "test-room"

	log.Printf("Connecting to room '%s' at %s", roomName, w.opts.LiveKitURL)

	// Create connection info with credentials
	connectInfo := lksdk.ConnectInfo{
		APIKey:              w.opts.APIKey,
		APISecret:           w.opts.APISecret,
		RoomName:            roomName,
		ParticipantIdentity: w.opts.AgentName,
		ParticipantName:     w.opts.AgentName,
	}

	// Create room callback with participant event handlers
	callback := &lksdk.RoomCallback{
		OnParticipantConnected: func(participant *lksdk.RemoteParticipant) {
			log.Printf("🎯 Participant connected: %s (identity: %s)",
				participant.Name(), participant.Identity())
		},
		OnParticipantDisconnected: func(participant *lksdk.RemoteParticipant) {
			log.Printf("👋 Participant disconnected: %s (identity: %s)",
				participant.Name(), participant.Identity())
		},
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: func(track *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				log.Printf("🎵 Track subscribed: %s from %s (kind: %s)",
					publication.SID(), rp.Identity(), track.Kind().String())

				// Handle audio tracks for speech processing
				if track.Kind() == webrtc.RTPCodecTypeAudio {
					log.Printf("🎤 Audio track detected, starting audio processing...")
					go w.handleAudioTrack(track, publication, rp)
				}
			},
			OnTrackUnsubscribed: func(track *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				log.Printf("🔇 Track unsubscribed: %s from %s",
					publication.SID(), rp.Identity())

				// Clean up audio processing resources for this participant
				if track.Kind() == webrtc.RTPCodecTypeAudio {
					w.mu.Lock()
					// Close VAD stream if it exists
					if vadStream, exists := w.vadStreams[rp.Identity()]; exists {
						vadStream.Close()
						delete(w.vadStreams, rp.Identity())
					}
					delete(w.audioBuffers, rp.Identity())
					delete(w.audioStreams, rp.Identity()) // Clean up AudioByteStream
					delete(w.opusDecoders, rp.Identity())
					w.mu.Unlock()
					log.Printf("🧹 Cleaned up audio resources (including AudioByteStream) for %s", rp.Identity())
				}
			},
			OnTrackPublished: func(publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				log.Printf("📡 Track published: %s from %s (kind: %s, source: %s)",
					publication.SID(), rp.Identity(), publication.Kind().String(), publication.Source().String())

				// CRITICAL FIX: Only subscribe to audio tracks from REAL USERS, not agent
				// This prevents audio feedback loops where agent processes its own TTS output
				if publication.Kind() == lksdk.TrackKindAudio {
					// Filter out agent's own tracks (based on Python LiveKit pattern)
					if rp.Identity() == w.opts.AgentName {
						log.Printf("🚫 Skipping agent's own audio track: %s", publication.SID())
						return
					}
					
					// DIAGNOSTIC: Log all track details for investigation
					log.Printf("🔍 DIAGNOSTIC - Audio track details:")
					log.Printf("  - Participant: %s", rp.Identity())
					log.Printf("  - Track ID: %s", publication.SID())
					log.Printf("  - Source: %s", publication.Source().String())
					log.Printf("  - Kind: %s", publication.Kind().String())
					
					// Only subscribe to SOURCE_MICROPHONE tracks from real users
					if publication.Source() == livekit.TrackSource_MICROPHONE {
						log.Printf("🔔 Auto-subscribing to user audio track from %s", rp.Identity())
						log.Printf("⚠️  WARNING: This may be capturing system audio instead of microphone!")
						if err := publication.SetSubscribed(true); err != nil {
							log.Printf("❌ Failed to subscribe to audio track: %v", err)
						}
					} else {
						log.Printf("🚫 Skipping non-microphone audio track from %s (source: %s)", 
							rp.Identity(), publication.Source().String())
					}
				}
			},
		},
	}

	// Connect to the room
	room, err := lksdk.ConnectToRoom(w.opts.LiveKitURL, connectInfo, callback)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to room: %w", err)
	}

	// Save room reference for conversation processing
	w.mu.Lock()
	w.room = room
	w.mu.Unlock()

	log.Printf("Connected to room successfully! Room name: %s", room.Name())
	
	// DISABLED: Automatic static audio test causes browser disconnection regression
	// The test runs before browsers can properly negotiate codecs, causing immediate disconnection
	// TODO: Re-enable this test only when explicitly requested or after ensuring browser compatibility
	/*
	go func() {
		time.Sleep(2 * time.Second) // Give room time to fully initialize
		log.Printf("🧪 Starting static audio playback test...")
		if err := w.testStaticAudioPlaybook(room); err != nil {
			log.Printf("❌ Static audio test failed: %v", err)
		}
	}()
	*/
	
	return room, nil
}

// Stop stops the worker gracefully
func (w *Worker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	log.Println("Stopping worker...")

	// Stop all active jobs
	for _, job := range w.jobs {
		if job.Session != nil && job.Session.Agent != nil {
			job.Session.Agent.Stop()
		}
	}

	// Stop scheduler
	w.scheduler.Stop()

	w.running = false
	log.Println("Worker stopped")

	return nil
}

// SubmitJob submits a new job for execution
func (w *Worker) SubmitJob(jobCtx *JobContext) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return ErrAgentNotStarted
	}

	// Generate job ID if not set
	if jobCtx.Process.ID == "" {
		jobCtx.Process.ID = fmt.Sprintf("job-%d", time.Now().UnixNano())
	}

	w.jobs[jobCtx.Process.ID] = jobCtx

	return w.scheduler.ScheduleJob(jobCtx)
}

// handleAudioTrack processes audio from a subscribed track using AudioByteStream pattern
func (w *Worker) handleAudioTrack(track *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, participant *lksdk.RemoteParticipant) {
	log.Printf("🎤 Starting audio processing for track %s from %s", publication.SID(), participant.Identity())

	// Create AudioByteStream for this participant (matches LiveKit JS pattern)
	participantID := participant.Identity()
	audioStream := NewAudioByteStream(24000, 1, 16) // 24kHz, mono, 16-bit
	
	w.mu.Lock()
	w.audioStreams[participantID] = audioStream
	w.mu.Unlock()
	
	audioSampleCount := 0
	frameCount := 0

	log.Printf("🔊 Created AudioByteStream for %s: 24kHz, mono, 16-bit", participantID)

	for {
		// Read RTP packet from the track
		rtpPacket, _, readErr := track.ReadRTP()
		if readErr != nil {
			if readErr == io.EOF {
				log.Printf("🔚 Audio track ended for %s", participant.Identity())
				return
			}
			log.Printf("❌ Error reading RTP packet: %v", readErr)
			continue
		}

		// Count packets for monitoring
		audioSampleCount++
		if audioSampleCount%100 == 0 {
			log.Printf("📊 Received %d audio packets from %s (payload type: %d, sequence: %d)",
				audioSampleCount, participant.Identity(), rtpPacket.PayloadType, rtpPacket.SequenceNumber)
		}

		// Phase 1.2: Convert RTP packet to audio data (48kHz → 24kHz conversion included)
		audioData := w.convertRTPToAudio(rtpPacket, participantID)
		if audioData != nil {
			// Phase 1.3: Use AudioByteStream for frame buffering and conversion
			frames := audioStream.WriteBytes(audioData)
			
			// Process any complete frames that were extracted
			for _, frame := range frames {
				frameCount++
				
				if frameCount%5 == 0 { // Log every 5 frames (~500ms at 100ms per frame)
					log.Printf("🎧 Created AudioFrame #%d via AudioByteStream: %s", frameCount, frame.String())
				}
				
				// Process frame with VAD and STT
				go w.processAudioFrame(frame, participantID)
			}
		}
	}
}

// convertRTPToAudio extracts and decodes Opus audio payload from RTP packet to PCM
func (w *Worker) convertRTPToAudio(rtpPacket *rtp.Packet, participantID string) []byte {
	if len(rtpPacket.Payload) == 0 {
		return nil
	}

	// Get or create Opus decoder for this participant
	w.mu.Lock()
	decoder, exists := w.opusDecoders[participantID]
	if !exists {
		// Create new Opus decoder: 48kHz sample rate, 1 channel (mono)
		// Note: Browser audio is still encoded at 48kHz, so decoder stays at 48kHz
		// We'll convert to 24kHz during processing for consistency with TTS output
		var err error
		decoder, err = opus.NewDecoder(48000, 1)
		if err != nil {
			w.mu.Unlock()
			log.Printf("❌ Failed to create Opus decoder for %s: %v", participantID, err)
			return nil
		}
		w.opusDecoders[participantID] = decoder
		log.Printf("🎵 Created Opus decoder for participant: %s (48kHz input, will convert to 24kHz for processing)", participantID)
	}
	w.mu.Unlock()

	// Decode Opus data to PCM
	// Create buffer for decoded PCM data (up to 5760 samples = 120ms at 48kHz)
	pcmBuffer := make([]int16, 5760)

	// Decode the Opus frame
	samplesDecoded, err := decoder.Decode(rtpPacket.Payload, pcmBuffer)
	if err != nil {
		log.Printf("❌ Failed to decode Opus frame from %s: %v", participantID, err)
		return nil
	}

	if samplesDecoded == 0 {
		return nil
	}

	// Convert int16 samples to byte array (little-endian PCM)
	pcmBytes := make([]byte, samplesDecoded*2) // 2 bytes per 16-bit sample
	for i := 0; i < samplesDecoded; i++ {
		sample := pcmBuffer[i]
		pcmBytes[i*2] = byte(sample & 0xff)          // Low byte
		pcmBytes[i*2+1] = byte((sample >> 8) & 0xff) // High byte
	}

	// Convert from 48kHz (browser standard) to 24kHz (our processing standard)
	// This ensures consistent sample rate throughout the processing pipeline
	convertedPCM := w.convertSampleRate(pcmBytes, 48000, 24000, 16)
	
	return convertedPCM
}

// processAudioFrame processes a single audio frame with VAD and STT
func (w *Worker) processAudioFrame(frame *media.AudioFrame, participantID string) {
	// Get or create audio buffer for this participant
	w.mu.Lock()
	buffer, exists := w.audioBuffers[participantID]
	if !exists {
		buffer = &AudioBuffer{
			frames:    make([]*media.AudioFrame, 0, 10), // ~1 second of 100ms frames
			maxFrames: 10,
			lastSTT:   time.Now(),
		}
		w.audioBuffers[participantID] = buffer
	}

	// Get or create VAD stream for this participant
	vadStream, vadExists := w.vadStreams[participantID]
	if !vadExists {
		var err error
		vadStream, err = w.vadService.CreateStream(context.Background())
		if err != nil {
			log.Printf("❌ Failed to create VAD stream for %s: %v", participantID, err)
			w.mu.Unlock()
			return
		}
		w.vadStreams[participantID] = vadStream
		log.Printf("🎤 Created VAD stream for participant: %s", participantID)
	}
	w.mu.Unlock()

	// Process frame with VAD
	vadEvents, err := vadStream.ProcessFrame(context.Background(), frame)
	if err != nil {
		log.Printf("❌ VAD processing failed for %s: %v", participantID, err)
		return
	}

	// Add frame to buffer
	buffer.mu.Lock()
	buffer.frames = append(buffer.frames, frame)

	// Process VAD events
	for _, vadEvent := range vadEvents {
		switch vadEvent.Type {
		case vad.VADEventTypeStartOfSpeech:
			log.Printf("🗣️ Speech started from %s (probability: %.2f)", participantID, vadEvent.Probability)

		case vad.VADEventTypeEndOfSpeech:
			log.Printf("🤐 Speech ended from %s (probability: %.2f)", participantID, vadEvent.Probability)

			// Trigger STT when speech ends (if we have enough frames)
			if len(buffer.frames) >= 1 && w.sttService != nil { // 1 frame = 100ms minimum (now using 100ms frames)
				// Copy frames for processing
				framesToProcess := make([]*media.AudioFrame, len(buffer.frames))
				copy(framesToProcess, buffer.frames)

				// Clear buffer
				buffer.frames = buffer.frames[:0]
				buffer.lastSTT = time.Now()
				buffer.mu.Unlock()

				// Process STT in background goroutine
				go w.processSTTBatch(framesToProcess, participantID)
				return
			}

		case vad.VADEventTypeInferenceDone:
			// Log VAD inference results periodically
			if vadEvent.SamplesIndex%4800 == 0 { // Every ~100ms at 48kHz
				log.Printf("🎤 VAD inference for %s: speaking=%v, probability=%.2f",
					participantID, vadEvent.Speaking, vadEvent.Probability)
			}
		}
	}

	// Check if we should process STT based on time or buffer size (fallback to original logic)
	minFramesForSTT := 1 // 1 * 100ms = 100ms minimum for OpenAI Whisper (now using 100ms frames)
	shouldProcessSTT := len(buffer.frames) >= buffer.maxFrames ||
		(len(buffer.frames) >= minFramesForSTT && time.Since(buffer.lastSTT) > time.Second*2) // Longer timeout when using VAD

	if shouldProcessSTT && w.sttService != nil && len(buffer.frames) >= minFramesForSTT {
		// Copy frames for processing
		framesToProcess := make([]*media.AudioFrame, len(buffer.frames))
		copy(framesToProcess, buffer.frames)

		// Clear buffer
		buffer.frames = buffer.frames[:0]
		buffer.lastSTT = time.Now()
		buffer.mu.Unlock()

		// Process STT in background goroutine
		go w.processSTTBatch(framesToProcess, participantID)
	} else {
		buffer.mu.Unlock()
	}
}

// processSTTBatch processes a batch of audio frames with Whisper STT and conversation processing
func (w *Worker) processSTTBatch(frames []*media.AudioFrame, participantID string) {
	if len(frames) == 0 {
		return
	}

	log.Printf("🎙️ Processing STT batch: %d frames (~%v) from %s",
		len(frames), time.Duration(len(frames))*100*time.Millisecond, participantID)

	// Accumulate frames into a single AudioFrame
	combinedFrame, err := w.sttService.AccumulateAudioForSTT(frames)
	if err != nil {
		log.Printf("❌ Failed to accumulate audio frames: %v", err)
		return
	}

	// 🔇 Audio Quality Assessment - Filter out noise and static
	if w.isLikelyNoise(combinedFrame) {
		log.Printf("🔇 Skipping likely noise/static audio: %d bytes, duration=%v",
			len(combinedFrame.Data), combinedFrame.Duration)
		return
	}

	// 🔍 AUDIO DIAGNOSTICS: Log detailed format information
	log.Printf("🔍 STT Audio Diagnostics for %s:", participantID)
	log.Printf("  - Format: %+v", combinedFrame.Format)
	log.Printf("  - Data size: %d bytes", len(combinedFrame.Data))
	log.Printf("  - Duration: %v", combinedFrame.Duration)
	log.Printf("  - Sample count: %d", combinedFrame.SampleCount())
	log.Printf("  - Timestamp: %v", combinedFrame.Timestamp)

	// 🎙️ DEBUG: Record audio before STT processing for analysis
	if err := w.recordAudioForDebugging(combinedFrame, participantID); err != nil {
		log.Printf("⚠️ Failed to record debug audio: %v", err)
		// Continue processing despite recording failure
	}

	// Send to Whisper STT
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	recognition, err := w.sttService.Recognize(ctx, combinedFrame)
	if err != nil {
		log.Printf("❌ STT recognition failed: %v", err)
		return
	}

	// Process the recognition result with noise filtering
	if recognition.Text != "" {
		log.Printf("🎯 STT Result from %s: '%s' (confidence: %.2f)",
			participantID, recognition.Text, recognition.Confidence)

		// 🚨 SYSTEM AUDIO DETECTION: Check for commercial/advertisement patterns
		if w.isSystemAudioContent(recognition.Text) {
			log.Printf("🚨 SYSTEM AUDIO DETECTED: '%s' - Browser may be capturing system audio instead of microphone!",
				recognition.Text)
			log.Printf("💡 SOLUTION: Check browser audio settings, disable 'Share system audio', ensure microphone-only input")
			return
		}

		// 🔇 Filter out likely false positives from noise/static
		if w.isValidSpeech(recognition.Text, recognition.Confidence) {
			// Process conversation with LLM
			go w.processConversation(participantID, recognition.Text, recognition.Confidence)
		} else {
			log.Printf("🚫 Filtered out likely noise: '%s' (confidence: %.2f)",
				recognition.Text, recognition.Confidence)
		}
	}
}

// Start starts the job scheduler
func (js *JobScheduler) Start(ctx context.Context) error {
	// Start worker goroutines
	numWorkers := 5 // Could be configurable
	for i := 0; i < numWorkers; i++ {
		worker := &jobWorker{
			id:       i,
			jobs:     js.jobQueue,
			quit:     make(chan bool),
			executor: js.executorType,
		}
		js.workers = append(js.workers, worker)
		js.wg.Add(1)
		go func(w *jobWorker) {
			defer js.wg.Done()
			w.start(ctx)
		}(worker)
	}

	return nil
}

// Stop stops the job scheduler
func (js *JobScheduler) Stop() {
	// Send quit signals to all workers (non-blocking to avoid deadlock)
	for _, worker := range js.workers {
		select {
		case worker.quit <- true:
		case <-time.After(1 * time.Second):
			// Worker already exited via context cancellation, which is fine
		}
	}

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		defer close(done)
		js.wg.Wait()
	}()

	// Wait for all workers to stop with overall timeout
	select {
	case <-done:
		// All workers stopped cleanly
	case <-time.After(5 * time.Second):
		log.Println("Warning: Force stopping job workers after 5s timeout")
	}
}

// ScheduleJob schedules a job for execution
func (js *JobScheduler) ScheduleJob(jobCtx *JobContext) error {
	select {
	case js.jobQueue <- jobCtx:
		return nil
	default:
		return fmt.Errorf("job queue is full")
	}
}

// start starts a job worker
func (jw *jobWorker) start(ctx context.Context) {
	for {
		select {
		case job := <-jw.jobs:
			jw.executeJob(ctx, job)
		case <-jw.quit:
			return
		case <-ctx.Done():
			return
		}
	}
}

// executeJob executes a job with proper cancellation support
func (jw *jobWorker) executeJob(ctx context.Context, jobCtx *JobContext) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Job %s panicked: %v", jobCtx.Process.ID, r)
			jobCtx.Process.UpdateStatus(JobStatusFailed)
		}
	}()

	log.Printf("Worker %d executing job %s", jw.id, jobCtx.Process.ID)

	jobCtx.Process.UpdateStatus(JobStatusRunning)

	// Execute the entrypoint function in a separate goroutine with cancellation
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("job panicked: %v", r)
			}
		}()

		// Call the actual entrypoint function
		if jobCtx.EntrypointFunc != nil {
			log.Printf("Calling entrypoint function for job %s", jobCtx.Process.ID)
			err := jobCtx.EntrypointFunc(jobCtx)
			done <- err
		} else {
			log.Printf("No entrypoint function for job %s", jobCtx.Process.ID)
			done <- fmt.Errorf("no entrypoint function provided")
		}
	}()

	// Wait for job completion or cancellation
	select {
	case err := <-done:
		if err != nil {
			if err == context.Canceled {
				log.Printf("Job %s cancelled", jobCtx.Process.ID)
				jobCtx.Process.UpdateStatus(JobStatusCancelled)
			} else {
				log.Printf("Job %s failed: %v", jobCtx.Process.ID, err)
				jobCtx.Process.UpdateStatus(JobStatusFailed)
			}
		} else {
			jobCtx.Process.UpdateStatus(JobStatusCompleted)
			log.Printf("Job %s completed", jobCtx.Process.ID)
		}
	case <-ctx.Done():
		// Context cancelled while waiting
		log.Printf("Job %s cancelled due to context", jobCtx.Process.ID)
		jobCtx.Process.UpdateStatus(JobStatusCancelled)
	}
}

// getOrCreateConversationContext gets or creates conversation context for a participant
func (w *Worker) getOrCreateConversationContext(participantID string) *ConversationContext {
	w.mu.Lock()
	defer w.mu.Unlock()

	if ctx, exists := w.conversations[participantID]; exists {
		ctx.LastActivity = time.Now()
		return ctx
	}

	ctx := &ConversationContext{
		ParticipantID:       participantID,
		ConversationHistory: make([]ConversationTurn, 0),
		LastActivity:        time.Now(),
	}
	w.conversations[participantID] = ctx
	log.Printf("💬 Created conversation context for participant: %s", participantID)
	return ctx
}

// addConversationTurn adds a turn to the conversation history
func (w *Worker) addConversationTurn(participantID, speaker, text string, confidence float64) {
	ctx := w.getOrCreateConversationContext(participantID)

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	turn := ConversationTurn{
		Timestamp:  time.Now(),
		Speaker:    speaker,
		Text:       text,
		Confidence: confidence,
	}

	ctx.ConversationHistory = append(ctx.ConversationHistory, turn)

	// Keep conversation history manageable (last 20 turns)
	if len(ctx.ConversationHistory) > 20 {
		ctx.ConversationHistory = ctx.ConversationHistory[len(ctx.ConversationHistory)-20:]
	}

	log.Printf("💬 Added conversation turn [%s]: %s", speaker, text)
}

// processConversation processes STT text through LLM to generate intelligent responses
func (w *Worker) processConversation(participantID, sttText string, confidence float64) {
	if w.llmService == nil {
		log.Printf("⚠️ LLM service not available, skipping conversation processing")
		return
	}

	// Add user turn to conversation history
	w.addConversationTurn(participantID, "user", sttText, confidence)

	// Get conversation context
	ctx := w.getOrCreateConversationContext(participantID)

	// Build conversation messages for LLM
	messages := w.buildLLMMessages(ctx)

	// Process through LLM
	chatOpts := &llm.ChatOptions{
		MaxTokens:   150,
		Temperature: 0.7,
	}

	log.Printf("🧠 Processing conversation with LLM: '%s'", sttText)

	chatCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	completion, err := w.llmService.Chat(chatCtx, messages, chatOpts)
	if err != nil {
		log.Printf("❌ LLM conversation processing failed: %v", err)
		return
	}

	response := completion.Message.Content
	if response == "" {
		log.Printf("⚠️ LLM returned empty response")
		return
	}

	// Add assistant turn to conversation history
	w.addConversationTurn(participantID, "assistant", response, 1.0)

	// Publish response to room using TTS
	w.publishResponse(participantID, response, completion.Usage)
}

// buildLLMMessages builds LLM messages from conversation history
func (w *Worker) buildLLMMessages(ctx *ConversationContext) []llm.Message {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	messages := []llm.Message{
		{
			Role: llm.RoleSystem,
			Content: `You are a helpful and friendly voice assistant. You're having a live voice conversation with a user through LiveKit. 
			
Key guidelines:
- Keep responses concise and conversational (1-2 sentences typically)
- Be natural and engaging, as if speaking face-to-face
- Ask follow-up questions to keep the conversation flowing
- Be helpful and provide useful information when asked
- If the user's speech is unclear or just a fragment, acknowledge it and ask for clarification
- Remember this is a voice conversation, so avoid overly complex or lengthy responses

The user is speaking to you through their microphone, and you'll respond via text in the chat.`,
		},
	}

	// Add conversation history (limit to last 10 turns for context management)
	historyLimit := 10
	startIdx := 0
	if len(ctx.ConversationHistory) > historyLimit {
		startIdx = len(ctx.ConversationHistory) - historyLimit
	}

	for _, turn := range ctx.ConversationHistory[startIdx:] {
		var role llm.MessageRole
		if turn.Speaker == "user" {
			role = llm.RoleUser
		} else {
			role = llm.RoleAssistant
		}

		messages = append(messages, llm.Message{
			Role:    role,
			Content: turn.Text,
		})
	}

	return messages
}

// publishResponse publishes LLM response back to LiveKit room using TTS
func (w *Worker) publishResponse(participantID, response string, usage llm.Usage) {
	w.mu.RLock()
	room := w.room
	ttsService := w.ttsService
	w.mu.RUnlock()

	if room == nil {
		log.Printf("❌ No room available to publish response")
		return
	}

	log.Printf("🧠 LLM generated response: %s", response)
	log.Printf("📊 Token usage: %d prompt + %d completion = %d total",
		usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)

	// Synthesize speech using TTS
	if ttsService != nil {
		log.Printf("🎤 Synthesizing speech for response...")

		// Create TTS options (use 24kHz to match LiveKit JS agents and PCM track configuration)
		ttsOpts := &tts.SynthesizeOptions{
			Voice:      "alloy", // OpenAI voice (alloy, echo, fable, onyx, nova, shimmer)
			Speed:      1.0,
			Volume:     1.0,
			Format: media.AudioFormat{
				SampleRate:    24000,
				Channels:      1,
				BitsPerSample: 16,
				Format:        media.AudioFormatPCM,
			}, // Match our 24kHz processing pipeline
			SampleRate: 24000,                           // Match OpenAI TTS standard from JS agents
		}

		// Synthesize the speech
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		audioFrame, err := ttsService.Synthesize(ctx, response, ttsOpts)
		if err != nil {
			log.Printf("❌ TTS synthesis failed: %v", err)
			// Fall back to text-only response
			log.Printf("📡 Response ready to publish (text-only): %s", response)
			return
		}

		log.Printf("🎵 TTS synthesis successful: %d bytes of %s audio",
			len(audioFrame.Data), audioFrame.Format.Format)
		log.Printf("🎵 Audio frame details: %s", audioFrame.String())

		// Publish both audio and text
		w.publishAudioResponse(room, audioFrame)
		w.publishTextResponse(room, response)

	} else {
		log.Printf("⚠️ TTS service not available, publishing text-only response")

		// Publish text response as data message
		err := room.LocalParticipant.PublishData([]byte(response), lksdk.WithDataPublishReliable(true))
		if err != nil {
			log.Printf("❌ Failed to publish text response: %v", err)
		} else {
			log.Printf("📡 Published text response to room: %s", response)
		}
	}
}

// publishAudioResponse publishes audio to the LiveKit room as an actual audio track
func (w *Worker) publishAudioResponse(room *lksdk.Room, audioFrame *media.AudioFrame) {
	log.Printf("🎵 Publishing audio frame: %s", audioFrame.String())
	
	// 🔍 TTS AUDIO DIAGNOSTICS: Log detailed format information
	log.Printf("🔍 TTS Audio Diagnostics:")
	log.Printf("  - Format: %+v", audioFrame.Format)
	log.Printf("  - Data size: %d bytes", len(audioFrame.Data))
	log.Printf("  - Duration: %v", audioFrame.Duration)
	log.Printf("  - Sample count: %d", audioFrame.SampleCount())
	log.Printf("  - Calculated duration: %v", time.Duration(audioFrame.SampleCount()*1000/audioFrame.Format.SampleRate)*time.Millisecond)

	// Check if we already have an audio track published
	if w.audioTrack == nil {
		// Create a new audio track for the assistant
		err := w.createAssistantAudioTrack(room)
		if err != nil {
			log.Printf("❌ Failed to create assistant audio track: %v", err)
			return
		}
	}

	// Convert PCM audio frame to the format LiveKit expects
	err := w.streamAudioToTrack(audioFrame)
	if err != nil {
		log.Printf("❌ Failed to stream audio to track: %v", err)
		return
	}

	log.Printf("🔊 Successfully streamed %d bytes of audio to LiveKit track", len(audioFrame.Data))
}

// publishTextResponse publishes a text response to the LiveKit room
func (w *Worker) publishTextResponse(room *lksdk.Room, response string) {
	err := room.LocalParticipant.PublishData([]byte(response), lksdk.WithDataPublishReliable(true))
	if err != nil {
		log.Printf("❌ Failed to publish text response: %v", err)
	} else {
		log.Printf("📡 Published text response: %s", response)
	}
}

// createAssistantAudioTrack creates and publishes an audio track for the assistant using PCM local track
func (w *Worker) createAssistantAudioTrack(room *lksdk.Room) error {
	log.Printf("🎤 Creating assistant audio track with automatic codec negotiation...")

	// Create audio sample provider for streaming TTS audio
	w.audioProvider = NewAudioSampleProvider()

	// CRITICAL FIX: Use default WebRTC Opus codec (no explicit configuration)
	// JavaScript LiveKit agents use webrtc.MimeTypeOpus with default parameters
	// This matches browser expectations and avoids codec compatibility issues
	
	log.Printf("🔍 Audio Track Configuration (Default WebRTC Opus):")
	log.Printf("  - Using: webrtc.MimeTypeOpus (browser standard)")
	log.Printf("  - No explicit parameters - automatic WebRTC negotiation")

	// Create a local audio track with default Opus codec (matches JS agent pattern)
	localTrack, err := lksdk.NewLocalSampleTrack(webrtc.RTPCodecCapability{
		MimeType: webrtc.MimeTypeOpus, // Use constant from webrtc library
	})
	if err != nil {
		return fmt.Errorf("failed to create local sample track: %w", err)
	}

	// Start the sample provider writing to the track
	err = localTrack.StartWrite(w.audioProvider, func() {
		log.Printf("🎤 Audio track write completed")
	})
	if err != nil {
		return fmt.Errorf("failed to start sample provider: %w", err)
	}

	// Publish the track to the room using MICROPHONE source (matches LiveKit JS agent pattern)
	// CRITICAL FIX: JavaScript LiveKit agents always use TrackSource.SOURCE_MICROPHONE for TTS audio
	// This signals to browser that it's a voice track and enables proper codec compatibility
	trackOptions := &lksdk.TrackPublicationOptions{
		Name:   "assistant-voice",
		Source: livekit.TrackSource_MICROPHONE, // Browser-compatible voice track designation
	}

	publication, err := room.LocalParticipant.PublishTrack(localTrack, trackOptions)
	if err != nil {
		return fmt.Errorf("failed to publish assistant audio track: %w", err)
	}

	// Store the track publication for future use
	w.audioTrack = publication
	w.isPublishingAudio = true

	log.Printf("✅ Assistant audio track created with default WebRTC Opus: %s", publication.SID())
	log.Printf("🔍 Audio track format: Default Opus codec (automatic WebRTC parameters)")
	log.Printf("🛡️ Track source: %s (will be filtered from STT input to prevent feedback)", publication.Source().String())
	return nil
}

// streamAudioToTrack converts and streams PCM audio data to the LiveKit track via SampleProvider
func (w *Worker) streamAudioToTrack(audioFrame *media.AudioFrame) error {
	if w.audioProvider == nil {
		return fmt.Errorf("no audio provider available")
	}

	// Convert audio frame to the format expected by our SampleProvider (24kHz PCM16)
	audioData, err := w.convertAudioFrameToPCM16(audioFrame)
	if err != nil {
		return fmt.Errorf("failed to convert audio frame: %w", err)
	}

	// Queue the audio data to the sample provider
	err = w.audioProvider.QueueAudio(audioData)
	if err != nil {
		return fmt.Errorf("failed to queue audio data: %w", err)
	}

	log.Printf("🎵 Successfully queued %d bytes of 48kHz PCM16 data to SampleProvider", len(audioData))
	return nil
}

// convertAudioFrameToPCM16 converts our AudioFrame to 48kHz PCM16 format for the Opus codec
func (w *Worker) convertAudioFrameToPCM16(audioFrame *media.AudioFrame) ([]byte, error) {
	// Convert to 48kHz PCM16 format as expected by Opus codec in browsers
	targetSampleRate := 48000
	
	if audioFrame.Format.SampleRate == targetSampleRate {
		// Already at target rate and format, use as-is
		return audioFrame.Data, nil
	} else if audioFrame.Format.SampleRate == 24000 {
		// Our processing rate - upsample to 48kHz for Opus codec
		return w.convertSampleRate(audioFrame.Data, 24000, 48000, audioFrame.Format.BitsPerSample), nil
	} else if audioFrame.Format.SampleRate == 22050 {
		// Common ElevenLabs rate - upsample to 48kHz
		return w.convertSampleRate(audioFrame.Data, 22050, 48000, audioFrame.Format.BitsPerSample), nil
	} else if audioFrame.Format.SampleRate == 16000 {
		// Common for some TTS services - upsample to 48kHz
		return w.convertSampleRate(audioFrame.Data, 16000, 48000, audioFrame.Format.BitsPerSample), nil
	} else {
		return nil, fmt.Errorf("unsupported sample rate for Opus codec conversion: %d Hz", audioFrame.Format.SampleRate)
	}
}

// isLikelyNoise checks if the audio is likely noise or static
func (w *Worker) isLikelyNoise(audioFrame *media.AudioFrame) bool {
	// Check for very short audio (likely noise bursts)
	if audioFrame.Duration < 200*time.Millisecond {
		return true
	}

	// TODO: Add more sophisticated noise detection:
	// - Audio energy analysis (very low or very high energy)
	// - Frequency analysis for static/hum patterns
	// - Zero-crossing rate analysis

	return false
}

// isSystemAudioContent detects if transcribed text appears to be from system audio/advertisements
func (w *Worker) isSystemAudioContent(text string) bool {
	cleanText := strings.ToLower(strings.TrimSpace(text))
	
	// Patterns that indicate system audio/advertisements/media
	systemAudioPatterns := []string{
		// Commercial patterns
		"beadaholique.com",
		"for all of your",
		"supply needs",
		"visit our website",
		"www.",
		".com",
		"disclaimer",
		"zeoranger.co.uk",
		"subs by",
		"bulletproof executive",
		"© the",
		"copyright",
		
		// Advertisement phrases
		"call now",
		"limited time",
		"order today",
		"free shipping",
		"sale ends",
		"special offer",
		"click here",
		"learn more",
		"sign up",
		"subscribe",
		
		// Media/subtitle patterns
		"previously on",
		"next episode",
		"season finale",
		"credits",
		"sponsored by",
		"brought to you by",
	}
	
	// Check for commercial domain patterns
	if strings.Contains(cleanText, ".com") || strings.Contains(cleanText, ".co.uk") || 
	   strings.Contains(cleanText, "www.") || strings.Contains(cleanText, "http") {
		return true
	}
	
	// Check for multi-word commercial phrases
	for _, pattern := range systemAudioPatterns {
		if strings.Contains(cleanText, pattern) {
			return true
		}
	}
	
	// Check for very long commercial-style sentences
	if len(strings.Fields(cleanText)) > 8 && strings.Contains(cleanText, "!") {
		return true
	}
	
	return false
}

// isValidSpeech checks if the transcribed text represents valid speech or is likely noise
func (w *Worker) isValidSpeech(text string, confidence float64) bool {
	// Clean up the text
	cleanText := strings.TrimSpace(strings.ToLower(text))

	// Filter out empty or very short texts
	if len(cleanText) == 0 || len(cleanText) == 1 {
		return false
	}

	// Known noise artifacts that Whisper commonly mistakes for speech
	noisePatterns := []string{
		"you", // Very common false positive from background noise
		"uh",  // Common for static
		"oh",  // Common for hum/noise
		"ah",  // Common for ambient noise
		"mm",  // Common for low-frequency noise
		"hmm", // Common for hum
		"um",  // Can be noise artifact
		"the", // Sometimes noise gets transcribed as "the"
		"a",   // Single letter often from noise
		"i",   // Single letter often from noise
	}

	// Check if the text exactly matches a noise pattern
	for _, pattern := range noisePatterns {
		if cleanText == pattern {
			log.Printf("🔇 Detected noise pattern: '%s'", cleanText)
			return false
		}
	}

	// Check for very low confidence (likely noise)
	if confidence < 0.7 {
		log.Printf("🔇 Low confidence transcription: '%s' (%.2f)", cleanText, confidence)
		return false
	}

	// Check for repetitive single-word patterns (common with noise)
	words := strings.Fields(cleanText)
	if len(words) == 1 && len(words[0]) <= 3 {
		log.Printf("🔇 Short single word (likely noise): '%s'", cleanText)
		return false
	}

	// If it passes all filters, it's likely valid speech
	return true
}

// recordAudioForDebugging saves audio data to a WAV file for manual analysis
func (w *Worker) recordAudioForDebugging(audioFrame *media.AudioFrame, participantID string) error {
	// Create timestamp for filename
	timestamp := time.Now().Format("20060102-150405.000")
	filename := fmt.Sprintf("debug-audio/audio-%s-%s.wav", participantID, timestamp)
	
	log.Printf("🎙️ Recording debug audio: %s (format: %+v, size: %d bytes, duration: %v)",
		filename, audioFrame.Format, len(audioFrame.Data), audioFrame.Duration)
	
	// Convert audio frame to WAV format using same logic as OpenAI STT
	wavData, err := w.convertAudioFrameToWAV(audioFrame)
	if err != nil {
		return fmt.Errorf("failed to convert audio to WAV: %w", err)
	}
	
	// Write WAV data to file
	if err := os.WriteFile(filename, wavData, 0644); err != nil {
		return fmt.Errorf("failed to write audio file: %w", err)
	}
	
	log.Printf("✅ Debug audio saved: %s (%d bytes WAV)", filename, len(wavData))
	return nil
}

// convertAudioFrameToWAV converts an AudioFrame to WAV format
// This is similar to the OpenAI STT convertToWAV function but standalone
func (w *Worker) convertAudioFrameToWAV(audio *media.AudioFrame) ([]byte, error) {
	var buf bytes.Buffer

	// RIFF header
	buf.WriteString("RIFF")
	fileSize := uint32(36 + len(audio.Data)) // Total file size - 8 bytes
	binary.Write(&buf, binary.LittleEndian, fileSize)
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	chunkSize := uint32(16) // PCM format chunk size
	binary.Write(&buf, binary.LittleEndian, chunkSize)
	
	audioFormat := uint16(1) // PCM
	binary.Write(&buf, binary.LittleEndian, audioFormat)
	
	numChannels := uint16(audio.Format.Channels)
	binary.Write(&buf, binary.LittleEndian, numChannels)
	
	sampleRate := uint32(audio.Format.SampleRate)
	binary.Write(&buf, binary.LittleEndian, sampleRate)
	
	bitsPerSample := uint16(audio.Format.BitsPerSample)
	byteRate := uint32(sampleRate * uint32(numChannels) * uint32(bitsPerSample) / 8)
	binary.Write(&buf, binary.LittleEndian, byteRate)
	
	blockAlign := uint16(numChannels * bitsPerSample / 8)
	binary.Write(&buf, binary.LittleEndian, blockAlign)
	
	binary.Write(&buf, binary.LittleEndian, bitsPerSample)

	// data chunk
	buf.WriteString("data")
	dataSize := uint32(len(audio.Data))
	binary.Write(&buf, binary.LittleEndian, dataSize)
	
	// Write audio data
	buf.Write(audio.Data)

	return buf.Bytes(), nil
}

// testStaticAudioPlayback tests browser audio playback using a known good audio file
func (w *Worker) testStaticAudioPlayback(room *lksdk.Room) error {
	log.Printf("🎵 Testing static audio playback with test file...")
	
	// Read the test audio file
	audioData, err := os.ReadFile("debug-audio/test-static.wav")
	if err != nil {
		return fmt.Errorf("failed to read test audio file: %w", err)
	}
	
	log.Printf("📁 Loaded test audio file: %d bytes", len(audioData))
	
	// Parse WAV header to get format information
	format, pcmData, err := w.parseWAVFile(audioData)
	if err != nil {
		return fmt.Errorf("failed to parse WAV file: %w", err)
	}
	
	log.Printf("🎶 Test audio format: %+v, PCM data: %d bytes", format, len(pcmData))
	
	// Create audio frame
	audioFrame := media.NewAudioFrame(pcmData, format)
	
	// If we don't have an audio track yet, create one
	if !w.isPublishingAudio {
		if err := w.createAssistantAudioTrack(room); err != nil {
			return fmt.Errorf("failed to create audio track for test: %w", err)
		}
		// Give the track a moment to be ready
		time.Sleep(100 * time.Millisecond)
	}
	
	// Stream the static audio data
	log.Printf("🎵 Starting static audio playback...")
	
	// Convert sample rate if needed (codec expects 48kHz for browser compatibility)
	if format.SampleRate != 48000 {
		log.Printf("⚡ Converting audio from %dHz to 48kHz for codec compatibility", format.SampleRate)
		convertedPCM := w.convertSampleRate(pcmData, format.SampleRate, 48000, format.BitsPerSample)
		format.SampleRate = 48000
		audioFrame = media.NewAudioFrame(convertedPCM, format)
	}
	
	// Split audio into chunks and send through audio provider (using 100ms chunks)
	chunkSize := 4800 * (format.BitsPerSample / 8) // 100ms at 48kHz for Opus codec
	if format.SampleRate == 48000 {
		chunkSize = 4800 * (format.BitsPerSample / 8) // 100ms at 48kHz
	}
	
	for i := 0; i < len(audioFrame.Data); i += chunkSize {
		end := i + chunkSize
		if end > len(audioFrame.Data) {
			end = len(audioFrame.Data)
		}
		
		chunk := audioFrame.Data[i:end]
		if len(chunk) > 0 {
			// Send chunk through audio provider
			if w.audioProvider != nil {
				if err := w.audioProvider.QueueAudio(chunk); err != nil {
					log.Printf("❌ Failed to queue audio chunk: %v", err)
				}
			}
			
			// Add delay to match real-time playback (100ms chunks)
			time.Sleep(100 * time.Millisecond)
		}
	}
	
	log.Printf("✅ Static audio playback test completed")
	return nil
}

// parseWAVFile parses a WAV file and returns format info and PCM data
func (w *Worker) parseWAVFile(data []byte) (media.AudioFormat, []byte, error) {
	if len(data) < 44 {
		return media.AudioFormat{}, nil, fmt.Errorf("WAV file too short")
	}
	
	// Check RIFF header
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return media.AudioFormat{}, nil, fmt.Errorf("invalid WAV file format")
	}
	
	// Find fmt chunk
	offset := 12
	for offset < len(data)-8 {
		chunkID := string(data[offset:offset+4])
		chunkSize := binary.LittleEndian.Uint32(data[offset+4:offset+8])
		
		if chunkID == "fmt " {
			if chunkSize < 16 {
				return media.AudioFormat{}, nil, fmt.Errorf("invalid fmt chunk size")
			}
			
			audioFormat := binary.LittleEndian.Uint16(data[offset+8:offset+10])
			if audioFormat != 1 { // PCM
				return media.AudioFormat{}, nil, fmt.Errorf("only PCM format supported")
			}
			
			channels := binary.LittleEndian.Uint16(data[offset+10:offset+12])
			sampleRate := binary.LittleEndian.Uint32(data[offset+12:offset+16])
			bitsPerSample := binary.LittleEndian.Uint16(data[offset+22:offset+24])
			
			format := media.AudioFormat{
				SampleRate:    int(sampleRate),
				Channels:      int(channels),
				BitsPerSample: int(bitsPerSample),
				Format:        media.AudioFormatPCM,
			}
			
			// Find data chunk
			dataOffset := offset + 8 + int(chunkSize)
			for dataOffset < len(data)-8 {
				dataChunkID := string(data[dataOffset:dataOffset+4])
				dataChunkSize := binary.LittleEndian.Uint32(data[dataOffset+4:dataOffset+8])
				
				if dataChunkID == "data" {
					pcmStart := dataOffset + 8
					pcmEnd := pcmStart + int(dataChunkSize)
					if pcmEnd > len(data) {
						pcmEnd = len(data)
					}
					return format, data[pcmStart:pcmEnd], nil
				}
				
				dataOffset += 8 + int(dataChunkSize)
			}
			
			return media.AudioFormat{}, nil, fmt.Errorf("data chunk not found")
		}
		
		offset += 8 + int(chunkSize)
	}
	
	return media.AudioFormat{}, nil, fmt.Errorf("fmt chunk not found")
}

// convertSampleRate performs basic sample rate conversion
func (w *Worker) convertSampleRate(pcmData []byte, fromRate, toRate, bitsPerSample int) []byte {
	if fromRate == toRate {
		return pcmData
	}
	
	bytesPerSample := bitsPerSample / 8
	sampleCount := len(pcmData) / bytesPerSample
	ratio := float64(toRate) / float64(fromRate)
	newSampleCount := int(float64(sampleCount) * ratio)
	
	result := make([]byte, newSampleCount * bytesPerSample)
	
	for i := 0; i < newSampleCount; i++ {
		srcIndex := int(float64(i) / ratio)
		if srcIndex >= sampleCount {
			srcIndex = sampleCount - 1
		}
		
		srcOffset := srcIndex * bytesPerSample
		dstOffset := i * bytesPerSample
		
		copy(result[dstOffset:dstOffset+bytesPerSample], pcmData[srcOffset:srcOffset+bytesPerSample])
	}
	
	return result
}
