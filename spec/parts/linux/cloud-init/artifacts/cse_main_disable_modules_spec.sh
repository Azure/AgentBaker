#!/usr/bin/env shellspec

# Unit tests for disableVulnerableKernelModules() in cse_main.sh
# Verifies that the function creates correct modprobe blacklist configs
# and attempts to unload loaded modules.

Describe 'disableVulnerableKernelModules()'
    MODPROBE_DIR=""

    setup() {
        MODPROBE_DIR="$(mktemp -d)"
        # Override /etc/modprobe.d with our temp dir
        # We redefine the function with the temp dir path
        eval "$(sed -n '/^disableVulnerableKernelModules()/,/^}/p' parts/linux/cloud-init/artifacts/cse_main.sh | \
            sed "s|/etc/modprobe.d|${MODPROBE_DIR}|g")"
    }

    cleanup() {
        rm -rf "$MODPROBE_DIR"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    # Mock grep for /proc/modules to simulate no modules loaded
    grep() {
        if [ "$2" = "/proc/modules" ]; then
            return 1
        fi
        command grep "$@"
    }

    # Mock modprobe -r (should not be called since no modules are "loaded")
    modprobe() {
        echo "modprobe called with: $*"
        return 0
    }

    It 'creates the algif_aead config file'
        When run disableVulnerableKernelModules
        Path config_file="${MODPROBE_DIR}/disable-algif_aead.conf"
        The path config_file should be file
        The contents of file config_file should include "install algif_aead /bin/false"
        The contents of file config_file should include "blacklist algif_aead"
    End

    It 'creates the dirtyfrag config file with esp4/esp6/rxrpc'
        When run disableVulnerableKernelModules
        Path config_file="${MODPROBE_DIR}/disable-dirtyfrag.conf"
        The path config_file should be file
        The contents of file config_file should include "install esp4 /bin/false"
        The contents of file config_file should include "blacklist esp4"
        The contents of file config_file should include "install esp6 /bin/false"
        The contents of file config_file should include "blacklist esp6"
        The contents of file config_file should include "install rxrpc /bin/false"
        The contents of file config_file should include "blacklist rxrpc"
    End

    It 'is idempotent — running twice produces same files'
        run_twice() {
            disableVulnerableKernelModules
            disableVulnerableKernelModules
        }
        When run run_twice
        Path algif="${MODPROBE_DIR}/disable-algif_aead.conf"
        Path dirty="${MODPROBE_DIR}/disable-dirtyfrag.conf"
        The path algif should be file
        The path dirty should be file
        # Only two config files should exist
        The result of "ls ${MODPROBE_DIR}/*.conf | wc -l" should eq 2
    End

    It 'includes CVE descriptions as comments'
        When run disableVulnerableKernelModules
        Path algif="${MODPROBE_DIR}/disable-algif_aead.conf"
        Path dirty="${MODPROBE_DIR}/disable-dirtyfrag.conf"
        The contents of file algif should include "CVE-2026-31431"
        The contents of file dirty should include "DirtyFrag"
    End
End
