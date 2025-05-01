# Overview

AgentBaker repo has 2 main services discussed below:

- VHD Builder
- AgentBaker Service

## VHD Builder

It builds VHDs using Packer for base OS: Windows, Azure Linux/Mariner and Ubuntu. For each OS there are multiple supported versions (windows 2019, 2022, ubuntu 2004, 2204 etc). THe VHDs are base images for a node in an aks cluster.

VHDs are built using [Packer](https://developer.hashicorp.com/packer/docs) in [vhdbuilder](../vhdbuilder/).

## AgentBaker Service

[apiserver](../apiserver/) is `go` based webserver. It receives request from external client and generates CSE and CustomData to be used on the VHD when a new node is created / provisioned.

The webserver is also used to determine the latest version of Linux VHDs available for provisioning within AKS clusters.

## Code Structure

[parts](../parts/) serves both AgentBaker Service and VHD build. AgentBaker service and VHDs are coupled because of this shared component. When building VHD, packer maps and renames scripts from [parts](../parts/)  depending on the OS / versions. The mappings can be found at [packer](../vhdbuilder/packer/).

Windows uses a different folder [cse](../staging/cse/windows/) for almost the same purpose. There are subtle differences as windows CSEs can be downloaded as a zip file during provisioning time due to restrictions on the file size on Windows system, while for linux based systems the cse/custom data are dropped in during provisioning time.

## Deployment and Release

Tags of AgentBaker and corresponding Linux VHDs are released every week. Linux VHDs are built with a particular image version in the YYYYMM.DD.PATCH format. All Linux VHD versions correspond to a particular tag of the AgentBaker go module. AgentBaker go module tags follow the format v0.YYYYMMDD.PATCH. The mapping between AgentBaker tag and Linux VHD version is defined within [linux_sig_version.json](../pkg/agent/datamodel/linux_sig_version.json).

Windows VHD are released separately, following windows patch tuesday schedule.

## Guidelines

### SRE Guidelines

The operational goals of this project are:

- achieve consistency across different OS as much as possible
- avoid functional regression when introducing new features (component updates, new drivers, new binaries), ensure that all supported OS / versions are tested
- avoid VHD build performance regressions when making any changes
- avoid node provisioning performance regression when making any changes

The SRE guidelines ground other coding guidelines and practices.

### Golang Guidelines

- Follow golang best practice
- Use vanilla go test framework

### PowerShell Guidelines

- follow powershell best practices

### ShellScripts Guidelines

- use shellcheck for sanity checking
- use ShellSpec for testing
- the shell scripts are used on both azure linux/mariner and ubuntu and cross platform portability is critical.
- when using functions defined in other files, ensure it is sourced properly.
- use local variables rather than constants when their scoping allows for it.
- avoid using variables declared inside another function, even they are visible. It is hard to reason and might introduce subtle bugs.
