# Analyze Termination — Stuck Pods & Zombie HCS Containers

## Purpose

Detect pods stuck in Terminating state, zombie HCS containers, orphaned shim processes, containerd reinstall events, and environmental factors (Defender scanning, snapshot bloat) that cause or amplify termination failures.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `<ts>_hyper-v-compute-operational.csv` | UTF-16-LE with BOM, CSV with embedded newlines | HCS operation events |
| `<ts>-hcsdiag-list.txt` | UTF-16-LE with BOM | `hcsdiag list` — active HCS compute systems |
| `<ts>-shimdiag-list-with-pids.txt` | UTF-16-LE with BOM | `shimdiag list --pids` — shim processes |
| `<ts>-cri-containerd-pods.txt` | UTF-16-LE with BOM | `crictl pods` — pods tracked by containerd |
| `<ts>_services.csv` | UTF-16-LE with BOM, CSV with embedded newlines | Service install/crash events |
| `processes.txt` | UTF-16-LE with BOM | `Get-Process` snapshot — check for MsMpEng.exe, handle counts |
| `scqueryex.txt` | UTF-16-LE with BOM | Service states — check WinDefend service |
| `containerd.err-*.log` | UTF-8 or UTF-16-LE | containerd stderr — file lock errors during teardown |

**Process ALL snapshots** — cross-snapshot comparison is critical for detecting stable (zombie) PIDs.

## Analysis Steps

### 1. containerd/Kubelet Reinstall Events

Parse `*_services.csv` — search for messages containing "installed in the system" combined with "containerd" or "Kubelet".

- Skip `#TYPE` header lines. Find the real CSV header containing "TimeCreated".
- Report the exact timestamp of the reinstall event.

**Context**: A fresh node provisioning also produces this event. Correlate with HCS failure count — if hundreds of 0xC0370103 errors follow the install event, it was a reinstall on a running node (not clean provisioning).

### 2. HCS Terminate/Shutdown Failures

Parse `*_hyper-v-compute-operational.csv` (handle embedded newlines properly — use CSV parser, not line splitting).

**Important**: Verify the CSV actually contains error-level events before reporting counts. Count rows where `LevelDisplayName` equals "Error" or "Critical" first. If zero error rows exist, report that explicitly — do NOT infer error counts from other sources or total event counts.

Look for rows where `Message` contains `0xC0370103` (HCS_E_PROCESS_ALREADY_STOPPED).

For each matching row:
- Extract the 64-character hex container ID from the message: `\[([a-f0-9]{64})\]`
- Record which Event ID triggered it (from the `Id` column)

**Classify by event type**:
- Event IDs 2002/2003 (Shutdown/Terminate) with 0xC0370103 → container couldn't be stopped cleanly
- Event IDs 2000/2001 (Create/Start) with 0xC0370103 → containerd retrying sandbox creation against stale HCS entries

**Cross-reference with crictl pods**: For Terminate/Shutdown failures, check if the container ID (first 13 chars) appears in crictl pods. Unmatched containers are confirmed zombies.

**Severity for terminate failures**: See common-reference.md for thresholds on unmatched container counts.

For Create/Start failures: always INFO (these are retry attempts, not stuck state).

### 3. Zombie HCS Container Detection

Parse `*-hcsdiag-list.txt`: each line with a 64-character hex ID (`[a-f0-9]{64}`) is an active HCS container.

Cross-reference with crictl pods (first 13 chars of container ID = pod ID prefix).

- Containers in hcsdiag with NO matching crictl pod are zombies
- A few (≤10) is normal
- Many more indicates systemic cleanup failure

### 4. Orphaned Shim Process Detection

Parse `*-shimdiag-list-with-pids.txt`: format `k8s.io-<64-char-hex-id>  <pid>`
- Extract first 13 chars of the hex ID and the PID: `k8s\.io-([a-f0-9]{13})[a-f0-9]*\s+(\d+)`

Cross-reference with crictl pods (first 13 chars).

- Any shim with no matching crictl pod is orphaned — these hold references to stopped pod sandboxes

### 5. Stable PID Detection Across Snapshots (Strongest Zombie Signal)

**This is the most reliable indicator of zombie pods.**

For each snapshot, build a map of `{shim_prefix_13 → pid}`. Then compare across snapshots:

A shim is "stable" (zombie) if:
- The same prefix appears in ≥2 snapshots
- The PID is identical across those snapshots

Healthy pods cycle: shim starts, container runs, shim exits. A shim with the same PID across all snapshots is **permanently stuck**.

- Report: pod name, PID, and how many snapshots it appeared in (e.g., "3/3 snapshots")

**Additional checks**:
- HCS containers that appear in hcsdiag in one snapshot but disappear in a later one (successful cleanup — good signal)
- New zombie containers appearing between snapshots (ongoing leak)
- Correlation between specific pod names and zombie state (same workload always getting stuck)
- Very high PID numbers in shimdiag (suggest many process creation/destruction cycles)
- Shim processes that change PID between snapshots but the pod stays NotReady (cycling but failing)

### 6. Microsoft Defender File Lock Analysis

Defender (MsMpEng.exe) real-time scanning can hold file locks on container layer files during snapshot create/delete operations, preventing containerd from completing container teardown.

**Check Defender status:**
- `processes.txt`: Look for `MsMpEng` — if present, note memory usage and handle count (high handles = active scanning)
- `scqueryex.txt`: Look for `WinDefend` service — is it RUNNING?
- Any `MpCmdRun.log` or Defender-related files in the bundle

**Check for missing Defender exclusions:**
AKS nodes should have these exclusions configured (via `Update-DefenderPreferences` in CSE). Current AKS CSE only excludes **processes** (containerd.exe, kubelet.exe, etc.) but NOT:
- ❌ `C:\ProgramData\containerd\` (snapshot/layer data — Defender scans every file here)
- ❌ `containerd-shim-runhcs-v1.exe` (shim process managing each container)
- ❌ `C:\k\` (kubelet working directory with pod volumes)

Without path exclusions, Defender scans snapshot files during create/delete, adding latency proportional to snapshot count. At 1000+ snapshots, this can push container stop operations past kubelet's grace period.

**Check containerd logs for file lock evidence:**
- `containerd.err-*.log`: Search for:
  - "The process cannot access the file because it is being used by another process"
  - "Access is denied" during remove/delete operations
  - "sharing violation"
  - "failed to remove" or "failed to cleanup" on snapshot paths
  - "context deadline exceeded" during container stop

**Note**: containerd stderr logs are often NOT captured in the diagnostic bundle. If no `containerd.err-*.log` files exist, report this as a diagnostic gap — the absence of lock errors doesn't mean they aren't occurring.

## Findings Format

```markdown
### Pod Termination Findings

<severity> **<LEVEL>** (<confidence> confidence): <description>
  - <detail line 1>
  - <detail line 2>
```

**Example**:
```markdown
🔴 **CRITICAL** (HIGH confidence): containerd/Kubelet reinstalled without clearing HCS state
  - 2026-03-23T03:41:56 — containerd service installed
  - Pre-existing HCS containers become zombies after reinstall

🔴 **CRITICAL** (HIGH confidence): 8 shim process(es) have stable PIDs across 3 snapshots
  - my-pod (PID 4532, 3/3 snapshots); other-pod (PID 6789, 3/3 snapshots)
  - These shims are NOT cycling — zombie pod sandboxes

🟡 **WARNING** (MEDIUM confidence): 120 HCS containers recorded Shutdown/Terminate failures
  - Error 0xC0370103 (HCS_E_PROCESS_ALREADY_STOPPED)
  - 95 of these have no matching crictl pod

🟡 **WARNING** (HIGH confidence): 5 shim process(es) have no matching crictl pod (orphaned)
  - PIDs: 4532, 6789, 1234, 5678, 9012

🟡 **WARNING** (MEDIUM confidence): Microsoft Defender running without containerd data path exclusion
  - MsMpEng.exe active (PID 1234, 500 handles)
  - Missing exclusion: C:\ProgramData\containerd\ — Defender scans all snapshot files
  - containerd.err logs not captured — cannot confirm file locks (diagnostic gap)
```

## Known Patterns

| Pattern | Severity | Confidence | Indicators | Remediation |
|---------|----------|------------|------------|-------------|
| containerd/Kubelet reinstall event | 🔴 CRITICAL | HIGH | "installed in the system" + "containerd"/"Kubelet" in services.csv, followed by 0xC0370103 errors | Drain node before reinstalling; kill zombies with `hcsdiag kill` |
| Stable shim PID across ≥2 snapshots | 🔴 CRITICAL | HIGH | Same prefix+PID in shimdiag across multiple snapshots | `Stop-Process -Id <pid> -Force`; `kubectl delete pod --force --grace-period=0` |
| >50 unmatched HCS containers | 🔴 CRITICAL | HIGH | Terminate/Shutdown 0xC0370103 errors with no crictl pod match | Systemic failure, likely from reinstall; `hcsdiag kill` each zombie |
| 6–50 unmatched HCS containers | 🟡 WARNING | HIGH | Moderate count of 0xC0370103 errors with no crictl match | Monitor; may self-resolve or need manual cleanup |
| 1–5 unmatched HCS containers | 🔵 INFO | MEDIUM | Small count of unmatched containers | Normal on healthy nodes |
| Orphaned shim (no crictl pod) | 🟡 WARNING | HIGH | Shim prefix not found in crictl pods | Holds sandbox references; `Stop-Process` to clean up |
| Defender running + file lock errors + no path exclusions | 🔴 CRITICAL | HIGH | MsMpEng active, "Access is denied"/"sharing violation" in containerd logs | Add path exclusions for `C:\ProgramData\containerd\`, `C:\k\` |
| Defender running + no path exclusions (no log evidence) | 🟡 WARNING | MEDIUM | MsMpEng active, missing exclusions, but no containerd.err logs to confirm | Add path exclusions proactively; logs may not be captured |
| Create/Start 0xC0370103 errors | 🔵 INFO | HIGH | Event IDs 2000/2001 with 0xC0370103 | Retry attempts, not stuck state |

## Cross-References

- **analyze-hcs.md**: Broader HCS operational health; this sub-skill focuses on zombie detection while HCS covers the full event spectrum
- **analyze-containers.md**: Stable shim PIDs → report pod names so container analysis can check if those pods appear as NotReady
- **analyze-disk.md**: Many zombie containers consume resources; may correlate with disk pressure
- **analyze-images.md**: High snapshot count amplifies Defender scanning latency. Root cause chain: mutable tags → dangling images → snapshot bloat → Defender scans more files → container stop slower → grace period exceeded → pods stuck Terminating
- **analyze-memory.md**: Zombie HCS containers consume memory resources
- **Remediation sequence**: (1) `hcsdiag kill <id>` for each zombie, (2) `kubectl delete pod --force --grace-period=0`, (3) `Stop-Process -Id <pid> -Force` for orphaned shims, (4) `Restart-Service containerd`
- **Prevention**: Drain node before reinstalling containerd/kubelet
