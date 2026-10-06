# Custom user data reference POC

This branch is a reference implementation, **not a deployable AKS preview**.
It does not claim phase-3 rollout, full cloud-init input parity, public API
approval, safe arbitrary live updates, or real-VM validation.

## Read the code in this order

1. [Versioned profile](../custom-node-config/profile.go): content revision,
   customer boot script, kubelet JSON/flags, containerd TOML overlay, OS sysctls.
2. [Boot transport](../aks-node-controller/pkg/nodeconfigutils/utils.go):
   `CustomDataWithUserData` embeds a pinned profile in the managed boothook
   **before ANC starts**. Customer scripts are staged, not executed by a
   competing cloud-init phase.
3. [Finalization hook](../parts/linux/cloud-init/artifacts/cse_config_kubelet.sh):
   the opt-in file check runs after managed kubelet writers, before
   daemon-reload/first kubelet start. This hook is in `nodePrep`, so PIS cached
   images do not skip it and shared-image `basePrep` does not execute customers.
4. [ANC commands](../aks-node-controller/customuserdata.go):
   `apply-custom-user-data` applies the staged profile; errors fail the existing
   kubelet/CSE path. `reconcile-custom-user-data --node-name=...` is an
   **explicitly invoked, one-shot** runtime loop using kubelet credentials.
5. [Shared executor](../custom-node-config/executor.go): customer precedence,
   containerd validation/restart/CRI probe, sysctl read-back, atomic file writes,
   serialized local execution, script journal, and compensating rollback.
6. [Runtime dispatcher](../custom-node-config/runtime.go): exact hash guard,
   per-pool selection and Option C-compatible results. Unknown handlers are
   reported `Unsupported`, not silently declared successful.
7. [Tests](../custom-node-config/executor_test.go): injected host operations and
   a temporary node filesystem. They do not execute scripts or change the host.

## POC contract and intentional restrictions

`apiVersion` is `aks.custom-node-config/v1alpha1`; `revision` is SHA-256 of the
Go JSON encoding of `spec`. Construct profiles with `NewProfile`, rather than
manually calculating a digest over formatted input.

- Boot scripts must begin `#!/bin/bash` followed by a newline. They run as root,
  with bash fail-fast, a 120-second limit and discarded output to avoid leaking
  customer secrets into CSE. This is not a script sandbox. Children in the
  script's process group are terminated on timeout; deliberately detached
  processes are not contained.
- JSON kubelet overrides replace top-level fields; nested objects/arrays are
  replaced, not recursively patched. Known conflicting legacy flags are removed.
  Only the two image-GC flags are admitted as explicit CLI overrides.
- Managed kubelet JSON/defaults must already exist. The RP helper enables the
  config-file path only when AKS supplies a complete managed baseline.
- Containerd overlays recursively merge TOML plugin tables. This POC admits
  config version 2, existing plugins, and only `image_pull_with_sync_fs` and
  `image_pull_progress_timeout`. Version 3/GPU/schema behavior needs separate
  qualification. The process may already be running; boot finalization can
  restart it before kubelet joins.
- Live mode admits only restoring `net.ipv4.tcp_retries2=15`, as an illustrative
  constrained handler. It does **not** assert that arbitrary network changes are
  nondisruptive. Kubelet/containerd changes fail with a replacement/disruption
  orchestration requirement.
- Mutable Ubuntu/Azure Linux only. Variants and other OS identities are rejected
  by the boot command. Production image capability admission is still needed.
- Limits are POC assumptions: 32 KiB encoded profile JSON, 16 KiB script,
  64 KiB composed decoded custom data, 256 KiB runtime document, 1 MiB minimum
  free disk before execution, five-minute boot command/two-minute runtime tick.
  These are not approved AKS/Compute service limits.
- There is no format-compatible support for arbitrary cloud-config, MIME user
  input, payload clearing, drift-free removal of formerly configured fields,
  or general flag precedence yet. Do not market this as full EKS parity.

## Run locally

Use the cross-repository workspace procedure in the AKS RP POC guide. The new
`custom-node-config` module is deliberately unpublished (`v0.0.0`); local
workspace version-specific replacements are required. No absolute machine
paths or local module replacements are committed into dependency manifests.

Run the shared module tests, nodeconfigutils tests excluding the historical
clone compatibility harness, ANC command tests and the RP demo.
Historical compatibility tests spawn Go in separate Git clones; an inherited
workspace excludes those clones. Validate them separately with an appropriate
standalone dependency setup; do not interpret that workspace failure as a
protobuf wire-compatibility result.

### Verification performed

- Shared executor and RP producer tests passed with Go's race detector.
- The existing nodeconfigutils suite, including historical forward/backward
  compatibility tests, passed using an external modfile and `GOWORK=off`.
- ANC POC command tests passed; the ANC binary/package compiled.
- LPC datamodel/genericconfig suites and targeted `go vet` passed.
- Modified bootstrap shell passed `bash -n` and error-level ShellCheck.
- The local demo applied boot intent for `tcp_retries2=8`, then reconciled a new
  live revision restoring `15`, reporting the correct feature revision.
- No real node, host service/sysctl, or Azure resource was changed. Actual node
  bootstrap and runtime safety have not been demonstrated by these tests.

## Production work still required

- Ship the binary, hook and capability marker together in a validated VHD;
  publishing the new JSON to an old image is unsupported.
- Prove hook ordering/effective values on real VMSS/VM nodes, PIS, repair,
  reimage, GPU/runtime migration and supported Kubernetes/containerd versions.
- Replace POC format/limits/protection policies with reviewed contracts.
- Validate upstream kubelet fields and all flags semantically/version-wise.
- Add durable crash recovery: local compensation is not an atomic multi-service
  transaction, and failure during a filesystem mutation can require node repair.
  A successful script with external effects can still run again after a crash
  before its journal is persisted; customers must make scripts idempotent.
- Add lifecycle-wide fencing/locking, safe rollout grants, maintenance/PDB
  integration, last-known-good recovery and post-kubelet effective-state checks.
- Integrate the runtime handler with the **single generic status writer**. This
  one-shot command owns the complete POC status document and must not run beside
  another writer. No timer is automatically installed/enabled.
- Implement support markers, diagnostics and approved user-data troubleshooting.
