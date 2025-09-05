#!/bin/bash

Describe 'ubuntu-snapshot-update.sh'
    setup() {
        Include ./parts/linux/cloud-init/artifacts/ubuntu/ubuntu-snapshot-update.sh
        TEST_DIR="/tmp/live-patching-test"
        mkdir -p ${TEST_DIR}
        SECURITY_PATCH_CONFIG_DIR="${TEST_DIR}"
        KUBECONFIG="${TEST_DIR}/kubeconfig"
        touch "${KUBECONFIG}"
        KUBECTL="kubectl"
    }
    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Mock apt_get_update 
        echo "apt_get_update mock called"
    End
    Mock unattended-upgrade
        echo "unattended-upgrade mock called"
    End

    It 'should update successfully for regular cluster'
        Mock lsb_release
            echo "jammy"     
        End
        sources_list=$(cat <<EOF
deb https://snapshot.ubuntu.com/ubuntu/20250815T000000Z jammy main restricted
deb https://snapshot.ubuntu.com/ubuntu/20250815T000000Z jammy-updates main restricted
deb https://snapshot.ubuntu.com/ubuntu/20250815T000000Z jammy universe
deb https://snapshot.ubuntu.com/ubuntu/20250815T000000Z jammy-updates universe
deb https://snapshot.ubuntu.com/ubuntu/20250815T000000Z jammy multiverse
deb https://snapshot.ubuntu.com/ubuntu/20250815T000000Z jammy-updates multiverse
deb https://snapshot.ubuntu.com/ubuntu/20250815T000000Z jammy-backports main restricted universe multiverse
deb https://snapshot.ubuntu.com/ubuntu/20250815T000000Z jammy-security main restricted
deb https://snapshot.ubuntu.com/ubuntu/20250815T000000Z jammy-security universe
deb https://snapshot.ubuntu.com/ubuntu/20250815T000000Z jammy-security multiverse
EOF
)
        apt_config=$(cat <<EOF
Dir::Etc::sourcelist "${SECURITY_PATCH_CONFIG_DIR}/sources.list";
Dir::Etc::sourceparts "";
EOF
)
        Mock kubectl
            if [[ "$@" == *"live-patching-golden-timestamp"* ]]; then
                echo "20250815T000000Z"
            fi
        End
        When run main 
        The status should be success
        The output should include 'live patching repo service is not set, use ubuntu snapshot repo'
        The output should include 'apt_get_update mock called'
        The output should include 'unattended-upgrade mock called'
        The output should include 'Executed unattended upgrade 1 times'
        The output should include 'snapshot update completed successfully'
        The contents of file "${SECURITY_PATCH_CONFIG_DIR}/sources.list" should eq "${sources_list}"
        The contents of file "${SECURITY_PATCH_CONFIG_DIR}/apt.conf" should eq "${apt_config}"
    End

    It 'should update successfully for ni cluster'
        Mock lsb_release
            echo "noble"     
        End
        sources_list=$(cat <<EOF
deb http://10.0.0.1/ubuntu noble main restricted
deb http://10.0.0.1/ubuntu noble-updates main restricted
deb http://10.0.0.1/ubuntu noble universe
deb http://10.0.0.1/ubuntu noble-updates universe
deb http://10.0.0.1/ubuntu noble multiverse
deb http://10.0.0.1/ubuntu noble-updates multiverse
deb http://10.0.0.1/ubuntu noble-backports main restricted universe multiverse
deb http://10.0.0.1/ubuntu noble-security main restricted
deb http://10.0.0.1/ubuntu noble-security universe
deb http://10.0.0.1/ubuntu noble-security multiverse
EOF
)
        apt_config=$(cat <<EOF
Dir::Etc::sourcelist "${SECURITY_PATCH_CONFIG_DIR}/sources.list";
Dir::Etc::sourceparts "";
EOF
)
        Mock kubectl
            if [[ "$@" == *"live-patching-golden-timestamp"* ]]; then
                echo "20250825T000000Z"
            elif [[ "$@" == *"live-patching-repo-service"* ]]; then
                echo "10.0.0.1"
            fi
        End
        When run main 
        The status should be success
        The output should include 'live patching repo service is: 10.0.0.1'
        The output should include 'apt_get_update mock called'
        The output should include 'unattended-upgrade mock called'
        The output should include 'Executed unattended upgrade 1 times'
        The output should include 'snapshot update completed successfully'
        The contents of file "${SECURITY_PATCH_CONFIG_DIR}/sources.list" should eq "${sources_list}"
        The contents of file "${SECURITY_PATCH_CONFIG_DIR}/apt.conf" should eq "${apt_config}"
    End

    It 'should do nothing if golden timestamp is not set'
        Mock kubectl
            echo ""
        End
        When run main
        The status should be success
        The output should include 'golden timestamp is not set, skip live patching'
    End

    It 'should do nothing if golden timestamp equals current timestamp'
        Mock kubectl
            echo "20250820T000000Z"
        End
        When run main
        The status should be success
        The output should include 'golden timestamp is: 20250820T000000Z'
        The output should include 'current timestamp is: 20250820T000000Z'
        The output should include 'golden and current timestamp is the same, nothing to patch'
    End
End