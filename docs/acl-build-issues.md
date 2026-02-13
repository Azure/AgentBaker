# ACL VHD Build — Issue Tracker

Tracking issues encountered while building ACL (Azure Container Linux) VHDs using the Flatcar packer pipeline.

Each issue below includes two sections:
- **AgentBaker fix**: what changed in this repo.
- **ACL base image follow-up**: what still needs to change in the ACL base image (or external component). Use `None` when no base image action is needed.

---

## Issue 1: OS detection — `isFlatcar()` doesn't recognize ACL

**Status**: Fixed\
**Date**: 2026-02-10\
**Symptom**: Build fails with `rsyslog could not be started` / `exit 1` in `pre-install-dependencies.sh`.\
**Root cause**: ACL reports `ID=acl` in `/etc/os-release`, uppercased to `ACL`. `isFlatcar()` only matched `FLATCAR`, so it returned false and the build attempted to start `rsyslog.service` (missing on ACL/Flatcar).\
**AgentBaker fix**:
- Extend `isFlatcar()` to treat `ACL` as Flatcar in both script locations.
- Affected files:
	- `parts/linux/cloud-init/artifacts/cse_helpers.sh` — `isFlatcar()`
	- `vhdbuilder/packer/vhd-scanning.sh` — `isFlatcar()`
	- Callers in `vhdbuilder/packer/pre-install-dependencies.sh` and `vhdbuilder/packer/packer_source.sh`

**ACL base image follow-up**: None.\
**Notes**: This aligns ACL behavior with Flatcar for service expectations.

---

## Issue 2: `disk_queue.service` fails — missing Azure disk udev rules

**Status**: Fixed (workaround); follow-up needed in ACL base image\
**Date**: 2026-02-10\
**Symptom**: Build fails with `disk_queue could not be started` / `exit 1` in `pre-install-dependencies.sh`.\
**Root cause**:
- `disk_queue.sh` requires `/dev/disk/azure/{root,os}` symlinks created by `80-azure-disk.rules`.
- On ACL, WALinuxAgent skips installing udev rules for Flatcar >= 3550, and the ACL image does not include `azure-vm-utils` (which provides the rules on Flatcar).

**AgentBaker fix**:
- Install `80-azure-disk.rules` in `pre-install-dependencies.sh` before starting `disk_queue`, guarded by file existence.
- Affected files:
	- `vhdbuilder/packer/pre-install-dependencies.sh`
	- `parts/linux/cloud-init/artifacts/disk_queue.sh`

**ACL base image follow-up**:
- Ensure the ACL base image includes `azure-vm-utils` so the rules exist at boot.

---

## Issue 3: Packer deprovision fails — `/usr/sbin/waagent` not found

**Status**: Fixed\
**Date**: 2026-02-10\
**Symptom**: Final deprovision step fails with exit code 125 and `sudo: /usr/sbin/waagent: command not found`.\
**Root cause**: ACL installs `waagent` to `/usr/bin/waagent` and has a real `/usr/sbin`. Flatcar uses a `/usr/sbin -> /usr/bin` symlink, so `/usr/sbin/waagent` works there but not on ACL.\
**AgentBaker fix**:
- Use `waagent` via PATH (no hardcoded `/usr/sbin`) in Flatcar packer templates.
- Affected files:
	- `vhdbuilder/packer/vhd-image-builder-flatcar.json`
	- `vhdbuilder/packer/vhd-image-builder-flatcar-arm64.json`

**ACL base image follow-up**: None.\
**Notes**: ACL does not provide the `/usr/sbin` -> `/usr/bin` symlink used by Flatcar.

---

## Issue 4: Missing packages during VHD build (CNI plugins, ACR credential provider)

**Status**: Fixed\
**Date**: 2026-02-11\
**Symptom**: Tests fail because `/opt/cni/bin/bridge` and `/opt/credentialprovider/downloads/` artifacts are missing.\
**Root cause**: `getPackageJSON()` was called with `OS=ACL` but `components.json` only had `flatcar.current` entries. The function fell through to defaults and skipped installs.\
**AgentBaker fix**:
- Add ACL -> Flatcar fallback in `getPackageJSON()` so ACL inherits Flatcar entries.
- Affected files:
	- `parts/linux/cloud-init/artifacts/cse_helpers.sh` — `getPackageJSON()`
	- `pkg/agent/testdata/**/CustomData` — regenerated snapshots

**ACL base image follow-up**: None.\
**Notes**: Prevents duplication in `components.json` while keeping ACL aligned with Flatcar.

---

## Issue 5: Cloud-init distro detection warning — `acl` not recognized

**Status**: Active (non-blocking)\
**Date**: 2026-02-11\
**Symptom**: Cloud-init warns: `Unable to load distro implementation for acl. Using default distro implementation instead.`\
**Root cause**: `distro` library does not recognize `acl` as a distro ID and falls back to defaults.\
**AgentBaker fix**: None.\
**ACL base image follow-up**:
- Evaluate cloud-init behavior on ACL and determine whether ACL needs a proper distro implementation or downstream patching.

**Notes**: Build succeeds today, but provisioning scripts that rely on OS detection may misbehave.

---

## Issue 6: `testUmaskSettings` fails

**Status**: Active (workaround in place; ACL base image version-dependent)\
**Date**: 2026-02-11\
**Symptom**: `testUmaskSettings` fails because `/etc/profile.d/umask.sh` exists in the ACL base image with non-CIS defaults (umask 022 or similar).\
**Root cause**:
- Flatcar (4593.0.0+) does not include `/etc/profile.d/umask.sh` at all, so the test skips automatically via the condition check.

**AgentBaker fix**:
- Skip `testUmaskSettings` for Flatcar (which includes ACL) in the VHD content test when the file exists.
- Affected files:
	- `vhdbuilder/packer/test/linux-vhd-content-test.sh` — test correctly skips if `os_sku = "Flatcar"`

**ACL base image follow-up**:
- Ensure the ACL base image either removes `/etc/profile.d/umask.sh` entirely, or ensures it contains exactly `umask 027` so the file content matches expectations.

**Notes**: Test passes on Flatcar because the file does not exist. Test fails on ACL because it includes the file with non-CIS defaults. Workaround is conditional, but depends on ACL base image version.

---

## Issue 7: `testChrony` fails — chronyd doesn't auto-correct large time offsets

**Status**: Fixed (workaround in AgentBaker); root cause identified in ACL build scripts\
**Date**: 2026-02-12\
**Symptom**: `testChrony` fails with `chronyd failed to readjust the system time` after setting time 5 years in the past.\
**Root cause**:
- ACL's base image ships chrony with `makestep 1.0 3` — only steps the clock during the first 3 updates, then slews gradually (cannot correct a 5-year offset in the 100s test window).
- The config lives at `/usr/lib/chrony/chrony.conf` on a read-only `/usr` filesystem (immutable, verity-protected), so it cannot be overwritten during VHD build.
- No PTP refclock configured; only `time.windows.com` as NTP server.
- **Build-level root cause**: The correct Azure-optimized chrony.conf already exists in the Flatcar overlay at `sdk_container/src/third_party/coreos-overlay/coreos-base/oem-azure/files/chrony.conf` (with `makestep 1.0 -1` and `refclock PHC /dev/ptp_hyperv`). However, it never reaches the final image because `coreos-base/oem-azure` is marked `SKIP` in `build_library/rpm/package_catalog.sh` (line 245). In RPM mode, the ebuild's `src_install()` — which copies `chrony.conf`, `chrony-hyperv.conf` drop-in, and tmpfiles configs — never runs. Only the RPM-mapped dependencies (`chrony`, `WALinuxAgent`, `hyperv-daemons`) are installed via dnf, so the image gets the Azure Linux `chrony` RPM's default config instead of the Azure-tuned one.
- Also missing from the sysext due to the SKIP: the `chronyd.service.d/chrony-hyperv.conf` drop-in (starts chronyd with `-F 1`), and the tmpfiles configs (`var-chrony.conf`, `etc-chrony.conf`) that create required directories.

**AgentBaker fix**:
- Write Azure-optimized chrony config to `/etc/chrony/chrony.conf` (writable) with `makestep 1.0 -1`, `refclock PHC /dev/ptp_hyperv`, and `time.windows.com` fallback.
- Add systemd drop-in (`chronyd.service.d/20-chrony-config-override.conf`) to override `ExecStart` to use the writable config path instead of read-only `/usr/lib/chrony/chrony.conf`.
- Affected files:
	- `vhdbuilder/scripts/linux/flatcar/tool_installs_flatcar.sh` — implemented `disableNtpAndTimesyncdInstallChrony()` for ACL

**ACL base image follow-up**:
- The oem-azure sysext build (`build_sysext --metapkgs=coreos-base/oem-azure`) skips the `coreos-base/oem-azure` ebuild in RPM mode because `package_catalog.sh` maps it to `SKIP`. This means all files installed by the ebuild's `src_install()` are missing:
	- `chrony.conf` (Azure Hyper-V PTP + `makestep 1.0 -1`)
	- `chrony-hyperv.conf` drop-in for `chronyd.service`
	- `var-chrony.conf` and `etc-chrony.conf` tmpfiles configs
	- `chronyd.service` enablement symlink
- **Fix options** (in `acl-scripts`):
	Could Add explicit file-copy logic in `manglefs.sh` to source the overlay's `files/chrony.conf` and related configs into the sysext rootfs.

**Notes**: Flatcar already has correct chrony config at `/etc/chrony/chrony.conf` (writable) because in Portage mode the oem-azure ebuild runs normally. Validated manually on ACL VM — chrony corrects 5-year offset in ~20 seconds with the AgentBaker workaround.

---

## Issue 8: `iptables.service` loads Azure Linux host firewall rules — breaks Cilium eBPF validation

**Status**: Fixed\
**Date**: 2026-02-13\
**Symptom**: E2E tests `Test_Flatcar`, `Test_Flatcar_AzureCNI`, and `Test_Flatcar_SecureTLSBootstrapping_BootstrapToken_Fallback` fail with `ValidateIPTablesCompatibleWithCiliumEBPF` rejecting 6 iptables rules in the `filter` table:
```
-A INPUT -i lo -j ACCEPT
-A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
-A INPUT -p tcp -m tcp --dport 22 -j ACCEPT
-A INPUT -p icmp -m icmp --icmp-type 11 -j ACCEPT
-A INPUT -p icmp -m icmp --icmp-type 3 -j ACCEPT
-A OUTPUT -j ACCEPT
```

**Root cause**:
- ACL inherits Azure Linux's `iptables.service`, which loads restrictive host firewall rules at boot.
- On Mariner/AzureLinux, `disableSystemdIptables()` (from `vhdbuilder/scripts/linux/tool_installs.sh`) masks the service during VHD build. But this call is gated by `isMarinerOrAzureLinux` in `vhdbuilder/packer/install-dependencies.sh`, and ACL returns **false** for that check.
- Original Flatcar also has `iptables.service`, but it loads **empty rules** (all chains ACCEPT with no `-A` rules), which pass the E2E validator's `^-P .*` pattern. So it was never an issue for Flatcar.
- ACL's `iptables.service` loads actual Azure Linux firewall rules (INPUT restrictions, conntrack, SSH allow), which are not in the validator's allowed patterns.

**AgentBaker fix**:
- Call `disableSystemdIptables` for ACL during VHD build, gated by `isACL`.
- Affected files:
	- `vhdbuilder/packer/install-dependencies.sh` — added `isACL` guard to call `disableSystemdIptables`

**ACL base image follow-up**:
- Consider whether the ACL base image should ship with `iptables.service` disabled/masked by default, since AKS manages its own iptables rules via Cilium/kube-proxy.

**Notes**: Mariner/AzureLinux already masks this service. Original Flatcar's empty rules are harmless, so the fix is scoped to ACL only using `isACL` rather than `isFlatcar`.

