#!/bin/bash
lsb_release() {
    echo "mock lsb_release"
}

readPackage() {
    local packageName=$1
    package=$(jq ".Packages" "spec/parts/linux/cloud-init/artifacts/test_components.json" | jq ".[] | select(.name == \"$packageName\")")
    echo "$package"
}

readContainerImage() {
    local containerImageName=$1
    containerImage=$(jq ".ContainerImages" "spec/parts/linux/cloud-init/artifacts/test_components.json" | jq ".[] | select(.downloadURL | contains(\"$containerImageName\"))")
    echo "$containerImage"
}

Describe 'cse_helpers.sh'
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Describe 'updatePackageVersions'
        It 'returns downloadURIs.ubuntu.r2204.versionsV2 of package pkgVersionsV2 for UBUNTU 22.04'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "UBUNTU" "22.04"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersion1 dummyVersion0.9"
        End
        It 'returns downloadURIs.ubuntu.r2004.versionsV2 of package pkgVersionsV2 for UBUNTU 20.04'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "UBUNTU" "20.04"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersion2"
        End
        It 'returns downloadURIs.ubuntu.r1804.versionsV2 of package pkgVersionsV2 for UBUNTU 18.04'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "UBUNTU" "18.04"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersion3 dummyVersion4"
        End
        It 'returns downloadURIs.mariner.current.versionsV2 of package pkgVersionsV2 for MARINER current'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "MARINER" "current"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersion5 dummyVersion6.1 dummyVersion6.0"
        End
        It 'returns <SKIP> if there is a <SKIP> in latestVersion'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "MARINERKATA" "current"
            The variable PACKAGE_VERSIONS[@] should equal "<SKIP>"
        End
        It 'returns downloadURIs.default.current.versionsV2 of package pkgVersionsV2 for unknown OS distro'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "unknownOS" "some_mariner_version"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersion7"
        End
        It 'returns downloadURIs.ubuntu.current.versionsV2 of package pkgVersionsV2 for UBUNTU unkown release'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageVersions "$package" "UBUNTU" "unknown_release"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersionCurrent"
        End
        It 'returns downloadURIs.default.current.versions of package pkgVersions for default.current as a fallback case'
            package=$(readPackage "pkgVersions")
            When call updatePackageVersions "$package" "default" "current"
            The variable PACKAGE_VERSIONS[@] should equal "dummyVersionFallback1.1 dummyVersionFallback1.0"
        End
    End
    Describe 'updatePackageDownloadURL'
        It 'returns downloadURIs.ubuntu.r2204.downloadURL of package pkgVersionsV2 for ubuntu.r2204'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageDownloadURL "$package" "UBUNTU" "22.04"
            The variable PACKAGE_DOWNLOAD_URL should equal "https://dummypath/v\${version}/dummy_\${version}_linux_\${CPU_ARCH}.tar.gz"
        End
        It 'returns downloadURIs.ubuntu.current.downloadURL of package pkgVersionsV2 for ubuntu unknown release'
            package=$(readPackage "pkgVersionsV2")
            When call updatePackageDownloadURL "$package" "UBUNTU" "dummy_release"
            The variable PACKAGE_DOWNLOAD_URL should equal "https://dummydefaultcurrentpath/v\${version}/dummy_\${version}_linux_\${CPU_ARCH}.tar.gz"
        End
    End
    Describe 'evalPackageDownloadURL'
        It 'returns empty string for empty downloadURL'
            When call evalPackageDownloadURL ""
            The output should equal ""
        End
        It 'returns evaluated downloadURL of package dummypath'
            version="0.0.1"
            CPU_ARCH="amd64"
            When call evalPackageDownloadURL "https://dummypath/v\${version}/binaries/dummypath-linux-\${CPU_ARCH}-v\${version}.tgz"
            The output should equal 'https://dummypath/v0.0.1/binaries/dummypath-linux-amd64-v0.0.1.tgz'
        End
    End
    Describe 'pkgVersionsV2'
        It 'returns release version r2004 for package pkgVersionsV2 in UBUNTU 20.04'
            package=$(readPackage "pkgVersionsV2")
            os="UBUNTU"
            osVersion="20.04"
            When call updateRelease "$package" "$os" "$osVersion"
            The variable RELEASE should equal "\"r2004\""
        End
        It 'returns release version current for package pkgVersionsV2 in Mariner.uknown_release'
            package=$(readPackage "pkgVersionsV2")
            os="MARINER"
            osVersion="uknown_release"
            When call updateRelease "$package" "$os" "$osVersion"
            The variable RELEASE should equal "current"
        End
    End
    Describe 'updateMultiArchVersions'
        It 'returns multiArchVersionsV2 for containerImage mcr.microsoft.com/dummyImageWithMultiArchVersionsV2'
            containerImage=$(readContainerImage "mcr.microsoft.com/dummyImageWithMultiArchVersionsV2")
            When call updateMultiArchVersions "$containerImage"
            The variable MULTI_ARCH_VERSIONS[@] should equal "dummyVersion1.1 dummyVersion2.1 dummyVersion1 dummyVersion2"
        End
        It 'returns multiArchVersions for containerImage mcr.microsoft.com/dummyImageWithOldMultiArchVersions'
            containerImage=$(readContainerImage "mcr.microsoft.com/dummyImageWithOldMultiArchVersions")
            When call updateMultiArchVersions "$containerImage"
            The variable MULTI_ARCH_VERSIONS[@] should equal "dummyVersion3 dummyVersion4 dummyVersion5"
        End
    End
End
