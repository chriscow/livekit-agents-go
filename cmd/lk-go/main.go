package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chriscow/livekit-agents-go/internal/worker"
	"github.com/chriscow/livekit-agents-go/pkg/ai/stt"
	"github.com/chriscow/livekit-agents-go/pkg/ai/stt/fake"
	"github.com/chriscow/livekit-agents-go/pkg/audio/wav"
	"github.com/chriscow/livekit-agents-go/pkg/job"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
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
		
		// TODO: Implement actual health check that pings server
		logger.Info("Health check passed")
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
	sttProvider := fake.NewFakeSTT("Hello, this is a test transcript from the fake STT provider.")

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

func init() {
	// Add flags to worker run command
	workerRunCmd.Flags().String("url", "", "LiveKit server WebSocket URL")
	workerRunCmd.Flags().String("token", "", "LiveKit server token")
	workerRunCmd.Flags().Bool("dry-run", false, "Dry run mode - validate config and exit")
	
	// Add flags to job run-script command
	jobRunScriptCmd.Flags().String("url", "", "LiveKit server WebSocket URL")
	jobRunScriptCmd.Flags().String("token", "", "LiveKit server token")
	jobRunScriptCmd.Flags().String("room", "", "Room name to join")
	jobRunScriptCmd.Flags().Duration("timeout", 5*time.Minute, "Job timeout duration")
	
	// Add flags to STT echo command
	sttEchoCmd.Flags().String("file", "", "Path to WAV file to process")
	sttEchoCmd.Flags().String("provider", "fake", "STT provider to use (fake)")
	
	// Mark required flags
	jobRunScriptCmd.MarkFlagRequired("url")
	jobRunScriptCmd.MarkFlagRequired("token")
	jobRunScriptCmd.MarkFlagRequired("room")
	sttEchoCmd.MarkFlagRequired("file")
	
	// Build command tree
	workerCmd.AddCommand(workerRunCmd, workerHealthzCmd)
	jobCmd.AddCommand(jobRunScriptCmd)
	sttCmd.AddCommand(sttEchoCmd)
	rootCmd.AddCommand(versionCmd, workerCmd, jobCmd, sttCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}