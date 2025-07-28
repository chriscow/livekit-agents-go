package job

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestJob_New(t *testing.T) {
	ctx := context.Background()
	
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config with ID",
			config: Config{
				ID:       "test-job-1",
				RoomName: "test-room",
				Timeout:  time.Minute,
			},
			wantErr: false,
		},
		{
			name: "valid config without ID",
			config: Config{
				RoomName: "test-room",
				Timeout:  time.Minute,
			},
			wantErr: false,
		},
		{
			name: "missing room name",
			config: Config{
				ID:      "test-job-1",
				Timeout: time.Minute,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := New(ctx, tt.config)
			
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if job == nil {
				t.Error("expected job but got nil")
				return
			}
			
			// Check ID is set
			if job.ID == "" {
				t.Error("job ID should not be empty")
			}
			
			// If ID was provided, it should match
			if tt.config.ID != "" && job.ID != tt.config.ID {
				t.Errorf("expected job ID %s, got %s", tt.config.ID, job.ID)
			}
			
			if job.RoomName != tt.config.RoomName {
				t.Errorf("expected room name %s, got %s", tt.config.RoomName, job.RoomName)
			}
			
			if job.Context == nil {
				t.Error("job context should not be nil")
			}
			
			if !job.IsActive() {
				t.Error("new job should be active")
			}
		})
	}
}

func TestJob_Shutdown(t *testing.T) {
	ctx := context.Background()
	job, err := New(ctx, Config{
		RoomName: "test-room",
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	if !job.IsActive() {
		t.Error("job should be active before shutdown")
	}

	// Shutdown the job
	reason := "test shutdown"
	job.Shutdown(reason)

	// Give shutdown a moment to complete
	time.Sleep(10 * time.Millisecond)

	if job.IsActive() {
		t.Error("job should not be active after shutdown")
	}

	// Should be able to wait for completion
	err = job.Wait()
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestJobContext_ShutdownHooks(t *testing.T) {
	ctx := context.Background()
	jobCtx := NewJobContext(ctx)

	// Track hook execution
	var hooksCalled int
	var hookReasons []string
	var mu sync.Mutex

	// Register two shutdown hooks
	jobCtx.OnShutdown(func(reason string) {
		mu.Lock()
		defer mu.Unlock()
		hooksCalled++
		hookReasons = append(hookReasons, reason)
	})

	jobCtx.OnShutdown(func(reason string) {
		mu.Lock()
		defer mu.Unlock()
		hooksCalled++
		hookReasons = append(hookReasons, reason)
	})

	// Shutdown should trigger both hooks
	reason := "test shutdown"
	jobCtx.Shutdown(reason)

	// Wait for hooks to complete
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if hooksCalled != 2 {
		t.Errorf("expected 2 hooks called, got %d", hooksCalled)
	}

	if len(hookReasons) != 2 {
		t.Errorf("expected 2 hook reasons, got %d", len(hookReasons))
	}

	for i, r := range hookReasons {
		if r != reason {
			t.Errorf("hook %d: expected reason %s, got %s", i, reason, r)
		}
	}
}

func TestJobContext_ShutdownIdempotent(t *testing.T) {
	ctx := context.Background()
	jobCtx := NewJobContext(ctx)

	var hooksCalled int
	var mu sync.Mutex

	jobCtx.OnShutdown(func(reason string) {
		mu.Lock()
		hooksCalled++
		mu.Unlock()
	})

	// Call shutdown multiple times
	jobCtx.Shutdown("first shutdown")
	jobCtx.Shutdown("second shutdown")
	jobCtx.Shutdown("third shutdown")

	// Wait for any async operations
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Hook should only be called once
	if hooksCalled != 1 {
		t.Errorf("expected 1 hook call, got %d", hooksCalled)
	}
}

func TestJobContext_OnShutdownAfterShutdown(t *testing.T) {
	ctx := context.Background()
	jobCtx := NewJobContext(ctx)

	// Shutdown first
	jobCtx.Shutdown("test shutdown")

	// Wait for shutdown to complete
	time.Sleep(10 * time.Millisecond)

	// Register hook after shutdown
	var hookCalled bool
	var mu sync.Mutex
	jobCtx.OnShutdown(func(reason string) {
		mu.Lock()
		hookCalled = true
		mu.Unlock()
	})

	// Wait for hook to be called
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !hookCalled {
		t.Error("hook should be called immediately when registered after shutdown")
	}
}

func TestJobContext_ConcurrentShutdown(t *testing.T) {
	ctx := context.Background()
	jobCtx := NewJobContext(ctx)

	var hooksCalled int
	var mu sync.Mutex

	// Register hook
	jobCtx.OnShutdown(func(reason string) {
		mu.Lock()
		hooksCalled++
		mu.Unlock()
	})

	// Start multiple goroutines trying to shutdown concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			jobCtx.Shutdown("concurrent test")
		}(i)
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Hook should only be called once despite multiple concurrent shutdowns
	if hooksCalled != 1 {
		t.Errorf("expected 1 hook call, got %d", hooksCalled)
	}
}

func TestJob_Timeout(t *testing.T) {
	ctx := context.Background()
	
	// Create job with short timeout
	job, err := New(ctx, Config{
		RoomName: "test-room",
		Timeout:  50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// Job should timeout
	err = job.Wait()
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}

	if job.IsActive() {
		t.Error("job should not be active after timeout")
	}
}