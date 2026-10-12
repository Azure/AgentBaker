#!/bin/bash

Describe 'NPD package selection'
    Include ./parts/linux/cloud-init/artifacts/cse_helpers.sh

    select_npd_packages() {
        local os="$1" release="$2" package
        while IFS= read -r package; do
            updatePackageVersions "${package}" "${os}" "${release}" ''
            printf '%s: %s\n' "$(jq -r .name <<< "${package}")" "${PACKAGE_VERSIONS[*]}"
        done < <(jq -c '.Packages[] | select(.name | startswith("node-problem-detector"))' parts/common/components.json)
    }

    It 'selects both packages for Ubuntu 24.04'
        When call select_npd_packages UBUNTU 24.04
        The status should be success
        The output should include 'node-problem-detector-kubernetes: 0.8.25-ubuntu24.04u16'
        The output should include 'node-problem-detector-aks-config: 1.0.0-ubuntu24.04u1'
    End

    It 'leaves unpublished Ubuntu 26.04 packages empty'
        When call select_npd_packages UBUNTU 26.04
        The status should be success
        The output should not include 'ubuntu26.04u'
    End

    It 'selects exact RPM releases for Azure Linux 3'
        When call select_npd_packages AZURELINUX 3.0
        The status should be success
        The output should include 'node-problem-detector-kubernetes: 0.8.25-16.azl3'
        The output should include 'node-problem-detector-aks-config: 1.0.0-1.azl3'
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
        The output should not include 'azl3'
    End

    It 'leaves Mariner extension-managed'
        When call select_npd_packages MARINER 2.0
        The status should be success
        The output should not include 'ubuntu'
        The output should not include 'azl'
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

    It 'installs and locks the Azure Linux binary without caching it'
        When call installNPDPackage node-problem-detector-kubernetes 0.8.25-16.azl3
        The status should be success
        The contents of file "${TEST_CALLS}" should equal 'dnf_install 30 1 600 node-problem-detector-kubernetes-0.8.25-16.azl3'
        The contents of file "${NPD_TDNF_LOCK_FILE}" should equal 'node-problem-detector-kubernetes'
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
    apt_get_install() { echo "apt_get_install $*" >> "${TEST_CALLS}"; [ "${APT_FAILS}" = false ]; }
    apt-mark() { echo "apt-mark $*" >> "${TEST_CALLS}"; }
    apt-get() { echo "apt-get $*" >> "${TEST_CALLS}"; }
    systemctl() { echo "systemctl $*" >> "${TEST_CALLS}"; }
    ln() { :; }

    It 'propagates package installation failure without holding the package'
        APT_FAILS=true
        When call installNPDPackage node-problem-detector-kubernetes 0.8.25-ubuntu24.04u16
        The status should be failure
        The contents of file "${TEST_CALLS}" should not include 'apt-mark'
    End

    It 'installs, holds and caches the config package before validating it'
        When call installNPDPackage node-problem-detector-aks-config 1.0.0-ubuntu24.04u1
        The contents of file "${TEST_CALLS}" should include 'apt_get_install 30 1 600 node-problem-detector-aks-config=1.0.0-ubuntu24.04u1'
        The contents of file "${TEST_CALLS}" should include 'apt-get download node-problem-detector-aks-config=1.0.0-ubuntu24.04u1'
        The contents of file "${TEST_CALLS}" should include 'apt-mark hold node-problem-detector-aks-config'
        # Package-owned executables are absent in the test image, so activation
        # assets must not be prepared and the capture marker must not be written.
        The status should be failure
        The output should include 'Missing executable'
        The contents of file "${TEST_CALLS}" should not include 'systemctl'
    End
End
