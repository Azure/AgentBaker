# Agentbaker

[![Coverage Status](https://coveralls.io/repos/github/Azure/AgentBaker/badge.svg?branch=master)](https://coveralls.io/github/Azure/AgentBaker?branch=master)

Agentbaker is a collection of components used to provision Kubernetes nodes in Azure.

Agentbaker has a few pieces

- Packer templates and scripts to build VM images.
- A set of templates and a public API to render those templates given input config.
- An API to retrieve the latest VM image version for new clusters.

The primary consumer of Agentbaker is Azure Kubernetes Service (AKS).

AKS uses Agentbaker to provision Linux and Windows Kubernetes nodes.

## Contributing

Developing agentbaker requires a few basic requisites:

- Go (at least version 1.19)
- Make

Run `make -C hack/tools install` to install all development tools.

If you change code or artifacts used to generate custom data or custom script extension payloads, you should run `make`.

This re-runs code to embed static files in Go code, which is what will actually be used at runtime.

This additionally runs unit tests (equivalent of `go test ./...`) and regenerates snapshot testdata.

## Style

We use [golangci-lint](https://golangci-lint.run/) to enforce style.

Run `make -C hack/tools install` to install the linter.

Run `./hack/tools/bin/golangci-lint run` to run the linter.

We currently have many failures we hope to eliminate.

We have [job to run golangci-lint on pull requests]().

This job uses the linters "no-new-issues" feature.

As long as PRs don't introduce net new issues, they should pass.

We also have a linting job to enforce commit message style.

We adhere to [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/).

Prefer pull requests with single commits.

To clean up in-progress commits, you can use `git rebase -i` to fixup commits.

See the [git documentation](https://git-scm.com/book/en/v2/Git-Tools-Rewriting-History#_squashing) for more details.

## Testing

Most code may be tested with vanilla Go unit tests.

## shell scripts unit tests

Please visit the official [GitHub link](https://github.com/shellspec/shellspec) for more details. Below is a brief use case.

### Installation 

`Shellspec` is used as a framework for unit test. There are 2 options to install it.

#### Option 1 - recommended, using makefile to install in project
`Shellspec` is already included in the makefile. You can install it simply by running `make tools-install` or `make generate` in root (/AgentBaker) directory. 

Note: `make generate` will install and run the shellspec tests.

#### Option 2 - install in your local machine
If you want to install it in your local machine, please run `curl -fsSL https://git.io/shellspec | sh`.

By default, it should be installed in `~/.local/lib/shellspec`. Please append it to the $PATH for your convenience. Example command `export PATH=$PATH:~/.local/lib/shellspec`.

### Authoring tests

You will need to write `xxx_spec.sh` file for the test.

For example, `AgentBaker/spec/parts/linux/cloud-init/artifacts/cse_install_spec.sh` is a test file for `AgentBaker/parts/linux/cloud-init/artifacts/cse_install.sh`

### Running tests locally

To run all tests, in AgentBaker folder, simply run `bash ./hack/tools/bin/shellspec` in root (/AgentBaker) directory. 

#### Useful commands for debugging

- `bash ./hack/tools/bin/shellspec -x` => with `-x`, it will show verbose trace for debugging.
- `bash ./hack/tools/bin/shellspec -E "<test name>"` => you can run a single test case by using `-E` and the test name. For example, `bash ./hack/tools/bin/shellspec -E "returns downloadURIs.ubuntu.\"r2004\".downloadURL of package runc for UBUNTU 20.04"`. You can also do `-xE` for verbose trace for a single test case.
- `bash ./hack/tools/bin/shellspec "path to xxx_spec.sh"` => by providing a full path a particular spec file, you can run only that spec file instead of all spec files in AgentBaker project. 
For example, `bash ./hack/tools/bin/shellspec "spec/parts/linux/cloud-init/artifacts/cse_install_spec.sh"`


## Snapshot

We also have snapshot data tests, which store the output of key APIs as files on disk.

We can manually verify the snapshot content looks correct.

We now have unit tests which can directly validate the content without leaving generated files on disk.

See `./pkg/agent/baker_test.go` for examples (search for `dynamic-config-dir` to see a validation sample.).

### E2E

Checkout the [e2e directory](e2e/).

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

