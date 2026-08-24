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

Both CSE scripts run in two phases — Windows `BasePrep`/`NodePrep` (`kuberneteswindowssetup.ps1.template`) and Linux `basePrep`/`nodePrep` (`cse_main.sh`). `basePrep` is gated only by the `base_prep.complete` marker, so it is **skipped on a PIS-cached VHD** (the marker is baked in); `nodePrep` runs whenever `PreProvisionOnly` is false — every real node. So node-specific, secret, or expiring data (e.g. the TLS bootstrap token / kubeconfig) must be written in `nodePrep`, not `basePrep`, or the baked copy goes stale. On PIS the phases are separate VM runs — variables re-initialize from the real node's live CustomData, so `nodePrep` can't rely on `basePrep` state. (Non-PIS: both run sequentially in one execution, no reboot between phases.)

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

The operational goals of this project are:

- achieve consistency across different OS as much as possible
- avoid functional regression when introducing new features (component updates, new drivers, new binaries), ensure that all supported OS / versions are tested
- avoid VHD build performance regressions when making any changes
- avoid node provisioning performance regression when making any changes

When making changes, reason whether the file is used in VHD building stage, or provision stage, or both. Make sure the changes are valid in its life stage. as an example, [windows-vhd-configuration.ps1](./vhdbuilder/packer/windows/windows-vhd-configuration.ps1) defines container images to be cached in VHD, while [configure-windows-vhd.ps1](./vhdbuilder/packer/windows/configure-windows-vhd.ps1) executes commands at provision time.

One way to debug / explore / just for fun is to run [e2e](./e2e/) tests. To run locally, follow the readme file under that folder.

The SRE guidelines ground other coding guidelines and practices.

### PowerShell Guidelines

- follow PowerShell best practices

### ShellScripts Guidelines

- use shellcheck for sanity checking
- use ShellSpec for testing
- the shell scripts are used on both azure linux/mariner and ubuntu and cross platform portability is critical.
- when using functions defined in other files, ensure it is sourced properly.
- use local variables rather than constants when their scoping allows for it.
- avoid using variables declared inside another function, even they are visible. It is hard to reason and might introduce subtle bugs.
