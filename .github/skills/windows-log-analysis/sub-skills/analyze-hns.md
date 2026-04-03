# Analyze HNS — Host Network Service Deep Inspection

## Purpose

Deep inspection of HNS (Host Network Service) operational health: endpoint lifecycle tracking, endpoint-to-pod correlation, load balancer policy integrity, WFP/VFP policy analysis, HNS service stability, and detection of state corruption. Covers the full spectrum of HNS internals including CNI errors, DNS configuration, and network adapter state.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `<ts>-hnsdiag-list.txt` | UTF-16-LE with BOM | `hnsdiag list all` — networks, endpoints, namespaces, load balancers |
| `<ts>-cri-containerd-pods.txt` | UTF-16-LE with BOM | `crictl pods` — running/stopped pods for correlation |
| `<ts>_services.csv` | UTF-16-LE with BOM, CSV with embedded newlines | Service events — HNS service start/stop/crash |
| `wfp/filters.xml` | UTF-8 or UTF-16-LE | WFP filter definitions |
| `wfp/wfpstate.xml` | UTF-8 or UTF-16-LE | WFP current state |
| `wfp/netevents.xml` | UTF-8 or UTF-16-LE | WFP network events (drops, blocks) |
| `c:\k\debug\*azure-vnet*.log` or `*azure-vnet*.log` | UTF-8 | Azure CNI plugin logs — HNS endpoint create/delete calls |
| `ip.txt` | UTF-16-LE with BOM | `ipconfig /all` output — adapter IPs, DNS servers |
| `netadapter.txt` | UTF-16-LE with BOM | `Get-NetAdapter` output — link state, MAC, speed |
| `kubeproxy.log` or `*kube-proxy*.log` | UTF-8 | kube-proxy logs — HNS load balancer operations |
| `<ts>-hcsdiag-list.txt` | UTF-16-LE with BOM | HCS compute systems — for container-to-endpoint correlation |

**Process ALL snapshots** — cross-snapshot comparison is critical for detecting endpoint leaks, LB count drops, and HNS service restarts.

## Analysis Steps

### 1. HNS Network Inventory (`*-hnsdiag-list.txt`)

Parse the `Networks:` section of hnsdiag output. Each network line has `Name` and `ID` (GUID).

**What to check:**
- At least one non-NAT network should exist (typically named `azure`, `cbr0`, or matching the CNI config)
- Network type should be appropriate for the cluster (L2Bridge for Azure CNI, Overlay for CNI Overlay)
- Multiple snapshots: verify the same networks persist (network disappearance = HNS reset)

- 🔴 CRITICAL: No non-NAT HNS network exists — CNI networking is broken, pods cannot get IPs
- 🔴 CRITICAL: Network disappeared between snapshots — HNS service was reset or crashed
- 🔵 INFO: Network inventory looks normal (list networks found)

### 2. HNS Endpoint-to-Pod Correlation (`*-hnsdiag-list.txt` vs `*-cri-containerd-pods.txt`)

Parse the `Endpoints:` section. Each endpoint line has `Name`, `ID` (GUID), and `Virtual Network Name`.

Parse crictl pods output (see common-reference.md for parsing format).

**Correlation method:**
- Endpoint names often contain the pod name, namespace, or container ID prefix
- Match endpoint names against pod names from crictl
- Count: total endpoints, matched endpoints, unmatched (orphaned) endpoints

**Endpoint leak detection:**
- Calculate `orphaned_count = total_endpoints - matched_endpoints - infrastructure_endpoints`
- Infrastructure endpoints: those on the `nat` network or with names like `host-vnic` are not pod endpoints

- 🔴 CRITICAL: >100 orphaned HNS endpoints — IP address exhaustion imminent, HNS degradation likely
- 🟡 WARNING: 20–100 orphaned HNS endpoints — leak accumulating
- 🔵 INFO: <20 orphaned endpoints (normal churn, recently terminated pods)

**Cross-snapshot trend:**
- Compare orphaned endpoint counts across snapshots
- Growing count confirms active leak (not just transient cleanup delay)
- 🔴 CRITICAL: Orphaned endpoint count growing across snapshots (active leak confirmed)

### 3. HNS Load Balancer Integrity (`*-hnsdiag-list.txt`)

Parse the `LoadBalancers:` section. Each LB line has `ID`, `Virtual IPs`, and `Direct IP IDs`.

**What to check:**
- Count total load balancers
- LBs with empty `Direct IP IDs` = no backend endpoints (service with no healthy pods)
- Cross-snapshot: sudden drop in LB count = HNS internal reset (kubernetes/kubernetes#110849)

**Load balancer disappearance detection:**
- Compare LB counts across snapshots
- Drop from many (e.g., >50) to ≤5 between snapshots = HNS LB reset event
- After reset, kube-proxy must reprogram all LBs (slow: ~30s per LB)

- 🔴 CRITICAL: LB count dropped by >90% between snapshots (HNS internal reset)
- 🟡 WARNING: LBs with no backend DIPs (services unreachable)
- 🟡 WARNING: Very high LB count (>500) — kube-proxy restart would be extremely slow
- 🔵 INFO: LB count stable across snapshots, proportional to service count

### 4. HNS Namespace Integrity (`*-hnsdiag-list.txt`)

Parse the `Namespaces:` section. Each namespace has `ID` and associated `Endpoint IDs`.

**What to check:**
- Namespaces with no associated endpoints = orphaned namespaces
- Namespaces referencing endpoint IDs that don't appear in the Endpoints section = stale references
- Excessive namespace count relative to pod count

- 🟡 WARNING: Namespaces referencing non-existent endpoints (stale state)
- 🟡 WARNING: >50 more namespaces than running pods (namespace leak)
- 🔵 INFO: Namespace count proportional to pod count

### 5. HNS Service Health (`*_services.csv`)

Parse service event CSV (see common-reference.md for CSV parsing rules).

Search for events where `Message` contains `hns` or `Host Network Service` (case-insensitive).

**What to look for:**
- Service stop events → HNS was restarted (intentional or crash)
- Service start events after stop → recovery timeline
- Crash or unexpected termination events
- Correlate HNS restart timestamps with LB disappearance and endpoint state changes

- 🔴 CRITICAL: HNS service crash/unexpected stop events — all container networking disrupted on restart
- 🟡 WARNING: HNS service was restarted (planned) — LBs and policies need reprogramming
- 🔵 INFO: HNS service running normally (no stop/start events)

### 6. Azure CNI / HNS Error Analysis (`*azure-vnet*.log`)

Search CNI log files for HNS-specific errors:

| Error Pattern | Meaning |
|--------------|---------|
| `hnsCall failed in Win32: The object already exists` (0x1392) | Stale endpoint blocking new creation |
| `hnsCall failed in Win32: Element not found` | HNS endpoint/network was deleted or lost (HNS reset) |
| `failed to create the new HNSEndpoint` | Endpoint creation failure — various causes |
| `failed to delete endpoint` | Cleanup failure — endpoint will leak |
| `ProvisionEndpoint` errors | CNI-level endpoint setup failure |
| `no available IPs` / `address exhaustion` | IP pool exhausted (likely from endpoint leak) |
| `timeout` + `hns` | HNS service unresponsive (deadlock/hang) |

- 🔴 CRITICAL: Repeated `The object already exists` errors — stale endpoints blocking pod creation
- 🔴 CRITICAL: `no available IPs` — IP exhaustion, likely from endpoint leak
- 🔴 CRITICAL: HNS timeout errors — HNS service hung/deadlocked
- 🟡 WARNING: `Element not found` errors — HNS state was reset
- 🟡 WARNING: Intermittent endpoint creation/deletion failures

### 7. kube-proxy / HNS Load Balancer Errors (`*kube-proxy*.log`, `kubeproxy.log`)

Search kube-proxy logs for HNS-related errors:

| Error Pattern | Meaning |
|--------------|---------|
| `HNS` + `loadbalancer` + `failed` | LB policy creation failure |
| `Local endpoint not found` | kube-proxy can't find HNS endpoint for pod (stale state) |
| `ModifyLoadBalancer` + error | LB update mismatch (kubernetes/kubernetes#131466) |
| `Stale` + `loadbalancer` | Stale LB rules not cleaned up (kubernetes/kubernetes#112836) |
| `hnsCallTimeout` or operation timeouts | HNS service unresponsive |

- 🔴 CRITICAL: Repeated LB creation failures — services unreachable
- 🟡 WARNING: `Local endpoint not found` — kube-proxy state out of sync with HNS
- 🟡 WARNING: Stale load balancer rules detected

### 8. WFP Filter Analysis (`wfp/filters.xml`)

Parse the WFP filters XML file.

**What to check:**
- Count total filter entries
- Look for filters referencing container IDs or endpoint GUIDs that don't appear in hnsdiag
- Check for duplicate filters (same conditions, different IDs)
- Extremely high filter count after HNS reset indicates stale accumulation

- 🟡 WARNING: >5000 WFP filters — stale filter accumulation, packet processing overhead
- 🟡 WARNING: Filters referencing non-existent endpoints (stale after HNS reset)
- 🔵 INFO: Filter count proportional to endpoint + policy count

### 9. WFP Network Events (`wfp/netevents.xml`)

Parse WFP network events for DROP/BLOCK events:

- High volume of DROP events with same source/destination = connectivity issue
- Events referencing container endpoints that no longer exist = stale policy enforcement
- Cluster of events at a specific timestamp may correlate with HNS restart

- 🟡 WARNING: High volume of WFP DROP events (>100 in collection window)
- 🔵 INFO: Low/zero DROP events (normal)

### 10. VFP Policy Dump (if present)

Look for VFP policy dump files (`*vfp*`, `*dumpVfpPolicies*`).

If present, check:
- Load balancer rules: VIP → DIP mappings should match HNS load balancer entries
- ACL rules: should correspond to network policies
- Ports with no rules may indicate unconfigured endpoints
- Mismatched VIP/DIP in VFP vs hnsdiag = policy corruption

- 🟡 WARNING: VFP LB rules reference endpoints not in hnsdiag (stale VFP state)
- 🔵 INFO: VFP policies consistent with hnsdiag state

### 11. DNS Resolution Configuration (`ip.txt`)

Parse `ip.txt` (`ipconfig /all` output) for DNS server configuration:
- Check that DNS servers are configured on the primary adapter
- Verify kube-dns / CoreDNS ClusterIP is present (typically `10.0.0.10` or similar)
- Look for mismatched or missing DNS suffixes

- 🟡 WARNING: DNS servers missing or pointing to unexpected addresses
- 🔵 INFO: DNS configuration looks normal

### 12. Network Adapter State (`netadapter.txt`)

Parse `Get-NetAdapter` output:
- Check adapter status (Up/Down/Disabled)
- Look for adapters in unexpected states
- Verify HNS transparent adapter exists

- 🔴 CRITICAL: Primary network adapter is Down or Disabled
- 🟡 WARNING: HNS transparent adapter missing

## Findings Format

```markdown
### HNS Deep Inspection Findings

🔴 **CRITICAL** (HIGH confidence): HNS load balancer count dropped from 245 to 1 between snapshots
  - Snapshot 1 (20260323-034156): 245 load balancers
  - Snapshot 2 (20260323-044156): 1 load balancer
  - Indicates HNS internal reset — all service routing was lost
  - kube-proxy must reprogram ~245 LBs (~2+ hours at ~30s/LB)

🔴 **CRITICAL** (HIGH confidence): 156 orphaned HNS endpoints (no matching pod)
  - Total endpoints: 203, matched to pods: 47
  - Count growing across snapshots: 120 → 140 → 156
  - IP address exhaustion imminent (subnet /24 = 254 usable)

🟡 **WARNING** (MEDIUM confidence): azure-vnet.log shows 23 "Element not found" HNS errors
  - Errors cluster around 03:42 UTC — correlates with HNS service restart in services.csv
  - HNS restart cleared endpoint state; CNI DEL calls failed for pre-existing endpoints

🟡 **WARNING** (MEDIUM confidence): 6,200 WFP filters detected (threshold: 5,000)
  - Likely stale accumulation after HNS reset
  - May degrade network packet processing performance
```

## Known Patterns

| Pattern | Severity | Confidence | Meaning |
|---------|----------|------------|---------|
| LB count drop >90% between snapshots | 🔴 CRITICAL | HIGH | HNS internal reset — all LB policies lost (k/k#110849) |
| >100 orphaned endpoints | 🔴 CRITICAL | HIGH | HNS endpoint leak — IP exhaustion imminent |
| Orphaned endpoints growing across snapshots | 🔴 CRITICAL | HIGH | Active endpoint leak (not transient) |
| `The object already exists` (0x1392) in CNI | 🔴 CRITICAL | HIGH | Stale endpoints blocking pod creation (k/k#74766) |
| `no available IPs` in CNI | 🔴 CRITICAL | HIGH | IP exhaustion from endpoint leak |
| HNS timeout errors in CNI/kube-proxy | 🔴 CRITICAL | MEDIUM | HNS service hung or deadlocked |
| Non-NAT network missing from hnsdiag | 🔴 CRITICAL | HIGH | CNI network gone — no pod networking |
| HNS service crash in services.csv | 🔴 CRITICAL | HIGH | HNS crashed — all networking disrupted |
| `Element not found` in CNI logs | 🟡 WARNING | MEDIUM | HNS state reset — CNI references invalid |
| Stale HNS LB rules (k/k#112836) | 🟡 WARNING | MEDIUM | LB rules accumulate, services may misbehave |
| `Local endpoint not found` in kube-proxy | 🟡 WARNING | MEDIUM | kube-proxy out of sync with HNS |
| >5000 WFP filters | 🟡 WARNING | MEDIUM | Stale filter accumulation after resets |
| 20–100 orphaned endpoints | 🟡 WARNING | LOW | Trending toward leak — monitor |
| >500 LBs on single node | 🟡 WARNING | LOW | Slow kube-proxy restart (~4+ hours to reprogram) |
| Namespaces referencing non-existent endpoints | 🟡 WARNING | LOW | Stale HNS state |
| Primary network adapter Down | 🔴 CRITICAL | HIGH | Node has lost network connectivity |
| HNS transparent adapter missing | 🟡 WARNING | MEDIUM | HNS networking may not be initialized |
| DNS servers misconfigured | 🟡 WARNING | MEDIUM | Pod DNS resolution may fail |

## Cross-References

- **analyze-termination.md**: Zombie HCS containers may hold HNS endpoints open, preventing cleanup. Correlate zombie container IDs with orphaned endpoint names.
- **analyze-containers.md**: Pods failing to start with CNI errors — cross-reference with HNS errors found here (stale endpoints, IP exhaustion).
- **analyze-services.md**: HNS service events in `*_services.csv` — correlate HNS restart timestamps with LB/endpoint anomalies found here.
- **analyze-memory.md**: Each leaked HNS endpoint consumes kernel memory. High orphaned endpoint count contributes to memory pressure.
