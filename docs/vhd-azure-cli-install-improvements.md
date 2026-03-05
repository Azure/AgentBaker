# VHD Build: Azure CLI Installation Improvements

**Extracted from:** [optimization-audit-python-pip.md](./optimization-audit-python-pip.md)
**Date:** 2026-03-05

These are the two actionable VHD build tasks related to `pip install azure-cli` in `vhdbuilder/packer/trivy-scan.sh`.

---

## 1. Ubuntu 22.04 ARM64 — Switch from pip to apt ✅ DONE

**Status:** Completed
**File:** `vhdbuilder/packer/trivy-scan.sh` (lines 71-76)
**Estimated savings:** 5-15 minutes per VHD build

### Problem

The Ubuntu 22.04 ARM64 code path used `pip install azure-cli`, which downloaded ~200+ transitive Python packages and compiled C extensions (cryptography, cffi) from source on ARM64, taking 5-15 minutes. Every other Ubuntu and AzureLinux path already used native OS packages (~30 seconds).

### Fix

Replaced the pip path with the same Microsoft apt repo approach already used for Ubuntu 24.04 ARM64. The `[arch=arm64]` package in the Microsoft apt repo supports both `jammy` (22.04) and `noble` (24.04).

```bash
# Before (5-15 min):
apt_get_install 5 1 60 python3-pip
pip install azure-cli

# After (~30 sec):
apt_get_install 5 1 60 ca-certificates curl apt-transport-https lsb-release gnupg
curl -sL https://packages.microsoft.com/keys/microsoft.asc | sudo apt-key add -
echo "deb [arch=arm64] https://packages.microsoft.com/repos/azure-cli/ $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/azure-cli.list
apt_get_update
apt_get_install 5 1 60 azure-cli
```

### Verification

The Azure DevOps PR gate pipeline (`.pipelines/.vsts-vhd-builder.yaml`) includes the `build2204arm64gen2containerd` job that builds Ubuntu 22.04 ARM64. The trivy scan runs as part of `test-scan-and-cleanup`.

---

## 2. Flatcar / AzureContainerLinux / AzureLinuxOSGuard — Tracked for reference

**Status:** Not started (not owned by this change)
**File:** `vhdbuilder/packer/trivy-scan.sh` (lines 93-101)
**Estimated savings:** 4-14 minutes per VHD build

### Problem

The Flatcar/ACL/AzureLinuxOSGuard path uses `python3 -m venv` + `pip install azure-cli`, which has the same slow compilation problem as the old 22.04 ARM64 path.

### Potential fixes

1. **If `dnf` is available:** Use the RPM-based install path already used for CBLMariner/AzureLinux (lines 89-92).
2. **If `dnf` is not available:** Consider `uv pip install --system azure-cli` (~30-60 sec vs 5-15 min).
3. **Investigate:** Determine whether these OS variants have native package manager support for Azure CLI.

### Notes

- These OS variants may have constraints (e.g., Flatcar's read-only root filesystem) that prevent using native package managers.
- This task requires investigation into what package managers are available on each variant.
