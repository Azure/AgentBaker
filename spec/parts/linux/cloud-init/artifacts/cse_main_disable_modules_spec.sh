#!/usr/bin/env shellspec

# Unit tests for vulnerable kernel module mitigation helpers in cse_main.sh
# and the OS gate that selects which OS variants get VHD-build-time apply.

load_kernel_mitigation_helpers() {
    UBUNTU_OS_NAME="UBUNTU"
    MARINER_OS_NAME="MARINER"
    MARINER_KATA_OS_NAME="MARINERKATA"
    AZURELINUX_KATA_OS_NAME="AZURELINUXKATA"
    AZURELINUX_OS_NAME="AZURELINUX"
    FLATCAR_OS_NAME="FLATCAR"
    ACL_OS_NAME="AZURECONTAINERLINUX"
    ACL_OS_VARIANT="AZURECONTAINERLINUX"
    AZURELINUX_OSGUARD_OS_VARIANT="OSGUARD"

    eval "$(sed -n '/^semverCompare()/,/^}/p' parts/linux/cloud-init/artifacts/cse_helpers.sh)"
    eval "$(sed -n '/^isACL()/,/^}/p' parts/linux/cloud-init/artifacts/cse_helpers.sh)"
    eval "$(sed -n '/^isMarinerOrAzureLinux()/,/^}/p' parts/linux/cloud-init/artifacts/cse_helpers.sh)"
    eval "$(sed -n '/^isAzureLinuxOSGuard()/,/^}/p' parts/linux/cloud-init/artifacts/cse_helpers.sh)"
    eval "$(sed -n '/^isUbuntu()/,/^}/p' parts/linux/cloud-init/artifacts/cse_helpers.sh)"
    eval "$(sed -n '/^ubuntuKernelNeedsVulnerableModuleMitigation()/,/^}/p' parts/linux/cloud-init/artifacts/cse_helpers.sh)"
}

Describe 'disableVulnerableKernelModule()'
    MODPROBE_DIR=""
    PROC_MODULES=""

    setup() {
        MODPROBE_DIR="$(mktemp -d)"
        PROC_MODULES="$(mktemp)"
        # Source only the function by extracting it
        eval "$(sed -n '/^disableVulnerableKernelModule()/,/^}/p' parts/linux/cloud-init/artifacts/cse_main.sh | \
            sed "s|/etc/modprobe.d|${MODPROBE_DIR}|g; s|/proc/modules|${PROC_MODULES}|g")"
    }

    cleanup() {
        rm -rf "$MODPROBE_DIR"
        rm -f "$PROC_MODULES"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    # Mock modprobe -r
    modprobe() { return 0; }

    It 'creates a config file for a single module'
        When call disableVulnerableKernelModule "algif_aead" "CVE-2026-31431 (Copy Fail)"
        The file "${MODPROBE_DIR}/disable-algif_aead.conf" should be exist
        The contents of file "${MODPROBE_DIR}/disable-algif_aead.conf" should include "install algif_aead /bin/false"
        The contents of file "${MODPROBE_DIR}/disable-algif_aead.conf" should include "blacklist algif_aead"
    End

    It 'creates separate config files per module'
        When call disableVulnerableKernelModule "esp4" "DirtyFrag ESP4"
        The file "${MODPROBE_DIR}/disable-esp4.conf" should be exist
        The contents of file "${MODPROBE_DIR}/disable-esp4.conf" should include "install esp4 /bin/false"
        The contents of file "${MODPROBE_DIR}/disable-esp4.conf" should include "blacklist esp4"
    End

    It 'is idempotent — running twice produces same content'
        first_run() {
            disableVulnerableKernelModule "rxrpc" "DirtyFrag RxRPC"
            cat "${MODPROBE_DIR}/disable-rxrpc.conf"
        }
        second_run() {
            disableVulnerableKernelModule "rxrpc" "DirtyFrag RxRPC"
            cat "${MODPROBE_DIR}/disable-rxrpc.conf"
        }
        When call first_run
        The output should eq "$(second_run)"
    End

    It 'attempts to unload a loaded module'
        loaded_test() {
            echo "rxrpc 425984 0" > "$PROC_MODULES"
            disableVulnerableKernelModule "rxrpc" "DirtyFrag RxRPC"
        }
        When call loaded_test
        The output should include "successfully unloaded rxrpc"
    End

    It 'does not attempt unload when module is not loaded'
        not_loaded_test() {
            : > "$PROC_MODULES"
            disableVulnerableKernelModule "rxrpc" "DirtyFrag RxRPC"
        }
        When call not_loaded_test
        The output should not include "unloaded"
    End
End

# Tests the Ubuntu kernel version gate used by the VHD-build-time apply.
Describe 'ubuntuKernelNeedsVulnerableModuleMitigation()'
    setup() {
        OS=""
        OS_VERSION=""
        OS_VARIANT=""
        UBUNTU_RELEASE=""
        KERNEL_RELEASE=""
        load_kernel_mitigation_helpers
    }

    BeforeEach 'setup'

    get_ubuntu_release() {
        echo "$UBUNTU_RELEASE"
    }

    uname() {
        if [ "$1" = "-r" ]; then
            echo "$KERNEL_RELEASE"
        fi
    }

    It 'requires the mitigation on Ubuntu 20.04'
        UBUNTU_RELEASE="20.04"
        KERNEL_RELEASE="5.4.0-1100-azure"
        When call ubuntuKernelNeedsVulnerableModuleMitigation
        The status should be success
        The output should include "Ubuntu 20.04 remains in scope"
    End

    It 'requires the mitigation on Ubuntu 22.04 kernels older than 5.15.0-1116-azure'
        UBUNTU_RELEASE="22.04"
        KERNEL_RELEASE="5.15.0-1115-azure"
        When call ubuntuKernelNeedsVulnerableModuleMitigation
        The status should be success
        The output should include "older than fixed kernel 5.15.0-1116-azure"
    End

    It 'skips the mitigation on Ubuntu 22.04 kernels at 5.15.0-1116-azure or newer'
        UBUNTU_RELEASE="22.04"
        KERNEL_RELEASE="5.15.0-1116-azure"
        When call ubuntuKernelNeedsVulnerableModuleMitigation
        The status should be failure
        The output should include "includes Copy Fail / DirtyFrag / Fragnesia fixes"
    End

    It 'skips the mitigation on Ubuntu 24.04 kernels at 6.8.0-1058-azure or newer'
        UBUNTU_RELEASE="24.04"
        KERNEL_RELEASE="6.8.0-1058-azure"
        When call ubuntuKernelNeedsVulnerableModuleMitigation
        The status should be failure
        The output should include "includes Copy Fail / DirtyFrag / Fragnesia fixes"
    End

    It 'keeps the mitigation enabled for unknown Ubuntu kernel flavors'
        UBUNTU_RELEASE="24.04"
        KERNEL_RELEASE="6.8.0-1058-custom"
        When call ubuntuKernelNeedsVulnerableModuleMitigation
        The status should be success
        The output should include "Unknown Ubuntu 24.04 kernel flavor"
    End

    It 'keeps the mitigation enabled when a fixed Ubuntu azure kernel has an unexpected suffix'
        UBUNTU_RELEASE="22.04"
        KERNEL_RELEASE="5.15.0-1116-azure-custom"
        When call ubuntuKernelNeedsVulnerableModuleMitigation
        The status should be success
        The output should include "Unknown Ubuntu 22.04 kernel flavor"
    End

    It 'skips the mitigation on future Ubuntu releases by default'
        UBUNTU_RELEASE="26.04"
        KERNEL_RELEASE="6.14.0-1000-azure"
        When call ubuntuKernelNeedsVulnerableModuleMitigation
        The status should be failure
        The output should include "not in the Copy Fail / DirtyFrag / Fragnesia mitigation scope"
    End
End

# Tests the OS gate that decides whether to call disableVulnerableKernelModule
# during VHD build. Apply on: Ubuntu 20.04, vulnerable Ubuntu 22.04 / 24.04
# kernels, Mariner/AzureLinux 2.0 (AzL2), AzureLinux OSGuard (defense-in-depth —
# hardened secure-boot variant intentionally retains the mitigation). Skip on:
# fixed Ubuntu 22.04 / 24.04 kernels, future Ubuntu releases, AzureLinux 3.0 regular/Kata
# (kernel 6.6.139.1-1.azl3+ has the upstream fix and customers reported the blacklist
# actively blocks legitimate workloads), ACL, Flatcar.
# See https://github.com/Azure/AKS/issues/5753.
Describe 'CVE kernel module mitigation OS gate'
    setup() {
        OS=""
        OS_VERSION=""
        OS_VARIANT=""
        UBUNTU_RELEASE=""
        KERNEL_RELEASE=""
        GATE_ACTIONS=""
        load_kernel_mitigation_helpers
        eval "$(sed -n '/^applyVulnerableKernelModuleMitigation()/,/^}/p' parts/linux/cloud-init/artifacts/cse_main.sh)"
    }

    BeforeEach 'setup'

    get_ubuntu_release() {
        echo "$UBUNTU_RELEASE"
    }

    uname() {
        if [ "$1" = "-r" ]; then
            echo "$KERNEL_RELEASE"
        fi
    }

    disableVulnerableKernelModule() {
        GATE_ACTIONS="${GATE_ACTIONS}APPLY:${1} "
    }

    gate() {
        applyVulnerableKernelModuleMitigation
        if [ -z "${GATE_ACTIONS}" ]; then
            echo "SKIP"
        else
            echo "${GATE_ACTIONS}"
        fi
    }

    It 'applies the mitigation on Ubuntu 20.04'
        OS="${UBUNTU_OS_NAME}"
        OS_VERSION="20.04"
        OS_VARIANT=""
        UBUNTU_RELEASE="20.04"
        KERNEL_RELEASE="5.4.0-1100-azure"
        When call gate
        The output should include "APPLY:algif_aead"
        The output should include "APPLY:esp4"
        The output should include "APPLY:esp6"
        The output should include "APPLY:rxrpc"
    End

    It 'applies the mitigation on vulnerable Ubuntu 22.04 kernels'
        OS="${UBUNTU_OS_NAME}"
        OS_VERSION="22.04"
        OS_VARIANT=""
        UBUNTU_RELEASE="22.04"
        KERNEL_RELEASE="5.15.0-1115-azure"
        When call gate
        The output should include "APPLY:algif_aead"
        The output should include "APPLY:esp4"
        The output should include "APPLY:esp6"
        The output should include "APPLY:rxrpc"
    End

    It 'skips on fixed Ubuntu 22.04 kernels'
        OS="${UBUNTU_OS_NAME}"
        OS_VERSION="22.04"
        OS_VARIANT=""
        UBUNTU_RELEASE="22.04"
        KERNEL_RELEASE="5.15.0-1116-azure"
        When call gate
        The output should include "SKIP"
        The output should not include "APPLY"
    End

    It 'skips on fixed Ubuntu 24.04 kernels'
        OS="${UBUNTU_OS_NAME}"
        OS_VERSION="24.04"
        OS_VARIANT=""
        UBUNTU_RELEASE="24.04"
        KERNEL_RELEASE="6.8.0-1058-azure"
        When call gate
        The output should include "SKIP"
        The output should not include "APPLY"
    End

    It 'applies the mitigation when a fixed Ubuntu generic kernel has an unexpected suffix'
        OS="${UBUNTU_OS_NAME}"
        OS_VERSION="24.04"
        OS_VARIANT=""
        UBUNTU_RELEASE="24.04"
        KERNEL_RELEASE="6.8.0-124-generic-custom"
        When call gate
        The output should include "APPLY:algif_aead"
    End

    It 'skips on future Ubuntu releases by default'
        OS="${UBUNTU_OS_NAME}"
        OS_VERSION="26.04"
        OS_VARIANT=""
        UBUNTU_RELEASE="26.04"
        KERNEL_RELEASE="6.14.0-1000-azure"
        When call gate
        The output should include "SKIP"
        The output should not include "APPLY"
    End

    It 'applies the mitigation on AzureLinux 3.0 OSGuard — defense-in-depth retained'
        OS="${AZURELINUX_OS_NAME}"
        OS_VERSION="3.0"
        OS_VARIANT="${AZURELINUX_OSGUARD_OS_VARIANT}"
        When call gate
        The output should include "APPLY:algif_aead"
        The output should include "APPLY:esp4"
        The output should include "APPLY:esp6"
        The output should include "APPLY:rxrpc"
    End

    It 'applies the mitigation on Mariner/AzureLinux 2.0 (AzL2) — VHDs are frozen so CSE-time apply is required'
        OS="${MARINER_OS_NAME}"
        OS_VARIANT=""
        OS_VERSION="2.0"
        When call gate
        The output should include "APPLY:algif_aead"
        The output should include "APPLY:esp4"
        The output should include "APPLY:esp6"
        The output should include "APPLY:rxrpc"
    End
    It 'applies the mitigation on Mariner Kata (AzL2) — VHDs are frozen so CSE-time apply is required'
        OS="${MARINER_KATA_OS_NAME}"
        OS_VARIANT=""
        OS_VERSION="2.0"
        When call gate
        The output should include "APPLY:algif_aead"
        The output should include "APPLY:esp4"
        The output should include "APPLY:esp6"
        The output should include "APPLY:rxrpc"
    End
    It 'skips on AzureLinux 3.0 regular (kernel 6.6.139.1-1.azl3+ has upstream fix)'
        OS="${AZURELINUX_OS_NAME}"
        OS_VERSION="3.0"
        OS_VARIANT=""
        When call gate
        The output should include "SKIP"
        The output should not include "APPLY"
    End

    It 'skips on AzureLinux 3.0 Kata (same kernel as AzL3 regular)'
        OS="${AZURELINUX_KATA_OS_NAME}"
        OS_VERSION="3.0"
        OS_VARIANT=""
        When call gate
        The output should include "SKIP"
        The output should not include "APPLY"
    End

    It 'skips on ACL (Flatcar-based; never in scope)'
        OS="${ACL_OS_NAME}"
        OS_VARIANT=""
        When call gate
        The output should include "SKIP"
        The output should not include "APPLY"
    End

    It 'skips on Flatcar (never in scope)'
        OS="${FLATCAR_OS_NAME}"
        OS_VARIANT=""
        When call gate
        The output should include "SKIP"
        The output should not include "APPLY"
    End
End

Describe 'CVE kernel module mitigation phase coverage'
    phase_body() {
        local phase="${1}"
        sed -n "/^function ${phase}/,/^}/p" parts/linux/cloud-init/artifacts/cse_main.sh
    }

    It 'runs mitigation from basePrep so VHD bakes keep the intended mitigation state'
        When call phase_body "basePrep"
        The output should include "applyVulnerableKernelModuleMitigation"
    End

    It 'does not run mitigation from nodePrep because this is not an AgentBakerSvc hotfix'
        When call phase_body "nodePrep"
        The output should not include "applyVulnerableKernelModuleMitigation"
    End
End
