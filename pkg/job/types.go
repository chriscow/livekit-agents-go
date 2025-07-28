package job

import (
	"context"
	"time"
)

// Job represents a single agent job execution context.
// It contains the job metadata and manages the lifecycle of agent work.
type Job struct {
	// ID is the unique identifier for this job
	ID string

	// RoomName is the LiveKit room this job is assigned to
	RoomName string

	// Context provides lifecycle management and shutdown coordination
	Context *JobContext
}

// JobContext manages the lifecycle and cleanup of a job.
// It is immutable after creation and provides coordinated shutdown.
type JobContext struct {
	// Ctx is the context that gets cancelled when the job ends
	Ctx context.Context

	// Private fields for managing shutdown
	cancel        context.CancelFunc
	shutdownHooks []func(string)
	shutdownOnce  bool
	shutdownMu    chan struct{} // acts as mutex for shutdown
}

// ShutdownInfo contains information about why a job shutdown occurred.
type ShutdownInfo struct {
	// Reason describes why the shutdown was initiated
	Reason string

	// Timestamp when the shutdown was initiated
	Timestamp time.Time

	// Graceful indicates if this was a planned shutdown
	Graceful bool
}

// Config contains configuration options for creating a new Job.
type Config struct {
	// ID for the job (if empty, one will be generated)
	ID string

	// RoomName is the LiveKit room to join
	RoomName string

	// Timeout for the overall job execution
	Timeout time.Duration
}

// Constants for job management
const (
	// AssignmentTimeout mirrors Python's worker.ASSIGNMENT_TIMEOUT
	AssignmentTimeout = 7500 * time.Millisecond

	// DefaultJobTimeout is the default timeout for job execution
	DefaultJobTimeout = 5 * time.Minute
)