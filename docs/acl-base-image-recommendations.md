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
