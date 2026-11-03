#!/bin/bash
# shellcheck disable=SC2016,SC2034,SC2329

Describe 'NVIDIA container toolkit release notes'
    Include ./parts/linux/cloud-init/artifacts/cse_helpers.sh
    eval "$(sed -n '/^writeNvidiaContainerToolkitReleaseNotes()/,/^}/p' ./vhdbuilder/packer/install-dependencies.sh)"

    setup() {
        TEST_DIR="$(mktemp -d)"
        VHD_LOGS_FILEPATH="${TEST_DIR}/vhd.log"
        COMPONENTS_FILEPATH="${TEST_DIR}/components.json"
        TEST_CALLS="${TEST_DIR}/calls"
        OS=UBUNTU
        OS_VARIANT=""
        CPU_ARCH=amd64
        UBUNTU_RELEASE=24.04
        FEATURE_FLAGS=""
        IS_KATA=false
        IMAGE_TOOLKIT_VERSION=1.19.1-1
        INSTALLED_TOOLKIT_VERSION=1.18.0-1
        QUERY_STATUS=0
        printf '%s\n' '{"Packages":[{"name":"nvidia-container-toolkit","downloadURIs":{"azurelinux":{"current":{"versionsV2":[{"latestVersion":"1.17.8"}]}}}}]}' > "$COMPONENTS_FILEPATH"
    }

    cleanup() { rm -rf "$TEST_DIR"; }
    BeforeEach setup
    AfterEach cleanup

    ctr() {
        printf '%s\n' "$*" > "$TEST_CALLS"
        printf '%s\n' "$IMAGE_TOOLKIT_VERSION"
        return "$QUERY_STATUS"
    }

    dpkg-query() {
        printf '%s\n' "$*" > "$TEST_CALLS"
        printf '%s\n' "$INSTALLED_TOOLKIT_VERSION"
        return "$QUERY_STATUS"
    }

    Describe 'cached CUDA driver images'
        Parameters
            amd64
            arm64
        End

        It "reports the toolkit from the cached Ubuntu $1 CUDA image without claiming it is installed"
            CPU_ARCH="$1"
            When call writeNvidiaContainerToolkitReleaseNotes mcr.microsoft.com/aks/aks-gpu-cuda-lts test-tag
            The status should be success
            The contents of file "$VHD_LOGS_FILEPATH" should include 'nvidia-container-toolkit version 1.19.1-1 (cached in the CUDA driver image; installed at node provisioning time)'
            The contents of file "$TEST_CALLS" should equal '-n k8s.io run --rm mcr.microsoft.com/aks/aks-gpu-cuda-lts:test-tag nvidia-container-toolkit-version /bin/sh -c dpkg-deb -f /opt/gpu/nvidia-container-toolkit_*.deb Version'
        End
    End

    It 'reports the installed Grace Blackwell toolkit instead of the cached CUDA image version'
        CPU_ARCH=arm64
        FEATURE_FLAGS=NVIDIA_GB
        When call writeNvidiaContainerToolkitReleaseNotes cached-image cached-tag
        The status should be success
        The contents of file "$VHD_LOGS_FILEPATH" should include 'nvidia-container-toolkit version 1.18.0-1 (installed on the VHD)'
        The contents of file "$TEST_CALLS" should equal '-W -f=${Version} nvidia-container-toolkit'
    End

    It 'reports the Azure Linux provisioning pin rather than querying installed packages'
        OS=AZURELINUX
        When call writeNvidiaContainerToolkitReleaseNotes '' ''
        The status should be success
        The contents of file "$VHD_LOGS_FILEPATH" should include 'nvidia-container-toolkit version 1.17.8 (pinned in components.json; installed at node provisioning time)'
        The path "$TEST_CALLS" should not be exist
    End

    Describe 'unsupported or uncached images'
        Parameters
            UBUNTU amd64 '' false
            AZURELINUX arm64 '' false
            AZURELINUX amd64 OSGUARD false
            AZURELINUX amd64 AZURECONTAINERLINUX false
            AZURELINUX amd64 '' true
            FLATCAR amd64 '' false
        End

        It "omits toolkit notes for $1 $2 $3 kata=$4"
            OS="$1" CPU_ARCH="$2" OS_VARIANT="$3" IS_KATA="$4"
            When call writeNvidiaContainerToolkitReleaseNotes '' ''
            The status should be success
            The path "$VHD_LOGS_FILEPATH" should not be exist
            The path "$TEST_CALLS" should not be exist
        End
    End

    It 'propagates a failed cached-image query without writing a version'
        QUERY_STATUS=42
        When call writeNvidiaContainerToolkitReleaseNotes cached-image cached-tag
        The status should equal 42
        The path "$VHD_LOGS_FILEPATH" should not be exist
    End

    It 'propagates a failed installed-package query without writing a version'
        CPU_ARCH=arm64
        FEATURE_FLAGS=NVIDIA_GB
        QUERY_STATUS=42
        When call writeNvidiaContainerToolkitReleaseNotes '' ''
        The status should equal 42
        The path "$VHD_LOGS_FILEPATH" should not be exist
    End

    It 'rejects missing Azure Linux version metadata'
        OS=AZURELINUX
        printf '%s\n' '{"Packages":[]}' > "$COMPONENTS_FILEPATH"
        When call writeNvidiaContainerToolkitReleaseNotes '' ''
        The status should be failure
        The error should include 'NVIDIA container toolkit version is missing'
        The path "$VHD_LOGS_FILEPATH" should not be exist
    End

    It 'rejects an empty cached-image version'
        IMAGE_TOOLKIT_VERSION=""
        When call writeNvidiaContainerToolkitReleaseNotes cached-image cached-tag
        The status should be failure
        The error should include 'NVIDIA container toolkit version is missing'
        The path "$VHD_LOGS_FILEPATH" should not be exist
    End
End
