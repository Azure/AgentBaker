The workflow becomes complicated as we add more VHDs. Here is a quick description of the current VHD build workflow:

# gen1
MarketplaceImage->(func/packer) -> VHD in classic SA
## gen1 mariner:
external-vhd->(func/init:internal-vhd)->(func/packer) -> VHD in classic SA

# sig:
MarketplaceImage->(func/init: ensure dest gallery/definition)->(func/packer: managed image)->dest SIG

# gen2:
MarketplaceImage->(func/init: ensure dest existing/static gallery/definition)->(func/packer: managed image)->dest SIG->(func/convert: sig->disk) -> VHD in classic SA

## gen2 mariner:
external-vhd-src->(func/init: ensure dest existing/static gallery/definition, internal-vhd->image; )->source SIG->(func/packer: managed image)->dest SIG->(func/convert: sig->disk) -> VHD in classic SA

# Roadmap
Goal1: remove mariner workflow so things will be simplified.

# Shared Ubuntu GPU installer cache

The non-FIPS Ubuntu 22.04/24.04 amd64 Gen2 shared images cache the CUDA-LTS
installer container without installing its driver into the host. Their build and
release defaults use `FEATURE_FLAGS=None`, not `NVIDIA_CUDA_PREBAKE`. Managed GPU
nodes use the ordinary node-time installer; CPU and GPU-opt-out nodes must not
depend on CSE cleanup to remove an image's NVIDIA DKMS registration.

The build retains gcc, make, libc6-dev and the nouveau blacklist in all initramfs
images independently of host prebaking. Content validation checks the booted
kernel's headers, installer cache, boot configuration and absence of NVIDIA
registration, modules and known host artifacts, including dangling registration
links and residue without a marker. The guard uses the shared-image identity,
not the disabled feature flag. It fails the build rather than cleaning up a
contaminated image. Stock-kernel framebuffer and WMI backlight modules are not
CUDA/GRID payload; the backlight exception applies only under
`kernel/drivers/platform/x86`, on disk and in initramfs. Copies under DKMS paths
and actual NVIDIA GPU modules remain failures. ARM64/Grace-Blackwell, FIPS, CVM
and other OS images are outside this policy.

Release the missing-driver validation fast fallback before broad image rollout
where validation-only callers would otherwise spend 115 seconds retrying an
absent `nvidia-modprobe`. Older CSE retains its ordinary installation fallback,
but its provisioning budget still needs verification. Retain legacy cleanup and
PIS caching; do not move driver activation from `nodePrep` into `basePrep`.

Before promotion, exercise changed Ubuntu images on CPU, GPU-opt-out, CUDA and
GRID nodes, distinct-VM PIS, and security-patched derivatives. Check kernel-update
hooks, GPU workloads, cache availability and provisioning latency. New source
does not repair existing registered bases, derived images, PIS caches or nodes;
refresh those through the supported image/node lifecycle. Re-enabling active
shared-image prebaking is not a safe rollback.

# Linux CSE configuration modules

`parts/linux/cloud-init/artifacts/cse_config.sh` is installed as
`/opt/azure/containers/provision_configs.sh`. Its modules use the same runtime naming:

- `cse_config_gpu.sh` -> `provision_configs_gpu.sh` (NVIDIA and AMD)
- `cse_config_localdns.sh` -> `provision_configs_localdns.sh`
- `cse_config_kubelet.sh` -> `provision_configs_kubelet.sh`
- `cse_config_network.sh` -> `provision_configs_network.sh`
- `cse_config_addons.sh` -> `provision_configs_addons.sh` (autoscaler, ACI connector, Azure Policy)

`cse_cmd.sh` and the ANC parser provide their paths through
`CSE_CONFIG_GPU_FILEPATH`, `CSE_CONFIG_LOCALDNS_FILEPATH`,
`CSE_CONFIG_KUBELET_FILEPATH`, `CSE_CONFIG_NETWORK_FILEPATH`, and
`CSE_CONFIG_ADDONS_FILEPATH`; the parent sources each explicitly.
If a variable is unset or empty, its path defaults to the corresponding sibling
of the sourced parent (`provision_configs_*.sh` on-node, `cse_config_*.sh` in the
source tree). Explicit paths take precedence.

Always package all modules, including images without GPU or LocalDNS enabled, with
the same `0744` permissions as the parent. Each Linux Packer JSON template that
copies the parent must upload the modules to `/home/packer/`; `copyPackerFiles`
in `packer_source.sh` installs them in `/opt/azure/containers/`. OSGuard's
`imagecustomizer/azlosguard/azlosguard.yml` installs them directly.

Traditional CustomData delivers the same files through
`parts/linux/cloud-init/nodecustomdata.yml` and `pkg/agent`'s source constants,
variables and template functions. Flatcar and ACL reuse those entries through
the Ignition archive conversion. Scriptless CustomData relies on the baked files
unless a hotfix explicitly selects an override.

When adding a module, also register its source-to-variable mapping in
`hotfix/hotfix_generate.py`. The current keys are `provisionConfigsGPU`,
`provisionConfigsLocalDNS`, `provisionConfigsKubelet`, `provisionConfigsNetwork`,
and `provisionConfigsAddons`. Each module can be
hotfixed independently; delivering this split to an older VHD requires the
updated parent and all new modules together. Keep the immutable VHD baseline so
subsequent hotfix payloads remain cumulative.
