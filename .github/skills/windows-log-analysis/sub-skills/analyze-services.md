# Analyze Services — Windows Services Health

## Purpose

Detect stopped/failed critical services, service crash patterns, unexpected service states, and service dependency issues on Windows AKS nodes by parsing `scqueryex.txt`, `services.csv`, `silconfig.log`, and cross-referencing with `processes.txt`.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `scqueryex.txt` | UTF-16-LE with BOM | `sc queryex` output (all services) + `sc qc` for key services |
| `<ts>_services.csv` | UTF-16-LE with BOM, CSV with embedded newlines | Service Control Manager event log |
| `silconfig.log` | UTF-16-LE with BOM or UTF-8 | Software Inventory Logging configuration |
| `processes.txt` | UTF-16-LE with BOM | Running processes with PIDs |
| `kubectl-get-nodes.log` | UTF-8 | `kubectl get nodes -o wide` output |
| `<ts>-containerd-info.txt` | UTF-16-LE with BOM | containerd version info |

**CSV parsing warning**: The `Message` field in `services.csv` contains embedded newlines. You MUST use proper CSV parsing — do NOT split on newlines. Also skip lines starting with `#TYPE`.

## Analysis Steps

### 1. Parse scqueryex.txt — Service State Inventory

The file contains output of `sc queryex` (all services) followed by `sc qc` blocks for specific services (hns, vfpext, dnscache, iphlpsvc, BFE, Dhcp, hvsics, NetSetupSvc, mpssvc).

**`sc queryex` format** — repeating blocks:
```
SERVICE_NAME: <name>
DISPLAY_NAME: <display_name>
        TYPE               : <hex> <type_text>
        STATE              : <code> <state_text>  (STOPPABLE, NOT_PAUSABLE, ...)
        WIN32_EXIT_CODE    : <code> (0x<hex>)
        SERVICE_EXIT_CODE  : <code> (0x<hex>)
        CHECKPOINT         : 0x<hex>
        WAIT_HINT          : 0x<hex>
        PID                : <pid>
        FLAGS              :
```

**State codes**:
- `1 STOPPED`
- `2 START_PENDING`
- `3 STOP_PENDING`
- `4 RUNNING`

**`sc qc` format** — repeating blocks:
```
[SC] QueryServiceConfig SUCCESS
SERVICE_NAME: <name>
        TYPE               : <hex> <type_text>
        START_TYPE         : <code> <type_text>
        ERROR_CONTROL      : <code> <type_text>
        BINARY_PATH_NAME   : <path>
        ...
        DEPENDENCIES       : <dep1>
                           : <dep2>
        SERVICE_START_NAME : <account>
```

**START_TYPE values**:
- `2 AUTO_START` — starts at boot
- `3 DEMAND_START` — manual start
- `4 DISABLED` — cannot start

Parse ALL service blocks and build a map of service name → state, PID, start type.

### 2. Critical Service Health Check

Check these AKS-critical services against expected states:

| Service Name | Expected State | Expected Start Type | Impact if Down |
|-------------|---------------|-------------------|----------------|
| `kubelet` | RUNNING | DEMAND_START (NSSM-managed; AUTO or DEMAND acceptable) | Node is NotReady, no pod scheduling |
| `containerd` | RUNNING | DEMAND_START (NSSM-managed; AUTO or DEMAND acceptable) | No container operations |
| `csi-proxy` | RUNNING | DEMAND_START (NSSM-managed; AUTO or DEMAND acceptable) | Volume mounts fail |
| `kubeproxy` / `kube-proxy` | RUNNING | DEMAND_START (NSSM-managed; AUTO or DEMAND acceptable) | Service routing broken |
| `hns` | RUNNING | AUTO_START | All networking broken |
| `vmcompute` | RUNNING | AUTO_START | No Hyper-V container support |
| `WinDefend` | RUNNING | AUTO_START | Security policy violation |
| `W32Time` | RUNNING | AUTO_START (or DEMAND) | Clock skew → cert/Kerberos failures |
| `Dnscache` | RUNNING | AUTO_START | DNS resolution failures |
| `BFE` | RUNNING | AUTO_START | Windows Filtering Platform down → firewall/HNS broken |
| `Dhcp` | RUNNING | AUTO_START | DHCP renewal failures |
| `vfpext` | RUNNING | AUTO_START | Virtual Filtering Platform (Azure CNI) broken |

- 🔴 CRITICAL: kubelet, containerd, hns, or vmcompute STOPPED or not found
- 🟡 WARNING: Other critical service STOPPED or DISABLED
- 🔵 INFO: All critical services RUNNING

### 3. Unexpected Stopped/Disabled Services Detection

Beyond the critical list, flag:
- Any service with `START_TYPE = AUTO_START` but `STATE = STOPPED` — service should be running but isn't
- Any service with non-zero `WIN32_EXIT_CODE` or `SERVICE_EXIT_CODE` — service exited with error
- Services in `START_PENDING` or `STOP_PENDING` — stuck in transition

- 🟡 WARNING: Auto-start service found stopped
- 🟡 WARNING: Service with non-zero exit code (crashed)
- 🔴 CRITICAL: Service stuck in pending state (possible deadlock)

### 4. Service PID Cross-Reference with processes.txt

For each RUNNING service with a PID > 0:
- Check if that PID exists in `processes.txt`
- A service reporting RUNNING with a PID that doesn't appear in the process list is a **zombie service** — the SCM thinks it's running but the process is gone

Parse `processes.txt` for PID column (typically `Id` or second column in `Get-Process` output).

- 🔴 CRITICAL: Critical service (kubelet, containerd) reports RUNNING but PID not in process list
- 🟡 WARNING: Non-critical service reports RUNNING but PID missing

### 5. services.csv Crash Pattern Correlation

Parse `*_services.csv` event logs (see common-reference.md for CSV handling).

Search for:
- Messages containing critical service names + "terminated unexpectedly" or "stopped" → crash events
- Rapid start/stop cycles for the same service (>3 events in 30 minutes) → crash loop
- Event ID 7034 (service crashed) or 7031 (service failure recovery action taken)
- Event ID 7023 (service terminated with error)

Build a crash timeline for each affected service.

- 🔴 CRITICAL: kubelet or containerd crash-looping (>3 restarts in 30 min)
- 🟡 WARNING: Any critical service crash event
- 🔵 INFO: Service restart events (single, clean restart)

### 6. silconfig.log Analysis

Software Inventory Logging (SIL) config log. Parse for:
- SIL service state (enabled/disabled)
- Configuration errors
- Aggregation server URI (if configured)

This is typically low-priority but can indicate:
- 🔵 INFO: SIL configuration status
- 🟡 WARNING: SIL errors that might indicate broader WMI issues

### 7. sc qc — Service Configuration Validation

For the services with `sc qc` output (hns, vfpext, dnscache, iphlpsvc, BFE, Dhcp, hvsics, NetSetupSvc, mpssvc):

- Verify `START_TYPE` matches expected (see critical services table)
- Check `DEPENDENCIES` — missing dependencies can explain cascade failures
- Check `BINARY_PATH_NAME` — unexpected paths may indicate tampering or misconfiguration

- 🟡 WARNING: Critical service with wrong START_TYPE (e.g., hns set to DEMAND_START)
- 🔵 INFO: Service dependency chains for context

### 8. Node Version & OS Information (`kubectl-get-nodes.log`, `*-containerd-info.txt`)

Parse `kubectl get nodes -o wide` output from `kubectl-get-nodes.log`.

**Format**: `NAME  STATUS  ROLES  AGE  VERSION  INT-IP  EXT-IP  OS-IMAGE...  KERNEL-VERSION  CONTAINER-RUNTIME`
- CONTAINER-RUNTIME is the last column
- KERNEL-VERSION is second-to-last
- OS-IMAGE is everything between column 7 and KERNEL-VERSION

**What to report**:
- 🔵 INFO: Each Windows node with its status, version, OS image, and runtime
- 🔵 INFO: containerd version skew between Windows and Linux nodes (extract version from `containerd://<version>` in CONTAINER-RUNTIME column). Different versions is normal if patch cycles differ, but worth noting.
- Very old node age combined with many error events (long-running degraded node) → 🟡 WARNING

Extract containerd version from latest snapshot's `*-containerd-info.txt`:
- Look for line matching `Version:\s*(\S+)`
- Report as 🔵 INFO

## Findings Format

```markdown
### Windows Services Health Findings

🔴 **CRITICAL** (HIGH confidence): kubelet service is STOPPED
  - SERVICE_NAME: kubelet, STATE: 1 STOPPED
  - WIN32_EXIT_CODE: 1 (0x1) — exited with error
  - Impact: Node is NotReady, no pod scheduling

🔴 **CRITICAL** (HIGH confidence): containerd crash-looping — 5 restarts in 20 minutes
  - [2026-03-23T03:30:00] containerd terminated unexpectedly
  - [2026-03-23T03:31:15] containerd service started
  - [2026-03-23T03:35:22] containerd terminated unexpectedly
  - ...

🟡 **WARNING** (MEDIUM confidence): W32Time service is STOPPED (START_TYPE: DEMAND_START)
  - Clock skew may cause certificate validation and Kerberos failures
  - Cross-reference: analyze-kubelet.md for cert rotation issues

🟡 **WARNING** (HIGH confidence): csi-proxy PID 4532 not found in processes.txt
  - Service reports RUNNING but process may have exited — zombie service state
```

## Known Patterns

| Pattern | Severity | Confidence | Meaning |
|---------|----------|------------|---------|
| kubelet STOPPED | 🔴 CRITICAL | HIGH | Node is NotReady |
| containerd STOPPED | 🔴 CRITICAL | HIGH | No container operations possible |
| hns STOPPED | 🔴 CRITICAL | HIGH | All pod networking broken |
| vmcompute STOPPED | 🔴 CRITICAL | HIGH | Hyper-V container runtime broken |
| Service crash-loop (>3 in 30 min) | 🔴 CRITICAL | HIGH | Persistent service failure |
| Service stuck in PENDING state | 🔴 CRITICAL | MEDIUM | Possible deadlock or dependency failure |
| RUNNING service with missing PID | 🔴 CRITICAL | MEDIUM | Zombie service — SCM stale state |
| W32Time STOPPED | 🟡 WARNING | HIGH | Clock skew risk → cert/Kerberos failures |
| AUTO_START service found STOPPED | 🟡 WARNING | MEDIUM | Service failed to start or crashed |
| Service with non-zero exit code | 🟡 WARNING | MEDIUM | Service exited abnormally |
| csi-proxy STOPPED | 🟡 WARNING | HIGH | Volume mount operations will fail |
| WinDefend STOPPED | 🟡 WARNING | MEDIUM | Security compliance issue |

## Cross-References

- **analyze-kubelet.md**: Stopped kubelet service explains NotReady node condition; W32Time issues cause clock skew → cert rotation failures
- **analyze-hns.md**: Stopped hns/vfpext/BFE services cause networking failures that may appear as CNI issues
- **analyze-containers.md**: Stopped containerd/vmcompute explains container creation failures
- **analyze-bootstrap.md**: Post-bootstrap service states validate whether provisioning completed correctly
- **analyze-images.md**: Stopped containerd prevents image GC and pull operations
