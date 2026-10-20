#!/bin/bash

Describe 'NPD package selection'
    Include ./parts/linux/cloud-init/artifacts/provision_helpers.sh

    select_npd_packages() {
        local os="$1" release="$2" package
        while IFS= read -r package; do
            updatePackageVersions "${package}" "${os}" "${release}" ''
            printf '%s: %s\n' "$(jq -r .name <<< "${package}")" "${PACKAGE_VERSIONS[*]}"
        done < <(jq -c '.Packages[] | select(.name | startswith("node-problem-detector"))' parts/common/components.json)
    }

    It 'selects revisionless versions for Ubuntu 24.04'
        When call select_npd_packages UBUNTU 24.04
        The status should be success
        The output should include 'node-problem-detector-kubernetes: 0.8.25'
        The output should include 'node-problem-detector-aks-config: 1.0.0'
    End

    It 'selects the 26.04 binary and the shared config for Ubuntu 26.04'
        When call select_npd_packages UBUNTU 26.04
        The status should be success
        The output should include 'node-problem-detector-kubernetes: 0.8.25'
        The output should include 'node-problem-detector-aks-config: 1.0.0'
    End

    It 'selects revisionless versions for Azure Linux 3'
        When call select_npd_packages AZURELINUX 3.0
        The status should be success
        The output should include 'node-problem-detector-kubernetes: 0.8.25'
        The output should include 'node-problem-detector-aks-config: 1.0.0'
    End

    select_npd_variant_packages() {
        local package
        while IFS= read -r package; do
            updatePackageVersions "${package}" AZURELINUX 3.0 OSGUARD
            printf '%s: %s\n' "$(jq -r .name <<< "${package}")" "${PACKAGE_VERSIONS[*]}"
        done < <(jq -c '.Packages[] | select(.name | startswith("node-problem-detector"))' parts/common/components.json)
    }

    It 'leaves Azure Linux OSGuard extension-managed'
        When call select_npd_variant_packages
        The status should be success
        The output should not include '0.8.25'
        The output should not include '1.0.0'
    End

    It 'leaves Mariner extension-managed'
        When call select_npd_packages MARINER 2.0
        The status should be success
        The output should not include '0.8.25'
        The output should not include '1.0.0'
    End
End

Describe 'NPD VHD installation helper'
    Include ./vhdbuilder/packer/install-npd.sh

    setup() {
        TEST_DIR="$(mktemp -d)"
        NPD_TDNF_LOCK_FILE="${TEST_DIR}/locks.d/node-problem-detector.conf"
        VHD_LOGS_FILEPATH="${TEST_DIR}/vhd.log"
        TEST_CALLS="${TEST_DIR}/calls"
        touch "${TEST_CALLS}"
        export TEST_CALLS
    }
    cleanup() { rm -rf "${TEST_DIR}"; }
    BeforeEach setup
    AfterEach cleanup

    isUbuntu() { return 1; }
    isAzureLinux() { return 0; }
    Mock tdnf
        echo "tdnf $*" >> "${TEST_CALLS}"
    End
    dnf_install() { echo "dnf_install $*" >> "${TEST_CALLS}"; }
    getLatestRPMPackageVersion() { printf '%s-16.azl3' "$2"; }
    logResolvedPackageVersion() { echo "resolved $*" >> "${TEST_CALLS}"; }

    It 'resolves the newest revision, installs and locks the Azure Linux binary without caching it'
        When call installNPDPackage node-problem-detector-kubernetes 0.8.25
        The status should be success
        The contents of file "${TEST_CALLS}" should equal 'resolved node-problem-detector-kubernetes 0.8.25 0.8.25-16.azl3
dnf_install 30 1 600 node-problem-detector-kubernetes-0.8.25-16.azl3'
        The contents of file "${NPD_TDNF_LOCK_FILE}" should equal 'node-problem-detector-kubernetes'
    End

    It 'fails when no package revision matches the pinned version'
        getLatestRPMPackageVersion() { return 0; }
        When call installNPDPackage node-problem-detector-kubernetes 9.9.9
        The status should be failure
        The output should include 'Failed to find node-problem-detector-kubernetes version 9.9.9'
        The contents of file "${TEST_CALLS}" should equal ''
        The path "${NPD_TDNF_LOCK_FILE}" should not be exist
    End

    It 'locks each Azure Linux package once'
        holdNPDPackage node-problem-detector-kubernetes
        When call holdNPDPackage node-problem-detector-kubernetes
        The status should be success
        The lines of contents of file "${NPD_TDNF_LOCK_FILE}" should equal 1
    End
End

Describe 'NPD VHD installation helper on Ubuntu'
    Include ./vhdbuilder/packer/install-npd.sh

    setup() {
        TEST_DIR="$(mktemp -d)"
        VHD_LOGS_FILEPATH="${TEST_DIR}/vhd.log"
        TEST_CALLS="${TEST_DIR}/calls"
        touch "${TEST_CALLS}"
        export TEST_CALLS
        APT_FAILS=false
    }
    cleanup() { rm -rf "${TEST_DIR}"; }
    BeforeEach setup
    AfterEach cleanup

    isUbuntu() { return 0; }
    isAzureLinux() { return 1; }
    npdConfigAptConf() { :; }
    apt_get_install() { echo "apt_get_install $*" >> "${TEST_CALLS}"; [ "${APT_FAILS}" = false ]; }
    getLatestDebPackageVersion() { printf '%s-ubuntu24.04u1' "$2"; }
    logResolvedPackageVersion() { echo "resolved $*" >> "${TEST_CALLS}"; }
    dpkg() { echo "dpkg $* $(cat)" >> "${TEST_CALLS}"; }
    apt-get() { echo "apt-get $*" >> "${TEST_CALLS}"; }
    systemctl() { echo "systemctl $*" >> "${TEST_CALLS}"; }
    ln() { :; }

    It 'propagates package installation failure without holding the package'
        APT_FAILS=true
        When call installNPDPackage node-problem-detector-kubernetes 0.8.25
        The status should be failure
        The contents of file "${TEST_CALLS}" should not include 'hold'
    End

    It 'installs, holds and caches the config package before validating it'
        When call installNPDPackage node-problem-detector-aks-config 1.0.0
        The contents of file "${TEST_CALLS}" should include 'resolved node-problem-detector-aks-config 1.0.0 1.0.0-ubuntu24.04u1'
        The contents of file "${TEST_CALLS}" should include 'apt_get_install 30 1 600 node-problem-detector-aks-config=1.0.0-ubuntu24.04u1'
        The contents of file "${TEST_CALLS}" should include 'apt-get download node-problem-detector-aks-config=1.0.0-ubuntu24.04u1'
        The contents of file "${TEST_CALLS}" should include 'dpkg --set-selections node-problem-detector-aks-config hold'
        # Package-owned executables are absent in the test image, so activation
        # assets must not be prepared and the capture marker must not be written.
        The status should be failure
        The output should include 'Missing executable'
        The contents of file "${TEST_CALLS}" should not include 'systemctl'
    End
End

Describe 'NPD VHD installation helper for releases sharing a config package'
    Include ./vhdbuilder/packer/install-npd.sh

    setup() {
        TEST_DIR="$(mktemp -d)"
        VHD_LOGS_FILEPATH="${TEST_DIR}/vhd.log"
        TEST_CALLS="${TEST_DIR}/calls"
        touch "${TEST_CALLS}"
        export TEST_CALLS
    }
    cleanup() { rm -rf "${TEST_DIR}"; }
    BeforeEach setup
    AfterEach cleanup

    isUbuntu() { return 0; }
    isAzureLinux() { return 1; }
    npdConfigAptConf() { printf '/tmp/npd-build/apt/apt.conf'; }
    apt-get() { echo "APT_CONFIG=${APT_CONFIG:-} apt-get $*" >> "${TEST_CALLS}"; }
    getLatestDebPackageVersion() { echo "resolve APT_CONFIG=${APT_CONFIG:-} $*" >> "${TEST_CALLS}"; printf '%s-ubuntu24.04u2' "$2"; }
    logResolvedPackageVersion() { :; }
    apt_get_install() { echo "apt_get_install $*" >> "${TEST_CALLS}"; }
    dpkg() { [ "$1" = --print-architecture ] && echo amd64 || echo "dpkg $* $(cat)" >> "${TEST_CALLS}"; }
    ln() { :; }

    It 'resolves, downloads and installs the config from the source release repository'
        When call installNPDPackage node-problem-detector-aks-config 1.0.0
        The output should include 'Missing executable'
        The status should be failure
        The contents of file "${TEST_CALLS}" should include 'resolve APT_CONFIG=/tmp/npd-build/apt/apt.conf node-problem-detector-aks-config 1.0.0'
        The contents of file "${TEST_CALLS}" should include 'APT_CONFIG=/tmp/npd-build/apt/apt.conf apt-get download node-problem-detector-aks-config=1.0.0-ubuntu24.04u2'
        The contents of file "${TEST_CALLS}" should include 'apt_get_install 30 1 600 /opt/node-problem-detector/downloads/node-problem-detector-aks-config_1.0.0-ubuntu24.04u2_amd64.deb'
        The contents of file "${TEST_CALLS}" should include 'dpkg --set-selections node-problem-detector-aks-config hold'
    End

    It 'uses the release repository for the binary'
        When call installNPDPackage node-problem-detector-kubernetes 0.8.25
        The status should be success
        The contents of file "${TEST_CALLS}" should include 'resolve APT_CONFIG= node-problem-detector-kubernetes 0.8.25'
        The contents of file "${TEST_CALLS}" should include 'apt_get_install 30 1 600 node-problem-detector-kubernetes=0.8.25-ubuntu24.04u2'
    End
End
