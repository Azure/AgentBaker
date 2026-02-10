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

## Issue 3: (placeholder — next build attempt)

**Status**: Not yet encountered
**Date**: —
**Symptom**: —
**Root cause**: —
**Fix**: —
