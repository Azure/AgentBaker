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

# First-boot ext4 root journal sizing

The standard Ubuntu 24.04, Ubuntu 26.04 minimal, and Azure Linux 3 VHDs install
an initramfs hook that runs before the root filesystem is mounted. It grows the
root partition when possible, grows ext4 to the partition size, and compares the
existing internal journal with a sparse ext4 filesystem created by the same
image's `mke2fs` toolchain. If the existing journal is smaller, it recreates the
journal with `tune2fs -j`, which selects that toolchain's default size; larger
journals are preserved.

Completion state is stored on the root filesystem and keyed by its UUID and
the VM's DMI product UUID, avoiding repeated journal changes on normal reboots.
When that state is readable, subsequent boots return before running filesystem
checks or inspecting the journal. If state access fails, the helper verifies
the filesystem before retrying the read.
No completion state is baked into the VHD. PIS support depends on seed and real
nodes having different DMI product UUIDs; verify this on the target image
lifecycle before release. If the helper cannot verify a safe filesystem and
journal, the initramfs refuses to continue to the root mount. The helper is
installed only in the standard image builders; CVM, FIPS, Kata, OS Guard,
encrypted, immutable, and non-target OS variants are excluded.
