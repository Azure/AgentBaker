#!/bin/bash

Describe 'SSH activation policy helpers'
    CSE_CONFIG_GPU_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"
    CSE_CONFIG_LOCALDNS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_localdns.sh"
    CSE_CONFIG_KUBELET_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh"
    CSE_CONFIG_NETWORK_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_network.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
    Include "./spec/support/ssh_policy.sh"
    BeforeEach setup_ssh_policy
    AfterEach cleanup_ssh_policy

    save_stopped_policy() {
        installSSHDisablePolicy || return 1
        local unit
        for unit in ssh.socket sshd.socket ssh.service sshd.service; do
            systemctl stop "$unit" || return 1
            systemctl disable "$unit" || return 1
        done
    }

    It 'preserves the first activation snapshot and blocks dependency starts'
        repeat_policy() {
            save_stopped_policy
            installSSHDisablePolicy
            systemctl start sshd.service
            cat "$SSH_POLICY_DIR/activation"
            cat "$SSH_MODEL_DIR/sshd.service.ActiveState"
        }
        When call repeat_policy
        The status should be success
        The output should equal "sshd.service enabled
inactive"
    End

    It 'restores an owned service and leaves later LocalUser reconciliation unchanged'
        restore_twice() {
            save_stopped_policy
            restoreSSH
            DISABLE_SSH=false
            SHOULD_RESTORE_SSH=true
            applySSHPolicy
            cat "$SSH_MODEL_DIR/sshd.service.ActiveState"
            cat "$SSH_MODEL_DIR/sshd.service.UnitFileState"
            grep -c '^start ' "$SSH_TEST_ROOT/activation-commands"
        }
        When call restore_twice
        The status should be success
        The output should equal "activeenabled1"
        The file "$SSH_POLICY_DIR/disabled" should not be exist
        The file "$SSH_POLICY_DIR/activation" should not be exist
    End

    It 'captures and restores an unambiguous stopped legacy service without boot links'
        restore_legacy_snapshot() {
            printf inactive > "$SSH_MODEL_DIR/sshd.service.ActiveState"
            printf disabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
            save_stopped_policy
            cat "$SSH_POLICY_DIR/activation"
            restoreSSH
        }
        When call restore_legacy_snapshot
        The status should be success
        The output should equal 'sshd.service disabled'
        The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal active
        The contents of file "$SSH_MODEL_DIR/sshd.service.UnitFileState" should equal disabled
        The contents of file "$SSH_TEST_ROOT/activation-commands" should equal 'start sshd.service'
        The file "$SSH_POLICY_DIR/disabled" should not be exist
        The file "$SSH_POLICY_DIR/activation" should not be exist
    End

    Describe 'invalid ownership records'
        Parameters
            missing 'owned activation record is missing'
            invalid 'no valid saved activation mode'
        End

        It "keeps its block when the saved record is $1"
            break_record() {
                save_stopped_policy
                if [ "$1" = missing ]; then
                    rm "$SSH_POLICY_DIR/activation"
                else
                    printf 'unknown enabled\n' > "$SSH_POLICY_DIR/activation"
                fi
                restoreSSH
            }
            When call break_record "$1"
            The status should be failure
            The stderr should include "$2"
            The file "$SSH_POLICY_DIR/disabled" should be exist
            The file "$SSH_TEST_ROOT/activation-commands" should not be exist
        End
    End

    It 'allows explicit legacy activation without consuming an unowned record'
        mkdir -p "$SSH_POLICY_DIR"
        printf 'sshd.service enabled\n' > "$SSH_POLICY_DIR/activation"
        printf inactive > "$SSH_MODEL_DIR/sshd.service.ActiveState"
        When call restoreSSH
        The status should be success
        The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal active
        The contents of file "$SSH_TEST_ROOT/activation-commands" should equal 'start sshd.service'
        The contents of file "$SSH_POLICY_DIR/activation" should equal 'sshd.service enabled'
    End

    Describe 'socket backing services'
        Parameters
            ssh no ssh.service
            sshd no custom-ssh.service
            sshd yes sshd@internal.service
        End

        It "preserves the block when $1.socket uses a masked $3"
            mask_socket_service() {
                local socket="$1.socket" accept="$2" service="$3"
                printf disabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
                printf loaded > "$SSH_MODEL_DIR/$socket.LoadState"
                printf enabled > "$SSH_MODEL_DIR/$socket.UnitFileState"
                save_stopped_policy
                printf '%s' "$accept" > "$SSH_MODEL_DIR/$socket.Accept"
                printf '%s' "$service" > "$SSH_MODEL_DIR/$socket.Triggers"
                printf masked > "$SSH_MODEL_DIR/$service.LoadState"
                restoreSSH
            }
            When call mask_socket_service "$1" "$2" "$3"
            The status should be failure
            The stderr should include "$3 is masked"
            The file "$SSH_POLICY_DIR/disabled" should be exist
            The file "$SSH_POLICY_DIR/activation" should be exist
            The file "$SSH_TEST_ROOT/activation-commands" should not be exist
        End
    End

    It 'restores an accepting socket without starting a template instance'
        restore_accepting_socket() {
            printf disabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
            printf loaded > "$SSH_MODEL_DIR/sshd.socket.LoadState"
            printf enabled > "$SSH_MODEL_DIR/sshd.socket.UnitFileState"
            printf yes > "$SSH_MODEL_DIR/sshd.socket.Accept"
            printf loaded > "$SSH_MODEL_DIR/sshd@internal.service.LoadState"
            save_stopped_policy
            restoreSSH
            cat "$SSH_TEST_ROOT/activation-commands"
        }
        When call restore_accepting_socket
        The status should be success
        The output should equal "enable sshd.socket
start sshd.socket"
        The contents of file "$SSH_MODEL_DIR/sshd@internal.service.ActiveState" should equal inactive
    End

    It 'does not guess a backing service when a non-accepting socket reports none'
        restore_unknown_socket() {
            printf disabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
            printf loaded > "$SSH_MODEL_DIR/sshd.socket.LoadState"
            printf enabled > "$SSH_MODEL_DIR/sshd.socket.UnitFileState"
            save_stopped_policy
            : > "$SSH_MODEL_DIR/sshd.socket.Triggers"
            restoreSSH
        }
        When call restore_unknown_socket
        The status should be failure
        The stderr should include "no backing service reported"
        The file "$SSH_POLICY_DIR/disabled" should be exist
        The file "$SSH_TEST_ROOT/activation-commands" should not be exist
    End

    It 'reinstates its block when the saved activation cannot start'
        fail_activation() {
            save_stopped_policy
            FAIL_COMMAND='start sshd.service'
            restoreSSH
        }
        When call fail_activation
        The status should be failure
        The stderr should include "Failed to restore SSH activation"
        The file "$SSH_POLICY_DIR/disabled" should be exist
        The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal inactive
    End
End
