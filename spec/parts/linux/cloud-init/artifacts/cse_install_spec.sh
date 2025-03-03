#!/bin/bash
lsb_release() {
    echo "mock lsb_release"
}

readPackage() {
    local packageName=$1
    package=$(jq ".Packages" "spec/parts/linux/cloud-init/artifacts/test_components.json" | jq ".[] | select(.name == \"$packageName\")")
    echo "$package"
}

Describe 'cse_install.sh'
    Include "./parts/linux/cloud-init/artifacts/cse_install.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Describe 'installContainerRuntime'
        logs_to_events() {
            echo "mock logs to events calling with $1"
        }
        NEEDS_CONTAINERD="true"
        COMPONENTS_FILEPATH="spec/parts/linux/cloud-init/artifacts/test_components.json"
        It 'returns expected output for successful installation of fake containerd in UBUNTU 20.04'
            UBUNTU_RELEASE="20.04"
            containerdPackage=$(readPackage "containerd")
            When call installContainerRuntime 
            The variable containerdMajorMinorPatchVersion should equal "1.2.3"
            The variable containerdHotFixVersion should equal ""
            The output line 3 should equal "mock logs to events calling with AKS.CSE.installContainerRuntime.installStandaloneContainerd"
            The output line 4 should equal "in installContainerRuntime - CONTAINERD_VERSION = 1.2.3"
        End
        It 'returns expected output for successful installation of containerd in Mariner'
            UBUNTU_RELEASE="" # mocking Mariner doesn't have command `lsb_release -cs`
            OS="MARINER"
            containerdPackage=$(readPackage "containerd")
            When call installContainerRuntime 
            The variable containerdMajorMinorPatchVersion should equal "1.2.3"
            The variable containerdHotFixVersion should equal "5.fake"
            The output line 3 should equal "mock logs to events calling with AKS.CSE.installContainerRuntime.installStandaloneContainerd"
            The output line 4 should equal "in installContainerRuntime - CONTAINERD_VERSION = 1.2.3-5.fake"
        End
        It 'skips the containerd installation for Mariner with Kata'
            UBUNTU_RELEASE="" # mocking Mariner doesn't have command `lsb_release -cs`
            OS="MARINER"
            containerdPackage=$(readPackage "containerd")
            IS_KATA="true"
            When call installContainerRuntime
            The output line 3 should equal "INFO: containerd package versions array is either empty or the first element is <SKIP>. Skipping containerd installation."   
        End         
        It 'returns expected output for successful installation of containerd in AzureLinux'
            UBUNTU_RELEASE="" # mocking AzureLinux doesn't have command `lsb_release -cs`
            OS="AZURELINUX"
            containerdPackage=$(readPackage "containerd")
            When call installContainerRuntime
            The variable containerdMajorMinorPatchVersion should equal "2.0.0"
            The variable containerdHotFixVersion should equal "1.fake"
            The output line 3 should equal "mock logs to events calling with AKS.CSE.installContainerRuntime.installStandaloneContainerd"
            The output line 4 should equal "in installContainerRuntime - CONTAINERD_VERSION = 2.0.0-1.fake"
        End
        It 'skips validation if components.json file is not found'
            COMPONENTS_FILEPATH="non_existent_file.json"
            installContainerdWithManifestJson() {
                echo "mock installContainerdWithManifestJson calling"
            }
            When call installContainerRuntime 
            The output line 2 should equal "Package \"containerd\" does not exist in $COMPONENTS_FILEPATH."
            The output line 3 should equal "mock installContainerdWithManifestJson calling"
        End
    End
    Describe 'getInstallModeAndCleanupContainerImages'
        logs_to_events() {
            echo "mock logs to events calling with $1"
        }

        VHD_LOGS_FILEPATH="non_existent_file.txt"
        setup() {
            touch "$VHD_LOGS_FILEPATH"
        }
        cleanup() {
            rm -f "$VHD_LOGS_FILEPATH"
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'should skip binary cleanup if SKIP_BINARY_CLEANUP is true'
            SKIP_BINARY_CLEANUP="true"
            IS_VHD="false"
            When run getInstallModeAndCleanupContainerImages $SKIP_BINARY_CLEANUP $IS_VHD
            The stdout should include "binaries will not be cleaned up"
            The 
        End

        It 'should cleanup container images if SKIP_BINARY_CLEANUP is false and VHD_LOGS_FILEPATH exists'
            SKIP_BINARY_CLEANUP="false"
            IS_VHD="false"
            cleanUpContainerImages() {
                echo "mock cleanUpContainerImages calling"
            }
            When run getInstallModeAndCleanupContainerImages $SKIP_BINARY_CLEANUP $IS_VHD
            The stdout should include "detected golden image pre-install"
            The stdout should include "mock logs to events calling with AKS.CSE.cleanUpContainerImages"
        End

        It 'should return true if VHD_LOGS_FILEPATH does not exist and IS_VHD is false'
            SKIP_BINARY_CLEANUP="false"
            IS_VHD="false"
            VHD_LOGS_FILEPATH="dummy-not-existant-file.txt"
            When run getInstallModeAndCleanupContainerImages $SKIP_BINARY_CLEANUP $IS_VHD
            The stdout should include "the file $VHD_LOGS_FILEPATH does not exist and IS_VHD is ${IS_VHD,,}, full install requred"
        End
    End
End