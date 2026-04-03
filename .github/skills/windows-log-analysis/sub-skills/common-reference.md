# Common Reference — Shared Encoding, Formats & Thresholds

## File Encoding

Windows AKS log bundles use mixed encodings:

- **Most PowerShell output files**: UTF-16-LE with BOM (`FF FE` first two bytes)
- **Some newer builds**: plain UTF-8 (with or without BOM `EF BB BF`)
- **Fallback**: latin-1 (ISO 8859-1) for binary-ish files

**How to handle:** Read raw bytes first. Check the first 2–3 bytes for BOM:
- `FF FE` → decode as UTF-16-LE, skip BOM
- `FE FF` → decode as UTF-16-BE, skip BOM
- `EF BB BF` → decode as UTF-8, skip BOM
- Otherwise → try UTF-8, fall back to latin-1

## Snapshot Timestamps

Files are prefixed with timestamps in `YYYYMMDD-HHMMSS` format (e.g., `20260323-034156`).
Multiple snapshots may exist in a single bundle — one per collection run. When analyzing,
process all snapshots to detect trends (disk growth, stable PIDs, escalating restart counts).

Discover timestamps by scanning filenames with pattern: `^(\d{8}-\d{6})`

## CSV Files (Event Logs)

Windows event log CSVs (`*_services.csv`, `*_hyper-v-compute-operational.csv`) have quirks:

- **Encoding**: UTF-16-LE with BOM (see above)
- **Embedded newlines**: The `Message` field often contains embedded newlines. You MUST use proper CSV parsing (e.g., `csv.DictReader`) — splitting on newlines will corrupt records.
- **#TYPE header**: Some CSVs start with a `#TYPE Selected.Microsoft.Management.Infrastructure.CimInstance` line before the real CSV header. Skip lines starting with `#TYPE`.
- **Key columns**: `TimeCreated`, `Id` (event ID), `LevelDisplayName` (Error/Critical/Warning/Information), `Message`

## crictl Output Parsing

`crictl` commands produce variable-width columnar output:

- **`crictl ps -a`** (containers): `CONTAINER  IMAGE  CREATED  STATE  NAME  ATTEMPT  POD_ID  POD`
  - `CREATED` is variable width ("19 seconds ago", "About an hour ago")
  - Parse columns from the **right end** where they are stable: POD, POD_ID, ATTEMPT, NAME, STATE
  
- **`crictl pods`** (pods): `POD_ID  CREATED  STATE  NAME  NAMESPACE  ATTEMPT  RUNTIME`
  - Same variable-width CREATED field
  - Parse from right: RUNTIME, ATTEMPT, NAMESPACE, NAME, STATE

- **`crictl images`** (images): `IMAGE  TAG  IMAGE_ID  SIZE`
  - Straightforward columns; skip header line starting with "IMAGE"
  - Dangling images have tag `<none>`

## containerd imageFsInfo JSON

`crictl imagefsinfo` output differs between containerd versions:

- **containerd 1.x**: `{"status": {"usedBytes": {"value": "12345"}}}`
- **containerd 2.x**: `{"status": {"imageFilesystems": [{"usedBytes": {"value": "12345"}}]}}`

Check for `status.usedBytes.value` first, then fall back to `status.imageFilesystems[].usedBytes.value`.
As a last resort, use regex: `"usedBytes"\s*:\s*\{\s*"value"\s*:\s*"(\d+)"`

**Important**: Windows reports sparse-file-inflated sizes (often 5–10× actual disk usage).

## Severity Levels

| Icon | Level | Meaning |
|------|-------|---------|
| 🔴 | CRITICAL | Immediate action required |
| 🟡 | WARNING | Degraded health or risk of future failure |
| 🔵 | INFO | Contextual data (versions, baselines) |

## Confidence Levels

| Level | Meaning |
|-------|---------|
| HIGH | Pattern matches known failure mode exactly, verified with corroborating evidence, no counter-evidence found |
| MEDIUM | Strong indicators but could have alternative explanations; state what those alternatives are |
| LOW | Anomalous but may be benign; flag for human review with reasoning |

## Verification Protocol

**Every finding at WARNING or CRITICAL must be verified before reporting.** Do not report a pattern match without checking whether it's real.

### Step 1: Verify the Claim

Before reporting any finding, confirm it with direct evidence from the logs:

- **Count verification**: If claiming "X errors found", show the actual count. Read the raw data — don't trust summaries or headers.
- **Timestamp verification**: If claiming "X happened at time T", quote the actual log line with the timestamp.
- **Cross-file verification**: If claiming a causal relationship (A caused B), verify timestamps show A before B.
- **Threshold verification**: If claiming a value exceeds a threshold, state both the actual value and the threshold.

### Step 2: Look for Counter-Evidence

Actively search for evidence that your finding is **wrong** or **misleading**:

- **False positive check**: Could this pattern appear in a healthy system? (e.g., a few transient errors during normal operation)
- **Alternative explanations**: What else could cause this pattern? List at least one alternative.
- **Scope check**: Is this a real problem or just noise? (e.g., 3 errors out of 10,000 operations is 0.03% — probably fine)
- **Context check**: Does surrounding context contradict the finding? (e.g., error log entries followed by successful retry)

### Step 3: State Your Confidence Reasoning

For each WARNING or CRITICAL finding, include a brief explanation of **why** you assigned that confidence level:

```
🔴 **CRITICAL** (HIGH confidence): Node condition Ready=False since 2026-03-23T03:30:00Z
  - Evidence: kubectl-describe-nodes.log line 42: "Ready False ... LastTransitionTime: 2026-03-23T03:30:00Z"
  - Corroboration: kubelet.log shows lease renewal failures starting 03:29:45Z
  - Counter-evidence considered: No subsequent Ready=True transition found in any snapshot
  - Confidence reasoning: Direct observation in two independent sources, no contradicting evidence
```

```
🟡 **WARNING** (MEDIUM confidence): 120 HCS Shutdown failures recorded
  - Evidence: hyper-v-compute-operational.csv contains 120 rows with EventId 2002
  - Counter-evidence: CSV Level column shows "Information" not "Error" for most rows — these may be normal lifecycle events, not failures
  - Alternative explanation: Normal container churn produces Shutdown events; only Error-level rows indicate real failures
  - Confidence reasoning: Pattern matches known issue but evidence is ambiguous without Level column validation
```

### Step 4: Report What You Couldn't Determine

If logs are missing, truncated, or ambiguous, say so explicitly:

```
⚠️ **Unable to determine**: Whether Defender is scanning containerd data directory
  - Would need: MpPreference export or real-time Defender scan logs (not in bundle)
  - Impact if true: Could explain slow container teardown
```

### Common False Positive Traps

| Trap | Why It's Wrong | How to Verify |
|------|---------------|---------------|
| CSV row count = error count | CSV contains ALL events (Info, Warning, Error) | Check the `Level` column for actual severity |
| High container restart count | Could be by-design short-lived containers (exit code 0) | Check exit codes — code 0 restarts are intentional |
| "Failed" in log line | May be followed by successful retry | Check subsequent lines for success |
| Large snapshot count | Normal for nodes with many images | Compare against image count — snapshots ≈ 2× images is normal |
| Service in "demand start" mode | May be intentional for optional services | Check if it's a critical AKS service (see table below) |
| Dangling images present | Small numbers are normal GC lag | Only flag if >20 dangling or growing across snapshots |

## Standard Thresholds

### Disk

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| C: drive free space | ≥30 GB | <30 GB | <15 GB |

### Container Images & Snapshots

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| Dangling images | <5 | ≥5 | ≥20 |
| containerd snapshots | <500 | ≥500 | ≥1000 |

### Container Health

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| Container restart attempts | 1–9 | — | ≥10 (crash-looping) |

### HCS & Termination

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| HCS terminate failures (unmatched) | 1–5 | 6–50 | >50 |
| Orphaned shims | — | any | stable PID across ≥2 snapshots |
| HCS Create-to-Start duration | <30s | 30–120s | >120s |
| HCS Shutdown-to-Terminate duration | — | <30s (graceful failed fast) | >60s (HCS struggled) |
| HCS error rate per operation type | <5% | 5–20% | >20% |
| HCS creates per 5-minute window | <20 | 20–50 | >50 (creation storm) |
| vmcompute working set (memory) | <150 MB | >500 MB | >1 GB |
| vmcompute handle count | — | >5000 | — |

### Memory

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| Available physical RAM | ≥2 GB | <2 GB | <500 MB |
| Pagefile (manual) size | ≥1024 MB | <1024 MB | — |
| Pagefile current usage | — | peak >80% of allocated | >90% of allocated |
| Commit charge vs limit | — | within 25% of limit | within 10% of limit |
| Single process working set | — | containerd/kubelet >1 GB | any process >4 GB |

### kube-proxy & Port Exhaustion

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| kube-proxy sync delay after restart | <5 min | 5–30 min | >30 min |
| Excluded port ranges count | — | >20 | overlaps NodePort 30000–32767 |
| Available ephemeral ports | >5000 | <5000 | <1000 or "Couldn't reserve" |
| Stale LB deletions per sync cycle | — | ongoing beyond startup | >50 per cycle |

### GPU

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| GPU temperature | <80°C | 80–90°C | >90°C |
| GPU memory usage | — | >95% | — |
| Power usage | — | at/exceeding cap | — |
| Uncorrectable ECC errors | 0 | — | >0 |
| Single-bit ECC errors (aggregate) | — | >100 | — |
| Retired GPU memory pages | 0 | >0 but below limit | approaching limit (~48) |

### CSI Proxy

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| CSI proxy working set | <50 MB | >200 MB | — |
| CSI proxy service state | RUNNING | — | STOPPED |

### Bootstrap (CSE)

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| CSE exit code | 0 | — | any non-zero (see analyze-bootstrap.md for full code table) |
| Download step duration | <2 min | >5 min | — |

## HCS Error Codes

| Code | Name | Meaning |
|------|------|---------|
| `0x80370100` / `0xC0370100` | `HCS_E_TERMINATED_DURING_START` | Container crashed during startup |
| `0x80370101` / `0xC0370101` | `HCS_E_IMAGE_MISMATCH` | OS version mismatch between container image and host |
| `0xC0370103` / `0x8037011F` | `HCS_E_PROCESS_ALREADY_STOPPED` | Container process already exited but HCS didn't clean up — zombie state |
| `0x80370106` / `0xC0370106` | `HCS_E_UNEXPECTED_EXIT` | Container exited unexpectedly |
| `0x80370109` / `0xC0370109` | `HCS_E_CONNECTION_TIMEOUT` | HCS operation timed out — vmcompute overloaded |
| `0x8037010E` / `0xC037010E` | `HCS_E_SYSTEM_NOT_FOUND` | Containerd referencing unknown container — race condition |
| `0x8037010F` | `HCS_E_SYSTEM_ALREADY_EXISTS` | Stale HCS state from previous containerd instance |
| `0x80370110` / `0xC0370110` | `HCS_E_SYSTEM_ALREADY_STOPPED` | Stopping already-stopped container — usually benign |
| `0x80370118` / `0xC0370118` | `HCS_E_OPERATION_TIMEOUT` | Operation exceeded internal timeout — HCS performance issue |
| `0x8037011E` / `0xC037011E` | `HCS_E_SERVICE_DISCONNECT` | vmcompute crashed/restarted — all containers affected |
| `0x800705AA` | Insufficient system resources | Resource exhaustion creating network compartments |

**Note**: Error codes appear in both HRESULT (`0x8037xxxx`) and NTSTATUS (`0xC037xxxx`) forms in logs. Match both patterns.

## Windows Event IDs

| Event ID | Source | Meaning |
|----------|--------|---------|
| 2000 | Hyper-V Compute | Create compute system |
| 2001 | Hyper-V Compute | Start compute system |
| 2002 | Hyper-V Compute | Shut down compute system |
| 2003 | Hyper-V Compute | Terminate compute system |
| 2003 | Resource Exhaustion Detector | Resource exhaustion detected |
| 2004 | System | Low memory condition |
| 6008 | System | Unexpected shutdown (preceding crash/BSOD) |

**Note:** Windows Event IDs are **not globally unique** — the same numeric ID (e.g., `2003`) can appear under different providers with different meanings. When writing analysis logic, always key on **Event ID + Source** together.
**CCG (Container Credential Guard) Events** — used by gMSA:

| Event ID | Source | Meaning |
|----------|--------|---------|
| 1 | CCG | Credential retrieval success |
| 2 | CCG | Credential retrieval failure |

## Critical Windows Services

These services are expected to be **Running** on a healthy AKS Windows node:

| Service Name | Display Name | Purpose |
|--------------|-------------|---------|
| `containerd` | containerd | Container runtime |
| `kubelet`    | kubelet | Kubernetes node agent |
| `kubeproxy`  | kube-proxy | Service routing / HNS LB programming |
| `csi-proxy`  | CSI Proxy | CSI volume operations (named pipes)  — tar/package name: `csiproxy` |
| `vmcompute`  | Hyper-V Host Compute Service | HCS — container lifecycle |
| `hns`        | Host Network Service | Container networking |
| `WinDefend`  | Windows Defender | Antimalware (may impact container performance) |

Additional services on specific configurations:
- `nvlddmkm` — NVIDIA display driver (GPU nodes)
- `sshd` — OpenSSH (if enabled)

## Mutable Tags

These image tags are considered mutable (cause dangling image accumulation):
`latest`, `development`, `main`, `master`, `edge`, `nightly`, `snapshot`

## shimdiag / hcsdiag Format

- **`hcsdiag list`**: Each line contains a 64-character hex container ID, optionally followed by status
- **`shimdiag list --pids`**: Format `k8s.io-<64-char-id>  <pid>` — the first 13 chars of the ID can be used to correlate with crictl pod IDs

## NVIDIA Xid Error Classification

| Xid Code | Name | Severity | Meaning |
|----------|------|----------|---------|
| 13 | Graphics Engine Exception | 🟡 WARNING | Application-level GPU fault — usually recoverable |
| 31 | GPU memory page fault | 🟡 WARNING | Application accessing invalid GPU memory |
| 38 | Driver firmware error | 🔴 CRITICAL | Firmware failure — may need driver reinstall |
| 43 | GPU stopped processing | 🟡 WARNING | Application hang on GPU |
| 48 | Double-bit ECC error | 🔴 CRITICAL | Uncorrectable memory error — hardware failure |
| 64 | ECC page retirement limit | 🔴 CRITICAL | Too many retired memory pages — GPU replacement |
| 79 | GPU has fallen off the bus | 🔴 CRITICAL | PCIe bus failure — hardware/power issue |
| 92 | High single-bit ECC error rate | 🟡 WARNING | Memory degrading — monitor |
| 94 | Contained ECC error | 🟡 WARNING | ECC error contained, no data loss |
| 95 | Uncontained ECC error | 🔴 CRITICAL | ECC error not contained — data corruption possible |

## File Discovery Patterns

This table maps log file patterns to the sub-skill(s) that analyze them. Use it to dispatch the right sub-skills based on which files are present in a bundle.

| File Pattern | Sub-Skill(s) |
|-------------|-------------|
| `<ts>-cri-containerd-containers.txt` | analyze-containers |
| `<ts>-cri-containerd-pods.txt` | analyze-containers, analyze-termination, analyze-hcs |
| `<ts>-cri-containerd-images.txt` | analyze-images |
| `<ts>-cri-containerd-imageFsInfo.txt` | analyze-images |
| `<ts>-disk-usage-all-drives.txt` | analyze-disk, analyze-images |
| `<ts>_hyper-v-compute-operational.csv` | analyze-hcs, analyze-termination, analyze-crashes |
| `<ts>_services.csv` | analyze-services, analyze-crashes, analyze-memory, analyze-termination, analyze-kubeproxy |
| `<ts>-hcsdiag-list.txt` | analyze-hcs, analyze-termination |
| `<ts>-shimdiag-list-with-pids.txt` | analyze-termination, analyze-hcs |
| `<ts>_pagefile.txt` | analyze-memory |
| `<ts>-hnsdiag-list.txt` | analyze-hns, analyze-kubeproxy |
| `<ts>-aks-info.log` | analyze-bootstrap, analyze-memory, analyze-gpu |
| `<ts>-containerd-info.txt` | analyze-hcs |
| `<ts>-containerd-toml.txt` | analyze-hcs, analyze-images |
| `processes.txt` | analyze-memory, analyze-hcs, analyze-csi, analyze-termination |
| `scqueryex.txt` | analyze-services, analyze-hcs, analyze-csi, analyze-kubeproxy |
| `available-memory.txt` | analyze-memory |
| `kubelet.log` / `kubelet.err.log` | analyze-kubelet, analyze-csi, analyze-gpu |
| `kubeproxy.log` / `kubeproxy.err.log` | analyze-kubeproxy |
| `csi-proxy.log` / `csi-proxy.err.log` | analyze-csi |
| `containerd.err-*.log` | analyze-termination, analyze-images |
| `*nvidia-smi*` | analyze-gpu |
| `excludedportrange.txt` | analyze-kubeproxy |
| `reservedports.txt` | analyze-kubeproxy |
| `wfp/netevents.xml` | analyze-kubeproxy, analyze-hns |
| `windowsnodereset.log` | analyze-bootstrap, analyze-crashes |
| `CustomDataSetupScript.log` | analyze-bootstrap |
| `CSEResult.log` | analyze-bootstrap |
| `Extension-Logs*` | analyze-extensions |
| `WER-*.zip` | analyze-crashes |
| `Minidump-*.zip` / `MemoryDump-*.zip` | analyze-crashes |
| `bootstrap-config` | analyze-bootstrap |
| `*-ccg-*.evtx` or CCG event logs | analyze-gmsa |
| `gmsa-*.log` or gMSA credential spec files | analyze-gmsa |
| `kubectl-describe-nodes.log` | analyze-gpu, analyze-kubelet |
| `*-aks-info.log` | analyze-kubelet |
| `*-ctr-logs/` (containerd snapshot dirs) | analyze-images |

## Dispatch Guidance

When analyzing a log bundle, don't run all sub-skills. Use these patterns:

### Always Run (Triage)
- analyze-containers — immediate pod/container health
- analyze-services — holistic service state + version info

### Symptom-Based Dispatch

| Reported Symptom | Run These Sub-Skills |
|-----------------|---------------------|
| Pods stuck Terminating | termination, hcs, images |
| Disk pressure / low space | disk, images |
| Pod crash-looping | containers, crashes, memory |
| Node NotReady | kubelet, services, crashes |
| Network/DNS issues | hns, kubeproxy |
| Volume mount failures | csi, kubelet |
| gMSA/AD auth failures | gmsa, hcs |
| GPU not working | gpu, kubelet |
| Node won't join cluster | bootstrap, extensions, services |
| Slow container operations | hcs, images, termination |
| Service routing broken | kubeproxy, hns |
| Memory pressure / OOM | memory, crashes, containers |
| Node rebooted unexpectedly | crashes, services, kubelet |
| Container image pull issues | images, disk |
| CSI / volume errors | csi, kubelet, disk |
| kube-proxy crash loop | kubeproxy, services, hns |

### Full Analysis

For unknown issues or comprehensive health checks, run all sub-skills in parallel.
