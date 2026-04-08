# tasks — Type-Safe DAG Execution Library

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

**Tasks must use pointer receivers for `Do`.** Since tasks write to `self.Output` during execution, a value receiver would discard the result. The framework validates this at graph construction time.

Dependencies are declared as a struct field named `Deps` containing pointers to upstream tasks. Outputs are written to a field named `Output`. Both are optional — a leaf task has no Deps, a sink task has no Output.

**The `Deps` struct must contain only pointers to types implementing `Task`.** Any other field type (e.g., `*string`, `int`, config structs) is a validation error. Use separate struct fields outside `Deps` for non-task data.

```go
type BuildOutput struct {
    ImagePath string
}

type BuildImage struct {
    Output BuildOutput
}

func (b *BuildImage) Do(ctx context.Context) error {
    b.Output = BuildOutput{ImagePath: "/img"}
    return nil
}

type DeployDeps struct {
    Build  *BuildImage
    Config *LoadConfig
}

type DeployOutput struct {
    URL string
}

type Deploy struct {
    Deps   DeployDeps
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
    Deps: DeployDeps{Build: build, Config: config},
}
```

No `Add()`, no `Connect()`, no `DependsOn()`. The struct field assignments *are* the dependency declarations.

### Execution

```go
func Execute(ctx context.Context, cfg Config, roots ...Task) error
```

`Execute` takes a context, a config, and one or more root tasks:

```go
// Single root with default config
err := tasks.Execute(ctx, tasks.Config{}, deploy)

// Multiple roots
err := tasks.Execute(ctx, tasks.Config{}, teardown1, teardown2)

// With options
err := tasks.Execute(ctx, tasks.Config{
    OnError:        tasks.CancelAll,
    MaxConcurrency: 4,
}, deploy)
```

When multiple roots are provided, all tasks across their graphs are deduplicated by pointer identity and run as a single DAG. If a root also appears as an interior node of another root's graph, it is deduplicated — not an error.

`Execute` proceeds as follows:

1. **Walks the graph** — reflects over each task's `Deps` field, follows pointers recursively to discover the full DAG. Nodes are identified by pointer identity.
2. **Validates** — checks for cycles (via topological sort on pointer-identity nodes), nil Deps pointers, and invalid Deps field types. Returns an error before running anything if the graph is invalid.
3. **Deduplicates** — the same task pointer reached via multiple paths (diamond dependency) is executed exactly once.
4. **Schedules** — runs tasks concurrently. A task starts only after all its Deps have completed successfully (or with the appropriate status per the error strategy).
5. **Outputs are available** — since Deps hold pointers to upstream tasks, `task.Deps.Upstream.Output` is directly readable inside `Do()`. The framework guarantees happens-before ordering: a task's goroutine is only launched after all upstream goroutines have completed and their results are visible (synchronized via `sync.WaitGroup` or channel).

## Configuration

```go
type Config struct {
    // OnError controls what happens when a task fails.
    // Default (zero value): CancelDependents.
    OnError ErrorStrategy

    // MaxConcurrency limits how many tasks run in parallel.
    // 0 (default): unlimited. 1: serial execution (useful for debugging).
    // Negative values are treated as 0 (unlimited).
    MaxConcurrency int
}
```

### Error Strategies

**`CancelDependents` (default / zero value):** When a task fails, all tasks that transitively depend on it are skipped (status `Skipped`). Independent branches continue running. Already-running tasks are not interrupted.

**`CancelAll`:** When any task fails, the context passed to all running and future tasks is canceled. Tasks currently in `Do()` receive cancellation via `ctx.Done()` and should return promptly (status `Canceled`). Tasks that haven't started yet also get status `Canceled`.

## Graph Discovery via Reflection

At `Execute` time, the framework:

1. For each task, checks if it has a `Deps` field of struct type.
2. Iterates over all fields in `Deps`. Each field must be a pointer to a struct that implements `Task`.
3. Follows those pointers recursively to discover the full graph.
4. Non-pointer fields in Deps, or pointers to non-Task types, are a validation error.
5. Nil pointer fields in Deps are a validation error.

The framework never touches `Output` — that's purely a user convention. Tasks write to `self.Output`, downstream tasks read `dep.Output`. The framework only cares about `Deps` pointers and the `Task` interface.

### Concurrency Safety

The framework guarantees that when `Do(ctx)` is called on a task, all upstream tasks have fully completed and their writes (including to `Output` fields) are visible. This happens-before relationship is established through Go synchronization primitives (e.g., `sync.WaitGroup.Done()` in the upstream goroutine, `sync.WaitGroup.Wait()` before launching the downstream goroutine).

Tasks must not mutate their `Deps` fields during `Do()`. Doing so is undefined behavior.

## Error Reporting

`Execute` returns a `*DAGError` containing the result of every task:

```go
type DAGError struct {
    // Results is keyed by task pointer. Since tasks must be pointers,
    // they are comparable and safe to use as map keys.
    Results map[Task]TaskResult
}

type TaskResult struct {
    Status TaskStatus
    Err    error // nil if Succeeded
}

type TaskStatus int

const (
    Succeeded TaskStatus = iota
    Failed               // Do() returned a non-nil error
    Skipped              // a dependency failed (CancelDependents mode); task was never started
    Canceled             // context was canceled (CancelAll mode); task may or may not have started
)
```

`DAGError` implements `error`. `Execute` returns `nil` if all tasks succeeded.

### Inspecting Results

```go
err := tasks.Execute(ctx, tasks.Config{}, root)
var dagErr *tasks.DAGError
if errors.As(err, &dagErr) {
    for task, result := range dagErr.Results {
        fmt.Printf("%T: %s %v\n", task, result.Status, result.Err)
    }
}
```

## Task Reuse

Each `Execute` call re-runs all tasks in the graph from scratch. The framework does **not** reset `Output` fields — it is the user's responsibility to ensure `Do()` overwrites `Output` fully. If a task can fail partway through writing `Output`, the user should write to a local variable first and assign to `Output` only on success.

## Accessing Transitive Dependencies

A task can read through its deps to access transitive outputs:

```go
func (c *CreateCluster) Do(ctx context.Context) error {
    rgName := c.Deps.Subnet.Deps.VNet.Deps.RG.Output.RGName
    return nil
}
```

This is safe — DAG ordering guarantees all transitive deps have completed. However, it creates coupling to the internal structure of transitive dependencies. Prefer declaring direct deps when practical.

## Complete Example

```go
package main

import (
    "context"
    "fmt"

    "github.com/example/tasks"
)

// --- Task definitions ---

type CreateRGOutput struct {
    RGName string
}

type CreateRG struct {
    Output CreateRGOutput
}

func (t *CreateRG) Do(ctx context.Context) error {
    t.Output.RGName = "my-rg"
    return nil
}

type CreateVNetDeps struct {
    RG *CreateRG
}

type CreateVNetOutput struct {
    VNetID string
}

type CreateVNet struct {
    Deps   CreateVNetDeps
    Output CreateVNetOutput
}

func (t *CreateVNet) Do(ctx context.Context) error {
    t.Output.VNetID = fmt.Sprintf("%s-vnet", t.Deps.RG.Output.RGName)
    return nil
}

type CreateSubnetDeps struct {
    VNet *CreateVNet
}

type CreateSubnetOutput struct {
    SubnetID string
}

type CreateSubnet struct {
    Deps   CreateSubnetDeps
    Output CreateSubnetOutput
}

func (t *CreateSubnet) Do(ctx context.Context) error {
    t.Output.SubnetID = fmt.Sprintf("%s-subnet", t.Deps.VNet.Output.VNetID)
    return nil
}

type CreateClusterDeps struct {
    RG     *CreateRG
    Subnet *CreateSubnet
}

type CreateClusterOutput struct {
    ClusterID string
}

type CreateCluster struct {
    Deps   CreateClusterDeps
    Output CreateClusterOutput
}

func (t *CreateCluster) Do(ctx context.Context) error {
    t.Output.ClusterID = fmt.Sprintf("cluster-in-%s-%s",
        t.Deps.RG.Output.RGName,
        t.Deps.Subnet.Output.SubnetID)
    return nil
}

type RunTestsDeps struct {
    Cluster *CreateCluster
}

type RunTests struct {
    Deps RunTestsDeps
}

func (t *RunTests) Do(ctx context.Context) error {
    fmt.Println("Running tests on", t.Deps.Cluster.Output.ClusterID)
    return nil
}

type TeardownDeps struct {
    RG    *CreateRG
    Tests *RunTests
}

type Teardown struct {
    Deps TeardownDeps
}

func (t *Teardown) Do(ctx context.Context) error {
    fmt.Println("Tearing down", t.Deps.RG.Output.RGName)
    return nil
}

// --- Wiring and execution ---

func main() {
    rg := &CreateRG{}
    vnet := &CreateVNet{Deps: CreateVNetDeps{RG: rg}}
    subnet := &CreateSubnet{Deps: CreateSubnetDeps{VNet: vnet}}
    cluster := &CreateCluster{Deps: CreateClusterDeps{RG: rg, Subnet: subnet}}
    tests := &RunTests{Deps: RunTestsDeps{Cluster: cluster}}
    teardown := &Teardown{Deps: TeardownDeps{RG: rg, Tests: tests}}

    // DAG (concurrent where possible):
    //
    //   CreateRG ──┬── CreateVNet ── CreateSubnet ──┐
    //              │                                 │
    //              ├──────────────── CreateCluster ──┘
    //              │                       │
    //              │                   RunTests
    //              │                       │
    //              └──────────────── Teardown

    err := tasks.Execute(context.Background(), tasks.Config{}, teardown)
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
| Cycle in dependency graph | Topological sort (pointer identity) | `"cycle detected: %T(%p) -> %T(%p) -> ..."` |
| Deps field is not a struct | Reflection | `"task %T.Deps must be a struct"` |

## What's NOT in Scope (V1)

Intentionally deferred to keep V1 minimal:

- **Retry / timeout** — implement inside `Do()`. Framework support later.
- **Conditional execution** — adds complexity. Deferred.
- **Observability hooks** — deferred. Users can wrap tasks.
- **Step naming / logging** — can use `fmt.Stringer`. Deferred.
- **WorkflowMutator pattern** — not needed. The graph is just Go structs; mutation is just Go code.
- **Output reset between re-runs** — user responsibility. Framework doesn't touch Output.
- **Linter for nil deps** — out of scope for the library, but a natural companion tool.
