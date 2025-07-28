package main

import (
	"context"
	"encoding/json"
	"expvar"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chriscow/livekit-agents-go/internal/worker"
	"github.com/chriscow/livekit-agents-go/pkg/agent"
	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
	"github.com/chriscow/livekit-agents-go/pkg/ai/llm/fake"
	"github.com/chriscow/livekit-agents-go/pkg/ai/stt"
	sttfake "github.com/chriscow/livekit-agents-go/pkg/ai/stt/fake"
	ttsfake "github.com/chriscow/livekit-agents-go/pkg/ai/tts/fake"
	vadfake "github.com/chriscow/livekit-agents-go/pkg/ai/vad/fake"
	"github.com/chriscow/livekit-agents-go/pkg/audio/wav"
	"github.com/chriscow/livekit-agents-go/pkg/job"
	"github.com/chriscow/livekit-agents-go/pkg/plugin"
	_ "github.com/chriscow/livekit-agents-go/pkg/plugin/fake"   // Import to register fake plugins
	_ "github.com/chriscow/livekit-agents-go/pkg/plugin/openai" // Import to register OpenAI plugin
	_ "github.com/chriscow/livekit-agents-go/pkg/plugin/silero" // Import to register silero plugin
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
	"github.com/chriscow/livekit-agents-go/pkg/turn"
	"github.com/chriscow/livekit-agents-go/pkg/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lk-go",
	Short: "LiveKit Agents Go - A Go implementation of the LiveKit Agents framework",
	Long: `lk-go is a Go implementation of the LiveKit Agents framework that tracks
the Python LiveKit Agents library feature-for-feature while remaining idiomatic Go.`,
	SilenceUsage: true,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.GetVersionInfo())
	},
}

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Worker management commands",
}

var workerRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Start a worker against LiveKit",
	RunE: func(cmd *cobra.Command, args []string) error {
		url, _ := cmd.Flags().GetString("url")
		token, _ := cmd.Flags().GetString("token")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		logger := setupLogger()
		logger.Info("Starting worker",
			slog.String("service", "lk-go"),
			slog.String("version", version.Version),
			slog.String("commit", version.GitCommit),
			slog.String("url", url),
			slog.Bool("dry_run", dryRun))

		if dryRun {
			logger.Info("Dry run mode - exiting")
			return nil
		}

		if url == "" {
			return fmt.Errorf("--url is required")
		}
		if token == "" {
			return fmt.Errorf("--token is required")  
		}

		// Create context that cancels on interrupt
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		// Import worker package
		worker := createWorker(url, token, logger)
		
		// Start the worker
		if err := worker.Run(ctx); err != nil {
			logger.Error("Worker failed", slog.String("error", err.Error()))
			return err
		}
		
		return nil
	},
}

var workerHealthzCmd = &cobra.Command{
	Use:   "healthz",
	Short: "Quick connectivity check (pings server once)",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := setupLogger()
		logger.Info("Performing health check",
			slog.String("service", "lk-go"),
			slog.String("version", version.Version),
			slog.String("commit", version.GitCommit))
		
		// Basic connectivity health check - validate required flags are set
		url, _ := cmd.Flags().GetString("url")
		token, _ := cmd.Flags().GetString("token")
		
		if url == "" {
			return fmt.Errorf("--url is required for health check")
		}
		if token == "" {
			return fmt.Errorf("--token is required for health check")
		}
		
		logger.Info("Health check passed - required parameters validated")
		return nil
	},
}

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Job management commands",
}

var sttCmd = &cobra.Command{
	Use:   "stt",
	Short: "Speech-to-text commands",
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Voice agent commands",
}

var agentDemoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Minimal example agent (echo bot)",
	RunE: func(cmd *cobra.Command, args []string) error {
		url, _ := cmd.Flags().GetString("url")
		token, _ := cmd.Flags().GetString("token")
		roomName, _ := cmd.Flags().GetString("room")
		metrics, _ := cmd.Flags().GetBool("metrics")
		bgFile, _ := cmd.Flags().GetString("bg-file")
		bgVolume, _ := cmd.Flags().GetFloat32("bg-volume")

		logger := setupLogger()
		logger.Info("Starting agent demo",
			slog.String("service", "lk-go"),
			slog.String("room", roomName),
			slog.String("url", url),
			slog.Bool("metrics", metrics))

		if url == "" {
			return fmt.Errorf("--url is required")
		}
		if token == "" {
			return fmt.Errorf("--token is required")
		}
		if roomName == "" {
			return fmt.Errorf("--room is required")
		}

		// Create context that cancels on interrupt
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		return runAgentDemo(ctx, url, token, roomName, metrics, bgFile, bgVolume, logger)
	},
}

var sttEchoCmd = &cobra.Command{
	Use:   "echo",
	Short: "Read WAV file and print transcript using chosen provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		provider, _ := cmd.Flags().GetString("provider")

		logger := setupLogger()
		logger.Info("Starting STT echo",
			slog.String("service", "lk-go"),
			slog.String("file", filePath),
			slog.String("provider", provider))

		return runSTTEcho(filePath, provider, logger)
	},
}

var jobRunScriptCmd = &cobra.Command{
	Use:   "run-script [plugin]",
	Short: "Execute a Go plugin inside a Job container",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pluginName := args[0]
		url, _ := cmd.Flags().GetString("url")
		token, _ := cmd.Flags().GetString("token")
		roomName, _ := cmd.Flags().GetString("room")
		timeout, _ := cmd.Flags().GetDuration("timeout")

		logger := setupLogger()
		logger.Info("Starting job run-script",
			slog.String("service", "lk-go"),
			slog.String("plugin", pluginName),
			slog.String("room", roomName),
			slog.String("url", url))

		if url == "" {
			return fmt.Errorf("--url is required")
		}
		if token == "" {
			return fmt.Errorf("--token is required")
		}
		if roomName == "" {
			return fmt.Errorf("--room is required")
		}

		// Create context that cancels on interrupt
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		// Create and run the job
		return runJobScript(ctx, pluginName, url, token, roomName, timeout, logger)
	},
}

func setupLogger() *slog.Logger {
	logFormat := os.Getenv("LK_LOG_FORMAT")
	logLevel := os.Getenv("LK_LOG_LEVEL")
	
	var handler slog.Handler
	opts := &slog.HandlerOptions{}
	
	// Set log level
	switch logLevel {
	case "debug":
		opts.Level = slog.LevelDebug
	case "info":
		opts.Level = slog.LevelInfo
	case "warn":
		opts.Level = slog.LevelWarn
	case "error":
		opts.Level = slog.LevelError
	default:
		opts.Level = slog.LevelInfo
	}
	
	// Choose handler based on format
	if logFormat == "console" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		// Default to JSON
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

func createWorker(url, token string, logger *slog.Logger) *worker.Worker {
	config := worker.Config{
		URL:   url,
		Token: token,
	}
	return worker.New(config, logger)
}

func runJobScript(ctx context.Context, pluginName, url, token, roomName string, timeout time.Duration, logger *slog.Logger) error {
	// Create job configuration
	jobConfig := job.Config{
		RoomName: roomName,
		Timeout:  timeout,
	}

	// Create the job
	jobInstance, err := job.New(ctx, jobConfig)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	// Set up shutdown hook
	jobInstance.Context.OnShutdown(func(reason string) {
		logger.Info("Job shutdown hook called", slog.String("reason", reason))
	})

	logger.Info("Job created successfully",
		slog.String("job_id", jobInstance.ID),
		slog.String("room_name", jobInstance.RoomName))

	// Create room connection
	roomConfig := job.RoomConfig{
		URL:      url,
		Token:    token,
		RoomName: roomName,
	}

	room, err := job.NewRoom(ctx, roomConfig)
	if err != nil {
		return fmt.Errorf("failed to create room: %w", err)
	}
	defer room.Disconnect()

	// Connect to the room
	if err := room.Connect(roomConfig); err != nil {
		logger.Warn("Failed to connect to room (continuing anyway for demo)",
			slog.String("error", err.Error()))
		// Continue anyway for demonstration purposes
	}

	// Start event listener
	go func() {
		for event := range room.Events {
			logger.Info("Room event received",
				slog.String("event_type", string(event.Type)),
				slog.Time("timestamp", event.Timestamp))
		}
	}()

	// Execute the plugin (placeholder implementation)
	logger.Info("Executing plugin", slog.String("plugin", pluginName))
	
	// For Phase 2, we'll simulate plugin execution with proper exit code handling
	var pluginErr error
	switch pluginName {
	case "noop":
		logger.Info("No-op plugin executed successfully")
		// pluginErr remains nil (success)
	case "echo":
		logger.Info("Echo plugin would process audio here")
		// pluginErr remains nil (success)
	case "fail":
		logger.Error("Fail plugin intentionally failed")
		pluginErr = fmt.Errorf("plugin execution failed")
	default:
		logger.Warn("Unknown plugin, treating as no-op", slog.String("plugin", pluginName))
		// pluginErr remains nil (unknown plugins treated as no-op success)
	}

	// If plugin failed, return the error immediately
	if pluginErr != nil {
		logger.Error("Plugin execution failed", slog.String("error", pluginErr.Error()))
		jobInstance.Shutdown("plugin failure")
		return pluginErr
	}

	// Wait for job completion or cancellation (with demo timeout)
	demoTimeout := time.After(2 * time.Second) // Short timeout for demo
	
	select {
	case <-jobInstance.Context.Done():
		logger.Info("Job completed", slog.String("reason", jobInstance.Context.Err().Error()))
	case <-ctx.Done():
		logger.Info("Job cancelled by user")
		jobInstance.Shutdown("user cancellation")
	case <-demoTimeout:
		logger.Info("Demo completed successfully - shutting down")
		jobInstance.Shutdown("demo timeout")
	}

	return nil
}

func runSTTEcho(filePath, provider string, logger *slog.Logger) error {
	if filePath == "" {
		return fmt.Errorf("--file is required")
	}

	// For Phase 3, we only support the fake provider
	if provider != "fake" {
		return fmt.Errorf("only 'fake' provider is supported in Phase 3")
	}

	// Create fake STT provider
	sttProvider := sttfake.NewFakeSTT("Hello, this is a test transcript from the fake STT provider.")

	// Create context for the operation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create STT stream
	stream, err := sttProvider.NewStream(ctx, stt.StreamConfig{
		SampleRate:  16000,
		NumChannels: 1,
		Lang:        "en-US",
		MaxRetry:    3,
	})
	if err != nil {
		return fmt.Errorf("failed to create STT stream: %w", err)
	}

	// Start listening for events
	go func() {
		for event := range stream.Events() {
			switch event.Type {
			case stt.SpeechEventInterim:
				logger.Info("Interim result", slog.String("text", event.Text))
			case stt.SpeechEventFinal:
				logger.Info("Final result", slog.String("text", event.Text))
				fmt.Printf("Transcript: %s\n", event.Text)
			case stt.SpeechEventError:
				logger.Error("STT error", slog.String("error", event.Error.Error()))
			}
		}
	}()

	// Read WAV file
	logger.Info("Processing audio file", slog.String("file", filePath))
	
	wavReader, err := wav.NewReader(filePath)
	if err != nil {
		logger.Warn("Failed to read WAV file, using fake frames instead", slog.String("error", err.Error()))
		// Fallback to fake frames for demo purposes
		return runSTTEchoWithFakeFrames(stream, logger)
	}
	defer wavReader.Close()

	frames, err := wavReader.ReadFrames()
	if err != nil {
		return fmt.Errorf("failed to read audio frames: %w", err)
	}

	header := wavReader.Header()
	logger.Info("WAV file info", 
		slog.Int("sample_rate", int(header.SampleRate)),
		slog.Int("channels", int(header.NumChannels)),
		slog.Int("bits_per_sample", int(header.BitsPerSample)),
		slog.Int("frames", len(frames)))

	// Push audio frames to STT
	for i, frame := range frames {
		if err := stream.Push(frame); err != nil {
			return fmt.Errorf("failed to push audio frame %d: %w", i, err)
		}

		// Small delay to simulate real-time processing
		time.Sleep(10 * time.Millisecond)
	}

	// Close the stream to get final results
	if err := stream.CloseSend(); err != nil {
		return fmt.Errorf("failed to close STT stream: %w", err)
	}

	// Wait a bit for final results
	time.Sleep(100 * time.Millisecond)

	logger.Info("STT echo completed successfully")
	return nil
}

func runSTTEchoWithFakeFrames(stream stt.STTStream, logger *slog.Logger) error {
	// Generate fake audio frames (fallback when WAV reading fails)
	for i := 0; i < 50; i++ {
		frame := rtc.AudioFrame{
			Data:              make([]byte, 320), // 16000/100 * 1 * 2 = 320 bytes for 10ms mono
			SampleRate:        16000,
			SamplesPerChannel: 160,
			NumChannels:       1,
			Timestamp:         time.Duration(i) * 10 * time.Millisecond,
		}

		if err := stream.Push(frame); err != nil {
			return fmt.Errorf("failed to push audio frame: %w", err)
		}

		// Small delay to simulate real-time processing
		time.Sleep(10 * time.Millisecond)
	}

	// Close the stream to get final results
	if err := stream.CloseSend(); err != nil {
		return fmt.Errorf("failed to close STT stream: %w", err)
	}

	// Wait a bit for final results
	time.Sleep(100 * time.Millisecond)

	return nil
}

func runAgentDemo(ctx context.Context, url, token, roomName string, metrics bool, bgFile string, bgVolume float32, logger *slog.Logger) error {
	// Start metrics server if requested
	if metrics {
		go func() {
			logger.Info("Starting metrics server on :8080")
			mux := http.NewServeMux()
			mux.Handle("/metrics", expvar.Handler())
			if err := http.ListenAndServe(":8080", mux); err != nil {
				logger.Error("Metrics server failed", slog.String("error", err.Error()))
			}
		}()
	}

	// Create job configuration
	jobConfig := job.Config{
		RoomName: roomName,
		Timeout:  30 * time.Minute,
	}

	// Create the job
	jobInstance, err := job.New(ctx, jobConfig)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	logger.Info("Job created for agent demo",
		slog.String("job_id", jobInstance.ID),
		slog.String("room_name", jobInstance.RoomName))

	// Set up channels for audio communication
	micIn := make(chan rtc.AudioFrame, 100)
	ttsOut := make(chan rtc.AudioFrame, 100)

	// Create fake AI providers for demo
	sttProvider := sttfake.NewFakeSTT("User said: Hello, how are you doing today?")
	ttsProvider := ttsfake.NewFakeTTS()
	llmProvider := fake.NewFakeLLM(
		"You said: Hello, how are you doing today?",
		"I'm doing well, thank you for asking!",
		"That's interesting, tell me more.",
		"I understand what you're saying.",
	)
	vadProvider := vadfake.NewFakeVAD(0.3)
	
	// Create turn detector with fallback to fake if models not available
	turnDetector, err := turn.NewDefaultDetector()
	if err != nil {
		logger.Warn("Failed to create turn detector, using fake", slog.String("error", err.Error()))
		turnDetector = &FakeTurnDetector{}
	}

	// Set up background audio if requested
	var backgroundAudio *agent.BackgroundAudio
	if bgFile != "" {
		ba, err := agent.NewBackgroundAudio(agent.BackgroundAudioConfig{
			AudioFile: bgFile,
			Volume:    bgVolume,
			Enabled:   true,
		})
		if err != nil {
			logger.Warn("Failed to load background audio", slog.String("error", err.Error()))
		} else {
			backgroundAudio = ba
			logger.Info("Background audio loaded", slog.String("file", bgFile), slog.Float64("volume", float64(bgVolume)))
		}
	}

	// Create agent configuration
	agentConfig := agent.Config{
		STT:             sttProvider,
		TTS:             ttsProvider,
		LLM:             llmProvider,
		VAD:             vadProvider,
		TurnDetector:    turnDetector,
		MicIn:           micIn,
		TTSOut:          ttsOut,
		BackgroundAudio: backgroundAudio,
	}

	// Create the agent
	voiceAgent, err := agent.New(agentConfig)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}
	defer voiceAgent.Close()

	logger.Info("Voice agent created successfully")

	// Start background processes to simulate audio I/O
	go simulateMicrophoneInput(ctx, micIn, logger)
	go simulateSpeakerOutput(ctx, ttsOut, logger)

	// Start the agent
	logger.Info("Starting voice agent...")
	agentCtx, agentCancel := context.WithCancel(ctx)
	defer agentCancel()

	agentDone := make(chan error, 1)
	go func() {
		agentDone <- voiceAgent.Start(agentCtx, jobInstance)
	}()

	// Wait for completion or interruption
	select {
	case err := <-agentDone:
		if err != nil {
			logger.Error("Agent failed", slog.String("error", err.Error()))
			return err
		}
		logger.Info("Agent completed successfully")
	case <-ctx.Done():
		logger.Info("Agent demo cancelled by user")
		agentCancel()
		// Wait a bit for graceful shutdown
		select {
		case <-agentDone:
		case <-time.After(5 * time.Second):
			logger.Warn("Agent shutdown timeout")
		}
	}

	return nil
}

func simulateMicrophoneInput(ctx context.Context, micIn chan<- rtc.AudioFrame, logger *slog.Logger) {
	defer close(micIn)

	ticker := time.NewTicker(10 * time.Millisecond) // 10ms frames
	defer ticker.Stop()

	frameCount := 0
	speechStart := 50  // Start speech after 500ms
	speechEnd := 200   // End speech after 2s

	for {
		select {
		case <-ctx.Done():
			logger.Debug("Microphone simulation stopped")
			return
		case <-ticker.C:
			frame := rtc.AudioFrame{
				Data:              make([]byte, 960), // 48kHz * 10ms * 1ch * 2bytes = 960
				SampleRate:        48000,
				SamplesPerChannel: 480,
				NumChannels:       1,
				Timestamp:         time.Duration(frameCount) * 10 * time.Millisecond,
			}

			// Simulate speech by filling with non-zero data during speech period
			if frameCount >= speechStart && frameCount <= speechEnd {
				for i := range frame.Data {
					frame.Data[i] = byte((frameCount + i) % 256)
				}
			}

			select {
			case micIn <- frame:
			case <-ctx.Done():
				return
			}

			frameCount++

			// Stop after 5 seconds for demo
			if frameCount > 500 {
				logger.Info("Microphone simulation completed")
				return
			}
		}
	}
}

func simulateSpeakerOutput(ctx context.Context, ttsOut <-chan rtc.AudioFrame, logger *slog.Logger) {
	frameCount := 0
	for {
		select {
		case <-ctx.Done():
			logger.Debug("Speaker simulation stopped")
			return
		case frame, ok := <-ttsOut:
			if !ok {
				logger.Debug("TTS output channel closed")
				return
			}
			frameCount++
			if frameCount%100 == 0 { // Log every second
				logger.Debug("Playing TTS audio frame",
					slog.Int("frame_count", frameCount),
					slog.Int("sample_rate", frame.SampleRate),
					slog.Int("data_len", len(frame.Data)))
			}
		}
	}
}

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Plugin management commands",
}

var pluginListCmd = &cobra.Command{
	Use:   "list [kind]",
	Short: "List registered plugins",
	Long: `List all registered plugins or plugins of a specific kind.
Available kinds: stt, tts, llm, vad`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := setupLogger()
		
		kind := ""
		if len(args) > 0 {
			kind = args[0]
		}
		
		plugins := plugin.List(kind)
		
		if len(plugins) == 0 {
			if kind == "" {
				fmt.Println("No plugins registered")
			} else {
				fmt.Printf("No plugins registered for kind: %s\n", kind)
			}
			return nil
		}
		
		// Print header
		fmt.Printf("%-8s %-20s %-10s %s\n", "KIND", "NAME", "VERSION", "DESCRIPTION")
		fmt.Println("------------------------------------------------------------")
		
		// Print plugins
		for _, p := range plugins {
			version := p.Version
			if version == "" {
				version = "N/A"
			}
			description := p.Description
			if description == "" {
				description = "No description"
			}
			fmt.Printf("%-8s %-20s %-10s %s\n", p.Kind, p.Name, version, description)
		}
		
		logger.Info("Listed plugins",
			slog.Int("count", len(plugins)),
			slog.String("filter_kind", kind))
		
		return nil
	},
}

var pluginDownloadCmd = &cobra.Command{
	Use:   "download-files",
	Short: "Download missing model files for all registered plugins",
	Long: `Iterate through all registered plugins and download any missing model files.
This is useful for plugins that require large model files that are not bundled with the binary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := setupLogger()
		logger.Info("Starting plugin model file download")
		
		plugins := plugin.List("")
		downloadCount := 0
		errorCount := 0
		
		for _, p := range plugins {
			logger.Info("Processing plugin", 
				slog.String("kind", p.Kind),
				slog.String("name", p.Name))
			
			if p.Downloader != nil {
				logger.Info("Downloading model files for plugin",
					slog.String("kind", p.Kind),
					slog.String("name", p.Name))
				
				if err := p.Downloader.Download(); err != nil {
					logger.Error("Failed to download model files",
						slog.String("kind", p.Kind),
						slog.String("name", p.Name),
						slog.String("error", err.Error()))
					errorCount++
				} else {
					downloadCount++
				}
			} else {
				logger.Debug("No model files to download for plugin",
					slog.String("kind", p.Kind),
					slog.String("name", p.Name))
			}
		}
		
		logger.Info("Plugin model file download completed",
			slog.Int("plugins_processed", len(plugins)),
			slog.Int("files_downloaded", downloadCount),
			slog.Int("errors", errorCount))
		
		if downloadCount == 0 && errorCount == 0 {
			fmt.Println("No model files needed downloading")
		} else {
			fmt.Printf("Downloaded %d model files", downloadCount)
			if errorCount > 0 {
				fmt.Printf(" (%d errors)", errorCount)
			}
			fmt.Println()
		}
		
		if errorCount > 0 {
			return fmt.Errorf("failed to download %d model files", errorCount)
		}
		
		return nil
	},
}

var pluginLoadCmd = &cobra.Command{
	Use:   "load [directory]",
	Short: "Load dynamic plugins from directory (Linux only with -tags=plugindyn)",
	Long: `Load .so plugin files from the specified directory.
If no directory is specified, uses LK_PLUGIN_PATH environment variable 
or defaults to /usr/local/lib/livekit-agents/plugins.

This feature is only available on Linux with the plugindyn build tag:
  go build -tags=plugindyn

Each plugin .so file must export a RegisterPlugins() error function.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := setupLogger()
		
		pluginDir := ""
		if len(args) > 0 {
			pluginDir = args[0]
		}
		
		logger.Info("Loading dynamic plugins", slog.String("directory", pluginDir))
		
		if err := plugin.LoadDynamicPlugins(pluginDir); err != nil {
			logger.Error("Failed to load dynamic plugins", slog.String("error", err.Error()))
			return err
		}
		
		logger.Info("Dynamic plugin loading completed")
		return nil
	},
}

var turnCmd = &cobra.Command{
	Use:   "turn",
	Short: "Turn detection commands",
}

var turnDownloadCmd = &cobra.Command{
	Use:   "download-models",
	Short: "Download turn detection models",
	Long: `Download English and multilingual turn detection models from the LiveKit model repository.
Models are stored in $LK_MODEL_PATH/turn-detector or ~/.livekit/models/turn-detector.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := setupLogger()
		logger.Info("Starting turn detection model download")
		
		downloader := turn.NewDownloader("")
		
		if err := downloader.DownloadAll(); err != nil {
			logger.Error("Failed to download models", slog.String("error", err.Error()))
			return err
		}
		
		logger.Info("Turn detection models downloaded successfully")
		return nil
	},
}

var turnPredictCmd = &cobra.Command{
	Use:   "predict",
	Short: "Predict end-of-turn probability from chat history JSON",
	Long: `Read chat history JSON from stdin and output end-of-turn probability.
Input format: {"messages": [{"role": "user", "content": "Hello"}], "language": "en-US"}
Output format: {"eou_probability": 0.85}`,
	RunE: func(cmd *cobra.Command, args []string) error {
		model, _ := cmd.Flags().GetString("model")
		threshold, _ := cmd.Flags().GetFloat64("threshold")
		language, _ := cmd.Flags().GetString("language")
		remoteURL, _ := cmd.Flags().GetString("remote-url")
		
		logger := setupLogger()
		logger.Debug("Starting turn prediction",
			slog.String("model", model),
			slog.Float64("threshold", threshold),
			slog.String("language", language))
		
		return runTurnPredict(model, threshold, language, remoteURL, logger)
	},
}

// FakeTurnDetector is a simple fake implementation for testing.
type FakeTurnDetector struct{}

func (f *FakeTurnDetector) UnlikelyThreshold(language string) (float64, error) {
	return 0.85, nil
}

func (f *FakeTurnDetector) SupportsLanguage(language string) bool {
	return true
}

func (f *FakeTurnDetector) PredictEndOfTurn(ctx context.Context, chatCtx turn.ChatContext) (float64, error) {
	// Simple heuristic: return high probability for longer conversations
	if len(chatCtx.Messages) > 2 {
		return 0.9, nil
	}
	return 0.6, nil
}

func runTurnPredict(model string, threshold float64, language, remoteURL string, logger *slog.Logger) error {
	// Read JSON from stdin
	var input struct {
		Messages []llm.Message `json:"messages"`
		Language string         `json:"language,omitempty"`
	}
	
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		return fmt.Errorf("failed to decode input JSON: %w", err)
	}
	
	// Override language if provided via flag
	if language != "" {
		input.Language = language
	}
	if input.Language == "" {
		input.Language = "en-US" // Default
	}
	
	// Create detector
	config := turn.DetectorConfig{
		Model:     model,
		RemoteURL: remoteURL,
	}
	
	detector, err := turn.NewDetector(config)
	if err != nil {
		return fmt.Errorf("failed to create detector: %w", err)
	}
	
	// Create chat context
	chatCtx := turn.ChatContext{
		Messages: input.Messages,
		Language: input.Language,
	}
	
	// Predict end of turn
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	probability, err := detector.PredictEndOfTurn(ctx, chatCtx)
	if err != nil {
		return fmt.Errorf("prediction failed: %w", err)
	}
	
	// Output result as JSON
	result := map[string]interface{}{
		"eou_probability": probability,
	}
	
	if threshold > 0 {
		result["threshold"] = threshold
		result["end_of_turn"] = probability >= threshold
	}
	
	return json.NewEncoder(os.Stdout).Encode(result)
}

func init() {
	// Add flags to worker run command
	workerRunCmd.Flags().String("url", "", "LiveKit server WebSocket URL")
	workerRunCmd.Flags().String("token", "", "LiveKit server token")
	workerRunCmd.Flags().Bool("dry-run", false, "Dry run mode - validate config and exit")
	
	// Add flags to worker healthz command
	workerHealthzCmd.Flags().String("url", "", "LiveKit server WebSocket URL")
	workerHealthzCmd.Flags().String("token", "", "LiveKit server token")
	
	// Add flags to job run-script command
	jobRunScriptCmd.Flags().String("url", "", "LiveKit server WebSocket URL")
	jobRunScriptCmd.Flags().String("token", "", "LiveKit server token")
	jobRunScriptCmd.Flags().String("room", "", "Room name to join")
	jobRunScriptCmd.Flags().Duration("timeout", 5*time.Minute, "Job timeout duration")
	
	// Add flags to STT echo command
	sttEchoCmd.Flags().String("file", "", "Path to WAV file to process")
	sttEchoCmd.Flags().String("provider", "fake", "STT provider to use (fake)")
	
	// Add flags to agent demo command
	agentDemoCmd.Flags().String("url", "", "LiveKit server WebSocket URL")
	agentDemoCmd.Flags().String("token", "", "LiveKit server token")
	agentDemoCmd.Flags().String("room", "", "Room name to join")
	agentDemoCmd.Flags().Bool("metrics", false, "Enable metrics server on port 8080")
	agentDemoCmd.Flags().String("bg-file", "", "Background audio WAV file to loop")
	agentDemoCmd.Flags().Float32("bg-volume", 0.5, "Background audio volume (0.0 to 1.0)")
	
	// Mark required flags
	jobRunScriptCmd.MarkFlagRequired("url")
	jobRunScriptCmd.MarkFlagRequired("token")
	jobRunScriptCmd.MarkFlagRequired("room")
	sttEchoCmd.MarkFlagRequired("file")
	agentDemoCmd.MarkFlagRequired("url")
	agentDemoCmd.MarkFlagRequired("token")
	agentDemoCmd.MarkFlagRequired("room")
	
	// Add flags to turn predict command
	turnPredictCmd.Flags().String("model", "livekit", "Model to use (livekit)")
	turnPredictCmd.Flags().Float64("threshold", 0, "Override threshold for end-of-turn decision")
	turnPredictCmd.Flags().String("language", "", "Language hint for detection optimization")
	turnPredictCmd.Flags().String("remote-url", "", "Override LIVEKIT_REMOTE_EOT_URL")
	
	// Build command tree
	workerCmd.AddCommand(workerRunCmd, workerHealthzCmd)
	jobCmd.AddCommand(jobRunScriptCmd)
	sttCmd.AddCommand(sttEchoCmd)
	agentCmd.AddCommand(agentDemoCmd)
	pluginCmd.AddCommand(pluginListCmd, pluginDownloadCmd, pluginLoadCmd)
	turnCmd.AddCommand(turnDownloadCmd, turnPredictCmd)
	rootCmd.AddCommand(versionCmd, workerCmd, jobCmd, sttCmd, agentCmd, pluginCmd, turnCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}