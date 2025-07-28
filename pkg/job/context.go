package job

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// NewJobContext creates a new JobContext with the given parent context.
// The context will be cancelled when Shutdown is called.
func NewJobContext(parent context.Context) *JobContext {
	ctx, cancel := context.WithCancel(parent)
	
	return &JobContext{
		Ctx:           ctx,
		cancel:        cancel,
		shutdownHooks: make([]func(string), 0),
		shutdownOnce:  false,
		// shutdownMu is zero-initialized (unlocked)
	}
}

// Shutdown timeout constant for testability
const ShutdownHookTimeout = 5 * time.Second

// Shutdown initiates graceful shutdown of the job.
// This method is idempotent - calling it multiple times is safe.
// All registered shutdown hooks will be called exactly once.
func (jc *JobContext) Shutdown(reason string) {
	jc.shutdownMu.Lock()
	defer jc.shutdownMu.Unlock()
	
	if jc.shutdownOnce {
		return // Already shut down
	}
	
	jc.shutdownOnce = true
	
	slog.Info("Job shutdown initiated", slog.String("reason", reason))
	
	// Execute all shutdown hooks
	var wg sync.WaitGroup
	for _, hook := range jc.shutdownHooks {
		wg.Add(1)
		go func(h func(string)) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Shutdown hook panicked", slog.Any("panic", r))
				}
			}()
			h(reason)
		}(hook)
	}
	
	// Wait for all hooks to complete with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		slog.Debug("All shutdown hooks completed")
	case <-time.After(ShutdownHookTimeout):
		slog.Warn("Shutdown hooks timed out", slog.Duration("timeout", ShutdownHookTimeout))
	}
	
	// Cancel the context to signal shutdown to all listeners
	jc.cancel()
}

// OnShutdown registers a callback to be executed when Shutdown is called.
// Callbacks are executed concurrently and should handle their own errors.
// If the job has already been shut down, the callback is executed immediately.
func (jc *JobContext) OnShutdown(callback func(reason string)) {
	jc.shutdownMu.Lock()
	defer jc.shutdownMu.Unlock()
	
	if jc.shutdownOnce {
		// Job already shut down, execute callback immediately
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Shutdown callback panicked", slog.Any("panic", r))
				}
			}()
			callback("job already shut down")
		}()
		return
	}
	
	jc.shutdownHooks = append(jc.shutdownHooks, callback)
}

// IsShutdown returns true if the job has been shut down.
func (jc *JobContext) IsShutdown() bool {
	select {
	case <-jc.Ctx.Done():
		return true
	default:
		return false
	}
}

// Done returns a channel that is closed when the job context is cancelled.
// This is equivalent to jc.Ctx.Done() but provides a more explicit API.
func (jc *JobContext) Done() <-chan struct{} {
	return jc.Ctx.Done()
}

// Err returns the error associated with the context cancellation.
// This is equivalent to jc.Ctx.Err().
func (jc *JobContext) Err() error {
	return jc.Ctx.Err()
}

// generateJobID creates a random job ID.
func generateJobID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("job_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("job_%x", bytes)
}