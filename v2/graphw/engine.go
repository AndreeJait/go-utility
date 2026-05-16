package graphw

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"sort"
	"sync"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
)

// compiledGraph is the immutable, executable graph produced by Builder.Compile.
type compiledGraph[S any] struct {
	reducer      Reducer[S]
	nodes        map[string]nodeDef[S]
	staticEdges  map[string][]string // from -> list of to nodes (fan-out)
	condEdges    map[string]condEdgeDef[S]
	checkpointer Checkpointer
	intBefore    map[string]bool
	intAfter     map[string]bool
	recLimit     int
	inputKeys    []string
	outputKeys   []string
	nodeOrder    []string
}

// Compile-time interface compliance check.
var _ Graph[any] = (*compiledGraph[any])(nil)

// Invoke runs the graph to completion and returns the final state.
func (g *compiledGraph[S]) Invoke(ctx context.Context, state S, opts ...RunOption) (S, error) {
	rc := &runConfig{}
	for _, opt := range opts {
		opt(rc)
	}

	// Merge compile-time and run-time interrupt settings
	intBefore := g.mergeInterruptSets(g.intBefore, rc.interruptBefore)
	intAfter := g.mergeInterruptSets(g.intAfter, rc.interruptAfter)

	// If threadID is set and checkpointer exists, try to restore state
	currentState := state
	nextNodes := g.resolveNextNodes(START)

	if rc.threadID != "" && g.checkpointer != nil {
		if cp, err := g.checkpointer.Get(ctx, rc.threadID, ""); err == nil && cp != nil {
			var restored S
			if err := json.Unmarshal(cp.State, &restored); err == nil {
				currentState = g.reducer(restored, state)
			}
			if len(cp.Next) > 0 {
				nextNodes = cp.Next
			}
		}
	}

	// Execute supersteps
	for step := 0; step < g.recLimit; step++ {
		// Check if we've reached END
		if len(nextNodes) == 0 || containsEnd(nextNodes) {
			break
		}

		// Check interrupt-before
		for _, node := range nextNodes {
			if intBefore[node] {
				logw.CtxInfof(ctx, "graphw: interrupt before node %q (step %d)", node, step)
				if err := g.saveCheckpoint(ctx, rc.threadID, currentState, nextNodes, step); err != nil {
					logw.CtxErrorf(ctx, "graphw: failed to save checkpoint: %v", err)
				}
				return currentState, &InterruptError{
					Node:    node,
					Step:    step,
					Payload: nil,
				}
			}
		}

		// Execute all nodes in parallel
		results, err := g.executeNodes(ctx, nextNodes, currentState)
		if err != nil {
			return currentState, err
		}

		// Apply reducers in deterministic order
		updates := g.sortResults(nextNodes, results)
		for _, r := range updates {
			currentState = g.reducer(currentState, r.State)
		}

		// Resolve next nodes for the next superstep
		resolvedNext, sends := g.resolveNextNodesFromResults(nextNodes, results, currentState, ctx)

		// Check interrupt-after
		for _, node := range nextNodes {
			if intAfter[node] {
				logw.CtxInfof(ctx, "graphw: interrupt after node %q (step %d)", node, step)
				if err := g.saveCheckpoint(ctx, rc.threadID, currentState, resolvedNext, step+1); err != nil {
					logw.CtxErrorf(ctx, "graphw: failed to save checkpoint: %v", err)
				}
				return currentState, &InterruptError{
					Node:    node,
					Step:    step,
					Payload: nil,
				}
			}
		}

		// Save checkpoint if checkpointer is configured
		if g.checkpointer != nil && rc.threadID != "" {
			if err := g.saveCheckpoint(ctx, rc.threadID, currentState, resolvedNext, step+1); err != nil {
				logw.CtxErrorf(ctx, "graphw: failed to save checkpoint: %v", err)
			}
		}

		// Process Sends (dynamic fan-out)
		if len(sends) > 0 {
			sendResults, err := g.executeSends(ctx, sends, currentState)
			if err != nil {
				return currentState, err
			}
			for _, r := range sendResults {
				currentState = g.reducer(currentState, r.State)
			}
		}

		nextNodes = resolvedNext
	}

	if !containsEnd(nextNodes) && len(nextNodes) > 0 {
		return currentState, fmt.Errorf("%w: exceeded %d supersteps", ErrRecursionLimit, g.recLimit)
	}

	return currentState, nil
}

// Stream runs the graph and yields Steps as each superstep completes.
func (g *compiledGraph[S]) Stream(ctx context.Context, state S, opts ...RunOption) iter.Seq2[Step[S], error] {
	return func(yield func(Step[S], error) bool) {
		rc := &runConfig{}
		for _, opt := range opts {
			opt(rc)
		}

		intBefore := g.mergeInterruptSets(g.intBefore, rc.interruptBefore)
		intAfter := g.mergeInterruptSets(g.intAfter, rc.interruptAfter)

		currentState := state
		nextNodes := g.resolveNextNodes(START)

		if rc.threadID != "" && g.checkpointer != nil {
			if cp, err := g.checkpointer.Get(ctx, rc.threadID, ""); err == nil && cp != nil {
				var restored S
				if err := json.Unmarshal(cp.State, &restored); err == nil {
					currentState = g.reducer(restored, state)
				}
				if len(cp.Next) > 0 {
					nextNodes = cp.Next
				}
			}
		}

		for step := 0; step < g.recLimit; step++ {
			if len(nextNodes) == 0 || containsEnd(nextNodes) {
				break
			}

			// Check interrupt-before
			for _, node := range nextNodes {
				if intBefore[node] {
					g.saveCheckpoint(ctx, rc.threadID, currentState, nextNodes, step)
					yield(Step[S]{}, &InterruptError{Node: node, Step: step})
					return
				}
			}

			results, err := g.executeNodes(ctx, nextNodes, currentState)
			if err != nil {
				yield(Step[S]{}, err)
				return
			}

			updates := g.sortResults(nextNodes, results)
			for _, r := range updates {
				currentState = g.reducer(currentState, r.State)
			}

			// Yield step
			stepResult := Step[S]{
				Nodes:   nextNodes,
				State:   currentState,
				Updates: updates,
			}
			if len(nextNodes) == 1 {
				stepResult.Node = nextNodes[0]
			}

			if !yield(stepResult, nil) {
				return
			}

			// Resolve next nodes
			resolvedNext, sends := g.resolveNextNodesFromResults(nextNodes, results, currentState, ctx)

			// Check interrupt-after
			for _, node := range nextNodes {
				if intAfter[node] {
					g.saveCheckpoint(ctx, rc.threadID, currentState, resolvedNext, step+1)
					yield(Step[S]{}, &InterruptError{Node: node, Step: step})
					return
				}
			}

			if g.checkpointer != nil && rc.threadID != "" {
				g.saveCheckpoint(ctx, rc.threadID, currentState, resolvedNext, step+1)
			}

			// Process Sends
			if len(sends) > 0 {
				sendResults, err := g.executeSends(ctx, sends, currentState)
				if err != nil {
					yield(Step[S]{}, err)
					return
				}
				for _, r := range sendResults {
					currentState = g.reducer(currentState, r.State)
				}
			}

			nextNodes = resolvedNext
		}
	}
}

// GetState retrieves the current state snapshot for a given thread.
func (g *compiledGraph[S]) GetState(threadID string) (*StateSnapshot[S], error) {
	if g.checkpointer == nil {
		return nil, fmt.Errorf("graphw: no checkpointer configured")
	}

	ctx := context.Background()
	cp, err := g.checkpointer.Get(ctx, threadID, "")
	if err != nil {
		return nil, fmt.Errorf("graphw: get state failed: %w", err)
	}
	if cp == nil {
		return nil, fmt.Errorf("graphw: no checkpoint found for thread %q", threadID)
	}

	var state S
	if err := json.Unmarshal(cp.State, &state); err != nil {
		return nil, fmt.Errorf("graphw: unmarshal state: %w", err)
	}

	return &StateSnapshot[S]{
		State:              state,
		Next:               cp.Next,
		ThreadID:           cp.ThreadID,
		CheckpointID:       cp.ID,
		ParentCheckpointID: cp.ParentID,
		CreatedAt:          cp.CreatedAt,
		Metadata:           cp.Metadata,
	}, nil
}

// UpdateState modifies the state for a given thread.
func (g *compiledGraph[S]) UpdateState(threadID string, values S, asNode string) error {
	if g.checkpointer == nil {
		return fmt.Errorf("graphw: no checkpointer configured")
	}

	ctx := context.Background()
	cp, err := g.checkpointer.Get(ctx, threadID, "")
	if err != nil {
		return fmt.Errorf("graphw: get checkpoint for update: %w", err)
	}

	var currentState S
	if cp != nil {
		if err := json.Unmarshal(cp.State, &currentState); err != nil {
			return fmt.Errorf("graphw: unmarshal state: %w", err)
		}
	}

	// Merge the new values using the reducer
	currentState = g.reducer(currentState, values)

	stateBytes, err := json.Marshal(currentState)
	if err != nil {
		return fmt.Errorf("graphw: marshal state: %w", err)
	}

	next := []string{}
	if cp != nil {
		next = cp.Next
	}

	newCheckpoint := Checkpoint{
		ID:        generateCheckpointID(),
		ThreadID:  threadID,
		ParentID:  "",
		State:     stateBytes,
		Next:      next,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"as_node": asNode},
	}
	if cp != nil {
		newCheckpoint.ParentID = cp.ID
	}

	return g.checkpointer.Put(ctx, newCheckpoint)
}

// Step executes exactly one superstep and returns the result.
// On the first call with a given threadID, it resolves the START edges and
// executes those nodes. On subsequent calls, it reads the checkpoint to
// determine which nodes to execute next.
func (g *compiledGraph[S]) Step(ctx context.Context, state S, opts ...RunOption) (*StepResult[S], error) {
	rc := &runConfig{}
	for _, opt := range opts {
		opt(rc)
	}

	if g.checkpointer == nil {
		return nil, ErrStepRequiresCheckpointer
	}
	if rc.threadID == "" {
		return nil, ErrStepRequiresThreadID
	}

	intBefore := g.mergeInterruptSets(g.intBefore, rc.interruptBefore)
	intAfter := g.mergeInterruptSets(g.intAfter, rc.interruptAfter)

	// Restore from checkpoint or use provided state
	currentState := state
	nextNodes := g.resolveNextNodes(START)
	step := 0

	if cp, err := g.checkpointer.Get(ctx, rc.threadID, ""); err == nil && cp != nil {
		var restored S
		if err := json.Unmarshal(cp.State, &restored); err == nil {
			currentState = g.reducer(restored, state)
		}
		if len(cp.Next) > 0 {
			nextNodes = cp.Next
		}
		// Restore step number from checkpoint metadata
		if cp.Metadata != nil {
			if s, ok := cp.Metadata["step"].(float64); ok {
				step = int(s)
			}
		}
	}

	// Check if done
	if len(nextNodes) == 0 || containsEnd(nextNodes) {
		return &StepResult[S]{
			State:  currentState,
			Next:   nil,
			IsDone: true,
		}, nil
	}

	// Check interrupt-before
	for _, node := range nextNodes {
		if intBefore[node] {
			logw.CtxInfof(ctx, "graphw: step interrupt before node %q (step %d)", node, step)
			if err := g.saveCheckpoint(ctx, rc.threadID, currentState, nextNodes, step); err != nil {
				logw.CtxErrorf(ctx, "graphw: failed to save checkpoint: %v", err)
			}
			return nil, &InterruptError{Node: node, Step: step}
		}
	}

	// Execute all nodes in parallel
	results, err := g.executeNodes(ctx, nextNodes, currentState)
	if err != nil {
		return nil, err
	}

	// Apply reducers in deterministic order
	updates := g.sortResults(nextNodes, results)
	for _, r := range updates {
		currentState = g.reducer(currentState, r.State)
	}

	// Resolve next nodes
	resolvedNext, sends := g.resolveNextNodesFromResults(nextNodes, results, currentState, ctx)

	// Check interrupt-after
	for _, node := range nextNodes {
		if intAfter[node] {
			logw.CtxInfof(ctx, "graphw: step interrupt after node %q (step %d)", node, step)
			if err := g.saveCheckpoint(ctx, rc.threadID, currentState, resolvedNext, step+1); err != nil {
				logw.CtxErrorf(ctx, "graphw: failed to save checkpoint: %v", err)
			}
			return nil, &InterruptError{Node: node, Step: step}
		}
	}

	// Process Sends (dynamic fan-out)
	if len(sends) > 0 {
		sendResults, err := g.executeSends(ctx, sends, currentState)
		if err != nil {
			return nil, err
		}
		for _, r := range sendResults {
			currentState = g.reducer(currentState, r.State)
		}
	}

	// Save checkpoint
	if err := g.saveCheckpoint(ctx, rc.threadID, currentState, resolvedNext, step+1); err != nil {
		logw.CtxErrorf(ctx, "graphw: failed to save checkpoint: %v", err)
	}

	isDone := len(resolvedNext) == 0 || containsEnd(resolvedNext)

	return &StepResult[S]{
		Nodes:   nextNodes,
		State:   currentState,
		Updates: updates,
		Next:    resolvedNext,
		IsDone:  isDone,
	}, nil
}

// Redirect changes the next nodes to execute for a given thread.
// It creates a new checkpoint with the current state but a modified Next field.
func (g *compiledGraph[S]) Redirect(threadID string, nextNodes []string) error {
	if g.checkpointer == nil {
		return fmt.Errorf("graphw: no checkpointer configured")
	}

	ctx := context.Background()
	cp, err := g.checkpointer.Get(ctx, threadID, "")
	if err != nil {
		return fmt.Errorf("graphw: get checkpoint for redirect: %w", err)
	}
	if cp == nil {
		return fmt.Errorf("graphw: no checkpoint found for thread %q", threadID)
	}

	newCheckpoint := Checkpoint{
		ID:        generateCheckpointID(),
		ThreadID:  threadID,
		ParentID:  cp.ID,
		State:     cp.State,
		Next:      nextNodes,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"redirect": true},
	}

	return g.checkpointer.Put(ctx, newCheckpoint)
}

// RevertTo time-travels to a previous checkpoint.
// It loads the target checkpoint's state and Next, then saves a new
// checkpoint forking from that point.
func (g *compiledGraph[S]) RevertTo(threadID, checkpointID string) error {
	if g.checkpointer == nil {
		return fmt.Errorf("graphw: no checkpointer configured")
	}

	ctx := context.Background()
	cp, err := g.checkpointer.Get(ctx, threadID, checkpointID)
	if err != nil {
		return fmt.Errorf("graphw: get checkpoint for revert: %w", err)
	}
	if cp == nil {
		return fmt.Errorf("graphw: checkpoint %q not found for thread %q", checkpointID, threadID)
	}

	newCheckpoint := Checkpoint{
		ID:        generateCheckpointID(),
		ThreadID:  threadID,
		ParentID:  cp.ID,
		State:     cp.State,
		Next:      cp.Next,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"reverted_from": cp.ID},
	}

	return g.checkpointer.Put(ctx, newCheckpoint)
}

// --- Internal Methods ---

// executeNodes executes all given nodes in parallel and returns their results.
// Interrupt panics from within nodes are caught and converted to InterruptErrors.
func (g *compiledGraph[S]) executeNodes(ctx context.Context, nodes []string, state S) (map[string]NodeResult[S], error) {
	type nodeResult struct {
		name   string
		result NodeResult[S]
		err    error
	}

	var wg sync.WaitGroup
	resultsCh := make(chan nodeResult, len(nodes))

	for _, nodeName := range nodes {
		def, exists := g.nodes[nodeName]
		if !exists {
			return nil, fmt.Errorf("%w: %q", ErrNodeNotFound, nodeName)
		}

		wg.Add(1)
		go func(name string, def nodeDef[S]) {
			defer wg.Done()

			var result NodeResult[S]
			var err error

			// Retry logic
			for attempt := 0; attempt <= def.retry; attempt++ {
				result, err = g.executeSingleNode(ctx, name, def, state)
				if err == nil {
					break
				}
				if _, ok := err.(*InterruptError); ok {
					break // Don't retry interrupts
				}
				if attempt < def.retry && def.retryDelay > 0 {
					select {
					case <-time.After(def.retryDelay):
					case <-ctx.Done():
						resultsCh <- nodeResult{name: name, err: ctx.Err()}
						return
					}
				}
			}

			resultsCh <- nodeResult{name: name, result: result, err: err}
		}(nodeName, def)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	results := make(map[string]NodeResult[S])
	for r := range resultsCh {
		if r.err != nil {
			return nil, fmt.Errorf("graphw: node %q failed: %w", r.name, r.err)
		}
		results[r.name] = r.result
	}

	return results, nil
}

// executeSingleNode executes a single node, catching interrupt panics.
func (g *compiledGraph[S]) executeSingleNode(ctx context.Context, name string, def nodeDef[S], state S) (result NodeResult[S], err error) {
	defer func() {
		if r := recover(); r != nil {
			if sig, ok := r.(interruptSignal); ok {
				err = &InterruptError{
					Node:    name,
					Payload: sig.payload,
				}
			} else {
				panic(r) // re-panic unexpected panics
			}
		}
	}()

	return def.fn(ctx, state)
}

// executeSends executes dynamic fan-out Sends.
func (g *compiledGraph[S]) executeSends(ctx context.Context, sends []Send, baseState S) ([]NodeResult[S], error) {
	var results []NodeResult[S]

	for _, send := range sends {
		def, exists := g.nodes[send.Node]
		if !exists {
			return nil, fmt.Errorf("%w: %q in Send", ErrNodeNotFound, send.Node)
		}

		// Use Send.State if provided, otherwise use base state
		var sendState S
		if send.State != nil {
			var ok bool
			sendState, ok = send.State.(S)
			if !ok {
				// Try JSON round-trip for type conversion
				bytes, err := json.Marshal(send.State)
				if err != nil {
					return nil, fmt.Errorf("graphw: send state marshal: %w", err)
				}
				if err := json.Unmarshal(bytes, &sendState); err != nil {
					return nil, fmt.Errorf("graphw: send state unmarshal: %w", err)
				}
			}
		} else {
			sendState = baseState
		}

		result, err := def.fn(ctx, sendState)
		if err != nil {
			return nil, fmt.Errorf("graphw: send node %q failed: %w", send.Node, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// resolveNextNodes determines which nodes should execute after the given node.
func (g *compiledGraph[S]) resolveNextNodes(from string) []string {
	// Check conditional edge first
	if _, hasCond := g.condEdges[from]; hasCond {
		// Conditional edges are resolved at runtime, not here.
		// Return a placeholder that will be resolved during execution.
		return nil // Will be resolved at runtime
	}

	// Check static edges (supports fan-out: multiple edges from one node)
	if tos, hasEdge := g.staticEdges[from]; hasEdge {
		result := make([]string, 0, len(tos))
		for _, to := range tos {
			if to == END {
				result = append(result, END)
			} else {
				result = append(result, to)
			}
		}
		return result
	}

	return nil
}

// resolveNextNodesFromResults determines the next nodes based on node results and edges.
func (g *compiledGraph[S]) resolveNextNodesFromResults(
	executedNodes []string,
	results map[string]NodeResult[S],
	state S,
	ctx context.Context,
) (nextNodes []string, sends []Send) {
	// Collect all next nodes and sends from results
	for _, nodeName := range executedNodes {
		r, ok := results[nodeName]
		if !ok {
			continue
		}

		// Collect Sends
		sends = append(sends, r.Sends...)

		// If node used Command-style routing (NodeResult.Route)
		if r.Route != "" {
			if r.Route == END {
				nextNodes = append(nextNodes, END)
			} else {
				nextNodes = append(nextNodes, r.Route)
			}
			continue
		}

		// Check conditional edge
		if condDef, hasCond := g.condEdges[nodeName]; hasCond {
			target, err := condDef.router(ctx, state)
			if err != nil {
				logw.CtxErrorf(ctx, "graphw: conditional edge router for %q failed: %v", nodeName, err)
				continue
			}

			// Apply mapping if provided
			if condDef.mapping != nil {
				if mapped, ok := condDef.mapping[target]; ok {
					target = mapped
				}
			}

			if target == END {
				nextNodes = append(nextNodes, END)
			} else {
				nextNodes = append(nextNodes, target)
			}
			continue
		}

		// Check static edges (supports fan-out)
		if tos, hasEdge := g.staticEdges[nodeName]; hasEdge {
			nextNodes = append(nextNodes, tos...)
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	deduped := make([]string, 0, len(nextNodes))
	for _, n := range nextNodes {
		if !seen[n] {
			seen[n] = true
			deduped = append(deduped, n)
		}
	}

	return deduped, sends
}

// sortResults sorts node results in deterministic order.
func (g *compiledGraph[S]) sortResults(nodes []string, results map[string]NodeResult[S]) []NodeResult[S] {
	sorted := make([]string, len(nodes))
	copy(sorted, nodes)
	sort.Strings(sorted)

	out := make([]NodeResult[S], 0, len(sorted))
	for _, name := range sorted {
		if r, ok := results[name]; ok {
			out = append(out, r)
		}
	}
	return out
}

// saveCheckpoint saves the current state to the checkpointer.
func (g *compiledGraph[S]) saveCheckpoint(ctx context.Context, threadID string, state S, next []string, step int) error {
	if g.checkpointer == nil || threadID == "" {
		return nil
	}

	stateBytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("graphw: marshal state: %w", err)
	}

	// Get previous checkpoint ID for parent chain
	parentID := ""
	if prev, err := g.checkpointer.Get(ctx, threadID, ""); err == nil && prev != nil {
		parentID = prev.ID
	}

	cp := Checkpoint{
		ID:        generateCheckpointID(),
		ThreadID:  threadID,
		ParentID:  parentID,
		State:     stateBytes,
		Next:      next,
		CreatedAt: time.Now(),
		Metadata:  map[string]any{"step": step},
	}

	return g.checkpointer.Put(ctx, cp)
}

// mergeInterruptSets merges compile-time and run-time interrupt settings.
func (g *compiledGraph[S]) mergeInterruptSets(compileSet map[string]bool, runNodes []string) map[string]bool {
	result := make(map[string]bool)
	for k, v := range compileSet {
		result[k] = v
	}
	for _, n := range runNodes {
		result[n] = true
	}
	return result
}

// --- Helper functions ---

func containsEnd(nodes []string) bool {
	for _, n := range nodes {
		if n == END {
			return true
		}
	}
	return false
}

func generateCheckpointID() string {
	return fmt.Sprintf("cp_%d", time.Now().UnixNano())
}