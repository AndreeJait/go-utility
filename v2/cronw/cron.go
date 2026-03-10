// Package cronw provides a robust, centralized cron job scheduler.
// It supports standard cron expressions, middleware chaining, and graceful shutdowns.
package cronw

import (
	"context"
	"fmt"

	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/robfig/cron/v3"
)

// Handler defines the signature for processing a scheduled job.
// If it returns an error, the error will be logged by the scheduler.
type Handler func(ctx context.Context) error

// Scheduler defines the contract for managing background cron jobs.
type Scheduler interface {
	// Register schedules a new job using a standard cron expression (e.g., "0 0 * * *").
	// It accepts a primary handler and an optional chain of middleware handlers.
	Register(pattern string, handler Handler, middlewares ...Handler) (int, error)

	// Start begins the cron scheduler in the background.
	Start()

	// Close halts the scheduler and blocks until all currently running jobs finish.
	// This ensures safe and graceful shutdowns.
	Close() error
}

type cronScheduler struct {
	cronEngine *cron.Cron
}

// New initializes a new Cron Scheduler.
// We configure it to use the standard 5-field cron specification (Minute, Hour, Dom, Month, Dow).
func New() Scheduler {
	// Note: If you want second-level precision (6 fields), use cron.WithSeconds()
	c := cron.New()
	return &cronScheduler{
		cronEngine: c,
	}
}

// ExecuteHandlers runs the middleware chain followed by the primary handler.
// It stops execution immediately if any handler returns an error.
func ExecuteHandlers(ctx context.Context, handlers ...Handler) error {
	for _, h := range handlers {
		if err := h(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *cronScheduler) Register(pattern string, handler Handler, middlewares ...Handler) (int, error) {
	// Combine middlewares and the main handler into a single execution chain
	allHandlers := append(middlewares, handler)

	// Wrap the execution chain in a standard func() that robfig/cron understands
	jobFunc := func() {
		// Create a fresh context for this specific job execution
		ctx := context.Background()

		logw.Infof("cronw: starting job [pattern: %s]", pattern)

		if err := ExecuteHandlers(ctx, allHandlers...); err != nil {
			logw.Errorf("cronw: job failed [pattern: %s]: %v", pattern, err)
		} else {
			logw.Infof("cronw: job completed successfully [pattern: %s]", pattern)
		}
	}

	// Add the job to the cron engine
	entryID, err := s.cronEngine.AddFunc(pattern, jobFunc)
	if err != nil {
		return 0, fmt.Errorf("cronw: failed to register pattern %s: %w", pattern, err)
	}

	return int(entryID), nil
}

func (s *cronScheduler) Start() {
	s.cronEngine.Start()
	logw.Info("cronw: scheduler started")
}

func (s *cronScheduler) Close() error {
	logw.Info("cronw: shutting down scheduler, waiting for active jobs to complete...")

	// Stop() prevents new jobs from starting and returns a context.
	// The context's Done() channel is closed when all running jobs finish.
	ctx := s.cronEngine.Stop()
	<-ctx.Done()

	logw.Info("cronw: scheduler successfully stopped")
	return nil
}
