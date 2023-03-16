# Agentbaker
[![Coverage Status](https://coveralls.io/repos/github/UtheMan/AgentBaker/badge.svg?branch=master)](https://coveralls.io/github/UtheMan/AgentBaker?branch=master)

Agentbaker is a collection of components used to provision Kubernetes nodes in Azure.

Agentbaker has a few pieces
- Packer templates and scripts to build VM images.
- A set of templates and a public API to render those templates given input config.
- An API to retrieve the latest VM image version for new clusters.

The primary consumer of Agentbaker is Azure Kubernetes Service (AKS).

AKS uses Agentbaker to provision Linux and Windows Kubernetes nodes.

# Contributing

Developing agentbaker requires a few basic requisites:
- Go (at least version 1.19)
- Make

If you change code or artifacts used to generate custom data or custom script extension payloads, you should run `make`.

This re-runs code to embed static files in Go code, which is what will actually be used at runtime.

This additionally runs unit tests (equivalent of `go test ./...`) and regenerates snapshot testdata.

## Testing

Most code may be tested with vanilla Go unit tests.

## Snapshot

We also have snapshot data tests, which store the output of key APIs as files on disk.

We can manually verify the snapshot content looks correct.

We now have unit tests which can directly validate the content without leaving generated files on disk. 

See `./pkg/agent/baker_test.go` for examples (search for `dynamic-config-dir` to see a validation sample.).

### E2E

We also have basic e2e tests which run a subset of the agentbaker API against a real Azure subscription.

These tests join a standalone VM to an existing cluster. 

They use a basic NodeBootstrappingConfiguration template, overriding it with per-scenario config.

`./e2e/nodebootstrapping_template.json` defines the base configuration.

Specific scenarios exist in `./e2e/scenarios/$SCENARIO_NAME`, for example `./e2e/vanilla-gpu` contains a GPU VM sku scenario to test driver provisioning.

You can run these locally:

```bash
cd e2e
# scenario name and VM size as args currently
bash ./e2e-local.sh vanilla-aks Standard_D4ads_v5
```

This will generate many files in the current directory including an SSH key in case you need to debug the VM afterwards.

## Contributor License Agreement (CLA)

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.opensource.microsoft.com.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

# CGManifest File
A cgmanifest file is a json file used to register components manually when the component type is not supported by governance. The file name is "cgmanifest.json" and you can have as many as you need and can be anywhere in your repository.

File path: `./vhdbuilder/cgmanifest.json`

Reference: https://docs.opensource.microsoft.com/tools/cg/cgmanifest.html

Package:
- Calico Windows: https://docs.projectcalico.org/release-notes/
- 