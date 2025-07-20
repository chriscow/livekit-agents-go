package agents

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	"os/exec"
	"strings"
	
	"github.com/spf13/cobra"
	"github.com/fsnotify/fsnotify"
)

// RunApp is the main entry point - equivalent to Python's cli.run_app()
func RunApp(opts *WorkerOptions) error {
	rootCmd := &cobra.Command{
		Use:   "agent",
		Short: "LiveKit Agent Framework",
		Long:  "Run LiveKit agents with different execution modes",
	}
	
	// Add subcommands
	rootCmd.AddCommand(newDevCommand(opts))
	rootCmd.AddCommand(newConsoleCommand(opts))
	rootCmd.AddCommand(newStartCommand(opts))
	rootCmd.AddCommand(newTestCommand(opts))
	rootCmd.AddCommand(newConnectCommand(opts))
	rootCmd.AddCommand(newDownloadCommand(opts))
	
	return rootCmd.Execute()
}

// Dev mode - with hot reload and debug logging
func newDevCommand(opts *WorkerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Development mode with hot reload",
		Long:  "Run agent in development mode with file watching and debug logging",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDev(opts)
		},
	}
	
	cmd.Flags().StringVar(&opts.Host, "host", opts.Host, "Host to bind to")
	cmd.Flags().IntVar(&opts.Port, "port", opts.Port, "Port to bind to")
	cmd.Flags().StringVar(&opts.APIKey, "api-key", opts.APIKey, "LiveKit API key")
	cmd.Flags().StringVar(&opts.APISecret, "api-secret", opts.APISecret, "LiveKit API secret")
	
	return cmd
}

// Console mode - local testing with mock room (Python equivalent)
func newConsoleCommand(opts *WorkerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "console",
		Short: "Start a new conversation inside the console",
		Long:  "Run agent locally with mock room and local audio I/O (equivalent to Python console mode)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConsole(opts)
		},
	}
	
	cmd.Flags().StringVar(&opts.LiveKitURL, "url", opts.LiveKitURL, "LiveKit server or Cloud project's websocket URL")
	cmd.Flags().StringVar(&opts.APIKey, "api-key", opts.APIKey, "LiveKit server or Cloud project's API key") 
	cmd.Flags().StringVar(&opts.APISecret, "api-secret", opts.APISecret, "LiveKit server or Cloud project's API secret")
	cmd.Flags().BoolVar(&opts.Record, "record", false, "Whether to record the conversation")
	
	return cmd
}

// Start mode - production deployment
func newStartCommand(opts *WorkerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start", 
		Short: "Production mode",
		Long:  "Run agent in production mode with optimizations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(opts)
		},
	}
	
	cmd.Flags().StringVar(&opts.Host, "host", opts.Host, "Host to bind to")
	cmd.Flags().IntVar(&opts.Port, "port", opts.Port, "Port to bind to")
	cmd.Flags().StringVar(&opts.APIKey, "api-key", opts.APIKey, "LiveKit API key")
	cmd.Flags().StringVar(&opts.APISecret, "api-secret", opts.APISecret, "LiveKit API secret")
	
	return cmd
}

// Development mode implementation
func runDev(opts *WorkerOptions) error {
	log.Printf("Starting agent in development mode...")
	
	// Set up file watcher for hot reload
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()
	
	// Watch current directory and subdirectories
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to set up file watching: %w", err)
	}
	
	// Start worker in a separate goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	workerDone := make(chan error, 1)
	go func() {
		worker := NewWorker(opts)
		workerDone <- worker.Start(ctx)
	}()
	
	// Handle file changes and signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Printf("File modified: %s - restarting agent...", event.Name)
				cancel()
				<-workerDone
				
				// Restart worker
				ctx, cancel = context.WithCancel(context.Background())
				go func() {
					worker := NewWorker(opts)
					workerDone <- worker.Start(ctx)
				}()
			}
		case err := <-watcher.Errors:
			log.Printf("File watcher error: %v", err)
		case <-sigChan:
			log.Println("Shutting down...")
			cancel()
			return <-workerDone
		case err := <-workerDone:
			if err != nil {
				log.Printf("Worker error: %v", err)
				time.Sleep(2 * time.Second) // Brief delay before restart
				
				// Restart worker
				ctx, cancel = context.WithCancel(context.Background())
				go func() {
					worker := NewWorker(opts)
					workerDone <- worker.Start(ctx)
				}()
			}
		}
	}
}

// Console mode implementation (Python console mode equivalent)
func runConsole(opts *WorkerOptions) error {
	log.Println("Starting agent in console mode...")
	
	// Note: Console mode requires liblivekit_ffi.dylib in project root
	// CGO automatically finds it there, no environment variables needed
	
	// Set console mode flag
	opts.ConsoleMode = true
	
	// Keep everything inside the same process when using console mode (like Python)
	opts.ExecutorType = JobExecutorThread
	
	// Set fake console credentials (like Python cli.py:151-153)
	if opts.LiveKitURL == "" {
		opts.LiveKitURL = "ws://localhost:7881/fake_console_url"
	}
	if opts.APIKey == "" {
		opts.APIKey = "fake_console_key"
	}
	if opts.APISecret == "" {
		opts.APISecret = "fake_console_secret"
	}
	
	// Create simulated job info for console mode (like Python simulate_job)
	opts.RoomName = "mock-console"
	
	log.Printf("Console mode configured:")
	log.Printf("  URL: %s", opts.LiveKitURL)
	log.Printf("  Room: %s", opts.RoomName)
	log.Printf("  Record: %v", opts.Record)
	
	// Create worker with console configuration (instead of separate ConsoleAgent)
	worker := NewWorker(opts)
	
	// Handle shutdown gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- worker.Start(ctx)
	}()
	
	select {
	case <-sigChan:
		log.Println("Shutting down...")
		cancel()
		return <-workerDone
	case err := <-workerDone:
		return err
	}
}

// Production mode implementation
func runStart(opts *WorkerOptions) error {
	log.Println("Starting agent in production mode...")
	
	worker := NewWorker(opts)
	
	// Handle shutdown gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- worker.Start(ctx)
	}()
	
	select {
	case <-sigChan:
		log.Println("Shutting down...")
		cancel()
		return <-workerDone
	case err := <-workerDone:
		return err
	}
}

// Test mode - run mock service tests
func newTestCommand(opts *WorkerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run tests on mock services",
		Long:  "Run comprehensive tests on mock implementations and services",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTest(cmd, opts)
		},
	}
	
	cmd.Flags().Bool("mock-only", false, "Run only mock service tests")
	cmd.Flags().Bool("plugins", false, "Test plugin registration")
	cmd.Flags().Bool("benchmarks", false, "Run performance benchmarks")
	cmd.Flags().String("scenario", "", "Run specific test scenario")
	
	return cmd
}

// Connect mode - direct room connection for testing
func newConnectCommand(opts *WorkerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect directly to a specific room",
		Long:  "Connect agent directly to a room for testing purposes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnect(cmd, opts)
		},
	}
	
	cmd.Flags().StringVar(&opts.Host, "host", opts.Host, "Host to connect to")
	cmd.Flags().IntVar(&opts.Port, "port", opts.Port, "Port to connect to")
	cmd.Flags().StringVar(&opts.APIKey, "api-key", opts.APIKey, "LiveKit API key")
	cmd.Flags().StringVar(&opts.APISecret, "api-secret", opts.APISecret, "LiveKit API secret")
	cmd.Flags().String("room", "", "Room name to connect to (required)")
	cmd.Flags().String("participant-identity", "", "Participant identity")
	cmd.MarkFlagRequired("room")
	
	return cmd
}

// Download mode - download plugin dependencies  
func newDownloadCommand(opts *WorkerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download-files",
		Short: "Download plugin dependencies",
		Long:  "Download required model files and dependencies for plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDownload(cmd, opts)
		},
	}
	
	cmd.Flags().String("plugin", "", "Download files for specific plugin only")
	cmd.Flags().Bool("force", false, "Force re-download of existing files")
	
	return cmd
}

// Test mode implementation
func runTest(cmd *cobra.Command, opts *WorkerOptions) error {
	log.Println("Running tests...")
	
	// Get command flags
	mockOnly, _ := cmd.Flags().GetBool("mock-only")
	plugins, _ := cmd.Flags().GetBool("plugins")
	benchmarks, _ := cmd.Flags().GetBool("benchmarks")
	scenario, _ := cmd.Flags().GetString("scenario")
	
	var testArgs []string
	testArgs = append(testArgs, "test")
	
	if mockOnly {
		testArgs = append(testArgs, "./test/mock")
	}
	
	if plugins {
		testArgs = append(testArgs, "./plugins/...")
	}
	
	if scenario != "" {
		testArgs = append(testArgs, "-run", scenario)
	}
	
	if benchmarks {
		testArgs = append(testArgs, "-bench=.")
		testArgs = append(testArgs, "-benchmem")
	}
	
	testArgs = append(testArgs, "-v")
	
	if len(testArgs) == 2 { // Only "test" and "-v"
		testArgs = append(testArgs, "./test/mock", "./...")
	}
	
	log.Printf("Running: go %s", strings.Join(testArgs, " "))
	
	testCmd := exec.Command("go", testArgs...)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr
	testCmd.Dir = "."
	
	return testCmd.Run()
}

// Connect mode implementation
func runConnect(cmd *cobra.Command, opts *WorkerOptions) error {
	// Get room name from flags
	roomName, _ := cmd.Flags().GetString("room")
	participantIdentity, _ := cmd.Flags().GetString("participant-identity")
	
	if roomName == "" {
		return fmt.Errorf("room name is required")
	}
	
	log.Printf("Connecting to room: %s", roomName)
	if participantIdentity != "" {
		log.Printf("Using participant identity: %s", participantIdentity)
	}
	
	// Create worker with connect-specific configuration
	connectOpts := *opts
	connectOpts.RoomName = roomName
	connectOpts.ParticipantIdentity = participantIdentity
	
	worker := NewWorker(&connectOpts)
	
	// Handle shutdown gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- worker.Start(ctx)
	}()
	
	select {
	case <-sigChan:
		log.Println("Shutting down...")
		cancel()
		return <-workerDone
	case err := <-workerDone:
		return err
	}
}

// Download mode implementation
func runDownload(cmd *cobra.Command, opts *WorkerOptions) error {
	// Get command flags
	plugin, _ := cmd.Flags().GetString("plugin")
	force, _ := cmd.Flags().GetBool("force")
	
	log.Println("Downloading plugin dependencies...")
	
	if plugin != "" {
		log.Printf("Downloading files for plugin: %s", plugin)
	} else {
		log.Println("Downloading files for all plugins")
	}
	
	if force {
		log.Println("Force re-download enabled")
	}
	
	// In Go, most dependencies are handled by go mod,
	// but some plugins might need model files, tokenizers, etc.
	
	pluginPaths := []string{
		"./plugins/silero",
		"./plugins/anthropic", 
		"./plugins/openai",
		"./plugins/deepgram",
	}
	
	if plugin != "" {
		pluginPaths = []string{fmt.Sprintf("./plugins/%s", plugin)}
	}
	
	for _, pluginPath := range pluginPaths {
		if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
			log.Printf("Plugin path does not exist: %s", pluginPath)
			continue
		}
		
		// Check for download scripts in plugin directories
		downloadScript := filepath.Join(pluginPath, "download.sh")
		if _, err := os.Stat(downloadScript); err == nil {
			log.Printf("Running download script for: %s", pluginPath)
			
			downloadCmd := exec.Command("bash", downloadScript)
			if force {
				downloadCmd.Env = append(os.Environ(), "FORCE_DOWNLOAD=true")
			}
			downloadCmd.Dir = pluginPath
			downloadCmd.Stdout = os.Stdout
			downloadCmd.Stderr = os.Stderr
			
			if err := downloadCmd.Run(); err != nil {
				log.Printf("Warning: Download script failed for %s: %v", pluginPath, err)
			}
		} else {
			log.Printf("No download script found for: %s", pluginPath)
		}
	}
	
	log.Println("Download complete")
	return nil
}