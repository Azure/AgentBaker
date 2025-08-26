#!/bin/bash

Describe 'mariner-package-update.sh'
    Describe 'dnf_update'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/mariner/mariner-package-update.sh"

            TEST_DIR="/tmp/live-patching-test"

            OS_RELEASE_FILE="${TEST_DIR}/etc/os-release"
            mkdir -p $(dirname ${OS_RELEASE_FILE})

            golden_timestamp="20250814T000000Z"
        }

        cleanup() {
            rm -rf "${TEST_DIR}"
        }

        BeforeEach 'setup'

        AfterEach 'cleanup'

        Mock dnf
            echo "dnf mock called with args: $@"
        End

        Mock tdnf
            echo "tdnf mock called with args: $@"
        End

        It 'should be successful for Mariner 2.0'
            cat <<EOF > "${OS_RELEASE_FILE}"
NAME="Common Base Linux Mariner"
VERSION="2.0.20250701"
ID=mariner
VERSION_ID="2.0"
PRETTY_NAME="CBL-Mariner/Linux"
ANSI_COLOR="1;34"
HOME_URL="https://aka.ms/cbl-mariner"
BUG_REPORT_URL="https://aka.ms/cbl-mariner"
SUPPORT_URL="https://aka.ms/cbl-mariner"
EOF
            When run dnf_update
            The status should be success
            The output should include "dnf mock called with args: update --exclude mshv-linuxloader --exclude kernel-mshv --repo mariner-official-base --repo mariner-official-microsoft --repo mariner-official-extras --repo mariner-official-nvidia -y --refresh"
            The output should include "Executed dnf update -y --refresh 1 times"
        End

        It 'should be successful for AzureLinux 3.0'
            cat <<EOF > "${OS_RELEASE_FILE}"
NAME="Microsoft Azure Linux"
VERSION="3.0.20250702"
ID=azurelinux
VERSION_ID="3.0"
PRETTY_NAME="Microsoft Azure Linux 3.0"
ANSI_COLOR="1;34"
HOME_URL="https://aka.ms/azurelinux"
BUG_REPORT_URL="https://aka.ms/azurelinux"
SUPPORT_URL="https://aka.ms/azurelinux"
EOF

            When run dnf_update
            The status should be success
            # 1755129600 is the timestamp for 2025-08-14 00:00:00
            The output should include "using snapshottime 1755129600 for azurelinux 3.0 snapshot-based update"
            The output should include "tdnf mock called with args: --snapshottime 1755129600 update --exclude mshv-linuxloader --exclude kernel-mshv --repo azurelinux-official-base --repo azurelinux-official-ms-non-oss --repo azurelinux-official-ms-oss --repo azurelinux-official-nvidia -y --refresh"
            The output should include "Executed dnf update -y --refresh 1 times"
        End
    End
End