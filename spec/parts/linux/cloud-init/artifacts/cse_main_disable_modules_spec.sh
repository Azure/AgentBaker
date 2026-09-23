#!/usr/bin/env shellspec

# Unit tests for vulnerable kernel module mitigation helpers in cse_main.sh
# and the OS gate that selects which OS variants get mitigation apply or cleanup.

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

Describe 'removeVulnerableKernelModuleDenyRules()'
    MODPROBE_DIR=""

    setup() {
        MODPROBE_DIR="$(mktemp -d)"
        eval "$(sed -n '/^removeVulnerableKernelModuleDenyRules()/,/^}/p' parts/linux/cloud-init/artifacts/cse_main.sh | \
            sed "s|/etc/modprobe.d|${MODPROBE_DIR}|g")"
    }

    cleanup() {
        rm -rf "$MODPROBE_DIR"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'removes stale Copy Fail / DirtyFrag / Fragnesia deny rules'
        cat > "${MODPROBE_DIR}/modprobe-CIS.conf" <<'EOF'
install algif_aead /bin/false
blacklist algif_aead
install esp4 /bin/false
blacklist esp4
install esp6 /bin/false
blacklist esp6
install rxrpc /bin/false
blacklist rxrpc
install cramfs /bin/false
blacklist cramfs
EOF
        When call removeVulnerableKernelModuleDenyRules
        The status should be success
        The contents of file "${MODPROBE_DIR}/modprobe-CIS.conf" should not include "algif_aead"
        The contents of file "${MODPROBE_DIR}/modprobe-CIS.conf" should not include "esp4"
        The contents of file "${MODPROBE_DIR}/modprobe-CIS.conf" should not include "esp6"
        The contents of file "${MODPROBE_DIR}/modprobe-CIS.conf" should not include "rxrpc"
        The contents of file "${MODPROBE_DIR}/modprobe-CIS.conf" should include "install cramfs /bin/false"
        The contents of file "${MODPROBE_DIR}/modprobe-CIS.conf" should include "blacklist cramfs"
        The output should include "Removed Copy Fail / DirtyFrag / Fragnesia module deny rules"
    End

    It 'removes stale deny rules with trailing comments and preserves unrelated content'
        cat > "${MODPROBE_DIR}/custom.conf" <<'EOF'
# keep this comment
install algif_aead /bin/false # stale mitigation
blacklist esp4 # stale mitigation
options dummy value=1
blacklist udf
EOF
        When call removeVulnerableKernelModuleDenyRules
        The status should be success
        The contents of file "${MODPROBE_DIR}/custom.conf" should include "# keep this comment"
        The contents of file "${MODPROBE_DIR}/custom.conf" should include "options dummy value=1"
        The contents of file "${MODPROBE_DIR}/custom.conf" should include "blacklist udf"
        The contents of file "${MODPROBE_DIR}/custom.conf" should not include "algif_aead"
        The contents of file "${MODPROBE_DIR}/custom.conf" should not include "esp4"
        The output should include "Removed Copy Fail / DirtyFrag / Fragnesia module deny rules"
    End
End

# Tests the Ubuntu kernel version gate used by the runtime apply/cleanup decision.
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

    It 'skips the mitigation on Ubuntu 24.04 CVM azure-fde kernels at the fixed ABI or newer'
        UBUNTU_RELEASE="24.04"
        KERNEL_RELEASE="6.8.0-1061-azure-fde"
        When call ubuntuKernelNeedsVulnerableModuleMitigation
        The status should be failure
        The output should include "includes Copy Fail / DirtyFrag / Fragnesia fixes"
    End

    It 'requires the mitigation on Ubuntu 24.04 CVM azure-fde kernels older than the fixed ABI'
        UBUNTU_RELEASE="24.04"
        KERNEL_RELEASE="6.8.0-1050-azure-fde"
        When call ubuntuKernelNeedsVulnerableModuleMitigation
        The status should be success
        The output should include "older than fixed kernel 6.8.0-1058-azure"
    End

    It 'skips the mitigation on Ubuntu 22.04 azure-fips kernels at the fixed ABI or newer'
        UBUNTU_RELEASE="22.04"
        KERNEL_RELEASE="5.15.0-1116-azure-fips"
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

    It 'keeps the mitigation enabled when the Ubuntu release cannot be detected'
        UBUNTU_RELEASE=""
        KERNEL_RELEASE="6.8.0-1058-azure"
        When call ubuntuKernelNeedsVulnerableModuleMitigation
        The status should be success
        The output should include "Unable to detect Ubuntu release"
    End

    It 'skips the mitigation on future Ubuntu releases by default'
        UBUNTU_RELEASE="26.04"
        KERNEL_RELEASE="6.14.0-1000-azure"
        When call ubuntuKernelNeedsVulnerableModuleMitigation
        The status should be failure
        The output should include "not in the Copy Fail / DirtyFrag / Fragnesia mitigation scope"
    End
End

# Exercise the Ubuntu 20.04 Azure FIPS exception across all four shell policies.
# Extract only the functions: never run VHD setup/cleanup or real modprobe operations.
Describe 'Ubuntu 20.04 Azure FIPS mitigation policy consistency'
    get_ubuntu_release() { echo "$UBUNTU_RELEASE"; }
    uname() { echo "$KERNEL_RELEASE"; }
    awk() { echo "$UBUNTU_RELEASE"; }

    verify_policy() {
        UBUNTU_RELEASE="$1"
        OS_VERSION="$1"
        KERNEL_RELEASE="$2"
        local expected="$3"
        local actual
        local source_file
        local output

        load_kernel_mitigation_helpers
        if output="$(ubuntuKernelNeedsVulnerableModuleMitigation)"; then
            actual="keep"
        else
            actual="remove"
        fi
        if [ "$actual" != "$expected" ]; then
            echo "CSE: expected ${expected}, got ${actual}: ${output}"
            return 1
        fi

        for source_file in \
            vhdbuilder/packer/packer_source.sh \
            vhdbuilder/packer/cleanup-vhd.sh \
            vhdbuilder/packer/test/linux-vhd-content-test.sh; do
            eval "$(sed -n '/^kernelVersionGe()/,/^}/p' "$source_file")"
            eval "$(sed -n '/^ubuntuKernelIncludesVulnerableModuleFixes()/,/^}/p' "$source_file")"
            if ubuntuKernelIncludesVulnerableModuleFixes "$UBUNTU_RELEASE"; then
                actual="remove"
            else
                actual="keep"
            fi
            if [ "$actual" != "$expected" ]; then
                echo "${source_file}: expected ${expected}, got ${actual}"
                return 1
            fi
        done
    }

    Parameters
        "20.04" "5.4.0-1162-azure-fips" "keep"
        "20.04" "5.4.0-1163-azure-fips" "keep"
        "20.04" "5.4.0-1164-azure-fips" "remove"
        "20.04" "5.4.0-1165-azure-fips" "remove"
        "20.04" "5.4.0-1200-azure-fips" "remove"
        "20.04" "5.4.0-1164-azure" "keep"
        "20.04" "5.4.0-1200-azure" "keep"
        "20.04" "5.4.0-1164-azure-fde" "keep"
        "20.04" "5.15.0-1114-azure-fde" "keep"
        "20.04" "5.4.0-1164-generic" "keep"
        "20.04" "5.4.0-1164-azure-nvidia" "keep"
        "20.04" "6.14.0-1010-azure-nvidia" "keep"
        "20.04" "5.15.0-1116-azure-fips" "keep"
        "20.04" "6.8.0-1058-azure-fips" "keep"
        "20.04" "5.4.1-1164-azure-fips" "keep"
        "20.04" "5.4.0-1164-azure-fips-custom" "keep"
        "20.04" "5.4.0-1164.170-azure-fips" "keep"
        "20.04" "5.4.0-1164custom-azure-fips" "keep"
        "20.04" "5.4.0-unknown-azure-fips" "keep"
        "20.04" "unknown" "keep"
        "20.04" "" "keep"
        "" "5.4.0-1164-azure-fips" "keep"
        "" "" "keep"
        "22.04" "5.15.0-1115-azure" "keep"
        "22.04" "5.15.0-1116-azure" "remove"
        "22.04" "5.15.0-1116-azure-fips" "remove"
        "22.04" "5.15.0-1120-azure-fde" "remove"
        "24.04" "6.8.0-1057-azure" "keep"
        "24.04" "6.8.0-1058-azure" "remove"
        "24.04" "6.8.0-1061-azure-fde" "remove"
        "24.04" "6.8.0-1058-azure-fips" "remove"
        "24.04" "6.14.0-1010-azure" "remove"
    End

    It 'only exempts fixed 5.4 Azure FIPS kernels on 20.04 and preserves later Ubuntu behavior'
        When call verify_policy "$1" "$2" "$3"
        The status should be success
        The output should be blank
    End
End

# Tests the OS gate that decides whether to apply or remove vulnerable-module
# deny rules during VHD build and provisioning. Apply on: other Ubuntu 20.04, vulnerable Ubuntu 22.04 / 24.04
# kernels, Mariner/AzureLinux 2.0 (AzL2), AzureLinux OSGuard (defense-in-depth —
# hardened secure-boot variant intentionally retains the mitigation). Remove stale deny rules on
# fixed Ubuntu 20.04 Azure FIPS 5.4, fixed Ubuntu 22.04 / 24.04 kernels and future Ubuntu releases. Skip on AzureLinux
# 3.0 regular/Kata (kernel 6.6.139.1-1.azl3+ has the upstream fix and customers reported
# the blacklist actively blocks legitimate workloads), ACL, Flatcar.
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
        eval "$(sed -n '/^reconcileVulnerableKernelModuleMitigation()/,/^}/p' parts/linux/cloud-init/artifacts/cse_main.sh)"
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

    removeVulnerableKernelModuleDenyRules() {
        GATE_ACTIONS="${GATE_ACTIONS}REMOVE "
    }

    gate() {
        reconcileVulnerableKernelModuleMitigation
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

    It 'removes stale deny rules on fixed Ubuntu 22.04 kernels'
        OS="${UBUNTU_OS_NAME}"
        OS_VERSION="22.04"
        OS_VARIANT=""
        UBUNTU_RELEASE="22.04"
        KERNEL_RELEASE="5.15.0-1116-azure"
        When call gate
        The output should include "REMOVE"
        The output should not include "APPLY"
    End

    It 'removes stale deny rules on fixed Ubuntu 24.04 kernels'
        OS="${UBUNTU_OS_NAME}"
        OS_VERSION="24.04"
        OS_VARIANT=""
        UBUNTU_RELEASE="24.04"
        KERNEL_RELEASE="6.8.0-1058-azure"
        When call gate
        The output should include "REMOVE"
        The output should not include "APPLY"
    End

    It 'removes stale deny rules on fixed Ubuntu 20.04 Azure FIPS kernels'
        OS="${UBUNTU_OS_NAME}"
        OS_VERSION="20.04"
        UBUNTU_RELEASE="20.04"
        KERNEL_RELEASE="5.4.0-1164-azure-fips"
        When call gate
        The output should include "REMOVE"
        The output should not include "APPLY"
    End

    It 'applies all four deny rules on older Ubuntu 20.04 Azure FIPS kernels'
        OS="${UBUNTU_OS_NAME}"
        OS_VERSION="20.04"
        UBUNTU_RELEASE="20.04"
        KERNEL_RELEASE="5.4.0-1163-azure-fips"
        When call gate
        The output should include "APPLY:algif_aead"
        The output should include "APPLY:esp4"
        The output should include "APPLY:esp6"
        The output should include "APPLY:rxrpc"
        The output should not include "REMOVE"
    End

    It 'applies all four deny rules on Ubuntu 20.04 CVM kernels'
        OS="${UBUNTU_OS_NAME}"
        OS_VERSION="20.04"
        UBUNTU_RELEASE="20.04"
        KERNEL_RELEASE="5.15.0-1114-azure-fde"
        When call gate
        The output should include "APPLY:algif_aead"
        The output should include "APPLY:esp4"
        The output should include "APPLY:esp6"
        The output should include "APPLY:rxrpc"
        The output should not include "REMOVE"
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

    It 'removes stale deny rules on future Ubuntu releases by default'
        OS="${UBUNTU_OS_NAME}"
        OS_VERSION="26.04"
        OS_VARIANT=""
        UBUNTU_RELEASE="26.04"
        KERNEL_RELEASE="6.14.0-1000-azure"
        When call gate
        The output should include "REMOVE"
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

    It 'reconciles mitigation from basePrep so VHD bakes keep the intended mitigation state'
        When call phase_body "basePrep"
        The output should include "reconcileVulnerableKernelModuleMitigation"
    End

    It 'reconciles mitigation from nodePrep for PIS and already-released VHDs'
        When call phase_body "nodePrep"
        The output should include "reconcileVulnerableKernelModuleMitigation"
    End

    It 'configures transparent huge page from basePrep so VHD bakes keep the intended state'
        When call phase_body "basePrep"
        The output should include "configureTransparentHugePage"
    End

    It 'applies and reconciles transparent huge page from nodePrep for PIS and already-released VHDs'
        When call phase_body "nodePrep"
        The output should include "applyTransparentHugePageValues"
        The output should include "reconcileTransparentHugePagePersistence"
    End
End
