package graphw

import (
	"fmt"
	"sort"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
)

// Builder constructs a compiled Graph via a fluent API.
// Use NewBuilder to create a builder, then call AddNode, AddEdge, and
// AddConditionalEdge to define the graph, and finally Compile to produce
// an executable Graph.
//
// Example:
//
//	type MyState struct {
//	    Messages []string
//	    Count    int
//	}
//
//	graph, err := graphw.NewBuilder[MyState](myReducer).
//	    AddNode("think", thinkNode).
//	    AddNode("act", actNode).
//	    AddEdge(graphw.START, "think").
//	    AddConditionalEdge("think", routeThink, nil).
//	    AddEdge("act", "think").
//	    Compile()
type Builder[S any] struct {
	reducer      Reducer[S]
	nodes        map[string]nodeDef[S]
	staticEdges  map[string][]string // from -> list of to nodes (fan-out)
	condEdges    map[string]condEdgeDef[S]
	inputKeys    []string
	outputKeys   []string
	nodeOrder    []string // preserves insertion order for determinism
}

type nodeDef[S any] struct {
	fn         NodeFunc[S]
	retry      int
	retryDelay time.Duration
}

type condEdgeDef[S any] struct {
	router  RouterFunc[S]
	mapping map[string]string // router return value -> node name (optional)
}

// NewBuilder creates a new graph builder with the given state reducer.
//
// The reducer defines how node outputs merge with the current state.
// For example:
//
//	func myReducer(current, update MyState) MyState {
//	    current.Messages = append(current.Messages, update.Messages...)
//	    current.Count += update.Count
//	    return current
//	}
func NewBuilder[S any](reducer Reducer[S]) *Builder[S] {
	return &Builder[S]{
		reducer:     reducer,
		nodes:       make(map[string]nodeDef[S]),
		staticEdges: make(map[string][]string),
		condEdges:   make(map[string]condEdgeDef[S]),
	}
}

// AddNode registers a node with the given name and function.
// Node names must be unique. The special names START and END are reserved.
func (b *Builder[S]) AddNode(name string, fn NodeFunc[S], opts ...NodeOption) *Builder[S] {
	if name == START || name == END {
		panic(fmt.Sprintf("graphw: cannot use reserved name %q for a node", name))
	}

	cfg := &nodeConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	if _, exists := b.nodes[name]; exists {
		panic(fmt.Sprintf("graphw: duplicate node name %q", name))
	}

	b.nodes[name] = nodeDef[S]{
		fn:         fn,
		retry:      cfg.retry,
		retryDelay: cfg.retryDelay,
	}
	b.nodeOrder = append(b.nodeOrder, name)
	return b
}

// AddEdge adds a static (unconditional) edge from one node to another.
// Use START as the from node to define the entry point.
// Use END as the to node to indicate the graph should terminate.
// Multiple edges from the same source node create parallel (fan-out) execution.
func (b *Builder[S]) AddEdge(from, to string) *Builder[S] {
	if from == END {
		panic("graphw: cannot add edge from END")
	}
	if to == START {
		panic("graphw: cannot add edge to START")
	}

	b.staticEdges[from] = append(b.staticEdges[from], to)
	return b
}

// AddConditionalEdge adds a conditional edge from a node.
// The router function is called with the current state and must return
// the name of the next node. If mapping is provided, the router's
// return value is looked up in the mapping to find the actual node name.
//
// Example with mapping:
//
//	builder.AddConditionalEdge("agent", routeDecision, map[string]string{
//	    "use_tool": "tool",
//	    "done":      graphw.END,
//	})
func (b *Builder[S]) AddConditionalEdge(from string, router RouterFunc[S], mapping map[string]string) *Builder[S] {
	if from == END {
		panic("graphw: cannot add conditional edge from END")
	}

	b.condEdges[from] = condEdgeDef[S]{
		router:  router,
		mapping: mapping,
	}
	return b
}

// SetInputSchema restricts which state keys are accepted as input.
// If set, only the specified keys are used from the initial state.
func (b *Builder[S]) SetInputSchema(keys ...string) *Builder[S] {
	b.inputKeys = keys
	return b
}

// SetOutputSchema restricts which state keys are returned as output.
// If set, only the specified keys are included in the final state.
func (b *Builder[S]) SetOutputSchema(keys ...string) *Builder[S] {
	b.outputKeys = keys
	return b
}

// Compile validates the graph and produces an executable Graph.
// Returns an error if the graph is invalid (e.g., no edge from START,
// referenced nodes don't exist, etc.).
func (b *Builder[S]) Compile(opts ...CompileOption) (Graph[S], error) {
	cfg := &compileConfig{
		recursionLimit: 25, // default
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Validation: must have at least one node
	if len(b.nodes) == 0 {
		return nil, ErrNoNodes
	}

	// Validation: must have an edge from START
	if _, hasStart := b.staticEdges[START]; !hasStart {
		if _, hasCondStart := b.condEdges[START]; !hasCondStart {
			return nil, ErrNoStartEdge
		}
	}

	// Validation: all referenced nodes in edges must exist
	for from, tos := range b.staticEdges {
		if from != START {
			if _, exists := b.nodes[from]; !exists {
				return nil, fmt.Errorf("%w: node %q referenced in edge does not exist", ErrNodeNotFound, from)
			}
		}
		for _, to := range tos {
			if to != END {
				if _, exists := b.nodes[to]; !exists {
					return nil, fmt.Errorf("%w: node %q referenced in edge does not exist", ErrNodeNotFound, to)
				}
			}
		}
	}

	// Validation: conditional edge mapping values must reference existing nodes or END
	for from, condDef := range b.condEdges {
		if from != START {
			if _, exists := b.nodes[from]; !exists {
				return nil, fmt.Errorf("%w: node %q referenced in conditional edge does not exist", ErrNodeNotFound, from)
			}
		}
		for key, target := range condDef.mapping {
			if target != END {
				if _, exists := b.nodes[target]; !exists {
					return nil, fmt.Errorf("%w: node %q referenced in conditional edge mapping does not exist", ErrNodeNotFound, target)
				}
			}
			_ = key // mapping key is just a lookup
		}
	}

	// Build interrupt sets
	intBefore := make(map[string]bool)
	for _, n := range cfg.interruptBefore {
		intBefore[n] = true
	}
	intAfter := make(map[string]bool)
	for _, n := range cfg.interruptAfter {
		intAfter[n] = true
	}

	g := &compiledGraph[S]{
		reducer:      b.reducer,
		nodes:        b.nodes,
		staticEdges:  b.staticEdges,
		condEdges:    b.condEdges,
		checkpointer: cfg.checkpointer,
		intBefore:    intBefore,
		intAfter:     intAfter,
		recLimit:     cfg.recursionLimit,
		inputKeys:    b.inputKeys,
		outputKeys:   b.outputKeys,
		nodeOrder:    b.sortedNodeOrder(),
	}

	logw.Infof("graphw: compiled graph with %d nodes", len(b.nodes))

	return g, nil
}

// sortedNodeOrder returns node names in sorted order for deterministic execution.
func (b *Builder[S]) sortedNodeOrder() []string {
	order := make([]string, len(b.nodeOrder))
	copy(order, b.nodeOrder)
	sort.Strings(order)
	return order
}