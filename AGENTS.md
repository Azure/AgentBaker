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

windows generates its CSE package using [script](./parts/windows/kuberneteswindowssetup.ps1.template).

Linux and Windows CSE use `basePrep`/`nodePrep` (`BasePrep`/`NodePrep` on Windows). PIS uses the same CSE version as its base VHD. When the base VHD changes, PIS uses a new image. `basePrep` installs static components, including packages selected by CustomData, and is skipped when `base_prep.complete` exists. `nodePrep` refreshes per-node, secret, or expiring data using live inputs, not bake-time variables. A toggle-only change may require a rebake; this existing constraint alone is not a PR regression.

The webserver is also used to determine the latest version of Linux VHDs available for provisioning within AKS clusters.

## Code Structure

[parts](./parts/) serves both AgentBaker Service and VHD build. AgentBaker service and VHDs are coupled because of this shared component. When building VHD, packer maps and renames scripts from [parts](./parts/)  depending on the OS / versions. The mappings can be found at [packer](./vhdbuilder/packer/).

Windows uses a different folder [cse](./staging/cse/windows/) for almost the same purpose. There are subtle differences as windows CSEs can be downloaded as a zip file during provisioning time due to restrictions on the file size on Windows system, while for linux based systems the cse/custom data are dropped in during provisioning time.

## Deployment and Release

The VHD build is triggered by Azure Devops [pipelines](.pipelines/). For release, the pipelines following the same templates for different OS versions:

- [linux/ubuntu](./.pipelines/templates/.builder-release-template.yaml)
- [windows](./.pipelines/templates/.builder-release-template-windows.yaml)

you can reason the steps by following the steps defined in the pipeline.

Tags of AgentBaker and corresponding Linux VHDs are released every week. Linux VHDs are built with a particular image version in the YYYYMM.DD.PATCH format. All Linux VHD versions correspond to a particular tag of the AgentBaker go module. AgentBaker go module tags follow the format v0.YYYYMMDD.PATCH. The mapping between AgentBaker tag and Linux VHD version is defined within [linux_sig_version.json](./pkg/agent/datamodel/linux_sig_version.json).

Windows VHD are released separately, following windows patch tuesday schedule.

## Contributing

When opening a pull request against this repository, the PR's head branch must live in `Azure/AgentBaker` itself — **not in a fork**.

## Guidelines

### SRE Guidelines

- Keep behavior consistent across supported OS versions and test affected variants.
- Avoid regressions in functionality, VHD build time, and node provisioning time.
- Check whether changed code runs during VHD build, provisioning, or both. For example, [windows-vhd-configuration.ps1](./vhdbuilder/packer/windows/windows-vhd-configuration.ps1) defines cached images; [configure-windows-vhd.ps1](./vhdbuilder/packer/windows/configure-windows-vhd.ps1) runs during provisioning.
- Use build flags only for system calls, not for OS-specific logic.

### ShellScripts Guidelines

- Use ShellCheck and ShellSpec.
- Keep scripts portable across Ubuntu and Azure Linux/Mariner.
- Source helpers before use.
- Prefer local variables; do not rely on variables declared inside another function.
