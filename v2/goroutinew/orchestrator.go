// Package goroutinew provides a robust, concurrent task orchestrator.
// It supports executing single tasks and batched tasks simultaneously
// within a bounded worker pool, ensuring strict timeouts and thread safety.
package goroutinew

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// StepFunc defines the signature for a concurrent task.
type StepFunc func(ctx context.Context, input any) (any, error)

// Orchestrator manages the registration and execution of concurrent tasks.
type Orchestrator struct {
	maxWorkers int
	timeout    time.Duration

	steps       map[string]StepFunc
	singleInput map[string]any
	batchInput  map[string][]any

	results sync.Map // Thread-safe storage for outputs

	errMu  sync.Mutex
	errors []error
}

// Option applies configuration to the Orchestrator.
type Option func(*Orchestrator)

// WithMaxWorkers limits the number of active goroutines to prevent resource exhaustion.
func WithMaxWorkers(workers int) Option {
	return func(o *Orchestrator) {
		o.maxWorkers = workers
	}
}

// WithTimeout sets a strict deadline for the entire orchestration process.
func WithTimeout(d time.Duration) Option {
	return func(o *Orchestrator) {
		o.timeout = d
	}
}

// New initializes a new Goroutine Orchestrator.
// Defaults to 10 concurrent workers and a 30-second total timeout.
func New(opts ...Option) *Orchestrator {
	o := &Orchestrator{
		maxWorkers:  10,
		timeout:     30 * time.Second,
		steps:       make(map[string]StepFunc),
		singleInput: make(map[string]any),
		batchInput:  make(map[string][]any),
		errors:      make([]error, 0),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// AddStep registers a handler function under a specific key.
func (o *Orchestrator) AddStep(key string, handler StepFunc) {
	o.steps[key] = handler
}

// AddInput assigns a single input to a registered step.
func (o *Orchestrator) AddInput(key string, input any) {
	o.singleInput[key] = input
}

// AddBatchInput assigns an array of inputs to a registered step.
// The orchestrator will run the handler concurrently for every item in the slice,
// while guaranteeing the final result array matches the input order.
func (o *Orchestrator) AddBatchInput(key string, inputs []any) {
	o.batchInput[key] = inputs
}

// job represents an internal unit of work for the worker pool.
type job struct {
	key        string
	batchIndex int // -1 if single execution
	input      any
	handler    StepFunc
}

// Run executes all registered inputs concurrently.
// It respects the maxWorkers limit and the context timeout.
func (o *Orchestrator) Run(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	var jobs []job

	// 1. Queue Single Inputs
	for key, input := range o.singleInput {
		if handler, exists := o.steps[key]; exists {
			jobs = append(jobs, job{key: key, batchIndex: -1, input: input, handler: handler})
		}
	}

	// 2. Queue Batch Inputs
	for key, inputs := range o.batchInput {
		if handler, exists := o.steps[key]; exists {
			// Pre-allocate slice to ensure thread-safe ordered insertion later
			o.results.Store(key, make([]any, len(inputs)))

			for i, input := range inputs {
				jobs = append(jobs, job{key: key, batchIndex: i, input: input, handler: handler})
			}
		}
	}

	// 3. Execute Jobs with Bounded Concurrency
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, o.maxWorkers)

	for _, j := range jobs {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire worker slot

		go func(currentJob job) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release worker slot

			res, err := currentJob.handler(timeoutCtx, currentJob.input)
			if err != nil {
				o.addError(fmt.Errorf("goroutinew step [%s] failed: %w", currentJob.key, err))
				return
			}

			if currentJob.batchIndex == -1 {
				// Store single result
				o.results.Store(currentJob.key, res)
			} else {
				// Store batch result in the exact index (Thread-safe because each index is unique)
				if val, ok := o.results.Load(currentJob.key); ok {
					slice := val.([]any)
					slice[currentJob.batchIndex] = res
				}
			}
		}(j)
	}

	wg.Wait()

	if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
		o.addError(context.DeadlineExceeded)
	}

	return o.GetError()
}

func (o *Orchestrator) addError(err error) {
	o.errMu.Lock()
	defer o.errMu.Unlock()
	o.errors = append(o.errors, err)
}

// GetError returns the first error encountered during execution, or nil if completely successful.
func (o *Orchestrator) GetError() error {
	o.errMu.Lock()
	defer o.errMu.Unlock()
	if len(o.errors) > 0 {
		return o.errors[0]
	}
	return nil
}

// GetErrors returns all errors encountered by the workers.
func (o *Orchestrator) GetErrors() []error {
	o.errMu.Lock()
	defer o.errMu.Unlock()
	return o.errors
}

// GetResp retrieves the result for a given key.
// If the key was a Batch, it returns []any. If it was Single, it returns any.
func (o *Orchestrator) GetResp(key string) any {
	if val, ok := o.results.Load(key); ok {
		return val
	}
	return nil
}
