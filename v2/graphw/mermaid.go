package graphw

import (
	"fmt"
	"strings"
)

// ExportMermaid generates a Mermaid flowchart diagram from a compiled graph.
// The graph must be a *compiledGraph[S]; this function panics if given
// a different Graph implementation.
//
// Example output:
//
//	graph TD
//	    __start__ --> think
//	    think -->|route_decision| act
//	    act --> think
//	    think --> __end__
func ExportMermaid[S any](g Graph[S]) string {
	cg, ok := g.(*compiledGraph[S])
	if !ok {
		panic("graphw: ExportMermaid requires a *compiledGraph")
	}

	var b strings.Builder
	b.WriteString("graph TD\n")

	// Static edges
	for from, tos := range cg.staticEdges {
		fromLabel := mermaidLabel(from)
		for _, to := range tos {
			toLabel := mermaidLabel(to)
			b.WriteString(fmt.Sprintf("    %s --> %s\n", fromLabel, toLabel))
		}
	}

	// Conditional edges
	for from, condDef := range cg.condEdges {
		fromLabel := mermaidLabel(from)
		if condDef.mapping != nil {
			for key, target := range condDef.mapping {
				targetLabel := mermaidLabel(target)
				b.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", fromLabel, key, targetLabel))
			}
		} else {
			// No mapping — router returns node names directly
			b.WriteString(fmt.Sprintf("    %s -->|conditional| {%s}\n", fromLabel, fromLabel))
		}
	}

	return b.String()
}

// mermaidLabel sanitizes a node name for Mermaid diagram syntax.
func mermaidLabel(name string) string {
	if name == START {
		return "START([__start__])"
	}
	if name == END {
		return "END(((__end__))"
	}
	return name
}