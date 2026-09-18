# Parked NVIDIA DKMS prebakes

The Ubuntu x86-64 CUDA prebake keeps installed kernel modules and userspace in
their existing locations. After `build-only` succeeds, AgentBaker moves only
`/var/lib/dkms/nvidia` to `/opt/azure/aks-gpu/dkms/nvidia`. The default DKMS tree
is unchanged; image-time kernel maintenance must not discover this registration.

## Compatibility and rollout

Parking requires **both** `NVIDIA_CUDA_PREBAKE` and `NVIDIA_DKMS_PARK` in
`FEATURE_FLAGS`. Use `NVIDIA_CUDA_PREBAKE,NVIDIA_DKMS_PARK`, without spaces:
the existing Packer command transport does not quote this value. The second
flag must only be enabled after every CSE consumer of the image has the
restoration logic in `ensureGPUDrivers`. Existing pipeline definitions do not
enable it. With only `NVIDIA_CUDA_PREBAKE`, the existing registered prebake
remains unchanged, including its installed modules and userspace; current
non-GPU cleanup is still required. Parking does not disable existing prebaking.

A repeated build that already carries the parked layout must retain the parking
opt-in; removing it fails the build rather than silently converting that image
back to an active registration.

Deploy CSE first, then enable parked-image builds and validate their consumers.
New CSE preserves old-image behavior when no parked layout is present. **Old CSE
on a parked image is not supported**: ordinary installation may re-register the
driver, but validation-only provisioning can load an existing `.ko` and report
success without reactivating DKMS. A marker alone cannot teach old CSE to restore.
Roll back the image selection before rolling CSE back to a version without
restoration support. Include PIS-cached images in rollout and rollback validation.

## Lifecycle and failure handling

The existing `dkms-marker` key/value format is preserved, adding
`dkms_registration=parked-v1`. This field identifies the image layout, not a
skip-build instruction or a current-state flag. It remains after restoration so
a retry can distinguish a valid active registration from missing state. Ordinary
driver installation can replace the marker using the installer's existing format.
No marker-provided path is used for moves or removal.

On a managed CUDA node, `nodePrep` resolves the live opt-out decision before
calling `ensureGPUDrivers`. Restoration precedes both installation and
validation, including on PIS nodes where `basePrep` is skipped. CPU, opt-out,
ARM64, and non-Ubuntu paths do not activate the parked tree. GRID nodes instead
run the existing CUDA-prebake cleanup, extended to remove parked state.

Moves refuse to merge, nest, or replace two independently existing trees.
Malformed layout metadata, missing driver sources, missing expected trees, and
failed moves fail managed-GPU provisioning before a validation shortcut can
mask the missing registration. A failed park fails the image build. Completed
moves and repeated builds are retryable; collisions require investigation rather
than destructive replacement.

Restoration does **not** produce modules for another kernel or driver version.
The normal install command is unchanged, and validation retains its existing
module-load failure fallback to installation. This change does not implement
skip-build or redesign driver-version mismatch handling.

CPU/opt-out and GRID cleanup still handles active registrations from legacy
images, and now also removes the fixed parked cache. Incomplete cleanup retains
the marker and reports `parked_after=true` when applicable. If both trees exist,
cleanup preserves them and reports incomplete rather than deleting a potentially
independent installation. GRID provisioning fails before installing or validating
over that unresolved state; CPU/opt-out cleanup remains best-effort.
Parking alone does
not remove installed `.ko` files, userspace, setuid binaries, or already loaded
modules; the existing cleanup limitations still apply.
