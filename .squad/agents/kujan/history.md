# Kujan — Session History

## Initial Context (seeded)

### AgentBaker Testing Architecture
- **Go tests**: Vanilla `go test` framework across `pkg/`, `apiserver/`, `vhdbuilder/`. No external test frameworks.
- **Snapshot tests**: Generated test data in `pkg/agent/` — CSE and CustomData golden files. Regenerated via `make generate` and `make generate-testdata`.
- **ShellSpec**: Shell script unit tests in `spec/parts/linux/cloud-init/artifacts/`. Covers CSE helpers, retry logic, config parsing, install scripts (Ubuntu, Mariner, Flatcar). Run via `make shellspec` (Docker-based).
- **E2e tests**: Full scenario tests in `e2e/` against real Azure infrastructure. Separate scenarios for Linux (`scenario_test.go`), Windows (`scenario_win_test.go`), GPU (`scenario_gpu_*.go`). Validators in `validators.go`.

### Key Testing Patterns
- ShellSpec tests use mocks for IMDS and kubelet responses (`imds_mocks/`, `kubelet_mocks/`)
- `retrycmd_if_failure` uses `timeout <val> "$@"` — commands must be external executables; shell builtins need `bash -c` wrapping
- NPD's `check_fs_corruption.sh` checks `journalctl -u containerd` for "structure needs cleaning"
- `ValidateGPUWorkloadSchedulable` takes a `resourceName` arg for MIG resource targeting
- E2e `collectGarbageVMSS` must clean stale K8s Node objects to prevent FailedToCreateRoute loops

### Critical Workflow
1. Modify code in `parts/` or `pkg/`
2. Run `make generate` to regenerate snapshot test data
3. Run `make test` to verify Go unit tests pass
4. Run `make shellspec` for shell script tests
5. E2e tests run in CI against real Azure infrastructure

---
*No sessions yet.*
