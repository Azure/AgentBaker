#!/bin/bash

Describe 'Azure Linux 3 Kata payload copies'
    Include './parts/root/common/opt/azure/containers/provision_source.sh'
    Include './parts/root/shared/opt/azure/image-build/packer_source.sh'

    setup() {
        TEST_ROOT=$(mktemp -d)
        export OS=azurelinux OS_VERSION=3.0 OS_VARIANT=''
        export UBUNTU_RELEASE=unused ENABLE_FIPS=false CPU_ARCH=amd64
        export FEATURE_FLAGS=kata,kata-cc
        SIGNED_FILES='kata-containers.img kata-containers-cc.img kata-containers-igvm.img kata-containers-igvm-debug.img igvm-measurement.cose reference-info-base64'
        ORDINARY_GUEST="$TEST_ROOT/usr/share/kata-containers/kata-containers.img"
        CONFIDENTIAL_DIR="$TEST_ROOT/opt/confidential-containers/share/kata-containers"
        command mkdir -p "$TEST_ROOT/home/packer/kata-cc"
        printf 'ordinary guest\n' > "$TEST_ROOT/home/packer/kata-containers.img"
        local file
        for file in $SIGNED_FILES; do
            printf 'signed %s\n' "$file" > "$TEST_ROOT/home/packer/kata-cc/$file"
            chmod 0600 "$TEST_ROOT/home/packer/kata-cc/$file"
        done
    }

    cleanup() {
        rm -rf "$TEST_ROOT"
    }

    install() {
        command install "$1" "$2" "$TEST_ROOT$3" "$TEST_ROOT$4" || exit 1
    }
    mkdir() {
        command mkdir "$1" "$TEST_ROOT$2"
    }


    signed_payload_is_intact() {
        local file
        for file in $SIGNED_FILES; do
            cmp "$TEST_ROOT/home/packer/kata-cc/$file" "$CONFIDENTIAL_DIR/$file" || return 1
            [ "$(stat -c '%a' "$CONFIDENTIAL_DIR/$file")" = 644 ] || return 1
        done
        printf 'intact\n'
    }

    ordinary_guest_mode() {
        stat -c '%a' "$ORDINARY_GUEST"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'copies all six signed files with mode 0644 and preserves the ordinary guest'
        When run installKataPayload
        The status should be success
        The result of function signed_payload_is_intact should equal intact
        The contents of file "$ORDINARY_GUEST" should equal 'ordinary guest'
        The result of function ordinary_guest_mode should equal 755
    End

    It 'copies only the ordinary guest when confidential Kata is disabled'
        FEATURE_FLAGS=kata
        rm -rf "$TEST_ROOT/home/packer/kata-cc"
        When run installKataPayload
        The status should be success
        The contents of file "$ORDINARY_GUEST" should equal 'ordinary guest'
        The path "$CONFIDENTIAL_DIR" should not be exist
    End

    Describe 'missing required signed payload'
        Parameters
            kata-containers.img
            kata-containers-cc.img
            kata-containers-igvm.img
            kata-containers-igvm-debug.img
            igvm-measurement.cose
            reference-info-base64
        End

        It "fails when $1 is absent"
            rm "$TEST_ROOT/home/packer/kata-cc/$1"
            When run installKataPayload
            The status should be failure
            The stderr should include "$TEST_ROOT/home/packer/kata-cc/$1"
        End
    End
End
