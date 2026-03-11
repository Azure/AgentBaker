# LocalDNS Hosts Plugin E2E Test Results

**Date:** March 6, 2026
**Test Location:** westus2
**KEEP_VMSS:** ✅ Enabled (VMs retained for debugging)

---

## Executive Summary

**Overall Status:** ⚠️ **Partial Success** - 3/5 tests passed

- ✅ **3 Tests PASSED** - Core functionality working
- ❌ **2 Tests FAILED** - Issues with localdns service startup
- 🔧 **Action Required:** Investigate localdns service startup failures

---

## Test Results by Scenario

### ✅ PASSED Tests

#### 1. Test_Ubuntu2204_LocalDNSHostsPlugin
- **Status:** ✅ PASS
- **Duration:** 275.61s (~4.6 minutes)
- **OS/VHD:** Ubuntu 22.04 Gen2 Containerd
- **Description:** Tests localdns hosts plugin works correctly on Ubuntu 22.04 with dynamic IP resolution
- **Scenario Logs:** `e2e/scenario-logs/Test_Ubuntu2204_LocalDNSHostsPlugin/`

#### 2. Test_Ubuntu2404_LocalDNSHostsPlugin
- **Status:** ✅ PASS
- **Duration:** 334.90s (~5.6 minutes)
- **OS/VHD:** Ubuntu 24.04 Gen2 Containerd
- **Description:** Tests localdns hosts plugin works correctly on Ubuntu 24.04
- **Scenario Logs:** `e2e/scenario-logs/Test_Ubuntu2404_LocalDNSHostsPlugin/`

#### 3. Test_AzureLinuxV3_LocalDNSHostsPlugin
- **Status:** ✅ PASS
- **Duration:** 342.41s (~5.7 minutes)
- **OS/VHD:** Azure Linux V3 Gen2
- **Description:** Tests localdns hosts plugin works correctly on Azure Linux V3
- **Scenario Logs:** `e2e/scenario-logs/Test_AzureLinuxV3_LocalDNSHostsPlugin/`

---

### ❌ FAILED Tests

#### 4. Test_Ubuntu2204_LocalDNSHostsPlugin_OldVHD_GracefulFallback
- **Status:** ❌ FAIL
- **Duration:** 744.67s (~12.4 minutes)
- **OS/VHD:** Ubuntu 22.04 Gen2 Containerd Private KubePkg (Old VHD)
- **Description:** Tests backward compatibility with old VHDs that don't have aks-hosts-setup artifacts
- **Exit Code:** 216
- **Error:** `VMExtensionProvisioningError`

**Root Cause:**
```
Unit localdns.service could not be found.
Failed to restart localdns.service: Unit localdns.service not found.
localdns could not be started
```

**Analysis:**
- The test was intended to verify graceful fallback when aks-hosts-setup artifacts are missing
- The VHD has `UnsupportedLocalDns=true` flag, which should disable localdns
- However, CSE is attempting to start localdns service, which doesn't exist on this old VHD
- The CSE retried 100 times (exit attempts 1-100) before failing with exit code 216

**Scenario Logs:** `e2e/scenario-logs/Test_Ubuntu2204_LocalDNSHostsPlugin_OldVHD_GracefulFallback/`

**VMSS Name:** `npes-2026-03-06-ubuntu2204localdnshostspluginoldvhdgracef`
**Status:** ⚠️ **Retained for debugging** - Manual deletion required

---

#### 5. Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless
- **Status:** ❌ FAIL
- **Duration:** 1022.38s (~17 minutes)
- **OS/VHD:** Ubuntu 22.04 Gen2 Containerd
- **Description:** Tests localdns hosts plugin works correctly on Ubuntu 22.04 scriptless path (aks-node-controller)
- **Exit Code:** 124 (timeout)
- **Error:** `VMExtensionProvisioningError`

**Root Cause:**
```
ExitCode: 124
aks-node-controller failed: provision failed: exitCode=124
systemctl restart localdns - timed out after 30 seconds
```

**Analysis:**
- Localdns setup partially succeeded (coredns started, network configured)
- CoreDNS-1.13.2 started successfully on 169.254.10.10 and 169.254.10.11
- The issue occurred when trying to restart localdns via systemctl
- The `systemctl restart localdns` command timed out after 30 seconds
- This suggests the systemd unit for localdns may not be properly configured in scriptless mode

**Key Log Snippets:**
```
Mar 06 03:16:02 localdns.sh: Setting up localdns dummy interface with IPs 169.254.10.10 and 169.254.10.11.
Mar 06 03:16:02 localdns-coredns: CoreDNS-1.13.2
Mar 06 03:16:02 localdns-coredns: linux/amd64, go1.25.7
Mar 06 03:16:03 localdns.sh: Localdns PID is 23289.
Mar 06 03:16:03 localdns.sh: Waiting for localdns to start and be able to serve traffic.
+ timeout 30 systemctl daemon-reload
+ timeout 30 systemctl restart localdns
[TIMEOUT after 30s]
```

**Scenario Logs:** `e2e/scenario-logs/Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless/`

**VMSS Name:** `lmah-2026-03-06-ubuntu2204localdnshostspluginscriptless`
**Status:** ⚠️ **Retained for debugging** - Manual deletion required

---

## Retained VMSS Resources

⚠️ **Important:** The following VMSS were kept for debugging (KEEP_VMSS=true). **You must manually delete them** when done investigating.

### VMSS to Delete Manually

1. **Test_Ubuntu2204_LocalDNSHostsPlugin_OldVHD_GracefulFallback**
   - VMSS Name: `npes-2026-03-06-ubuntu2204localdnshostspluginoldvhdgracef`
   - Location: westus2
   - Status: Failed (exit 216 - localdns.service not found)

2. **Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless**
   - VMSS Name: `lmah-2026-03-06-ubuntu2204localdnshostspluginscriptless`
   - Location: westus2
   - Status: Failed (exit 124 - systemctl restart timeout)

### Cleanup Commands

```bash
# Delete VMSS for OldVHD test
az vmss delete --name npes-2026-03-06-ubuntu2204localdnshostspluginoldvhdgracef \
  --resource-group abe2e-westus2 \
  --subscription 8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8

# Delete VMSS for Scriptless test
az vmss delete --name lmah-2026-03-06-ubuntu2204localdnshostspluginscriptless \
  --resource-group abe2e-westus2 \
  --subscription 8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8
```

---

## Recommendations

### High Priority Issues

1. **OldVHD Graceful Fallback Test Failure**
   - **Issue:** CSE attempts to start localdns service on old VHDs that don't have it
   - **Expected:** Should detect `UnsupportedLocalDns=true` and skip localdns setup
   - **Action:** Review CSE logic that checks for localdns support before attempting to start the service

2. **Scriptless Localdns Systemd Timeout**
   - **Issue:** `systemctl restart localdns` times out in scriptless/aks-node-controller mode
   - **Root Cause:** Possible systemd unit misconfiguration or missing dependencies
   - **Action:** Review systemd unit file for localdns in scriptless mode

---

**Report Generated:** March 6, 2026
**Tests Passed:** 3/5
