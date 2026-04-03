# Analyze HCS — Host Compute Service Operational Health

## Purpose

Perform deep analysis of HCS (Host Compute Service) operational health by examining the full spectrum of Hyper-V Compute events, detecting performance degradation, tracking container lifecycle completeness, identifying resource leaks in vmcompute.exe, and correlating HCS operations with specific workloads. Complements analyze-termination.md (which focuses on zombie detection and 0xC0370103); this sub-skill covers the broader HCS operational picture.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `<ts>_hyper-v-compute-operational.csv` | UTF-16-LE with BOM, CSV with embedded newlines | HCS operation events (Create/Start/Shutdown/Terminate) |
| `<ts>-hcsdiag-list.txt` | UTF-16-LE with BOM | `hcsdiag list` — active HCS compute systems |
| `<ts>-shimdiag-list-with-pids.txt` | UTF-16-LE with BOM | `shimdiag list --pids` — shim processes |
| `<ts>-cri-containerd-pods.txt` | UTF-16-LE with BOM | `crictl pods` — pods tracked by containerd |
| `<ts>-containerd-info.txt` | UTF-16-LE with BOM | containerd version info (version, build, revision) |
| `<ts>-containerd-toml.txt` | UTF-16-LE with BOM | containerd configuration (TOML format) |
| `<ts>_services.csv` | UTF-16-LE with BOM, CSV with embedded newlines | Service install/crash events |
| `processes.txt` | UTF-16-LE with BOM | `Get-Process` snapshot — vmcompute memory/handle usage |
| `scqueryex.txt` | UTF-16-LE with BOM | Service states — vmcompute service status |

**Process ALL snapshots** — cross-snapshot comparison is essential for detecting HCS performance trends and resource leaks.

## Analysis Steps

### 1. vmcompute Service Health

**Check service status (`scqueryex.txt`)**:
- Search for `vmcompute` (service name) or `Hyper-V Host Compute Service` (display name)
- 🔴 CRITICAL: Service is STOPPED or not found — all container operations will fail
- 🔵 INFO: Service is RUNNING — normal

**Check for vmcompute crashes (`*_services.csv`)**:
- Search for messages mentioning "vmcompute" combined with "terminated unexpectedly", "stopped", "failed", or "crash"
- 🔴 CRITICAL: vmcompute crash detected — all in-flight HCS operations fail, containers may become orphaned
- Report exact timestamp of crash and any restart event

**Check vmcompute resource usage (`processes.txt`)**:
- Find `vmcompute` process entry. Note working set (memory) and handle count.
- 🟡 WARNING: Working set > 500 MB — possible memory leak (normal is ~50-150 MB)
- 🔴 CRITICAL: Working set > 1 GB — severe memory leak, vmcompute likely degraded
- 🟡 WARNING: Handle count > 5000 — possible handle leak
- Compare across snapshots: steadily increasing memory/handles = confirmed leak

### 2. Full HCS Event Classification (`*_hyper-v-compute-operational.csv`)

Parse all events using proper CSV parsing (handle embedded newlines). Skip `#TYPE` header lines.

**Classify every event by Event ID and Level**:

| Event ID | Operation | Success Level | Failure Level |
|----------|-----------|---------------|---------------|
| 2000 | Create | Information | Error/Critical |
| 2001 | Start | Information | Error/Critical |
| 2002 | Shutdown | Information | Error/Critical |
| 2003 | Terminate | Information | Error/Critical |

**Build an operation summary table**:
```
Event ID | Operation  | Success | Error | Total
2000     | Create     | 145     | 3     | 148
2001     | Start      | 142     | 6     | 148
2002     | Shutdown   | 130     | 15    | 145
2003     | Terminate  | 140     | 5     | 145
```

**Severity for error rates per operation type**:
- 🔵 INFO: Error rate < 5%
- 🟡 WARNING: Error rate 5–20%
- 🔴 CRITICAL: Error rate > 20%

### 3. HCS Error Code Analysis

For every Error/Critical event, extract the error code from the `Message` field using pattern: `(0x[0-9A-Fa-f]{8})`

**Group errors by code and report counts**:

| Code | Name | Count | Severity |
|------|------|-------|----------|
| `0x80370100` / `0xC0370100` | `HCS_E_TERMINATED_DURING_START` | Container crashed during startup | 🟡 per occurrence |
| `0x80370101` / `0xC0370101` | `HCS_E_IMAGE_MISMATCH` | OS version mismatch | 🔴 if any — misconfigured image |
| `0x80370106` / `0xC0370106` | `HCS_E_UNEXPECTED_EXIT` | Container exited unexpectedly | 🟡 per occurrence |
| `0x80370109` / `0xC0370109` | `HCS_E_CONNECTION_TIMEOUT` | HCS operation timed out | 🔴 — vmcompute overloaded |
| `0x8037010E` / `0xC037010E` | `HCS_E_SYSTEM_NOT_FOUND` | Containerd referencing unknown container | 🟡 race condition |
| `0x80370110` / `0xC0370110` | `HCS_E_SYSTEM_ALREADY_STOPPED` | Stopping already-stopped container | 🔵 usually benign |
| `0x80370118` / `0xC0370118` | `HCS_E_OPERATION_TIMEOUT` | Operation exceeded internal timeout | 🔴 — HCS performance issue |
| `0x8037011E` / `0xC037011E` | `HCS_E_SERVICE_DISCONNECT` | vmcompute crashed/restarted | 🔴 — all containers affected |
| `0x8037011F` / `0xC037011F` / `0xC0370103` | `HCS_E_PROCESS_ALREADY_STOPPED` | Process already exited | (defer to analyze-termination) |
| `0x800705AA` | Insufficient system resources | Resource exhaustion creating compartments | 🔴 — node resource issue |

**Note**: Error codes appear in both HRESULT (`0x8037xxxx`) and NTSTATUS (`0xC037xxxx`) forms in logs. Match both patterns.

For `HCS_E_PROCESS_ALREADY_STOPPED` (any variant), note count but defer detailed analysis to analyze-termination sub-skill. Focus here on all OTHER error types.

### 4. Container Lifecycle Completeness

For each unique container ID (64-char hex, extracted via `\[([a-f0-9]{64})\]` from Message field), track which operations occurred:

**Expected complete lifecycle**: Create (2000) → Start (2001) → Shutdown (2002) or Terminate (2003)

**Detect incomplete lifecycles**:
- 🟡 WARNING: Container has Create but no Start — creation failed, sandbox may be leaked
- 🟡 WARNING: Container has Create + Start but no Shutdown/Terminate — container may still be running or was orphaned
- 🔵 INFO: Container has Create + Start + Terminate (no Shutdown) — killed without graceful stop (normal for force-delete)
- 🔵 INFO: Container has full Create → Start → Shutdown → Terminate — healthy lifecycle

**Cross-reference with hcsdiag**: Containers that appear in hcsdiag but have no Terminate event in the log are still running (expected) or orphaned (if no matching crictl pod).

Report counts per lifecycle pattern. If >10% of containers have incomplete lifecycles, flag as 🟡 WARNING.

### 5. HCS Operation Duration Analysis

Extract timestamps from `TimeCreated` column for each event. For each container ID, calculate:
- **Create-to-Start duration**: Time between Event 2000 and Event 2001 for same container
- **Shutdown-to-Terminate duration**: Time between Event 2002 and Event 2003 for same container (indicates graceful shutdown failed and force-terminate was needed)

**Thresholds**:
- 🔵 INFO: Create-to-Start < 30 seconds — normal
- 🟡 WARNING: Create-to-Start 30–120 seconds — slow container startup
- 🔴 CRITICAL: Create-to-Start > 120 seconds — severely degraded HCS performance

- 🔵 INFO: No Shutdown-to-Terminate pair (graceful shutdown succeeded) — healthy
- 🟡 WARNING: Shutdown-to-Terminate < 30 seconds — graceful shutdown failed quickly
- 🔴 CRITICAL: Shutdown-to-Terminate > 60 seconds — HCS struggled to terminate

**Detect degradation trends**: Compare operation durations across the event log timeline. If average Create-to-Start increases over time, vmcompute may be degrading (memory leak, resource exhaustion).

Report:
- Average, median, and max Create-to-Start duration
- Count and average Shutdown-to-Terminate duration (these indicate failed graceful shutdowns)
- Any outlier operations (> 2× the average)

### 6. HCS Event Rate and Burst Analysis

Calculate the rate of HCS operations over time:
- Group events into 5-minute windows by `TimeCreated`
- Count Create (2000) events per window

**Thresholds**:
- 🔵 INFO: < 20 creates per 5 minutes — normal churn
- 🟡 WARNING: 20–50 creates per 5 minutes — high container churn
- 🔴 CRITICAL: > 50 creates per 5 minutes — storm of container creation (likely cascading failures)

**Detect retry storms**: If Create events for the SAME container ID appear multiple times, containerd is retrying sandbox creation. Count containers with >1 Create event.
- 🟡 WARNING: Any container with >1 Create attempt
- 🔴 CRITICAL: Containers with >3 Create attempts — persistent creation failure

### 7. HCS Resource Tracking Across Snapshots

Compare hcsdiag output across snapshots:

**Container count trend**:
- Count HCS containers (64-char hex lines) in each snapshot's hcsdiag output
- 🟡 WARNING: Count increasing across snapshots without corresponding crictl pod increase — containers accumulating (leak)
- 🔴 CRITICAL: >50 more HCS containers than crictl pods — severe container leak

**New vs disappeared containers**:
- Containers present in later snapshot but not earlier = new (normal churn)
- Containers present in earlier snapshot but not later = cleaned up (good)
- Containers present in ALL snapshots with no matching crictl pod = zombies (defer count to analyze-termination, but note the accumulation pattern here)

### 8. containerd Version & Configuration (`*-containerd-info.txt`, `*-containerd-toml.txt`)

Extract containerd version from `*-containerd-info.txt`:
- Look for `Version:\s*(\S+)` — report as 🔵 INFO
- Note build revision and Go version if present

Parse `*-containerd-toml.txt` for configuration relevant to HCS:
- Check `[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runhcs-wcow-process]` — runtime type for Windows containers
- Check snapshot plugin configuration
- Note any non-default settings that affect container lifecycle

- 🔵 INFO: containerd version and runtime configuration
- 🟡 WARNING: Unexpected runtime configuration or missing Windows-specific plugins

**Correlation**: If `_services.csv` shows "A service was installed in the system" for containerd, flag immediately — this indicates containerd was reinstalled and is a key trigger for termination analysis.

### 9. Novel Issue Detection

Beyond the numbered patterns above, also check for:
- **vmcompute RPC failures**: Messages containing "The RPC server is unavailable" indicate vmcompute has become unresponsive (memory leak). Cross-reference with vmcompute memory in processes.txt.
- **Burst of errors after a specific timestamp**: May indicate a Windows Update, service restart, or external event that disrupted HCS.
- **Errors concentrated on specific container IDs**: Same container failing repeatedly = workload-specific issue. Extract pod name from crictl correlation.
- **Mixed error types on same container**: e.g., Create succeeds → Start fails → Create retried — indicates transient vs persistent failure.
- **Errors only on Event ID 2000 (Create)**: Sandbox creation failures, possibly storage/network related.
- **Errors only on Event ID 2002/2003 (Shutdown/Terminate)**: Cleanup issues, possibly Defender or file lock related.
- **Long gaps between events**: If no HCS events for extended periods on a busy node, vmcompute may have been hung.
- **HCS_E_SYSTEM_ALREADY_EXISTS (0x8037010F)**: Containerd trying to create a container that HCS already tracks — stale state from previous containerd instance.

## Findings Format

```markdown
### HCS Operational Health Findings

🔵 **INFO** (HIGH confidence): vmcompute service running, working set 85 MB, 1200 handles
  - No signs of memory leak across 3 snapshots

🟡 **WARNING** (HIGH confidence): HCS Create error rate 12% (18 failures out of 148 operations)
  - 12× HCS_E_UNEXPECTED_EXIT (0x80370106)
  - 3× HCS_E_CONNECTION_TIMEOUT (0x80370109)
  - 3× HCS_E_TERMINATED_DURING_START (0x80370100)

🔴 **CRITICAL** (HIGH confidence): 3 containers with HCS_E_CONNECTION_TIMEOUT — vmcompute overloaded
  - Affected containers: abc123..., def456..., ghi789...
  - All timeouts occurred between 03:41:00–03:43:00 (burst)

🟡 **WARNING** (MEDIUM confidence): Average Create-to-Start duration 45s (normal <30s)
  - Max: 180s (container abc123...)
  - Duration increasing over time: 20s avg in first hour → 65s avg in last hour

🔵 **INFO** (HIGH confidence): HCS operation summary
  | Operation  | Success | Error | Error Rate |
  |-----------|---------|-------|------------|
  | Create     | 130     | 18    | 12%        |
  | Start      | 128     | 2     | 2%         |
  | Shutdown   | 120     | 10    | 8%         |
  | Terminate  | 125     | 5     | 4%         |

🟡 **WARNING** (MEDIUM confidence): 8 containers have incomplete lifecycles
  - 5 containers: Create + Start only (no Shutdown/Terminate in log)
  - 3 containers: Create only (Start never succeeded)

🔴 **CRITICAL** (HIGH confidence): vmcompute memory increasing across snapshots
  - Snapshot 1: 90 MB, Snapshot 2: 250 MB, Snapshot 3: 680 MB
  - Consistent with vmcompute memory leak (hcsshim#1680)
```

## Known Patterns

| Pattern | Indicators | Severity | Root Cause |
|---------|-----------|----------|------------|
| vmcompute memory leak | Working set increasing across snapshots, >500 MB, RPC failures | 🔴 CRITICAL | Failed exec probes leak memory in vmcompute (hcsshim#1680). Common with ama-logs-windows daemonset. |
| vmcompute crash | `HCS_E_SERVICE_DISCONNECT` errors, vmcompute terminated in services.csv | 🔴 CRITICAL | Memory exhaustion or bug in vmcompute. All in-flight operations lost. |
| Container creation storm | >50 creates/5min, same IDs retried >3×, cascading errors | 🔴 CRITICAL | High pod churn overwhelms HCS. Containers fail to start, containerd retries, amplifying the problem. |
| OS version mismatch | `HCS_E_IMAGE_MISMATCH` (0x80370101) on Create | 🔴 CRITICAL | Container image built for different Windows version than host. Check node OS version vs image tag. |
| Stale HCS state after containerd restart | `HCS_E_SYSTEM_ALREADY_EXISTS` (0x8037010F) on Create | 🟡 WARNING | Previous containerd instance left HCS containers. New containerd tries to create same IDs. |
| Slow HCS operations (degradation) | Create-to-Start >30s, increasing over time | 🟡 WARNING | vmcompute resource exhaustion, disk I/O contention, or Defender scanning interference. |
| Graceful shutdown failures | Many Shutdown→Terminate pairs in quick succession | 🟡 WARNING | Containers not responding to graceful shutdown. App ignores SIGTERM or HCS shutdown signal. |
| Network compartment exhaustion | Error 0x800705AA "Insufficient system resources" | 🔴 CRITICAL | Too many containers created/destroyed without cleanup. Reboot may be needed. |
| HCS notification timeout | "timeout waiting for notification" in containerd logs | 🟡 WARNING | HCS operation hung. Shim waits for callback that never fires. Container stuck. |

## Cross-References

- **→ analyze-termination**: For zombie container detection, stable PID analysis, and 0xC0370103 deep-dive. This sub-skill identifies the HCS operational context; termination sub-skill identifies the stuck pods.
- **→ analyze-services**: For service crash events and kube-proxy/kubelet service state. vmcompute crashes will also appear in services.csv analysis.
- **→ analyze-containers**: For pod/container health. HCS errors cause container restarts and pod NotReady states.
- **→ common-reference**: For HCS event ID definitions, error code table, encoding details.
- **Timeline correlation**: Use HCS event timestamps to anchor when problems started. A burst of HCS errors at time T should correlate with container restarts and pod state changes at time T+seconds.
- **vmcompute memory → probe failures**: If vmcompute memory is high, check container analysis for crash-looping containers with exec/command probes. Each failed probe leaks vmcompute memory.
- **HCS creation failures → disk analysis**: Failed container creation can be caused by disk pressure (insufficient space for container layers/snapshots).
