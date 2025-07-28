package job

import (
	"context"
	"fmt"
	"log/slog"
)

// New creates a new Job with the given configuration.
// It initializes the JobContext and sets up proper lifecycle management.
func New(parentCtx context.Context, cfg Config) (*Job, error) {
	if cfg.RoomName == "" {
		return nil, fmt.Errorf("room name is required")
	}

	// Generate ID if not provided
	jobID := cfg.ID
	if jobID == "" {
		jobID = generateJobID()
	}

	// Set up timeout context if specified
	var ctx context.Context
	if cfg.Timeout > 0 {
		ctx, _ = context.WithTimeout(parentCtx, cfg.Timeout)
	} else {
		ctx = parentCtx
	}

	// Create the job context
	jobContext := NewJobContext(ctx)

	job := &Job{
		ID:       jobID,
		RoomName: cfg.RoomName,
		Context:  jobContext,
	}

	slog.Info("Created new job",
		slog.String("job_id", jobID),
		slog.String("room_name", cfg.RoomName),
		slog.Duration("timeout", cfg.Timeout))

	return job, nil
}

// Shutdown gracefully shuts down the job with the given reason.
func (j *Job) Shutdown(reason string) {
	slog.Info("Shutting down job",
		slog.String("job_id", j.ID),
		slog.String("reason", reason))
	
	j.Context.Shutdown(reason)
}

// Wait blocks until the job context is cancelled.
// Returns the context error (context.Canceled or context.DeadlineExceeded).
func (j *Job) Wait() error {
	<-j.Context.Done()
	return j.Context.Err()
}

// IsActive returns true if the job is still running (not shut down).
func (j *Job) IsActive() bool {
	return !j.Context.IsShutdown()
}

// String returns a string representation of the job for logging.
func (j *Job) String() string {
	status := "active"
	if j.Context.IsShutdown() {
		status = "shutdown"
	}
	return fmt.Sprintf("Job{ID: %s, Room: %s, Status: %s}", j.ID, j.RoomName, status)
}