#!/bin/bash
lsb_release() {
    echo "mock lsb_release"
}

readPackage() {
    local packageName=$1
    package=$(jq ".Packages" "./parts/linux/cloud-init/artifacts/components.json" | jq ".[] | select(.name == \"$packageName\")")
    echo "$package"
}

Describe 'cse_install.sh'
    Include "./parts/linux/cloud-init/artifacts/cse_install.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Describe 'returnPackageVersions'
        It 'returns downloadURIs.ubuntu."r2004".versions of package runc for UBUNTU 20.04'
            package=$(readPackage "runc")
            When call returnPackageVersions "$package" "UBUNTU" "20.04"
            The variable PACKAGE_VERSIONS[@] should equal "1.1.12-ubuntu20.04u1"
        End

        It 'returns downloadURIs.ubuntu."r2204".versions of package containerd for UBUNTU 22.04'
            package=$(readPackage "containerd")
            When call returnPackageVersions "$package" "UBUNTU" "22.04"
            The variable PACKAGE_VERSIONS[@] should equal "1.7.20"
        End

        It 'returns downloadURIs.ubuntu."r1804".versions of package containerd for UBUNTU 18.04'
            package=$(readPackage "containerd")
            When call returnPackageVersions "$package" "UBUNTU" "18.04"
            The variable PACKAGE_VERSIONS[@] should equal "1.7.1-1"
        End

        It 'returns downloadURIs.default.current.versions of package cni-plugins for UBUNTU 20.04'
            package=$(readPackage "cni-plugins")
            When call returnPackageVersions "$package" "UBUNTU" "20.04"
            The variable PACKAGE_VERSIONS[@] should equal "1.4.1"
        End

        It 'returns downloadURIs.default.current.versions of package azure-cni for UBUNTU 20.04'
            package=$(readPackage "azure-cni")
            When call returnPackageVersions "$package" "UBUNTU" "20.04"
            The variable PACKAGE_VERSIONS[@] should equal "1.4.54 1.5.28"
        End

        It 'returns downloadURIs.mariner.current.versions of package runc for MARINER'
            package=$(readPackage "runc")
            When call returnPackageVersions "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_VERSIONS[@] should equal "1.1.9-5.cm2"
        End

        It 'returns downloadURIs.mariner.current.versions of package containerd for MARINER'
            package=$(readPackage "containerd")
            When call returnPackageVersions "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_VERSIONS[@] should equal "1.6.26-5.cm2"
        End

        It 'returns downloadURIs.default.current.versions of package cni-plugins for MARINER'
            package=$(readPackage "cni-plugins")
            When call returnPackageVersions "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_VERSIONS[@] should equal "1.4.1"
        End

        It 'returns downloadURIs.default.current.versions of package azure-cni for MARINER'
            package=$(readPackage "azure-cni")
            When call returnPackageVersions "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_VERSIONS[@] should equal "1.4.54 1.5.28"
        End
    End
    Describe 'returnPackageDownloadURL'
        It 'returns downloadURIs.ubuntu."r2004".downloadURL of package runc for UBUNTU 20.04'
            package=$(readPackage "runc")
            When call returnPackageDownloadURL "$package" "UBUNTU" "20.04"
            The variable PACKAGE_DOWNLOAD_URL should equal ''
        End

        It 'returns downloadURIs.ubuntu."r2204".downloadURL of package containerd for UBUNTU 22.04'
            package=$(readPackage "containerd")
            When call returnPackageDownloadURL "$package" "UBUNTU" "22.04"
            The variable PACKAGE_DOWNLOAD_URL should equal ''
        End

        It 'returns downloadURIs.ubuntu."r1804".downloadURL of package containerd for UBUNTU 18.04'
            package=$(readPackage "containerd")
            When call returnPackageDownloadURL "$package" "UBUNTU" "18.04"
            The variable PACKAGE_DOWNLOAD_URL should equal ''
        End

        It 'returns downloadURIs.default.current.downloadURL of package cni-plugins for UBUNTU 20.04'
            package=$(readPackage "cni-plugins")
            When call returnPackageDownloadURL "$package" "UBUNTU" "20.04"
            The variable PACKAGE_DOWNLOAD_URL should equal "https://acs-mirror.azureedge.net/cni-plugins/v\${version}/binaries/cni-plugins-linux-\${CPU_ARCH}-v\${version}.tgz"
        End

        It 'returns downloadURIs.default.current.downloadURL of package azure-cni for UBUNTU 20.04'
            package=$(readPackage "azure-cni")
            When call returnPackageDownloadURL "$package" "UBUNTU" "20.04"
            The variable PACKAGE_DOWNLOAD_URL should equal "https://acs-mirror.azureedge.net/azure-cni/v\${version}/binaries/azure-vnet-cni-linux-\${CPU_ARCH}-v\${version}.tgz"
        End

        It 'returns downloadURIs.mariner.current.downloadURL of package runc for MARINER'
            package=$(readPackage "runc")
            When call returnPackageDownloadURL "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_DOWNLOAD_URL should equal ''
        End

        It 'returns downloadURIs.mariner.current.downloadURL of package containerd for MARINER'
            package=$(readPackage "containerd")
            When call returnPackageDownloadURL "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_DOWNLOAD_URL should equal ''
        End

        It 'returns downloadURIs.default.current.downloadURL of package cni-plugins for MARINER'
            package=$(readPackage "cni-plugins")
            When call returnPackageDownloadURL "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_DOWNLOAD_URL should equal "https://acs-mirror.azureedge.net/cni-plugins/v\${version}/binaries/cni-plugins-linux-\${CPU_ARCH}-v\${version}.tgz"
        End

        It 'returns downloadURIs.default.current.downloadURL of package azure-cni for MARINER'
            package=$(readPackage "azure-cni")
            When call returnPackageDownloadURL "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_DOWNLOAD_URL should equal "https://acs-mirror.azureedge.net/azure-cni/v\${version}/binaries/azure-vnet-cni-linux-\${CPU_ARCH}-v\${version}.tgz"
        End
    End
    Describe 'evalPackageDownloadURL'
        It 'returns empty string for empty downloadURL'
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
        COMPONENTS_FILEPATH="./parts/linux/cloud-init/artifacts/components.json"
        It 'returns expected output for successful installation of containerd in UBUNTU 20.04'
            UBUNTU_RELEASE="20.04"
            containerdPackage=$(readPackage "containerd")
            When call installContainerRuntime 
            The variable containerdMajorMinorPatchVersion should equal "1.7.20"
            The variable containerdHotFixVersion should equal ""
            The output line 3 should equal "mock logs to events calling with AKS.CSE.installContainerRuntime.installStandaloneContainerd"
            The output line 4 should equal "in installContainerRuntime - CONTAINERD_VERSION = 1.7.20"
        End
        It 'returns expected output for successful installation of containerd in Mariner'
            UBUNTU_RELEASE="" # mocking Mariner doesn't have command `lsb_release -cs`
            containerdPackage=$(readPackage "containerd")
            When call installContainerRuntime 
            The variable containerdMajorMinorPatchVersion should equal "1.6.26"
            The variable containerdHotFixVersion should equal "5.cm2"
            The output line 3 should equal "mock logs to events calling with AKS.CSE.installContainerRuntime.installStandaloneContainerd"
            The output line 4 should equal "in installContainerRuntime - CONTAINERD_VERSION = 1.6.26-5.cm2"
        End
        It 'exits with error if components.json file is not found'
            COMPONENTS_FILEPATH="non_existent_file.json"
            When run installContainerRuntime 
            The status should equal $ERR_CONTAINERD_VERSION_INVALID
            The output line 2 should equal "Unexpected. Package \"containerd\" does not exist in non_existent_file.json."
        End
    End
    Describe 'returnRelease'
        It 'returns release version r2004 for package runc in UBUNTU 20.04'
            package=$(readPackage "runc")
            os="UBUNTU"
            osVersion="20.04"
            When call returnRelease "$package" "$os" "$osVersion"
            The variable RELEASE should equal "\"r2004\""
        End
        It 'returns release version current for package runc in Mariner'
            package=$(readPackage "runc")
            os="MARINER"
            osVersion=""
            When call returnRelease "$package" "$os" "$osVersion"
            The variable RELEASE should equal "current"
        End
        It 'returns release version current for package containerd in UBUNTU 20.04'
            package=$(readPackage "containerd")
            os="UBUNTU"
            osVersion="20.04"
            When call returnRelease "$package" "$os" "$osVersion"
            The variable RELEASE should equal "\"r2004\""
        End
        It 'returns release version r1804 for package containerd in UBUNTU 18.04'
            package=$(readPackage "containerd")
            os="UBUNTU"
            osVersion="18.04"
            When call returnRelease "$package" "$os" "$osVersion"
            The variable RELEASE should equal "\"r1804\""
        End
    End
    Describe 'returnPkgVersionsOrVersions'
        It 'returns pkgVersions for package kubernetes-binaries in default.current'
            package=$(readPackage "kubernetes-binaries")
            os="default"
            release="current"
            When call returnPkgVersionsOrVersions "$package" "$os" "$release"
            The variable PACKAGE_VERSIONS[@] should equal "1.27.16 1.28.12 1.29.7 1.30.3 1.27.15 1.28.11 1.29.6 1.30.2"
        End
        It 'returns pkgVersions for package containerd in Ubuntu 22.04'
            package=$(readPackage "containerd")
            os="ubuntu"
            release="r2204"
            When call returnPkgVersionsOrVersions "$package" "$os" "$release"
            The variable PACKAGE_VERSIONS[@] should equal "1.7.20"
        End
    End
End