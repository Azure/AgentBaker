#!/bin/bash

Describe 'KMPP package selection'
    Include ./parts/linux/cloud-init/artifacts/cse_helpers.sh

    select_kmpp_packages() {
        local os="$1" release="$2" variant="${3:-}" package
        while IFS= read -r package; do
            updatePackageVersions "${package}" "${os}" "${release}" "${variant}"
            printf '%s: %s\n' "$(jq -r .name <<< "${package}")" "${PACKAGE_VERSIONS[*]}"
        done < <(jq -c '.Packages[] | select(.name == "kmpp-containers")' parts/common/components.json)
    }

    It 'bakes KMPP on Azure Linux 3.0'
        When call select_kmpp_packages AZURELINUX 3.0
        The status should be success
        The output should include 'kmpp-containers: 3.1.0_212'
    End

    Parameters
        'Azure Linux OSGuard' AZURELINUX 3.0 OSGUARD
        'Azure Linux Kata' AZURELINUXKATA 3.0 ''
        'Azure Container Linux' AZURELINUX 3.0 AZURECONTAINERLINUX
        'Mariner 2.0' MARINER 2.0 ''
        'Ubuntu 24.04' UBUNTU 24.04 ''
        'Flatcar' FLATCAR 4152.2.3 ''
    End

    It "skips KMPP on $1"
        When call select_kmpp_packages "$2" "$3" "$4"
        The status should be success
        The output should include 'kmpp-containers: '
        The output should not include '3.1.0_212'
        The output should not include '0.1.6'
    End
End

Describe 'KMPP VHD installation helpers'
    Include ./vhdbuilder/packer/install-kmpp.sh

    setup() {
        TEST_DIR="$(mktemp -d)"
        KMPP_DISABLE_DEFAULT_PROVIDER_FILE="${TEST_DIR}/etc/kmpp/disableDefaultProvider"
        KMPP_TDNF_LOCK_FILE="${TEST_DIR}/etc/tdnf/locks.d/kmpp.conf"
        KMPP_SYSTEMD_DIR="${TEST_DIR}/usr/lib/systemd/system"
        KMPP_DATA_DIR="${TEST_DIR}/var/opt/msft/ap/data"
        VHD_LOGS_FILEPATH="${TEST_DIR}/vhd.log"
        TEST_CALLS="${TEST_DIR}/calls"
        KMPP_GROUPS="kmpp kmppcert"
        mkdir -p "${KMPP_SYSTEMD_DIR}" "${KMPP_DATA_DIR}/kmpp-private/private" "${KMPP_DATA_DIR}/kmpp/certs"
        echo default > "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service"
        echo tpm > "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service.tpm"
        touch "${TEST_CALLS}"
        export TEST_CALLS
    }
    cleanup() { rm -rf "${TEST_DIR}"; }
    BeforeEach setup
    AfterEach cleanup

    getLatestRPMPackageVersion() { printf '%s-1' "$2"; }
    logResolvedPackageVersion() { echo "resolved $*" >> "${TEST_CALLS}"; }
    dnf_install() {
        echo "dnf_install $*" >> "${TEST_CALLS}"
        # The postinst must see the opt-out file, so record whether it existed at install time.
        [ -f "${KMPP_DISABLE_DEFAULT_PROVIDER_FILE}" ] && echo "optout present" >> "${TEST_CALLS}"
        return 0
    }
    systemctl() { echo "systemctl $*" >> "${TEST_CALLS}"; }
    gpasswd() { echo "gpasswd $*" >> "${TEST_CALLS}"; }
    id() { echo "${KMPP_GROUPS}"; }

    Describe 'installKMPP'
        It 'opts out of the default provider before install, then holds, masks and wipes state'
            When call installKMPP 3.1.0_212
            The status should be success
            The output should include 'Installing kmpp-containers version 3.1.0_212'
            The path "${KMPP_DISABLE_DEFAULT_PROVIDER_FILE}" should be file
            The contents of file "${TEST_CALLS}" should equal 'resolved kmpp-containers 3.1.0_212 3.1.0_212-1
dnf_install 30 1 600 kmpp-containers-3.1.0_212-1
optout present
systemctl daemon-reload
systemctl stop dbus-com.microsoft.kmpp.service dbus-com.microsoft.kmppctrl.service
systemctl disable dbus-com.microsoft.kmpp.service dbus-com.microsoft.kmppctrl.service
systemctl mask dbus-com.microsoft.kmpp.service dbus-com.microsoft.kmppctrl.service'
            The contents of file "${KMPP_TDNF_LOCK_FILE}" should equal 'kmpp-containers'
            The path "${KMPP_DATA_DIR}/kmpp-private" should not be exist
            The path "${KMPP_DATA_DIR}/kmpp" should not be exist
            The contents of file "${VHD_LOGS_FILEPATH}" should equal '  - kmpp-containers version 3.1.0_212-1'
        End

        It 'reverts the TPM unit swap and tss membership from the build VM'
            mv "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service" "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service.default"
            mv "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service.tpm" "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service"
            KMPP_GROUPS="kmpp kmppcert tss"
            When call installKMPP 3.1.0_212
            The status should be success
            The output should include 'Reverting TPM unit swap'
            The contents of file "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service" should equal 'default'
            The contents of file "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service.tpm" should equal 'tpm'
            The path "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service.default" should not be exist
            The contents of file "${TEST_CALLS}" should include 'gpasswd -d kmpp tss'
        End

        It 'leaves the units alone when no TPM swap happened'
            When call installKMPP 3.1.0_212
            The status should be success
            The output should not include 'Reverting TPM unit swap'
            The contents of file "${KMPP_SYSTEMD_DIR}/dbus-com.microsoft.kmpp.service" should equal 'default'
            The contents of file "${TEST_CALLS}" should not include 'gpasswd'
        End

        It 'fails when no package matches the pinned version'
            getLatestRPMPackageVersion() { return 1; }
            When call installKMPP 9.9.9
            The status should be failure
            The output should include 'Failed to find valid kmpp-containers version for 9.9.9'
            The contents of file "${TEST_CALLS}" should not include 'dnf_install'
        End

        It 'fails when the install fails'
            dnf_install() { return 1; }
            When call installKMPP 3.1.0_212
            The status should be failure
            The output should include 'Installing kmpp-containers'
            The path "${KMPP_TDNF_LOCK_FILE}" should not be exist
        End
    End

End
