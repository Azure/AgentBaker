# LocalDNS Scriptless Test Fix Summary

## Problem

The `Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless` E2E test was failing with exit code 124 (timeout). The localdns service was stuck in an infinite restart loop and never completed provisioning.

## Root Cause Analysis

### Investigation Process

1. **Symptom**: CSE timed out after 15 minutes, exit code 124
2. **Observation**: localdns.sh script repeatedly started, waited ~30 seconds, then triggered cleanup
3. **Key Finding**: localdns.sh never reached `systemd-notify --ready` to signal service startup completion

### The Problem

The scriptless test was missing DNS overrides in the LocalDnsProfile configuration:

```go
// BROKEN - Missing DNS overrides
aksNodeConfig.LocalDnsProfile = &aksnodeconfigv1.LocalDnsProfile{
    EnableLocalDns:    true,
    EnableHostsPlugin: true,
}
```

**Why this broke localdns:**

1. **No DNS overrides** → CoreDNS corefile only generated the `health-check.localdns.local:53` block
2. **No server blocks** → No `ready 169.254.10.10:8181` health endpoint configured
3. **Health check fails** → `curl -s http://169.254.10.10:8181/ready` times out (CoreDNS not listening on port 8181)
4. **wait_for_localdns_ready() fails** → Script triggers cleanup after ~30 seconds
5. **Service restarts** → Infinite loop, never calls `systemd-notify --ready`
6. **CSE times out** → Exit code 124 after 15 minutes

### Why Non-Scriptless Tests Worked

The non-scriptless tests use `BootstrapConfigMutator` which operates on a `NodeBootstrappingConfiguration` (NBC) that already has default DNS overrides from `baseTemplateLinux()`:

```go
// baseTemplateLinux() sets up full LocalDNSProfile with VnetDNSOverrides and KubeDNSOverrides
// Test mutator only modifies EnableLocalDNS and EnableHostsPlugin, preserving the overrides
nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
```

The scriptless test uses `AKSNodeConfigMutator` which **replaces the entire LocalDnsProfile**, wiping out the default DNS overrides that were set in `nbcToAKSNodeConfigV1()`.

## The Fix

Added full DNS overrides to the scriptless test's LocalDnsProfile configuration:

```go
aksNodeConfig.LocalDnsProfile = &aksnodeconfigv1.LocalDnsProfile{
    EnableLocalDns:       true,
    EnableHostsPlugin:    true,
    CpuLimitInMilliCores: to.Ptr(int32(2008)),
    MemoryLimitInMb:      to.Ptr(int32(128)),
    VnetDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
        ".": { /* root domain config with health endpoint */ },
        "cluster.local": { /* cluster DNS config */ },
        "testdomain456.com": { /* test domain */ },
    },
    KubeDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
        ".": { /* root domain for ClusterFirst pods */ },
        "cluster.local": { /* cluster DNS */ },
        "testdomain567.com": { /* test domain */ },
    },
}
```

**Result**: The CoreDNS corefile now includes server blocks with the `ready 169.254.10.10:8181` health endpoint, allowing the health check to succeed and localdns to start properly.

## Files Modified

- `e2e/scenario_localdns_hosts_test.go`:
  - Added DNS overrides to `Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless`
  - Added import for `github.com/Azure/azure-sdk-for-go/sdk/azcore/to`
  - Added comment explaining why DNS overrides are required

## Technical Details

### CoreDNS Corefile Template Structure

The corefile template (`pkg/agent/baker.go` line 1894) has:

1. **health-check.localdns.local:53** - Always present, uses `whoami` plugin
2. **VnetDNS server blocks** - Generated from `$.VnetDNSOverrides` map
3. **KubeDNS server blocks** - Generated from `$.KubeDNSOverrides` map

Each VnetDNS/KubeDNS server block includes:
- `ready 169.254.10.10:8181` - Health endpoint for localdns.sh health checks
- `forward`, `cache`, `prometheus` - CoreDNS plugins for DNS functionality

### localdns.sh Health Check

Line 55 in `parts/linux/cloud-init/artifacts/localdns.sh`:
```bash
CURL_COMMAND="curl -s http://${LOCALDNS_NODE_LISTENER_IP}:8181/ready"
```

The `wait_for_localdns_ready()` function (lines 344-369) polls this endpoint until it returns "OK". Without DNS overrides, this endpoint never exists, causing the health check to fail.

## Verification

To verify the fix works, run:

```bash
cd e2e
go test -v -timeout 90m -run '^Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless$'
```

Expected: Test passes, localdns service starts successfully, health checks pass.
