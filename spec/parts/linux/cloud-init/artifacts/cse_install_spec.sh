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
        exit() {
            echo "mock exit calling with $1"
        }
        VHD_LOGS_FILEPATH="non_existent_file.txt"
        setup() {
            touch "$VHD_LOGS_FILEPATH"
        }
        cleanup() {
            VHD_LOGS_FILEPATH="non_existent_file.txt"
            rm -f "$VHD_LOGS_FILEPATH"
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'
        It 'should skip binary cleanup if SKIP_BINARY_CLEANUP is true'
            SKIP_BINARY_CLEANUP="true"
            IS_VHD="false"
            When call getInstallModeAndCleanupContainerImages $SKIP_BINARY_CLEANUP $IS_VHD
            The output line 1 should equal "binaries will not be cleaned up"
            The output line 2 should equal "true"
        End
        It 'should cleanup container images if SKIP_BINARY_CLEANUP is false and VHD_LOGS_FILEPATH exists'
            SKIP_BINARY_CLEANUP="false"
            IS_VHD="false"
            cleanUpContainerImages() {
                echo "mock cleanUpContainerImages calling"
            }
            When call getInstallModeAndCleanupContainerImages $SKIP_BINARY_CLEANUP $IS_VHD
            The output line 1 should equal "detected golden image pre-install"
            The output line 2 should equal "mock logs to events calling with AKS.CSE.cleanUpContainerImages"
            The output line 3 should equal "false"
        End
        It 'should error if IS_VHD is true and VHD_LOGS_FILEPATH does not exist'
            SKIP_BINARY_CLEANUP="false"
            IS_VHD="true"
            VHD_LOGS_FILEPATH="dummy-not-existant-file.txt"
            When call getInstallModeAndCleanupContainerImages $SKIP_BINARY_CLEANUP $IS_VHD
            The output line 1 should equal "Using VHD distro but file $VHD_LOGS_FILEPATH not found"
            The output line 2 should equal "mock exit calling with 65"
        End
        It 'should return true if VHD_LOGS_FILEPATH does not exist and IS_VHD is false'
            SKIP_BINARY_CLEANUP="false"
            IS_VHD="false"
            VHD_LOGS_FILEPATH="dummy-not-existant-file.txt"
            When call getInstallModeAndCleanupContainerImages $SKIP_BINARY_CLEANUP $IS_VHD
            The output line 1 should equal "the file $VHD_LOGS_FILEPATH does not exist and IS_VHD is "${IS_VHD,,}", full install requred"
            The output line 2 should equal "true"
        End
    End
    Describe 'extractKubeBinaries'
        k8s_version="1.31.5"        
        is_private_url="false"
        k8s_downloads_dir="/opt/kubernetes/downloads"
        ORAS_REGISTRY_CONFIG_FILE=/etc/oras/config.yaml
        CPU_ARCH="amd64"
        KUBE_BINARY_URL=""

        Mock cleanup
        cleanup() {
            #clean up $k8s_tgz_tmp if it exists
            if [ -f "$k8s_tgz_tmp" ]; then
                m -f "$k8s_tgz_tmp"
            fi
        }

        # mock extractKubeBinariesToUsrLocalBin as we don't really want to extract the binaries
        Mock extractKubeBinariesToUsrLocalBin
        extractKubeBinariesToUsrLocalBin() {
            echo "mock extractKubeBinariesToUsrLocalBin calling with $1 $2 $3 $4"
        }

        # Mock retrycmd_get_tarball_from_registry_with_oras as we don't really want to download the tarball
        # The real download is tested in e2e test.
        Mock retrycmd_get_tarball_from_registry_with_oras
        retrycmd_get_tarball_from_registry_with_oras() {
            echo "mock retrycmd_get_tarball_from_registry_with_oras calling with $1 $2 $3 $4"
            # create a fake tarball
            touch "$k8s_tgz_tmp"
        }

        # mock retrycmd_get_tarball as we don't really want to download the tarball
        Mock retrycmd_get_tarball
        retrycmd_get_tarball() {
            echo "mock retrycmd_get_tarball calling with $1 $2 $3 $4 $5"
            touch "$k8s_tgz_tmp"
        }

        AfterEach 'cleanup'
        It 'should use retrycmd_get_tarball_from_registry_with_oras to download kube binaries' 
            kube_binary_url="mcr.microsoft.com/oss/binaries/kubernetes/kubernetes-node:FakeTag"
            When call extractKubeBinaries $k8s_version $kube_binary_url $is_private_url $k8s_downloads_dir
            The status should be success
            The output line 1 should include "detect kube_binary_url"
            The output line 2 should include "mock retrycmd_get_tarball_from_registry_with_oras calling"
            The output line 3 should include "mock extractKubeBinariesToUsrLocalBin calling"
        End
        It 'should use retrycmd_get_tarball to download kube binaries'
            kube_binary_url="https://acs-mirror.azureedge.net/kubernetes/v1.31.5/binaries/Fakefile"
            When call extractKubeBinaries $k8s_version $kube_binary_url $is_private_url $k8s_downloads_dir
            The status should be success
            The output line 1 should include "mock retrycmd_get_tarball calling"
            The output line 2 should include "mock extractKubeBinariesToUsrLocalBin calling"
        End
        It 'should use a pre-cached private kube binary if available (this is an unavailable case)'
            is_private_url="true"
            K8S_PRIVATE_PACKAGES_CACHE_DIR="/opt/kubernetes/downloads/private-packages"
            kube_binary_url="https://acs-mirror.azureedge.net/kubernetes/fake/binaries/kubernetes-node-linux-amd64.tar.gz"
            When call extractKubeBinaries $k8s_version $kube_binary_url $is_private_url $k8s_downloads_dir
            The status should be failure
            The output line 1 should include "cached package /opt/kubernetes/downloads/private-packages/kubernetes-node-linux-amd64.tar.gz not found"
        End
    End
End