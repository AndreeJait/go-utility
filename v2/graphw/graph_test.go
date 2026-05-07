package graphw

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// testState is a simple state type for testing.
type testState struct {
	Messages []string
	Count    int
}

// testReducer appends messages and sums counts.
func testReducer(current, update testState) testState {
	current.Messages = append(current.Messages, update.Messages...)
	current.Count += update.Count
	return current
}

// TestBuilderValidation_NoNodes tests that compiling a graph with no nodes fails.
func TestBuilderValidation_NoNodes(t *testing.T) {
	_, err := NewBuilder[testState](testReducer).Compile()
	if err != ErrNoNodes {
		t.Fatalf("expected ErrNoNodes, got %v", err)
	}
}

// TestBuilderValidation_NoStartEdge tests that compiling a graph without a START edge fails.
func TestBuilderValidation_NoStartEdge(t *testing.T) {
	_, err := NewBuilder[testState](testReducer).
		AddNode("a", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: s}, nil
		}).
		Compile()
	if err != ErrNoStartEdge {
		t.Fatalf("expected ErrNoStartEdge, got %v", err)
	}
}

// TestBuilderValidation_InvalidEdgeToSTART tests that adding an edge to START panics.
func TestBuilderValidation_InvalidEdgeToSTART(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for edge to START")
		}
	}()
	NewBuilder[testState](testReducer).AddEdge("a", START)
}

// TestBuilderValidation_InvalidEdgeFromEND tests that adding an edge from END panics.
func TestBuilderValidation_InvalidEdgeFromEND(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for edge from END")
		}
	}()
	NewBuilder[testState](testReducer).AddEdge(END, "a")
}

// TestBuilderValidation_DuplicateNode tests that adding a duplicate node panics.
func TestBuilderValidation_DuplicateNode(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for duplicate node")
		}
	}()
	noop := func(ctx context.Context, s testState) (NodeResult[testState], error) {
		return NodeResult[testState]{State: s}, nil
	}
	NewBuilder[testState](testReducer).
		AddNode("a", noop).
		AddNode("a", noop)
}

// TestLinearGraph tests a simple linear graph: START -> A -> B -> END.
func TestLinearGraph(t *testing.T) {
	graph, err := NewBuilder[testState](testReducer).
		AddNode("a", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"a"}, Count: 1}}, nil
		}).
		AddNode("b", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"b"}, Count: 2}}, nil
		}).
		AddEdge(START, "a").
		AddEdge("a", "b").
		AddEdge("b", END).
		Compile()
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	result, err := graph.Invoke(context.Background(), testState{})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}

	if len(result.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result.Messages))
	}
	if result.Messages[0] != "a" || result.Messages[1] != "b" {
		t.Errorf("expected messages [a, b], got %v", result.Messages)
	}
	if result.Count != 3 {
		t.Errorf("expected count 3, got %d", result.Count)
	}
}

// TestConditionalEdges tests a graph with conditional routing.
func TestConditionalEdges(t *testing.T) {
	graph, err := NewBuilder[testState](testReducer).
		AddNode("router", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			// Add a message based on the input, pass count through
			return NodeResult[testState]{State: testState{Messages: []string{"routed"}, Count: 1}}, nil
		}).
		AddNode("positive", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"positive"}, Count: 10}}, nil
		}).
		AddNode("negative", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"negative"}, Count: 20}}, nil
		}).
		AddEdge(START, "router").
		AddConditionalEdge("router", func(ctx context.Context, s testState) (string, error) {
			if s.Count >= 5 {
				return "pos", nil
			}
			return "neg", nil
		}, map[string]string{"pos": "positive", "neg": "negative"}).
		AddEdge("positive", END).
		AddEdge("negative", END).
		Compile()
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	// Test with initial count that triggers positive path
	// After router: Count 0+1=1. s.Count=1 < 5, so goes to negative
	result, err := graph.Invoke(context.Background(), testState{Count: 0})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	// router (count+1=1) → negative (count+20=21)
	if result.Count != 21 {
		t.Errorf("expected count 21, got %d", result.Count)
	}

	// Test with initial count that triggers positive path
	// After router: Count 10+1=11. s.Count=11 >= 5, so goes to positive
	result, err = graph.Invoke(context.Background(), testState{Count: 10})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	// router (10+1=11) → positive (11+10=21)
	if result.Count != 21 {
		t.Errorf("expected count 21, got %d", result.Count)
	}
}

// TestCycle tests a graph with a cycle (agent loop pattern).
func TestCycle(t *testing.T) {
	callCount := 0

	graph, err := NewBuilder[testState](testReducer).
		AddNode("agent", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			callCount++
			if callCount >= 3 {
				return NodeResult[testState]{
					State: testState{Messages: []string{fmt.Sprintf("agent-%d", callCount)}, Count: 1},
					Route: END,
				}, nil
			}
			return NodeResult[testState]{
				State: testState{Messages: []string{fmt.Sprintf("agent-%d", callCount)}, Count: 1},
			}, nil
		}).
		AddEdge(START, "agent").
		AddConditionalEdge("agent", func(ctx context.Context, s testState) (string, error) {
			if callCount >= 3 {
				return END, nil
			}
			return "agent", nil
		}, nil).
		Compile()
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	result, err := graph.Invoke(context.Background(), testState{})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}

	if callCount != 3 {
		t.Errorf("expected 3 agent calls, got %d", callCount)
	}
	if len(result.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(result.Messages))
	}
}

// TestCommandRouting tests that NodeResult.Route overrides edge routing.
func TestCommandRouting(t *testing.T) {
	graph, err := NewBuilder[testState](testReducer).
		AddNode("start", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{
				State: testState{Messages: []string{"start"}, Count: 1},
				Route: "end",
			}, nil
		}).
		AddNode("middle", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"middle"}, Count: 1}}, nil
		}).
		AddNode("end", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"end"}, Count: 1}}, nil
		}).
		AddEdge(START, "start").
		AddEdge("start", "middle"). // This edge should be overridden by Route
		AddEdge("end", END).
		Compile()
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	result, err := graph.Invoke(context.Background(), testState{})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}

	// "middle" should NOT be in the path because Route overrode the edge
	for _, msg := range result.Messages {
		if msg == "middle" {
			t.Error("middle should not be in the path when Route overrides")
		}
	}
}

// TestRecursionLimit tests that the graph returns ErrRecursionLimit.
func TestRecursionLimit(t *testing.T) {
	graph, err := NewBuilder[testState](testReducer).
		AddNode("loop", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Count: 1}}, nil
		}).
		AddEdge(START, "loop").
		AddEdge("loop", "loop"). // Infinite loop
		Compile(WithRecursionLimit(5))
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	_, err = graph.Invoke(context.Background(), testState{})
	if err == nil {
		t.Fatal("expected error for recursion limit, got nil")
	}
	if err.Error() != "graphw: recursion limit exceeded: exceeded 5 supersteps" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCheckpointing tests that the InMemoryCheckpointer saves and restores state.
func TestCheckpointing(t *testing.T) {
	cp := NewInMemoryCheckpointer()

	graph, err := NewBuilder[testState](testReducer).
		AddNode("a", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"a"}, Count: 1}}, nil
		}).
		AddEdge(START, "a").
		AddEdge("a", END).
		Compile(WithCheckpointer(cp))
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	result, err := graph.Invoke(context.Background(), testState{}, WithThreadID("thread-1"))
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}

	// Check checkpoint was saved
	snapshot, err := graph.GetState("thread-1")
	if err != nil {
		t.Fatalf("get state failed: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.State.Messages) != 1 {
		t.Errorf("expected 1 message in checkpoint, got %d", len(snapshot.State.Messages))
	}
	if snapshot.State.Count != 1 {
		t.Errorf("expected count 1 in checkpoint, got %d", snapshot.State.Count)
	}

	// Verify the result matches
	if result.Count != 1 {
		t.Errorf("expected count 1, got %d", result.Count)
	}
}

// TestInterruptBefore tests that interrupt-before pauses execution.
func TestInterruptBefore(t *testing.T) {
	graph, err := NewBuilder[testState](testReducer).
		AddNode("a", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"a"}, Count: 1}}, nil
		}).
		AddNode("b", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"b"}, Count: 2}}, nil
		}).
		AddEdge(START, "a").
		AddEdge("a", "b").
		AddEdge("b", END).
		Compile(WithCheckpointer(NewInMemoryCheckpointer()))
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	// Invoke with interrupt before "b"
	_, err = graph.Invoke(context.Background(), testState{},
		WithThreadID("thread-1"),
		WithInterruptBefore("b"),
	)
	if err == nil {
		t.Fatal("expected InterruptError, got nil")
	}

	intErr, ok := err.(*InterruptError)
	if !ok {
		t.Fatalf("expected InterruptError, got %T: %v", err, err)
	}
	if intErr.Node != "b" {
		t.Errorf("expected interrupt at node 'b', got %q", intErr.Node)
	}
}

// TestStreaming tests that Stream yields steps correctly.
func TestStreaming(t *testing.T) {
	graph, err := NewBuilder[testState](testReducer).
		AddNode("a", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"a"}, Count: 1}}, nil
		}).
		AddNode("b", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"b"}, Count: 2}}, nil
		}).
		AddEdge(START, "a").
		AddEdge("a", "b").
		AddEdge("b", END).
		Compile()
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	var steps []Step[testState]
	for step, err := range graph.Stream(context.Background(), testState{}) {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		steps = append(steps, step)
	}

	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}

	// Check step 1
	if len(steps[0].Nodes) != 1 || steps[0].Nodes[0] != "a" {
		t.Errorf("step 0: expected node 'a', got %v", steps[0].Nodes)
	}
	if steps[0].State.Count != 1 {
		t.Errorf("step 0: expected count 1, got %d", steps[0].State.Count)
	}

	// Check step 2
	if len(steps[1].Nodes) != 1 || steps[1].Nodes[0] != "b" {
		t.Errorf("step 1: expected node 'b', got %v", steps[1].Nodes)
	}
	if steps[1].State.Count != 3 {
		t.Errorf("step 1: expected count 3, got %d", steps[1].State.Count)
	}
}

// TestUpdateState tests that UpdateState modifies the checkpoint state.
func TestUpdateState(t *testing.T) {
	cp := NewInMemoryCheckpointer()
	graph, err := NewBuilder[testState](testReducer).
		AddNode("a", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"a"}, Count: 1}}, nil
		}).
		AddEdge(START, "a").
		AddEdge("a", END).
		Compile(WithCheckpointer(cp))
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	// First invocation
	_, err = graph.Invoke(context.Background(), testState{}, WithThreadID("thread-1"))
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}

	// Update state
	err = graph.UpdateState("thread-1", testState{Count: 99}, "human")
	if err != nil {
		t.Fatalf("update state failed: %v", err)
	}

	// Verify updated state
	snapshot, err := graph.GetState("thread-1")
	if err != nil {
		t.Fatalf("get state failed: %v", err)
	}
	if snapshot.State.Count != 100 { // reducer sums: 1 + 99 = 100
		t.Errorf("expected count 100, got %d", snapshot.State.Count)
	}
}

// TestInMemoryCheckpointer tests the in-memory checkpointer directly.
func TestInMemoryCheckpointer(t *testing.T) {
	cp := NewInMemoryCheckpointer()
	ctx := context.Background()

	// Put
	err := cp.Put(ctx, Checkpoint{
		ID:        "cp-1",
		ThreadID:  "thread-1",
		State:     []byte(`{"messages":["hello"],"count":1}`),
		Next:      []string{"node-a"},
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}

	// Get latest
	got, err := cp.Get(ctx, "thread-1", "")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.ID != "cp-1" {
		t.Errorf("expected ID cp-1, got %s", got.ID)
	}

	// List
	checkpoints, err := cp.List(ctx, "thread-1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(checkpoints) != 1 {
		t.Errorf("expected 1 checkpoint, got %d", len(checkpoints))
	}

	// Delete
	err = cp.Delete(ctx, "thread-1", "cp-1")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify deleted
	got, err = cp.Get(ctx, "thread-1", "")
	if err != nil {
		t.Fatalf("get after delete failed: %v", err)
	}
	if got != nil {
		t.Error("expected nil checkpoint after deletion")
	}
}

// TestInterruptFunction tests that the Interrupt function works with panic-recover.
func TestInterruptFunction(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected Interrupt to panic")
		}
		sig, ok := r.(interruptSignal)
		if !ok {
			t.Fatalf("expected interruptSignal, got %T", r)
		}
		if sig.payload != "test-payload" {
			t.Errorf("expected payload 'test-payload', got %v", sig.payload)
		}
	}()

	Interrupt("test-payload")
}

// TestResumeValue tests the ResumeValue context helper.
func TestResumeValue(t *testing.T) {
	ctx := context.Background()

	// Without resume value
	if val := ResumeValue(ctx); val != nil {
		t.Errorf("expected nil, got %v", val)
	}

	// With resume value
	ctx = WithResumeValue(ctx, "approved")
	if val := ResumeValue(ctx); val != "approved" {
		t.Errorf("expected 'approved', got %v", val)
	}
}

// TestMermaidExport tests the Mermaid diagram export.
func TestMermaidExport(t *testing.T) {
	graph, err := NewBuilder[testState](testReducer).
		AddNode("think", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: s}, nil
		}).
		AddNode("act", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: s}, nil
		}).
		AddEdge(START, "think").
		AddConditionalEdge("think", func(ctx context.Context, s testState) (string, error) {
			return "act", nil
		}, map[string]string{"act": "act", END: END}).
		AddEdge("act", "think").
		Compile()
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	diagram := ExportMermaid(graph)
	if !strings.Contains(diagram, "graph TD") {
		t.Error("expected 'graph TD' in Mermaid diagram")
	}
	if !strings.Contains(diagram, "think") {
		t.Error("expected 'think' node in Mermaid diagram")
	}
	if !strings.Contains(diagram, "act") {
		t.Error("expected 'act' node in Mermaid diagram")
	}
}

// TestSendDynamicFanout tests the Send mechanism for dynamic fan-out.
func TestSendDynamicFanout(t *testing.T) {
	graph, err := NewBuilder[testState](testReducer).
		AddNode("fan_out", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{
				State: testState{Messages: []string{"fan_out"}, Count: 0},
				Sends: []Send{
					{Node: "process", State: testState{Messages: []string{"item-1"}, Count: 1}},
					{Node: "process", State: testState{Messages: []string{"item-2"}, Count: 2}},
				},
			}, nil
		}).
		AddNode("process", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: append(s.Messages, "processed"), Count: s.Count * 10}}, nil
		}).
		AddNode("gather", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: s}, nil
		}).
		AddEdge(START, "fan_out").
		AddEdge("fan_out", "gather").
		AddEdge("process", "gather").
		AddEdge("gather", END).
		Compile()
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	result, err := graph.Invoke(context.Background(), testState{})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}

	// The reducer should have accumulated messages and summed counts
	if len(result.Messages) == 0 {
		t.Error("expected messages in result")
	}
	if result.Count == 0 {
		t.Error("expected non-zero count in result")
	}
}

// TestParallelFanOutFanIn tests parallel fan-out and fan-in.
func TestParallelFanOutFanIn(t *testing.T) {
	graph, err := NewBuilder[testState](testReducer).
		AddNode("split", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"split"}, Count: 0}}, nil
		}).
		AddNode("left", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"left"}, Count: 10}}, nil
		}).
		AddNode("right", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"right"}, Count: 20}}, nil
		}).
		AddNode("merge", func(ctx context.Context, s testState) (NodeResult[testState], error) {
			return NodeResult[testState]{State: testState{Messages: []string{"merged"}, Count: 0}}, nil
		}).
		AddEdge(START, "split").
		AddEdge("split", "left").
		AddEdge("split", "right").
		AddEdge("left", "merge").
		AddEdge("right", "merge").
		AddEdge("merge", END).
		Compile()
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	result, err := graph.Invoke(context.Background(), testState{})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}

	// left (10) + right (20) = 30 from parallel nodes, then merge adds 0
	if result.Count != 30 {
		t.Errorf("expected count 30, got %d", result.Count)
	}
	// split + left + right + merged = 4 messages
	if len(result.Messages) != 4 {
		t.Errorf("expected 4 messages, got %d: %v", len(result.Messages), result.Messages)
	}
}

// Compile-time interface compliance checks
func TestInterfaceCompliance(t *testing.T) {
	var _ Graph[testState] = (*compiledGraph[testState])(nil)
	var _ Checkpointer = (*InMemoryCheckpointer)(nil)
}