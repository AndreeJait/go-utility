package graphw

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// InMemoryCheckpointer is a thread-safe in-memory implementation of Checkpointer.
// It is intended for testing and development. State is lost when the process exits.
type InMemoryCheckpointer struct {
	mu          sync.RWMutex
	checkpoints map[string][]Checkpoint // threadID -> checkpoints (newest first)
}

// NewInMemoryCheckpointer creates a new in-memory checkpointer.
func NewInMemoryCheckpointer() *InMemoryCheckpointer {
	return &InMemoryCheckpointer{
		checkpoints: make(map[string][]Checkpoint),
	}
}

// Put saves a checkpoint.
func (c *InMemoryCheckpointer) Put(ctx context.Context, checkpoint Checkpoint) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	threadID := checkpoint.ThreadID
	c.checkpoints[threadID] = append(c.checkpoints[threadID], checkpoint)

	// Sort newest first
	sort.Slice(c.checkpoints[threadID], func(i, j int) bool {
		return c.checkpoints[threadID][i].CreatedAt.After(c.checkpoints[threadID][j].CreatedAt)
	})

	return nil
}

// Get retrieves a checkpoint by thread ID and checkpoint ID.
// If checkpointID is empty, returns the latest checkpoint for the thread.
func (c *InMemoryCheckpointer) Get(ctx context.Context, threadID, checkpointID string) (*Checkpoint, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	checkpoints, exists := c.checkpoints[threadID]
	if !exists || len(checkpoints) == 0 {
		return nil, nil
	}

	// If checkpointID is empty, return the latest
	if checkpointID == "" {
		return &checkpoints[0], nil
	}

	// Find by ID
	for i := range checkpoints {
		if checkpoints[i].ID == checkpointID {
			return &checkpoints[i], nil
		}
	}

	return nil, fmt.Errorf("graphw: checkpoint %q not found for thread %q", checkpointID, threadID)
}

// List returns checkpoints for a thread, ordered from newest to oldest.
func (c *InMemoryCheckpointer) List(ctx context.Context, threadID string, opts ...CheckpointListOpt) ([]Checkpoint, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	checkpoints, exists := c.checkpoints[threadID]
	if !exists {
		return nil, nil
	}

	cfg := &checkpointListConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	result := make([]Checkpoint, len(checkpoints))
	copy(result, checkpoints)

	if cfg.limit > 0 && cfg.limit < len(result) {
		result = result[:cfg.limit]
	}

	return result, nil
}

// Delete removes a checkpoint.
func (c *InMemoryCheckpointer) Delete(ctx context.Context, threadID, checkpointID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	checkpoints, exists := c.checkpoints[threadID]
	if !exists {
		return nil
	}

	for i, cp := range checkpoints {
		if cp.ID == checkpointID {
			c.checkpoints[threadID] = append(checkpoints[:i], checkpoints[i+1:]...)
			return nil
		}
	}

	return nil
}

// Compile-time interface compliance check.
var _ Checkpointer = (*InMemoryCheckpointer)(nil)