#!/usr/bin/env shellspec

# Unit tests for disableVulnerableKernelModule() in cse_main.sh
# and the OS gate that selects which OS variants get the runtime apply.

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

# Tests the OS gate that decides whether to call disableVulnerableKernelModule
# at CSE provisioning time. Apply on: Ubuntu, Mariner/AzureLinux 2.0 (AzL2), AzureLinux OSGuard
# (defense-in-depth — hardened secure-boot variant intentionally retains the mitigation). Skip on:
# AzureLinux 3.0 regular/Kata (kernel 6.6.139.1-1.azl3+ has the upstream fix and
# customers reported the blacklist actively blocks legitimate workloads), ACL, Flatcar.
# See https://github.com/Azure/AKS/issues/5753.
Describe 'CVE kernel module mitigation OS gate'
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"

    gate() {
        # Mirrors the condition in cse_main.sh basePrep — must be kept in sync.
        if isUbuntu "$OS" || isAzureLinuxOSGuard "$OS" "$OS_VARIANT" || { isMarinerOrAzureLinux "$OS" && [ "${OS_VERSION}" = "2.0" ]; }; then
            echo "APPLY"
        else
            echo "SKIP"
        fi
    }

    It 'applies the mitigation on Ubuntu'
        OS="${UBUNTU_OS_NAME}"
        OS_VARIANT=""
        When call gate
        The output should equal "APPLY"
    End

    It 'applies the mitigation on AzureLinux 3.0 OSGuard — defense-in-depth retained'
        OS="${AZURELINUX_OS_NAME}"
        OS_VARIANT="${AZURELINUX_OSGUARD_OS_VARIANT}"
        When call gate
        The output should equal "APPLY"
    End

    It 'applies the mitigation on Mariner/AzureLinux 2.0 (AzL2) — VHDs are frozen so CSE-time apply is required'
        OS="${MARINER_OS_NAME}"
        OS_VARIANT=""
        OS_VERSION="2.0"
        When call gate
        The output should equal "APPLY"
    End
    It 'applies the mitigation on Mariner Kata (AzL2) — VHDs are frozen so CSE-time apply is required'
        OS="${MARINER_KATA_OS_NAME}"
        OS_VARIANT=""
        OS_VERSION="2.0"
        When call gate
        The output should equal "APPLY"
    End
    It 'skips on AzureLinux 3.0 regular (kernel 6.6.139.1-1.azl3+ has upstream fix)'
        OS="${AZURELINUX_OS_NAME}"
        OS_VARIANT=""
        When call gate
        The output should equal "SKIP"
    End

    It 'skips on AzureLinux 3.0 Kata (same kernel as AzL3 regular)'
        OS="${AZURELINUX_KATA_OS_NAME}"
        OS_VARIANT=""
        When call gate
        The output should equal "SKIP"
    End

    It 'skips on ACL (Flatcar-based; never in scope)'
        OS="${ACL_OS_NAME}"
        OS_VARIANT=""
        When call gate
        The output should equal "SKIP"
    End

    It 'skips on Flatcar (never in scope)'
        OS="${FLATCAR_OS_NAME}"
        OS_VARIANT=""
        When call gate
        The output should equal "SKIP"
    End
End
