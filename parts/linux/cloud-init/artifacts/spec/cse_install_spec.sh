Describe 'cse_install.sh'
  Include ./cse_install.sh
  Describe 'returnPackageVersions'
    readPackage() {
        local packageName=$1
        packages=$(jq ".Packages" "./spec/test_components.json" | jq ".[] | select(.name == \"$packageName\")")
        echo "$packages"
    }

    It 'returns downloadURIs.ubuntu.current.versions of package runc for UBUNTU 20.04'
        package=$(readPackage "runc")
        When call returnPackageVersions "$package" "UBUNTU" "20.04"
        The line 1 of output should equal '[ "1.1.12" ]'
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

    It 'returns downloadURIs.default.current.versions of package cni-plugins for UBUNTU'
        package=$(readPackage "cni-plugins")
        When call returnPackageVersions "$package" "UBUNTU" "20.04"
        The line 1 of output should equal '[ "1.4.1" ]'
    End

    It 'returns downloadURIs.default.current.versions of package azure-cni for UBUNTU'
        package=$(readPackage "azure-cni")
        When call returnPackageVersions "$package" "UBUNTU" "20.04"
        The line 1 of output should equal '[ "1.4.54", "1.5.28" ]'
    End

  End
End