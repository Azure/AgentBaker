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
