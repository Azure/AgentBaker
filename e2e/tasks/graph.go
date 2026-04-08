package tasks

import (
	"fmt"
	"reflect"
)

// graph represents the discovered DAG.
type graph struct {
	// nodes is all tasks in the graph, deduplicated by pointer identity.
	nodes []Task
	// deps maps each task to its direct dependencies.
	deps map[Task][]Task
	// dependents maps each task to the tasks that depend on it.
	dependents map[Task][]Task
	// order is a valid topological sort, populated by validateNoCycles.
	// The scheduler does not use it — execution order is driven by the
	// dep-counting algorithm in runGraph. This field exists for test
	// introspection (asserting correct ordering).
	order []Task
}

// discoverGraph walks the Deps fields of the given root tasks recursively
// to build the full DAG. Tasks are deduplicated by pointer identity.
func discoverGraph(roots []Task) (*graph, error) {
	g := &graph{
		deps:       make(map[Task][]Task),
		dependents: make(map[Task][]Task),
	}
	visited := make(map[Task]bool)
	for _, root := range roots {
		if err := g.walk(root, visited); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (g *graph) walk(task Task, visited map[Task]bool) error {
	if visited[task] {
		return nil
	}
	visited[task] = true
	g.nodes = append(g.nodes, task)

	deps, err := extractDeps(task)
	if err != nil {
		return err
	}

	g.deps[task] = deps
	for _, dep := range deps {
		g.dependents[dep] = append(g.dependents[dep], task)
		if err := g.walk(dep, visited); err != nil {
			return err
		}
	}
	return nil
}

var taskType = reflect.TypeOf((*Task)(nil)).Elem()

// extractDeps reads the Deps field of a task via reflection and returns
// all dependency tasks found as pointer fields.
func extractDeps(task Task) ([]Task, error) {
	v := reflect.ValueOf(task)
	if v.Kind() != reflect.Ptr {
		return nil, &ValidationError{Task: task, Message: "task must be a pointer"}
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return nil, &ValidationError{Task: task, Message: "task must be a pointer to a struct"}
	}

	depsField := v.FieldByName("Deps")
	if !depsField.IsValid() {
		return nil, nil
	}

	if depsField.Kind() != reflect.Struct {
		return nil, &ValidationError{
			Task:    task,
			Message: "Deps field must be a struct",
		}
	}

	depsType := depsField.Type()
	var deps []Task
	for i := range depsField.NumField() {
		field := depsField.Field(i)
		fieldInfo := depsType.Field(i)

		if field.Kind() != reflect.Ptr {
			return nil, &ValidationError{
				Task:    task,
				Message: fmt.Sprintf("Deps.%s must be a pointer, got %s", fieldInfo.Name, field.Type()),
			}
		}

		if field.IsNil() {
			return nil, &ValidationError{
				Task:    task,
				Message: fmt.Sprintf("Deps.%s is nil", fieldInfo.Name),
			}
		}

		if !field.Type().Implements(taskType) {
			return nil, &ValidationError{
				Task:    task,
				Message: fmt.Sprintf("Deps.%s: %s does not implement Task", fieldInfo.Name, field.Type()),
			}
		}

		dep := field.Interface().(Task)
		deps = append(deps, dep)
	}
	return deps, nil
}
