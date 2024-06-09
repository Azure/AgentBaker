Describe 'cse_install.sh'
  Include ./cse_install.sh
  Describe 'returnPackageVersions'
    readPackage() {
        local packageName=$1
        packages=$(jq ".Packages" "./spec/test_components.json" | jq ".[] | select(.name == \"$packageName\")")
        echo "$packages"
    }

    It 'returns downloadURIs.ubuntu."2004".versions of package runc for UBUNTU 20.04'
        package=$(readPackage "runc")
        When call returnPackageVersions "$package" "UBUNTU" "20.04"
        The line 1 of output should equal '[ "1.1.12-ubuntu20.04u1" ]'
    End

    It 'returns downloadURIs.ubuntu.current.versions of package containerd for UBUNTU 22.04'
        package=$(readPackage "containerd")
        When call returnPackageVersions "$package" "UBUNTU" "22.04"
        The line 1 of output should equal '[ "1.7.15-1" ]'
    End

    It 'returns downloadURIs.ubuntu."1804".versions of package containerd for UBUNTU 18.04'
        package=$(readPackage "containerd")
        When call returnPackageVersions "$package" "UBUNTU" "18.04"
        The line 1 of output should equal '[ "1.7.1-1" ]'
    End

    It 'returns downloadURIs.default.current.versions of package cni-plugins for UBUNTU 20.04'
        package=$(readPackage "cni-plugins")
        When call returnPackageVersions "$package" "UBUNTU" "20.04"
        The line 1 of output should equal '[ "1.4.1" ]'
    End

    It 'returns downloadURIs.default.current.versions of package azure-cni for UBUNTU 20.04'
        package=$(readPackage "azure-cni")
        When call returnPackageVersions "$package" "UBUNTU" "20.04"
        The line 1 of output should equal '[ "1.4.54", "1.5.28" ]'
    End

    It 'returns downloadURIs.mariner.current.versions of package runc for MARINER'
        package=$(readPackage "runc")
        When call returnPackageVersions "$package" "MARINER" "some_mariner_version"
        The line 1 of output should equal '[ "1.4.54" ]'
    End

    It 'returns downloadURIs.mariner.current.versions of package containerd for MARINER'
        package=$(readPackage "containerd")
        When call returnPackageVersions "$package" "MARINER" "some_mariner_version"
        The line 1 of output should equal '[ "1.3.4" ]'
    End

    It 'returns downloadURIs.default.current.versions of package cni-plugins for MARINER'
        package=$(readPackage "cni-plugins")
        When call returnPackageVersions "$package" "MARINER" "some_mariner_version"
        The line 1 of output should equal '[ "1.4.1" ]'
    End

    It 'returns downloadURIs.default.current.versions of package azure-cni for MARINER'
        package=$(readPackage "azure-cni")
        When call returnPackageVersions "$package" "MARINER" "some_mariner_version"
        The variable PackageVersions[@] should equal "1.4.54 1.5.28"
    End
  End
  Describe 'returnPackageDownloadURL'
    readPackage() {
        local packageName=$1
        packages=$(jq ".Packages" "./spec/test_components.json" | jq ".[] | select(.name == \"$packageName\")")
        echo "$packages"
    }

    It 'returns downloadURIs.ubuntu."2004".downloadURL of package runc for UBUNTU 20.04'
        package=$(readPackage "runc")
        When call returnPackageDownloadURL "$package" "UBUNTU" "20.04"
        The line 1 of output should equal 'ubuntu_2004_runc_download_url'
    End

    It 'returns downloadURIs.ubuntu."2204".downloadURL of package containerd for UBUNTU 22.04'
        package=$(readPackage "containerd")
        When call returnPackageDownloadURL "$package" "UBUNTU" "22.04"
        The line 1 of output should equal 'ubuntu_current_containerd_download_url'
    End

    It 'returns downloadURIs.ubuntu."1804".downloadURL of package containerd for UBUNTU 18.04'
        package=$(readPackage "containerd")
        When call returnPackageDownloadURL "$package" "UBUNTU" "18.04"
        The line 1 of output should equal 'ubuntu_1804_containerd_download_url'
    End

    It 'returns downloadURIs.default.current.downloadURL of package cni-plugins for UBUNTU 20.04'
        package=$(readPackage "cni-plugins")
        When call returnPackageDownloadURL "$package" "UBUNTU" "20.04"
        The line 1 of output should equal 'https://acs-mirror.azureedge.net/cni-plugins/v${version}/binaries/cni-plugins-linux-${CPU_ARCH}-v${version}.tgz'
    End

    It 'returns downloadURIs.default.current.downloadURL of package azure-cni for UBUNTU 20.04'
        package=$(readPackage "azure-cni")
        When call returnPackageDownloadURL "$package" "UBUNTU" "20.04"
        The line 1 of output should equal 'https://acs-mirror.azureedge.net/azure-cni/v${version}/binaries/azure-vnet-cni-linux-${CPU_ARCH}-v${version}.tgz'
    End

    It 'returns downloadURIs.mariner.current.downloadURL of package runc for MARINER'
        package=$(readPackage "runc")
        When call returnPackageDownloadURL "$package" "MARINER" "some_mariner_version"
        The line 1 of output should equal 'mariner_current_runc_download_url'
    End

    It 'returns downloadURIs.mariner.current.downloadURL of package containerd for MARINER'
        package=$(readPackage "containerd")
        When call returnPackageDownloadURL "$package" "MARINER" "some_mariner_version"
        The line 1 of output should equal 'mariner_containerd_download_url'
    End

    It 'returns downloadURIs.default.current.downloadURL of package cni-plugins for MARINER'
        package=$(readPackage "cni-plugins")
        When call returnPackageDownloadURL "$package" "MARINER" "some_mariner_version"
        The line 1 of output should equal 'https://acs-mirror.azureedge.net/cni-plugins/v${version}/binaries/cni-plugins-linux-${CPU_ARCH}-v${version}.tgz'
    End

    It 'returns downloadURIs.default.current.downloadURL of package azure-cni for MARINER'
        package=$(readPackage "azure-cni")
        When call returnPackageDownloadURL "$package" "MARINER" "some_mariner_version"
        The line 1 of output should equal 'https://acs-mirror.azureedge.net/azure-cni/v${version}/binaries/azure-vnet-cni-linux-${CPU_ARCH}-v${version}.tgz'
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
        When call evalPackageDownloadURL 'https://acs-mirror.azureedge.net/azure-cni/v${version}/binaries/azure-vnet-cni-linux-${CPU_ARCH}-v${version}.tgz'
        The output should equal 'https://acs-mirror.azureedge.net/azure-cni/v0.0.1/binaries/azure-vnet-cni-linux-amd64-v0.0.1.tgz'
    End
  End
End