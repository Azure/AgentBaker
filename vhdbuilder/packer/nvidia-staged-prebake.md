# Opt-in: stage the whole NVIDIA CUDA prebake

This is an independent alternative to parking only DKMS registration. It does
**not** enable `install-skip-build` or change DKMS's global `dkms_tree`.

## Rollout and rollback

1. Release the updated CSE first, including validation-only and PIS consumers.
2. Validate both Ubuntu 22.04 and 24.04 x86-64 images using
   `FEATURE_FLAGS=NVIDIA_CUDA_PREBAKE,NVIDIA_PREBAKE_STAGE`.
   Use this exact comma-separated combination: Packer currently transports this
   value as an unquoted `sudo` environment-assignment argument. Spaces split it
   into separate arguments. The release report condition accepts this combination.
3. Only then enable staged images for compatible consumers. Pipeline defaults
   remain `NVIDIA_CUDA_PREBAKE`, which still builds the existing registered layout.

To roll back image production, remove only `NVIDIA_PREBAKE_STAGE`. Keep the
updated CSE until staged images are retired. **Do not roll CSE back underneath
staged images:** old validation-only CSE can try loading a driver without knowing
how to restore it. New CSE continues to support old registered images, including
their existing CPU/opt-out and GRID cleanup.

## Owned payload and inactivity

`build-only` runs the NVIDIA runfile installer, not merely DKMS. Its authoritative
installed-file/symlink inventory is `/var/lib/nvidia/log`. Staging uses that
inventory rather than moving all of `/usr/bin`, all DKMS modules, or a guessed
list of executables. This includes installer-owned binaries (including setuid
`nvidia-modprobe`), firmware, EGL/Vulkan/X configuration, service units, udev
configuration and symlinks when installed by the runfile.

The small manifest additionally records:

- `/var/lib/dkms/nvidia` and the runfile uninstall bookkeeping `/var/lib/nvidia`;
- NVIDIA's named `.ko` files, including compressed forms, in `updates/dkms`;
- the freshly created `/usr/bin/lib64` library overlay;
- the producer's nouveau blacklist, linker configuration and existing DKMS marker.

`/usr/src/nvidia-<version>` stays in place, so absolute DKMS `source` symlinks
remain usable even in the cache. Unrelated modules, files, users and directories
are not removed. An existing prebake, library overlay or producer configuration
is rejected before building, as are pre-existing NVIDIA modules or linker entries.
An unknown runfile-log format or replacement of a
pre-existing file outside the disposable library overlay fails staging rather
than guessing ownership.

The payload moves under `/opt/azure/aks-gpu/staged`, owned by root with mode
`0700`. **Being off PATH is not sufficient for setuid binaries:** directory
access prevents unprivileged execution while retaining original file modes and
ownership for restoration. This is not a production security certification.

Staging refreshes module/linker state, reloads systemd, regenerates **all**
generated initrds, and inspects each with `lsinitramfs`. Leftover NVIDIA modules
outside the owned module locations, failed refreshes, missing initrds, or NVIDIA
module/configuration/binary exposure in an initrd fail the opt-in image build.
The pre-existing AgentBaker `nvidia-modprobe.service` definition remains; without
the executable or any loadable NVIDIA module it cannot activate this payload.

## Provisioning and failure handling

The existing `nodePrep` GPU dispatch uses live `GPU_NODE` and skip decisions,
including real PIS nodes that skip `basePrep`. Managed CUDA/CUDA-LTS nodes restore
before either installer or validation-only dispatch. Other nodes remove only the
owned inactive cache; GRID never restores CUDA. Old images keep legacy cleanup.

Restoration refreshes depmod, ldconfig and systemd. A restored **build-only**
payload still lacks device/runtime initialization, including the container
toolkit and fabric manager. The first provisioning attempt therefore requires
the ordinary installer **even when validation-only was requested**. A root-only
`staged-needs-install` directory preserves that requirement across failed CSE
attempts; it is retired only after successful installation. There is no new
skip-build path or claim that matching cached artifacts alone provide a working
GPU container runtime.

Subsequent validation-only nodes with a mismatched kernel, architecture or driver version,
missing DKMS registration, or a missing/mismatched module go through the normal
installer. A successful `nvidia-modprobe` alone is not accepted as proof of a
registered restored driver. There is no `dkms install` shortcut.

A kernel update while the cache is inactive does not build NVIDIA for the new
kernel. Restoring cannot manufacture those modules or replay missed kernel
hooks. Repair can compile from source and has **unmeasured provisioning latency**.
Normal installer failures remain failures.

The manifest supports interrupted file moves; a sentinel distinguishes staging
from restoration. A transaction requires same-filesystem renames, refuses
collisions before moving files, checks `mv -n` postconditions, and retains state
on a failed refresh. Missing files in an otherwise fully inactive cache select
ordinary installation without restoring a partial payload. Active collisions
still fail. Before recursive deletion the cache is atomically renamed to the
root-only `staged-discard` location, so interrupted deletion remains retryable
even after the internal manifest/sentinel is gone.
Separate `/usr`, `/var`, or `/opt` mounts are not supported
by this opt-in. A failed/interrupted build without a complete manifest must be
rebuilt, not published. Incomplete transactions are never treated as disposable
inactive caches. CPU cleanup stays best effort and explicitly reports residue.

## Required before enabling or merging

Focused ShellSpec fixtures cover filesystem ownership, modes, symlinks,
interruption, refresh failures, live eligibility, dispatch and repair decisions.
They do not substitute for:

- real build-only inventory verification and boot/initrd inspection on both
  supported Ubuntu versions;
- managed CUDA provisioning on both dispatch paths and on PIS;
- CPU, live opt-out and GRID nonactivation/legacy-image compatibility;
- security-image kernel-update capture, normal-installer repair, reboot, DKMS
  registration, driver/device health and measured latency.

These VHD/GPU/kernel-update end-to-end tests are not performed by the unit suite.

Public producer references:
[aks-gpu build and initialization](https://github.com/Azure/aks-gpu/blob/af17acc5c58f0f24aecbc1775f107596b5ff292a/install.sh),
[runfile inventory format](https://github.com/NVIDIA/nvidia-installer/blob/main/backup.c),
[runfile entry tags](https://github.com/NVIDIA/nvidia-installer/blob/main/backup.h).
