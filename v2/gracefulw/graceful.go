package gracefulw

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	// Ensure this matches your project's module path
	"github.com/AndreeJait/go-utility/v2/logw"
)

// CleanupFunc defines the signature for functions executed during the shutdown process.
// It receives a context with a timeout to ensure operations do not hang indefinitely.
type CleanupFunc func(ctx context.Context) error

// Task represents a single unit of work to be gracefully stopped.
// Examples include database connections, Redis clients, or HTTP servers.
type Task struct {
	Name    string
	Cleanup CleanupFunc
}

var (
	mu    sync.Mutex
	tasks []Task
)

// Register adds a cleanup function to the graceful shutdown queue.
// It is thread-safe and can be safely called from any goroutine during application initialization.
//
// Example:
//
//	gracefulw.Register("PostgreSQL", db.Close)
func Register(name string, cleanup CleanupFunc) {
	mu.Lock()
	defer mu.Unlock()

	tasks = append(tasks, Task{
		Name:    name,
		Cleanup: cleanup,
	})

	logw.Infof("Registered service '%s' for graceful shutdown", name)
}

// Start executes a blocking function (like starting an HTTP server) in a background goroutine,
// and immediately blocks the main thread waiting for an OS termination signal.
// Upon receiving the signal (SIGINT or SIGTERM), it triggers the graceful shutdown of all registered tasks.
func Start(startFunc func() error, shutdownTimeout time.Duration) {
	// Execute the blocking process in a separate goroutine
	go func() {
		err := startFunc()
		// We ignore http.ErrServerClosed because it is expected when the server is intentionally shut down
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logw.Errorf("Service stopped unexpectedly: %v", err)
		}
	}()

	// Block the main thread and wait for OS termination signals
	Wait(shutdownTimeout)
}

// Wait blocks the main execution thread until an OS termination signal is received.
// It uses modern signal.NotifyContext for safe and clean context cancellation.
func Wait(timeout time.Duration) {
	// Create a context that cancels when an Interrupt or SIGTERM signal is received
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Block execution until the signal is caught
	<-ctx.Done()
	logw.Warning("Received termination signal, initiating graceful shutdown...")

	// Create a new context specifically for the shutdown process with a hard deadline
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	Execute(shutdownCtx)
}

// Execute runs all registered cleanup tasks concurrently.
// It is exposed publicly to allow manual triggering of the shutdown process,
// which is especially useful for unit testing without relying on OS signals.
func Execute(ctx context.Context) {
	var wg sync.WaitGroup

	// Safely retrieve and clear the task list to prevent duplicate executions
	mu.Lock()
	registeredTasks := tasks
	tasks = nil
	mu.Unlock()

	for _, task := range registeredTasks {
		wg.Add(1)

		go func(t Task) {
			defer wg.Done()

			logw.Infof("Stopping service: %s...", t.Name)
			if err := t.Cleanup(ctx); err != nil {
				logw.Errorf("Failed to stop service '%s': %v", t.Name, err)
			} else {
				logw.Infof("Service '%s' stopped successfully", t.Name)
			}
		}(task)
	}

	// Wait for all cleanup tasks to finish in a separate goroutine
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Block until either all tasks are done OR the timeout context expires
	select {
	case <-done:
		logw.Info("Graceful shutdown completed successfully. All services stopped.")
	case <-ctx.Done():
		logw.Error("Graceful shutdown timed out. Forcing exit!")
	}
}
