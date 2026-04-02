---
name: windows-log-analysis
description: >
  Analyzes Windows AKS node log bundles collected by collect-windows-logs.ps1.
  Use this skill when asked to diagnose a Windows AKS node issue, investigate
  disk pressure, container failures, image accumulation, service crashes,
  network problems, or extension errors on a Windows node.
allowed-tools: shell
---

# Windows AKS Node Log Analysis Skill

## Overview

Windows AKS node log bundles are collected by `staging/cse/windows/debug/collect-windows-logs.ps1`.
Each collection run produces files prefixed with a timestamp in `yyyyMMdd-HHmmss` format. Multiple
snapshots may be present in a single extracted bundle — one per collection run.

This skill uses **sub-skill files** that instruct LLM sub-agents how to analyze each log category.
The sub-skills are in the `sub-skills/` directory relative to this file.

---

## How to Analyze a Log Bundle

### Step 1: Discover the Bundle Structure

List the extracted log directory to identify:
- Available snapshot timestamps (filenames matching `YYYYMMDD-HHMMSS-*`)
- Which file types are present
- Whether Extension-Logs zips/directories exist

### Step 2: Dispatch Sub-Skills

Read `common-reference.md` first — it contains shared encoding/format knowledge and **dispatch guidance** for choosing the right sub-skills based on symptoms.

**Always run** (triage):

| Sub-Skill | What It Covers |
|-----------|---------------|
| `common-reference.md` | Encoding, formats, thresholds, error codes, dispatch guidance |
| `analyze-containers.md` | Container restarts, crash-loops, pod readiness |
| `analyze-services.md` | Windows service health, node versions, OS info |

**Dispatch by symptom** (see common-reference.md § Dispatch Guidance for full table):

| Sub-Skill | When to Run |
|-----------|------------|
| `analyze-termination.md` | Pods stuck Terminating, zombie HCS, orphaned shims |
| `analyze-hcs.md` | HCS operational health, container lifecycle, vmcompute issues |
| `analyze-hns.md` | HNS endpoints, load balancers, CNI/DNS, WFP/VFP analysis |
| `analyze-kubeproxy.md` | Service routing, DSR policies, port range conflicts, SNAT |
| `analyze-images.md` | Dangling images, mutable tags, snapshot bloat, GC failures |
| `analyze-disk.md` | C: drive free space trends |
| `analyze-kubelet.md` | Node conditions, lease renewal, evictions, clock skew, certs |
| `analyze-memory.md` | Physical memory, pagefile, OOM, process memory |
| `analyze-crashes.md` | App crashes, BSODs, WER reports, kernel dumps |
| `analyze-csi.md` | CSI proxy, SMB/Azure Files mount failures, Azure Disk |
| `analyze-gmsa.md` | gMSA/CCG authentication, Kerberos, credential specs |
| `analyze-gpu.md` | GPU health, nvidia-smi, DirectX device plugin |
| `analyze-bootstrap.md` | Node provisioning, CSE flow, bootstrap config |
| `analyze-extensions.md` | Azure VM extension execution errors |

For unknown issues or comprehensive health checks, run all sub-skills in parallel.

### Step 3: Verify and Challenge Findings

Before synthesizing, apply skeptical review to each sub-skill's findings:

1. **Cross-validate**: Does finding A from one sub-skill contradict finding B from another? If so, investigate — one of them is wrong.
2. **Check proportionality**: Is the severity proportionate to the evidence? (e.g., 3 transient errors ≠ CRITICAL)
3. **Verify causal chains**: If claiming "A caused B", confirm timestamps show A preceded B and no other explanation fits better.
4. **Challenge your top diagnosis**: Actively look for evidence it's wrong. What would you expect to see if your diagnosis were correct but don't? What alternative diagnosis fits the same evidence?
5. **Separate observation from inference**: State what you directly observed vs. what you inferred. Mark inferences explicitly.

### Step 4: Synthesize Findings

Combine verified findings from all sub-skills into a unified diagnosis using the decision tree and root cause chains below.

### Step 5: State Overall Confidence

End the report with an explicit confidence assessment:

```markdown
## Confidence Assessment

**Primary diagnosis**: [your diagnosis]
**Confidence**: HIGH / MEDIUM / LOW
**Why this confidence level**: [1-2 sentences explaining what evidence supports it and what gaps remain]
**What would change my mind**: [what evidence, if found, would invalidate this diagnosis]
**What I couldn't verify**: [list any claims that lack full evidence]
```

---

## Synthesis Decision Tree

```
Any CRITICAL in containers?
  ├─ Yes, crash-looping containers
  │    → Check images for dangling images / mutable tags
  │    → Check crashes + memory for OOM or service crashes
  │    → Check disk for pressure causing evictions
  └─ Yes, pods not Ready
       → Check services for service crashes near the failure time
       → Check termination for zombie HCS state

Pods stuck in Terminating?
  → Check termination findings:
       - CRITICAL: containerd/kubelet reinstalled (services)
       - CRITICAL: stable shim PIDs across snapshots
       - CRITICAL/WARNING: HCS terminate failures
       - WARNING: Defender without containerd data path exclusion
       → Check images for snapshot bloat amplifying Defender latency
       → Check hcs for lifecycle completeness

Any CRITICAL in images?
  → Immediate: crictl rmi --prune
  → Root cause: switch to immutable image tags

Any CRITICAL in disk (< 15 GB free)?
  → Check images for dangling image count (most common cause)
  → Check crashes for WER dump accumulation

Any CRITICAL in hns?
  ├─ Endpoint leaks → Check termination for zombie HCS holding endpoints
  ├─ LB count drop → Check services for HNS restart events
  └─ CNI failures → Check kubelet for DiskPressure/MemoryPressure

Any CRITICAL in kubeproxy?
  ├─ Port range conflicts → Check excludedportrange.txt vs NodePort range
  ├─ Stale LB rules → Check hns for LB inventory
  └─ Service unreachable → Check hns for endpoint state

Any CRITICAL in kubelet?
  ├─ NotReady → Check crashes for kubelet crash / BSOD
  ├─ DiskPressure → Check disk + images
  └─ MemoryPressure → Check memory for pagefile/RAM

Any CRITICAL in memory?
  → Check crashes for OOM-triggered crashes
  → Check containers for crash-loops from OOM kills

Any CRITICAL in crashes?
  ├─ BSOD/kernel dump → Escalate to Windows platform team
  └─ containerd/kubelet crash → Check termination for orphaned HCS post-crash

Any CRITICAL in csi?
  → Check kubelet for volume mount timeout correlation
  → Check services for csi-proxy service state

Any CRITICAL in gmsa?
  → Check hcs for credential setup errors
  → Check hns for DNS resolution to domain controllers
  → Check kubelet for clock skew (Kerberos sensitivity)

Any CRITICAL in bootstrap?
  → Check extensions for CSE exit codes
  → Check services for startup ordering failures

Any CRITICAL in gpu?
  → Check kubelet for device plugin registration
  → Check services for GPU-related service state

Any CRITICAL in extensions?
  → Node likely failed to provision — check bootstrap for full timeline
```

---

## Root Cause Chain Tracing

Common root cause chains on Windows AKS nodes:

| Symptom | → Check | → Root Cause |
|---------|---------|-------------|
| Disk pressure | images (dangling count) | Mutable image tags causing accumulation |
| Crash-looping containers | crashes + memory (OOM, service crashes) | Memory exhaustion or service instability |
| Pods stuck Terminating | termination (reinstall, zombies) | containerd reinstalled without draining |
| Node not joining cluster | bootstrap + extensions (exit codes, CSE flow) | Extension download/execution failure |
| High restart counts | disk + memory | Disk pressure causing evictions + OOM |
| DNS resolution failures | hns (endpoints, DNS config) | HNS endpoint leaks or misconfigured DNS |
| SNAT exhaustion | kubeproxy (WFP netevents) | High outbound connection churn |
| Node NotReady | kubelet (conditions, lease renewal) | Lease renewal timeout or kubelet crash |
| Memory exhaustion / OOM | memory (available, pagefile, processes) | Undersized pagefile or memory leak |
| Unexpected reboots | crashes (Event 6008, WER, minidumps) | BSOD, containerd OOM, or Windows Update |
| containerd crash → orphaned containers | crashes + termination | containerd crash without clean shutdown |
| Slow pod termination | termination (Defender) + images | Defender scanning snapshots without path exclusions |
| Service routing broken | kubeproxy + hns (LB policies, endpoints) | Stale HNS policies or kube-proxy sync failure |
| Volume mount failures | csi + kubelet (mount timeouts) | Stale SMB mappings or credential expiry |
| gMSA auth failures | gmsa + hns (DNS) + kubelet (clock) | CCG plugin error, DC unreachable, or clock skew |
| GPU scheduling failures | gpu + kubelet (device plugin) | Driver mismatch or device plugin not registered |

---

## Timeline Correlation

When findings span multiple sub-skills, build a timeline:

1. **Anchor events**: Find the earliest significant event (reboot, service crash, reinstall)
2. **Cascade tracking**: Trace the effect forward in time:
   - Service reinstall → HCS zombies → pods stuck → disk fills
   - OOM event → containerd crash → container restarts → pod not Ready
   - HNS restart → LB policies lost → service unreachable
3. **Timestamp alignment**: Match timestamps across CSV events, snapshot prefixes, and kubectl output
4. **Snapshot comparison**: Use multi-snapshot data to distinguish "always broken" from "recently degraded"

---

## Common Remediations

| Issue | Immediate Fix | Root Cause Fix |
|-------|--------------|----------------|
| Dangling images filling disk | `crictl rmi --prune` | Switch to immutable image tags |
| Pods stuck Terminating | `hcsdiag kill <id>` + force delete pod | Drain before reinstalling containerd |
| Crash-looping container | `kubectl describe pod` + check logs | Fix OOM/resource limits or application bug |
| Extension failure | Re-run CSE or reimage node | Fix network/firewall blocking downloads |
| HNS endpoint leaks | Restart HNS service or drain node | Fix workload churn, investigate HNS bugs |
| Node NotReady (lease) | Restart kubelet, check apiserver connectivity | Fix network path to apiserver |
| Memory pressure / OOM | Kill memory-hungry processes | Increase pagefile, fix memory leaks, set resource limits |
| BSOD / kernel crash | Reboot node, collect dumps | Escalate to Windows platform team with dump files |
| Defender slowing container ops | `Add-MpPreference -ExclusionPath "C:\ProgramData\containerd"` | Update CSE `Update-DefenderPreferences` to include containerd paths |
| Service routing broken | Restart kube-proxy, verify HNS LB state | Fix stale LB policy cleanup, update kube-proxy |
| Volume mount failures | Remove stale SMB mappings: `Remove-SmbGlobalMapping` | Fix credential rotation, update CSI proxy |
| gMSA auth failures | Verify DC connectivity + clock sync | Fix CCG plugin config, ensure DNS to DCs works |
| Port range conflicts | Adjust NodePort range to avoid excluded ranges | Configure service port ranges at cluster level |
