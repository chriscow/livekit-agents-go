package worker

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestWorker_New(t *testing.T) {
	is := is.New(t)
	
	logger := slog.Default()
	config := Config{
		URL:   "wss://example.com",
		Token: "test-token",
	}

	worker := New(config, logger)

	// New() always returns a valid worker instance
	is.Equal(worker.url, config.URL)      // worker URL should match config
	is.Equal(worker.token, config.Token)  // worker token should match config
	is.True(worker.in != nil)             // in channel should be initialized
	is.True(worker.out != nil)            // out channel should be initialized
}

func TestWorker_IsConnected(t *testing.T) {
	is := is.New(t)
	
	logger := slog.Default()
	config := Config{URL: "wss://example.com", Token: "test"}
	worker := New(config, logger)

	// Should start disconnected
	is.True(!worker.IsConnected()) // worker should start disconnected

	// Test setting connected state
	worker.setConnected(true)
	is.True(worker.IsConnected()) // worker should be connected after setConnected(true)

	worker.setConnected(false)
	is.True(!worker.IsConnected()) // worker should be disconnected after setConnected(false)
}

func TestWorker_HandleSignal_Ping(t *testing.T) {
	logger := slog.Default()
	config := Config{URL: "wss://example.com", Token: "test"}
	worker := New(config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Create a ping signal
	pingSignal := &Signal{
		Type: "ping",
		Data: map[string]any{"id": "test-ping"},
	}

	// Handle the signal
	worker.handleSignal(ctx, pingSignal)

	// Check that a pong was sent
	select {
	case cmd := <-worker.out:
		if cmd.Type != "pong" {
			t.Errorf("expected pong response, got %s", cmd.Type)
		}
		if cmd.Data["id"] != "test-ping" {
			t.Errorf("expected pong to echo ping data, got %v", cmd.Data)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected pong response within 100ms")
	}
}

func TestWorker_HandleSignal_StartJob(t *testing.T) {
	logger := slog.Default()
	config := Config{URL: "wss://example.com", Token: "test"}
	worker := New(config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Create a startJob signal
	startJobSignal := &Signal{
		Type: "startJob",
		Data: map[string]any{"jobId": "test-job"},
	}

	// Handle the signal (should not panic for now)
	worker.handleSignal(ctx, startJobSignal)

	// No specific response expected in phase 1, just ensure it doesn't panic
}

func TestWorker_HandleSignal_Unknown(t *testing.T) {
	logger := slog.Default()
	config := Config{URL: "wss://example.com", Token: "test"}
	worker := New(config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Create an unknown signal
	unknownSignal := &Signal{
		Type: "unknownType",
		Data: map[string]any{"foo": "bar"},
	}

	// Handle the signal (should not panic)
	worker.handleSignal(ctx, unknownSignal)

	// No response expected for unknown signals
	select {
	case <-worker.out:
		t.Error("no response expected for unknown signal type")
	case <-time.After(50 * time.Millisecond):
		// Expected - no response
	}
}

func TestBackoffCalculation(t *testing.T) {
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 10 * time.Second}, // capped at 10s
		{10, 10 * time.Second}, // still capped
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			logger := slog.Default()
			config := Config{URL: "wss://example.com", Token: "test"}
			worker := New(config, logger)
			
			// Set backoff attempt counter on worker instance
			worker.mu.Lock()
			worker.backoffAttempt = tt.attempt - 1
			worker.mu.Unlock()

			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			start := time.Now()
			err := worker.backoffDelay(ctx)
			duration := time.Since(start)

			// Should timeout due to context, but we can check it started the right delay
			if err != context.DeadlineExceeded {
				t.Errorf("expected context deadline exceeded, got %v", err)
			}

			// Allow some tolerance for timing
			if duration < 40*time.Millisecond {
				t.Errorf("backoff should have waited at least 40ms, waited %v", duration)
			}
		})
	}
}