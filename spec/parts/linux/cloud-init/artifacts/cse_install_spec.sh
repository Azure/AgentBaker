#!/bin/bash
readPackage() {
    local packageName=$1
    packages=$(jq ".Packages" "./spec/parts/linux/cloud-init/artifacts/test_components.json" | jq ".[] | select(.name == \"$packageName\")")
    echo "$packages"
}

Describe 'cse_install.sh'
  Include "./parts/linux/cloud-init/artifacts/cse_install.sh"
  Describe 'returnPackageVersions'
    It 'returns downloadURIs.ubuntu."r2004".versions of package runc for UBUNTU 20.04'
        package=$(readPackage "runc")
        When call returnPackageVersions "$package" "UBUNTU" "20.04"
        The variable PackageVersions[@] should equal "1.1.12-ubuntu20.04u1"
    End

    It 'returns downloadURIs.ubuntu.current.versions of package containerd for UBUNTU 22.04'
        package=$(readPackage "containerd")
        When call returnPackageVersions "$package" "UBUNTU" "22.04"
        The variable PackageVersions[@] should equal "1.7.15-1"
    End

    It 'returns downloadURIs.ubuntu."r1804".versions of package containerd for UBUNTU 18.04'
        package=$(readPackage "containerd")
        When call returnPackageVersions "$package" "UBUNTU" "18.04"
        The variable PackageVersions[@] should equal "1.7.1-1"
    End

    It 'returns downloadURIs.default.current.versions of package cni-plugins for UBUNTU 20.04'
        package=$(readPackage "cni-plugins")
        When call returnPackageVersions "$package" "UBUNTU" "20.04"
        The variable PackageVersions[@] should equal "1.4.1"
    End

    It 'returns downloadURIs.default.current.versions of package azure-cni for UBUNTU 20.04'
        package=$(readPackage "azure-cni")
        When call returnPackageVersions "$package" "UBUNTU" "20.04"
        The variable PackageVersions[@] should equal "1.4.54 1.5.28"
    End

    It 'returns downloadURIs.mariner.current.versions of package runc for MARINER'
        package=$(readPackage "runc")
        When call returnPackageVersions "$package" "MARINER" "some_mariner_version"
        The variable PackageVersions[@] should equal "1.4.54"
    End

    It 'returns downloadURIs.mariner.current.versions of package containerd for MARINER'
        package=$(readPackage "containerd")
        When call returnPackageVersions "$package" "MARINER" "some_mariner_version"
        The variable PackageVersions[@] should equal "1.3.4"
    End

    It 'returns downloadURIs.default.current.versions of package cni-plugins for MARINER'
        package=$(readPackage "cni-plugins")
        When call returnPackageVersions "$package" "MARINER" "some_mariner_version"
        The variable PackageVersions[@] should equal "1.4.1"
    End

    It 'returns downloadURIs.default.current.versions of package azure-cni for MARINER'
        package=$(readPackage "azure-cni")
        When call returnPackageVersions "$package" "MARINER" "some_mariner_version"
        The variable PackageVersions[@] should equal "1.4.54 1.5.28"
    End
  End
  Describe 'returnPackageDownloadURL'
    It 'returns downloadURIs.ubuntu."r2004".downloadURL of package runc for UBUNTU 20.04'
        package=$(readPackage "runc")
        When call returnPackageDownloadURL "$package" "UBUNTU" "20.04"
        The output should equal 'ubuntu_2004_runc_download_url'
    End

    It 'returns downloadURIs.ubuntu."r2204".downloadURL of package containerd for UBUNTU 22.04'
        package=$(readPackage "containerd")
        When call returnPackageDownloadURL "$package" "UBUNTU" "22.04"
        The output should equal 'ubuntu_current_containerd_download_url'
    End

    It 'returns downloadURIs.ubuntu."r1804".downloadURL of package containerd for UBUNTU 18.04'
        package=$(readPackage "containerd")
        When call returnPackageDownloadURL "$package" "UBUNTU" "18.04"
        The output should equal 'ubuntu_1804_containerd_download_url'
    End

    It 'returns downloadURIs.default.current.downloadURL of package cni-plugins for UBUNTU 20.04'
        package=$(readPackage "cni-plugins")
        When call returnPackageDownloadURL "$package" "UBUNTU" "20.04"
        The output should equal "https://acs-mirror.azureedge.net/cni-plugins/v\${version}/binaries/cni-plugins-linux-\${CPU_ARCH}-v\${version}.tgz"
    End

    It 'returns downloadURIs.default.current.downloadURL of package azure-cni for UBUNTU 20.04'
        package=$(readPackage "azure-cni")
        When call returnPackageDownloadURL "$package" "UBUNTU" "20.04"
        The output should equal "https://acs-mirror.azureedge.net/azure-cni/v\${version}/binaries/azure-vnet-cni-linux-\${CPU_ARCH}-v\${version}.tgz"
    End

    It 'returns downloadURIs.mariner.current.downloadURL of package runc for MARINER'
        package=$(readPackage "runc")
        When call returnPackageDownloadURL "$package" "MARINER" "some_mariner_version"
        The output should equal 'mariner_current_runc_download_url'
    End

    It 'returns downloadURIs.mariner.current.downloadURL of package containerd for MARINER'
        package=$(readPackage "containerd")
        When call returnPackageDownloadURL "$package" "MARINER" "some_mariner_version"
        The output should equal 'mariner_containerd_download_url'
    End

    It 'returns downloadURIs.default.current.downloadURL of package cni-plugins for MARINER'
        package=$(readPackage "cni-plugins")
        When call returnPackageDownloadURL "$package" "MARINER" "some_mariner_version"
        The output should equal "https://acs-mirror.azureedge.net/cni-plugins/v\${version}/binaries/cni-plugins-linux-\${CPU_ARCH}-v\${version}.tgz"
    End

    It 'returns downloadURIs.default.current.downloadURL of package azure-cni for MARINER'
        package=$(readPackage "azure-cni")
        When call returnPackageDownloadURL "$package" "MARINER" "some_mariner_version"
        The output should equal "https://acs-mirror.azureedge.net/azure-cni/v\${version}/binaries/azure-vnet-cni-linux-\${CPU_ARCH}-v\${version}.tgz"
    End
  End
  Describe 'evalPackageDownloadURL'
    It 'returns returns empty string for empty downloadURL'
        When call evalPackageDownloadURL ""
        The output should equal ""
    End
    It 'returns evaluated downloadURL of package azure-cni'
        version="0.0.1"
        CPU_ARCH="amd64"
        When call evalPackageDownloadURL "https://acs-mirror.azureedge.net/azure-cni/v\${version}/binaries/azure-vnet-cni-linux-\${CPU_ARCH}-v\${version}.tgz"
        The output should equal 'https://acs-mirror.azureedge.net/azure-cni/v0.0.1/binaries/azure-vnet-cni-linux-amd64-v0.0.1.tgz'
    End
  End
  Describe 'installContainerRuntime'
    logs_to_events() {
        echo "mock logs to events calling with $1"
    }
    NEEDS_CONTAINERD="true"
    UBUNTU_RELEASE="20.04"
    containerdPackage=$(readPackage "containerd")
    COMPONENTS_FILEPATH="./spec/parts/linux/cloud-init/artifacts/test_components.json"
    It 'returns expected output for successful installation of containerd'
        When call installContainerRuntime 
        The variable containerdMajorMinorPatchVersion should equal "1.7.15"
        The variable containerdHotFixVersion should equal "1"
        The output line 2 should equal "mock logs to events calling with AKS.CSE.installContainerRuntime.installStandaloneContainerd"
        The output line 3 should equal "in installContainerRuntime - CONTAINERD_VERSION = 1.7.15"
    End
  End
End