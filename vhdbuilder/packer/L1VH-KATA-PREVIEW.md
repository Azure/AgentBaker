# Azure Linux 3 Kata L1VH preview builds

This opt-in release job builds a dedicated L1VH Kata preview image from an approved
private Azure Compute Gallery image version. It uses the existing Kata provisioners
and UVM artifact download, with a separate image family:

| Purpose | Name |
| --- | --- |
| Release job | `buildAzureLinuxV3gen2kataL1VHPreview` |
| Build gallery definition | `AzureLinuxV3katagen2l1vhpreview` |
| Publishing SKU / proposed final definition | `V3katagen2l1vhpreview` |
| Build artifact | `azurelinuxv3-gen2-kata-l1vhpreview` |

## Required inputs

Queue `.pipelines/.vsts-vhd-builder-release.yaml` from the feature branch with:

- `buildAzureLinuxV3gen2kataL1VHPreview: true` (defaults to false).
- `l1vhSourceImageVersionId`: the full ARM resource ID of the approved Azure Linux
  **base OS** image version, ending in `/versions/<major>.<minor>.<patch>`.
  `latest`, VHD URLs and Marketplace URNs are rejected.
- `l1vhSchedulerType`: `AzureManaged` or `GuestManaged`, **confirmed with the image
  owner and Compute team**. The default `Unspecified` intentionally fails validation.

The existing release-job switches still default to true; deselect other image jobs
when queuing a preview-only build. Retain the pipeline's normal service connection,
network, storage, build-region, and publishing variables.

The pipeline identity needs read access to the source gallery/image; the source
version must be replicated to the Packer build region. Confirm the source is Linux,
generalized, x64, Gen2, has the required L1VH kernel/boot components, and can boot on
the existing `Standard_D16ads_v5` build VM and the scan/prefetch VMs. Image features
alone do not install an L1VH-compatible kernel. If the source cannot boot on those
VMs, the build/test/prefetch VM choices need a follow-up change with the image owner.

## Source selection and build metadata

`build-l1vh-kata-preview.sh` derives a temporary JSON template from the existing
Mariner template, removes its four Marketplace builder source fields, and sets
`shared_image_gallery.id`. The provisioners and output gallery are preserved.
`IMG_OFFER` and `IMG_SKU` remain available as OS identity for existing Kata download
and provisioning conditions; they are not used as the preview's Packer source.

Build setup creates the dedicated gallery definition with:

- `DiskControllerTypes=SCSI,NVMe`
- `VirtualizationType=Direct`
- `DirectVirtualizationSchedulerType=<confirmed scheduler>`

These are **image-definition features**, not Azure resource tags. The helper uses
the existing pipeline Azure CLI authentication and REST API `2025-12-03`, waits for
provisioning, and verifies the definition. A matching existing definition is reused;
incompatible definitions fail without being altered. Standard Kata definitions and
historical image versions are never retagged by this job.

## Final publishing integration (required downstream)

The release flow exports the intermediate gallery image to a VHD blob, including
prefetch processing where configured. **Definition features are not embedded in that
blob.** The generated `vhd-publishing-info.json` adds these fields only for the preview:

- `source_image_version_id`: the exact private gallery version used as input.
- `gallery_image_features`: an array of `{ "name": "...", "value": "..." }` entries
  containing the three features above.

It omits `publisher_base_image_version` and `publisher_base_image_sku` for this source
type and does not query Marketplace for provenance.

Before promoting this artifact, extend the downstream production VHD publisher to:

1. Accept this separate SKU and the new metadata fields; do not require Marketplace
   provenance for a gallery-source build.
2. Create the dedicated final `V3katagen2l1vhpreview` definition with the full feature
   array **before** publishing image versions, in each destination gallery.
3. Validate matching features on existing preview definitions and fail on a mismatch.
4. Preserve the source-version provenance and verify features after publication.

The AgentBaker job does not itself deploy those final production definitions. An old
publisher that ignores the new fields is not sufficient for L1VH. Existing definitions
can support feature updates via `allowUpdateImage` in the newer API, but this preview
deliberately uses a new definition rather than updating a shared image family.

## AKS RP integration and qualification

The normal `AKSAzureLinuxV3Gen2Kata` mapping still selects `V3katagen2`. In a follow-up,
wire the preview image definition/version into AKS RP's enrolled-subscription and
L1VH-capable node-pool image selection, preserving existing Kata behavior elsewhere.
Keep the definition naming and version policy consistent with the downstream publisher.

Run the existing VHD content/security checks and AKS Kata E2Es against the resulting
image. Validate the customer-facing L1VH SKU, image-feature validation, intended
scheduler activation, Kata workload isolation, reboot, scale-out and image upgrade.
If the preview image is intended to support nested-virtualization SKUs too, validate
those as well. A forced-L1VH test flag alone does not qualify the customer SKU path.

## References

- [Packer gallery source](https://developer.hashicorp.com/packer/integrations/hashicorp/azure/latest/components/builder/arm#shared-image-gallery)
- [Gallery Images Create Or Update, API 2025-12-03](https://learn.microsoft.com/en-us/rest/api/compute/gallery-images/create-or-update?view=rest-compute-2025-12-03)
- [Gallery sharing with RBAC](https://learn.microsoft.com/en-us/azure/virtual-machines/share-gallery)
