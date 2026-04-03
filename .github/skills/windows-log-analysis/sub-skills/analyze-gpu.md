# Analyze GPU — GPU / DirectX Device Health Sub-Skill

## Purpose

Detect GPU health issues on Windows AKS nodes: nvidia-smi output parsing for driver/hardware status, DirectX device plugin scheduling failures, Xid error classification, ECC memory errors, driver version mismatches, GPU thermal/power issues, and GPU process-to-container correlation. Windows AKS uses the DirectX device plugin (`microsoft.com/directx` resource) rather than the Linux NVIDIA device plugin (`nvidia.com/gpu`).

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `*-nvidia-smi.txt` or `*nvidia-smi*` | UTF-8 or UTF-16-LE with BOM | `nvidia-smi` output — GPU inventory, utilization, temperature, errors |
| `kubectl-describe-nodes.log` | UTF-8 | `kubectl describe node` — resource capacity/allocatable including GPU |
| `<ts>-aks-info.log` | UTF-16-LE with BOM | `kubectl describe node` + node YAML — GPU resource scheduling info |
| `kubelet.log` or `kubelet.err.log` | UTF-8 | Kubelet logs — device plugin registration, GPU allocation errors |
| `<ts>-cri-containerd-pods.txt` | UTF-16-LE with BOM | `crictl pods` — running pods for GPU process correlation |

## Analysis Steps

### 1. nvidia-smi Output Parsing (`*nvidia-smi*`)

Parse the standard nvidia-smi output table. The header section contains:

```
+-----------------------------------------------------------------------------------------+
| NVIDIA-SMI 537.70            Driver Version: 537.70       CUDA Version: 12.2            |
|-------------------------------------+------------------------+----------------------+
| GPU  Name                 Persistence-M | Bus-Id          Disp.A | Volatile Uncorr. ECC |
| Fan  Temp   Perf          Pwr:Usage/Cap |          Memory-Usage  | GPU-Util  Compute M. |
|=====================================+========================+======================+
|   0  Tesla T4                   On  | 00000001:00:00.0   Off |                    0 |
| N/A   42C    P8              10W / 70W |      0MiB / 15360MiB  |      0%      Default |
+-------------------------------------+------------------------+----------------------+
```

**Extract per GPU:**
- **GPU index**: integer (0, 1, 2, ...)
- **GPU name**: e.g., `Tesla T4`, `Tesla V100`, `A100`
- **Driver version**: from header line (e.g., `537.70`)
- **CUDA version**: from header line (e.g., `12.2`)
- **Temperature**: in °C (e.g., `42C`)
- **Power usage/cap**: e.g., `10W / 70W`
- **Memory usage**: e.g., `0MiB / 15360MiB`
- **GPU utilization**: e.g., `0%`
- **ECC errors**: `Volatile Uncorr. ECC` column — `0` is healthy, `N/A` means ECC not supported, any number > 0 is concerning
- **Persistence mode**: `On` or `Off`

- 🔴 CRITICAL: nvidia-smi output shows `No devices were found` or `NVIDIA-SMI has failed` — GPU not detected by driver
- 🔴 CRITICAL: GPU temperature > 90°C — thermal throttling or imminent shutdown
- 🔴 CRITICAL: Uncorrectable ECC errors > 0 — hardware memory failure, GPU should be replaced
- 🟡 WARNING: GPU temperature 80–90°C — elevated, approaching thermal limit
- 🟡 WARNING: Power usage at or exceeding cap — GPU power-limited
- 🟡 WARNING: GPU memory usage > 95% — memory pressure, OOM kills possible
- 🔵 INFO: GPU inventory, driver version, CUDA version, utilization (baseline data)

### 2. nvidia-smi Error Detection (`*nvidia-smi*`)

Check for error conditions in nvidia-smi output:

| Error Output | Meaning |
|-------------|---------|
| `NVIDIA-SMI has failed because it couldn't communicate with the NVIDIA driver` | Driver not loaded or GPU fallen off bus |
| `No devices were found` | No GPU hardware detected or driver failure |
| `ERR!` in any field | GPU in error state — field-specific failure |
| `Unknown Error` | Driver internal error |
| `GPU has fallen off the bus` | PCIe bus error — hardware failure (Xid 79) |
| `Xid` in nvidia-smi output or accompanying logs | NVIDIA Xid error — see classification below |

- 🔴 CRITICAL: nvidia-smi cannot communicate with driver — GPU non-functional
- 🔴 CRITICAL: `ERR!` in temperature or power fields — GPU hardware failure
- 🔴 CRITICAL: `GPU has fallen off the bus` — PCIe failure, node may need reboot/replacement

### 3. Xid Error Classification

If Xid errors are present in nvidia-smi output or system event logs, classify them:

| Xid Code | Name | Severity | Meaning |
|----------|------|----------|---------|
| 13 | Graphics Engine Exception | 🟡 WARNING | Application-level GPU fault — usually recoverable |
| 31 | GPU memory page fault | 🟡 WARNING | Application accessing invalid GPU memory |
| 32 | Invalid or corrupted push buffer | 🟡 WARNING | Driver/application command stream error |
| 38 | Driver firmware error | 🔴 CRITICAL | Firmware failure — may need driver reinstall |
| 43 | GPU stopped processing | 🟡 WARNING | Application hang on GPU — context reset needed |
| 45 | Preemptive cleanup | 🔵 INFO | Normal cleanup after context error |
| 48 | Double-bit ECC error | 🔴 CRITICAL | Uncorrectable memory error — hardware failure |
| 63 | ECC page retirement (row remapping) | 🟡 WARNING | Memory cells retired — monitoring needed |
| 64 | ECC page retirement limit | 🔴 CRITICAL | Too many retired pages — GPU replacement needed |
| 69 | Graphics Engine Class Error | 🟡 WARNING | Application context error |
| 79 | GPU has fallen off the bus | 🔴 CRITICAL | PCIe bus failure — hardware/power issue |
| 92 | High single-bit ECC error rate | 🟡 WARNING | Memory degrading — monitor for uncorrectable errors |
| 94 | Contained ECC error | 🟡 WARNING | ECC error contained, no data loss — but monitor |
| 95 | Uncontained ECC error | 🔴 CRITICAL | ECC error not contained — data corruption possible |

- 🔴 CRITICAL: Xid 48, 64, 79, 95 — hardware failure requiring GPU replacement or node reimage
- 🔴 CRITICAL: Xid 38 — firmware error, driver reinstall needed
- 🟡 WARNING: Xid 13, 31, 43, 63, 69, 92, 94 — application or recoverable errors, monitor frequency
- 🔵 INFO: Xid 45 — normal preemptive cleanup

### 4. ECC Memory Error Analysis (`*nvidia-smi*`)

If nvidia-smi includes detailed ECC information (from `nvidia-smi -q` or similar extended output):

**Check fields:**
- `Volatile` vs `Aggregate` ECC counts — volatile resets on driver restart, aggregate persists
- `Single Bit` (correctable) — corrected by ECC, but high count indicates degrading memory
- `Double Bit` (uncorrectable) — data corruption, GPU should be replaced
- `Retired Pages` — GPU memory pages permanently retired due to errors

- 🔴 CRITICAL: Double-bit ECC errors > 0 — uncorrectable memory errors
- 🔴 CRITICAL: Retired page count approaching limit (typically 48 per GPU)
- 🟡 WARNING: Single-bit ECC errors > 100 (aggregate) — memory degrading
- 🟡 WARNING: Retired pages > 0 but below limit — monitor trend
- 🔵 INFO: ECC supported with zero errors (healthy)
- 🔵 INFO: ECC not supported (consumer GPU or ECC disabled)

### 5. GPU Process List Analysis (`*nvidia-smi*`)

nvidia-smi includes a process section at the bottom:

```
+-----------------------------------------------------------------------------------------+
| Processes:                                                                              |
|  GPU   GI   CI        PID   Type   Process name                          GPU Memory     |
|        ID   ID                                                           Usage          |
|=========================================================================================+
|    0   N/A  N/A      4528      C   ...container_process.exe                  1234MiB    |
+-----------------------------------------------------------------------------------------+
```

**What to check:**
- Identify processes using GPU memory — PID, process name, memory usage
- `C` type = compute, `G` type = graphics
- Cross-reference PIDs with crictl pods to identify which containers use the GPU
- `No running processes found` when pods should be using GPU = scheduling or device plugin issue

- 🟡 WARNING: GPU processes running but no matching pod found — orphaned GPU workload
- 🟡 WARNING: Multiple containers sharing a single GPU — potential contention
- 🔵 INFO: GPU process list with container correlation

### 6. DirectX Device Plugin / GPU Scheduling (`kubectl-describe-nodes.log`, `*-aks-info.log`)

Parse `kubectl describe node` output for GPU resource information.

**Check Capacity and Allocatable sections:**
```
Capacity:
  microsoft.com/directx:  1
Allocatable:
  microsoft.com/directx:  1
```

**What to check:**
- `microsoft.com/directx` should appear in both Capacity and Allocatable
- If missing entirely: DirectX device plugin not installed or not running
- If Capacity shows GPU but Allocatable is 0: GPU marked unhealthy by device plugin
- Compare Allocatable count against Allocated count to see remaining GPU slots

**Check Conditions and Events:**
- Look for device plugin registration events: `RegisteredNode`, `DevicePluginRegistered`
- Look for errors: `FailedToStartDevicePlugin`, `UnregisterDevicePlugin`

**Check node labels:**
- `kubernetes.azure.com/accelerator=nvidia` — AKS GPU node label
- `node.kubernetes.io/instance-type` — VM SKU should be GPU-capable (NC*, ND*, NV* series)

- 🔴 CRITICAL: `microsoft.com/directx` missing from Capacity — GPU not detected by device plugin
- 🔴 CRITICAL: `microsoft.com/directx` in Capacity but 0 in Allocatable — GPU marked unhealthy
- 🟡 WARNING: Device plugin registration errors in kubelet events
- 🟡 WARNING: GPU-capable VM SKU but no `kubernetes.azure.com/accelerator` label
- 🔵 INFO: GPU resources available and allocatable (report count)

### 7. Kubelet Device Plugin Logs (`kubelet.log`)

Search kubelet logs for device plugin registration and GPU allocation:

| Log Pattern | Meaning |
|------------|---------|
| `DevicePlugin` + `Register` | Device plugin registering with kubelet |
| `DevicePlugin` + `allocate` | GPU allocation request for a pod |
| `DevicePlugin` + `failed` or `error` | Device plugin operation failure |
| `directx` or `microsoft.com/directx` | DirectX resource-related log entries |
| `endpoint is not registered` | Device plugin crashed or unregistered — GPU scheduling broken |
| `CDI` or `device` + `not found` | Device not available for allocation |

- 🔴 CRITICAL: `endpoint is not registered` for directx — device plugin down, GPU pods cannot be scheduled
- 🟡 WARNING: Device plugin allocation failures — GPU pods will be pending
- 🔵 INFO: Device plugin registered and allocating normally

### 8. Driver Version Compatibility

Cross-reference nvidia-smi driver version with known AKS Windows GPU requirements:

**Check:**
- Driver version from nvidia-smi header
- Compare against minimum required versions for the VM SKU family
- NC-series (Tesla K80): older driver OK
- NCv3-series (Tesla V100): requires driver ≥ 471.x
- NCasT4_v3 (Tesla T4): requires driver ≥ 471.x
- NDv2 (Tesla V100): requires driver ≥ 471.x

**CUDA version compatibility:**
- CUDA version in nvidia-smi indicates maximum CUDA version supported by the driver
- Container images requesting newer CUDA will fail

- 🟡 WARNING: Driver version below recommended minimum for VM SKU
- 🟡 WARNING: CUDA version may be incompatible with workload requirements
- 🔵 INFO: Driver version and CUDA version (baseline data)

## Findings Format

```markdown
### GPU / DirectX Device Health Findings

🔴 **CRITICAL** (HIGH confidence): nvidia-smi failed — GPU not detected
  - Output: "NVIDIA-SMI has failed because it couldn't communicate with the NVIDIA driver"
  - GPU workloads will fail to run on this node
  - Possible causes: driver not installed, GPU fallen off bus, driver crash

🔴 **CRITICAL** (HIGH confidence): Uncorrectable ECC errors detected on GPU 0
  - Volatile Uncorr. ECC: 3
  - Xid 48 (Double-bit ECC error) likely — hardware memory failure
  - GPU should be replaced — node reimage/replacement needed

🔴 **CRITICAL** (HIGH confidence): microsoft.com/directx missing from node Capacity
  - DirectX device plugin not running or failed to register
  - GPU pods will remain in Pending state with "Insufficient microsoft.com/directx"
  - Cross-ref: Check kubelet logs for device plugin registration errors

🟡 **WARNING** (MEDIUM confidence): GPU temperature at 87°C (threshold: 80°C warning)
  - GPU 0 (Tesla T4): 87°C, power 68W/70W (near cap)
  - May indicate inadequate cooling or sustained heavy workload

🔵 **INFO**: GPU inventory — 1x Tesla T4, Driver 537.70, CUDA 12.2
🔵 **INFO**: GPU utilization 45%, memory 8192MiB/15360MiB (53%)
🔵 **INFO**: 1 compute process using GPU (PID 4528, 1234MiB)
```

## Known Patterns

| Pattern | Severity | Confidence | Meaning |
|---------|----------|------------|---------|
| `NVIDIA-SMI has failed` | 🔴 CRITICAL | HIGH | GPU driver not communicating — GPU non-functional |
| `No devices were found` | 🔴 CRITICAL | HIGH | No GPU hardware detected |
| `GPU has fallen off the bus` (Xid 79) | 🔴 CRITICAL | HIGH | PCIe bus failure — hardware issue |
| Uncorrectable ECC errors > 0 | 🔴 CRITICAL | HIGH | Hardware memory failure — GPU replacement needed |
| Xid 48 (Double-bit ECC) | 🔴 CRITICAL | HIGH | Uncorrectable memory error |
| Xid 64 (ECC retirement limit) | 🔴 CRITICAL | HIGH | Too many retired memory pages |
| Xid 95 (Uncontained ECC) | 🔴 CRITICAL | HIGH | ECC error with potential data corruption |
| `microsoft.com/directx` missing from Capacity | 🔴 CRITICAL | HIGH | DirectX device plugin not running |
| Allocatable directx = 0 with Capacity > 0 | 🔴 CRITICAL | HIGH | GPU marked unhealthy by device plugin |
| `endpoint is not registered` for directx | 🔴 CRITICAL | HIGH | Device plugin crashed — GPU scheduling broken |
| `ERR!` in nvidia-smi fields | 🔴 CRITICAL | MEDIUM | GPU hardware error in specific subsystem |
| Temperature > 90°C | 🔴 CRITICAL | MEDIUM | Thermal throttling or imminent shutdown |
| Xid 38 (Driver firmware error) | 🔴 CRITICAL | MEDIUM | Firmware failure — needs driver reinstall |
| Temperature 80–90°C | 🟡 WARNING | MEDIUM | Elevated temperature — monitor |
| GPU memory > 95% | 🟡 WARNING | MEDIUM | Memory pressure — OOM possible |
| Power at/exceeding cap | 🟡 WARNING | LOW | GPU power-limited |
| Single-bit ECC errors > 100 (aggregate) | 🟡 WARNING | MEDIUM | Memory degrading — monitor |
| Retired pages > 0 | 🟡 WARNING | MEDIUM | Memory cells retired — trending toward failure |
| Xid 13, 31, 43, 63, 69 | 🟡 WARNING | MEDIUM | Application-level GPU errors — usually recoverable |
| Device plugin allocation failures | 🟡 WARNING | MEDIUM | GPU pods will be pending |
| Driver version below recommended | 🟡 WARNING | LOW | May cause compatibility issues |
| GPU-capable SKU without accelerator label | 🟡 WARNING | LOW | AKS label missing — scheduling may not target correctly |

## Cross-References

- **analyze-kubelet.md**: Device plugin registration happens through kubelet. If kubelet is crash-looping or NotReady, the DirectX device plugin cannot register, causing GPU scheduling failures. Kubelet events show device plugin registration success/failure.
- **analyze-services.md**: Service events may contain GPU driver crash events or blue screen codes related to GPU hardware failures. Service events show if the NVIDIA driver service (`nvlddmkm`) stopped or crashed.
- **analyze-containers.md**: Pods stuck in Pending with `Insufficient microsoft.com/directx` — correlate with GPU availability findings here. Containers crash-looping may be due to CUDA version mismatch with the host driver.
- **analyze-memory.md**: GPU memory exhaustion (found here) is separate from system RAM. However, some GPU workloads also consume significant host memory — correlate with system memory pressure.
