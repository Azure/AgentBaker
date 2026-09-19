# Dedicated AMD GPU VHD

The `2404gen2amdgpucontainerd` image is an opt-in Ubuntu 24.04, x86-64,
Generation 2 image for `Standard_ND96isr_MI300X_v5` and
`Standard_ND96is_MI300X_v5`. Its build flag is `AMD_GPU` and its pipeline
artifact is `2404-amdgpu-gen2-containerd`. FIPS, Trusted Launch, confidential
VMs, ARM64, and combinations with NVIDIA image flags are rejected.

## Host and container boundary

The host contains the official AMDGPU DKMS driver, firmware, AMD SMI diagnostics,
matching kernel headers, and build prerequisites. Exact AMD package versions,
module version, repository, and signing-key fingerprint live in
`vhdbuilder/packer/amd-gpu-components.json`, validated separately by
`schemas/amd-gpu-components.cue`. The ordinary component manifest is unchanged.
The build authenticates AMD repository metadata, installs only the selected
driver, firmware, SMI and SMI sysdeps packages, builds the module for the image
kernel, and removes its temporary repository and package-download state. It writes
`/opt/azure/amd-gpu/driver.json` only after successful verification.

The ROCm compute runtime, HIP/RCCL, PyTorch, math libraries, compilers for AI workloads, and heavier
tools such as `rocminfo` belong in containers. Workloads use the normal `runc`
container runtime; the AMD Kubernetes device plugin advertises `amd.com/gpu` and supplies access to
the kernel devices. No NVIDIA container runtime is required. The AMD image
does not cache NVIDIA driver or plugin payloads.

Host operators can run `amd-smi` directly through `/usr/local/bin/amd-smi`.
The image pins `amdrocm-amdsmi10.0` and `amdrocm-sysdeps10.0`, both `10.0.0-4`,
in `AMDGPUDiagnostics`. The installer verifies the separate AMD ROCm repository
key and signed metadata, then installs just these packages with Ubuntu Python
and C++ runtime dependencies. It does not install the ROCm compute runtime,
SDK, or Python packages from pip. `pciutils` (`lspci`) and `numactl` are retained
for PCI and NUMA diagnosis.

Read-only examples on a GPU node (the default SSH user needs `sudo` for AMD
SMI hardware access):

```bash
sudo amd-smi version
sudo amd-smi list --json
sudo amd-smi static --json
sudo amd-smi metric --json
lspci -nn
numactl --hardware
```

AMD SMI initializes hardware even for `--help`, so CPU VHD builds validate its
Python bindings and management library without initializing GPUs. GPU E2E
checks require its CLI to discover eight GPUs matching kernel PCI addresses.
Some telemetry fields are unavailable (`N/A`) on MI300X virtual functions.

Keep DKMS sources, the host compiler, and matching headers installed: kernel
updates need them to rebuild the driver. Do not delete these files just to
reduce the image size. A future, smaller alternative is an AKS-built signed
module package for each exact supported kernel ABI. That requires coordinated
kernel/module updates and qualification; a module copied from a different
kernel is not sufficient.

## Measured package footprint

Official package metadata was measured on an Ubuntu 24.04 MI300X test VM on
2026-09-19. Decimal MB/GB are used. Installed sizes below are package metadata,
not compressed VHD sizes.

| Payload | Download | Installed |
| --- | ---: | ---: |
| AMDGPU 31.50 driver and firmware | 29.84 MB | 671.67 MB |
| ROCm 10 gfx942 SDK plus runtime development package closure | 2.198 GB | 9.626 GB |
| Included AMD SMI and its AMD sysdeps package | 18.18 MB | 100.38 MB |

The driver also requires Ubuntu build/kernel dependencies and creates module
and initramfs files. The measured driver/build dependency closure was 1.287 GB,
including packages already present in Ubuntu; this is not an incremental VHD
size estimate. The full SDK package set is 9.626 GB; the host includes only
its 100.38 MB AMD SMI diagnostics subset.
Driver, firmware and SMI together account for 772.05 MB of declared AMD package
contents, before Ubuntu dependencies and generated module/initramfs files.
Even ROCm 10's base/runtime packages pull LLVM; they are unnecessary for AMD SMI.

The full ROCm PyTorch development image tested separately has 20.455 GB of
compressed OCI layers. Keeping that image out of the VHD does not make its
first pull smaller. For production workloads, use a separately qualified
runtime image containing only the required framework/runtime libraries and
GPU architecture, and keep compilers and build caches in a separate build
stage. Measure that image's first-pull time and digest before adopting it.

## Provisioning and image selection

Legacy provisioning sets `NodeBootstrappingConfiguration.EnableAMDGPU=true`;
ANC sets `GpuConfig.enable_amd_gpu=true`. Leave NVIDIA enablement false. AMD
driver configuration also requires `ConfigGPUDriverIfNeeded=true` (ANC:
`GpuConfig.config_gpu_driver=true`), preserving an explicit driver opt-out.
Validation runs in `nodePrep`, including when a PIS image skips `basePrep`.
It verifies installed package versions, the DKMS module for the running kernel,
the loaded module version, `/dev/kfd`, and eight active KFD GPUs. Preallocated
DRM render nodes are not counted as GPUs. Missing or incompatible drivers fail
provisioning; there is no vendor download or compile fallback on node startup.

Build this flavor with the separate manual pipeline
`.pipelines/.vsts-vhd-builder-amd.yaml` (`trigger: none`, `pr: none`). Register it
with the existing nonproduction VHD builder pool, service connection, and
build-environment variables. It captures only `2404gen2amdgpucontainerd` and
replicates to France Central by default; it does not publish a production image.
The normal PR and release build matrices are unchanged and have no dependency
on the AMD pipeline. A failed AMD bake fails only that separate run.

AMD installers, package metadata, content checks, and bootstrap validation live
in AMD-specific files. Shared scripts load them only behind `AMD_GPU` or
`AMD_GPU_NODE` guards. Node provisioning sources the validator baked into this
image, so normal VHDs do not execute or depend on AMD package installation.
Existing CPU/NVIDIA SKU naming and image-selection defaults remain unchanged.

Use the captured AMD gallery image explicitly for testing. Its existing Ubuntu
24.04 Gen2 `Distro` describes OS behavior; its distinct gallery image name
selects the driver payload. Do not point the production SIG map at an image
before that image has been published and qualified.

The AMD flags configure bootstrap validation; they do not select an image.
`GetNodeBootstrapping` and `GetLatestSigImageConfig` still return ordinary Ubuntu
image metadata for the existing `Distro`. An opt-in caller must set the VM or
VMSS `ImageReference.ID` to the captured AMD gallery image version explicitly,
as the E2E scenario does, instead of using the returned ordinary
`SigImageConfig` for image selection.

Native AKS node-pool availability also requires service-side work: permit the
supported AMD SKUs/driver policy, select this separate gallery image, propagate
AMD enablement to both provisioning contracts, and arrange the AMD device
plugin. This AgentBaker change alone does not enable a public AKS SKU or change
the AKS resource provider's existing AMD driver-policy validation.

## Qualification

On 2026-09-19, dedicated AMD build `181874913`
successfully captured and tested `2404gen2amdgpucontainerd/1.1789808022.4162`
and completed France Central replication. Its image inputs match PR commit
`3ce6d173`; validation commit `72e925a9` changes only the pipeline entrypoint.
The E2E runner includes the AMD test invocation fix in `4de7f56d`; that change
does not alter the captured VHD payload.

A fresh MI300X node booted this exact image, joined AKS, and passed
`Ubuntu2404_MI300X_AMDGPU` with kernel `6.8.0-1067-azure` and AMDGPU module
`7.1.3.31500000`. AMD SMI and the device plugin discovered eight GPUs. Using
the [scenario's digest-pinned images](../e2e/scenario/scenario_gpu_amd.go),
PyTorch 2.13.0 / ROCm 10.0 completed 40 FP32 training steps per GPU, checking
outputs, loss, gradients, and updated parameters against a CPU reference.

Additional checks passed before and after restarting that same instance. A
changed boot ID and node readiness were required before the second round:

- Eight-rank all-to-all transfers checked every received element at
  64/80/128/256 MiB per peer: 42,631 measured calls across both rounds, 291.16 TB
  received including 254.77 TB between GPUs, and zero mismatched elements.
  Independent calculation verified complete timing samples and byte totals.
- A 49,016,832-parameter Transformer completed 100 BF16 DDP/AdamW steps across
  eight GPUs in each round, processing 1,638,400 synthetic tokens per round.
  Mean loss decreased from 6.519 to 0.000249; losses, gradients, and weights
  remained finite, and all eight ranks produced the same full-model SHA-256.
- The 40-step CPU-reference training check passed again on all eight GPUs
  after reboot.
- Host driver, AMD SMI, and runtime health checks passed before and after these
  workloads; the host had no ROCm compute SDK installed.

Remote-send bandwidth per GPU (decimal GB/s):

| MiB per peer | Median before reboot | Median after reboot | Effective before | Effective after |
| --- | ---: | ---: | ---: | ---: |
| 64 | 278.7 | 278.8 | 269.9 | 272.6 |
| 80 | 281.9 | 282.0 | 281.2 | 277.7 |
| 128 | 286.5 | 286.5 | 286.3 | 286.3 |
| 256 | 291.1 | 291.4 | 289.6 | 290.5 |

Rates use decimal GB/s and seven remote sends per GPU divided by the slowest
rank's call time, without send/receive double-counting. Effective rates include
all timed calls, including outliers; setup and data comparison are outside the
timed calls. These bounded tests do not establish peak throughput, model
quality, or integrity of untested transfers and network/storage paths.

After capturing the image and replicating it to `francecentral`, use the
repository's normal E2E Azure configuration and its captured build metadata:

The test nodes need outbound HTTPS access to the pinned images on Docker Hub.
The standard E2E firewall does not include this access. For an isolated AMD lab,
scope an additional rule to its GPU nodes for `registry-1.docker.io`,
`auth.docker.io`, and the image-layer CDN. The current image digests were served
by `production.cloudfront.docker.com`; recheck CDN destinations when updating
them. Remove temporary rules with the lab resources. A recurring AMD pipeline
can instead mirror the exact digests into an approved reachable registry and
verify the copied manifests and layers before use.

```bash
cd e2e
AGENTBAKER_E2E_ENABLE_MI300X=true ./e2e-local.sh \
  --subscription-id "$SUBSCRIPTION_ID" \
  --vm-sku Standard_ND96isr_MI300X_v5 \
  --vhd-metadata-file /absolute/path/to/vhd-build-metadata.json \
  --parallel 1 --disable-scriptless=false \
  --disable-scriptless-compilation=true \
  --ignore-missing-vhd=false --skip-capacity-errors=false \
  --tags '' --skip-tags '' \
  Ubuntu2404_MI300X_AMDGPU
```

The metadata must contain `2404gen2amdgpucontainerd`, its real image-version
resource ID, and the replication region. This scenario allocates MI300X
capacity and is skipped unless explicitly enabled. The existing `--vm-sku`
option sets the tested node size before capability queries; the scenario keeps
the shared AKS system pool on the ordinary CPU SKU. It validates the driver,
deploys the digest-pinned AMD device plugin, requires eight advertised GPUs,
and requires completed reference-checked training. A managed 256 GiB OS disk
provides room to unpack the development workload image. The scenario does not
yet automate reboot, serviced-kernel, or large all-to-all qualification.

The command above uses scriptless NBC provisioning with the scripts and ANC
binary baked into the image. It avoids embedding the full legacy script payload
in Azure custom data, which can exceed Azure's size limit. This path still
executes an NBC command; it does not independently qualify native ANC JSON
provisioning. AMD ANC environment generation is covered by parser tests, but
native ANC boot requires separate qualification.

AgentBaker's Ubuntu 24.04 build currently uses the 6.8 Azure LTS kernel policy.
Native ANC JSON and legacy provisioning, PIS, and a serviced-kernel update with
DKMS rebuild and subsequent GPU workloads require separate qualification.
Record the image version, kernel, driver,
workload image digests, and results for each path.

Production release additionally requires vendor driver security qualification.
A successful workload run is not evidence that all driver security issues are
resolved. Secure Boot support needs a separately qualified signing and trust
chain; this initial flavor does not enable it.
