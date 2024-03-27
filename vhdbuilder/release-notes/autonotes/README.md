# autonotes

This is a simple Go tool to help shuffle release note files
from Azure Pipelines into the AgentBaker repo structure.

```bash
# download all release notes from this run ID.
autonotes --build 40965293
```

## Install

```bash
cd vhdbuilder/release-notes/autonotes
go install .
cd ../../..
```

It requires Azure CLI, the azure-devops extension, and an authenticated
session to function.

```bash
az login
az extension add -n azure-devops
az devops configure --defaults organization=https://dev.azure.com/msazure project=CloudNativeCompute
```

It accepts:
- `--build`: a run ID for a VHD build (usually a run from the weekly VHD
release, but it can be any build). e.g. `40965293`.
- `--path`: a relative or absolute path to the root folder for Ubuntu
  VHD release notes. The default value is
  `vhdbuilder/release-notes/AKSUbuntu`, which will work when executing
  the binary from the root of the AgentBaker repo. 
- `--date`: the VHD build date, in the format `YYYY.MM.DD`, e.g.
  `2021.03.31`. This will be used for file naming in the output.
- `--include`: only download release notes for VHDs in this
  comma-separated list. e.g. `1604,1804`.
- `--ignore`: skip downloading release notes for VHDs in this
  comma-separated list. e.g. `1804-gen2-gpu`.

Example invocations:
```bash
# download all release notes from this run ID.
autonotes --build 40965293
# download ONLY 1804-gen2-gpu release notes from this run ID.
autonotes --build 40968951 --include 1804-gen2-gpu
# download everything EXCEPT 1804-gen2-gpu release notes from this run ID.
autonotes --build 40968951 --ignore 1804-gen2-gpu
# download ONLY 1604,1804,1804-containerd release notes from this run ID.
autonotes --build 40968951 --include 1604,1804,1804-containerd
```

## Adding new VHDs

When we add new VHDs, a developer should update the `artifactToPath` map
with the new artifact names and output paths. The artifact names can be
obtained from the name of the per-VHD artifact folders in the build
outputs. The output paths follow whatever pattern we decide by
convention (?).
