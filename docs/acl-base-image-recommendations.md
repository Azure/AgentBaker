# ACL Base Image vs AgentBaker — Change Recommendations

Analysis of all 9 issues from [acl-build-issues.md](acl-build-issues.md), categorizing which fixes belong in the ACL base image (`acl-scripts`) and which should stay in AgentBaker.

---

## Changes to make in the ACL base image (`acl-scripts`)

### Issue 2: Install `azure-vm-utils` — udev disk rules (High priority)

**Why base image**: The `/dev/disk/azure/{root,os,resource}` symlinks are fundamental Azure infrastructure. Every Azure VM needs them, not just AKS nodes. Currently `package_catalog.sh` line 248 has `["sys-apps/azure-vm-utils"]="SKIP"`.

**Fix**: Either:
- Map `azure-vm-utils` to the Azure Linux RPM equivalent instead of `SKIP`
- Or add it as an explicit install in `build_image_util.sh`

Once fixed, the 40-line inline udev rule workaround in AgentBaker's `pre-install-dependencies.sh` can be removed.

---

### Issue 7: Install oem-azure files — chrony config, drop-ins, tmpfiles (High priority)

**Why base image**: Time synchronization via PTP/Hyper-V is fundamental Azure VM behavior. The correct files *already exist* in the overlay at `sdk_container/src/third_party/coreos-overlay/coreos-base/oem-azure/files/` — they just never reach the final image because `oem-azure` is `SKIP`ped and the ebuild's `src_install()` never runs.

`manglefs.sh` already handles moving chrony configs from `/etc` to `/usr/lib` and patching the `chronyd.service`. But it's moving the **wrong** chrony.conf (the default Azure Linux RPM one with `makestep 1.0 3`) instead of the Azure-optimized one (with `makestep 1.0 -1` and `refclock PHC /dev/ptp_hyperv`).

**Fix in `manglefs.sh`**: Add explicit file copies that source the overlay's oem-azure files into the sysext rootfs:
```bash
# Copy Azure-optimized chrony.conf from oem-azure overlay
local oem_azure_files="<path-to-overlay>/coreos-base/oem-azure/files"
if [[ -f "${oem_azure_files}/chrony.conf" ]]; then
    cp "${oem_azure_files}/chrony.conf" "${rootfs}/usr/share/oem-azure/chrony.conf"
fi
# Copy chrony-hyperv.conf drop-in
if [[ -f "${oem_azure_files}/chrony-hyperv.conf" ]]; then
    mkdir -p "${rootfs}/usr/lib/systemd/system/chronyd.service.d"
    cp "${oem_azure_files}/chrony-hyperv.conf" "${rootfs}/usr/lib/systemd/system/chronyd.service.d/"
fi
# Copy tmpfiles configs (var-chrony.conf, etc-chrony.conf)
```

Once the base image ships the correct chrony config, the entire 50+ line `disableNtpAndTimesyncdInstallChrony()` workaround in AgentBaker's `vhdbuilder/scripts/linux/flatcar/tool_installs_flatcar.sh` can be removed.

---

### Issue 6: Remove or fix `/etc/profile.d/umask.sh` (Medium priority)

**Why base image**: This is a CIS hardening concern at the OS level. Flatcar 4593+ already removed the file. ACL should match.

**Fix**: Either remove `/etc/profile.d/umask.sh` from the base image entirely, or ensure it contains `umask 027` for CIS compliance. The simplest path: add `rm -f "${rootfs}/etc/profile.d/umask.sh"` in `build_image_util.sh` during image finalization.

---

### Issue 11: Add `tmpfiles.d/logrotate.conf` — logrotate state directory (High priority)

**Why base image**: The `/var/lib/logrotate/` directory is required by logrotate's built-in `logrotate.service` to write its state file (`logrotate.status`). This is basic OS functionality — log rotation should work on any ACL VM, not just AKS nodes. The Azure Linux 3 logrotate RPM creates this directory at RPM install time but does not ship a `tmpfiles.d` drop-in. On ACL's immutable rootfs with a separate `/var` partition, directories must be created at boot via `systemd-tmpfiles`. Upstream Flatcar 4459.2.2 ships `usr/lib/tmpfiles.d/logrotate.conf` for exactly this purpose; ACL does not.

**Fix**: Create `/usr/lib/tmpfiles.d/logrotate.conf` in the ACL image with:
```
d /var/lib/logrotate 0755 root root -
```

This can be added in `manglefs.sh` or `build_image_util.sh`:
```bash
# Create tmpfiles.d entry for logrotate state directory
# Azure Linux 3 RPM creates /var/lib/logrotate at install time but doesn't ship
# a tmpfiles.d drop-in. On Flatcar's immutable rootfs, /var is populated at boot
# via systemd-tmpfiles, so the directory must be declared here.
echo 'd /var/lib/logrotate 0755 root root -' > "${rootfs}/usr/lib/tmpfiles.d/logrotate.conf"
```

Once fixed, the `mkdir -p /var/lib/logrotate` workaround in AgentBaker's `pre-install-dependencies.sh` can be removed.

---

### Issue 12: Fix bash `update-ssh-keys` cross-device `mv` (Medium priority)

**Why base image**: The `update-ssh-keys` binary is a core OS tool called by `update-ssh-keys-after-ignition.service` (from `coreos-init`). ACL's bash replacement has a bug in `regenerate()` — it creates a temp file in `/tmp` (tmpfs) and tries to `mv` it to `/home/core/.ssh/authorized_keys` (ext4). The cross-device `mv` fails with `EEXIST` when waagent has already created the target. Flatcar's Rust binary creates its temp file alongside the target (same filesystem) so `rename()` atomically overwrites. This is an OS-level tool bug, not an AKS concern.

**Fix**: In `build_library/rpm/additional_files/update-ssh-keys`, change both `mktemp` calls in `regenerate()` and the `add|force-add` case to create temp files on the same filesystem as the target:
```bash
# In regenerate():
temp_file=$(mktemp "${KEYS_FILE}.XXXXXX")

# In add|force-add case:
temp_key=$(mktemp "${key_file}.XXXXXX")
```

Once fixed, the `update-ssh-keys-after-ignition.service` allowlist entry in AgentBaker's `e2e/validators.go` can be removed.

---

## Changes to keep in AgentBaker

### Issue 1: `isFlatcar()` recognizing ACL

**Why AgentBaker**: ACL reports `ID=acl` — that's correct for ACL's identity. AgentBaker is responsible for mapping OS identities to behavior profiles. The `isFlatcar()` + `isACL()` functions in `parts/linux/cloud-init/artifacts/cse_helpers.sh` are the right place for this logic. No base image change needed.

---

### Issue 3: PATH-based `waagent` lookup

**Why AgentBaker**: Using `waagent` via PATH instead of hardcoding `/usr/sbin/waagent` is genuinely better practice. ACL correctly doesn't symlink `/usr/sbin -> /usr/bin` (that's a Flatcar-ism). The packer template fixes in `vhd-image-builder-flatcar.json` are the right approach. No reason to add a symlink to the base image.

---

### Issue 4: ACL -> Flatcar fallback in `getPackageJSON()`

**Why AgentBaker**: This is about `components.json` lookup logic. ACL inherits Flatcar's component entries, and the fallback in `cse_helpers.sh` is the right architectural approach to avoid duplicating Flatcar entries for ACL. Purely an AgentBaker concern.

---

### Issue 8: Disable `iptables.service` for ACL

**Why AgentBaker**: The base image's firewall rules are reasonable for a general Azure VM (SSH access, conntrack, etc.). AgentBaker (the AKS-specific tool) should adjust the firewall for AKS's needs — it already does this for Mariner/AzureLinux via `disableSystemdIptables`. The ACL guard in `vhdbuilder/packer/install-dependencies.sh` is the right place. If ACL were ever used outside AKS, the base image rules would be desired.

---

### Issue 9: Explicit Ignition enable symlinks (workaround)

**Why AgentBaker**: The immediate workaround uses Ignition's `storage.links` to create enable symlinks directly in `sysinit.target.wants/`, bypassing the broken preset mechanism entirely. This is safe on both ACL and upstream Flatcar (`overwrite: true` handles the case where presets already work). The fix is in `parts/linux/cloud-init/flatcar.yml`.

---

### Issue 13: Route ACL to `update-ca-trust` for CA certificate handling

**Why AgentBaker**: ACL uses Azure Linux's `update-ca-trust` instead of Flatcar's `update-ca-certificates`, and ACL's `/usr` is dm-verity read-only so certs must be placed under `/etc/pki/ca-trust/source/anchors` (not `/usr/share/pki/...`). This is a provisioning-time tool routing decision — AgentBaker is responsible for knowing which cert update tool each distro uses. The `isACL` check in `configureHTTPProxyCA()` and the dedicated `acl/update_certs.service` file (using `/etc/pki/ca-trust/source/anchors` + `update-ca-trust`) are the correct approach. No base image change needed.

---

### Issue 14: `/etc/protocols` header differs on ACL

**Why AgentBaker**: ACL's `iana-etc` RPM uses a different header comment than Flatcar's. The file contents are functionally identical — same protocol entries, just different comment formatting. The E2E test was checking for a Flatcar-specific header string. Relaxing the check to `"tcp"` validates the file has real protocol data without coupling to one distro's comment style. No base image change needed.

---

### Issue 15: `/etc/ssl/certs/ca-certificates.crt` is a symlink on ACL

**Why AgentBaker**: On ACL, `update-ca-trust` creates this path as a symlink to `/etc/pki/tls/certs/ca-bundle.crt` rather than a regular file. This is standard Azure Linux/RHEL CA trust layout. The E2E test validator was checking `ValidateFileIsRegularFile`, which rejects symlinks. Changing to `ValidateFileExists` (which follows symlinks via `test -f`) correctly validates the trust bundle is present regardless of file type. No base image change needed.

---

## Changes to make in both (AgentBaker workaround now, base image fix long-term)

### Issue 9: Ignition preset mechanism broken on ACL (High priority)

**Why base image (long-term)**: The root cause is that ACL's systemd v255 lacks `systemd-preset-all.service` (introduced in v256), and `/etc/machine-id` is pre-populated so `ConditionFirstBoot=yes` fails. This breaks *any* Ignition-defined service that uses `enabled: true` — not just AgentBaker's two services. Any future use of Ignition presets on ACL will hit the same problem.

**AgentBaker workaround (already implemented)**: Added `storage.links` to `flatcar.yml` to create explicit enable symlinks, bypassing the preset mechanism.

**Fix in ACL base image** (choose one or both):
1. **Add `systemd-preset-all.service` equivalent**: Create a boot service that runs `systemctl preset-all` unconditionally during early boot. This is what systemd v256+ provides natively. Could be added to bootengine or as a new systemd unit in the sysext.
2. **Fix first-boot detection**: Investigate why `/etc/machine-id` is populated after switch-root despite being emptied during VHD build (`cleanup-vhd.sh` and `build_image_util.sh:1241`). The E2E diagnostics show a freshly generated machine-id (`a0902b9f403843fbbdc3b1a79d2c3800`), suggesting something in the boot chain (possibly `systemd-machine-id-setup` in initrd, or `waagent`) regenerates it before PID 1 evaluates `ConditionFirstBoot=yes`. Fixing this would make systemd's built-in `-Dfirst-boot-full-preset=true` logic work correctly.

Once the base image properly processes Ignition presets, the `storage.links` workaround in AgentBaker becomes redundant (but harmless due to `overwrite: true`).

---
