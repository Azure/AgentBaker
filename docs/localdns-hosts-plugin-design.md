# Design Document: LocalDNS Hosts Plugin for AKS Critical FQDNs

**Author:** Saewon Kwak
**Date:** February 2026
**Status:** Draft
**PR:** [#7639](https://github.com/Azure/AgentBaker/pull/7639)

---

## Executive Summary

This feature enhances DNS reliability for Azure Kubernetes Service (AKS) nodes by caching critical Azure infrastructure FQDNs in a local hosts file. The VHD ships with **hardcoded IP addresses** for these FQDNs, providing immediate DNS availability at boot. A systemd timer periodically refreshes these entries by querying the upstream DNS server. The LocalDNS CoreDNS instance consults this file before forwarding queries, reducing external DNS dependencies and improving container image pull reliability.

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

AKS nodes depend on several critical Azure FQDNs during operation:

| FQDN | Purpose |
|------|---------|
| mcr.microsoft.com | Microsoft Container Registry for container images |
| packages.aks.azure.com | AKS package repository |
| login.microsoftonline.com | Azure AD authentication |
| management.azure.com | Azure Resource Manager API |
| packages.microsoft.com | Microsoft package repository |
| acs-mirror.azureedge.net | ACS mirror for artifacts |
| eastus.data.mcr.microsoft.com | Regional MCR data endpoint |

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

1. **Ships hardcoded IPs** in the VHD for immediate availability at boot
2. **Periodically refreshes** the hosts file by querying system DNS
3. **Stores results** in a local hosts file (`/etc/localdns/hosts`)
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
┌───────────────────────────────────────────────────────────────────────────┐
│                               AKS Node                                     │
│                                                                            │
│  ┌────────────────┐         ┌───────────────────┐                         │
│  │  aks-hosts-    │────────▶│  aks-hosts-       │                         │
│  │  setup.timer   │ triggers│  setup.sh         │                         │
│  └────────────────┘         └─────────┬─────────┘                         │
│    (every 15 min)                     │                                   │
│                                       │ 1. queries DNS                    │
│                                       ▼                                   │
│                             ┌─────────────────────┐                       │
│                             │  System DNS Server  │                       │
│                             │  (Azure DNS or      │                       │
│                             │   Custom DNS)       │                       │
│                             └─────────┬───────────┘                       │
│                                       │ 2. returns IPs                    │
│                                       ▼                                   │
│                             ┌───────────────────┐                         │
│                             │  /etc/localdns/   │                         │
│                             │      hosts        │◀─── 3. script writes    │
│                             └─────────┬─────────┘                         │
│                                       │                                   │
│                                       │ 4. LocalDNS reads hosts file      │
│                                       ▼                                   │
│                                                                            │
│  ┌────────────────┐         ┌─────────────────────────────────────────┐   │
│  │   Pods/        │────────▶│            LocalDNS (CoreDNS)           │   │
│  │   Kubelet      │  query  │                                         │   │
│  └────────────────┘         │  ┌─────────────┐    ┌────────────────┐  │   │
│                             │  │ hosts plugin│───▶│ forward plugin │  │   │
│                             │  │ (check file │    │ (query upstream│  │   │
│                             │  │  first)     │    │  DNS server)   │  │   │
│                             │  └─────────────┘    └────────────────┘  │   │
│                             │        │                    │           │   │
│                             │        ▼                    ▼           │   │
│                             │   Found in file?      Upstream DNS      │   │
│                             │   Return immediately  Server            │   │
│                             └─────────────────────────────────────────┘   │
│                                                                            │
└───────────────────────────────────────────────────────────────────────────┘
```

**Query Flow in LocalDNS:**
1. DNS query arrives at LocalDNS (CoreDNS) on 169.254.10.10:53
2. **hosts plugin** checks `/etc/localdns/hosts` for a matching entry
3. If found → return IP immediately (no external query needed)
4. If not found → **fallthrough** to the next plugin (forward plugin)
5. **forward plugin** queries the upstream DNS server

### Component Interaction

```
                    Boot Sequence
                         │
                         ▼
              ┌──────────────────┐
              │ /etc/localdns/   │
              │     hosts        │ ◀── (1) Hardcoded IPs already present in VHD
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │   localdns       │
              │   .service       │ ◀── (2) LocalDNS starts with hardcoded entries
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │   kubelet        │
              │   .service       │ ◀── (3) Kubelet starts (can pull images immediately)
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │  aks-hosts-      │
              │  setup.timer     │ ◀── (4) Timer fires (OnBootSec=0)
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │  aks-hosts-      │
              │  setup.sh        │ ◀── (5) Script queries DNS and refreshes hosts file
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │ /etc/localdns/   │
              │     hosts        │ ◀── (6) Hosts file updated with fresh IPs
              └──────────────────┘
```

**Ordering Guarantees:**
- `aks-hosts-setup.service` runs **after** network is online (After=network-online.target)
- `aks-hosts-setup.service` runs **before** LocalDNS and kubelet (Before=kubelet.service localdns.service)
- `localdns.service` starts **before** kubelet to ensure DNS is ready for container pulls

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

### 2. VHD Preloaded Hosts File

**File:** `vhdbuilder/packer/packer_source.sh`
**Location on node:** `/etc/localdns/hosts`

#### Purpose

The VHD ships with a **hardcoded hosts file** containing known-good IP addresses for critical Azure endpoints. This ensures DNS resolution works immediately at boot, before the timer has a chance to refresh from live DNS.

#### Benefits

| Benefit | Description |
|---------|-------------|
| Immediate availability | DNS works from the moment LocalDNS starts |
| No boot-time DNS dependency | Node can start pulling images before network DNS is verified |
| Fallback resilience | If DNS is slow/unavailable at boot, hardcoded IPs still work |

#### Hardcoded IPs

```
# mcr.microsoft.com - Microsoft Container Registry
20.61.99.68 mcr.microsoft.com
2603:1061:1002::2 mcr.microsoft.com

# packages.aks.azure.com - AKS packages
20.7.0.233 packages.aks.azure.com

# login.microsoftonline.com - Azure AD authentication
20.190.151.68 login.microsoftonline.com
20.190.151.70 login.microsoftonline.com
20.190.151.67 login.microsoftonline.com
20.190.151.69 login.microsoftonline.com
2603:1037:1:c8::8 login.microsoftonline.com
2603:1036:3000:d8::5 login.microsoftonline.com
2603:1037:1:c8::9 login.microsoftonline.com
2603:1037:1:c8::a login.microsoftonline.com

# management.azure.com - Azure Resource Manager
20.37.158.0 management.azure.com
2603:1030:408:6::3e8 management.azure.com

# packages.microsoft.com - Microsoft packages
52.184.220.97 packages.microsoft.com
2600:1417:76:1a2::e59 packages.microsoft.com

# acs-mirror.azureedge.net - AKS container images mirror
152.199.39.108 acs-mirror.azureedge.net
2606:2800:233:1cb7:261b:1f9c:2074:3c acs-mirror.azureedge.net

# eastus.data.mcr.microsoft.com - MCR data endpoint (regional)
204.79.197.219 eastus.data.mcr.microsoft.com
2620:1ec:bdf::50 eastus.data.mcr.microsoft.com
```

#### IP Address Maintenance

These hardcoded IPs should be updated periodically as part of VHD releases:
- IPs are stable Azure anycast addresses
- The runtime timer will update to fresh IPs shortly after boot
- Even stale hardcoded IPs typically continue working (Azure maintains backward compatibility)

---

### 4. Systemd Timer

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

### 5. Systemd Service

**File:** `parts/linux/cloud-init/artifacts/aks-hosts-setup.service`
**Location on node:** `/etc/systemd/system/aks-hosts-setup.service`

#### Configuration

| Setting | Value | Purpose |
|---------|-------|---------|
| Type | oneshot | Script runs once per trigger |
| After | network-online.target | Ensures network is available |
| Before | kubelet.service, localdns.service | Runs before consumers |

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

### 6. CoreDNS Configuration Update

**File:** `pkg/agent/baker.go`

#### Change Description

The LocalDNS Corefile template is modified to include a `hosts` plugin block that checks `/etc/localdns/hosts` before forwarding to the upstream DNS server.

#### Before

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

#### After

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

#### Fallthrough Behavior

The `fallthrough` directive ensures that:
- Queries matching hosts file entries return cached IPs
- Queries NOT in hosts file are forwarded to the upstream DNS server
- No queries are blocked or dropped

---

### 5. CSE Integration

**Files:**
- `parts/linux/cloud-init/artifacts/cse_config.sh`
- `parts/linux/cloud-init/artifacts/cse_main.sh`

#### New Function: enableAKSHostsSetup()

```bash
enableAKSHostsSetup() {
    echo "Enabling aks-hosts-setup timer..."
    # 30 = timeout in seconds for systemctl enable/start operation
    systemctlEnableAndStart aks-hosts-setup.timer 30 || exit $ERR_SYSTEMCTL_START_FAIL
    echo "aks-hosts-setup timer enabled successfully."
}
```

#### Activation Logic

The timer is only enabled when LocalDNS is enabled:

```bash
if [ "${SHOULD_ENABLE_LOCALDNS}" = "true" ]; then
    logs_to_events "AKS.CSE.enableLocalDNSForScriptless" enableLocalDNSForScriptless
    logs_to_events "AKS.CSE.enableAKSHostsSetup" enableAKSHostsSetup
fi
```

---

## Data Flow

### Boot Time Flow

| Step | Action | Component |
|------|--------|-----------|
| 1 | Node boots | System |
| 2 | **Hardcoded hosts file already present** | /etc/localdns/hosts (from VHD) |
| 3 | Network comes online | systemd |
| 4 | LocalDNS starts (uses hardcoded IPs) | localdns.service |
| 5 | Kubelet starts | kubelet.service |
| 6 | Timer triggers (OnBootSec=0) | aks-hosts-setup.timer |
| 7 | Script refreshes with live DNS | aks-hosts-setup.sh |
| 8 | Hosts file updated with fresh IPs | /etc/localdns/hosts |

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
| 2 | LocalDNS receives query on 169.254.10.10:53 | CoreDNS processes query |
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
| **Improved Reliability** | DNS failures for critical FQDNs don't immediately impact operations |
| **Reduced Latency** | Local cache eliminates DNS round-trip for cached domains |
| **Resilience** | Nodes can continue operating briefly during DNS outages |
| **Self-Healing** | Timer automatically refreshes cache every 15 minutes |
| **Thundering Herd Prevention** | RandomizedDelaySec prevents cluster-wide simultaneous resolution |
| **Zero Configuration** | Automatic when LocalDNS is enabled |

---

## Failure Handling

| Failure Scenario | Behavior | Impact |
|------------------|----------|--------|
| DNS resolution fails on boot | Script exits 0, timer retries in 15 min | Hosts file empty, all queries go to upstream DNS |
| nslookup not available | Script fails gracefully | All queries go to upstream DNS |
| Hosts file write fails | Existing file preserved, error logged | Stale cache used |
| Invalid DNS response | Filtered by regex, not written | Valid entries only |
| All resolutions fail | Existing hosts file preserved | Stale cache used |
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
| pkg/agent/baker_test.go | CoreDNS Corefile generation |

### Manual Verification

1. Deploy node with LocalDNS enabled
2. Verify timer is running: `systemctl status aks-hosts-setup.timer`
3. Check hosts file: `cat /etc/localdns/hosts`
4. Verify CoreDNS config: `kubectl exec -n kube-system localdns-xxx -- cat /etc/coredns/Corefile`

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
| Flatcar Container Linux | x64/ARM64 | ⚠️ Degrades gracefully | nslookup not available; script runs but no hosts cached |

### Feature Requirements

| Requirement | Status | Notes |
|-------------|--------|-------|
| LocalDNS enabled | ✅ Required | Feature dependency - timer only enabled when LocalDNS is active |
| Scriptless provisioning | ✅ Supported | Timer enabled via CSE |
| Legacy provisioning | ✅ Supported | Timer enabled via CSE |

### Graceful Degradation

If `nslookup` is not available on a distro (e.g., Flatcar):
- The timer and service run without errors (exit 0)
- No IP addresses are cached in `/etc/localdns/hosts`
- All DNS queries fall through to the upstream DNS server
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
| parts/linux/cloud-init/artifacts/cse_config.sh | Modified | Adds `enableAKSHostsSetup()` function to enable and start the timer |
| parts/linux/cloud-init/artifacts/cse_main.sh | Modified | Calls `enableAKSHostsSetup()` when LocalDNS is enabled |
| pkg/agent/baker.go | Modified | Adds `hosts /etc/localdns/hosts` plugin block to LocalDNS Corefile template |
| pkg/agent/baker_test.go | Modified | Updates expected Corefile output to include hosts plugin |
| vhdbuilder/packer/*.json | Modified | Uploads new artifacts to `/home/packer/` during VHD build |
| vhdbuilder/packer/packer_source.sh | Modified | Copies artifacts to final locations and writes hardcoded `/etc/localdns/hosts` |

### References

- [CoreDNS Hosts Plugin](https://coredns.io/plugins/hosts/)
- [Systemd Timer Documentation](https://www.freedesktop.org/software/systemd/man/systemd.timer.html)
- [AKS LocalDNS Feature](https://learn.microsoft.com/en-us/azure/aks/local-dns)
