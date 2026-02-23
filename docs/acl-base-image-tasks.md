# ACL Base Image — Requested Changes for AKS Node Provisioning

---

## Background

ACL (Azure Container Linux) is a Flatcar Container Linux derivative that replaces Flatcar's Gentoo-based package layer with Azure Linux 3 RPMs. It keeps Flatcar's immutable filesystem layout (read-only `/usr`, overlay `/etc`, separate `/var` populated at boot via `systemd-tmpfiles`) but uses Azure Linux packages instead of Gentoo ebuilds.

**What is AgentBaker?** AgentBaker is the AKS (Azure Kubernetes Service) component that provisions new nodes. It builds VHD (Virtual Hard Disk) base images using Packer and generates node bootstrap scripts. While integrating the ACL base image into the AgentBaker VHD pipeline, we hit several gaps — missing packages, configs, or files that upstream Flatcar provides but ACL doesn't yet. We've added workarounds in AgentBaker, but each task below would be better addressed in the ACL base image so we can remove those workarounds.

**How ACL packages work:** In `acl-scripts`, `build_library/rpm/package_catalog.sh` maps Flatcar Gentoo package names to Azure Linux RPM equivalents. Entries marked `SKIP` mean the Flatcar package is excluded and no RPM substitute is installed. Several tasks below stem from `SKIP`ped packages that provide files needed on Azure VMs.

**Relevant repos:**
| Repo | Purpose |
|------|---------|
| `acl-scripts` | ACL image build scripts (all file paths below are relative to this repo unless noted) |
| `AgentBaker` | AKS node provisioning ([Azure/AgentBaker](https://github.com/Azure/AgentBaker), branch [`aadagarwal/acl-v20260127`](https://github.com/Azure/AgentBaker/tree/aadagarwal/acl-v20260127)) |
| `azurelinux` | Azure Linux 3 RPM specs |

---

## How to read this document

Each task is self-contained. For each one you'll find:

1. **What's wrong** — the symptom when booting ACL on Azure
2. **Why it happens** — root cause in `acl-scripts` with file paths and line numbers
3. **What to change** — a concrete fix
4. **How to verify** — how to confirm the fix works

---

## Task Summary

| # | Task | File to change (in `acl-scripts`) | Workaround in |
|---|------|-----------------------------------|---------------|
| 1 | [Install `azure-vm-utils` package](#task-1-install-azure-vm-utils-package) | `build_library/rpm/package_catalog.sh` | VHD build |
| 2 | [Ship Azure-optimized chrony config](#task-2-ship-azure-optimized-chrony-config) | `coreos-base/oem-azure/files/manglefs.sh` | VHD build, VHD tests, E2E tests |
| 3 | [Add `tmpfiles.d/logrotate.conf`](#task-3-add-tmpfilesdlogrotateconf) | `build_library/rpm/build_image_util.sh` | VHD build |
| 4 | [Add boot-time systemd preset processing](#task-4-add-boot-time-systemd-preset-processing) | New service file (via `manglefs.sh` or `build_image_util.sh`) | VHD build, E2E tests |
| 5 | [Fix `update-ssh-keys` cross-device mv bug](#task-5-fix-update-ssh-keys-cross-device-mv-bug) | `build_library/rpm/additional_files/update-ssh-keys` | E2E tests |
| 6 | [Remove or fix `/etc/profile.d/umask.sh`](#task-6-remove-or-fix-etcprofiledumasksh) | `build_library/rpm/build_image_util.sh` | VHD tests |
| 7 | [Evaluate cloud-init distro detection](#task-7-evaluate-cloud-init-distro-detection) | Investigation | — |

### Tasks by workaround layer

**VHD build (packer)** — workarounds baked into the VHD at build time:
- [Task 1](#task-1-install-azure-vm-utils-package): Inline udev rules in `pre-install-dependencies.sh`
- [Task 2](#task-2-ship-azure-optimized-chrony-config): Azure-tuned chrony config in `tool_installs_flatcar.sh`
- [Task 3](#task-3-add-tmpfilesdlogrotateconf): `mkdir -p /var/lib/logrotate` in `pre-install-dependencies.sh`
- [Task 4](#task-4-add-boot-time-systemd-preset-processing): Explicit enable symlinks in `flatcar.yml`

**VHD content tests** — test adjustments for ACL in `linux-vhd-content-test.sh`:
- [Task 2](#task-2-ship-azure-optimized-chrony-config): `testChrony` uses `chronyd` service name for Flatcar/ACL
- [Task 6](#task-6-remove-or-fix-etcprofiledumasksh): `testUmaskSettings` skipped for Flatcar/ACL

**AgentBaker E2E tests** — test adjustments in `e2e/`:
- [Task 2](#task-2-ship-azure-optimized-chrony-config): Pre-existing chrony restart tests cover ACL via Flatcar scenarios
- [Task 4](#task-4-add-boot-time-systemd-preset-processing): `DebugIgnitionPresetMechanism()` diagnostic helper in `validators.go` (debug only, not wired into tests)
- [Task 5](#task-5-fix-update-ssh-keys-cross-device-mv-bug): `update-ssh-keys-after-ignition.service` added to failure allowlist in `validators.go`

---

## Task 1: Install `azure-vm-utils` package

### What's wrong

Azure VMs rely on udev rules to create `/dev/disk/azure/{root,os,resource}` symlinks. These symlinks are required by any Azure VM for disk identification — particularly on NVMe-capable VM sizes where the `/usr/sbin/azure-nvme-id` binary is needed. Without them, disk operations fail.

### Why it happens

In `build_library/rpm/package_catalog.sh` (line 248):
```bash
["sys-apps/azure-vm-utils"]="SKIP"
```

The Flatcar Gentoo ebuild `sys-apps/azure-vm-utils` (v0.6.0 stable / v0.7.0 testing, from https://github.com/Azure/azure-vm-utils) ships:
- `80-azure-disk.rules` — combined SCSI + NVMe udev rules that create `/dev/disk/azure/` symlinks
- `/usr/sbin/azure-nvme-id` — NVMe device identification binary

Because it's `SKIP`ped, neither file reaches the ACL image.

### What to change

Azure Linux 3 currently ships the **older** `azure-nvme-utils` package (v0.1.1), which only provides NVMe-specific rules (`80-azure-nvme.rules`) — **not** the combined SCSI+NVMe rules that `azure-vm-utils` provides. The upstream project was renamed from `azure-nvme-utils` to `azure-vm-utils` starting at v0.3.0.

**Upstream issue:** [microsoft/azurelinux#15661](https://github.com/microsoft/azurelinux/issues/15661) — tracks updating `azure-nvme-utils` to `azure-vm-utils` in Azure Linux.

**Two options:**

**Option A (recommended)**: Update the Azure Linux `azure-nvme-utils` spec to the current upstream `azure-vm-utils` (v0.6.0+), which covers both SCSI and NVMe. Then map it in `package_catalog.sh`:
```bash
["sys-apps/azure-vm-utils"]="azure-vm-utils"   # or whatever the updated RPM is named
```

**Option B**: If updating the Azure Linux RPM is not feasible short-term, install the Flatcar Gentoo package directly by removing the `SKIP` (this means building from the ebuild at `sdk_container/src/third_party/portage-stable/sys-apps/azure-vm-utils/` rather than using an RPM). This is less clean but gets the files into the image.

> **Note:** On standard Azure Linux VMs (non-ACL), SCSI disk symlinks come from WALinuxAgent's `66-azure-storage.rules`, but WALinuxAgent [skips udev rule installation on Flatcar >= 3550](https://github.com/Azure/WALinuxAgent). That's why ACL needs `azure-vm-utils` to provide its own rules.

### How to verify

```bash
# The udev rules file exists (combined SCSI+NVMe)
test -f /usr/lib/udev/rules.d/80-azure-disk.rules && echo "OK: udev rules present"

# The NVMe ID binary exists
test -f /usr/sbin/azure-nvme-id && echo "OK: azure-nvme-id present"

# On an Azure VM, symlinks are created at boot
ls -la /dev/disk/azure/
# Expected: root, os, resource symlinks pointing to real devices
```

### Current AgentBaker workarounds

**VHD build (packer):** We copy ~65 lines of inline udev rules (matching `azure-vm-utils` v0.7.0) into the VHD at build time in `vhdbuilder/packer/pre-install-dependencies.sh`. This handles SCSI disk identification but the `azure-nvme-id` binary is still missing, so NVMe disk naming doesn't work. Once the package is in the base image, this workaround is removed automatically (guarded by a file-existence check).
- Commit: [`04075c8`](https://github.com/Azure/AgentBaker/commit/04075c8463)

**VHD content tests:** No changes needed.

**AgentBaker E2E tests:** No changes needed.

---

## Task 2: Ship Azure-optimized chrony config

### What's wrong

ACL ships chrony with the **default Azure Linux RPM config** (`makestep 1.0 3`), which only corrects the clock during the first 3 NTP updates, then switches to gradual slewing. Azure VMs can wake from hibernation or migration with large time offsets that need immediate correction. The correct Azure config uses `makestep 1.0 -1` (always step) and `refclock PHC /dev/ptp_hyperv` (use Hyper-V PTP clock for sub-microsecond accuracy).

The correct config files **already exist in acl-scripts** — they're just not being installed into the final image.

### Why it happens

The oem-azure overlay at `sdk_container/src/third_party/coreos-overlay/coreos-base/oem-azure/files/` contains the correct Azure-optimized files:

| File | Purpose |
|------|---------|
| `chrony.conf` | Azure-tuned config with `makestep 1.0 -1` and `refclock PHC /dev/ptp_hyperv` |
| `chrony-hyperv.conf` | systemd drop-in adding `Wants=dev-ptp_hyperv.device` / `After=dev-ptp_hyperv.device` |
| `var-chrony.conf` | tmpfiles config to create `/var/lib/chrony` at boot |
| `etc-chrony.conf` | tmpfiles config for `/etc/chrony/` directory and symlink |

However, `coreos-base/oem-azure` is `SKIP`ped in `package_catalog.sh` (line 246), so the ebuild's `src_install()` never runs and none of these files reach the image.

The oem-azure `manglefs.sh` (lines 60–88) already moves chrony configs from `/etc` to `/usr/lib/chrony/` and patches `chronyd.service` to use `-f /usr/lib/chrony/chrony.conf`. But it moves the **Azure Linux RPM default** config, not the Azure-optimized one from the overlay.

### What to change

In `sdk_container/src/third_party/coreos-overlay/coreos-base/oem-azure/files/manglefs.sh`, add after the existing chrony handling (after the `chronyd.service` sed block, around line 88):

```bash
# Copy Azure-optimized chrony.conf from this directory.
# Overwrites the RPM default that manglefs already moved from /etc.
# Key differences: makestep 1.0 -1 (always-step), PTP refclock for Hyper-V clock.
script_dir="$(dirname "${BASH_SOURCE[0]}")"

# 1. Azure-optimized chrony.conf → /usr/lib/chrony/chrony.conf
if [[ -f "${script_dir}/chrony.conf" ]]; then
    cp "${script_dir}/chrony.conf" "${rootfs}/usr/lib/chrony/chrony.conf"
fi

# 2. chronyd.service drop-in (Wants/After dev-ptp_hyperv.device)
if [[ -f "${script_dir}/chrony-hyperv.conf" ]]; then
    mkdir -p "${rootfs}/usr/lib/systemd/system/chronyd.service.d"
    cp "${script_dir}/chrony-hyperv.conf" \
       "${rootfs}/usr/lib/systemd/system/chronyd.service.d/"
fi

# 3. var-chrony.conf tmpfiles (creates /var/lib/chrony at boot)
if [[ -f "${script_dir}/var-chrony.conf" ]]; then
    cp "${script_dir}/var-chrony.conf" "${rootfs}/usr/lib/tmpfiles.d/"
fi
```

**Important — chrony user/group mismatch:** The overlay's `var-chrony.conf` uses `ntp:ntp` (Flatcar convention), but the Azure Linux chrony RPM uses `chrony:chrony`. `build_image_util.sh` (line ~614) already creates a sysusers.d entry for the `chrony` user, and line ~461 already creates a `chrony.conf` tmpfiles entry using `0755 root root`. Either:
- Update `var-chrony.conf` to use `chrony:chrony` to match the Azure Linux RPM, or
- Rely on the existing tmpfiles entry in `build_image_util.sh` and skip copying `var-chrony.conf`

**Note on `etc-chrony.conf`:** This file creates a symlink `/etc/chrony/chrony.conf → ../../usr/share/oem-azure/chrony.conf`, which is a Flatcar ebuild path that doesn't exist on ACL. Since `manglefs.sh` patches `chronyd.service` to use `-f /usr/lib/chrony/chrony.conf` directly, the symlink is unnecessary. You can skip copying `etc-chrony.conf` or update its symlink target to `../../usr/lib/chrony/chrony.conf`.

### How to verify

Boot an ACL VM on Azure:
```bash
# Config has PTP refclock and always-step
grep -q 'refclock PHC /dev/ptp_hyperv' /usr/lib/chrony/chrony.conf && echo "OK: PTP configured"
grep -q 'makestep 1.0 -1' /usr/lib/chrony/chrony.conf && echo "OK: always-step configured"

# chrony is using PTP
chronyc sources
# Expected: line starting with #* PHC0 or similar PTP source

# Time correction test (set clock back, verify fast correction)
sudo date -s "2021-01-01"
sleep 30
date  # Should be back to current time within ~20 seconds
```

### Current AgentBaker workarounds

**VHD build (packer):** We write the correct chrony config to `/etc/chrony/chrony.conf` (writable overlay path) and add a systemd drop-in to override `ExecStart` to use our config — ~110 lines in `vhdbuilder/scripts/linux/flatcar/tool_installs_flatcar.sh`.
- Commit: [`91fb4c3`](https://github.com/Azure/AgentBaker/commit/91fb4c3846)

**VHD content tests:** `testChrony` in `vhdbuilder/packer/test/linux-vhd-content-test.sh` uses service name `chronyd` for Flatcar/ACL (same as Mariner/AzureLinux). The time-correction subtest runs fully for ACL — no skip.

**AgentBaker E2E tests:** Pre-existing chrony restart tests (`Test_Flatcar_AzureCNI_ChronyRestarts`) in `e2e/scenario_test.go` cover ACL since it shares the Flatcar VHD config.

---

## Task 3: Add `tmpfiles.d/logrotate.conf`

### What's wrong

`logrotate.service` fails on every boot with:
```
error creating stub state file /var/lib/logrotate/logrotate.status: No such file or directory
```

Log rotation never runs, meaning logs grow unbounded until disk fills up.

### Why it happens

ACL has an immutable rootfs where `/var` is a separate partition populated at boot via `systemd-tmpfiles`. The Azure Linux 3 `logrotate` RPM (v3.21.0 — see `SPECS/logrotate/logrotate.spec` in the `azurelinux` repo) creates `/var/lib/logrotate/` at RPM install time but ships **no** `tmpfiles.d` drop-in to recreate it at boot.

On a traditional mutable filesystem (regular Azure Linux), the directory persists across reboots. On ACL's Flatcar-style layout, `/var` is empty at boot and must be populated from `tmpfiles.d` declarations — so the directory disappears.

`build_image_util.sh` already has `sudo mkdir -p "${root_fs_dir}/var/lib/logrotate"` (line ~1182) which creates the directory at build time, but this is not sufficient for the same reason — it doesn't persist.

### What to change

Add a single tmpfiles.d entry in `build_library/rpm/build_image_util.sh`, near the existing `mkdir -p` for logrotate:

```bash
# Create tmpfiles.d entry for logrotate state directory.
# The Azure Linux 3 logrotate RPM doesn't ship a tmpfiles.d drop-in,
# so /var/lib/logrotate is not recreated at boot on ACL's immutable rootfs.
echo 'd /var/lib/logrotate 0755 root root -' | sudo tee "${root_fs_dir}/usr/lib/tmpfiles.d/logrotate.conf" > /dev/null
```

That's it — one line of content in one file. Uses `${root_fs_dir}` and `sudo tee` to match the patterns already used in `build_image_util.sh`.

### How to verify

```bash
# Directory exists after boot
test -d /var/lib/logrotate && echo "OK: logrotate dir exists"

# Service runs successfully
sudo systemctl start logrotate.service
systemctl status logrotate.service
# Expected: exit status 0

# State file created
test -f /var/lib/logrotate/logrotate.status && echo "OK: state file exists"
```

### Current AgentBaker workarounds

**VHD build (packer):** We run `mkdir -p /var/lib/logrotate` at VHD build time in `vhdbuilder/packer/pre-install-dependencies.sh`. This creates the directory in the VHD but it won't persist across reboots on fresh VMs if `/var` is repopulated.
- Commit: [`5d762e8`](https://github.com/Azure/AgentBaker/commit/5d762e88b6)

**VHD content tests:** No changes needed.

**AgentBaker E2E tests:** No changes needed.

---

## Task 4: Add boot-time systemd preset processing

### What's wrong

When services are configured via Ignition with `enabled: true`, Ignition writes a preset file at `/etc/systemd/system-preset/20-ignition.preset`. On upstream Flatcar, `systemd-preset-all.service` processes this file at first boot and creates the enable symlinks. On ACL, this service doesn't exist, so the preset file is never processed and Ignition-enabled services never start.

This affects any ACL user who uses Ignition to enable services — not just AKS.

### Why it happens

Three things combine:

1. **`systemd-preset-all.service` does not exist**: This service was introduced in upstream systemd v256. Azure Linux ships systemd v255 (see `SPECS/systemd/systemd.spec` in the `azurelinux` repo). Azure Linux handles presets at RPM install time via `%post` scriptlets — not at boot.

2. **First-boot detection fails**: Azure Linux's systemd is built with `-Dfirst-boot-full-preset=true` (line 703 of the systemd spec), which makes PID 1 internally run preset-all on first boot — but only if `ConditionFirstBoot=yes` is true, which requires `/etc/machine-id` to be empty or missing. Despite `build_image_util.sh` (line ~1245) removing machine-id from the overlay lowerdir, something repopulates it before PID 1 evaluates the condition.

3. **`99-default-disable.preset` catch-all**: The `azurelinux-release` RPM ships `/usr/lib/systemd/system-preset/99-default-disable.preset` containing `disable *`. Even if presets were re-applied, only services explicitly enabled in higher-priority preset files (like Ignition's `20-ignition.preset` or the `azurelinux-release` `90-default.preset`) would survive.

### What to change

**Option A (recommended)**: Create a simple early-boot service that processes preset files:

```ini
# /usr/lib/systemd/system/acl-preset-all.service
[Unit]
Description=Apply Preset Settings
DefaultDependencies=no
Conflicts=shutdown.target
After=local-fs.target
Before=sysinit.target

[Service]
Type=oneshot
ExecStart=/usr/bin/systemctl preset-all
RemainAfterExit=yes

[Install]
WantedBy=sysinit.target
```

Enable it **statically** (don't rely on presets to enable the preset service):
```bash
sudo mkdir -p "${root_fs_dir}/usr/lib/systemd/system/sysinit.target.wants"
sudo ln -sf ../acl-preset-all.service \
   "${root_fs_dir}/usr/lib/systemd/system/sysinit.target.wants/acl-preset-all.service"
```

This could be added in `manglefs.sh` or `build_image_util.sh`.

> **Caution:** `systemctl preset-all` will re-apply the `99-default-disable.preset` catch-all (`disable *`), which disables every service not explicitly listed in a higher-priority preset file. Ignition writes to `/etc/systemd/system-preset/20-ignition.preset` (priority 20 > 99), so Ignition-enabled services will be correctly enabled. But verify no critical services are unexpectedly disabled. The `90-default.preset` from `azurelinux-release` explicitly enables `sshd`, `chronyd`, `waagent`, `cloud-init`, `systemd-networkd`, etc.

**Option B**: Fix the first-boot detection chain so systemd's built-in `-Dfirst-boot-full-preset=true` works. This requires investigating why `/etc/machine-id` is populated before PID 1 evaluates `ConditionFirstBoot=yes`. Key places to check:
- `build_image_util.sh` line ~1245 (removes machine-id from lowerdir `/usr/share/flatcar/etc`)
- Bootengine `initrd-setup-root` (may remove blank machine-id, causing systemd to regenerate it)
- `systemd-machine-id-setup` in initrd (may run before switch-root)

### How to verify

Create a test Ignition config that enables a simple service:
```yaml
systemd:
  units:
    - name: test-preset.service
      enabled: true
      contents: |
        [Unit]
        Description=Test Preset
        [Service]
        Type=oneshot
        ExecStart=/bin/echo "preset works"
        [Install]
        WantedBy=multi-user.target
```

Boot a VM with this config:
```bash
systemctl is-enabled test-preset.service  # Expected: enabled
systemctl status test-preset.service      # Expected: inactive (dead), exit 0
cat /etc/systemd/system-preset/20-ignition.preset
# Expected: "enable test-preset.service"
```

### Current AgentBaker workarounds

**VHD build (packer):** In the Ignition config (`parts/linux/cloud-init/flatcar.yml`), we bypass presets entirely by using Ignition's `storage.links` to directly create enable symlinks:
```yaml
storage:
  links:
    - path: /etc/systemd/system/sysinit.target.wants/ignition-file-extract.service
      target: /etc/systemd/system/ignition-file-extract.service
```
This works for our specific services, but any other Ignition user on ACL who uses `enabled: true` will hit the same issue.
- Commit: [`a108d2b`](https://github.com/Azure/AgentBaker/commit/a108d2b457)

**VHD content tests:** No changes needed.

**AgentBaker E2E tests:** `DebugIgnitionPresetMechanism()` in `e2e/validators.go` is a diagnostic helper that checks preset files, machine-id state, and enable symlinks. It's a debug utility only — not wired into any test.
- Commit: [`16c956ad`](https://github.com/Azure/AgentBaker/commit/16c956ad7c)

---

## Task 5: Fix `update-ssh-keys` cross-device mv bug

### What's wrong

`update-ssh-keys-after-ignition.service` fails on boot with:
```
mv: cannot create regular file '/home/core/.ssh/authorized_keys': File exists
```

SSH still works (waagent separately creates `authorized_keys`), but the failed service generates log noise.

### Why it happens

ACL replaces Flatcar's Rust `update-ssh-keys` binary with a bash script at `build_library/rpm/additional_files/update-ssh-keys` (the Rust package `coreos-base/update-ssh-keys` is `SKIP`ped in `package_catalog.sh`, line 244).

The bash script's `regenerate()` function (around line 134) creates a temp file in `/tmp` then tries to `mv` it to `/home/core/.ssh/authorized_keys`:

```bash
temp_file=$(mktemp)                    # creates in /tmp (tmpfs)
# ... build content ...
mv "$temp_file" "$KEYS_FILE"           # target is on /home (ext4)
```

This fails because:
- `/tmp` is a tmpfs, `/home` is ext4 — **different filesystems**
- Cross-device `mv` cannot use atomic `rename(2)` and falls back to copy+delete, which fails with `EEXIST` when the target already exists
- The script uses `set -euo pipefail`, so the failure causes immediate exit

Flatcar's Rust binary creates temp files in the same directory as the target (same filesystem), so `rename(2)` atomically replaces the file.

### What to change

In `build_library/rpm/additional_files/update-ssh-keys`, change two `mktemp` calls:

**1. In `regenerate()`** (around line 134):
```bash
# BEFORE:
temp_file=$(mktemp)

# AFTER:
temp_file=$(mktemp "${KEYS_FILE}.XXXXXX")
```

**2. In the `add|force-add` case** (around line 165):
```bash
# BEFORE:
temp_key=$(mktemp)

# AFTER:
temp_key=$(mktemp "${key_file}.XXXXXX")
```

By creating temp files adjacent to their targets (same directory = same filesystem), `mv` uses `rename(2)` which atomically **overwrites** existing files.

### How to verify

```bash
# Boot an ACL VM and check the service
systemctl status update-ssh-keys-after-ignition.service
# Expected: inactive (dead) with exit code 0 (not failed)

# Verify authorized_keys exists
test -f /home/core/.ssh/authorized_keys && echo "OK"

# Manual test
sudo /usr/bin/update-ssh-keys -a test_key < /dev/null 2>&1
echo $?  # Expected: 0
```

### Current AgentBaker workarounds

**VHD build (packer):** No changes needed — the bug is in the ACL base image's bash script.

**VHD content tests:** No changes needed.

**AgentBaker E2E tests:** Added `update-ssh-keys-after-ignition.service` to the failure allowlist in `e2e/validators.go` (`ValidateNoFailedSystemdUnits`), suppressing the test failure since actual SSH behavior is unaffected.
- Commit: [`965f7fd`](https://github.com/Azure/AgentBaker/commit/965f7fd1d6)

---

## Task 6: Remove or fix `/etc/profile.d/umask.sh`

### What's wrong

ACL ships `/etc/profile.d/umask.sh` with conditional umask values (`umask 002` when user/group names match, `umask 022` otherwise). Neither value meets CIS hardening standards (CIS requires `umask 027` or stricter). AKS VHD content tests verify CIS compliance and fail on ACL.

### Why it happens

The file is installed by the Azure Linux `bash` RPM (see `SPECS/bash/bash.spec` in the `azurelinux` repo, `%install` section). Upstream Flatcar (version 4593+) removed `/etc/profile.d/umask.sh` entirely. The Azure Linux `filesystem` RPM also removed its umask handling (Nov 2023 changelog), but the `bash` RPM still creates it.

On ACL, `/etc` is an overlay, so the file can't be persistently modified at VHD build time.

### What to change

**Option A (recommended — match Flatcar)**: Remove the file during image build in `build_library/rpm/build_image_util.sh`:
```bash
sudo rm -f "${root_fs_dir}/etc/profile.d/umask.sh"
```

**Option B**: Replace with CIS-compliant content:
```bash
echo 'umask 027' | sudo tee "${root_fs_dir}/etc/profile.d/umask.sh" > /dev/null
```

### How to verify

```bash
# Option A:
test ! -f /etc/profile.d/umask.sh && echo "OK: file removed"
# Option B:
grep -q 'umask 027' /etc/profile.d/umask.sh && echo "OK: CIS compliant"
```

### Current AgentBaker workarounds

**VHD build (packer):** No changes possible — ACL uses an overlay filesystem on `/etc`, so modifications to `/etc/profile.d/umask.sh` at VHD build time don't persist.

**VHD content tests:** `testUmaskSettings` in `vhdbuilder/packer/test/linux-vhd-content-test.sh` is skipped for Flatcar/ACL since the overlay filesystem prevents persistent modification.
- Commit: [`91fb4c3`](https://github.com/Azure/AgentBaker/commit/91fb4c3846)

**AgentBaker E2E tests:** No changes needed.

---

## Task 7: Evaluate cloud-init distro detection

### What's wrong

Cloud-init logs a warning during boot:
```
Unable to load distro implementation for acl. Using default distro implementation instead.
```

### Why it happens

Cloud-init's distro library does not have a class for `acl`. ACL's `/etc/os-release` currently sets `ID=flatcar` (inherited from the Flatcar fork). Cloud-init upstream does have Flatcar support, so the warning may be caused by a cloud-init configuration override (e.g., `distro:` in `/etc/cloud/cloud.cfg`) that specifies `acl` instead of `flatcar`.

The Azure Linux cloud-init spec (v24.3.1) builds with `--variant azurelinux`, which renders a `cloud.cfg` configured for the `azurelinux` distro. If ACL uses this cloud.cfg unchanged, cloud-init may attempt `azurelinux` or `acl` distro detection rather than `flatcar`.

### What to change

This is an investigation item. Steps to debug:
1. Check what `distro:` is set in `/etc/cloud/cloud.cfg` on a running ACL VM
2. Check `ID=` in `/etc/os-release` on a running ACL VM
3. Check cloud-init's distro detection path in the logs: `grep -i distro /var/log/cloud-init.log`

Options once root cause is identified:
1. If `cloud.cfg` specifies the wrong distro, fix it in the ACL image build to use `flatcar` (or whichever distro class matches)
2. Contribute an ACL distro class to cloud-init upstream
3. Accept the warning if default behavior is sufficient for all use cases

### How to verify

```bash
cloud-init status --long
grep -i "distro\|Unable to load" /var/log/cloud-init.log
```

---

## Appendix A: Root cause pattern — RPM `SKIP` gaps

Most tasks above stem from a single pattern: Flatcar Gentoo packages that are `SKIP`ped in `build_library/rpm/package_catalog.sh` provide files needed on Azure VMs, and the Azure Linux RPM substitute either doesn't exist or is incomplete.

| Package (Gentoo name) | Line | What's missing | Task |
|------------------------|------|----------------|------|
| `sys-apps/azure-vm-utils` | 248 | udev disk rules + NVMe ID binary (Azure Linux has `azure-nvme-utils` but it's NVMe-only, v0.1.1) | Task 1 |
| `coreos-base/oem-azure` | 246 | Azure-tuned chrony config, systemd drop-ins, tmpfiles | Task 2 |
| `coreos-base/update-ssh-keys` | 244 | Rust binary replaced by bash script with cross-device `mv` bug | Task 5 |

When reviewing other `SKIP`ped packages, consider whether they provide files needed for Azure VM operation.

## Appendix B: Root cause pattern — missing `tmpfiles.d` entries

ACL's immutable rootfs uses a separate `/var` partition populated at boot via `systemd-tmpfiles`. Azure Linux RPMs assume a persistent rootfs, so directories created at RPM install time under `/var` don't survive reboots on ACL.

Any RPM that creates directories under `/var/lib/`, `/var/log/`, or `/var/cache/` without shipping a corresponding `tmpfiles.d` drop-in will break on ACL.

Known instances:
| RPM | Missing `tmpfiles.d` entry | Task |
|-----|---------------------------|------|
| `logrotate` (v3.21.0) | `d /var/lib/logrotate 0755 root root -` | Task 3 |
| `chrony` (v4.5) | `d /var/lib/chrony 0770 chrony chrony -` (handled by `build_image_util.sh` line ~461, but with `0755 root root` ownership) | Task 2 |

Audit command to find more:
```bash
# Find RPMs that own /var paths but don't ship tmpfiles.d entries
rpm -qa --queryformat '%{NAME}\n' | while read pkg; do
    has_var=$(rpm -ql "$pkg" 2>/dev/null | grep -c '^/var/')
    has_tmpfiles=$(rpm -ql "$pkg" 2>/dev/null | grep -c 'tmpfiles.d')
    if [[ $has_var -gt 0 && $has_tmpfiles -eq 0 ]]; then
        echo "MISSING: $pkg has /var files but no tmpfiles.d entry"
    fi
done
```

## Appendix C: Changes handled in AgentBaker (no base image changes needed)

These changes were made in AgentBaker's `aadagarwal/acl-v20260127` branch. They don't require ACL base image changes — listed for context only.

| Change | What it does | Commit |
|--------|-------------|--------|
| OS detection | Extended `isFlatcar()` to match ACL, added `isACL()` helper (`parts/linux/cloud-init/artifacts/cse_helpers.sh`) | [`683984748f`](https://github.com/Azure/AgentBaker/commit/683984748f) |
| Package fallback | `components.json` lookups fall back from `acl` → `flatcar` entries | [`fcf079e68b`](https://github.com/Azure/AgentBaker/commit/fcf079e68b) |
| Disable iptables | Masks Azure Linux's `iptables.service` (which ships `FORWARD DROP`/`INPUT DROP` rules that block Cilium eBPF) — see [details below](#disable-iptablesservice) | [`1b3e82ee52`](https://github.com/Azure/AgentBaker/commit/1b3e82ee52) |
| Disable resolved cache | Repoints `resolv.conf` to upstream DNS for localdns compatibility — see [details below](#disable-systemd-resolved-cache) | [`6d7d99c6a4`](https://github.com/Azure/AgentBaker/commit/6d7d99c6a4) |
| CA cert path | Routes CA certs to `/etc/pki/ca-trust/source/anchors` with `update-ca-trust` (ACL's `/usr` is read-only, so certs go under writable `/etc`) | [`1e414f4ec9`](https://github.com/Azure/AgentBaker/commit/1e414f4ec9) |
| Kubelet install | Routes ACL through Flatcar path (install kubelet/kubectl from URL, not package manager) | [`e466ce277f`](https://github.com/Azure/AgentBaker/commit/e466ce277f) |
| waagent PATH | Uses `waagent` via PATH instead of hardcoded `/usr/sbin/waagent` (ACL has it at `/usr/bin/waagent`) | [`91d7246adb`](https://github.com/Azure/AgentBaker/commit/91d7246adb) |

### Disable iptables.service

ACL inherits Azure Linux's `iptables.service`, which loads host firewall rules including `FORWARD DROP` and `INPUT DROP` chains. These block Cilium eBPF host routing and pod-to-pod traffic on AKS nodes. (Original Flatcar's `iptables.service` loads empty rules.)

Standard Azure Linux VHDs already disable iptables in their build path, but `isMarinerOrAzureLinux("ACL")` returns `false`, so ACL was skipped. Fix: call `disableSystemdIptables` (runs `systemctl disable --now iptables && systemctl mask iptables`) for ACL during VHD build.

### Disable systemd-resolved cache

AKS uses _localdns_ — a per-node CoreDNS that listens on `169.254.10.10`. When localdns starts, it configures `systemd-resolved` to use `169.254.10.10` as its upstream, which updates `/run/systemd/resolve/resolv.conf`. But the stub at `/run/systemd/resolve/stub-resolv.conf` always shows `127.0.0.53`.

On ACL, `/etc/resolv.conf` points to the stub by default. DNS queries work (app → `127.0.0.53` → resolved → `169.254.10.10` → localdns → upstream), but the extra hop through resolved's stub adds latency and a conflicting cache layer.

Standard Azure Linux VHDs install `resolv-uplink-override.service` — a oneshot that runs `ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf` at boot (after `systemd-networkd`, before `kubelet`). ACL was skipped because `isMarinerOrAzureLinux("ACL")` returns `false`. Fix: call `disableSystemdResolvedCache` for ACL during VHD build.
