# Kata direct-virtualization image features

The existing Azure Linux 3 Kata build continues to use its Marketplace source,
Packer template, pipeline job, and image family. The resulting Kata image supports
both nested and direct virtualization; its gallery definition must advertise:

| Feature | Value |
| --- | --- |
| `VirtualizationType` | `Direct` |
| `DirectVirtualizationSchedulerType` | `GuestManaged` |

These are `properties.features` entries, not ARM resource tags. The scheduler value
is fixed for Azure Linux; no new pipeline parameter or private base-image input is needed.

## Build and release flow

`ensure_sig_vhd_exists` invokes `ensure_kata_image_features` after the usual definition
creation/existence check. It applies only to `OS_SKU=AzureLinux`, `SKU_NAME=V3katagen2`
(the normal build definition is `AzureLinuxV3katagen2`). It also updates definitions
that already exist; it does not require rebuilding a definition or moving versions.

The helper reads with API `2025-12-03`, merges the two virtualization features while
preserving unrelated features, and PATCHes with `allowUpdateImage=true`. It preserves
the definition's identity, resource tags, and other properties, waits for provisioning,
and verifies the feature set. A matching definition needs no write. The change is
definition-wide, including existing versions; validate both nested and L1VH behavior
in the test galleries before promoting the change.

The normal release publishing-info step adds `gallery_image_features`, containing
the two `{name, value}` entries, to the existing Kata artifact. Marketplace provenance,
the publishing SKU `V3katagen2`, and versioning are retained. Other SKUs do not get this
field or the feature-update calls.

## Required RP publishing integration

AgentBaker creates the intermediate build image; the RP release publisher creates or
updates the final AKS gallery definitions. **Exporting a gallery image to a VHD blob
does not carry its definition features.** The downstream publisher must consume
`gallery_image_features` and merge these entries into the existing final `V3katagen2`
definition in each destination gallery, preserving features such as NVMe/security.
It must update existing definitions as well as newly created ones and read back the
features after publishing. Merely ignoring the new artifact field is insufficient.

No separate preview pipeline, gallery-source selection, or new RP image family is
needed. L1VH SKU support, test quota, and preview enrollment remain separate concerns.

## Validation

Use the existing Kata build and development/E2E galleries. Verify a Kata workload on
both a nested-virtualization SKU and an official L1VH SKU, including the intended
scheduler selection. A BusyBox pod must explicitly use the Kata RuntimeClass; a
regular pod reaching Running does not demonstrate sandboxing. Prior forced-L1VH
AFEC tests do not replace validation of the official SKU and feature path.

The Azure Linux team confirmed the current Kata image has the OS support; this
metadata change does not install or switch kernels. The pipeline identity must be
able to update the definition, and the new feature API must be available in the
target subscriptions/regions.

Reference: [Gallery Images Update, API 2025-12-03](https://learn.microsoft.com/en-us/rest/api/compute/gallery-images/update?view=rest-compute-2025-12-03).
