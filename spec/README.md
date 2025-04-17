# Shell scripts unit tests

Please visit the official [GitHub link](https://github.com/shellspec/shellspec) for more details. Below is a brief use case.

## Installation 

`Shellspec` is used as a framework for unit test. There are 2 options to install it.

### Option 1 - recommended, using makefile to install in project
`Shellspec` is already included in the makefile. You can install it simply by running `make tools-install` or `make generate` in root (/AgentBaker) directory. 

Note: `make generate` will install and run the shellspec tests.

### Option 2 - install in your local machine
If you want to install it in your local machine, please run `curl -fsSL https://git.io/shellspec | sh`.

By default, it should be installed in `~/.local/lib/shellspec`. Please append it to the `$PATH` for your convenience. Example command `export PATH=$PATH:~/.local/lib/shellspec`.

## Authoring tests

You will need to write `xxx_spec.sh` file for the test.

For example, `AgentBaker/spec/parts/linux/cloud-init/artifacts/cse_install_spec.sh` is a test file for `AgentBaker/parts/linux/cloud-init/artifacts/cse_install.sh`

## Running tests locally

To run all tests, in AgentBaker folder, simply run `bash ./hack/tools/bin/shellspec` in root (/AgentBaker) directory. 

### Useful commands for debugging

- `bash ./hack/tools/bin/shellspec -x` => with `-x`, it will show verbose trace for debugging.
- `bash ./hack/tools/bin/shellspec -E "<test name>"` => you can run a single test case by using `-E` and the test name. For example, `bash ./hack/tools/bin/shellspec -E "returns downloadURIs.ubuntu.\"r2004\".downloadURL of package runc for UBUNTU 20.04"`. You can also do `-xE` for verbose trace for a single test case.
- `bash ./hack/tools/bin/shellspec "path to xxx_spec.sh"` => by providing a full path a particular spec file, you can run only that spec file instead of all spec files in AgentBaker project. 
For example, `bash ./hack/tools/bin/shellspec "spec/parts/linux/cloud-init/artifacts/cse_install_spec.sh"`