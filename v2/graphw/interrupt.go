package graphw

import (
	"context"
	"fmt"
)

// interruptSignal is the sentinel type used by Interrupt() to pause execution.
// It is carried via panic and recovered by the engine.
type interruptSignal struct {
	node    string
	step    int
	payload any
}

// InterruptError represents an interruption in graph execution.
// It carries information about which node was interrupted and any payload.
type InterruptError struct {
	// Node is the name of the node where execution was interrupted.
	Node string
	// Step is the superstep number where execution was interrupted.
	Step int
	// Payload is optional data passed from the interrupted node.
	Payload any
}

// Error implements the error interface.
func (e *InterruptError) Error() string {
	return fmt.Sprintf("graphw: execution interrupted at node %q (step %d)", e.Node, e.Step)
}

// Interrupt pauses execution and surfaces a payload to the caller.
// Call this from within a NodeFunc to request human-in-the-loop review.
//
// On the first invocation, this function panics with an interrupt signal.
// The engine catches this, saves a checkpoint, and returns an InterruptError.
//
// On resume, the engine re-invokes the interrupted node with the resume value
// available via ResumeValue(ctx). Use this pattern:
//
//	func reviewNode(ctx context.Context, state MyState) (graphw.NodeResult[MyState], error) {
//	    if graphw.ResumeValue(ctx) == nil {
//	        graphw.Interrupt(map[string]any{"draft": state.Draft}) // pauses execution
//	    }
//	    // Execution resumes here with the human's input
//	    state.Approved = true
//	    return graphw.NodeResult[MyState]{State: state}, nil
//	}
//
// This function uses panic-recover internally. Do not catch the panic yourself —
// the engine will handle it.
func Interrupt(payload any) {
	panic(interruptSignal{payload: payload})
}

type resumeKey struct{}

// WithResumeValue sets a resume value in the context for an interrupted node.
// This is used internally by the engine when resuming from an interrupt.
func WithResumeValue(ctx context.Context, value any) context.Context {
	return context.WithValue(ctx, resumeKey{}, value)
}

// ResumeValue retrieves the resume value from the context.
// Call this inside a node that previously called Interrupt() to get the
// human's input. Returns nil if no resume value is present (first invocation).
func ResumeValue(ctx context.Context) any {
	return ctx.Value(resumeKey{})
}