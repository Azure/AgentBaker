# Hockney — History

## Project Context
- **Project:** AgentBaker — builds VHDs using Packer and provides a Go-based API server that generates CSE/CustomData for AKS node provisioning
- **Tech stack:** Go, PowerShell, Bash, Packer, Azure DevOps pipelines
- **User:** Tim Wright
- **Focus:** Windows AKS nodes — VHD builds (Windows 2019, 2022, 2025), CSE scripts, container image caching, node provisioning

## Key Architecture
- Windows VHD and CSE scripts release on different cadences with no guaranteed deployment order
- `staging/cse/windows/` — CSE scripts downloaded as zip at provisioning time
- `vhdbuilder/packer/windows/` — VHD build scripts
- `parts/common/components.json` — shared component versions across all OS types
- Changes to `parts/` or `pkg/` require `make generate` to regenerate snapshot test data

## Learnings
