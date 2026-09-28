# AgentBaker

AgentBaker provides components that build VM images and provision Kubernetes nodes in Azure.

AgentBaker includes:

- A VHD builder that creates Linux and Windows node images.
- Tools and scripts that provision VMs as Kubernetes nodes.

The primary consumer of AgentBaker is Azure Kubernetes Service (AKS).

AKS uses AgentBaker to provision Linux and Windows Kubernetes nodes.

## Style

We use [golangci-lint](https://golangci-lint.run/) to enforce style.

Run `make -C hack/tools install` to install the linter.

Pull request titles must follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) because pull requests are squashed during merge.

## Tests

### Shell scripts

For ShellSpec unit test instructions, see the [ShellSpec README](./spec/README.md).

### E2E

The E2E suite creates VM scale sets, provisions Kubernetes nodes with AgentBaker output, and validates them against AKS clusters.

See the [E2E directory](e2e/).

### ACL TL source images in the VHD release pipeline

When queuing `.pipelines/.vsts-vhd-builder-release.yaml`, set
`aclTlSourceGallery`, `aclTlSourceImage`, and `aclTlSourceVersion` to the
**published signed** AMD64 source gallery unique name, image definition, and
exact version. For ARM64, use `aclArm64TlSourceGallery`,
`aclArm64TlSourceImage`, and `aclArm64TlSourceVersion`. These string parameters
apply only to the non-FIPS `buildacltlgen2` and `buildaclarm64tlgen2` jobs;
their defaults are the existing ACL source images. An empty or malformed
selector fails the corresponding setup step rather than selecting `latest`.
Packer resolves `/SharedGalleries/<uniqueName>/images/<definition>/versions/<version>`;
the signed source must be directly shared with the builder identity and
available in Packer's region. RBAC read access alone is insufficient.

To compare default-off and audit-default images, queue one run per mode with
that mode's exact signed source version(s). Set the queue parameter
`aclIpeExpectedMode=off` or `audit` for the corresponding AMD64 `ACL` check;
its default `none` leaves ordinary E2E unchanged. This validation-only
selector does not set an IPE tag or infer the mode from the image. Select the
intended TL build jobs and disable unrelated build jobs (including the
separate FIPS TL jobs). The builder still publishes the normal `aclgen2TL` /
`aclgen2arm64TL` outputs. Same-run E2E uses `useVhdMetadataArtifacts: true` and
`IgnoreScenariosWithMissingVhd: true`; an ordinary run can skip a missing ACL
image or region, but an opted-in run fails if ACL is absent, filtered, skipped,
or lacks passing mode-specific first-boot evidence (and audit-deny evidence in
audit mode). Opt-in also runs E2E despite `SKIP_E2E_TESTS=true`. When it runs,
only the exact `ACL` scenario is selected, not other Linux or ACL scenarios;
all Go unit tests still run first. `ACL` boots the output in a scenario-owned,
single-VM Compute VMSS that joins an existing AKS cluster as a Kubernetes
Ready node and hosts a targeted test pod. It does not create an AKS-managed
node pool; its optional [isolated-VMSS off-to-audit transition](e2e/README.md#opt-in-acl-ipe-first-boot-checks-on-an-aks-registered-vmss-node)
does not test an AKS-managed pool. Require evidence
from each run: build logs identify the exact source; its E2E metadata maps
`aclgen2TL` (and `aclgen2arm64TL` if selected) to the output SIG resource
ID/version; replication to the test region completed; and E2E results show the
intended ACL scenario ran without being skipped. The metadata identifies the
output, not the source: correlate it with the source logs by build ID. Source
selection alone does not enable IPE on-node validation or change destination
gallery settings.

## Contributor License Agreement (CLA)

This project welcomes contributions and suggestions. Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.opensource.microsoft.com.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

# CGManifest File

A cgmanifest file is a json file used to register components manually when the component type is not supported by
governance. The file name is "cgmanifest.json" and you can have as many as you need and can be anywhere in your
repository.

File path: `./vhdbuilder/cgmanifest.json`

Reference: https://docs.opensource.microsoft.com/tools/cg/cgmanifest.html

Package:

- Calico Windows: https://docs.projectcalico.org/release-notes/
