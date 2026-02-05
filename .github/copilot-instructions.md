# Overview

AgentBaker repo has 2 main services discussed below:

- VHD Builder
- AgentBaker Service

## VHD Builder

It builds VHDs using Packer for base OS: Windows, Azure Linux/Mariner and Ubuntu. For each OS there are multiple supported versions (windows 2019, 2022, ubuntu 2004, 2204 etc). The VHDs are base images for a node in an aks cluster.

VHDs are built using [Packer](https://developer.hashicorp.com/packer/docs) in [vhdbuilder](./vhdbuilder/).

Windows VHD is configured through [VHD](./vhdbuilder/packer/windows/windows-vhd-configuration.ps1)

## AgentBaker Service

[apiserver](./apiserver/) is `go` based webserver. It receives request from external client and generates CSE and CustomData to be used on the VHD when a new node is created / provisioned.

windows generates its CSE package using [script](./parts/windows/kuberneteswindowssetup.ps1).

The webserver is also used to determine the latest version of Linux VHDs available for provisioning within AKS clusters.

## Code Structure

[parts](./parts/) serves both AgentBaker Service and VHD build. AgentBaker service and VHDs are coupled because of this shared component. When building VHD, packer maps and renames scripts from [parts](./parts/)  depending on the OS / versions. The mappings can be found at [packer](./vhdbuilder/packer/).

> **IMPORTANT**: When making changes to files in the `parts` or `pkg` directories, you must run `make generate` afterward to regenerate the snapshot test data. This ensures consistency between the code and tests and prevents regressions.

Windows uses a different folder [cse](./staging/cse/windows/) for almost the same purpose. There are subtle differences as windows CSEs can be downloaded as a zip file during provisioning time due to restrictions on the file size on Windows system, while for linux based systems the cse/custom data are dropped in during provisioning time.

## Deployment and Release

The VHD build is triggered by Azure Devops [pipelines](.pipelines/). For release, the pipelines following the same templates for different OS versions:

- [linux/ubuntu](./.pipelines/templates/.builder-release-template.yaml)
- [windows](./.pipelines/templates/.builder-release-template-windows.yaml)

you can reason the steps by following the steps defined in the pipeline.

Tags of AgentBaker and corresponding Linux VHDs are released every week. Linux VHDs are built with a particular image version in the YYYYMM.DD.PATCH format. All Linux VHD versions correspond to a particular tag of the AgentBaker go module. AgentBaker go module tags follow the format v0.YYYYMMDD.PATCH. The mapping between AgentBaker tag and Linux VHD version is defined within [linux_sig_version.json](./pkg/agent/datamodel/linux_sig_version.json).

Windows VHD are released separately, following windows patch tuesday schedule.

## Guidelines

### SRE Guidelines

The operational goals of this project are:

- achieve consistency across different OS as much as possible
- avoid functional regression when introducing new features (component updates, new drivers, new binaries), ensure that all supported OS / versions are tested
- avoid VHD build performance regressions when making any changes
- avoid node provisioning performance regression when making any changes

When making changes, reason whether the file is used in VHD building stage, or provision stage, or both. Make sure the changes are valid in its life stage. as an example, [windows-vhd-configuration.ps1](./vhdbuilder/packer/windows/windows-vhd-configuration.ps1) defines container images to be cached in VHD, while [configure-windows-vhd.ps1](./vhdbuilder/packer/windows/configure-windows-vhd.ps1) executes commands at provision time.

One way to debug / explore / just for fun is to run [e2e](./e2e/) tests. To run locally, follow the readme file under that folder. 

The SRE guidelines ground other coding guidelines and practices.

### Golang Guidelines

- Follow Go best practice
- Use vanilla go test framework

### PowerShell Guidelines

- follow PowerShell best practices

### ShellScripts Guidelines

- use shellcheck for sanity checking
- use ShellSpec for testing
- the shell scripts are used on both azure linux/mariner and ubuntu and cross platform portability is critical.
- when using functions defined in other files, ensure it is sourced properly.
- use local variables rather than constants when their scoping allows for it.
- avoid using variables declared inside another function, even they are visible. It is hard to reason and might introduce subtle bugs.

## Pull Request Review Guidelines

When reviewing pull requests, perform breaking change analysis to prevent regressions. VHDs remain in production for 6 months, so backward compatibility is critical.

**Review Approach**: Focus on high-level architecture, security vulnerabilities, and logic bugs. Apply deep reasoning similar to advanced models (e.g., Claude Opus) - don't just pattern match, but truly understand the code's intent, dependencies, and potential failure modes.

### Breaking Change Detection

Analyze PRs for these compatibility scenarios:

**1. Linux Provisioning Script Changes**
- **Context**: Scripts in `parts/linux/cloud-init/artifacts/` run during critical VM bootstrap and are used in both:
  - VHD build (uploaded via packer configs in `vhdbuilder/packer/*.json`)
  - VM provisioning (CSE - embedded in Go service via `pkg/agent/const.go`)
  - Versions synchronized via `pkg/agent/datamodel/linux_sig_version.json`
- **What to check**: Changes that could break VM provisioning in production
- **Breaking signals**:
  - **Script logic errors**: Syntax errors, wrong commands, incorrect flags, broken pipes
  - **Dependency issues**:
    - Calling functions before they're sourced
    - Using variables declared in other functions
    - Removing `source` statements that break dependency chains
  - **Cross-distro compatibility**:
    - Commands that don't work on both Ubuntu and Azure Linux/Mariner (check distro-specific variants: `ubuntu/`, `mariner/`)
    - Package manager assumptions (apt vs dnf/tdnf)
    - Missing OS-specific conditional logic
  - **External dependency violations**:
    - NEW: Downloading from internet URLs not in `parts/common/components.json` or allowed sources (packages.aks.azure.com)
    - All external dependencies MUST be referenced in `parts/common/components.json` for Renovate updates
    - Only allowed runtime downloads: packages.aks.azure.com or other explicitly allowed sources in CSE
  - **Function signature changes**: Parameters, return values, exit codes that break callers
  - **Missing test coverage**: Changes to provisioning logic without corresponding e2e tests

**2. Windows Bidirectional Compatibility**
- **Context**: Windows VHD and CSE scripts release on different cadences with no guaranteed order
- **What to check**: Changes to `staging/cse/windows/` (CSE scripts) or `vhdbuilder/packer/windows/` (VHD scripts)
- **Breaking signals**:
  - New CSE scripts assuming capabilities that old VHDs don't have
  - New VHD scripts expecting features that old CSE versions don't provide
  - Changes to shared state (registry keys, files, environment variables) that break coordination
  - Removing PowerShell functions or cmdlets that the other component might call

**3. aks-node-controller Migration (Dual-Mode Support)**
- **Context**: Transitioning from uploading scripts during both VHD build and CSE to only uploading aks-node-controller during VHD build
- **What to check**: Any changes must work in BOTH deployment modes
- **Breaking signals**:
  - Assumptions that scripts are always uploaded during CSE (new mode won't do this)
  - Assumptions that aks-node-controller is always present (old VHDs won't have it)
  - Missing feature detection to determine which mode is running
  - Hardcoded paths that differ between deployment modes

**4. Cross-OS Compatibility**
- **What to check**: Changes work on Ubuntu, Azure Linux/Mariner, and Windows
- **Breaking signals**:
  - Linux commands that don't work on both Ubuntu and Azure Linux/Mariner
  - Missing conditional logic for OS-specific behaviors
  - Package manager assumptions (apt vs dnf/tdnf)
  - Systemd differences between distributions

### Analysis Approach

**Dynamic Dependency Tracing**:
1. For each changed file, identify what depends on it
2. Follow `source` statements in bash scripts to trace dependency chains
3. Check for function calls, variable references across files
4. Look for hardcoded paths in VHD build scripts (`vhdbuilder/packer/`) that reference changed files
5. Trace through as many levels as needed within the codebase
6. **Check external dependencies**:
   - Search for new URLs being downloaded (curl, wget, etc.)
   - Verify all external dependencies are in `parts/common/components.json` for Renovate updates
   - Flag downloads from unauthorized sources (only packages.aks.azure.com and sources in components.json allowed)

**Historical Context**:
- Look for related changes that previously caused issues
- Identify patterns of fragile areas that break frequently

**Test Coverage Assessment**:
- Note if changed code has e2e test coverage
- Flag changes to untested areas as higher risk
- Mention if new behavior lacks corresponding test additions

### Review Output Format

Provide targeted inline comments on specific lines where you detect issues:

**For each breaking change or risk:**
- Comment directly on the problematic line or code block
- Explain why this is risky (e.g., "This removes function X which may be called by VHDs built in the last 6 months")
- Suggest specific mitigations or alternatives
- Include actionable next steps (e.g., "Verify this function is not used by checking references in `vhdbuilder/packer/`")

**Risk indicators to include:**
- Severity: 🔴 High Risk | 🟡 Medium Risk | 🟢 Low Risk
- Category: Script Logic | Cross-OS | External Dependency | Test Coverage | etc.

**Only comment when you have substantive findings** - avoid noise on trivial or obviously safe changes.

### Review Philosophy

Think like an experienced reviewer who "eyeballs" PRs for subtle risks. Look beyond pattern matching:
- Understand the architecture and how components interact
- Consider timing of releases and deployment sequences
- Reason about implicit dependencies and assumptions
- Flag changes that "feel risky" even without obvious red flags
- Balance thoroughness with actionable feedback
- Focus on high-impact issues that could break production VM provisioning