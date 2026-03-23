# AgentBaker E2E vs AKS-RP E2Ev3 — Reference Guide

## TL;DR

- **AgentBaker E2E** tests **node bootstrapping** — can a single VM successfully join an existing AKS cluster using the CSE/cloud-init that AgentBaker generates?
- **AKS-RP E2Ev3** tests the **AKS Resource Provider** — does the RP correctly handle full cluster lifecycle (create, upgrade, scale, stop/start, delete)?

They sit at different layers of the stack:

```
┌──────────────────────────────────────────────┐
│  AKS-RP E2Ev3                                │  "Did the RP create/manage
│  ARM API · Cluster lifecycle · HCP/CCP       │   a cluster correctly?"
└──────────────────┬───────────────────────────┘
                   │ internally invokes
                   ▼
┌──────────────────────────────────────────────┐
│  AgentBaker E2E                              │  "Did AgentBaker produce the right
│  CSE · cloud-init · node bootstrap           │   bootstrap config, and does the
│  OS-level validation · kubelet join          │   VM come up correctly?"
└──────────────────────────────────────────────┘
```

---

## Where the VM Joins the Cluster (AgentBaker E2E)

The complete code path from test entry to a VM successfully joining the cluster:

### Call Chain

```
RunScenario            (test_helpers.go)
 └─ runScenario        (test_helpers.go)
     ├─ s.Config.Cluster(...)              → prepareCluster (cluster.go)
     ├─ prepareAKSNode                     (test_helpers.go)
     │   ├─ getBaseNBC                     (node_config.go)       ← build bootstrap config from real cluster
     │   ├─ nbcToAKSNodeConfigV1           (node_config.go)       ← [scriptless path only]
     │   ├─ ConfigureAndCreateVMSS         (vmss.go)
     │   │   └─ CreateVMSSWithRetry        (vmss.go)
     │   │       └─ CreateVMSS             (vmss.go)
     │   │           ├─ createVMSSModel    (vmss.go)              ← CSE + CustomData attached
     │   │           │   ├─ getBaseVMSSModel (vmss.go)            ← CSE extension wired here
     │   │           │   └─ s.PrepareVMSSModel (types.go)
     │   │           ├─ BeginCreateOrUpdate (Azure ARM call)      ← VMSS created, CSE executes
     │   │           ├─ waitForVMSSVM      (vmss.go)              ← poll until VM instance appears
     │   │           ├─ PollUntilDone      (vmss.go)              ← wait for CSE to finish
     │   │           └─ waitForVMRunningState (vmss.go)           ← confirm PowerState/running
     │   ├─ getCustomScriptExtensionStatus (test_helpers.go)      ← verify CSE exit code = 0
     │   └─ Kube.WaitUntilNodeReady        (kube.go)             ← watch k8s API until NodeReady=True
     └─ validateVM                         (test_helpers.go)
         ├─ validateSSHConnectivity        (test_helpers.go)
         ├─ ValidateNodeCanRunAPod         (test_helpers.go)
         └─ ValidateCommonLinux/Windows    (validation.go)
```

### Key Files and What They Do

| File | Key Functions | Role |
|------|--------------|------|
| `test_helpers.go` | `RunScenario`, `runScenario`, `prepareAKSNode`, `validateVM`, `getCustomScriptExtensionStatus` | Orchestrates the full test lifecycle |
| `vmss.go` | `ConfigureAndCreateVMSS`, `CreateVMSS`, `createVMSSModel`, `getBaseVMSSModel` | Builds the ARM VMSS model with CSE extension and calls Azure to create it |
| `node_config.go` | `getBaseNBC`, `baseTemplateLinux`, `nbcToAKSNodeConfigV1` | Generates the NodeBootstrappingConfiguration from real cluster params |
| `cluster.go` | `prepareCluster` | Creates/reuses an AKS cluster, sets up bastion, extracts bootstrap token + CA cert |
| `kube.go` | `WaitUntilNodeReady` | Watches the Kubernetes API for the new node to reach `Ready` status |
| `validation.go` | `ValidateCommonLinux`, `ValidateCommonWindows` | Runs ~20+ validators (TLS, DNS, systemd, iptables, kubelet, etc.) |
| `validators.go` | ~80 individual validator functions | Deep OS-level checks via SSH (files, services, network config, GPU, etc.) |
| `types.go` | `Scenario`, `Config`, `ScenarioRuntime`, `ScenarioVM` | Core type definitions |

### The Two Bootstrap Paths

| | Legacy (NBC/bash CSE) | Scriptless (aks-node-controller) |
|---|---|---|
| **Mutator field** | `BootstrapConfigMutator` | `AKSNodeConfigMutator` |
| **Config type** | `*datamodel.NodeBootstrappingConfiguration` | `*aksnodeconfigv1.Configuration` |
| **CSE command** | `ab.GetNodeBootstrapping()` → bash script | `nodeconfigutils.CSE` (constant) |
| **Custom data** | Gzipped cloud-init with all scripts embedded | cloud-init that downloads + runs compiled `aks-node-controller` binary |

---

## Side-by-Side Comparison

### Scope & Purpose

| Dimension | AgentBaker E2E | AKS-RP E2Ev3 |
|---|---|---|
| **What is tested** | AgentBaker bootstrapping service (CSE/CustomData for a single VM) | AKS Resource Provider (cluster lifecycle, ARM API, HCP, CCP) |
| **Infrastructure per test** | 1 VMSS with 1 VM, joined to a pre-existing shared cluster | Full AKS managed cluster (resource group, VNet, VMSS node pools) |
| **RP interaction** | Minimal — calls `AgentBaker.GetNodeBootstrapping()` directly | Full — exercises the real AKS ARM API |
| **Validation method** | SSH into VM → run bash commands → check OS-level state | ARM API responses → K8s API state → power state → error codes |

### Framework & Execution

| Dimension | AgentBaker E2E | AKS-RP E2Ev3 |
|---|---|---|
| **Framework** | Vanilla `go test` + `testify` | Custom DAG framework on `go-workflow` (Plans/Scenarios/Mutators/Suites) |
| **Parallelism** | `go test -parallel 100` — tests share clusters, each gets own VMSS | Each scenario runs as a separate Pod on an e2e-underlay cluster |
| **Local run** | `./e2e-local.sh` | `./hack/e2ev3 mv --build-version <ver>` |
| **Filtering** | Env vars: `TAGS_TO_RUN=gpu=true` | CLI: `--scenario Scenario_Preflight` |
| **Cluster reuse** | Clusters are cached and reused across tests | Each scenario creates a fresh cluster |

### Scenario Coverage

| Category | AgentBaker E2E | AKS-RP E2Ev3 |
|---|---|---|
| **OS flavors** | Ubuntu, AzureLinux, Flatcar, Mariner, Windows 2022/23H2/2025 | Same + sovereign cloud variants |
| **GPU (deep)** | NVIDIA SMI, MIG, DCGM, device plugin, NPD GPU checks | Not a focus |
| **Node bootstrap paths** | CSE (traditional) vs Scriptless (aks-node-controller) | Indirectly via `node/nativenode` SIG |
| **DNS/LocalDNS** | Deep — service, hosts plugin, resolution checks | Via CCP localdns SIG |
| **Cluster CRUD/lifecycle** | Not tested | Core focus |
| **Networking (CNI/overlay)** | Only node-level iptables verification | Deep — CNI overlay, SwiftV1/V2, Cilium, ALB, private clusters |
| **Security features** | Node-level only (SSH, TLS bootstrap, leaked secrets) | Deep — KMS, AAD, workload identity, egress lockdown, image integrity |
| **RP API correctness** | Not in scope | Core — preflight, ETag, abort, RBAC, NSP |
| **HCP/CCP internals** | Not in scope | HCP gRPC APIs, CCP namespace lifecycle, billing |
| **Autoscaling** | Not in scope | Cluster autoscaler, Karpenter |

### Validation Depth

**AgentBaker E2E** validates at the OS level (~80 validators):
- TLS/Auth: kubelet cert rotation, leaked secrets, SSH pubkey disabled
- DNS: LocalDNS service, resolution, hosts plugin
- Networking: iptables rules, IMDS restriction, NIC buffer sizes
- OS config: sysctl, ulimits, AppArmor, systemd units
- GPU: NVIDIA drivers, MIG mode, DCGM exporter, NPD GPU checks
- Files: content checks, permissions, existence
- Windows: services, DLLs, Cilium, containerd

**AKS-RP E2Ev3** validates at the service level:
- ARM API responses (HTTP status, LRO state, ETag headers)
- Cluster provisioning/power state transitions
- Agent pool operations (create/scale/upgrade/delete)
- Kubernetes API object state (deployments, pods, nodes)
- HCP gRPC API correctness
- CCP namespace lifecycle

---

## Summary

They are **complementary**, not competing:

- **AgentBaker E2E** = bottom of the stack. "Given an existing cluster, does the bootstrap config make a VM join correctly?" Validates by SSHing into the VM.
- **AKS-RP E2Ev3** = top of the stack. "Does the RP correctly orchestrate cluster lifecycle?" Validates via ARM API and K8s API. Implicitly relies on AgentBaker working (since cluster creation triggers node bootstrapping), but doesn't validate node-level details.
