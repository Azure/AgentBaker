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

# Linux build steps

The pipeline's `Build Go binaries` step runs `make -f packer.mk -j4 build-tools`
to install the latest supported Microsoft Go toolchain, generate the prefetch
script, and build the target-architecture binaries. The following VHD step runs
in the same job and reuses those files. For local builds, run `build-tools` before
`run-packer` or `run-imagecustomizer`.

# Ubuntu BCC build

Ubuntu BCC remains pinned to v0.29.0, fetched with a shallow tag clone, and built
with at most eight parallel make jobs. Check build-VM peak memory before raising
this limit, since BCC installation overlaps container-image caching.

# MGLRU default

Ubuntu, Mariner/Azure Linux (including OSGuard), and Azure Container Linux (ACL)
images install `/etc/tmpfiles.d/aks-mglru.conf` to disable Multi-Gen LRU at each boot.
`systemd-tmpfiles-setup.service` applies the rule during system initialization,
before kubelet starts. The boot-only `w!` rule writes `0` to
`/sys/kernel/mm/lru_gen/enabled` if it exists; kernels without MGLRU are skipped.
Installation is gated by OS family, not OS or build-kernel version. The rule is
retained even when the build kernel lacks MGLRU, since the image may boot a
different supported kernel later. On the current Ubuntu 20.04/22.04 and
Mariner/Azure Linux 2 kernels without MGLRU, it is a no-op; if a later kernel
introduces the interface, the same rule disables it at boot.

This preserves traditional reclaim behavior in response to the kubelet
memory-accounting regression reported in
[kubernetes/kubernetes#127844](https://github.com/kubernetes/kubernetes/issues/127844).
It is an image default, not a new Custom Node Configuration API.
Flatcar images remain excluded. ACL AMD64 and ARM64 Packer templates upload the
same rule; the generated ACL CVM template inherits it from the AMD64 template.
ACL stores the rule under writable `/etc`, without modifying immutable `/usr`.
The rule works independently of script-based/ANC provisioning and PIS base-prep markers.
Existing nodes need a node-image upgrade to receive it; changing CSE alone does
not retrofit the rule onto an older VHD.

The Linux VHD content test checks both the installed rule and the effective
disabled state after boot (without applying the rule in the test). Roll out new
images through the normal canary stages and validate memory-sensitive workloads,
reclaim CPU, latency, memory pressure, and evictions before broad deployment.

# Linux CSE configuration modules

`parts/linux/cloud-init/artifacts/cse_config.sh` is installed as
`/opt/azure/containers/provision_configs.sh`. Its modules use the same runtime naming:

- `cse_config_gpu.sh` -> `provision_configs_gpu.sh` (NVIDIA and AMD)
- `cse_config_localdns.sh` -> `provision_configs_localdns.sh`
- `cse_config_kubelet.sh` -> `provision_configs_kubelet.sh`
- `cse_config_network.sh` -> `provision_configs_network.sh`
- `cse_config_addons.sh` -> `provision_configs_addons.sh` (autoscaler, ACI connector, Azure Policy)
- `cse_config_chrony.sh` -> `provision_configs_chrony.sh`

`cse_cmd.sh` and the ANC parser provide their paths through
`CSE_CONFIG_GPU_FILEPATH`, `CSE_CONFIG_LOCALDNS_FILEPATH`,
`CSE_CONFIG_KUBELET_FILEPATH`, `CSE_CONFIG_NETWORK_FILEPATH`, and
`CSE_CONFIG_ADDONS_FILEPATH`, and `CSE_CONFIG_CHRONY_FILEPATH`; the parent sources each explicitly.
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
`provisionConfigsAddons`, and `provisionConfigsChrony`. Each module can be
hotfixed independently; delivering this split to an older VHD requires the
updated parent and all new modules together. Keep the immutable VHD baseline so
subsequent hotfix payloads remain cumulative.
