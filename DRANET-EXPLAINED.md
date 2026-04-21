# DRANET on AKS: A Practical Guide

## What Problem Does DRANET Solve?

Some Azure VM sizes come with special high-speed networking hardware called **RDMA (Remote Direct Memory Access)** NICs. These are used by AI/ML workloads (like distributed GPU training) to move data between machines at extremely high speeds — bypassing the normal network stack entirely.

The problem: Kubernetes has no built-in way to discover, manage, or assign these RDMA NICs to pods. Without DRANET, users have to manually configure privileged pods, mount host devices, and hope they don't conflict with other workloads on the same node.

**DRANET** is an AKS-managed component that automatically discovers RDMA hardware on a node and makes it available to pods through standard Kubernetes APIs.

---

## How Does It Work? (The Full Flow)

### Step 1: Cluster Creation — Enabling Managed RDMA

When you create or update an AKS cluster, you can enable managed RDMA on a node pool:

```json
{
  "agentPoolProfiles": [{
    "vmSize": "Standard_ND96isr_H100_v5",
    "NvidiaRDMAProfile": {
      "managementMode": "Managed"
    }
  }]
}
```

- **Kubernetes 1.35**: You must explicitly set `"Managed"` (opt-in).
- **Kubernetes 1.36+**: `"Managed"` will be the default for RDMA-capable SKUs.

Only certain VM sizes have RDMA hardware. The main ones are the large GPU VMs:

| SKU Family | Example | GPU |
|---|---|---|
| ND96asr v4 | `Standard_ND96asr_v4` | 8x A100 |
| ND*s H100 v5 | `Standard_ND96isr_H100_v5` | 8x H100 |
| ND*s H200 v5 | `Standard_ND96isr_H200_v5` | 8x H200 |

### Step 2: Node Labeling — Marking RDMA-Capable Nodes

When AKS provisions nodes from an RDMA-capable SKU with managed RDMA enabled, it applies a special label:

```
kubernetes.azure.com/network-dra=rdma
```

This label is how DRANET knows which nodes to run on. It's applied during node bootstrap (the scripts that run when a VM first joins the cluster).

### Step 3: DRANET DaemonSet Deployment

DRANET is deployed as a **DaemonSet** — a Kubernetes workload type that runs one pod per matching node automatically.

```
┌─────────────────────────────────────────────┐
│  AKS Cluster                                │
│                                             │
│   Node 1 (H100, label: network-dra=rdma)    │
│   ┌─────────────┐                           │
│   │ DRANET Pod   │  ← Runs here             │
│   └─────────────┘                           │
│                                             │
│   Node 2 (H100, label: network-dra=rdma)    │
│   ┌─────────────┐                           │
│   │ DRANET Pod   │  ← Runs here             │
│   └─────────────┘                           │
│                                             │
│   Node 3 (D16s_v3, no RDMA)                 │
│   (no DRANET pod)  ← Skipped               │
│                                             │
└─────────────────────────────────────────────┘
```

The DaemonSet uses **node affinity** so it only runs on nodes with the `kubernetes.azure.com/network-dra=rdma` label:

```yaml
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.azure.com/network-dra
          operator: In
          values: ["rdma"]
```

### Step 4: Node Initialization (What Happens When DRANET Starts)

When the DRANET pod starts on a node, two init containers run first:

**Init Container 1 — Enable NRI:**
NRI (Node Resource Interface) is a containerd plugin that lets DRANET hook into container creation. The init container checks if NRI is enabled in `/etc/containerd/config.toml`. If not, it adds the config and restarts containerd.

**Init Container 2 — Install OFED Drivers (optional):**
OFED (OpenFabrics Enterprise Distribution) is the Mellanox driver stack that makes RDMA hardware work. There are two options for installing these drivers:

| Option | How | Pros | Cons |
|---|---|---|---|
| **Baked in VHD** (preferred) | Drivers pre-installed in the node image | Node ready immediately at boot | Requires node reimage for driver updates |
| **DaemonSet init container** | Installed at runtime by DRANET | Can update drivers without reimaging | Slower node readiness, more privileged |

### Step 5: Device Discovery and Validation

Once initialized, the main DRANET container:

1. **Scans the node** for RDMA-capable devices (e.g., InfiniBand NICs under `/dev/infiniband`)
2. **Validates drivers** — checks that OFED kernel modules are loaded and functional
3. **Validates compatibility** — ensures the hardware/driver combination is supported

If validation fails, the node does **not** advertise any RDMA resources, preventing pods from being scheduled there with broken hardware.

### Step 6: Resource Advertisement (Making Devices Visible to Kubernetes)

DRANET uses the **Kubernetes DRA (Dynamic Resource Allocation) API** to tell the scheduler about available RDMA devices. It creates:

- **ResourceSlice** objects — each one represents a concrete RDMA device on a specific node

Think of this like a GPU plugin telling Kubernetes "this node has 4 GPUs available" — except DRANET says "this node has N RDMA NICs available."

```
┌──────────────┐         ┌─────────────────────────────┐
│  DRANET Pod  │ ──────► │  Kubernetes API Server       │
│  (on Node 1) │ creates │                             │
└──────────────┘         │  ResourceSlice:             │
                         │    node: node-1             │
                         │    devices:                 │
                         │      - rdma-nic-0           │
                         │      - rdma-nic-1           │
                         │      - rdma-nic-2           │
                         │      - rdma-nic-3           │
                         └─────────────────────────────┘
```

### Step 7: Pod Requests RDMA Device

Users request RDMA devices in their pod spec using a **ResourceClaimTemplate**:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-training-job
spec:
  containers:
  - name: trainer
    image: my-nccl-training:latest
    resources:
      claims:
      - name: rdma-nic
  resourceClaims:
  - name: rdma-nic
    resourceClaimTemplateName: rdma-claim-template
```

### Step 8: Scheduling and Device Attachment

When the pod is created:

1. **Scheduler** sees the resource claim, finds a node that has available RDMA devices (via the ResourceSlice), and places the pod there.
2. **Kubelet** calls DRANET's `PrepareResourceClaim` API to set up the device.
3. **NRI hook** fires during container creation — DRANET assigns the RDMA character devices (e.g., `/dev/infiniband/uverbs0`) to the container.
4. The pod starts with **exclusive access** to its assigned RDMA NIC.

```
Pod Scheduling Flow:

  User creates Pod
        │
        ▼
  Scheduler checks ResourceSlices
  "Node 1 has 4 RDMA NICs, 2 free"
        │
        ▼
  Pod scheduled to Node 1
        │
        ▼
  Kubelet → DRANET: PrepareResourceClaim
  DRANET reserves rdma-nic-2 for this pod
        │
        ▼
  Container creation → NRI hook
  DRANET attaches /dev/infiniband/* to container
        │
        ▼
  Pod running with RDMA access
```

### Step 9: Cleanup

When the pod terminates, DRANET releases the RDMA device and re-advertises it as available. No stale allocations are left behind.

---

## What DRANET Does NOT Do

- **Primary pod networking** — Cilium (or your CNI) still handles the default NIC and pod-to-pod traffic.
- **GPU management** — NVIDIA device plugin handles GPUs. DRANET only manages the RDMA network interfaces.
- **OFED driver updates** (in the VHD-baked model) — driver updates require a new node image.

---

## Health Monitoring

DRANET exposes a `/healthz` HTTP endpoint on port 9177. Kubernetes uses this as a readiness probe — a node only becomes a candidate for RDMA workloads once DRANET is healthy and has published its ResourceSlices.

If DRANET crashes or is being updated (rolling restart), pods requiring RDMA devices will stay `Pending` until the DRANET pod is back and has republished its resources.

---

## Limitations

- **Secure Boot, vTPM, FIPS images, and Confidential VMs (CVM)** are NOT supported due to unsigned NVIDIA networking drivers.
- Mixed-mode node pools (some nodes managed, some unmanaged) are NOT supported. The setting applies to the entire node pool.

---

## AzureLinux Support for RDMA: Required Changes

### What We Know Works Today

The Azure Networking team has **validated RDMA on AzureLinux AKS** using the NVIDIA Network Operator on HBv4 nodes, achieving **~390 GB/s** — 98% of theoretical max InfiniBand bandwidth. This confirms:

- AzureLinux's **inbox kernel drivers** (`mlx5_core`, `mlx5_ib`) work correctly — no external OFED driver packaging needed.
- NVIDIA's precompiled OFED driver container does **not** work on AzureLinux. The OFED driver deployment in the NVIDIA Network Operator helm chart must be **disabled** when using AzureLinux, since AzureLinux relies on inbox (in-kernel) Mellanox drivers instead.
- The **`rdma-core` package** is the critical missing piece. Without it, kernel modules `rdma_cm`, `rdma_ucm`, `ib_umad`, and `ib_ipoib` do not autoload.

This simplifies the AzureLinux RDMA story: the kernel already has everything it needs. We just need to install the userspace package and wire up the AKS platform plumbing.

> **Note on NVIDIA Network Operator v25.7.0+:** The `NicClusterPolicy` resource was removed from the helm chart and must be applied manually via `kubectl` after installation.

---

### 1. Install `rdma-core` in the AzureLinux VHD (P0 — Critical Blocker)

**The problem:** Without `rdma-core`, the RDMA kernel modules (`rdma_cm`, `rdma_ucm`, `ib_umad`, `ib_ipoib`) do not autoload. Today, customers must work around this with a privileged DaemonSet to install the package at runtime — a fragile approach that every RDMA customer would have to replicate.

**The fix:** Add `rdma-core` to the AzureLinux VHD image.

**Where to change:**

- **`vhdbuilder/scripts/linux/mariner/tool_installs_mariner.sh`** — add `rdma-core` to the package install list (either unconditionally for all VHDs, or gated to GPU/HPC SKU VHD variants).
- **`components.json`** — pin the `rdma-core` version for AzureLinux 3.0.

**Scope decision:** Should `rdma-core` be installed on **all** AzureLinux images or only GPU/HPC images?
- `rdma-core` is a small userspace library (~2 MB) with no runtime overhead if no RDMA hardware is present.
- Installing it on all images is the simplest approach and avoids SKU-specific VHD variants.
- This is the question the networking team raised: *"Can your engineers determine whether it is appropriate to include the rdma-core package on all images?"*

**Recommendation:** Install `rdma-core` on all AzureLinux VHDs. The package is small, has no side effects on non-RDMA nodes, and avoids the need for every RDMA customer to deploy a privileged DaemonSet workaround.

### 2. NRI Support in containerd Configuration (P1)

**Current state:** AgentBaker's containerd config templates (`CONTAINERD_CONFIG_CONTENT` / `CONTAINERD_CONFIG_NO_GPU_CONTENT`) do **not** include NRI plugin configuration. The DRANET DaemonSet has an init container that can add NRI config at runtime, but this requires restarting containerd.

**Required work:**

- **Add NRI config to the AzureLinux containerd config template** used during node bootstrap. The NRI section should be added to the containerd `config.toml` when the node pool has managed RDMA enabled:
  ```toml
  [plugins."io.containerd.nri.v1.nri"]
    disable = false
    disable_connections = false
    plugin_config_path = "/etc/nri/conf.d"
    plugin_path = "/opt/nri/plugins"
    plugin_registration_timeout = "5s"
    plugin_request_timeout = "5s"
    socket_path = "/var/run/nri/nri.sock"
  ```
- Baking this into the config avoids a containerd restart at DaemonSet startup, improving node readiness time.
- **Verify containerd version** — AzureLinux's containerd package must be a version that supports NRI (1.7+). Check the version in `components.json`.

### 3. Node Label Application — AKS-RP + AgentBaker (P0)

**Current state:** The label `kubernetes.azure.com/network-dra=rdma` is proposed but **not yet implemented** in either AKS-RP or AgentBaker.

**Required work in AKS-RP:**

- **Maintain an RDMA-capable SKU mapping** — a list of VM sizes that have RDMA hardware. This must cover both GPU SKUs (ND-series with InfiniBand) and HPC SKUs (HB-series with InfiniBand, e.g., HBv4). This is a broader set than the existing `FabricManagerGPUSizes` map.
- **Pass RDMA enablement signal to AgentBaker** — when `NvidiaRDMAProfile.managementMode == "Managed"`, AKS-RP must include the RDMA label in the node pool's `CustomNodeLabels` so it flows through to kubelet `--node-labels`.

**Required work in AgentBaker:**

- **Apply the label during bootstrap** — the label must be included in `KUBELET_NODE_LABELS` (set in `cse_cmd.sh`). This is already supported via the `CustomNodeLabels` mechanism in `AgentPoolProfile.GetKubernetesLabels()`, so no new code may be needed if AKS-RP passes it down correctly.
- **Alternatively, add conditional logic in CSE** — detect RDMA SKUs at boot and call `addKubeletNodeLabel "kubernetes.azure.com/network-dra=rdma"` in `cse_helpers.sh`.

### 4. RDMA SKU Mapping (P0)

**Current state:** `gpu_components.go` contains `FabricManagerGPUSizes` which lists multi-GPU SKUs (A100/H100/H200), but there is **no dedicated RDMA SKU list**.

**Required work:**

- **Create an explicit `RDMACapableSizes` map** in `gpu_components.go` (or a new file). The RDMA mapping must be independent from the fabric-manager mapping because:
  - HPC SKUs like **HBv4** have InfiniBand RDMA but no GPUs or fabric manager.
  - Future RDMA SKUs may not be GPU SKUs at all.
- **Known RDMA-capable SKU families to include:**
  - ND96asr v4 (A100) — GPU + IB
  - ND*s H100 v5 — GPU + IB
  - ND*s H200 v5 — GPU + IB
  - HBv3, HBv4 — HPC + IB (no GPU)
  - HC-series — HPC + IB (no GPU)

### 5. Disable OFED Driver Deployment for AzureLinux (P1 — DRANET/Network Operator Integration)

Since AzureLinux uses inbox kernel drivers, when DRANET or the NVIDIA Network Operator is deployed on AzureLinux nodes:

- **The OFED driver init container in the DRANET DaemonSet should be a no-op** on AzureLinux. The init container should detect the OS and skip driver installation.
- **For the NVIDIA Network Operator helm chart**, the `OFEDDriver` deployment must be explicitly disabled when targeting AzureLinux node pools.
- AgentBaker or AKS-RP should pass a signal (e.g., an environment variable or node annotation) that DRANET can use to determine whether to skip OFED driver installation.

### 6. E2E Test Coverage (P1)

**Current state:** E2E tests validate NRI socket presence (`/var/run/nri/nri.sock`) but there are no RDMA-specific E2E tests in AgentBaker.

**Required work:**

- **Add E2E scenario** for AzureLinux + RDMA SKU (e.g., HBv4) that validates:
  - Inbox Mellanox drivers are loaded (`lsmod | grep mlx5`)
  - InfiniBand devices are present (`ls /dev/infiniband/`)
  - `rdma-core` tools work (`ibv_devinfo`)
  - RDMA kernel modules autoloaded (`rdma_cm`, `rdma_ucm`, `ib_umad`, `ib_ipoib`)
  - DRANET pod is running and healthy
  - ResourceSlices are published
  - A test pod can successfully claim and use an RDMA device
- **Bandwidth validation** — optionally run an IB bandwidth test and verify ≥95% of theoretical max (validated at ~98% on HBv4).
- **Add to the AzureLinux VHD validation matrix** in the existing test infrastructure.

### 7. CSE Bootstrap Flow Updates (P1)

**Current state:** `cse_config.sh` handles GPU driver configuration for AzureLinux via `downloadGPUDrivers()` + `installNvidiaContainerToolkit()`. There is no RDMA-specific configuration step.

**Required work in `cse_config.sh` or `cse_install_mariner.sh`:**

- Add a function (e.g., `configureRDMA()`) that:
  - Verifies `rdma-core` is installed
  - Loads InfiniBand modules if not auto-loaded: `modprobe mlx5_ib`
  - Verifies `/dev/infiniband` directory exists
  - Configures AzureLinux-specific sysctl settings for RDMA performance (e.g., locked memory limits)
- Gate this function on RDMA SKU detection or a flag passed from AKS-RP.

---

### Summary of Changes

| Area | Component | Change | Priority | Notes |
|---|---|---|---|---|
| VHD | `tool_installs_mariner.sh` | Install `rdma-core` package | **P0** | Single package, ~2 MB, enables kernel module autoload |
| VHD | `components.json` | Pin `rdma-core` version | **P0** | For AzureLinux 3.0 section |
| AKS-RP | SKU mapping | Create RDMA-capable SKU list (ND + HB series) | **P0** | Broader than FabricManagerGPUSizes |
| AKS-RP | Node pool API | Pass RDMA label via CustomNodeLabels | **P0** | When managementMode == Managed |
| AgentBaker | `gpu_components.go` | Add `RDMACapableSizes` map | **P0** | Include HPC SKUs (HBv3/v4, HC) |
| VHD | containerd config | Add NRI plugin section for RDMA nodes | P1 | Avoids containerd restart at runtime |
| AgentBaker | `cse_config.sh` | Add `configureRDMA()` function | P1 | Module load verification, sysctl tuning |
| DRANET | DaemonSet init | Skip OFED install on AzureLinux (inbox drivers) | P1 | Detect OS, no-op for AzureLinux |
| E2E | `scenario_test.go` | Add AzureLinux RDMA validation scenario | P1 | Including bandwidth validation |

### Key Insight: AzureLinux RDMA Is Simpler Than Expected

The AzureLinux kernel already ships with inbox Mellanox drivers (`mlx5_core`, `mlx5_ib`). Unlike Ubuntu where NVIDIA's precompiled OFED driver containers are used, AzureLinux does not need external OFED drivers at all. The only blocker is the missing `rdma-core` userspace package — a single, small RPM install in the VHD gets AzureLinux to full RDMA functionality at 98% theoretical bandwidth.
