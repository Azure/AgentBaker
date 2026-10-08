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

### Ubuntu GRID driver branches

The Ubuntu A10 GRID path uses `aks-gpu-grid` on the patched R580/vGPU 19.6
LTS branch. NCv6 RTX PRO 6000 BSE uses `aks-gpu-grid-v20` on the patched
R595/vGPU 20.2 branch. Driver versions and image tags are pinned in
`parts/common/components.json`; Renovate is constrained to the corresponding
branch for each image.

[Azure's Linux GRID support matrix](https://learn.microsoft.com/azure/virtual-machines/linux/n-series-driver-setup#supported-grid-drivers)
lists v19.6 and v20.2 for NVadsA10_v5, but only v20.2 for NCv6 and NCasT4_v3.
This does not change existing CUDA selection for compute SKUs, or the separate
Azure Linux/ACL sysext installation paths. The existing NCads_A10_v4 aliases
retain their A10 GRID image selection.

The Ubuntu GRID E2E scenarios require the exact configured driver version on
every GPU, in addition to checking GRID licensing and service health. These
checks do not establish application-specific renderer compatibility.

### Shell scripts

For ShellSpec unit test instructions, see the [ShellSpec README](./spec/README.md).

### ACL artifact streaming

ACL uses the standalone `artifact-streaming` system extension rather than
installing RPMs into its read-only base OS. When streaming is enabled, CSE
resolves the extension using the booted ACL `VERSION_ID`, activates it, prepares
writable configuration/cache paths, reloads units, and uses the existing
ACR Mirror enablement path. Other operating systems and streaming-disabled
nodes retain their existing behavior.

The matching extension must be built, signed, and published before enabling
this path. It contains ACR Mirror and the OverlayBD dependencies; the legacy
`overlaybd` extension alone is not sufficient. A missing payload remains a
provisioning error, not a successful fallback to non-streaming behavior.

### E2E

The E2E suite creates VM scale sets, provisions Kubernetes nodes with AgentBaker output, and validates them against AKS clusters.

See the [E2E directory](e2e/).

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
