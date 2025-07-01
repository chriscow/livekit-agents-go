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

// Console mode - local testing without external server
func newConsoleCommand(opts *WorkerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "console",
		Short: "Console mode for local testing",
		Long:  "Run agent locally with terminal audio I/O",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConsole(opts)
		},
	}
	
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

// Console mode implementation  
func runConsole(opts *WorkerOptions) error {
	log.Println("Starting agent in console mode...")
	
	// Set up console-specific configuration
	consoleOpts := *opts
	consoleOpts.ExecutorType = JobExecutorThread // Use thread executor for console
	
	// TODO: Set up local audio I/O instead of LiveKit connection
	// This would use local microphone/speakers for testing
	
	worker := NewWorker(&consoleOpts)
	
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