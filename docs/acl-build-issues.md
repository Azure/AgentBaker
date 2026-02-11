# ACL VHD Build — Issue Tracker

Tracking issues encountered while building ACL (Azure Container Linux) VHDs using the Flatcar packer pipeline.

---

## Issue 1: OS detection — `isFlatcar()` doesn't recognize ACL

**Status**: Fixed
**Date**: 2026-02-10
**Symptom**: Build fails with `rsyslog could not be started` / `exit 1` in `pre-install-dependencies.sh`.

**Root cause**: ACL's `/etc/os-release` has `ID=acl`, which gets uppercased to `ACL`. The `isFlatcar()` function only checked for `FLATCAR`, so it returned false for ACL. This caused the build to try starting `rsyslog.service` (which ACL doesn't have, like Flatcar) and fail.

**Affected files/functions**:
- `parts/linux/cloud-init/artifacts/cse_helpers.sh` — `isFlatcar()` function (line ~814)
- `vhdbuilder/packer/vhd-scanning.sh` — `isFlatcar()` function (line ~253)
- `vhdbuilder/packer/pre-install-dependencies.sh` — callers at lines 49, 65
- `vhdbuilder/packer/packer_source.sh` — caller at line 411

**Fix**:
- Added `ACL_OS_NAME="ACL"` constant to `cse_helpers.sh`
- Updated `isFlatcar()` in `cse_helpers.sh` to also check `$os = $ACL_OS_NAME`
- Updated `isFlatcar()` in `vhd-scanning.sh` to also check `$os = "ACL"`

**Log reference**: `failed.log` lines 503, 768–773, 1847–1851, 3040–3063

---

## Issue 2: `disk_queue.service` fails — missing Azure disk udev rules

**Status**: Fixed (workaround in AgentBaker; root fix needed in ACL image)
**Date**: 2026-02-10
**Symptom**: Build fails with `disk_queue could not be started` / `exit 1` in `pre-install-dependencies.sh`.

**Root cause**: `disk_queue.service` runs `disk_queue.sh`, which requires `/dev/disk/azure/root` or `/dev/disk/azure/os` symlinks. These symlinks are created by `80-azure-disk.rules` udev rules. On ACL, neither source of these rules is present:

1. **WALinuxAgent skips udev rules on Flatcar >= 3550** — ACL is Flatcar-based (v4459), so the patched WALinuxAgent assumes the OS already ships the rules and calls `set_udev_files()` is skipped (see `0001-flatcar-changes.patch` in acl-scripts).
2. **`azure-vm-utils` is missing from the ACL image** — Real Flatcar ships `azure-vm-utils` v0.7.0 (which provides `80-azure-disk.rules`), and the ACL `coreos-0.0.1-r318.ebuild` lists it as a dependency, but the tested ACL image (v4459.2.2) doesn't have it installed.

This creates a gap unique to ACL: WALinuxAgent won't install the rules, and `azure-vm-utils` isn't there either.

**Why other distros are unaffected**:
- **Flatcar**: `azure-vm-utils` v0.7.0 is in the base image → rules exist at boot
- **Ubuntu / Azure Linux 3**: WALinuxAgent installs its own udev rules → symlinks exist

**Affected files**:
- `vhdbuilder/packer/pre-install-dependencies.sh` — `disk_queue` started at line 116
- `parts/linux/cloud-init/artifacts/disk_queue.sh` — checks for `/dev/disk/azure/{root,os}`

**Fix (workaround)**: Install `80-azure-disk.rules` in `pre-install-dependencies.sh` before starting `disk_queue`, guarded by a file-existence check so it's a no-op on distros that already have the rules. Same guard added to `install-dependencies.sh` for idempotency.

**Proper fix**: Ensure the ACL base image ships `azure-vm-utils` so the rules are present at boot, matching Flatcar behavior.

---

## Issue 3: Packer deprovision fails — `/usr/sbin/waagent` not found

**Status**: Fixed
**Date**: 2026-02-10
**Symptom**: Build completes all install phases successfully, then fails at the final Packer deprovision step with exit code 125:
```
sudo: /usr/sbin/waagent: command not found
Script exited with non-zero exit status: 125
```

**Root cause**: The Packer template `vhd-image-builder-flatcar.json` (line 774) hardcodes:
```
sudo /usr/sbin/waagent -force -deprovision+user && export HISTSIZE=0 && sync || exit 125
```

On **Flatcar**, `/usr/sbin` is a symlink to `/usr/bin` (usr-merge), so `/usr/sbin/waagent` resolves to `/usr/bin/waagent` transparently. On **ACL**, `/usr/sbin` is a real directory with ~400+ binaries, and the Azure Linux `waagent` RPM only installs to `/usr/bin/waagent`. There is no `/usr/sbin/waagent`.

**Why Flatcar works**: `ls -ld /usr/sbin` → `lrwxrwxrwx. 1 root root 3 /usr/sbin -> bin`. Same inode for both paths.

**Why ACL fails**: `/usr/sbin` is a separate directory. `waagent` exists only at `/usr/bin/waagent`.

**Affected files**:
- `vhdbuilder/packer/vhd-image-builder-flatcar.json` — line 774 (hardcoded `/usr/sbin/waagent`)
- All other packer templates also hardcode the same path (`vhd-image-builder-base.json`, `vhd-image-builder-arm64-gen2.json`, `vhd-image-builder-cvm.json`, `vhd-image-builder-flatcar-arm64.json`)

**Fix (v2 — current)**: Changed the Flatcar packer templates (`vhd-image-builder-flatcar.json`, `vhd-image-builder-flatcar-arm64.json`) to use bare `waagent` instead of `/usr/sbin/waagent` in the deprovision command. This lets the shell resolve `waagent` via PATH, finding it at `/usr/bin/waagent` regardless of whether `/usr/sbin` is a symlink (Flatcar) or a separate read-only directory (ACL). This matches the pattern already used by the Mariner packer templates.

**Log reference**: `failed.log` lines 51535 (`command not found`), 51536 (`cleanup provisioner`), 51706 (`exit status: 125`)

---

## Issue 4: Missing packages during VHD build (CNI plugins, ACR credential provider)

**Status**: Fixed
**Date**: 2026-02-11
**Symptom**: VHD build succeeds but test phase fails with:
- `testContainerNetworkingPluginsInstalled` — CNI bridge binary not found at `/opt/cni/bin/bridge`
- `testAcrCredentialProviderInstalled` — tarball missing at `/opt/credentialprovider/downloads/`

**Root cause**: `getPackageJSON()` in `cse_helpers.sh` is called with OS=`ACL` during VHD build,
but `components.json` has no `acl` key for packages — only `flatcar.current` entries. The function
couldn't resolve ACL-specific entries, fell through to `.downloadURIs.default.current` (which doesn't
exist for these packages), and returned empty/skip. This caused the install script to skip:
- `containernetworking-plugins` package installation
- `azure-acr-credential-provider-pmc` package installation

**Build log evidence**:
```
"containernetworking-plugins package versions array is either empty or the first element is <SKIP>. 
 Skipping containernetworking-plugins installation."

"azure-acr-credential-provider-pmc package versions array is either empty or the first element is <SKIP>. 
 Skipping azure-acr-credential-provider-pmc installation."
```

**Fix**: Added ACL→flatcar fallback in `getPackageJSON()` function. When OS is ACL, the jq search
path now tries:
1. `.downloadURIs.acl.{variant}/current` (ACL-specific, if it exists)
2. `.downloadURIs.acl.current` (ACL default)  
3. `.downloadURIs.flatcar.current` (fallback to Flatcar)
4. `.downloadURIs.default.current` (global default)

This allows ACL to inherit Flatcar package entries without needing duplicate entries in `components.json`,
since both are Flatcar-based distributions with identical runtime dependencies.

**Affected files**:
- `parts/linux/cloud-init/artifacts/cse_helpers.sh` — `getPackageJSON()` function (lines 885-888)
- `pkg/agent/testdata/**/CustomData` — auto-regenerated snapshot test data

**Full analysis**: See `acl_test_failures_analysis.txt`
