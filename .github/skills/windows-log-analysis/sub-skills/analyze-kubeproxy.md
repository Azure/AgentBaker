# Analyze kube-proxy — Windows kube-proxy & Service Routing Sub-Skill

## Purpose

Detect kube-proxy health issues on Windows nodes: HNS load balancer policy sync failures, DSR mode degraded policies, stale load balancer rules, excluded/reserved port range conflicts with NodePort services, service unreachability from policy programming gaps, and kube-proxy crash/restart patterns. Complements `analyze-hns.md` which inspects HNS data plane state — this sub-skill focuses on kube-proxy's control plane behavior and its interaction with HNS.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `kubeproxy.log` or `*kube-proxy*.log` | UTF-8 | kube-proxy stdout logs — service sync, LB policy operations |
| `kubeproxy.err.log` or `*kube-proxy*.err.log` | UTF-8 | kube-proxy stderr logs — errors, warnings |
| `excludedportrange.txt` | UTF-16-LE with BOM | `netsh interface ipv4 show excludedportrange` output — OS-reserved port ranges |
| `reservedports.txt` | UTF-16-LE with BOM | Summary of reserved port blocks and available ephemeral range |
| `wfp/netevents.xml` | UTF-8 or UTF-16-LE | WFP network events — SNAT/drop analysis |
| `<ts>-hnsdiag-list.txt` | UTF-16-LE with BOM | `hnsdiag list all` — cross-reference LB state |
| `<ts>_services.csv` | UTF-16-LE with BOM, CSV | Service events — kube-proxy service start/stop/crash |
| `scqueryex.txt` | UTF-16-LE with BOM | `sc queryex` output — kube-proxy service state and PID |

**Process ALL snapshots** — cross-snapshot comparison reveals policy programming progress, stale rule accumulation, and crash-restart patterns.

## Analysis Steps

### 1. kube-proxy Log Error Classification (`kubeproxy.log`, `kubeproxy.err.log`)

Parse kube-proxy logs for error and warning patterns. kube-proxy on Windows uses the `winkernel` proxier which programs HNS load balancer policies instead of iptables.

**Critical error patterns:**

| Error Pattern | Meaning |
|--------------|---------|
| `Unable to find HNS Network specified by` | kube-proxy cannot find HNS network — networking is not initialized (k/k#79515, k/k#74435) |
| `Network Id not found` | Same as above — HNS network missing or wrong KUBE_NETWORK env |
| `failed to create loadbalancer` or `HNS` + `loadbalancer` + `failed` | LB policy creation failure — service will be unreachable |
| `The specified port already exists` | Stale HNS policy blocking new policy creation (k/k#68923) |
| `Local endpoint not found` | kube-proxy can't find HNS endpoint for a pod — state desync |
| `ModifyLoadBalancer` + error | LB update failure — VIP/DIP mismatch (k/k#131466) |
| `hnsCallTimeout` or `timeout` + `hns` | HNS service unresponsive — possible deadlock |
| `proxier sync failed` or `syncProxyRules` + `error` | Full sync cycle failed — all policies may be stale |

- 🔴 CRITICAL: `Unable to find HNS Network` — kube-proxy non-functional, no service routing on this node
- 🔴 CRITICAL: Repeated `failed to create loadbalancer` — services not being programmed
- 🔴 CRITICAL: HNS timeout errors — HNS service hung, all policy operations blocked
- 🟡 WARNING: `The specified port already exists` — stale policy conflict (k/k#68923)
- 🟡 WARNING: `Local endpoint not found` — kube-proxy state out of sync with HNS
- 🟡 WARNING: `ModifyLoadBalancer` errors — LB updates failing

**Informational patterns:**

| Log Pattern | Meaning |
|------------|---------|
| `Adding new service port` | Service discovery working — normal operation |
| `Synced policies for service` | Policy programming successful |
| `Deleting stale load balancer` | Cleanup of old rules — may be normal or symptom of k/k#112836 |
| `Using WinKernel proxier` | Confirms Windows kernelspace mode |
| `flag --enable-dsr` or `WinDSR` | DSR mode enabled |

### 2. Stale Load Balancer Rule Detection (`kubeproxy.log`)

Search for patterns indicating stale HNS load balancer rule accumulation (kubernetes/kubernetes#112836).

**Detection method:**
- Count `Deleting stale load balancer` or `Stale` + `loadbalancer` messages
- High frequency of stale LB deletions after pod churn = active bug
- Cross-reference with HNS sub-skill: compare kube-proxy's expected LB count against hnsdiag LB count
- After kube-proxy restart, expect a burst of policy recreation — sustained stale rules after stabilization is a bug

- 🔴 CRITICAL: >50 stale LB rule deletions in a single sync cycle — massive rule accumulation (k/k#112836)
- 🟡 WARNING: Ongoing stale LB deletions beyond initial startup — rules not being cleaned properly
- 🔵 INFO: Stale LB cleanup during startup only (normal after restart)

### 3. DSR Mode Detection and Degraded Policy Analysis (`kubeproxy.log`)

Check if DSR (Direct Server Return) mode is enabled and functioning:

**DSR detection:**
- Search for `enable-dsr=true`, `--enable-dsr`, or `WinDSR` in logs
- DSR mode uses HNS load balancers with `DSR` flag — more efficient but has known issues

**DSR degraded policy detection (microsoft/Windows-Containers#79):**
- When DSR LB policies are deleted before the HNS network is torn down, policies can enter a `Degraded` state
- These degraded policies consume resources and cannot be easily removed
- Search logs for: `degraded`, `policy cleanup failed`, `failed to delete loadbalancer`
- Cross-reference with hnsdiag: LB entries with no corresponding kube-proxy service = potentially degraded

- 🟡 WARNING: DSR enabled with policy deletion errors — degraded policies may accumulate
- 🟡 WARNING: hnsdiag shows LBs not tracked by kube-proxy (potential degraded DSR policies)
- 🔵 INFO: DSR mode enabled and functioning normally

### 4. Excluded Port Range Analysis (`excludedportrange.txt`)

Parse `excludedportrange.txt` which contains output of `netsh interface ipv4 show excludedportrange protocol=tcp`.

**Format:**
```
Protocol tcp Port Exclusion Ranges

Start Port    End Port
----------    --------
     50000       50063    *
     50064       50127
     ...

* - Administered port exclusions.
```

**What to check:**
- Parse each `Start Port — End Port` range
- Compare against Kubernetes NodePort range (default 30000–32767)
- Compare against common service ports (80, 443, 8080, etc.)
- Lines marked with `*` are administratively set; others are dynamic (Hyper-V, WinNAT)
- Count total excluded ranges — excessive exclusions reduce available ports

**Conflict detection:**
- For each excluded range, check if it overlaps with NodePort range 30000–32767
- Check if excluded ranges overlap with kube-proxy's health check port (default 10256)
- Check if excluded ranges overlap with kubelet port (10250)

- 🔴 CRITICAL: Excluded port ranges overlap with NodePort range 30000–32767 — NodePort services will fail to bind
- 🔴 CRITICAL: Excluded port ranges overlap with kube-proxy health check port (10256)
- 🟡 WARNING: >20 excluded port ranges — port space fragmentation may cause binding failures
- 🔵 INFO: No conflicts detected with service/NodePort ranges

### 5. Reserved Ports / Ephemeral Port Exhaustion (`reservedports.txt`)

Parse `reservedports.txt` which summarizes ephemeral port availability.

**What to check:**
- Look for: `Couldn't reserve more than X ranges of 64 ports` — indicates ephemeral port exhaustion
- Parse available ephemeral port range (default Windows: 49152–65535)
- Calculate: total ephemeral range minus excluded ranges = available ports
- Each HNS load balancer reserves a 64-port block from the ephemeral range

**Port exhaustion indicators:**
- `Couldn't reserve` message = critical exhaustion (Azure/AKS#1375)
- Available port blocks < number of services = services cannot get port reservations
- DNS failures often follow port exhaustion (DNS uses ephemeral ports)

- 🔴 CRITICAL: `Couldn't reserve` message present — ephemeral port exhaustion, DNS and services impacted
- 🔴 CRITICAL: Available ephemeral ports < 1000 — imminent exhaustion
- 🟡 WARNING: Available ephemeral ports < 5000 — approaching exhaustion
- 🔵 INFO: Ephemeral port availability healthy (>5000 ports available)

### 6. SNAT Exhaustion Detection (`wfp/netevents.xml`, `kubeproxy.log`)

In WFP netevents, look for high volumes of `DROP` events with reason related to port exhaustion or connection limits.

In kube-proxy logs, search for `SNAT` port exhaustion messages or `masquerade` failures.

- 🔴 CRITICAL: Evidence of SNAT port exhaustion (repeated connection failures to external endpoints, high DROP count in WFP netevents)
- 🟡 WARNING: Elevated DROP events in WFP netevents targeting outbound connections

### 7. Service Unreachability Diagnosis

Correlate kube-proxy policy state with service health:

**Indicators of service unreachability:**
- kube-proxy log shows `Adding new service port` but no corresponding `Synced policies` — policy creation stalled
- Slow policy programming after restart (k/k#109162): count time from first service sync to all policies programmed
- Large service count (>200) + recent kube-proxy restart = extended unreachability window (~30s per LB)

**Cross-reference with hnsdiag:**
- Expected LBs (from kube-proxy service list) vs actual LBs in hnsdiag
- Missing LBs = services not programmed on this node
- Extra LBs in hnsdiag not in kube-proxy = stale rules from prior instance

- 🔴 CRITICAL: kube-proxy running but many services have no HNS LB policy — services unreachable from this node
- 🟡 WARNING: Slow policy programming detected — extended service unreachability after restart (k/k#109162)
- 🟡 WARNING: hnsdiag shows extra LBs not tracked by kube-proxy — stale policies

### 8. kube-proxy Crash/Restart Detection (`*_services.csv`, `scqueryex.txt`)

**From services.csv:**
- Search for kube-proxy service events (service name: `kube-proxy` or `kubeproxy`)
- Map start/stop patterns to identify crash-restart cycles
- Rapid restarts (>3 in 30 minutes) = crash loop

**From scqueryex.txt:**
- Check kube-proxy service state: `RUNNING`, `STOPPED`, `START_PENDING`
- `STOPPED` state = kube-proxy not running, no policy programming

**From kube-proxy logs:**
- Multiple `Using WinKernel proxier` or `Starting kube-proxy` lines = restarts
- Gaps in timestamps between log entries = crash and restart period

- 🔴 CRITICAL: kube-proxy service STOPPED — no service routing on this node
- 🔴 CRITICAL: kube-proxy restarting >3 times in 30 minutes — crash loop
- 🟡 WARNING: kube-proxy restarted once or twice — policies were temporarily lost
- 🔵 INFO: kube-proxy running normally, no restart events

### 9. kube-proxy Startup Sync Delay (kubeproxy.log)

After kube-proxy starts (or restarts), it must reprogram all HNS load balancer policies. This is slow on Windows (~30s per LB for large clusters).

**Detection:**
- Find first `syncProxyRules` or `Adding new service port` timestamp after start
- Find last `Synced policies` timestamp
- Calculate delta — this is the service blackout window
- With >200 services, this can exceed 1 hour (k/k#109162)

- 🔴 CRITICAL: Sync delay >30 minutes — extended service blackout after kube-proxy restart
- 🟡 WARNING: Sync delay >5 minutes — noticeable service disruption
- 🔵 INFO: Sync completed within 5 minutes (normal for moderate service count)

## Findings Format

```markdown
### kube-proxy & Service Routing Findings

🔴 **CRITICAL** (HIGH confidence): Ephemeral port exhaustion detected
  - reservedports.txt: "Couldn't reserve more than 0 ranges of 64 ports"
  - 47 excluded port ranges consuming most of ephemeral range (49152-65535)
  - DNS resolution likely failing on this node (Azure/AKS#1375)
  - Mitigation: expand ephemeral range via `netsh int ipv4 set dynamicportrange`

🔴 **CRITICAL** (HIGH confidence): kube-proxy cannot find HNS network
  - kubeproxy.err.log: "Unable to find HNS Network specified by azure" (repeated 230 times)
  - No service routing operational on this node
  - Cross-ref: HNS sub-skill should confirm network existence in hnsdiag

🟡 **WARNING** (MEDIUM confidence): Stale HNS load balancer rule accumulation (k/k#112836)
  - 87 "Deleting stale load balancer" messages in 10-minute window
  - Rules accumulating after each pod deletion
  - Will cause increasing memory usage and slower sync cycles

🟡 **WARNING** (MEDIUM confidence): Excluded port range conflicts with NodePort range
  - Range 30000-30063 excluded (overlaps NodePort default 30000-32767)
  - NodePort services assigned ports in this range will fail to bind

🔵 **INFO**: DSR mode enabled, policies programming normally
🔵 **INFO**: kube-proxy startup sync completed in 3m12s for 145 services
```

## Known Patterns

| Pattern | Severity | Confidence | Meaning |
|---------|----------|------------|---------|
| `Unable to find HNS Network` | 🔴 CRITICAL | HIGH | kube-proxy non-functional — HNS network missing (k/k#79515) |
| `Couldn't reserve` in reservedports.txt | 🔴 CRITICAL | HIGH | Ephemeral port exhaustion — DNS/services broken (Azure/AKS#1375) |
| Excluded port range overlaps NodePort 30000–32767 | 🔴 CRITICAL | HIGH | NodePort services cannot bind on this node |
| kube-proxy service STOPPED | 🔴 CRITICAL | HIGH | No service routing on this node |
| Repeated `failed to create loadbalancer` | 🔴 CRITICAL | HIGH | Services not being programmed |
| HNS timeout errors in kube-proxy | 🔴 CRITICAL | MEDIUM | HNS hung — all policy operations blocked |
| kube-proxy crash loop (>3 restarts/30min) | 🔴 CRITICAL | HIGH | Sustained service routing outage |
| Sync delay >30 minutes after restart | 🔴 CRITICAL | MEDIUM | Extended service blackout (k/k#109162) |
| >50 stale LB deletions per sync cycle | 🔴 CRITICAL | MEDIUM | Massive stale rule accumulation (k/k#112836) |
| `The specified port already exists` | 🟡 WARNING | HIGH | Stale HNS policy blocking new creation (k/k#68923) |
| `Local endpoint not found` | 🟡 WARNING | MEDIUM | kube-proxy state desync with HNS |
| `ModifyLoadBalancer` errors | 🟡 WARNING | MEDIUM | LB update failures (k/k#131466) |
| DSR + policy deletion errors | 🟡 WARNING | MEDIUM | Degraded DSR policies accumulating (ms/Windows-Containers#79) |
| Excluded port range overlaps 10250/10256 | 🟡 WARNING | HIGH | kubelet/kube-proxy health check port blocked |
| >20 excluded port ranges | 🟡 WARNING | LOW | Port space fragmentation — may cause binding issues |
| Available ephemeral ports < 5000 | 🟡 WARNING | MEDIUM | Approaching port exhaustion |
| Sync delay 5–30 minutes | 🟡 WARNING | MEDIUM | Noticeable service disruption after restart |
| Extra LBs in hnsdiag vs kube-proxy | 🟡 WARNING | LOW | Stale policies from prior kube-proxy instance |
| SNAT port exhaustion in WFP/kube-proxy | 🔴 CRITICAL | MEDIUM | Outbound connections failing — SNAT ports depleted |

## Cross-References

- **analyze-hns.md**: Covers HNS data plane state (endpoints, LBs, WFP). This sub-skill covers kube-proxy's control plane view. Cross-reference LB counts: kube-proxy expected vs hnsdiag actual. HNS LB reset events (analyzed there) cause kube-proxy sync storms (analyzed here). Port exhaustion found here directly causes DNS failures.
- **analyze-kubelet.md**: Kubelet health affects kube-proxy — if kubelet is NotReady, kube-proxy may lose API server connectivity and stop syncing policies.
- **analyze-services.md**: Service events in `*_services.csv` provide kube-proxy crash/restart timestamps. System-level port reservation changes appear in system events.
- **analyze-containers.md**: Pod failures may be caused by service unreachability (no LB policies) rather than container issues — correlate pod network errors with kube-proxy policy state.
