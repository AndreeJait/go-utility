package gracefulw

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// clearTasks is a helper function to reset the global state between tests.
func clearTasks() {
	mu.Lock()
	defer mu.Unlock()
	tasks = nil
}

func TestRegister(t *testing.T) {
	clearTasks()

	Register("Database", func(ctx context.Context) error { return nil })
	Register("Cache", func(ctx context.Context) error { return nil })

	mu.Lock()
	defer mu.Unlock()

	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks to be registered, got %d", len(tasks))
	}
	if tasks[0].Name != "Database" || tasks[1].Name != "Cache" {
		t.Errorf("Tasks were not registered in the correct order")
	}
}

func TestExecute_Success(t *testing.T) {
	clearTasks()

	var counter atomic.Int32

	// Register 3 tasks that simply increment a thread-safe counter
	for i := 0; i < 3; i++ {
		Register("MockTask", func(ctx context.Context) error {
			counter.Add(1)
			return nil
		})
	}

	// Register a task that returns an error (to ensure it doesn't panic or halt execution)
	Register("FailingTask", func(ctx context.Context) error {
		return errors.New("simulated cleanup failure")
	})

	// Execute with a generous timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	Execute(ctx)

	// Ensure all 3 successful tasks ran
	if counter.Load() != 3 {
		t.Errorf("Expected 3 tasks to successfully increment the counter, got %d", counter.Load())
	}

	// Ensure the task list is cleared after execution
	mu.Lock()
	defer mu.Unlock()
	if len(tasks) != 0 {
		t.Errorf("Expected tasks to be cleared after execution, got %d remaining", len(tasks))
	}
}

func TestExecute_Timeout(t *testing.T) {
	clearTasks()

	// Register a task that takes 2 seconds to complete
	Register("SlowTask", func(ctx context.Context) error {
		select {
		case <-time.After(2 * time.Second):
			return nil
		case <-ctx.Done():
			// The context should cancel before the 2 seconds are up
			return ctx.Err()
		}
	})

	// Provide a very short timeout of 50 milliseconds to Execute
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	Execute(ctx)
	duration := time.Since(start)

	// Execution should finish in roughly ~50ms due to the timeout,
	// definitely not the full 2 seconds.
	if duration >= 1*time.Second {
		t.Errorf("Execute did not respect the timeout context. Took %v", duration)
	}
}

func TestExecute_MultipleCalls(t *testing.T) {
	clearTasks()

	var counter atomic.Int32

	Register("SingleRunTask", func(ctx context.Context) error {
		counter.Add(1)
		return nil
	})

	ctx := context.Background()

	// Call Execute twice
	Execute(ctx)
	Execute(ctx)

	// The task should only run once because tasks are cleared during the first Execute
	if counter.Load() != 1 {
		t.Errorf("Expected task to run exactly once, but ran %d times", counter.Load())
	}
}
