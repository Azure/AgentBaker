# Fenster — History

## Project Context
- **Project:** AgentBaker — builds VHDs using Packer and provides a Go-based API server that generates CSE/CustomData for AKS node provisioning
- **Tech stack:** Go, Bash, Packer, Azure DevOps pipelines
- **User:** Tim Wright
- **Focus:** Linux AKS nodes — VHD builds (Ubuntu 2204/2404, Azure Linux 3.0), provisioning scripts, cloud-init artifacts, cross-distro compatibility

## Key Architecture
- Linux VHDs built via Packer; scripts sourced from `parts/linux/cloud-init/artifacts/`
- CSE scripts embedded in Go service via `pkg/agent/const.go`
- Versions synchronized via `pkg/agent/datamodel/linux_sig_version.json`
- Tags follow v0.YYYYMMDD.PATCH format; VHD versions follow YYYYMM.DD.PATCH
- First apt-get call after Ubuntu VHD boot triggers expensive apt initialization (~30-50s) — use dpkg -i for cached debs
- VHD build runs apt-get clean but keeps /var/lib/apt/lists/
- Background aptmarkWALinuxAgent holds dpkg lock early in CSE

## Learnings
