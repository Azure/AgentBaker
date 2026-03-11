# LocalDNS Hosts Plugin E2E Test Results

**Date:** March 6, 2026
**Test Location:** westus2
**KEEP_VMSS:** ✅ Enabled - All VMs retained for debugging

---

## Executive Summary

**Overall Status:** ⚠️ **Partial Success** - 3/5 tests passed

- ✅ **3 Tests PASSED** - Core functionality working
- ❌ **2 Tests FAILED** - Issues with localdns service startup
- 🔧 **Action Required:** Investigate localdns service startup failures

---

## Test Results

### ✅ PASSED Tests (3/5)

#### 1. Test_Ubuntu2204_LocalDNSHostsPlugin ✅
- **Duration:** 275.61s (~4.6 minutes)
- **OS/VHD:** Ubuntu 22.04 Gen2 Containerd
- **VMSS:** `fkms-2026-03-06-ubuntu2204localdnshostsplugin`
- **Logs:** `e2e/scenario-logs/Test_Ubuntu2204_LocalDNSHostsPlugin/`

#### 2. Test_Ubuntu2404_LocalDNSHostsPlugin ✅
- **Duration:** 334.90s (~5.6 minutes)
- **OS/VHD:** Ubuntu 24.04 Gen2 Containerd
- **VMSS:** `043y-2026-03-06-ubuntu2404localdnshostsplugin`
- **Logs:** `e2e/scenario-logs/Test_Ubuntu2404_LocalDNSHostsPlugin/`

#### 3. Test_AzureLinuxV3_LocalDNSHostsPlugin ✅
- **Duration:** 342.41s (~5.7 minutes)
- **OS/VHD:** Azure Linux V3 Gen2
- **VMSS:** `rlbx-2026-03-06-azurelinuxv3localdnshostsplugin`
- **Logs:** `e2e/scenario-logs/Test_AzureLinuxV3_LocalDNSHostsPlugin/`

---

### ❌ FAILED Tests (2/5)

#### 4. Test_Ubuntu2204_LocalDNSHostsPlugin_OldVHD_GracefulFallback ❌
- **Duration:** 744.67s (~12.4 minutes)
- **Exit Code:** 216
- **VMSS:** `npes-2026-03-06-ubuntu2204localdnshostspluginoldvhdgracef`
- **Logs:** `e2e/scenario-logs/Test_Ubuntu2204_LocalDNSHostsPlugin_OldVHD_GracefulFallback/`

**Error:**
```
Unit localdns.service could not be found.
Failed to restart localdns.service: Unit localdns.service not found.
localdns could not be started (exit 216)
```

**Root Cause:**
- Old VHD has `UnsupportedLocalDns=true` flag (doesn't have localdns artifacts)
- CSE attempts to start localdns service anyway
- Retried 100 times before failing
- **Expected behavior:** Should detect flag and skip localdns setup gracefully

#### 5. Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless ❌
- **Duration:** 1022.38s (~17 minutes)
- **Exit Code:** 124 (timeout)
- **VMSS:** `lmah-2026-03-06-ubuntu2204localdnshostspluginscriptless`
- **Logs:** `e2e/scenario-logs/Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless/`

**Error:**
```
aks-node-controller failed: provision failed: exitCode=124
systemctl restart localdns - timed out after 30 seconds
```

**Root Cause:**
- CoreDNS started successfully (PID 23289, listening on 169.254.10.10, 169.254.10.11)
- Network configuration completed
- `systemctl restart localdns` timed out after 30 seconds
- **Likely issue:** Systemd unit misconfiguration in scriptless/aks-node-controller mode

---

## Retained VMSS Resources

⚠️ **All 5 VMSS were retained** for debugging (KEEP_VMSS=true). **Manual deletion required!**

### Resource Details

**Location:** westus2  
**Resource Group:** abe2e-westus2  
**Subscription:** 8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8

| Test Status | VMSS Name | Test Scenario |
|------------|-----------|---------------|
| ✅ PASS | `fkms-2026-03-06-ubuntu2204localdnshostsplugin` | Ubuntu 22.04 Standard |
| ✅ PASS | `043y-2026-03-06-ubuntu2404localdnshostsplugin` | Ubuntu 24.04 Standard |
| ✅ PASS | `rlbx-2026-03-06-azurelinuxv3localdnshostsplugin` | Azure Linux V3 |
| ❌ FAIL | `npes-2026-03-06-ubuntu2204localdnshostspluginoldvhdgracef` | Old VHD Fallback |
| ❌ FAIL | `lmah-2026-03-06-ubuntu2204localdnshostspluginscriptless` | Scriptless Mode |

### Cleanup Commands

```bash
# Delete all VMSS at once
az vmss delete --name fkms-2026-03-06-ubuntu2204localdnshostsplugin \
  --resource-group abe2e-westus2 --subscription 8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8 --no-wait

az vmss delete --name 043y-2026-03-06-ubuntu2404localdnshostsplugin \
  --resource-group abe2e-westus2 --subscription 8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8 --no-wait

az vmss delete --name rlbx-2026-03-06-azurelinuxv3localdnshostsplugin \
  --resource-group abe2e-westus2 --subscription 8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8 --no-wait

az vmss delete --name npes-2026-03-06-ubuntu2204localdnshostspluginoldvhdgracef \
  --resource-group abe2e-westus2 --subscription 8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8 --no-wait

az vmss delete --name lmah-2026-03-06-ubuntu2204localdnshostspluginscriptless \
  --resource-group abe2e-westus2 --subscription 8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8 --no-wait
```

Or delete all from the resource group:
```bash
az vmss list --resource-group abe2e-westus2 --query "[?starts_with(name, '2026-03-06')].name" -o tsv | \
  xargs -I {} az vmss delete --name {} --resource-group abe2e-westus2 --no-wait
```

---

## Key Findings

### ✅ What Works

1. **Core LocalDNS Hosts Plugin Functionality**
   - Successfully deploys on Ubuntu 22.04, 24.04, and Azure Linux V3
   - Hosts file is populated with resolved IPs for critical FQDNs:
     - mcr.microsoft.com
     - login.microsoftonline.com
     - acs-mirror.azureedge.net
     - management.azure.com (22.04 only)
     - packages.aks.azure.com (22.04 only)
     - packages.microsoft.com (22.04 only)
   - DNS resolution bypasses work correctly via hosts plugin
   - aks-hosts-setup.service runs successfully
   - aks-hosts-setup.timer is active and working

### ❌ What's Broken

#### 1. **Old VHD Backward Compatibility** (High Priority)

**Problem:** CSE tries to start localdns on old VHDs that don't support it

**Details:**
- Old VHD has `UnsupportedLocalDns=true` in VHD manifest
- This flag indicates the VHD lacks localdns artifacts
- CSE ignores this flag and attempts to start localdns.service
- Service doesn't exist → fails after 100 retry attempts → exit 216

**Expected Behavior:**
- CSE should check `UnsupportedLocalDns` flag before attempting localdns setup
- If true, skip localdns initialization entirely (graceful degradation)
- Node should provision successfully without localdns

**Impact:** 
- Breaks backward compatibility with older VHDs
- Prevents nodes from joining clusters when using old base images
- Critical for safe rollout of new CSE versions

**Files to Fix:**
- `parts/linux/cloud-init/artifacts/cse_helpers.sh` - Feature detection logic
- `parts/linux/cloud-init/artifacts/cse_main.sh` - Localdns startup logic
- Look for: `getUnsupportedLocalDns()` or similar function

#### 2. **Scriptless Mode Systemd Timeout** (High Priority)

**Problem:** systemctl restart localdns times out in aks-node-controller mode

**Details:**
- Scriptless path uses aks-node-controller instead of CSE scripts
- CoreDNS binary starts successfully via `systemd-cat`
- Localdns listens correctly on 169.254.10.10 and 169.254.10.11
- Network configuration completes successfully
- `systemctl restart localdns` command times out after 30 seconds
- Provisioning fails with exit code 124 (timeout)

**Likely Root Causes:**
- Systemd unit file not created properly by aks-node-controller
- Systemd unit exists but has incorrect configuration (Type=, dependencies, etc.)
- Unit is a oneshot that doesn't properly handle restart
- Missing systemd unit file entirely (trying to restart non-existent service)

**Impact:**
- Blocks scriptless provisioning path adoption
- Affects future aks-node-controller migration strategy

**Files to Investigate:**
- Systemd unit generation code in aks-node-controller
- `parts/linux/cloud-init/artifacts/localdns.sh` - Systemd interaction
- Check: How does scriptless mode create localdns.service?

---

## Debugging Recommendations

### For OldVHD Test
```bash
# SSH into the failed VM
az vmss list-instances --name npes-2026-03-06-ubuntu2204localdnshostspluginoldvhdgracef \
  --resource-group abe2e-westus2 -o table

# Once connected, check:
1. VHD manifest: cat /opt/azure/vhd-install.complete
2. CSE log: cat /var/log/azure/aks/cluster-provision.log | grep -i "localdns\|UnsupportedLocalDns"
3. Systemd units: systemctl list-unit-files | grep localdns
4. Check if localdns artifacts exist: ls -la /opt/azure/containers/localdns/
```

### For Scriptless Test
```bash
# SSH into the failed VM
az vmss list-instances --name lmah-2026-03-06-ubuntu2204localdnshostspluginscriptless \
  --resource-group abe2e-westus2 -o table

# Once connected, check:
1. Systemd unit file: systemctl cat localdns
2. Systemd status: systemctl status localdns --no-pager -l
3. Check if coredns is actually running: ps aux | grep coredns
4. aks-node-controller logs: journalctl -u aks-node-controller -n 200
5. Check systemd unit creation: ls -la /etc/systemd/system/localdns*
```

---

## Logs Available

All scenario logs preserved in: **`e2e/scenario-logs/`**

Each test directory contains:
- `cluster-provision.log` - Main CSE provisioning log
- `cluster-provision-cse-output.log` - CSE script output
- `kubelet.log` - Kubelet service logs
- `aks-node-controller.log` - Node controller logs (scriptless)
- `serial-console-vm-0.log` - VM serial console output
- `journalctl` - Full systemd journal
- `syslog` - System log
- `azure.json` - Azure cloud provider config
- `sshkey` - SSH private key for VM access
- And more...

---

## Configuration Used

```bash
E2E_LOCATION=westus2
GALLERY_NAME=PackerSigGalleryEastUS
SIG_VERSION_TAG_NAME=buildId
SIG_VERSION_TAG_VALUE=155648650
KEEP_VMSS=true
```

**Image Versions:**
- Ubuntu 22.04: `1.1772760851.25282`
- Ubuntu 24.04: `1.1772761191.9801`
- Azure Linux V3: `1.1772760844.18907`
- Old VHD: `1.1704411049.2812`

---

## Next Steps

1. **Fix Old VHD Compatibility** (Priority 1)
   - Add check for `UnsupportedLocalDns` flag in CSE
   - Skip localdns setup if flag is true
   - Test with old VHD to verify graceful fallback

2. **Fix Scriptless Systemd Issue** (Priority 1)
   - Investigate systemd unit creation in aks-node-controller
   - Fix timeout in systemctl restart localdns
   - Ensure unit file is properly configured

3. **Add Unit Tests**
   - Test feature detection logic for localdns support
   - Test systemd unit generation in scriptless mode
   - Verify backward compatibility checks

4. **Clean Up Resources**
   - Delete the 5 retained VMSS when done debugging
   - Use cleanup commands provided above

---

**Total Test Duration:** ~17 minutes  
**Report Generated:** March 6, 2026 at 03:20 UTC
