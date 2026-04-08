package tasks

import "fmt"

// validateNoCycles checks the graph for cycles using Kahn's algorithm.
// On success, populates g.order with a valid topological sort.
func validateNoCycles(g *graph) error {
	inDegree := make(map[Task]int, len(g.nodes))
	for _, node := range g.nodes {
		inDegree[node] = len(g.deps[node])
	}

	var queue []Task
	for _, node := range g.nodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	var sorted []Task
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)

		for _, dependent := range g.dependents[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(sorted) != len(g.nodes) {
		var cycleNodes []string
		for _, node := range g.nodes {
			if inDegree[node] > 0 {
				cycleNodes = append(cycleNodes, fmt.Sprintf("%T(%p)", node, node))
			}
		}
		return &ValidationError{
			Message: fmt.Sprintf("cycle detected among tasks: %v", cycleNodes),
		}
	}

	g.order = sorted
	return nil
}
