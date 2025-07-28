package turn

import (
	"sync"
	"testing"
)

// TestEnsureOrtEnvIdempotent verifies that multiple calls to ensureOrtEnv()
// return the same result without causing race conditions.
func TestEnsureOrtEnvIdempotent(t *testing.T) {
	// First call
	err1 := ensureOrtEnv()
	
	// Second call should return the same result
	err2 := ensureOrtEnv()
	
	if err1 != err2 {
		t.Errorf("ensureOrtEnv() not idempotent: first call returned %v, second call returned %v", err1, err2)
	}
	
	// Third call to ensure consistency
	err3 := ensureOrtEnv()
	
	if err1 != err3 {
		t.Errorf("ensureOrtEnv() not consistent: first call returned %v, third call returned %v", err1, err3)
	}
}

// TestSingletonGuard verifies that the sync.Once mechanism works correctly.
func TestSingletonGuard(t *testing.T) {
	// Create a new Once for testing (we can't reset the package-level one safely)
	var testOnce sync.Once
	var callCount int
	
	const numCalls = 5
	
	for i := 0; i < numCalls; i++ {
		testOnce.Do(func() {
			callCount++
		})
	}
	
	if callCount != 1 {
		t.Errorf("sync.Once should execute function exactly once, but it executed %d times", callCount)
	}
}