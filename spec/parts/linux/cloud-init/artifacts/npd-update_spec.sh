#!/bin/bash
# shellcheck disable=SC2317

Describe 'NPD config handler'
    Include ./parts/linux/cloud-init/artifacts/ubuntu/npd-update.sh

    setup() {
        TEST_DIR="$(mktemp -d)"
        NPD_OS_RELEASE_FILE="${TEST_DIR}/os-release"
        NPD_SKIP_FILE="${TEST_DIR}/skip_vhd_npd"
        NPD_UPDATE_DIR="${TEST_DIR}/work"
        NPD_PACKAGE_CACHE="${TEST_DIR}/cache"
        printf 'ID=ubuntu\nVERSION_ID="24.04"\n' > "${NPD_OS_RELEASE_FILE}"
        touch "${NPD_SKIP_FILE}"
        INSTALLED_VERSION="1.0.0-ubuntu24.04u1"
        TEST_DESIRED='{"ubuntuPackageVersions":{"24.04":"1.1.0-ubuntu24.04u1"}}'
        mkdir -p "${NPD_UPDATE_DIR}" "${NPD_PACKAGE_CACHE}"
        TEST_CALLS="${TEST_DIR}/calls"
        touch "${TEST_CALLS}"
        FAIL_ACTIVATION=false
        FAIL_DOWNLOAD=false
        export TEST_CALLS FAIL_ACTIVATION FAIL_DOWNLOAD INSTALLED_VERSION
    }

    cleanup() { rm -rf "${TEST_DIR}"; }
    BeforeEach setup
    AfterEach cleanup

    npd_installed_version() { printf '%s' "${INSTALLED_VERSION}"; }
    npd_find_cached_package() {
        [ "$1" = "${INSTALLED_VERSION}" ] || [ -f "${NPD_PACKAGE_CACHE}/$1.deb" ] || return 1
        printf '%s' "${NPD_PACKAGE_CACHE}/$1.deb"
    }
    npd_refresh_metadata() { echo refresh >> "${TEST_CALLS}"; }
    npd_download_package() {
        echo download >> "${TEST_CALLS}"
        [ "${FAIL_DOWNLOAD}" = false ] || return 1
        touch "${NPD_PACKAGE_CACHE}/$1.deb"
    }
    npd_validate_package() { echo validate >> "${TEST_CALLS}"; }
    npd_install_package() { echo "install $1" >> "${TEST_CALLS}"; }
    npd_restart_and_check() {
        echo restart >> "${TEST_CALLS}"
        if [ "${FAIL_ACTIVATION}" = true ]; then
            FAIL_ACTIVATION=false
            return 1
        fi
    }

    It 'does not touch extension-managed NPD'
        rm "${NPD_SKIP_FILE}"
        When call updateNPDConfigs "${TEST_DESIRED}" '{}'
        The status should be success
        The output should include 'extension-managed'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    It 'does not touch Azure Linux even with a skip marker'
        printf 'ID=azurelinux\nVERSION_ID="3.0"\n' > "${NPD_OS_RELEASE_FILE}"
        When call updateNPDConfigs "${TEST_DESIRED}" '{}'
        The status should be success
        The output should include 'unsupported'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    It 'treats an untargeted Ubuntu release as no action'
        When call updateNPDConfigs '{"ubuntuPackageVersions":{"22.04":"1.0.0-ubuntu22.04u1"}}' '{}'
        The status should be success
        The output should include 'no package selected'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    It 'does not download an equal baked version'
        When call updateNPDConfigs '{"ubuntuPackageVersions":{"24.04":"1.0.0-ubuntu24.04u1"}}' '{}'
        The status should be success
        The output should include 'satisfies'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    It 'uses Debian version order and keeps a newer installed revision'
        INSTALLED_VERSION='1.0.0-ubuntu24.04u10'
        When call updateNPDConfigs '{"ubuntuPackageVersions":{"24.04":"1.0.0-ubuntu24.04u9"}}' '{}'
        The status should be success
        The output should include 'satisfies'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    It 'rejects a targeted version for the wrong Ubuntu release'
        When call updateNPDConfigs '{"ubuntuPackageVersions":{"24.04":"1.0.0-ubuntu22.04u1"}}' '{}'
        The status should be failure
        The output should include 'InvalidConfig'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    It 'accepts a future release using the same RP version grammar'
        When call npd_desired_version '{"ubuntuPackageVersions":{"28.04":"1.0.0-ubuntu28.04u1"}}' 28.04
        The status should be success
        The output should equal '1.0.0-ubuntu28.04u1'
    End

    It 'rejects a malformed version map'
        When call updateNPDConfigs '{"ubuntuPackageVersions":[]}' '{}'
        The status should be failure
        The output should include 'InvalidConfig'
        The stderr should include 'invalid version map'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    It 'stages the update before installation and checkpoints only after health'
        When call updateNPDConfigs "${TEST_DESIRED}" '{}'
        The status should be success
        The output should include 'updated successfully'
        The contents of file "${TEST_CALLS}" should include 'download'
        The contents of file "${TEST_CALLS}" should include "install ${NPD_PACKAGE_CACHE}/1.1.0-ubuntu24.04u1.deb"
        The contents of file "${TEST_CALLS}" should include 'restart'
        The path "${NPD_UPDATE_DIR}/pending" should not be exist
    End

    It 'keeps running files untouched when the download fails'
        FAIL_DOWNLOAD=true
        When call updateNPDConfigs "${TEST_DESIRED}" '{}'
        The status should be failure
        The output should include 'PackageDownload'
        The contents of file "${TEST_CALLS}" should include 'download'
        The contents of file "${TEST_CALLS}" should not include 'install'
        The path "${NPD_UPDATE_DIR}/pending" should not be exist
    End

    It 'reinstalls the prior deb and restarts after activation failure'
        FAIL_ACTIVATION=true
        When call updateNPDConfigs "${TEST_DESIRED}" '{}'
        The status should be failure
        The output should include 'PackageActivation'
        The contents of file "${TEST_CALLS}" should include "install ${NPD_PACKAGE_CACHE}/1.0.0-ubuntu24.04u1.deb"
        The path "${NPD_UPDATE_DIR}/pending" should not be exist
    End

    It 'retains pending recovery when recovery also fails'
        npd_restart_and_check() { return 1; }
        When call updateNPDConfigs "${TEST_DESIRED}" '{}'
        The status should be failure
        The output should include 'PackageActivation'
        The contents of file "${NPD_UPDATE_DIR}/pending" should equal "${INSTALLED_VERSION}"
    End

    It 'repairs interrupted work even when the new goal has no target'
        printf '%s' "${INSTALLED_VERSION}" > "${NPD_UPDATE_DIR}/pending"
        When call updateNPDConfigs '{}' '{}'
        The status should be success
        The output should include 'no package selected'
        The contents of file "${TEST_CALLS}" should include "install ${NPD_PACKAGE_CACHE}/${INSTALLED_VERSION}.deb"
        The path "${NPD_UPDATE_DIR}/pending" should not be exist
    End

    It 'recognizes checkpointed work with an installed matching version'
        INSTALLED_VERSION='1.1.0-ubuntu24.04u1'
        When call npdConfigsIsCurrent "${TEST_DESIRED}" "${TEST_DESIRED}" '{}'
        The status should be success
    End

    It 'recovers interrupted work before rejecting a malformed new goal'
        printf '%s' "${INSTALLED_VERSION}" > "${NPD_UPDATE_DIR}/pending"
        When call updateNPDConfigs '{"ubuntuPackageVersions":[]}' '{}'
        The status should be failure
        The output should include 'InvalidConfig'
        The stderr should include 'invalid version map'
        The contents of file "${TEST_CALLS}" should include "install ${NPD_PACKAGE_CACHE}/${INSTALLED_VERSION}.deb"
        The path "${NPD_UPDATE_DIR}/pending" should not be exist
    End

    It 'does not skip a pending transaction based on version alone'
        INSTALLED_VERSION='1.1.0-ubuntu24.04u1'
        touch "${NPD_UPDATE_DIR}/pending"
        When call npdConfigsIsCurrent "${TEST_DESIRED}" "${TEST_DESIRED}" '{}'
        The status should be failure
    End

    It 'does not inherit securityPatch snapshot-only apt configuration'
        export APT_CONFIG=/var/lib/security-patch/apt.conf
        npd_refresh_metadata() {
            test -z "${APT_CONFIG+x}" || return 1
            echo refresh >> "${TEST_CALLS}"
        }
        When call updateNPDConfigs "${TEST_DESIRED}" '{}'
        The status should be success
        The output should include 'updated successfully'
        The variable APT_CONFIG should equal /var/lib/security-patch/apt.conf
    End
End

Describe 'CSE baked NPD activation'
    Include ./parts/linux/cloud-init/artifacts/cse_config.sh
    setup_activation() {
        NPD_SKIP_FILE="$(mktemp)"
        OS=UBUNTU
        UBUNTU_OS_NAME=UBUNTU
        ERR_SYSTEMCTL_START_FAIL=4
    }
    cleanup_activation() { rm -f "${NPD_SKIP_FILE}"; }
    BeforeEach setup_activation
    AfterEach cleanup_activation

    Mock dpkg-query
        printf installed
    End
    systemctlEnableAndStart() { echo "start $*"; }

    It 'starts the held package-owned service'
        When call configureNodeProblemDetector
        The status should be success
        The output should equal 'start node-problem-detector 30'
    End

    It 'skips old images without the marker'
        rm "${NPD_SKIP_FILE}"
        When call configureNodeProblemDetector
        The status should be success
        The output should include 'extension-managed'
    End

    It 'does not start Azure Linux NPD'
        OS=AZURELINUX
        When call configureNodeProblemDetector
        The status should be success
        The output should include 'extension-managed'
    End

    It 'propagates activation failure'
        systemctlEnableAndStart() { return 1; }
        When call configureNodeProblemDetector
        The status should equal 4
    End
End
