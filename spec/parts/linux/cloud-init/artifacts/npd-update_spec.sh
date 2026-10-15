#!/bin/bash
# shellcheck disable=SC2317

Describe 'NPD config handler'
    Include ./parts/linux/cloud-init/artifacts/npd-update.sh

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
    # dpkg journal repair runs through timeout; record it instead of touching dpkg.
    Mock timeout
        echo "repair $*" >> "${TEST_CALLS}"
        exit "${REPAIR_STATUS:-0}"
    End
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

    It 'does not touch Azure Container Linux even with a skip marker'
        printf 'ID=azurelinux\nVERSION_ID="3.0"\nVARIANT_ID=azurecontainerlinux\n' > "${NPD_OS_RELEASE_FILE}"
        When call updateNPDConfigs "${TEST_DESIRED}" '{}'
        The status should be success
        The output should include 'unsupported'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    It 'ignores the Azure Linux map on Ubuntu'
        When call updateNPDConfigs '{"azureLinuxPackageVersions":{"3.0":"1.1.0-1.azl3"}}' '{}'
        The status should be success
        The output should include 'no package selected for ubuntu 24.04'
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
        When call npd_desired_version '{"ubuntuPackageVersions":{"28.04":"1.0.0-ubuntu28.04u1"}}' ubuntu 28.04
        The status should be success
        The output should equal '1.0.0-ubuntu28.04u1'
    End

    It 'rejects a version with a trailing newline that command substitution would strip'
        When call npd_desired_version '{"ubuntuPackageVersions":{"24.04":"1.0.0-ubuntu24.04u1\n"}}' ubuntu 24.04
        The status should be failure
        The stderr should include 'invalid version'
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

    It 'finishes an interrupted package-manager transaction before restoring the prior package'
        printf '%s' "${INSTALLED_VERSION}" > "${NPD_UPDATE_DIR}/pending"
        When call npd_recover_pending
        The status should be success
        The contents of file "${TEST_CALLS}" should equal "repair 300 dpkg --force-confdef --force-confold --configure -a
install ${NPD_PACKAGE_CACHE}/${INSTALLED_VERSION}.deb
restart"
    End

    It 'still restores the prior package when package-manager repair fails'
        printf '%s' "${INSTALLED_VERSION}" > "${NPD_UPDATE_DIR}/pending"
        export REPAIR_STATUS=1
        When call npd_recover_pending
        The status should be success
        The output should include 'repair failed; attempting rollback'
        The contents of file "${TEST_CALLS}" should include "install ${NPD_PACKAGE_CACHE}/${INSTALLED_VERSION}.deb"
        The path "${NPD_UPDATE_DIR}/pending" should not be exist
    End

    It 'keeps the pending marker when both repair and rollback fail'
        printf '%s' "${INSTALLED_VERSION}" > "${NPD_UPDATE_DIR}/pending"
        export REPAIR_STATUS=1
        npd_install_package() { echo "install $1" >> "${TEST_CALLS}"; return 1; }
        When call updateNPDConfigs "${TEST_DESIRED}" '{}'
        The status should be failure
        The output should include 'RecoveryFailed'
        The path "${NPD_UPDATE_DIR}/pending" should be exist
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

Describe 'NPD config handler on Azure Linux 3'
    Include ./parts/linux/cloud-init/artifacts/npd-update.sh

    setup() {
        TEST_DIR="$(mktemp -d)"
        NPD_OS_RELEASE_FILE="${TEST_DIR}/os-release"
        NPD_SKIP_FILE="${TEST_DIR}/skip_vhd_npd"
        NPD_UPDATE_DIR="${TEST_DIR}/work"
        NPD_PACKAGE_CACHE="${TEST_DIR}/cache"
        NPD_CONFIG_DIR="${TEST_DIR}/etc/node-problem-detector.d"
        NPD_UNIT_FILE="${TEST_DIR}/etc/systemd/system/node-problem-detector.service"
        NPD_TDNF_CONFIG="${TEST_DIR}/etc/tdnf/tdnf.conf"
        NPD_TDNF_LOCK_FILE="${TEST_DIR}/etc/tdnf/locks.d/node-problem-detector.conf"
        printf 'NAME="Microsoft Azure Linux"\nID=azurelinux\nVERSION_ID="3.0"\n' > "${NPD_OS_RELEASE_FILE}"
        touch "${NPD_SKIP_FILE}"
        INSTALLED_VERSION="1.0.0-1.azl3"
        TEST_DESIRED='{"ubuntuPackageVersions":{"24.04":"9.0.0-ubuntu24.04u1"},"azureLinuxPackageVersions":{"3.0":"1.1.0-1.azl3"}}'
        mkdir -p "${NPD_UPDATE_DIR}" "${NPD_PACKAGE_CACHE}" "${NPD_CONFIG_DIR}/plugin" "$(dirname "${NPD_UNIT_FILE}")" "$(dirname "${NPD_TDNF_CONFIG}")"
        printf '[main]\ngpgcheck=1\n' > "${NPD_TDNF_CONFIG}"
        TEST_CALLS="${TEST_DIR}/calls"
        touch "${TEST_CALLS}"
        FAIL_ACTIVATION=false
        export TEST_CALLS FAIL_ACTIVATION INSTALLED_VERSION TEST_DIR
    }

    cleanup() { rm -rf "${TEST_DIR}"; }
    BeforeEach setup
    AfterEach cleanup

    # The shellspec image has no rpm; compare like rpm.vercmp for these versions.
    Mock rpm
        case "$1" in
            --define)
                a="${2#_npd_installed }"; b="${4#_npd_desired }"
                if [ "$a" = "$b" ]; then echo 0
                elif dpkg --compare-versions "$a" gt "$b"; then echo 1
                else echo -1; fi
                ;;
            -U|-e) echo "rpm $*" >> "${TEST_CALLS}" ;;
            -qp) printf 'node-problem-detector-aks-config-1.0.0-1.azl3.x86_64' ;;
            -q) printf '%s\n' ${RPM_ENTRIES:-node-problem-detector-aks-config-1.0.0-1.azl3.x86_64} ;;
            *) exit 1 ;;
        esac
    End
    Mock tdnf
        echo "tdnf $*" >> "${TEST_CALLS}"
    End
    npd_installed_version() { printf '%s' "${INSTALLED_VERSION}"; }
    npd_find_cached_package() {
        [ "$1" = "${INSTALLED_VERSION}" ] || [ -f "${NPD_PACKAGE_CACHE}/$1.rpm" ] || return 1
        printf '%s' "${NPD_PACKAGE_CACHE}/$1.rpm"
    }
    npd_download_package() {
        echo download >> "${TEST_CALLS}"
        touch "${NPD_PACKAGE_CACHE}/$1.rpm"
    }
    npd_validate_package() { echo validate >> "${TEST_CALLS}"; }
    npd_restart_and_check() {
        echo restart >> "${TEST_CALLS}"
        if [ "${FAIL_ACTIVATION}" = true ]; then
            FAIL_ACTIVATION=false
            return 1
        fi
    }

    It 'selects the Azure Linux map for this release and ignores Ubuntu entries'
        When call npd_desired_version "${TEST_DESIRED}" azurelinux 3.0
        The status should be success
        The output should equal '1.1.0-1.azl3'
    End

    It 'rejects a package built for another Azure Linux release'
        When call updateNPDConfigs '{"azureLinuxPackageVersions":{"3.0":"1.1.0-1.azl4"}}' '{}'
        The status should be failure
        The output should include 'InvalidConfig'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    It 'rejects Debian-style versions in the Azure Linux map'
        When call npd_desired_version '{"azureLinuxPackageVersions":{"3.0":"1.1.0-ubuntu24.04u1"}}' azurelinux 3.0
        The status should be failure
        The stderr should include 'invalid version'
    End

    It 'treats an untargeted Azure Linux node as no action'
        When call updateNPDConfigs '{"ubuntuPackageVersions":{"24.04":"1.0.0-ubuntu24.04u1"}}' '{}'
        The status should be success
        The output should include 'no package selected for azurelinux 3.0'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    It 'uses RPM version order and keeps a newer installed release'
        INSTALLED_VERSION='1.0.0-10.azl3'
        When call updateNPDConfigs '{"azureLinuxPackageVersions":{"3.0":"1.0.0-9.azl3"}}' '{}'
        The status should be success
        The output should include 'satisfies'
        The contents of file "${TEST_CALLS}" should equal ''
    End

    It 'downloads through an unlocked tdnf config, installs with rpm, and holds the package'
        When call updateNPDConfigs "${TEST_DESIRED}" '{}'
        The status should be success
        The output should include 'updated successfully: 1.1.0-1.azl3'
        The contents of file "${TEST_CALLS}" should include "tdnf -c ${NPD_UPDATE_DIR}/tdnf/tdnf.conf makecache --refresh"
        The contents of file "${TEST_CALLS}" should include "rpm -U --oldpackage --replacepkgs ${NPD_PACKAGE_CACHE}/1.1.0-1.azl3.rpm"
        The contents of file "${NPD_TDNF_LOCK_FILE}" should equal 'node-problem-detector-aks-config'
        The path "${NPD_UPDATE_DIR}/pending" should not be exist
    End

    It 'reinstalls the prior rpm after activation failure'
        FAIL_ACTIVATION=true
        When call updateNPDConfigs "${TEST_DESIRED}" '{}'
        The status should be failure
        The output should include 'PackageActivation'
        The contents of file "${TEST_CALLS}" should include "rpm -U --oldpackage --replacepkgs ${NPD_PACKAGE_CACHE}/1.0.0-1.azl3.rpm"
        The path "${NPD_UPDATE_DIR}/pending" should not be exist
    End

    It 'replaces locally modified configs and removes stale saved configs'
        printf old > "${NPD_CONFIG_DIR}/plugin/check.sh"
        printf new > "${NPD_CONFIG_DIR}/plugin/check.sh.rpmnew"
        printf stale > "${NPD_CONFIG_DIR}/plugin/dropped.json.rpmsave"
        printf unit > "${NPD_UNIT_FILE}.rpmnew"
        When call npd_remove_obsolete_configs
        The status should be success
        The contents of file "${NPD_CONFIG_DIR}/plugin/check.sh" should equal new
        The path "${NPD_CONFIG_DIR}/plugin/check.sh.rpmnew" should not be exist
        The path "${NPD_CONFIG_DIR}/plugin/dropped.json.rpmsave" should not be exist
        The contents of file "${NPD_UNIT_FILE}" should equal unit
        The path "${NPD_SKIP_FILE}" should be exist
    End

    It 'drops a stale rpm database entry left by an interrupted upgrade'
        export RPM_ENTRIES='node-problem-detector-aks-config-1.0.0-1.azl3.x86_64 node-problem-detector-aks-config-1.1.0-1.azl3.x86_64'
        When call npd_remove_stale_rpm_entries "${NPD_PACKAGE_CACHE}/1.0.0-1.azl3.rpm"
        The status should be success
        The contents of file "${TEST_CALLS}" should equal 'rpm -e --justdb --nodeps --noscripts node-problem-detector-aks-config-1.1.0-1.azl3.x86_64'
    End

    It 'does not need rpm database repair before restoring'
        When call npd_repair_package_state
        The status should be success
        The contents of file "${TEST_CALLS}" should equal ''
    End

    It 'does not duplicate the tdnf lock'
        mkdir -p "$(dirname "${NPD_TDNF_LOCK_FILE}")"
        printf 'node-problem-detector-kubernetes\nnode-problem-detector-aks-config\n' > "${NPD_TDNF_LOCK_FILE}"
        When call npd_hold_packages
        The status should be success
        The lines of contents of file "${NPD_TDNF_LOCK_FILE}" should equal 2
    End

    It 'is safe under the Azure Linux dispatcher nounset and errexit options'
        When run env NPD_OS_RELEASE_FILE="${NPD_OS_RELEASE_FILE}" bash -euc 'source ./parts/linux/cloud-init/artifacts/npd-update.sh; npd_platform; echo; npd_desired_version "$1" azurelinux 3.0' _ "${TEST_DESIRED}"
        The status should be success
        The output should include '1.1.0-1.azl3'
    End
End

Describe 'CSE baked NPD activation'
    Include ./parts/linux/cloud-init/artifacts/cse_helpers.sh
    Include ./parts/linux/cloud-init/artifacts/cse_config.sh
    setup_activation() {
        NPD_SKIP_FILE="$(mktemp)"
        OS=UBUNTU
        OS_VARIANT=""
        ERR_SYSTEMCTL_START_FAIL=4
    }
    cleanup_activation() { rm -f "${NPD_SKIP_FILE}"; }
    BeforeEach setup_activation
    AfterEach cleanup_activation

    Mock dpkg-query
        printf installed
    End
    Mock rpm
        [ "${RPM_INSTALLED:-true}" = true ]
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

    It 'starts Azure Linux 3 NPD from the RPM package'
        OS=AZURELINUX
        When call configureNodeProblemDetector
        The status should be success
        The output should equal 'start node-problem-detector 30'
    End

    It 'skips Azure Linux images with the marker but without the config package'
        OS=AZURELINUX
        export RPM_INSTALLED=false
        When call configureNodeProblemDetector
        The status should be success
        The output should include 'not installed'
    End

    It 'leaves Azure Container Linux and Mariner extension-managed'
        OS=AZURELINUX
        OS_VARIANT=AZURECONTAINERLINUX
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

Describe 'GPU driver NPD restart'
    Include ./parts/linux/cloud-init/artifacts/cse_config_gpu.sh
    setup_gpu() { NPD_SKIP_FILE="$(mktemp)"; }
    cleanup_gpu() { rm -f "${NPD_SKIP_FILE}"; }
    BeforeEach setup_gpu
    AfterEach cleanup_gpu
    Mock systemctl
        echo "systemctl $*"
    End

    It 'leaves baked NPD for nodePrep activation'
        When call restartExtensionManagedNPD
        The status should be success
        The output should equal ''
    End

    It 'still restarts extension-managed NPD'
        rm "${NPD_SKIP_FILE}"
        When call restartExtensionManagedNPD
        The status should be success
        The output should equal 'systemctl restart node-problem-detector'
    End
End
