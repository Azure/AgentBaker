# NVIDIA prebakes without image-time DKMS registration

The shared Ubuntu x86-64 VHD contains the compiled CUDA module and libraries,
but no NVIDIA registration in `/var/lib/dkms`. `aks-gpu`
`build-only` uses `--no-dkms` and installs modules under
`/lib/modules/<target-kernel>/updates/dkms`, the existing CPU/GRID cleanup path.
Both the build step and the final VHD content test reject active registration.
An older build container that still registers NVIDIA fails the build.
The companion container change is [Azure/aks-gpu#191](https://github.com/Azure/aks-gpu/pull/191).

CPU and opted-out nodes do not invoke the GPU installer. Even if artifact cleanup
fails, normal DKMS kernel-update discovery has no NVIDIA registration to rebuild.
This does not remove all driver files, prevent module loading, or block an
explicit administrator request to add/install the remaining NVIDIA sources.
Existing registered images and PIS caches still need their existing cleanup or
replacement; changing the image build does not repair deployed nodes.

## Managed CUDA node flow

`nodePrep` resolves the live GPU opt-out setting, including on PIS nodes that skip
`basePrep`. For a marked CUDA prebake, CSE checks whether DKMS records the on-disk
driver as installed for the running kernel and architecture. A loadable module,
an `added` record, or a `built` record is not enough.

If installation is requested, or the prebake has no complete DKMS installation,
CSE runs the existing container `install` action. The normal NVIDIA installer
uses `--dkms`; it compiles, installs, and registers the driver. Device and
container-runtime setup remain unchanged. If validation-only mode already has
a complete installation, CSE keeps the existing validation path.

CSE checks the installed state after either path. An incomplete installation or
a DKMS module mismatch fails provisioning with the existing GPU error code.
The check permits DKMS's harmless `original_module exists` backup notice.
GRID, non-Ubuntu, ARM64, and unmarked validation paths keep their existing behavior.

There is an existing GRID cleanup limit. Cleanup can return success while leaving
CUDA files in place. In validation-only mode, if the remaining driver passes
`nvidia-modprobe` and `nvidia-smi`, CSE can finish without running the GRID installer.
This change does not fix that path. A mocked control-flow test shows the path is
possible; it has not been verified on a real GRID node.

This is a safety-only change. It does not use `dkms add`, move DKMS caches, or
enable `install-skip-build`. A fresh unregistered CUDA image requires a full
installation, even in validation-only mode. This can remove the startup-time
saving from validating an already-loadable prebake. No startup-time measurement
has been made. Nodes with a complete DKMS installation can still validate it. The
container's unused skip-build mode is unchanged and needs separate work before
AgentBaker can enable it for these unregistered images.

## Release order and validation

1. Ship the updated CSE to all consumers first, including validation-only and
   PIS paths. The new unregistered VHD needs this check on those paths. Runtime
   containers can keep their existing normal `install --dkms` behavior.
2. Publish the updated `aks-gpu` build image and update its builder reference, then
   enable these builder checks and publish the new VHDs. This source change does
   not invent a new image tag or publish an image. The existing
   `NVIDIA_CUDA_PREBAKE` flag is retained; no parking/layout flag is introduced.
3. Rebuild PIS caches from compatible images. For rollback, restore compatible
   VHD/PIS image selection before rolling CSE back.

Before release, test actual VHD builds on Ubuntu 22.04 and 24.04; CPU security
patching with intentionally failed cleanup; managed CUDA install and
validation-only/PIS paths; kernel/driver mismatches; GRID and opt-out paths; and
GPU kernel update plus reboot. Unit tests cannot establish hardware correctness.

Local tests cover the installer arguments, no-registration build checks, normal
CUDA dispatch, PIS gates, and rejection of incomplete DKMS states. The separate
`gpu_prebake_dkms_spec.sh` uses actual DKMS with fixture source/module trees to
check status parsing and kernel-update discovery. It requires root and DKMS in
a disposable Linux test environment; it does not compile a real NVIDIA driver.
