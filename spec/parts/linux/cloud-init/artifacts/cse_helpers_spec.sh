#!/bin/bash
lsb_release() {
    echo "mock lsb_release"
}

readPackage() {
    local packageName=$1
    package=$(jq ".Packages" "./parts/linux/cloud-init/artifacts/components.json" | jq ".[] | select(.name == \"$packageName\")")
    echo "$package"
}

readContainerImage() {
    local containerImageName=$1
    containerImage=$(jq ".ContainerImages" "./parts/linux/cloud-init/artifacts/components.json" | jq ".[] | select(.downloadURL | contains(\"$containerImageName\"))")
    echo "$containerImage"
}

Describe 'cse_helpers.sh'
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Describe 'updatePackageVersions'
        It 'returns downloadURIs.ubuntu."r2004".versions of package runc for UBUNTU 20.04'
            package=$(readPackage "runc")
            When call updatePackageVersions "$package" "UBUNTU" "20.04"
            The variable PACKAGE_VERSIONS[@] should equal "1.1.14-ubuntu20.04u1"
        End

        It 'returns downloadURIs.ubuntu."r2204".versions of package containerd for UBUNTU 22.04'
            package=$(readPackage "containerd")
            When call updatePackageVersions "$package" "UBUNTU" "22.04"
            The variable PACKAGE_VERSIONS[@] should equal "1.7.20"
        End

        It 'returns downloadURIs.ubuntu."r1804".versions of package containerd for UBUNTU 18.04'
            package=$(readPackage "containerd")
            When call updatePackageVersions "$package" "UBUNTU" "18.04"
            The variable PACKAGE_VERSIONS[@] should equal "1.7.1-1"
        End

        It 'returns downloadURIs.default.current.versions of package cni-plugins for UBUNTU 20.04'
            package=$(readPackage "cni-plugins")
            When call updatePackageVersions "$package" "UBUNTU" "20.04"
            The variable PACKAGE_VERSIONS[@] should equal "1.4.1"
        End

        It 'returns downloadURIs.default.current.versions of package azure-cni for UBUNTU 20.04'
            package=$(readPackage "azure-cni")
            When call updatePackageVersions "$package" "UBUNTU" "20.04"
            The variable PACKAGE_VERSIONS[@] should equal "1.4.54 1.5.32 1.6.3 1.5.35"
        End

        It 'returns downloadURIs.mariner.current.versions of package runc for MARINER'
            package=$(readPackage "runc")
            When call updatePackageVersions "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_VERSIONS[@] should equal "1.1.9-5.cm2"
        End

        It 'returns downloadURIs.mariner.current.versions of package containerd for MARINER'
            package=$(readPackage "containerd")
            When call updatePackageVersions "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_VERSIONS[@] should equal "1.6.26-5.cm2"
        End

        It 'returns downloadURIs.default.current.versions of package cni-plugins for MARINER'
            package=$(readPackage "cni-plugins")
            When call updatePackageVersions "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_VERSIONS[@] should equal "1.4.1"
        End

        It 'returns downloadURIs.default.current.versions of package azure-cni for MARINER'
            package=$(readPackage "azure-cni")
            When call updatePackageVersions "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_VERSIONS[@] should equal "1.4.54 1.5.32 1.6.3 1.5.35"
        End
    End
    Describe 'updatePackageDownloadURL'
        It 'returns downloadURIs.ubuntu."r2004".downloadURL of package runc for UBUNTU 20.04'
            package=$(readPackage "runc")
            When call updatePackageDownloadURL "$package" "UBUNTU" "20.04"
            The variable PACKAGE_DOWNLOAD_URL should equal ''
        End

        It 'returns downloadURIs.ubuntu."r2204".downloadURL of package containerd for UBUNTU 22.04'
            package=$(readPackage "containerd")
            When call updatePackageDownloadURL "$package" "UBUNTU" "22.04"
            The variable PACKAGE_DOWNLOAD_URL should equal ''
        End

        It 'returns downloadURIs.ubuntu."r1804".downloadURL of package containerd for UBUNTU 18.04'
            package=$(readPackage "containerd")
            When call updatePackageDownloadURL "$package" "UBUNTU" "18.04"
            The variable PACKAGE_DOWNLOAD_URL should equal ''
        End

        It 'returns downloadURIs.default.current.downloadURL of package cni-plugins for UBUNTU 20.04'
            package=$(readPackage "cni-plugins")
            When call updatePackageDownloadURL "$package" "UBUNTU" "20.04"
            The variable PACKAGE_DOWNLOAD_URL should equal "https://acs-mirror.azureedge.net/cni-plugins/v\${version}/binaries/cni-plugins-linux-\${CPU_ARCH}-v\${version}.tgz"
        End

        It 'returns downloadURIs.default.current.downloadURL of package azure-cni for UBUNTU 20.04'
            package=$(readPackage "azure-cni")
            When call updatePackageDownloadURL "$package" "UBUNTU" "20.04"
            The variable PACKAGE_DOWNLOAD_URL should equal "https://acs-mirror.azureedge.net/azure-cni/v\${version}/binaries/azure-vnet-cni-linux-\${CPU_ARCH}-v\${version}.tgz"
        End

        It 'returns downloadURIs.mariner.current.downloadURL of package runc for MARINER'
            package=$(readPackage "runc")
            When call updatePackageDownloadURL "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_DOWNLOAD_URL should equal ''
        End

        It 'returns downloadURIs.mariner.current.downloadURL of package containerd for MARINER'
            package=$(readPackage "containerd")
            When call updatePackageDownloadURL "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_DOWNLOAD_URL should equal ''
        End

        It 'returns downloadURIs.default.current.downloadURL of package cni-plugins for MARINER'
            package=$(readPackage "cni-plugins")
            When call updatePackageDownloadURL "$package" "MARINER" "some_mariner_version"
            The variable PACKAGE_DOWNLOAD_URL should equal "https://acs-mirror.azureedge.net/cni-plugins/v\${version}/binaries/cni-plugins-linux-\${CPU_ARCH}-v\${version}.tgz"
        End

        It 'returns downloadURIs.default.current.downloadURL of package azure-cni for MARINER'
            package=$(readPackage "azure-cni")
            When call updatePackageDownloadURL "$package" "MARINER" "some_mariner_version"
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
    Describe 'updateRelease'
        It 'returns release version r2004 for package runc in UBUNTU 20.04'
            package=$(readPackage "runc")
            os="UBUNTU"
            osVersion="20.04"
            When call updateRelease "$package" "$os" "$osVersion"
            The variable RELEASE should equal "\"r2004\""
        End
        It 'returns release version current for package runc in Mariner'
            package=$(readPackage "runc")
            os="MARINER"
            osVersion=""
            When call updateRelease "$package" "$os" "$osVersion"
            The variable RELEASE should equal "current"
        End
        It 'returns release version current for package containerd in UBUNTU 20.04'
            package=$(readPackage "containerd")
            os="UBUNTU"
            osVersion="20.04"
            When call updateRelease "$package" "$os" "$osVersion"
            The variable RELEASE should equal "\"r2004\""
        End
        It 'returns release version r1804 for package containerd in UBUNTU 18.04'
            package=$(readPackage "containerd")
            os="UBUNTU"
            osVersion="18.04"
            When call updateRelease "$package" "$os" "$osVersion"
            The variable RELEASE should equal "\"r1804\""
        End
    End
    Describe 'updateMultiArchVersions'
        It 'returns multiArchVersionsV2 for containerImage kube-proxy'
            containerImage=$(readContainerImage "kube-proxy")
            When call updateMultiArchVersions "$containerImage"
            The variable MULTI_ARCH_VERSIONS[@] should equal "v1.27.16 v1.28.14 v1.29.9 v1.30.5 v1.27.15 v1.28.13 v1.29.8 v1.30.4"
        End
    End
End