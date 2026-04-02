# Analyze Containers — Container & Pod Health

## Purpose

Detect container restarts, crash-looping containers, and pods not in Ready state on Windows AKS nodes.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `<ts>-cri-containerd-containers.txt` | UTF-16-LE with BOM | `crictl ps -a` output (all containers) |
| `<ts>-cri-containerd-pods.txt` | UTF-16-LE with BOM | `crictl pods` output (all pods) |

Analyze the **latest snapshot** by default.

## Analysis Steps

### 1. Container Restart Analysis

Parse `*-cri-containerd-containers.txt` (crictl ps -a output).

**Parsing**: The CREATED field is variable-width ("19 seconds ago", "About an hour ago"). Parse columns from the **right end**:
- `parts[-1]` = POD name
- `parts[-3]` = ATTEMPT (restart count)
- `parts[-4]` = NAME (container name)
- `parts[-5]` = STATE

Skip header line (starts with "CONTAINER") and lines with fewer than 5 columns.

**What to report**:
- Any container with ATTEMPT > 0 has restarted
- Sort by attempt count descending
- See common-reference.md for severity thresholds

**Additional checks**:
- Containers in `Exited` state that were never restarted (single failure — may indicate a different root cause than crash-looping)
- All containers of a specific pod failing (vs just one sidecar) — suggests pod-level or node-level issue
- Very high restart counts (hundreds) suggesting persistent infrastructure issue vs application bug
- Patterns in container names suggesting system components failing (e.g., kube-proxy, azure-cni)

### 2. Pod Readiness Analysis

Parse `*-cri-containerd-pods.txt` (crictl pods output).

**Parsing**: Same variable-width CREATED issue. Parse from right:
- `parts[-2]` = ATTEMPT
- `parts[-3]` = NAMESPACE
- `parts[-4]` = NAME
- `parts[-5]` = STATE

**What to report**:
- Any pod not in `Ready` state (state is Error, Unknown, NotReady, etc.)
- Pods with non-zero ATTEMPT but currently Ready (recovered from past issues)
- Pods in unusual states not covered above

## Findings Format

```markdown
### Container & Pod Health Findings

<severity> **<LEVEL>** (<confidence> confidence): <description>
  - <detail line 1>
  - <detail line 2>
```

**Example**:
```markdown
🔴 **CRITICAL** (HIGH confidence): 3 container(s) with restart history, 2 crash-looping (≥10 restarts)
  - my-app in my-pod-abc (attempt=45, state=Running)
  - sidecar in my-pod-abc (attempt=12, state=Exited)
  - helper in other-pod (attempt=3, state=Running)

🔴 **CRITICAL** (HIGH confidence): 1 pod(s) not in Ready state
  - broken-pod (kube-system) state=NotReady attempt=5
```

## Known Patterns

| Pattern | Severity | Confidence | Indicators | Remediation |
|---------|----------|------------|------------|-------------|
| Container ATTEMPT ≥ 10 | 🔴 CRITICAL | HIGH | Crash-looping container with high restart count | Check container logs, OOM events, application config |
| Container ATTEMPT 1–9 | 🟡 WARNING | HIGH | Container has restarted but not yet crash-looping | Monitor; check for transient errors |
| Pod not in Ready state | 🔴 CRITICAL | HIGH | Pod STATE is Error, Unknown, or NotReady | Check HCS zombie state, container logs, node conditions |
| Pod non-zero ATTEMPT but Ready | 🔵 INFO | HIGH | Pod recovered from past issues | No action needed; note for historical context |
| All containers in a pod failing | 🔴 CRITICAL | MEDIUM | Every container in a pod is Exited/failing | Likely pod-level or node-level issue, not app bug |
| System component container failing | 🔴 CRITICAL | HIGH | kube-proxy, azure-cni, or other system container restarting | Node-level issue; check system events and networking |

## Cross-References

- **analyze-termination.md**: Pods stuck in non-Ready state may be related to HCS zombie issues or stuck termination
- **analyze-disk.md**: Disk pressure causes pod evictions and restart loops
- **analyze-images.md**: If the failing container uses a mutable tag, the image may have changed
- **analyze-hcs.md**: Container names and pod names help correlate with shimdiag/hcsdiag findings
- **analyze-memory.md**: Check system events for OOM (Event ID 2004) if containers are crash-looping
