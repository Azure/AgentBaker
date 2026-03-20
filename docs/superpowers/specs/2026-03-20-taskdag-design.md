# taskflow — Type-Safe DAG Execution Library

## Problem

`go-workflow` wires task dependencies through untyped closures. Dependencies are declared separately from the data they carry (`DependsOn` + `Input` callbacks), making it easy to forget one or the other. The result: nil-pointer panics at runtime, logic that's hard to follow, and no compile-time safety on data flow between tasks.

## Goal

A general-purpose Go library for defining and executing tasks as a DAG with:

1. **Type-safe dependencies** — each task declares its upstream tasks as typed struct fields. The compiler enforces field types; the framework enforces that they're wired.
2. **Concurrent execution** — independent tasks run in parallel automatically.
3. **Simplicity** — no wrapper types, no magic registration functions, no hidden state. Tasks are plain Go structs.

## Core Design

### Task Contract

A task is any struct that implements:

```go
type Task interface {
    Do(ctx context.Context) error
}
```

Dependencies are declared as a struct field named `Deps` containing pointers to upstream tasks. Outputs are written to a field named `Output`. Both are optional — a leaf task has no Deps, a sink task has no Output.

```go
type BuildImage struct {
    Output BuildOutput
}

func (b *BuildImage) Do(ctx context.Context) error {
    b.Output = BuildOutput{ImagePath: "/img"}
    return nil
}

type Deploy struct {
    Deps struct {
        Build  *BuildImage
        Config *LoadConfig
    }
    Output DeployOutput
}

func (d *Deploy) Do(ctx context.Context) error {
    d.Output = DeployOutput{
        URL: fmt.Sprintf("%s:%d", d.Deps.Build.Output.ImagePath, d.Deps.Config.Output.Port),
    }
    return nil
}
```

### Wiring

The DAG is expressed through plain Go struct initialization:

```go
build := &BuildImage{}
config := &LoadConfig{}
deploy := &Deploy{
    Deps: struct{ Build *BuildImage; Config *LoadConfig }{
        Build: build, Config: config,
    },
}
```

No `Add()`, no `Connect()`, no `DependsOn()`. The struct field assignments *are* the dependency declarations.

### Execution

```go
err := taskflow.Execute(ctx, deploy)
```

`Execute` takes a root task and:

1. **Walks the graph** — reflects over each task's `Deps` field, follows pointers recursively to discover the full DAG.
2. **Validates** — checks for cycles and nil Deps pointers. Returns an error before running anything if the graph is invalid.
3. **Deduplicates** — the same task pointer reached via multiple paths (diamond dependency) is executed exactly once.
4. **Schedules** — runs tasks concurrently. A task starts only after all its Deps have completed successfully.
5. **Populates outputs** — since Deps hold pointers to upstream tasks, `task.Deps.Upstream.Output` is directly readable after the upstream completes. No copying needed — it's just Go pointer dereferencing.

### Multiple Roots

```go
err := taskflow.Execute(ctx, teardown1, teardown2)
```

All tasks across both graphs are deduplicated and run as a single DAG.

## Configuration

```go
err := taskflow.Execute(ctx, root, taskflow.Config{
    OnError:        taskflow.CancelAll,
    MaxConcurrency: 4,
})
```

### Config Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `OnError` | `ErrorStrategy` | `CancelDependents` | What happens when a task fails |
| `MaxConcurrency` | `int` | `0` (unlimited) | Max number of tasks running in parallel |

### Error Strategies

**`CancelDependents` (default):** When a task fails, all tasks that transitively depend on it are skipped. Independent branches continue running.

**`CancelAll`:** When any task fails, the context passed to all running tasks is canceled. Fail-fast.

## Graph Discovery via Reflection

At `Execute` time, the framework:

1. For each task, checks if it has a `Deps` field of struct type.
2. Iterates over all fields in `Deps`. Each field must be a pointer to a struct that implements `Task`.
3. Follows those pointers recursively to discover the full graph.
4. Non-pointer fields in Deps, or pointers to non-Task types, are a validation error.
5. Nil pointer fields in Deps are a validation error.

The framework never touches `Output` — that's purely a user convention. Tasks write to `self.Output`, downstream tasks read `dep.Output`. The framework only cares about `Deps` pointers and the `Task` interface.

## Error Reporting

`Execute` returns a `DAGError` containing the result of every task:

```go
type DAGError struct {
    Results map[Task]TaskResult
}

type TaskResult struct {
    Status TaskStatus // Succeeded, Failed, Skipped, Canceled
    Err    error      // nil if Succeeded
}

type TaskStatus int

const (
    Succeeded TaskStatus = iota
    Failed
    Skipped   // dependency failed, this task was not run
    Canceled  // context was canceled while running
)
```

`DAGError` implements `error`. `Execute` returns `nil` if all tasks succeeded.

## Task Reuse

Each `Execute` call re-runs all tasks in the graph from scratch. Previous `Output` values are overwritten.

## Accessing Transitive Dependencies

A task can read through its deps to access transitive outputs:

```go
func (c *CreateCluster) Do(ctx context.Context) error {
    rgName := c.Deps.Subnet.Deps.VNet.Deps.RG.Output.RGName
    return nil
}
```

Safe — DAG ordering guarantees all transitive deps have completed.

## Complete Example

```go
package main

import (
    "context"
    "fmt"

    "github.com/example/taskflow"
)

// --- Task definitions ---

type CreateRG struct {
    Output struct{ RGName string }
}

func (t *CreateRG) Do(ctx context.Context) error {
    t.Output.RGName = "my-rg"
    return nil
}

type CreateVNet struct {
    Deps struct {
        RG *CreateRG
    }
    Output struct{ VNetID string }
}

func (t *CreateVNet) Do(ctx context.Context) error {
    t.Output.VNetID = fmt.Sprintf("%s-vnet", t.Deps.RG.Output.RGName)
    return nil
}

type CreateSubnet struct {
    Deps struct {
        VNet *CreateVNet
    }
    Output struct{ SubnetID string }
}

func (t *CreateSubnet) Do(ctx context.Context) error {
    t.Output.SubnetID = fmt.Sprintf("%s-subnet", t.Deps.VNet.Output.VNetID)
    return nil
}

type CreateCluster struct {
    Deps struct {
        RG     *CreateRG
        Subnet *CreateSubnet
    }
    Output struct{ ClusterID string }
}

func (t *CreateCluster) Do(ctx context.Context) error {
    t.Output.ClusterID = fmt.Sprintf("cluster-in-%s-%s",
        t.Deps.RG.Output.RGName,
        t.Deps.Subnet.Output.SubnetID)
    return nil
}

type RunTests struct {
    Deps struct {
        Cluster *CreateCluster
    }
}

func (t *RunTests) Do(ctx context.Context) error {
    fmt.Println("Running tests on", t.Deps.Cluster.Output.ClusterID)
    return nil
}

type Teardown struct {
    Deps struct {
        RG    *CreateRG
        Tests *RunTests
    }
}

func (t *Teardown) Do(ctx context.Context) error {
    fmt.Println("Tearing down", t.Deps.RG.Output.RGName)
    return nil
}

// --- Wiring and execution ---

func main() {
    rg := &CreateRG{}
    vnet := &CreateVNet{Deps: struct{ RG *CreateRG }{RG: rg}}
    subnet := &CreateSubnet{Deps: struct{ VNet *CreateVNet }{VNet: vnet}}
    cluster := &CreateCluster{Deps: struct {
        RG     *CreateRG
        Subnet *CreateSubnet
    }{RG: rg, Subnet: subnet}}
    tests := &RunTests{Deps: struct{ Cluster *CreateCluster }{Cluster: cluster}}
    teardown := &Teardown{Deps: struct {
        RG    *CreateRG
        Tests *RunTests
    }{RG: rg, Tests: tests}}

    // DAG (concurrent where possible):
    //
    //   CreateRG ──┬── CreateVNet ── CreateSubnet ──┐
    //              │                                 │
    //              └──────────────── CreateCluster ──┘
    //                                     │
    //                                 RunTests
    //                                     │
    //                                 Teardown

    err := taskflow.Execute(context.Background(), teardown)
    if err != nil {
        panic(err)
    }
}
```

## Validation Rules (enforced at Execute time)

| Rule | Detection | Error |
|------|-----------|-------|
| Nil pointer in Deps | Reflection | `"task %T has nil dependency field %s"` |
| Deps field is not a pointer to Task | Reflection | `"task %T.Deps.%s: %T does not implement Task"` |
| Cycle in dependency graph | Topological sort | `"cycle detected: A -> B -> A"` |
| Deps field is not a struct | Reflection | `"task %T.Deps must be a struct"` |

## What's NOT in Scope (V1)

Intentionally deferred to keep V1 minimal:

- **Retry / timeout** — implement inside `Do()`. Framework support later.
- **Conditional execution** — adds complexity. Deferred.
- **Observability hooks** — deferred. Users can wrap tasks.
- **Step naming / logging** — can use `fmt.Stringer`. Deferred.
- **WorkflowMutator pattern** — not needed. The graph is just Go structs; mutation is just Go code.

## Package Name Candidates

- `taskflow` — descriptive, flows well
- `tasks` — minimal, Go-idiomatic
- `rundag` — action-oriented
- `orchid` — short for orchestration, catchy

Open to your preference.
