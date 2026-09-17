# Ubuntu CUDA prebake lifecycle

`NVIDIA_CUDA_PREBAKE` builds the CUDA driver without a GPU. Before image capture,
AgentBaker parks the whole prebake under `/opt/azure/aks-gpu/prebake` (mode `0700`,
owned by root). The driver source stays under `/usr/src`.

This directory is a private cache, not an installation. CPU nodes, GPU nodes with
driver installation disabled, and GRID nodes do not restore it. Failure to remove
an intact parked cache cannot register NVIDIA with DKMS or expose its binaries on
the normal system paths. The root-only parent also prevents an unprivileged user
from executing parked setuid binaries by their absolute paths.

## Files and ownership

`prebakedGPUDriverFiles` uses NVIDIA's `/var/lib/nvidia/log` inventory for installed
files and symlinks. It also includes the library directory staged by aks-gpu,
library configuration, installer metadata, and NVIDIA modules installed by DKMS.
It does not move shared parent directories or modules from other drivers.

The installer writes libraries through an overlay on
`/usr/lib/<arch>-linux-gnu`; aks-gpu moves the overlay output to `/usr/bin/lib64`.
The inventory's original library paths must **not** be moved: they refer to the
base OS again after the overlay is removed. A replacement of an existing host
file outside that overlay is rejected at build time and requires investigation.

`files.list` records each owned path and its inode/file mode. Same-filesystem
renames preserve this identity. A retry accepts an already-restored destination
only when it retains that identity; a missing or conflicting file fails setup.
The payload must remain root-owned and private. It is not a portable tar archive:
file-based repackaging that changes inode numbers is not supported.

The park operation regenerates dependency indexes for the affected kernels and
refreshes the linker cache. It audits all bootable initramfs images; if one contains
a prebaked NVIDIA module, it rebuilds that initramfs and checks it again. It also checks the effective
`dkms status`; a redirect of `dkms_tree` into the parked cache fails the VHD build.
`park.complete` is published only after these checks succeed.

## Node provisioning

`ensureGPUDrivers` calls `setPrebakedGPUDriverState restore` only for managed CUDA
nodes, from `nodePrep`. PIS preprovisioning must not activate the payload.

- Matching kernel, architecture and driver version: restore the files, with the
  DKMS tree last. Run `depmod` for the affected kernels and `ldconfig`, then remove
  the private payload and parked-layout marker field.
- Kernel, architecture or driver-version mismatch: discard the inactive cache,
  emit `event=cache_miss`, and require the existing normal driver installer. This
  also applies when the original request would only validate a driver. Neither a
  rename nor `depmod` can supply a module for a different kernel.
- Partial restoration: retry using the file identities in the manifest. A change
  of kernel/driver during a partial restore fails explicitly; it cannot discard
  the cache as if nothing had been activated.
- GRID: discard an intact whole CUDA cache without touching a newly installed
  GRID driver. Retain legacy CUDA teardown for older VHDs. A partial restore or
  failed cache deletion stops GRID setup.
- CPU/driver opt-out: discard an intact cache without module unloads or linker
  updates. Cleanup remains best effort and reports any incomplete operation.

Restoration does not call `dkms install`. This change does not enable
`install-skip-build`: the normal aks-gpu installer remains responsible for driver
installation, including compilation when required. Source under `/usr/src` can
still be registered deliberately; this change prevents automatic activation of
the prebake, not privileged driver installation.

## Compatibility and rollout

Deploy compatible CSE before publishing the new VHD layout. The consumer accepts
legacy active prebakes, registration-only parking, and whole-payload parking.
Keep the legacy cleanup functions until old image support ends. Existing nodes
and already-published VHDs are not repaired by this change.

The `AKS_GPU_PREBAKE` messages remain node-local. Fleet telemetry ingestion is a
separate change; this implementation does not claim to make cleanup measurable
across the fleet.

## Release validation

Use VHDs built from this PR, selected through `VHD_BUILD_ID`, not images selected
by the default `branch=refs/heads/main` tag. Test Ubuntu 22.04 and 24.04:

| Case | Required result |
| --- | --- |
| Image before CSE | No active NVIDIA registration, modules or installer-owned userspace; no prebaked NVIDIA module in initramfs |
| Managed matching CUDA | Restore succeeds; GPU workloads run; later kernel update and reboot retain a working driver |
| Security-patched kernel mismatch | Normal installer is selected explicitly and builds for the running kernel |
| GRID, including GRID v20 | CUDA remains inactive; GRID installation and workload validation pass |
| CPU and driver opt-out | NVIDIA stays inactive even if cache deletion is skipped or fails; a kernel update does not compile NVIDIA |
| PIS | Payload stays parked through preprovision/capture and is restored only on the real managed CUDA node |
| Interrupted restore | Retry completes safely, or a changed destination produces a clear failure without overwriting it |
| Legacy images | Existing installation and cleanup paths still work |

Measure build duration and total node provisioning duration against the current
prebaked VHD, including compile activity and cold-cache runs. Measure `depmod` and
`ldconfig` separately to explain any change. No performance threshold or live-GPU
result is implied by the unit tests.
