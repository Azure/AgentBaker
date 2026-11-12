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