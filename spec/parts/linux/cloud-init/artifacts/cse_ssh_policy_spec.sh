#!/bin/bash

Describe 'persistent SSH disablement'
    CSE_CONFIG_GPU_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"
    CSE_CONFIG_LOCALDNS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_localdns.sh"
    CSE_CONFIG_KUBELET_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh"
    CSE_CONFIG_NETWORK_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_network.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"

    setup() {
        TEST_ROOT=$(mktemp -d)
        SSH_POLICY_DIR="$TEST_ROOT/policy"
        SSH_SYSTEMD_UNIT_DIR="$TEST_ROOT/systemd"
        EVENTS_LOGGING_DIR="$TEST_ROOT/events"
        MODEL="$TEST_ROOT/model"
        mkdir -p "$MODEL"
        local unit
        for unit in ssh.service sshd.service ssh.socket sshd.socket; do
            printf not-found > "$MODEL/$unit.LoadState"
            printf disabled > "$MODEL/$unit.UnitFileState"
            printf inactive > "$MODEL/$unit.ActiveState"
        done
    }

    cleanup() {
        find "$TEST_ROOT" -type f -delete
        find "$TEST_ROOT" -depth -type d -empty -delete
    }

    BeforeEach setup
    AfterEach cleanup

    systemctl() {
        local action="$1" unit="$2"
        printf '%s\n' "$*" >> "$TEST_ROOT/commands"
        [ "$action" != daemon-reload ] || return 0
        case "$unit" in ssh|sshd) unit="$unit.service" ;; esac
        case "$action" in
            cat)
                case "$(cat "$MODEL/$unit.LoadState")" in
                    loaded|masked) return 0 ;;
                    *) return 1 ;;
                esac
                ;;
            stop)
                # systemctl stop succeeds without clearing an existing failed state.
                [ "$(cat "$MODEL/$unit.ActiveState")" = failed ] ||
                    printf inactive > "$MODEL/$unit.ActiveState"
                ;;
            start)
                [ "$(cat "$MODEL/$unit.LoadState")" = loaded ] || return 1
                if [ -f "$SSH_POLICY_DIR/disabled" ] &&
                    grep -Fxq "ConditionPathExists=!$SSH_POLICY_DIR/disabled" \
                        "$SSH_SYSTEMD_UNIT_DIR/$unit.d/90-aks-ssh-access.conf"; then
                    return 0
                fi
                printf active > "$MODEL/$unit.ActiveState"
                ;;
            *) echo "Unexpected systemctl command: $*" >&2; return 1 ;;
        esac
    }

    timeout() { shift; "$@"; }
    sleep() { :; }

    Describe 'service and socket starts'
        Parameters
            ssh.service
            sshd.service
            ssh.socket
            sshd.socket
        End
        It "keeps $1 blocked after repeated disable without changing enablement"
            block_start() {
                printf loaded > "$MODEL/$1.LoadState"
                printf enabled > "$MODEL/$1.UnitFileState"
                printf active > "$MODEL/$1.ActiveState"
                disableSSH
                disableSSH
                systemctl start "$1"
            }
            When call block_start "$1"
            The status should be success
            The contents of file "$MODEL/$1.ActiveState" should equal inactive
            The contents of file "$MODEL/$1.UnitFileState" should equal enabled
            The contents of file "$TEST_ROOT/commands" should not include 'disable '
        End
    End

    It 'blocks a socket installed and enabled by a later package migration'
        migrate() {
            printf loaded > "$MODEL/ssh.service.LoadState"
            printf enabled > "$MODEL/ssh.service.UnitFileState"
            disableSSH
            printf loaded > "$MODEL/ssh.socket.LoadState"
            printf enabled > "$MODEL/ssh.socket.UnitFileState"
            printf disabled > "$MODEL/ssh.service.UnitFileState"
            systemctl start ssh.socket
        }
        When call migrate
        The status should be success
        The contents of file "$MODEL/ssh.socket.ActiveState" should equal inactive
        The contents of file "$MODEL/ssh.socket.UnitFileState" should equal enabled
    End

    It 'accepts a successful stop of an already-failed SSH service'
        stop_failed() {
            printf loaded > "$MODEL/sshd.service.LoadState"
            printf failed > "$MODEL/sshd.service.ActiveState"
            disableSSH
        }
        When run stop_failed
        The status should be success
        The file "$SSH_POLICY_DIR/disabled" should be exist
        The contents of file "$MODEL/sshd.service.ActiveState" should equal failed
        The contents of file "$TEST_ROOT/commands" should include 'stop sshd'
    End

    It 'stops an active masked service without removing its mask'
        stop_masked() {
            printf masked > "$MODEL/sshd.service.LoadState"
            printf masked > "$MODEL/sshd.service.UnitFileState"
            printf active > "$MODEL/sshd.service.ActiveState"
            disableSSH
        }
        masked_stop_event() {
            jq -se '[.[].Message | fromjson |
                select(.Unit == "sshd" and .Phase == "Completed")] |
                length == 1 and .[0].Skipped == false and .[0].StopExitCode == 0' \
                "$EVENTS_LOGGING_DIR"/*.json
        }
        When call stop_masked
        The status should be success
        The contents of file "$MODEL/sshd.service.ActiveState" should equal inactive
        The contents of file "$MODEL/sshd.service.UnitFileState" should equal masked
        The contents of file "$TEST_ROOT/commands" should not include 'unmask '
        The result of function masked_stop_event should equal true
    End

    It 'stops listeners but fails rather than overwriting an administrator drop-in'
        conflict() {
            printf loaded > "$MODEL/sshd.service.LoadState"
            printf active > "$MODEL/sshd.service.ActiveState"
            mkdir -p "$SSH_SYSTEMD_UNIT_DIR/ssh.socket.d"
            printf custom > "$SSH_SYSTEMD_UNIT_DIR/ssh.socket.d/90-aks-ssh-access.conf"
            disableSSH
        }
        When run conflict
        The status should equal 172
        The stderr should include 'Refusing to overwrite'
        The contents of file "$MODEL/sshd.service.ActiveState" should equal inactive
        The contents of file "$SSH_SYSTEMD_UNIT_DIR/ssh.socket.d/90-aks-ssh-access.conf" should equal custom
    End
End
