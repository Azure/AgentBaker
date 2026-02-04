# Design Document: LocalDNS Hosts Plugin for AKS Critical FQDNs

**Author:** Saewon Kwak
**Date:** February 2026
**Status:** Draft
**PR:** [#7639](https://github.com/Azure/AgentBaker/pull/7639)

---

## Executive Summary

This feature enhances DNS reliability for Azure Kubernetes Service (AKS) nodes by caching critical Azure infrastructure FQDNs in a local hosts file. **AKS-RP provides IP addresses** for these FQDNs at node provisioning time, which CSE writes to `/etc/localdns/hosts`. A systemd timer periodically refreshes these entries by querying live DNS. The LocalDNS CoreDNS instance consults this file before forwarding queries, reducing external DNS dependencies and improving container image pull reliability.

---

## Table of Contents

1. [Problem Statement](#problem-statement)
2. [Solution Overview](#solution-overview)
3. [Architecture](#architecture)
4. [Component Details](#component-details)
5. [Data Flow](#data-flow)
6. [Integration Points](#integration-points)
7. [Benefits](#benefits)
8. [Failure Handling](#failure-handling)
9. [Security Considerations](#security-considerations)
10. [Testing Strategy](#testing-strategy)
11. [Future Enhancements](#future-enhancements)

---

## Problem Statement

### Background

AKS nodes depend on several critical Azure FQDNs during operation. These FQDNs vary by Azure cloud:

**Azure Public Cloud:**

| FQDN | Purpose |
|------|---------|
| mcr.microsoft.com | Microsoft Container Registry for container images |
| packages.aks.azure.com | AKS package repository |
| login.microsoftonline.com | Azure AD authentication |
| management.azure.com | Azure Resource Manager API |
| packages.microsoft.com | Microsoft package repository |
| acs-mirror.azureedge.net | ACS mirror for artifacts |
| eastus.data.mcr.microsoft.com | Regional MCR data endpoint |

**Azure China Cloud:**

| FQDN | Purpose |
|------|---------|
| mcr.azk8s.cn | China MCR mirror |
| login.chinacloudapi.cn | Azure AD authentication (China) |
| management.chinacloudapi.cn | Azure Resource Manager (China) |

**Azure Government Cloud:**

| FQDN | Purpose |
|------|---------|
| mcr.microsoft.us | US Government MCR |
| login.microsoftonline.us | Azure AD authentication (US Gov) |
| management.usgovcloudapi.net | Azure Resource Manager (US Gov) |

### Impact of DNS Failures

DNS resolution failures for these FQDNs can cause:

1. **Failed container image pulls** - Kubelet cannot pull required images
2. **Authentication failures** - Azure AD tokens cannot be obtained
3. **Node provisioning delays** - Bootstrap process stalls
4. **Cluster instability** - Nodes fail health checks

### Current State

Without this feature, every DNS query for these critical FQDNs goes directly to the configured upstream DNS server (Azure DNS 168.63.129.16 or a custom DNS server). If the upstream DNS is slow or temporarily unavailable, critical operations fail immediately.

---

## Solution Overview

### Approach

Implement a local DNS caching mechanism that:

1. **Receives IP addresses from AKS-RP** at node provisioning time
2. **Writes entries to hosts file** before LocalDNS starts
3. **Periodically refreshes** the hosts file by querying live DNS
4. **Configures LocalDNS** (CoreDNS) to check this file first
5. **Falls through** to the upstream DNS server for queries not in the cache

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Use hosts file format | Simple, well-understood, CoreDNS native support |
| 15-minute refresh interval | Balance between freshness and system load |
| Use nslookup | Available on both Ubuntu and Azure Linux |
| Graceful degradation | Keep existing cache on resolution failure |
| LocalDNS-only | Only enabled when LocalDNS feature is active |

---

## Architecture

### High-Level Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                  AKS Node                                        │
│                                                                                  │
│  ┌────────────────┐         ┌───────────────────┐                               │
│  │  aks-hosts-    │────────▶│  aks-hosts-       │                               │
│  │  setup.timer   │ triggers│  setup.sh         │                               │
│  └────────────────┘         └─────────┬─────────┘                               │
│    (every 15 min)                     │                                         │
│                                       │ 1. queries DNS                          │
│                                       ▼                                         │
│                             ┌─────────────────────┐                             │
│                             │  System DNS Server  │                             │
│                             │  (Azure DNS or      │                             │
│                             │   Custom DNS)       │                             │
│                             └─────────┬───────────┘                             │
│                                       │ 2. returns IPs                          │
│                                       ▼                                         │
│                             ┌───────────────────┐                               │
│                             │  /etc/localdns/   │                               │
│                             │      hosts        │◀─── 3. script writes          │
│                             └─────────┬─────────┘                               │
│                                       │                                         │
│                                       │ 4. LocalDNS reads hosts file            │
│              ┌────────────────────────┴────────────────────────┐                │
│              │                                                 │                │
│              ▼                                                 ▼                │
│  ┌───────────────────────────────────┐   ┌───────────────────────────────────┐  │
│  │   VnetDNS Listener (169.254.10.10)│   │  KubeDNS Listener (169.254.10.11) │  │
│  │                                   │   │                                   │  │
│  │  ┌─────────────┐ ┌──────────────┐ │   │  ┌─────────────┐ ┌──────────────┐ │  │
│  │  │hosts plugin │→│forward plugin│ │   │  │hosts plugin │→│forward plugin│ │  │
│  │  │(check file) │ │(Azure DNS)   │ │   │  │(check file) │ │(CoreDNS/     │ │  │
│  │  └─────────────┘ └──────────────┘ │   │  └─────────────┘ │Azure DNS)    │ │  │
│  │                                   │   │                  └──────────────┘ │  │
│  │  Serves: Kubelet, dnsPolicy:      │   │  Serves: Pods with dnsPolicy:     │  │
│  │          Default pods             │   │          ClusterFirst (default)   │  │
│  └───────────────────┬───────────────┘   └───────────────────┬───────────────┘  │
│                      ▲                                       ▲                   │
│                      │                                       │                   │
│  ┌───────────────────┴──┐                    ┌───────────────┴──┐               │
│  │   Kubelet            │                    │   Pods           │               │
│  │   (host network)     │                    │ (ClusterFirst)   │               │
│  └──────────────────────┘                    └──────────────────┘               │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

**Query Flow in LocalDNS (both listeners):**
1. DNS query arrives at LocalDNS (CoreDNS)
   - Kubelet/system daemons → VnetDNS (169.254.10.10:53)
   - Pods with `dnsPolicy: ClusterFirst` → KubeDNS (169.254.10.11:53)
2. **hosts plugin** checks `/etc/localdns/hosts` for a matching entry
3. If found → return IP immediately (no external query needed)
4. If not found → **fallthrough** to the next plugin (forward plugin)
5. **forward plugin** queries:
   - VnetDNS: Azure DNS (168.63.129.16)
   - KubeDNS: CoreDNS service IP for cluster.local, Azure DNS for external

### Component Interaction

```
                    Boot Sequence
                         │
                         ▼
              ┌──────────────────┐
              │   CSE runs       │
              │   enableAKS-     │ ◀── (1) CSE writes AKS-RP provided entries
              │   HostsSetup()   │
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │ /etc/localdns/   │
              │     hosts        │ ◀── (2) Hosts file populated with AKS-RP IPs
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │  aks-hosts-      │
              │  setup.timer     │ ◀── (3) Timer enabled and fires (OnBootSec=0)
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │  aks-hosts-      │
              │  setup.sh        │ ◀── (4) Script queries DNS and refreshes hosts file
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │ /etc/localdns/   │
              │     hosts        │ ◀── (5) Hosts file updated with fresh IPs
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │   localdns       │
              │   .service       │ ◀── (6) LocalDNS starts with fresh entries
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │   kubelet        │
              │   .service       │ ◀── (7) Kubelet starts (can pull images immediately)
              └──────────────────┘
```

**Ordering Guarantees:**
- **CSE writes hosts file before LocalDNS starts** - ensures entries are available
- **Timer refreshes hosts file with live DNS** - ensures fresh IPs before LocalDNS
- `localdns.service` starts **after** hosts file is populated
- `kubelet.service` starts **after** LocalDNS to ensure DNS is ready for container pulls

---

## Component Details

### 1. AKS Hosts Setup Script

**File:** `parts/linux/cloud-init/artifacts/aks-hosts-setup.sh`
**Location on node:** `/opt/azure/containers/aks-hosts-setup.sh`

#### Purpose
Resolves DNS records for critical FQDNs and writes them to a hosts file.

#### Key Features

| Feature | Description |
|---------|-------------|
| IPv4 and IPv6 support | Resolves both A and AAAA records |
| Input validation | Regex filtering prevents invalid IP injection |
| Graceful failure | Preserves existing file on resolution failure |
| Cross-platform | Uses nslookup (works on Ubuntu and Azure Linux) |

#### Target FQDNs

```bash
CRITICAL_FQDNS=(
    "acs-mirror.azureedge.net"
    "eastus.data.mcr.microsoft.com"
    "login.microsoftonline.com"
    "management.azure.com"
    "mcr.microsoft.com"
    "packages.aks.azure.com"
    "packages.microsoft.com"
)
```

#### Output Format

```
# AKS critical FQDN addresses resolved at Wed Feb 4 10:00:00 UTC 2026
# This file is automatically generated by aks-hosts-setup.service

# mcr.microsoft.com
52.184.232.37 mcr.microsoft.com
2603:1000:4::24 mcr.microsoft.com

# packages.aks.azure.com
20.118.79.163 packages.aks.azure.com
```

---

### 2. AKS-RP Provided Hosts Entries

**API Field:** `LocalDNSProfile.CriticalHostsEntries`
**When Used:** AKS-RP provides IP addresses for critical FQDNs at node provisioning time

#### Purpose

AKS-RP provides FQDN-to-IP mappings at node provisioning time via the `NodeBootstrappingConfiguration`. CSE writes these entries to `/etc/localdns/hosts` before starting LocalDNS, ensuring DNS resolution is available for container image pulls.

#### Benefits

| Benefit | Description |
|---------|-------------|
| Fresh IPs | AKS-RP provides up-to-date IP addresses |
| Cloud-specific | AKS-RP provides correct endpoints for each cloud (Public, China, Government) |
| Regional support | AKS-RP can provide region-specific endpoints |
| Centralized management | IP updates are managed by AKS-RP |
| No VHD coupling | IPs are decoupled from VHD release cycle |

#### Provisioning Flow

The hosts file is written **before** kubelet starts, ensuring DNS is available for the first container image pull:

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                           Node Provisioning Timeline                              │
└──────────────────────────────────────────────────────────────────────────────────┘

AKS-RP                      AgentBaker                    Node (CSE Execution)
───────                     ──────────                    ────────────────────
   │                            │                                │
   │  NodeBootstrappingConfig   │                                │
   │  ┌──────────────────────┐  │                                │
   │  │ LocalDNSProfile:     │  │                                │
   │  │   CriticalHostsEntries: │                                │
   │  │     mcr.microsoft.com   │                                │
   │  │       -> [20.1.2.3]     │                                │
   │  └──────────────────────┘  │                                │
   │ ──────────────────────────►│                                │
   │                            │                                │
   │                            │  Generate CSE with             │
   │                            │  LOCALDNS_CRITICAL_HOSTS_ENTRIES
   │                            │  (base64 encoded)              │
   │                            │ ──────────────────────────────►│
   │                            │                                │
   │                            │                     ┌──────────▼──────────┐
   │                            │                     │ 1. CSE starts       │
   │                            │                     │ 2. enableAKSHosts() │
   │                            │                     │    - Decode base64  │
   │                            │                     │    - Write hosts    │
   │                            │                     │      file           │
   │                            │                     │ 3. Start localdns   │
   │                            │                     │    service          │
   │                            │                     │ 4. Configure kubelet│
   │                            │                     │    DNS              │
   │                            │                     │ 5. Start kubelet    │
   │                            │                     │    (uses LocalDNS)  │
   │                            │                     └─────────────────────┘
   │                            │                                │
   │                            │                     ┌──────────▼──────────┐
   │                            │                     │ kubelet pulls images│
   │                            │                     │ DNS: 169.254.10.10  │
   │                            │                     │  -> hosts file      │
   │                            │                     │  -> mcr.microsoft.  │
   │                            │                     │       com resolves! │
   │                            │                     └─────────────────────┘
```

#### Data Contract

**Go Struct (pkg/agent/datamodel/types.go):**

```go
type LocalDNSProfile struct {
    // EnableLocalDNS specifies if LocalDNS should be enabled for the nodepool
    EnableLocalDNS bool `json:"enableLocalDNS,omitempty"`

    // CriticalHostsEntries contains FQDN to IP address mappings for critical Azure infrastructure.
    // When provided by AKS-RP, CSE will write these to /etc/localdns/hosts.
    // Key: FQDN (e.g., "mcr.microsoft.com")
    // Value: List of IP addresses (IPv4 and/or IPv6)
    CriticalHostsEntries map[string][]string `json:"criticalHostsEntries,omitempty"`
}
```

**Proto Definition (aksnodeconfig/v1/localdns_config.proto):**

```protobuf
message LocalDnsProfile {
  bool enable_local_dns = 1;
  optional int32 cpu_limit_in_milli_cores = 2;
  optional int32 memory_limit_in_mb = 3;
  map<string, LocalDnsOverrides> vnet_dns_overrides = 4;
  map<string, LocalDnsOverrides> kube_dns_overrides = 5;

  // CriticalHostsEntries contains FQDN to IP address mappings for critical Azure infrastructure.
  // When provided by AKS-RP, CSE will write these to /etc/localdns/hosts.
  map<string, CriticalHostsEntry> critical_hosts_entries = 6;
}

message CriticalHostsEntry {
  // IP addresses (both IPv4 and IPv6) for the FQDN.
  repeated string ip_addresses = 1;
}
```

#### API Contract

AKS-RP sends the following in `NodeBootstrappingConfiguration`:

```json
{
  "agentPoolProfile": {
    "localDNSProfile": {
      "enableLocalDNS": true,
      "criticalHostsEntries": {
        "mcr.microsoft.com": ["20.61.99.68", "2603:1061:1002::2"],
        "login.microsoftonline.com": ["20.190.151.68", "20.190.151.70"],
        "packages.aks.azure.com": ["20.7.0.233"],
        "management.azure.com": ["20.37.158.0"],
        "<region>.dp.kubernetesconfiguration.azure.com": ["10.1.2.3"]
      }
    }
  }
}
```

#### Environment Variable

CSE receives the hosts entries as a base64-encoded string:

```bash
LOCALDNS_CRITICAL_HOSTS_ENTRIES="IyBBS1MgY3JpdGljYWwgRlFETiBhZGRyZXNzZXMgcHJvdmlkZWQgYnkgQUtTLVJQCi..."
```

When decoded, it produces a hosts-file format:

```
# AKS critical FQDN addresses provided by AKS-RP
# This file is written by CSE during node provisioning

# login.microsoftonline.com
20.190.151.68 login.microsoftonline.com
20.190.151.70 login.microsoftonline.com

# management.azure.com
20.37.158.0 management.azure.com

# mcr.microsoft.com
20.61.99.68 mcr.microsoft.com
2603:1061:1002::2 mcr.microsoft.com

# packages.aks.azure.com
20.7.0.233 packages.aks.azure.com
```

---

### 3. Systemd Timer

**File:** `parts/linux/cloud-init/artifacts/aks-hosts-setup.timer`
**Location on node:** `/etc/systemd/system/aks-hosts-setup.timer`

#### Configuration

| Setting | Value | Purpose |
|---------|-------|---------|
| OnBootSec | 0 | Run immediately at boot |
| OnUnitActiveSec | 15min | Refresh interval |
| AccuracySec | 1s | Timer precision |
| RandomizedDelaySec | 60s | Thundering herd prevention |

#### Unit File

```ini
[Unit]
Description=Run AKS hosts setup periodically
Before=localdns.service

[Timer]
OnBootSec=0
OnUnitActiveSec=15min
AccuracySec=1s
RandomizedDelaySec=60s

[Install]
WantedBy=timers.target
```

---

### 4. Systemd Service

**File:** `parts/linux/cloud-init/artifacts/aks-hosts-setup.service`
**Location on node:** `/etc/systemd/system/aks-hosts-setup.service`

#### Configuration

| Setting | Value | Purpose |
|---------|-------|----------|
| Type | oneshot | Script runs once per trigger |
| After | network-online.target | Ensures network is available for DNS queries |
| Before | kubelet.service, localdns.service | Ensures hosts file is refreshed before services start |

#### Unit File

```ini
[Unit]
Description=Populate /etc/localdns/hosts with critical AKS FQDN addresses
After=network-online.target
Wants=network-online.target
Before=kubelet.service localdns.service

[Service]
Type=oneshot
ExecStart=/opt/azure/containers/aks-hosts-setup.sh

[Install]
WantedBy=multi-user.target kubelet.service
```

---

### 5. CoreDNS Configuration Update

**File:** `pkg/agent/baker.go`

#### Change Description

The LocalDNS Corefile template is modified to include a `hosts` plugin block that checks `/etc/localdns/hosts` before forwarding to the upstream DNS server. The hosts plugin is added to **both** VnetDNS and KubeDNS root domain (`.`) server blocks to ensure all DNS consumers benefit from the cache.

#### VnetDNS (169.254.10.10) - Before

```
.:53 {
    log
    bind 169.254.10.10
    forward . 168.63.129.16 {
        policy sequential
        max_concurrent 1000
    }
}
```

#### VnetDNS (169.254.10.10) - After

```
.:53 {
    log
    bind 169.254.10.10
    # Check /etc/localdns/hosts first for critical AKS FQDNs
    hosts /etc/localdns/hosts {
        fallthrough
    }
    forward . 168.63.129.16 {
        policy sequential
        max_concurrent 1000
    }
}
```

#### KubeDNS (169.254.10.11) - Before

```
.:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
        policy sequential
        max_concurrent 1000
    }
}
```

#### KubeDNS (169.254.10.11) - After

```
.:53 {
    errors
    bind 169.254.10.11
    # Check /etc/localdns/hosts first for critical AKS FQDNs
    hosts /etc/localdns/hosts {
        fallthrough
    }
    forward . 10.0.0.10 {
        policy sequential
        max_concurrent 1000
    }
}
```

#### Why Both Listeners?

| Listener | Serves | Benefits from hosts cache |
|----------|--------|---------------------------|
| VnetDNS (169.254.10.10) | Kubelet, `dnsPolicy: Default` pods | ✅ Image pulls, system DNS |
| KubeDNS (169.254.10.11) | `dnsPolicy: ClusterFirst` pods (default) | ✅ Application pods querying Azure endpoints |

Most pods use `dnsPolicy: ClusterFirst` by default. Without the hosts plugin on KubeDNS, these pods wouldn't benefit from the cache when querying external Azure FQDNs like `mcr.microsoft.com` or `login.microsoftonline.com`.

#### Fallthrough Behavior

The `fallthrough` directive ensures that:
- Queries matching hosts file entries return cached IPs
- Queries NOT in hosts file are forwarded to the upstream DNS server
- No queries are blocked or dropped

---

### 6. CSE Integration

**Files:**
- `parts/linux/cloud-init/artifacts/cse_config.sh`
- `parts/linux/cloud-init/artifacts/cse_main.sh`

#### New Function: enableAKSHostsSetup()

```bash
enableAKSHostsSetup() {
    local hosts_file="/etc/localdns/hosts"

    # Write AKS-RP provided critical hosts entries to the hosts file
    echo "AKS-RP provided critical hosts entries, writing to ${hosts_file}..."
    mkdir -p "$(dirname "${hosts_file}")"
    echo "${LOCALDNS_CRITICAL_HOSTS_ENTRIES}" | base64 -d > "${hosts_file}"
    chmod 644 "${hosts_file}"
    echo "Critical hosts entries written from AKS-RP."

    # Enable the timer for periodic refresh (every 15 minutes)
    # This will update the hosts file with live DNS resolution
    echo "Enabling aks-hosts-setup timer..."
    systemctlEnableAndStart aks-hosts-setup.timer 30 || exit $ERR_SYSTEMCTL_START_FAIL
    echo "aks-hosts-setup timer enabled successfully."
}
```

#### Activation Logic

The hosts setup and LocalDNS are enabled when LocalDNS is configured:

```bash
if [ "${SHOULD_ENABLE_LOCALDNS}" = "true" ]; then
    # Write hosts file BEFORE starting LocalDNS so it has entries to serve
    logs_to_events "AKS.CSE.enableAKSHostsSetup" enableAKSHostsSetup

    # Start LocalDNS after hosts file is populated
    logs_to_events "AKS.CSE.enableLocalDNSForScriptless" enableLocalDNSForScriptless
fi
```

---

## Data Flow

### Boot Time Flow

| Step | Action | Component |
|------|--------|-----------|
| 1 | Node boots | System |
| 2 | Network comes online | systemd |
| 3 | CSE runs `enableAKSHostsSetup()` | cse_main.sh |
| 4 | **AKS-RP provided entries written to hosts file** | LOCALDNS_CRITICAL_HOSTS_ENTRIES |
| 5 | Timer enabled for periodic refresh | aks-hosts-setup.timer |
| 6 | Timer triggers (OnBootSec=0) | aks-hosts-setup.timer |
| 7 | Script refreshes with live DNS | aks-hosts-setup.sh |
| 8 | Hosts file updated with fresh IPs | /etc/localdns/hosts |
| 9 | LocalDNS starts (uses fresh IPs) | localdns.service |
| 10 | Kubelet starts | kubelet.service |

### Runtime Flow

| Step | Action | Component |
|------|--------|-----------|
| 1 | Timer triggers (every 15 min) | aks-hosts-setup.timer |
| 2 | Script re-resolves FQDNs | aks-hosts-setup.sh |
| 3 | File updated atomically | /etc/localdns/hosts |
| 4 | CoreDNS hosts plugin reloads file | hosts plugin |

### Query Resolution Flow

| Step | Action | Result |
|------|--------|--------|
| 1 | Pod queries mcr.microsoft.com | DNS query sent to LocalDNS |
| 2 | LocalDNS receives query on 169.254.10.10:53 | LocalDNS processes the query |
| 3 | **hosts plugin** checks /etc/localdns/hosts | File lookup |
| 4a | **If found in hosts file:** Return IP immediately | No external query needed |
| 4b | **If not found in hosts file:** fallthrough to forward plugin | Forward plugin queries upstream DNS |

---

## Integration Points

### VHD Build Pipeline

New artifacts are installed during VHD build via Packer:

| Source | Destination | Permissions |
|--------|-------------|-------------|
| aks-hosts-setup.sh | /opt/azure/containers/ | 0755 |
| aks-hosts-setup.service | /etc/systemd/system/ | 0644 |
| aks-hosts-setup.timer | /etc/systemd/system/ | 0644 |

### Packer Configuration Files Modified

- vhdbuilder/packer/vhd-image-builder-base.json
- vhdbuilder/packer/vhd-image-builder-arm64-gen2.json
- vhdbuilder/packer/vhd-image-builder-mariner*.json
- vhdbuilder/packer/packer_source.sh

---

## Benefits

| Benefit | Description |
|---------|-------------|
| **Immediate Availability** | AKS-RP provides IPs at provisioning time - DNS works before kubelet starts |
| **Improved Reliability** | DNS failures for critical FQDNs don't immediately impact operations |
| **Reduced Latency** | Local cache eliminates DNS round-trip for cached domains |
| **Resilience** | Nodes can continue operating during DNS outages using cached IPs |
| **Self-Healing** | Timer automatically refreshes cache every 15 minutes |
| **Thundering Herd Prevention** | RandomizedDelaySec prevents cluster-wide simultaneous resolution |
| **Zero Configuration** | Automatic when LocalDNS is enabled |

---

## Failure Handling

| Failure Scenario | Behavior | Impact |
|------------------|----------|--------|
| DNS resolution fails on refresh | Script exits 0, timer retries in 15 min | AKS-RP provided IPs still available |
| nslookup not available | Script fails gracefully | AKS-RP provided IPs still available |
| Hosts file write fails | Existing file preserved, error logged | Previous IPs still used |
| Invalid DNS response | Filtered by regex, not written | Valid entries only |
| All resolutions fail | Existing hosts file preserved | Previous IPs still used |
| LocalDNS not enabled | Timer not started | Feature inactive |

---

## Security Considerations

| Aspect | Implementation |
|--------|----------------|
| No new network dependencies | Uses existing system DNS configuration |
| File permissions | Standard 644 permissions on hosts file |
| Input validation | Regex filtering prevents DNS injection |
| No credentials | Script uses unauthenticated DNS queries |
| No external URLs | Only resolves, doesn't fetch from new sources |

---

## Testing Strategy

### Unit Tests

| Test File | Coverage |
|-----------|----------|
| spec/parts/linux/cloud-init/artifacts/aks_hosts_setup_spec.sh | Script behavior |
| spec/parts/linux/cloud-init/artifacts/cse_config_spec.sh | enableAKSHostsSetup function |

### Integration Tests

| Test File | Coverage |
|-----------|----------|
| pkg/agent/baker_test.go | LocalDNS Corefile generation |

### Manual Verification

1. Deploy node with LocalDNS enabled
2. Verify timer is running: `systemctl status aks-hosts-setup.timer`
3. Check hosts file: `cat /etc/localdns/hosts`
4. Verify LocalDNS Corefile:
   ```bash
   kubectl node-shell <node>
   cat /opt/azure/containers/localdns/updated.localdns.corefile
   ```

---

## Compatibility Matrix

### Operating System Support

| OS | Architecture | Status | Notes |
|----|-------------|--------|-------|
| Ubuntu 20.04 | x64 | ✅ Supported | `bind9-dnsutils` provides nslookup |
| Ubuntu 22.04 | x64 | ✅ Supported | `bind9-dnsutils` provides nslookup |
| Ubuntu 24.04 | x64 | ✅ Supported | `bind9-dnsutils` provides nslookup |
| Ubuntu 24.04 | ARM64 | ✅ Supported | `bind9-dnsutils` provides nslookup |
| Azure Linux (Mariner) | x64 | ✅ Supported | `bind-utils` provides nslookup |
| Azure Linux (Mariner) | ARM64 | ✅ Supported | `bind-utils` provides nslookup |
| Azure Linux (Mariner) CVM | x64 | ✅ Supported | `bind-utils` provides nslookup |
| Flatcar Container Linux | x64/ARM64 | ⚠️ Limited | nslookup not available; uses AKS-RP provided IPs only (no refresh) |

### Feature Requirements

| Requirement | Status | Notes |
|-------------|--------|-------|
| LocalDNS enabled | ✅ Required | Feature dependency - timer only enabled when LocalDNS is active |
| Scriptless provisioning | ✅ Supported | Timer enabled via CSE |
| Legacy provisioning | ✅ Supported | Timer enabled via CSE |

### Graceful Degradation

If `nslookup` is not available on a distro (e.g., Flatcar):
- The timer and service run without errors (exit 0)
- **AKS-RP provided IPs remain in use** - DNS resolution still works
- IPs will not be refreshed dynamically, but AKS-RP IPs are typically current
- System journal logs: `"WARNING: No IP addresses resolved for any domain"`

---

## Future Enhancements

| Enhancement | Description | Priority |
|-------------|-------------|----------|
| Configurable FQDN list | Allow cluster-specific endpoints | Medium |
| Metrics export | Cache hit/miss statistics | Low |
| TTL awareness | Respect DNS TTL values | Medium |
| IPv6 preference | Configurable address family | Low |
| Health endpoint | Expose cache status | Low |

---

## Appendix

### Related Files Changed

| File | Change Type | Purpose |
|------|-------------|---------|
| parts/linux/cloud-init/artifacts/aks-hosts-setup.sh | New | Script that resolves critical AKS FQDNs and writes IPs to hosts file |
| parts/linux/cloud-init/artifacts/aks-hosts-setup.service | New | Systemd oneshot service that executes the script |
| parts/linux/cloud-init/artifacts/aks-hosts-setup.timer | New | Systemd timer that triggers the service at boot and every 15 minutes |
| parts/linux/cloud-init/artifacts/cse_config.sh | Modified | Adds `enableAKSHostsSetup()` function to write AKS-RP entries and enable timer |
| parts/linux/cloud-init/artifacts/cse_main.sh | Modified | Calls `enableAKSHostsSetup()` before `enableLocalDNSForScriptless()` |
| pkg/agent/baker.go | Modified | Adds `hosts /etc/localdns/hosts` plugin block to LocalDNS Corefile template |
| pkg/agent/baker_test.go | Modified | Updates expected Corefile output to include hosts plugin |
| pkg/agent/datamodel/types.go | Modified | Adds `CriticalHostsEntries` field to `LocalDNSProfile` |
| aks-node-controller/proto/aksnodeconfig/v1/localdns_config.proto | Modified | Adds `CriticalHostsEntry` message and `critical_hosts_entries` field |
| vhdbuilder/packer/*.json | Modified | Uploads new artifacts to `/home/packer/` during VHD build |
| vhdbuilder/packer/packer_source.sh | Modified | Copies artifacts to final locations |

### References

- [CoreDNS Hosts Plugin](https://coredns.io/plugins/hosts/)
- [Systemd Timer Documentation](https://www.freedesktop.org/software/systemd/man/systemd.timer.html)
- [AKS LocalDNS Feature](https://learn.microsoft.com/en-us/azure/aks/local-dns)
