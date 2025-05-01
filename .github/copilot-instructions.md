# Overview

AgentBaker builds VHDs using Packer for various base OS: Windows, Azure Linux and Ubuntu. For each OS there are various versions (windows 2019, 2022, ubuntu 2004, 2204 etc). The goal is to

- achieve consistency among all builds as much as possible
- avoid function regression when introducing new features (component updates, new drivers, new binaries), ensure that all supported OS / versions are tested
- avoid performance regression when making changes

## AgentBaker Service 

[apiserver](../apiserver/) is go based webserver. It receives request from RP and generates CSE and CustomData to be used on the VHD when a new node is created / provisioned.

## VHD Builder

VHDs are built using [Packer](https://developer.hashicorp.com/packer/docs) in [vhdbuilder](../vhdbuilder/).

## Parts

[parts](../parts/) serves both AgentBaker Service and VHD build. AgentBaker service and VHDs are coupled because of this shared component.

Windows uses a different folder [cse](../staging/cse/windows/) for almost the same purpose. There are subtle differences as windows CSEs can be downloaded as a zip file during provisioning time due to restrictions on the file size on Windows system, while for linux based systems the cse/custom data are dropped in during provisioning time.

## Release

AgentBaker follows a weekky release. All VHDs are built and tagged. New versions of AgentBaker service is deployed. Older versions (up to 6 months) of AgentBaker service are kept for the coupling reason above.

## Coding standing

### Golang Guidelines

- Follow golang best practice
- Use vanilla go test framework

### PowerShell Guidelines

- follow powershell best practices

### ShellScripts Guidelines

- use shellcheck for sanity checking
- use ShellSpec for testing
- the shell scripts are used on both azure linux and ubuntu and cross platform portability is critical.
- when using functions defined in other files, ensure it is sourced properly.
- prefer to avoid sharing variables between functions unless they are environment variables.
