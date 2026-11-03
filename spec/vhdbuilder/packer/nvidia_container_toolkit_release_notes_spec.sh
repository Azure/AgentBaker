#!/bin/bash
# shellcheck disable=SC2034,SC2329

Describe 'cached NVIDIA container toolkit release notes'
    eval "$(sed -n '/^logCachedNvidiaContainerToolkitVersion()/,/^}/p' ./vhdbuilder/packer/install-dependencies.sh)"

    setup() {
        TEST_DIR="$(mktemp -d)"
        VHD_LOGS_FILEPATH="${TEST_DIR}/vhd.log"
        export TEST_CALLS="${TEST_DIR}/calls"
        export TOOLKIT_VERSION=1.19.1-1 CTR_STATUS=0
    }

    cleanup() { rm -rf "$TEST_DIR"; }
    BeforeEach setup
    AfterEach cleanup

    Mock ctr
        printf '%s\n' "$*" > "$TEST_CALLS"
        printf '%s\n' "$TOOLKIT_VERSION"
        exit "$CTR_STATUS"
    End

    It 'reports the toolkit from the selected cached image rather than the host'
        When call logCachedNvidiaContainerToolkitVersion example.com/cuda:test-tag
        The status should be success
        The contents of file "$VHD_LOGS_FILEPATH" should include 'nvidia-container-toolkit version 1.19.1-1 (cached in CUDA driver image; installed at node provisioning time)'
        The contents of file "$TEST_CALLS" should include "-n k8s.io run --rm example.com/cuda:test-tag nvidia-container-toolkit-version-$$"
    End

    Describe 'lookup failures'
        Parameters
            42
            124
        End

        It "warns without failing the build when the query exits $1"
            CTR_STATUS="$1"
            When call logCachedNvidiaContainerToolkitVersion example.com/cuda:test-tag
            The status should be success
            The error should include 'Warning: unable to read NVIDIA container toolkit version'
            The path "$VHD_LOGS_FILEPATH" should not be exist
        End
    End

    It 'warns instead of recording an empty version'
        TOOLKIT_VERSION=""
        When call logCachedNvidiaContainerToolkitVersion example.com/cuda:test-tag
        The status should be success
        The error should include 'Warning: unable to read NVIDIA container toolkit version'
        The path "$VHD_LOGS_FILEPATH" should not be exist
    End
End
