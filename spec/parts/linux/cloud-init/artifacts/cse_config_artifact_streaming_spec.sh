#!/bin/bash

Describe 'ACL artifact streaming'
    CSE_CONFIG_GPU_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"
    CSE_CONFIG_LOCALDNS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_localdns.sh"
    CSE_CONFIG_KUBELET_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh"
    CSE_CONFIG_NETWORK_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_network.sh"
    CSE_CONFIG_ADDONS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_addons.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"

    setup_streaming() {
        TEST_ACR_DIR="$(mktemp -d)"
        ACR_MIRROR_SETUP_SCRIPT="${TEST_ACR_DIR}/setup.sh"
        printf '#!/bin/sh\necho "configure streaming"\n' > "${ACR_MIRROR_SETUP_SCRIPT}"
        chmod +x "${ACR_MIRROR_SETUP_SCRIPT}"
        OS="$AZURELINUX_OS_NAME"
        OS_VARIANT="$ACL_OS_VARIANT"
    }
    cleanup_streaming() {
        rm -f "${TEST_ACR_DIR}/setup.sh"
        rmdir "${TEST_ACR_DIR}"
    }
    BeforeEach 'setup_streaming'
    AfterEach 'cleanup_streaming'

    installArtifactStreamingSysext() {
        echo "install ACL streaming extension"
        return "${INSTALL_RC:-0}"
    }
    waitForContainerdReady() { echo "wait for containerd"; }
    retrycmd_if_failure() {
        echo "enable streaming services"
        return "${ENABLE_RC:-0}"
    }

    It 'installs the complete ACL payload before enabling services'
        When run ensureArtifactStreaming
        The line 1 of output should equal "install ACL streaming extension"
        The line 2 of output should equal "wait for containerd"
        The line 3 of output should equal "enable streaming services"
        The line 4 of output should equal "configure streaming"
        The status should be success
    End

    It 'does not enable or configure services if the ACL payload is unavailable'
        INSTALL_RC=231
        When run ensureArtifactStreaming
        The output should equal "install ACL streaming extension"
        The status should equal "$ERR_ARTIFACT_STREAMING_INSTALL"
    End

    It 'preserves the non-ACL path'
        OS_VARIANT=""
        When run ensureArtifactStreaming
        The output should not include "install ACL streaming extension"
        The line 1 of output should equal "wait for containerd"
        The line 2 of output should equal "enable streaming services"
        The line 3 of output should equal "configure streaming"
        The status should be success
    End

    It 'preserves service startup failure'
        ENABLE_RC=1
        When run ensureArtifactStreaming
        The output should not include "configure streaming"
        The status should equal "$ERR_ARTIFACT_STREAMING_INSTALL"
    End
End
