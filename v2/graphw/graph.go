package graphw

import (
	"context"
	"errors"
	"iter"
	"time"
)

// START is the sentinel node name for the graph entry point.
var START = "__start__"

// END is the sentinel node name for the graph exit point.
var END = "__end__"

var (
	// ErrInterrupt indicates that execution was paused for human-in-the-loop review.
	ErrInterrupt = errors.New("graphw: execution interrupted")

	// ErrRecursionLimit indicates the graph exceeded the maximum number of supersteps.
	ErrRecursionLimit = errors.New("graphw: recursion limit exceeded")

	// ErrNoStartEdge indicates no edge is defined from the START node.
	ErrNoStartEdge = errors.New("graphw: no edge defined from START")

	// ErrNodeNotFound indicates a referenced node was not registered in the graph.
	ErrNodeNotFound = errors.New("graphw: node not found")

	// ErrDuplicateNode indicates a node with the same name was already registered.
	ErrDuplicateNode = errors.New("graphw: duplicate node name")

	// ErrInvalidEdge indicates an invalid edge definition.
	ErrInvalidEdge = errors.New("graphw: invalid edge")

	// ErrNoNodes indicates the graph has no nodes.
	ErrNoNodes = errors.New("graphw: graph has no nodes")

	// ErrStepRequiresCheckpointer indicates Step was called without a checkpointer.
	ErrStepRequiresCheckpointer = errors.New("graphw: Step requires a checkpointer")

	// ErrStepRequiresThreadID indicates Step was called without a threadID.
	ErrStepRequiresThreadID = errors.New("graphw: Step requires a threadID option")
)

// --- Core Function Types ---

// Reducer defines how node outputs merge with the current state.
// This is the core mechanism for state management in the graph.
//
// Example — a state with messages and a counter:
//
//	type MyState struct {
//	    Messages []string
//	    Count    int
//	}
//
//	func myReducer(current, update MyState) MyState {
//	    current.Messages = append(current.Messages, update.Messages...)
//	    current.Count += update.Count
//	    return current
//	}
type Reducer[S any] func(current S, update S) S

// NodeFunc is the function signature for a graph node.
// It receives the current state and returns a result with state update and routing.
type NodeFunc[S any] func(ctx context.Context, state S) (NodeResult[S], error)

// RouterFunc is the function signature for conditional edge routing.
// It inspects the state and returns the name of the next node to execute.
type RouterFunc[S any] func(ctx context.Context, state S) (string, error)

// --- Result Types ---

// NodeResult contains the output of a node execution.
type NodeResult[S any] struct {
	// State is the state update to be merged via the Reducer.
	State S
	// Route overrides the next node to execute (Command-style routing).
	// If non-empty, it takes precedence over edge-based routing.
	Route string
	// Sends creates dynamic fan-out targets at runtime.
	// Each Send routes state to a specific node instance in the next superstep.
	Sends []Send
}

// Send routes state to a specific node instance (dynamic fan-out / map-reduce).
// This enables runtime-determined parallelism where the number of tasks
// is not known at compile time.
type Send struct {
	// Node is the name of the target node.
	Node string
	// State is the input state for the target node.
	// Must be of the same type as the graph's state type S.
	State any
}

// Step represents one superstep in the graph execution (for streaming).
type Step[S any] struct {
	// Node is the name of the node that executed in this step.
	// If multiple nodes executed in parallel, this is empty and
	// the individual node results are in Nodes.
	Node string
	// Nodes contains the names of all nodes that executed in this superstep.
	Nodes []string
	// State is the state after the reducer merge.
	State S
	// Updates contains the state deltas from the nodes that executed.
	Updates []NodeResult[S]
}

// StepResult contains the output of a single superstep execution.
// It is returned by the Step method for distributed/async execution patterns
// where each superstep is handled separately (e.g., via a message queue).
type StepResult[S any] struct {
	// Nodes contains the names of all nodes that executed in this superstep.
	Nodes []string
	// State is the graph state after the reducer merge.
	State S
	// Updates contains the state deltas from the nodes that executed.
	Updates []NodeResult[S]
	// Next contains the names of nodes scheduled to execute next.
	// Empty or contains only END means the graph is done.
	Next []string
	// IsDone is true when the graph has reached END or has no more nodes to execute.
	IsDone bool
}

// --- Graph Interface ---

// Graph is the compiled, executable stateful graph.
// Use Builder to construct a Graph, then call Invoke or Stream to execute it.
type Graph[S any] interface {
	// Invoke runs the graph to completion and returns the final state.
	Invoke(ctx context.Context, state S, opts ...RunOption) (S, error)

	// Stream runs the graph and yields Steps as each superstep completes.
	Stream(ctx context.Context, state S, opts ...RunOption) iter.Seq2[Step[S], error]

	// GetState retrieves the current state snapshot for a given thread.
	// Requires a Checkpointer to be configured.
	GetState(threadID string) (*StateSnapshot[S], error)

	// UpdateState modifies the state for a given thread.
	// This is useful for human-in-the-loop workflows where a human
	// modifies the state before resuming execution.
	// Requires a Checkpointer to be configured.
	UpdateState(threadID string, values S, asNode string) error

	// Step executes exactly one superstep and returns the result.
	// On the first call with a given threadID, it resolves the START edges and
	// executes those nodes. On subsequent calls, it reads the checkpoint to
	// determine which nodes to execute next.
	// Requires a Checkpointer to be configured and a threadID via WithThreadID.
	Step(ctx context.Context, state S, opts ...RunOption) (*StepResult[S], error)

	// Redirect changes the next nodes to execute for a given thread.
	// It creates a new checkpoint with the current state but modified Next field.
	// Useful for dynamically altering execution flow (e.g., from a State API).
	// Requires a Checkpointer to be configured.
	Redirect(threadID string, nextNodes []string) error

	// RevertTo time-travels to a previous checkpoint.
	// It loads the target checkpoint's state and Next, then saves a new
	// checkpoint forking from that point. The graph resumes from the reverted
	// state on the next Step call.
	// Requires a Checkpointer to be configured.
	RevertTo(threadID, checkpointID string) error
}

// StateSnapshot captures the state at a point in time for checkpointing.
type StateSnapshot[S any] struct {
	// State is the graph state at this checkpoint.
	State S
	// Next contains the names of nodes scheduled to execute next.
	Next []string
	// ThreadID identifies the conversation thread.
	ThreadID string
	// CheckpointID is the unique identifier for this checkpoint.
	CheckpointID string
	// ParentCheckpointID is the ID of the previous checkpoint (for time-travel).
	ParentCheckpointID string
	// CreatedAt is when this checkpoint was created.
	CreatedAt time.Time
	// Metadata contains optional key-value pairs for this checkpoint.
	Metadata map[string]any
}

// --- Checkpointing ---

// Checkpointer is the interface for graph state persistence.
// Implementations store and retrieve checkpoints keyed by thread ID.
//
// The interface uses []byte for state (JSON-serialized) rather than the generic S,
// so multiple backends can implement it without type parameters.
// The engine handles serialization/deserialization between S and []byte.
type Checkpointer interface {
	// Put saves a checkpoint.
	Put(ctx context.Context, checkpoint Checkpoint) error
	// Get retrieves a checkpoint by thread ID and checkpoint ID.
	// If checkpointID is empty, returns the latest checkpoint for the thread.
	Get(ctx context.Context, threadID, checkpointID string) (*Checkpoint, error)
	// List returns checkpoints for a thread, ordered from newest to oldest.
	List(ctx context.Context, threadID string, opts ...CheckpointListOpt) ([]Checkpoint, error)
	// Delete removes a checkpoint.
	Delete(ctx context.Context, threadID, checkpointID string) error
}

// Checkpoint represents a saved graph state at a point in time.
type Checkpoint struct {
	// ID is the unique identifier for this checkpoint.
	ID string
	// ThreadID identifies the conversation thread.
	ThreadID string
	// ParentID is the ID of the previous checkpoint.
	ParentID string
	// State is the JSON-serialized graph state.
	State []byte
	// Next contains the names of nodes scheduled to execute next.
	Next []string
	// CreatedAt is when this checkpoint was created.
	CreatedAt time.Time
	// Metadata contains optional key-value pairs.
	Metadata map[string]any
}

// CheckpointListOpt is a functional option for listing checkpoints.
type CheckpointListOpt func(*checkpointListConfig)

type checkpointListConfig struct {
	limit int
}

// WithCheckpointLimit sets the maximum number of checkpoints to return.
func WithCheckpointLimit(limit int) CheckpointListOpt {
	return func(c *checkpointListConfig) {
		c.limit = limit
	}
}

// --- Run Options ---

// RunOption is a functional option for graph execution.
type RunOption func(*runConfig)

type runConfig struct {
	threadID       string
	interruptBefore []string
	interruptAfter  []string
}

// WithThreadID sets the thread ID for this execution.
// Thread IDs enable state persistence and conversation continuity
// across multiple Invoke/Stream calls.
func WithThreadID(id string) RunOption {
	return func(c *runConfig) {
		c.threadID = id
	}
}

// WithInterruptBefore pauses execution before the specified nodes execute.
// The graph will return ErrInterrupt with the interrupt info.
func WithInterruptBefore(nodes ...string) RunOption {
	return func(c *runConfig) {
		c.interruptBefore = append(c.interruptBefore, nodes...)
	}
}

// WithInterruptAfter pauses execution after the specified nodes complete.
func WithInterruptAfter(nodes ...string) RunOption {
	return func(c *runConfig) {
		c.interruptAfter = append(c.interruptAfter, nodes...)
	}
}

// --- Compile Options ---

// CompileOption is a functional option for graph compilation.
type CompileOption func(*compileConfig)

type compileConfig struct {
	checkpointer   Checkpointer
	recursionLimit int
	interruptBefore []string
	interruptAfter  []string
}

// WithCheckpointer sets the checkpointer for state persistence.
func WithCheckpointer(cp Checkpointer) CompileOption {
	return func(c *compileConfig) {
		c.checkpointer = cp
	}
}

// WithRecursionLimit sets the maximum number of supersteps before
// the graph returns ErrRecursionLimit. Default is 25.
func WithRecursionLimit(limit int) CompileOption {
	return func(c *compileConfig) {
		c.recursionLimit = limit
	}
}

// WithCompileInterruptBefore pauses execution before the specified nodes
// at compile time. Can be overridden at run time.
func WithCompileInterruptBefore(nodes ...string) CompileOption {
	return func(c *compileConfig) {
		c.interruptBefore = append(c.interruptBefore, nodes...)
	}
}

// WithCompileInterruptAfter pauses execution after the specified nodes
// at compile time. Can be overridden at run time.
func WithCompileInterruptAfter(nodes ...string) CompileOption {
	return func(c *compileConfig) {
		c.interruptAfter = append(c.interruptAfter, nodes...)
	}
}

// --- Node Options ---

// NodeOption is a functional option for node registration.
type NodeOption func(*nodeConfig)

type nodeConfig struct {
	retry      int
	retryDelay time.Duration
}

// WithRetry sets the maximum number of retries for a node on failure.
func WithRetry(maxRetries int) NodeOption {
	return func(c *nodeConfig) {
		c.retry = maxRetries
	}
}

// WithRetryDelay sets the delay between retries.
func WithRetryDelay(delay time.Duration) NodeOption {
	return func(c *nodeConfig) {
		c.retryDelay = delay
	}
}