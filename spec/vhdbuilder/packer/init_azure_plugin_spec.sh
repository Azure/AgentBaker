#!/usr/bin/env bash

Describe 'init_packer_azure_plugin'
    Include './vhdbuilder/packer/init-azure-plugin.sh'

    setup() {
        export TEST_DIR
        TEST_DIR=$(mktemp -d)
        export PACKER_INIT_RESULT=success
        export CHECKSUM_RESULT=0
        export TEST_HOST_ARCH=x86_64
    }

    cleanup() {
        rm -f -- "$TEST_DIR"/*
        rmdir -- "$TEST_DIR"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Mock packer
        printf '%s\n' "$*" >> "$TEST_DIR/packer-calls"
        if [[ $1 == init ]]; then
            if [[ -f "$TEST_DIR/installed" || $PACKER_INIT_RESULT == success ]]; then
                echo 'Packer init succeeded'
                exit 0
            fi
            if [[ $PACKER_INIT_RESULT == missing-checksum ]]; then
                echo 'https://github.com/hashicorp/packer-plugin-azure/releases/download/v2.5.0/packer-plugin-azure_v2.5.0_SHA256SUMS: 404 Not Found' >&2
            else
                echo 'Invalid plugin configuration' >&2
            fi
            exit 1
        fi
        if [[ $1 == plugins && $2 == install ]]; then
            touch "$TEST_DIR/installed"
            exit 0
        fi
        exit 1
    End

    Mock uname
        case "$1" in
            -s) echo Linux ;;
            -m) echo "$TEST_HOST_ARCH" ;;
        esac
    End

    Mock curl
        printf '%s\n' "$7" > "$TEST_DIR/curl-url"
        printf 'archive' > "$6"
    End

    Mock sha256sum
        cat > "$TEST_DIR/checksum-input"
        exit "$CHECKSUM_RESULT"
    End

    Mock python3
        case "$4" in
            *_linux_amd64.zip) touch "$5/packer-plugin-azure_v2.5.0_x5.0_linux_amd64" ;;
            *_linux_arm64.zip) touch "$5/packer-plugin-azure_v2.5.0_x5.0_linux_arm64" ;;
        esac
        touch "$5/LICENSE.txt"
    End

    It 'uses the normal Packer init when the plugin is available'
        When call init_packer_azure_plugin
        The status should be success
        The output should include 'Packer init succeeded'
        The path "$TEST_DIR/curl-url" should not be exist
    End

    It 'propagates unrelated Packer init failures without downloading'
        PACKER_INIT_RESULT=invalid-config
        When call init_packer_azure_plugin
        The status should be failure
        The stderr should include 'Invalid plugin configuration'
        The path "$TEST_DIR/curl-url" should not be exist
    End

    It 'installs the verified amd64 plugin after the GitHub checksum 404'
        PACKER_INIT_RESULT=missing-checksum
        When call init_packer_azure_plugin
        The status should be success
        The output should include 'Installing verified Azure Packer plugin 2.5.0'
        The stderr should include 'packer-plugin-azure_v2.5.0_SHA256SUMS: 404'
        The path "$TEST_DIR/installed" should be exist
        The contents of file "$TEST_DIR/curl-url" should include 'packer-plugin-azure_2.5.0_linux_amd64.zip'
        The contents of file "$TEST_DIR/checksum-input" should include '0817b59e20eab4acac1a18b4d1186923abfd6f927ce842b0b754145a8a5dcc42'
    End

    It 'uses the verified arm64 plugin on an arm64 host'
        PACKER_INIT_RESULT=missing-checksum
        TEST_HOST_ARCH=aarch64
        When call init_packer_azure_plugin
        The status should be success
        The output should include 'Installing verified Azure Packer plugin 2.5.0'
        The stderr should include 'packer-plugin-azure_v2.5.0_SHA256SUMS: 404'
        The contents of file "$TEST_DIR/curl-url" should include 'packer-plugin-azure_2.5.0_linux_arm64.zip'
        The contents of file "$TEST_DIR/checksum-input" should include '326637b4472e35f1d34110da17402daeff83dbcac7f3ce4adb13a1595cf0e0d3'
    End

    It 'refuses to install an archive with a bad checksum'
        PACKER_INIT_RESULT=missing-checksum
        CHECKSUM_RESULT=1
        When call init_packer_azure_plugin
        The status should be failure
        The output should include 'Installing verified Azure Packer plugin 2.5.0'
        The stderr should include 'Azure Packer plugin archive checksum mismatch'
        The path "$TEST_DIR/installed" should not be exist
    End
End
