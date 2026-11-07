#!/bin/bash

Describe 'addMarinerNvidiaRepo'
    setup() {
        TEST_DIR=$(mktemp -d)
        eval "$(sed -n '/^addMarinerNvidiaRepo()/,/^}/p' './vhdbuilder/scripts/linux/mariner/tool_installs_mariner.sh' | sed "s|/etc/yum.repos.d|${TEST_DIR}|g")"
    }

    cleanup() {
        rm -rf "$TEST_DIR"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Describe 'Azure Linux 3'
        Parameters
            x86_64
            aarch64
        End

        It "leaves architecture substitution to the package manager on $1"
            OS_VERSION=3.0
            basearch="$1"

            When call addMarinerNvidiaRepo

            The status should be success
            The contents of file "${TEST_DIR}/azurelinux-nvidia.repo" should include "name=Azure Linux Official Nvidia 3.0 \$basearch"
            The contents of file "${TEST_DIR}/azurelinux-nvidia.repo" should include "baseurl=https://packages.microsoft.com/azurelinux/3.0/prod/nvidia/\$basearch/"
            The contents of file "${TEST_DIR}/azurelinux-nvidia.repo" should include 'gpgcheck=1'
            The contents of file "${TEST_DIR}/azurelinux-nvidia.repo" should include 'repo_gpgcheck=1'
            The contents of file "${TEST_DIR}/azurelinux-nvidia.repo" should include 'sslverify=1'
            The path "${TEST_DIR}/mariner-nvidia.repo" should not be exist
        End
    End

    It 'preserves the Mariner 2 repository'
        OS_VERSION=2.0

        When call addMarinerNvidiaRepo

        The status should be success
        The contents of file "${TEST_DIR}/mariner-nvidia.repo" should include 'baseurl=https://packages.microsoft.com/cbl-mariner/2.0/prod/nvidia/x86_64'
        The path "${TEST_DIR}/azurelinux-nvidia.repo" should not be exist
    End
End
Describe 'removeOldAzureLinuxKernels'
    setup() {
        TEST_DIR=$(mktemp -d)
        mkdir -p "${TEST_DIR}/boot"
        touch "${TEST_DIR}/boot/vmlinuz-6.6.159.1-2.azl3"
        INSTALLED="${TEST_DIR}/installed"
        REMOVED="${TEST_DIR}/removed"
        touch "$REMOVED"
        printf '%s\n' '100 kernel-6.6.159.1-1.azl3.x86_64' '200 kernel-6.6.159.1-2.azl3.x86_64' > "$INSTALLED"
        RUNNING_PACKAGE=kernel
        RUNNING_NEVRA=kernel-6.6.159.1-2.azl3.x86_64
        BLOCKED_REMOVAL=""
        eval "$(sed -n '/^removeOldAzureLinuxKernels()/,/^}/p' './vhdbuilder/scripts/linux/mariner/tool_installs_mariner.sh' | sed "s|/boot/|${TEST_DIR}/boot/|g")"
    }

    cleanup() {
        rm -rf "$TEST_DIR"
    }

    uname() {
        echo "6.6.159.1-2.azl3"
    }

    rpm() {
        case "$1 $2" in
            "-qf --queryformat")
                if [ "$3" = '%{NAME}\n' ]; then echo "$RUNNING_PACKAGE"; else echo "$RUNNING_NEVRA"; fi ;;
            "-q --queryformat")
                if [ "$3" = '%{INSTALLTIME} %{NAME}-%{VERSION}-%{RELEASE}.%{ARCH}\n' ]; then cat "$INSTALLED"; else cut -d' ' -f2 "$INSTALLED"; fi ;;
            "-e --test")
                [ "$3" != "$BLOCKED_REMOVAL" ] || { echo "$3 is needed by kernel-uki"; return 1; } ;;
            "-e "*)
                echo "$2" >> "$REMOVED"
                grep -vF " $2" "$INSTALLED" > "${INSTALLED}.tmp"; mv "${INSTALLED}.tmp" "$INSTALLED" ;;
        esac
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'removes only the superseded kernel package'
        When call removeOldAzureLinuxKernels
        The status should be success
        The output should include 'Removing superseded kernel kernel-6.6.159.1-1.azl3.x86_64'
        The contents of file "$REMOVED" should equal 'kernel-6.6.159.1-1.azl3.x86_64'
        The contents of file "$INSTALLED" should equal '200 kernel-6.6.159.1-2.azl3.x86_64'
    End

    It 'skips images that boot a different kernel package such as Kata kernel-mshv'
        RUNNING_PACKAGE=kernel-mshv
        When call removeOldAzureLinuxKernels
        The status should be success
        The output should include "provided by 'kernel-mshv'"
        The contents of file "$REMOVED" should equal ''
    End

    It 'skips when the running kernel is not the newest installed kernel'
        RUNNING_NEVRA=kernel-6.6.159.1-1.azl3.x86_64
        When call removeOldAzureLinuxKernels
        The status should be success
        The output should include 'is not the newest installed kernel'
        The contents of file "$REMOVED" should equal ''
    End

    It 'keeps an old kernel that another package depends on'
        BLOCKED_REMOVAL=kernel-6.6.159.1-1.azl3.x86_64
        When call removeOldAzureLinuxKernels
        The status should be success
        The output should include 'Keeping kernel-6.6.159.1-1.azl3.x86_64 because other packages depend on it'
        The contents of file "$REMOVED" should equal ''
    End
End