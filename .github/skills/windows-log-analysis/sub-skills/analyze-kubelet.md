# Analyze Kubelet — Kubelet Health & Node Conditions Sub-Skill

## Purpose

Detect kubelet health issues, node condition problems (DiskPressure, MemoryPressure, NotReady), eviction signals, lease renewal failures, volume mount errors, and pod scheduling issues on Windows AKS nodes.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `kubectl-describe-nodes.log` | UTF-8 | `kubectl describe node` output |
| `<ts>-aks-info.log` | UTF-16-LE with BOM | `kubectl describe node` + node YAML (allocatable, capacity, conditions) |
| `kubelet.log` | UTF-8 | Kubelet stdout logs (if present) |
| `kubelet.err.log` | UTF-8 | Kubelet stderr logs (if present) |
| `<ts>-cri-containerd-pods.txt` | UTF-16-LE with BOM | `crictl pods` — cross-reference pod state |
| `*_services.csv` | UTF-8 | Service status timeline used for kubelet crash/restart and clock skew checks |

## Analysis Steps

### 1. Node Conditions (`*-aks-info.log`, `kubectl-describe-nodes.log`)

Parse the `kubectl describe node` section for `Conditions:` block.

Look for:
- `Ready` condition: should be `True`. Any other value means node is unhealthy.
- `DiskPressure`: `True` means disk eviction threshold breached
- `MemoryPressure`: `True` means memory eviction threshold breached
- `PIDPressure`: `True` means PID exhaustion
- `NetworkUnavailable`: `True` means network not configured

- 🔴 CRITICAL: `Ready=False` or `Ready=Unknown` — node is NotReady
- 🔴 CRITICAL: `DiskPressure=True` or `MemoryPressure=True` — active eviction
- 🟡 WARNING: `NetworkUnavailable=True`
- 🔵 INFO: All conditions normal

Also extract from node YAML:
- `allocatable` resources (CPU, memory, pods)
- `capacity` resources
- Taints (especially `node.kubernetes.io/not-ready`, `node.kubernetes.io/unreachable`)

### 2. Lease Renewal Issues (kubelet logs)

If `kubelet.log` or `kubelet.err.log` exists, search for:
- `"failed to renew lease"` or `"lease renewal"` — lease renewal timeout
- `"node not found"` — kubelet cannot find its own node object
- `"use of closed network connection"` — apiserver connectivity loss

- 🔴 CRITICAL: Repeated lease renewal failures (causes false NotReady transitions)
- 🟡 WARNING: Intermittent lease renewal warnings

### 3. Volume Mount Errors (kubelet logs, describe node)

In kubelet logs, search for:
- `"MountVolume"` + `"failed"` or `"timed out"` — volume mount failures
- `"UnmountVolume"` + `"failed"` — volume unmount failures
- `"FailedMount"` in describe node events section

- 🔴 CRITICAL: Volume mount timeouts preventing pod startup
- 🟡 WARNING: Intermittent volume mount errors

### 4. Eviction Signals (kubelet logs, describe node)

Search for:
- `"eviction"` or `"Evicted"` in describe node events
- `"eviction manager"` in kubelet logs
- `"threshold"` + `"met"` — eviction threshold reached

- 🔴 CRITICAL: Active evictions occurring (pods being killed)
- 🟡 WARNING: Eviction thresholds approaching

### 5. Kubelet Crash-Restart Cycles (`*_services.csv`)

Search services.csv for kubelet service events:
- Service start/stop patterns indicating crash-restart cycles
- Rapid restart sequences (multiple starts within minutes)

- 🔴 CRITICAL: Kubelet restarting repeatedly (>3 restarts in 30 minutes)
- 🟡 WARNING: Kubelet restarted once or twice

### 6. Pod Scheduling Cross-Reference (`*-cri-containerd-pods.txt`)

Cross-reference pods in NotReady/Error state with node conditions:
- If node has DiskPressure, pods may be evicted
- If node is NotReady, new pods won't be scheduled

### 7. Clock Skew Detection

Compare timestamps across different log files to detect time drift:

- Extract timestamps from kubelet logs and compare against system event log timestamps (`*_services.csv`) for events at similar real-world times
- If timestamps differ by >30 seconds between sources, flag clock skew
- Check `*_services.csv` for W32Time-related errors (Event source "Microsoft-Windows-Time-Service" or "W32Time")
- Check for `"time is not synchronized"` or `"NtpClient"` errors in system events

**Impact of clock skew**:
- >5 minutes: Kerberos authentication fails (relevant for gMSA workloads)
- >1 minute: Certificate validation may fail, API server requests rejected
- Any drift: Log correlation becomes unreliable

- 🔴 CRITICAL: Clock skew >5 minutes — Kerberos auth will fail, certs may be rejected
- 🟡 WARNING: Clock skew >30 seconds — log correlation degraded, cert validation at risk
- 🔵 INFO: W32Time service errors detected (even if skew not directly measurable)

### 8. Certificate Rotation Failures

Search kubelet logs for certificate-related errors:

- `"certificate has expired"` or `"x509: certificate has expired"` — expired cert
- `"certificate signed by unknown authority"` — CA trust issue
- `"tls: handshake failure"` or `"TLS handshake error"` — TLS negotiation failed
- `"failed to rotate"` + `"certificate"` — cert rotation mechanism failed
- `"CSR"` + `"denied"` or `"not approved"` — certificate signing request rejected

Certificate rotation failures cause kubelet to lose API server connectivity, leading to NotReady.

- 🔴 CRITICAL: Certificate expired or rotation failing — kubelet will lose API server access
- 🟡 WARNING: TLS handshake errors (intermittent — may self-resolve on rotation)
- 🔵 INFO: Successful certificate rotation events

### 9. Kerberos Authentication Failures (gMSA)

If clock skew >5 minutes is detected (step 7), flag as likely Kerberos failure:

- Kerberos protocol requires clocks synchronized within 5 minutes (MaxClockSkew)
- gMSA workloads use Kerberos for domain authentication — clock skew breaks all gMSA containers
- Search kubelet logs for `"gMSA"` or `"credential spec"` errors that may correlate with time issues

- 🔴 CRITICAL: Clock skew >5 min + gMSA workloads present → Kerberos authentication broken
- 🟡 WARNING: Clock skew >5 min (Kerberos risk even if gMSA not confirmed)

## Findings Format

```markdown
### Kubelet & Node Condition Findings

🔴 **CRITICAL** (HIGH confidence): Node condition Ready=False since 2026-03-23T03:30:00Z
  - Last transition from True → False
  - Taint applied: node.kubernetes.io/not-ready:NoSchedule

🔴 **CRITICAL** (MEDIUM confidence): Repeated lease renewal failures in kubelet.log
  - 45 "failed to renew lease" entries over 15 minutes
  - Likely cause of NotReady transition

🟡 **WARNING** (HIGH confidence): DiskPressure=True
  - Eviction threshold breached, pods may be evicted
```

## Known Patterns

| Pattern | Severity | Confidence | Meaning |
|---------|----------|------------|---------|
| Ready=False or Ready=Unknown | 🔴 CRITICAL | HIGH | Node is NotReady, workloads impacted |
| DiskPressure=True | 🔴 CRITICAL | HIGH | Disk eviction threshold breached |
| MemoryPressure=True | 🔴 CRITICAL | HIGH | Memory eviction threshold breached |
| Repeated lease renewal failures | 🔴 CRITICAL | MEDIUM | apiserver connectivity issues causing false NotReady |
| Volume mount timeouts | 🔴 CRITICAL | HIGH | Pods cannot start due to volume issues |
| Kubelet restarting >3 times in 30min | 🔴 CRITICAL | HIGH | Kubelet crash loop |
| x509 certificate expired | 🔴 CRITICAL | HIGH | Kubelet cannot authenticate to API server |
| Certificate rotation failing | 🔴 CRITICAL | HIGH | Kubelet will lose API server access when current cert expires |
| Clock skew >5 minutes | 🔴 CRITICAL | HIGH | Kerberos auth fails, cert validation unreliable |
| Clock skew >5 min + gMSA workloads | 🔴 CRITICAL | HIGH | All gMSA containers broken |
| Intermittent lease warnings | 🟡 WARNING | MEDIUM | Transient apiserver connectivity |
| NetworkUnavailable=True | 🟡 WARNING | MEDIUM | Network plugin not initialized |
| Single kubelet restart | 🟡 WARNING | LOW | May be routine update or transient issue |
| TLS handshake errors (intermittent) | 🟡 WARNING | MEDIUM | May resolve on cert rotation |
| Clock skew >30 seconds | 🟡 WARNING | MEDIUM | Log correlation degraded |
| W32Time service errors | 🔵 INFO | MEDIUM | Time sync issues — check for downstream impact |

## Cross-References

- **analyze-disk.md**: DiskPressure condition correlates with C: drive free space findings
- **analyze-memory.md**: MemoryPressure correlates with available memory and pagefile findings
- **analyze-containers.md**: NotReady node explains pod failures and restart loops
- **analyze-termination.md**: Kubelet restart can cause orphaned HCS containers
- **analyze-services.md**: Kubelet service events in services.csv provide crash timestamps
- **analyze-services.md**: W32Time service state in scqueryex.txt — if STOPPED, clock skew is likely; kubelet/containerd service states validate crash-loop detection
