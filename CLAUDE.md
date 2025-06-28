# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Essential Commands

### Development Setup
```bash
make -C hack/tools install    # Install all development tools (golangci-lint, etc.)
```

### Build and Test
```bash
make                         # Main build target - runs generate, embed files, tests
make generate               # Regenerate Go testdata, manifest.cue, validate shell scripts
make test                   # Run all unit tests
make test-aks-node-controller  # Test the aks-node-controller component
```

### Linting and Validation
```bash
make test-style             # Run all style validations (go, shell, copyright)
./hack/tools/bin/golangci-lint run  # Manual Go linting
make validate-shell         # Shell script validation with shellcheck
make validate-components    # Validate components.json against schema
```

### Testing
```bash
go test ./...               # Run all Go unit tests
make shellspec              # Run shell script unit tests with ShellSpec
make shellspec-focus        # Run focused shell script tests
```

## Architecture Overview

AgentBaker is a collection of components for provisioning Kubernetes nodes in Azure, primarily consumed by AKS. It consists of two main services:

### VHD Builder
- Builds VM images using Packer for Windows, Azure Linux/Mariner, and Ubuntu
- VHDs serve as base images for AKS cluster nodes
- Configuration: `vhdbuilder/packer/` directory
- Windows VHD: `vhdbuilder/packer/windows/windows-vhd-configuration.ps1`

### AgentBaker Service
- Go-based web server (`apiserver/`) that generates CSE (Custom Script Extension) and CustomData
- Receives requests from external clients and renders templates for node provisioning
- Determines latest Linux VHD versions for AKS clusters
- Windows CSE: `parts/windows/kuberneteswindowssetup.ps1`

## Key Directories and Components

### Code Organization
- `parts/` - Shared components used by both AgentBaker Service and VHD builds
- `pkg/agent/` - Core agent logic and baker functionality  
- `apiserver/` - Web server API implementation
- `aks-node-controller/` - Node controller component with protobuf definitions
- `e2e/` - End-to-end tests and test helpers
- `vhdbuilder/` - Packer templates and VHD building logic
- `staging/cse/windows/` - Windows-specific CSE components (downloadable at provision time)

### Generated Content
**CRITICAL**: After modifying files in `parts/` or `pkg/` directories, you MUST run:
```bash
make generate
```
This regenerates snapshot test data and ensures consistency between code and tests.

## Development Guidelines

### File Lifecycle Understanding
When making changes, determine whether files are used during:
- **VHD build stage** - Files embedded into base images
- **Node provision stage** - Files used when creating new cluster nodes  
- **Both stages** - Shared components in `parts/`

### Language-Specific Guidelines

#### Go Code
- Follow standard Go practices
- Use vanilla Go test framework
- Located primarily in `pkg/`, `apiserver/`, `aks-node-controller/`

#### Shell Scripts  
- Use shellcheck for validation (automated via `make validate-shell`)
- Use ShellSpec for testing (`make shellspec`)
- Must work on both Azure Linux/Mariner and Ubuntu (cross-platform compatibility critical)
- Source functions properly from other files
- Prefer local variables over constants when scoping allows
- Avoid using variables declared in other functions

#### PowerShell
- Follow PowerShell best practices
- Used primarily for Windows VHD configuration and CSE
- Located in `vhdbuilder/packer/windows/` and `staging/cse/windows/`

## Release Process

### Linux VHDs and AgentBaker
- Released weekly with tags in format `v0.YYYYMMDD.PATCH`
- VHD versions use format `YYYYMM.DD.PATCH`
- Mapping defined in `pkg/agent/datamodel/linux_sig_version.json`

### Windows VHDs  
- Released separately following Windows Patch Tuesday schedule

## Testing and Validation

### Required Before Commits
1. Run `make generate` after modifying `parts/` or `pkg/` directories
2. Run `make test-style` to validate style compliance  
3. Ensure all tests pass with `make test`

### E2E Testing
For local testing and debugging:
```bash
# See e2e/README.md for setup instructions
```

### Protocol Buffers (aks-node-controller)
```bash
make -C aks-node-controller proto-lint  # Lint protobuf files
```

## CI Integration

The project uses Azure DevOps pipelines with templates:
- Linux/Ubuntu: `.pipelines/templates/.builder-release-template.yaml`
- Windows: `.pipelines/templates/.builder-release-template-windows.yaml`
- golangci-lint runs automatically on PRs using "no-new-issues" feature
- Commit messages must follow [conventional commits](https://www.conventionalcommits.org/) format