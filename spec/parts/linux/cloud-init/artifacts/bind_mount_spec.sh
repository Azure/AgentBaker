#!/bin/bash

Describe 'bind-mount.sh'
    exec 19>/dev/null
    export BASH_XTRACEFD=19
    Include './parts/linux/cloud-init/artifacts/bind-mount.sh'
    set +x
    unset BASH_XTRACEFD
    exec 19>&-

    setup() {
        TEST_ROOT="$(mktemp -d)"
        MOUNT_POINT="${TEST_ROOT}/mnt/aks"
        KUBELET_MOUNT_POINT="${MOUNT_POINT}/kubelet"
        KUBELET_DIR="${TEST_ROOT}/var/lib/kubelet"
        SENTINEL_FILE="${TEST_ROOT}/opt/azure/containers/bind-sentinel"
        mkdir -p "${KUBELET_DIR}" "$(dirname "${SENTINEL_FILE}")"
    }

    cleanup() {
        rm -rf "${TEST_ROOT}"
    }

    mount() {
        echo "mount $*"
    }

    chmod() {
        echo "chmod $*"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'moves kubelet state to the temporary disk on first boot'
        touch "${KUBELET_DIR}/fresh-state"

        When call prepareKubeletDisk

        The status should be success
        The path "${KUBELET_MOUNT_POINT}/fresh-state" should be file
        The path "${SENTINEL_FILE}" should be file
        The path "${KUBELET_DIR}" should be directory
        The output should include "mount --bind ${KUBELET_MOUNT_POINT} ${KUBELET_DIR}"
    End

    It 'resets retained kubelet state after an OS disk reimage'
        mkdir -p "${KUBELET_MOUNT_POINT}" "${MOUNT_POINT}/containers"
        touch "${KUBELET_MOUNT_POINT}/stale-state"
        touch "${MOUNT_POINT}/containers/image-cache"
        touch "${KUBELET_DIR}/fresh-state"

        When call prepareKubeletDisk

        The status should be success
        The path "${KUBELET_MOUNT_POINT}/stale-state" should not be exist
        The path "${KUBELET_MOUNT_POINT}/fresh-state" should be file
        The path "${MOUNT_POINT}/containers/image-cache" should be file
        The path "${SENTINEL_FILE}" should be file
        The output should include "mount --bind ${KUBELET_MOUNT_POINT} ${KUBELET_DIR}"
    End

    It 'preserves kubelet state when the sentinel exists on reboot'
        mkdir -p "${KUBELET_MOUNT_POINT}"
        touch "${KUBELET_MOUNT_POINT}/current-state"
        touch "${SENTINEL_FILE}"

        When call prepareKubeletDisk

        The status should be success
        The path "${KUBELET_MOUNT_POINT}/current-state" should be file
        The output should include "mount --bind ${KUBELET_MOUNT_POINT} ${KUBELET_DIR}"
    End
End