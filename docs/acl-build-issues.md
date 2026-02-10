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

## Issue 2: (placeholder — next build attempt)

**Status**: Not yet encountered
**Date**: —
**Symptom**: —
**Root cause**: —
**Fix**: —
