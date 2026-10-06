# POC: Azure Local edge node-image variant (AKS Arc "VM Everywhere")

This directory is a **proof-of-concept** that makes AgentBaker's existing
Image Customizer (MIC) pipeline emit an **on-prem Azure Local edge VHD** in
addition to the cloud AKS node image — the "BEST / Tier-1" option from the
VM-based AKS Arc Everywhere image supply-chain analysis (§9.5).

It is **opt-in and default-off**: the AKS default build (`azlosguard`) is not
modified. Everything here is selected only when you explicitly ask for the
`azurelocal` config.

## What it reuses vs. changes

| Bucket | Source | Shared with AKS? |
|--------|--------|------------------|
| Build tool | Image Customizer (MIC) | ✅ same |
| Base OS | Azure Linux 3.0 (`BASE_IMG`) | ✅ same |
| System images | MCR `oss/v2/*` via `parts/common/components.json` | ✅ same |
| k8s | PMC packages (kubeadm/kubelet/kubectl) | ✅ same source, different delivery |
| **Bootstrap** | **kubeadm + NoCloud seed** (no CSE/hosted-CP) | ❌ edge-only |
| **Agents** | **edge set** (lbagent/cert-tattoo/fluent-bit) | ❌ edge-only |
| **Distribution** | **SFS** 3-layer catalog (not SIG) | ❌ edge-only |

## The three gated parameters (analysis §9.5)

| Parameter | AKS default | Edge value | Where it lives |
|-----------|-------------|------------|----------------|
| `bootstrapMode` | `cse` | `kubeadm-nocloud` | `azurelocal.yml` postCustomization env `BOOTSTRAP_MODE` |
| `agentSet` | `aks` | `edge` | `azurelocal.yml` postCustomization env `AGENT_SET` |
| `publishTarget` | `sig` | `sfs` | env `PUBLISH_TARGET` → `../scripts/publish-imagecustomizer-sfs.sh` |

`bootstrapMode=kubeadm-nocloud` means the AKS CSE bootstrap chain
(`aks-node-controller`, `provision*.sh`/`cse_*`, `secure-tls-bootstrap`) and the
Azure-fabric units (`block_wireserver`, `configure-azure-network`,
`reconcile-private-hosts`, `kms.service`, `init-aks-cloud.sh`) are **not baked or
enabled**. The node instead boots against a NoCloud (`cidata`) seed ISO produced
by `AKS-InfraX/AKSWinAgent` and is joined by kubeadm. The postinstall asserts
none of those AKS artifacts leaked into the image.

## Files

| File | Purpose |
|------|---------|
| `azurelocal.yml` | MIC config: shared base + cloud-agnostic units; omits AKS CSE/agents; bakes the 3 params |
| `scripts/azurelocal-postinstall.sh` | Runs in the MIC chroot: installs k8s, pre-caches MCR images, adds edge agents, guards against CSE leakage |
| `scripts/install-k8s-pmc.sh` | Installs kubeadm/kubelet/kubectl from PMC into `/opt/bin` |
| `files/90-nocloud-datasource.cfg` | Pins cloud-init to the NoCloud datasource |
| `files/pmc-kubernetes.repo` | PMC kubernetes package repo |
| `../scripts/publish-imagecustomizer-sfs.sh` | Stages the VHD + emits an SFS publish manifest (default-off) |

## Build locally

```bash
export IMG_CUSTOMIZER_CONTAINER=<image-customizer container>   # e.g. mcr.microsoft.com/.../imagecustomizer
export IMG_CUSTOMIZER_VERSION=<tag>
export BASE_IMG=<azure linux 3.0 core vhdx oras ref>
export BASE_IMG_VERSION=<tag>

# Build the edge variant (config=azurelocal, publishTarget=sfs):
make -f packer.mk run-imagecustomizer-edge

# Output: out/azurelocal.vhd  (fixed-size Gen2 VHD)
# SFS staging: out/sfs-stage/azurelocal-<version>.vhd + .sfs.json
```

The default AKS build is unaffected:

```bash
IMG_CUSTOMIZER_CONFIG=azlosguard make -f packer.mk build-imagecustomizer   # unchanged
```

## Local-only: get the VHD without any publishing

The build step writes the image to the local filesystem **before** any publish.
`run-imagecustomizer-edge` runs only `build-imagecustomizer` — it never calls a
publish script, so nothing touches SFS or SIG.

```bash
# Canonical values from the official pipeline
# (.pipelines/templates/.builder-release-template.yaml):
export IMG_CUSTOMIZER_CONTAINER=mcr.microsoft.com/azurelinux/imagecustomizer
export IMG_CUSTOMIZER_VERSION=0.17.0
export BASE_IMG=mcr.microsoft.com/azurelinux/3.0/image/osguard
export BASE_IMG_VERSION=3.0.20260107

make -f packer.mk run-imagecustomizer-edge
# -> ./out/azurelocal.vhd     (fully local; no Azure/SFS calls)
```

Just do **not** run `make -f packer.mk publish-imagecustomizer`.

### Emit a dynamic `.vhdx` instead of fixed `.vhd`

For a quick local Hyper-V boot test, override the output format (default is
`vhd-fixed`, unchanged for the AKS build):

```bash
OUTPUT_IMAGE_FORMAT=vhdx make -f packer.mk run-imagecustomizer-edge
# -> ./out/azurelocal.vhdx
```

`OUTPUT_IMAGE_FORMAT` accepts `vhd-fixed` (default), `vhd`, `vhdx`, `qcow2`, `raw`.
The output extension and the optional SFS staging name follow the format.

### Local build still requires
- **Docker** with `--privileged` (the builder runs the Image Customizer container
  privileged and binds `/dev`). A rootless-only or runtime-less host cannot build.
- An **amd64 host** (the base image `.../3.0/image/osguard` and the baked
  `*-linux-amd64` binaries are x86_64). On arm64 you need qemu/binfmt emulation.
- A one-time **`oras pull`** of `BASE_IMG`. If you already have the base `.vhdx`,
  drop it at `build/azurelocal/image.vhdx` and the pull is skipped.
- Outbound network at build time for the PMC + MCR `oss/v2/*` pre-cache.

## POC scope / known gaps

These are intentionally left as seams (they belong to other teams/repos):

1. **Edge agents** (`agentSet=edge`) are a placeholder — wire in
   lbagent/cert-tattoo/fluent-bit from `aksarc-vhd/AzureLocal`.
2. **SFS publish** stages the VHD + manifest; the signed upload is owned by the
   Aks-Arc-Assembly `sfs-publishing` pipeline.
3. **PMC package names** may differ per channel; `install-k8s-pmc.sh` tries
   versioned then unversioned names. Reconcile with the exact package set
   `aksarc-vhd/AzureLocal` consumes (analysis §7).
4. **Multi-version merged VHD** (6 k8s versions) is not implemented; the POC
   installs a single `K8S_VERSION`.
5. Upstreaming requires **AKS-team ownership + a shared CI lane** so the edge
   path cannot regress the AKS build (analysis §9.5).

## Build via the ADO VHD builder pipeline (build-only)

The `.pipelines/.vsts-vhd-builder-release.yaml` pipeline can build this config on
a real amd64 + privileged-Docker agent (which the local dev box may lack). A
gated, default-off job `buildAzureLocalEdge` runs the Image Customizer build and
publishes the resulting VHD as a **pipeline artifact**, skipping all SIG/SFS
publish, scanning and prefetch steps.

To run it:

1. Queue the pipeline from this feature branch.
2. Set the **`buildAzureLocalEdge`** parameter to `true` (leave the other
   `buildX` SKUs off for a fast, isolated run).
3. The built image is published as the `vhd-azurelocal-edge-*` artifact
   (`out/azurelocal.vhd` plus `out/azurelocal/`).

Under the hood the job sets `IMG_CUSTOMIZER_CONFIG=azurelocal`, reuses the same
OSGuard `BASE_IMG` + Image Customizer version as `azlosguard`, and sets
`BUILD_ONLY=true`. The `BUILD_ONLY` flag is additive in the shared
`.builder-release-template.yaml`: every existing SKU leaves it unset, so their
behaviour is unchanged.
