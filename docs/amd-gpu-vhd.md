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
`parts/common/components.json`.
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

Read-only examples on a GPU node:

```bash
amd-smi version
amd-smi list --json
amd-smi static --json
amd-smi metric --json
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

Build this flavor through the AMD job in `.pipelines/.vsts-vhd-builder.yaml`,
or enable `build2404amdgpugen2containerd` in the release pipeline. The release
parameter defaults to false. Existing CPU/NVIDIA image identities and
production image-selection defaults remain unchanged.

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

Local repository generation, Go tests, lint, and focused ShellSpec checks
passed. The actual installer also compiled the pinned module for
`6.8.0-1067-azure` in an isolated Ubuntu 24.04 CPU container. An idempotent
reinstall, APT autoremove, and the actual VHD driver content check passed;
the installed module and DKMS rebuild dependencies survived cleanup. The
container test supplied the target kernel to the installer; it did not boot
that kernel or build/capture a complete VHD.

The AMD SMI addition also passed an actual minimal Ubuntu 24.04 installation,
repeat installation, APT autoremove, and the diagnostics content test with
`lspci`/`numactl`. Only the two selected AMD diagnostics packages were installed;
its Python binding and library loaded without GPU hardware or the compute SDK.
On the existing MI300X lab, `amd-smi` version, discovery, static information and
metrics succeeded. The new `/usr/local/bin/amd-smi` symlink was also tested with
a clean environment and detected eight GPUs.

The existing MI300X lab used AMDGPU 31.50 with ROCm 10 containers on kernel
6.17.0-1022-azure. Its eight-GPU training and exact all-to-all integrity tests
passed. Those results establish this host/container combination; they do not
establish that a newly baked AgentBaker image has booted successfully.
The new training fixture also passed 40 steps on each of eight GPUs, comparing
outputs, losses, gradients, and updated parameters with a CPU reference.

After capturing the image and replicating it to `francecentral`, use the
repository's normal E2E Azure configuration and its captured build metadata:

```bash
cd e2e
AGENTBAKER_E2E_ENABLE_MI300X=true ./e2e-local.sh \
  --subscription-id "$SUBSCRIPTION_ID" \
  --vhd-metadata-file /absolute/path/to/vhd-build-metadata.json \
  --parallel 1 --disable-scriptless \
  Ubuntu2404_MI300X_AMDGPU
```

The metadata must contain `2404gen2amdgpucontainerd`, its real image-version
resource ID, and the replication region. This scenario allocates MI300X
capacity and is skipped unless explicitly enabled. It validates the driver,
deploys the digest-pinned AMD device plugin, requires eight advertised GPUs,
and requires completed reference-checked training. A managed 256 GiB OS disk
provides room to unpack the development workload image. The scenario does not
yet automate reboot, serviced-kernel, or large all-to-all qualification.

The command above exercises legacy NBC provisioning. The current E2E runner's
scriptless path still executes an NBC command; it does not independently
qualify native ANC JSON provisioning. AMD ANC environment generation is
covered by parser tests, but native ANC boot requires separate qualification.

AgentBaker's Ubuntu 24.04 build currently uses the 6.8 Azure LTS kernel policy.
Require a successful build and boot of the captured image on MI300X, including
node readiness, eight advertised GPUs, AI/reference checks, large all-to-all
integrity/bandwidth checks, reboot, and kernel-update/rebuild validation. Run
both legacy and ANC provisioning, and PIS validation before supporting PIS.
Record the image ID, kernel, driver, workload image digests, and results.

Production release additionally requires vendor driver security qualification.
A successful workload run is not evidence that all driver security issues are
resolved. Secure Boot support needs a separately qualified signing and trust
chain; this initial flavor does not enable it.
