package main

import (
	"log"
	"os"

	"livekit-agents-go/agents"
)

// entrypoint is the main worker entrypoint function
//   Flow from start to entrypoint:

// 1. main() function (/Users/chris/dev/livekit-agents-go/examples/minimal_worker/main.go:34)
//   - Creates WorkerOptions with EntrypointFunc: entrypoint (:37)
//   - Calls agents.RunApp(opts) (:63)
//
// 2. RunApp() function (agents/cli.go:20)
//   - Sets up CLI commands (dev, console, start, etc.)
//   - Executes the appropriate command
//
// 3. Command execution (e.g., runStart, runDev, runConsole)
//   - Creates worker := NewWorker(opts) (:205, :121, :177)
//   - Calls worker.Start(ctx) (:216, :122, :188)
//
// 4. Worker.Start() method (agents/worker.go:340)
//   - Creates test job context: jobCtx := NewJobContext(ctx) (:380)
//   - Sets entrypoint function: jobCtx.EntrypointFunc = w.opts.EntrypointFunc (:381)
//   - Submits job: w.SubmitJob(jobCtx) (:385)
//
// 5. Worker.SubmitJob() method (agents/worker.go:547)
//   - Calls w.scheduler.ScheduleJob(jobCtx) (:562)
//
// 6. Job scheduler execution (agents/worker.go:894-970)
//   - ScheduleJob() queues the job (:896)
//   - Worker goroutine picks up job in executeJob() (:918)
//   - Finally calls the entrypoint: jobCtx.EntrypointFunc(jobCtx) (:942)
func entrypoint(ctx *agents.JobContext) error {
	// Connect to the RTC room (matching Python: await ctx.connect())
	err := ctx.Connect()
	if err != nil {
		return err
	}

	// Log connection success (matching Python: logger.info(f"connected to the room {ctx.room.name}"))
	if ctx.Room != nil {
		log.Printf("connected to the room %s", ctx.Room.Name())
	} else {
		log.Printf("connected to the room (room name not available yet)")
	}

	// In Python this just connects and waits, so we'll do the same
	// The worker framework handles the lifecycle
	<-ctx.Context.Done()
	log.Println("Context cancelled, disconnecting from room")
	return ctx.Context.Err()
}

func main() {
	// Configure minimal worker options (matching Python cli.run_app(WorkerOptions(...)))
	opts := &agents.WorkerOptions{
		EntrypointFunc: entrypoint,
		AgentName:      "MinimalWorker", // Match Python logger name "minimal-worker"
		APIKey:         os.Getenv("LIVEKIT_API_KEY"),
		APISecret:      os.Getenv("LIVEKIT_API_SECRET"),
		Host:           os.Getenv("LIVEKIT_HOST"),
		LiveKitURL:     os.Getenv("LIVEKIT_URL"),
		Metadata: map[string]string{
			"description": "A minimal worker that just connects to rooms",
			"version":     "1.0.0",
			"type":        "minimal-worker",
		},
	}

	// Set defaults for development
	if opts.Host == "" {
		opts.Host = "localhost:7880"
	}
	if opts.LiveKitURL == "" {
		opts.LiveKitURL = "ws://localhost:7880"
	}

	log.Printf("Starting Minimal Worker: %s", opts.AgentName)
	log.Printf("LiveKit Host: %s", opts.Host)
	log.Printf("LiveKit URL: %s", opts.LiveKitURL)

	// Run with CLI (equivalent to Python's cli.run_app())
	if err := agents.RunApp(opts); err != nil {
		log.Fatal("Failed to run worker:", err)
	}
}
