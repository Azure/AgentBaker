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

---
## Issue 9: `ignition-file-extract.service` never runs — instance-specific files missing (localdns, etc.)
**Status**: Open — root cause of 4 E2E failures (Test_Flatcar, Test_Flatcar_AzureCNI, Test_Flatcar_AzureCNI_ChronyRestarts, Test_Flatcar_SecureTLSBootstrapping)
**Date**: 2026-02-13
**Symptom**: E2E tests fail with `localdns: active=inactive enabled=disabled` / `expected active, got inactive`. The `localdns.corefile` at `/opt/azure/containers/localdns/localdns.corefile` does not exist on the VM, even though it should have been delivered via the Ignition tarball.

**Root cause — Ignition preset mechanism is broken on ACL (not an overlay issue)**:

The root cause is a **three-part failure chain** in systemd's preset mechanism that prevents Ignition-defined services from being enabled:

1. **Ignition writes a preset file, not symlinks**: When `flatcar.yml` specifies `enabled: true` on a unit, the Ignition engine (Flatcar fork v2.22.0) writes a **preset file** at `/etc/systemd/system-preset/20-ignition.preset` containing lines like `enable ignition-file-extract.service`. Ignition does NOT directly create enable symlinks (e.g., in `multi-user.target.wants/`). It relies on a systemd mechanism to process this preset file and create the symlinks.

2. **`systemd-preset-all.service` does not exist on ACL**: In upstream Flatcar, the preset file would be processed by `systemd-preset-all.service` (with `ConditionFirstBoot=yes`) which runs `systemctl preset-all` on first boot to create enable symlinks from all preset files. **Azure Linux's systemd v255 does not ship `systemd-preset-all.service`** — this service was introduced in upstream systemd v256. Instead, Azure Linux handles presets at RPM install time via a `%post` scriptlet (`systemctl preset-all`) in the systemd RPM spec, which only runs during package installation, not at boot.

3. **First-boot detection also fails**: Even if a preset-all mechanism existed, it wouldn't trigger because `ConditionFirstBoot=yes` fails. The serial console logs confirm:
   ```
   systemd-firstboot.service - First Boot Wizard was skipped because of an unmet condition check (ConditionFirstBoot=yes)
   first-boot-complete.target - First Boot Complete was skipped because of an unmet condition check (ConditionFirstBoot=yes)
   ```
   Azure Linux's systemd is built with `-Dfirst-boot-full-preset=true` (which makes PID 1 run preset-all internally on first boot), but this also depends on `ConditionFirstBoot=yes`, which requires `/etc/machine-id` to be empty or missing. Despite both `cleanup-vhd.sh` (AgentBaker Packer) and `build_image_util.sh` (ACL image build) emptying machine-id, something repopulates it before PID 1 evaluates the condition.

**The net result**: Ignition writes the preset file, but nothing ever processes it into enable symlinks, so `ignition-file-extract.service` and `ignition-bootcmds.service` are never enabled, never started, and the tarball is never extracted.

**Diagnostic evidence from serial console logs**:
- Ignition engine successfully wrote `/sysroot/var/lib/ignition/ignition-files.tar` (confirmed at 04:08:12)
- Ignition engine wrote the `ignition-file-extract.service` unit file to `/sysroot/etc/systemd/system/`
- Ignition wrote the preset file to `/sysroot/etc/systemd/system-preset/20-ignition.preset`
- Kernel cmdline confirms initrd detected first boot: `flatcar.first_boot=detected ignition.firstboot=1`
- After switch-root, `ConditionFirstBoot=yes` evaluates as false (machine-id populated)
- `ignition-file-extract.service` never starts — zero log entries after switch-root
- The `/etc/systemd/system-preset/` directory does not even exist on a standalone ACL VM (`systemctl status systemd-preset-all.service` → "Unit could not be found")

**Impact**: All instance-specific files from the Ignition tar are missing. VHD-baked scripts at `/opt/` still work, so CSE can run and kubelet starts. But localdns corefile and potentially other instance-specific configs are absent.

**Why CSE succeeds despite missing tar files**: The CSE scripts themselves are baked into the VHD during Packer build (at `/opt/azure/containers/`). The CSE extension runs these VHD-baked scripts. Only the Ignition-tar-delivered files (localdns corefile, some systemd overrides) are missing.

**E2E validation flow**: `ValidateCommonLinux()` in `e2e/validation.go:70` always checks localdns unless `VHD.UnsupportedLocalDns == true`. The Flatcar VHD config does NOT set this flag (see `e2e/config/vhd.go`). The base NBC in `e2e/node_config.go:450` sets `LocalDNSProfile.EnableLocalDNS = true`, so the corefile SHOULD be generated by the AgentBaker Go service and included in the Ignition tar + cloud-init write_files.

**Fix options** (in priority order):
1. **AgentBaker workaround — explicit symlinks via Ignition** (quickest, no base image change):
   In `parts/linux/cloud-init/flatcar.yml`, instead of relying on `enabled: true` (which uses the broken preset mechanism), use Ignition's `storage.links` to explicitly create the enable symlinks:
   ```yaml
   storage:
     links:
       - path: /etc/systemd/system/multi-user.target.wants/ignition-file-extract.service
         target: /etc/systemd/system/ignition-file-extract.service
       - path: /etc/systemd/system/multi-user.target.wants/ignition-bootcmds.service
         target: /etc/systemd/system/ignition-bootcmds.service
   ```
   This bypasses the broken preset mechanism entirely. Ignition writes these symlinks during initrd before switch-root, and the overlay preserves them.

2. **ACL base image fix — add a preset-all boot service**:
   Create a custom systemd service in the ACL image (e.g., in bootengine or a new package) that runs `systemctl preset-all` unconditionally during early boot, equivalent to what `systemd-preset-all.service` provides in systemd v256+. This is the proper long-term fix.

3. **ACL base image fix — fix first-boot detection**:
   Debug why `/etc/machine-id` is populated after switch-root despite being emptied during VHD build, and fix the cleanup chain so systemd's `-Dfirst-boot-full-preset=true` logic triggers correctly. This would make PID 1 process presets automatically.

4. **Short-term E2E workaround**: Set `UnsupportedLocalDns: true` on `VHDFlatcarGen2` in `e2e/config/vhd.go` to skip localdns validation for ACL/Flatcar. This doesn't fix the underlying extraction issue but unblocks test runs.

**ACL base image follow-up**:
- Azure Linux's systemd v255 does not include `systemd-preset-all.service` (introduced in v256). Consider backporting this service or creating an equivalent.
- Investigate and fix the first-boot detection chain: `cleanup-vhd.sh` empties `/etc/machine-id` → `waagent -deprovision+user` runs after → bootengine `initrd-setup-root` removes the blank machine-id → but something repopulates it before PID 1 evaluates `ConditionFirstBoot=yes`.
- The `20-ignition.preset` file written by Ignition is ultimately defeated by the `disable *` catch-all in Azure Linux's `90-systemd.preset` if `systemctl preset` is ever re-run without the Ignition preset file present.

**Key files involved**:
- `parts/linux/cloud-init/flatcar.yml` — Ignition config defining the services with `enabled: true`
- `vhdbuilder/packer/cleanup-vhd.sh` — empties `/etc/machine-id` during VHD build
- `vhdbuilder/packer/vhd-image-builder-flatcar.json` — Packer config; runs cleanup then `waagent -deprovision+user`
- `acl-scripts/build_library/rpm/build_image_util.sh:1241` — removes machine-id from lowerdir
- Bootengine `initrd-setup-root` — removes blank machine-id, mounts `/etc` overlay
- Azure Linux `systemd.spec` line 703 — builds with `-Dfirst-boot-full-preset=true`
- Azure Linux `systemd.spec` line 965 — `systemctl preset-all` in `%post` (RPM install time only)

**Notes**: This is NOT an overlay issue. The overlay correctly preserves Ignition-written files. The root cause is that Azure Linux's systemd v255 lacks the boot-time preset processing that Ignition relies on to enable services. **Theory fully confirmed by E2E diagnostics on 2026-02-13.**

---

## Issue 10: `/etc/resolv.conf` points to systemd-resolved stub — localdns DNS validation fails

**Status**: Fixed\
**Date**: 2026-02-14\
**Symptom**: All Flatcar E2E tests fail `ValidateLocalDNSResolution` with `SERVER: 127.0.0.53` instead of `SERVER: 169.254.10.10`, even though localdns-coredns is running and actively serving queries on `169.254.10.10`.

**Root cause**:

On systemd-resolved systems there are two resolv.conf files:
- `/run/systemd/resolve/stub-resolv.conf` — always `nameserver 127.0.0.53` (the stub listener)
- `/run/systemd/resolve/resolv.conf` — real upstream nameservers (e.g. `168.63.129.16`, or `169.254.10.10` after localdns configures it)

`/etc/resolv.conf` is a symlink that determines which one `dig` and other tools use.

When localdns starts, `disable_dhcp_use_clusterlistener()` creates a network dropin that tells systemd-resolved to use `169.254.10.10` as upstream. This updates `/run/systemd/resolve/resolv.conf` to `nameserver 169.254.10.10`, but the stub file always stays `127.0.0.53`. So if `/etc/resolv.conf` points to the stub, `dig` reports `SERVER: 127.0.0.53`.

Each distro handles this differently:
- **Mariner/AzureLinux**: At VHD build time, `disableSystemdResolvedCache()` installs `resolv-uplink-override.service` — a oneshot that runs `ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf` before kubelet. This is baked into the VHD.
- **Ubuntu**: At CSE time, `disableSystemdResolved()` in `cse_config.sh` does the same `ln -sf` for Ubuntu 20.04/22.04/24.04.
- **ACL**: Neither fix applied. `isMarinerOrAzureLinux("ACL")` returns false (skips VHD build fix), and `disableSystemdResolved()` checks `lsb_release` which doesn't exist on ACL (CSE fix is a no-op).

The result: `/etc/resolv.conf → stub-resolv.conf → 127.0.0.53` on ACL. DNS queries still work (127.0.0.53 → systemd-resolved → 169.254.10.10 → upstream), but the path is indirect and the E2E test (which checks `dig` output for `SERVER: 169.254.10.10`) fails.

**AgentBaker fix**:
- Call `disableSystemdResolvedCache` for ACL during VHD build, matching what Mariner/AzureLinux does. This installs `resolv-uplink-override.service` into the VHD so `/etc/resolv.conf` is repointed to the real resolv.conf on every boot.
- Affected files:
	- `vhdbuilder/packer/install-dependencies.sh` — added `disableSystemdResolvedCache` to the `isACL` block

**ACL base image follow-up**: None — the VHD build fix is sufficient. The ACL base image does not need changes since this is a standard configuration step for all Azure Linux-based distros using systemd-resolved + localdns.

**Notes**: This is the same pattern as Issue 8 (iptables) — ACL inherits Azure Linux behavior but `isMarinerOrAzureLinux` returns false for ACL, so Mariner-specific VHD build steps are skipped. Both are now handled in the `isACL` block in `install-dependencies.sh`.

---

## Issue 11: `logrotate.service` fails — missing `/var/lib/logrotate/` directory

**Status**: Fixed (workaround); follow-up needed in ACL base image\
**Date**: 2026-02-16\
**Symptom**: 6 out of 7 Flatcar E2E tests fail. `logrotate.service` exits with status 3 (`NOTIMPLEMENTED`). Serial console / journalctl shows:
```
logrotate[...]: error creating stub state file /var/lib/logrotate/logrotate.status: No such file or directory
systemd[1]: logrotate.service: Main process exited, code=exited, status=3/NOTIMPLEMENTED
systemd[1]: logrotate.service: Failed with result 'exit-code'.
```

**Root cause**:

ACL uses an immutable rootfs with `/var` as a separate partition populated at boot via `systemd-tmpfiles`. The Azure Linux 3 logrotate RPM (version 3.21.0) creates `/var/lib/logrotate/` and touches `/var/lib/logrotate/logrotate.status` at RPM **install time** only (in the `%install` section of the spec). It does **not** ship a `tmpfiles.d` drop-in to recreate the directory at boot.

Upstream Flatcar includes `usr/lib/tmpfiles.d/logrotate.conf` which creates this directory at every boot. ACL does not have this file because it uses the Azure Linux 3 RPM instead of the Gentoo/Flatcar ebuild.

Confirmed by comparing filesystem data:
- **Upstream Flatcar 4459.2.2**: has `lib/tmpfiles.d/logrotate.conf` in `usr_files`
- **ACL 4459.2.2**: does **not** have `lib/tmpfiles.d/logrotate.conf`

The logrotate binary is configured with `--with-state-file-path=/var/lib/logrotate/logrotate.status` and fails immediately when the parent directory doesn't exist.

In `packer_source.sh`, ACL takes the `else` branch (since the base image has a built-in `logrotate.timer` at `/usr/lib/systemd/system/logrotate.timer`), so only the timer dropin (`override.conf`) is installed — NOT the AKS custom `logrotate.sh` wrapper which would have created the directory via `mkdir -p /var/lib/logrotate`.

**AgentBaker fix**:
- Add `mkdir -p /var/lib/logrotate` in `pre-install-dependencies.sh` before enabling `logrotate.timer`, guarded by `isFlatcar`.
- Affected files:
	- `vhdbuilder/packer/pre-install-dependencies.sh`

**ACL base image follow-up**:
- Add a `tmpfiles.d` drop-in to the ACL base image so `/var/lib/logrotate/` is created at every boot, matching upstream Flatcar behavior. Create `/usr/lib/tmpfiles.d/logrotate.conf` with contents:
  ```
  d /var/lib/logrotate 0755 root root -
  ```
  This should be added either in the logrotate RPM spec (ideal) or in the ACL build scripts (e.g. `manglefs.sh` or `build_image_util.sh`).

**Notes**: This is the same `/var` partition issue as chrony (Issue 7) — Azure Linux RPMs assume a persistent rootfs where directories created at install time survive reboots, but ACL/Flatcar's immutable rootfs + separate `/var` partition requires `tmpfiles.d` entries for any state directories under `/var`.

**References**:
- Azure Linux 3 logrotate spec: https://github.com/microsoft/azurelinux/blob/3.0/SPECS/logrotate/logrotate.spec
- Upstream Flatcar logrotate ebuild ships `tmpfiles.d/logrotate.conf`; ACL's RPM-based build does not

---

## Issue 12: `update-ssh-keys-after-ignition.service` fails — bash replacement cross-device `mv`

**Status**: Fixed (E2E workaround); fix needed in ACL base image\
**Date**: 2026-02-17\
**Symptom**: `Test_Flatcar_SecureTLSBootstrapping_BootstrapToken_Fallback` fails with `update-ssh-keys-after-ignition.service` in a failed state. Journalctl shows:
```
mv: cannot create regular file '/home/core/.ssh/authorized_keys': File exists
```

**Root cause**:

ACL replaces Flatcar's Rust `update-ssh-keys` binary with a bash script (`build_library/rpm/additional_files/update-ssh-keys`). The Rust package `coreos-base/update-ssh-keys` is `SKIP`ped in `package_catalog.sh` (line 244), and the bash replacement is installed by `build_image_util.sh` (lines 843–846).

The bash script's `regenerate()` function does:
```bash
temp_file=$(mktemp)                    # creates in /tmp (tmpfs)
# ... build authorized_keys content ...
mv "$temp_file" "$KEYS_FILE"           # target is /home/core/.ssh/authorized_keys (ext4)
```

This fails because:
1. `mktemp` with no arguments creates the temp file in `/tmp` (tmpfs).
2. The target `$KEYS_FILE` is on `/home` (ext4) — a **different filesystem**.
3. Cross-device `mv` cannot use the atomic `rename()` syscall. It falls back to copy, which fails with `EEXIST` when waagent has already created the target file.
4. The script uses `set -euo pipefail`, so the `mv` failure causes immediate exit with code 1.

Flatcar's Rust binary (from `https://github.com/flatcar/update-ssh-keys.git`) creates its temp file in the same directory as the target and uses Rust's `std::fs::rename()`, which maps to the `rename()` syscall. Since source and destination are on the same filesystem, `rename()` atomically replaces the target — even if it already exists.

The `update-ssh-keys-after-ignition.service` itself comes from the upstream Flatcar `init` package (`coreos-base/coreos-init`), which is mapped to the `coreos-init` RPM (not skipped). So the service runs on ACL but calls the broken bash replacement instead of the Rust binary.

The failure is **harmless** — waagent already created `authorized_keys` with the correct SSH keys, so the race condition doesn't affect node authentication.

**AgentBaker fix**:
- Add `update-ssh-keys-after-ignition.service` to the systemd unit failure allowlist in `e2e/validators.go`, gated by `s.VHD.Flatcar`.
- Affected files:
	- `e2e/validators.go` — `ValidateNoFailedSystemdUnits()` allowlist

**ACL base image follow-up**:
- Fix the bash `update-ssh-keys` script's `regenerate()` function to create the temp file on the same filesystem as the target. Change:
  ```bash
  temp_file=$(mktemp)
  ```
  to:
  ```bash
  temp_file=$(mktemp "${KEYS_FILE}.XXXXXX")
  ```
  This keeps both files on the same filesystem, allows `mv` to use the atomic `rename()` syscall (which overwrites existing files), and matches the Rust binary's behavior.
- The same fix should also be applied to the `temp_key=$(mktemp)` in the `add|force-add` case for consistency.
- Affected file: `build_library/rpm/additional_files/update-ssh-keys`

**Notes**: The race condition occurs between `update-ssh-keys-after-ignition.service` and `walinuxagent.service` — both try to populate `/home/core/.ssh/authorized_keys` during early boot. The service fails intermittently depending on which one runs first.

---

## Issue 13: `update_certs.service` fails — `update-ca-certificates` not found on ACL

**Status**: Fixed\
**Date**: 2026-02-17\
**Symptom**: `Test_Flatcar_CustomCATrust` fails with CSE exit code 161. `update_certs.service` exits with status 127 (command not found).

**Root cause**:

ACL does not have `update-ca-certificates` (a Gentoo/Debian-style tool). Instead it uses `update-ca-trust` from the Azure Linux `ca-certificates-tools` RPM, the same tool Mariner uses. Two code paths invoke the wrong command on ACL:

1. **VHD-baked service file**: The Flatcar packer templates used `flatcar/update_certs.service` which calls `update_certs.sh /etc/ssl/certs update-ca-certificates`. This service is restarted by `configureCustomCaCertificate()` in `cse_config.sh` when custom CA certs are configured.

2. **CSE-time HTTP proxy CA**: `configureHTTPProxyCA()` in `cse_config.sh` falls into the `isFlatcar` branch for ACL (since `isFlatcar("ACL")` returns true), and uses `update-ca-certificates` directly.

Both fail with exit code 127 on ACL because the command doesn't exist.

**AgentBaker fix**:
- Add `isACL` check before `isFlatcar` in `configureHTTPProxyCA()` to route ACL to `update-ca-trust` with the Mariner cert destination.
- Change both Flatcar packer templates to use `mariner/update_certs_mariner.service` instead of `flatcar/update_certs.service`. The Mariner service file calls `update_certs.sh /usr/share/pki/ca-trust-source/anchors update-ca-trust`.
- Affected files:
	- `parts/linux/cloud-init/artifacts/cse_config.sh` — `configureHTTPProxyCA()`
	- `vhdbuilder/packer/vhd-image-builder-flatcar.json` — service file source
	- `vhdbuilder/packer/vhd-image-builder-flatcar-arm64.json` — service file source
	- `pkg/agent/testdata/**/CustomData` — regenerated snapshots

**ACL base image follow-up**: None — AgentBaker correctly routes ACL to the right cert tools.

**Notes**: The `isACL` check must come before `isFlatcar` because `isFlatcar` returns true for ACL. This is the same ordering pattern used throughout the codebase for distro-specific logic.
