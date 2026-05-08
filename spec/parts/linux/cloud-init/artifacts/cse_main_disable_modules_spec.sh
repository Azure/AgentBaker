#!/usr/bin/env shellspec

# Unit tests for disableVulnerableKernelModule() in cse_main.sh

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
        The contents of file "${MODPROBE_DIR}/disable-algif_aead.conf" should include "CVE-2026-31431"
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
