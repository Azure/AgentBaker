#!/bin/bash

setup_ssh_policy() {
    SSH_TEST_ROOT=$(mktemp -d)
    SSH_POLICY_DIR="$SSH_TEST_ROOT/policy"
    SSH_SYSTEMD_UNIT_DIR="$SSH_TEST_ROOT/systemd"
    SSH_MODEL_DIR="$SSH_TEST_ROOT/model"
    EVENTS_LOGGING_DIR="$SSH_TEST_ROOT/events"
    mkdir -p "$SSH_MODEL_DIR" "$EVENTS_LOGGING_DIR"
    SSH_MODEL_OS=azurelinux
    FAIL_COMMAND=""
    DISABLE_SSH=true
    DISABLE_PUBKEY_AUTH=false
    SHOULD_RESTORE_SSH=false
    EXPECTED_PUBKEY=""
    SSHD_CONFIG_FILE="$SSH_TEST_ROOT/sshd_config"
    printf 'PubkeyAuthentication yes\nMatch User entra\n  PubkeyAuthentication yes\n' > "$SSHD_CONFIG_FILE"
    local unit
    for unit in ssh.socket sshd.socket ssh.service sshd.service ssh@internal.service sshd@internal.service; do
        printf not-found > "$SSH_MODEL_DIR/$unit.LoadState"
        printf disabled > "$SSH_MODEL_DIR/$unit.UnitFileState"
        printf inactive > "$SSH_MODEL_DIR/$unit.ActiveState"
        printf '%s' "$unit" > "$SSH_MODEL_DIR/$unit.Id"
    done
    printf loaded > "$SSH_MODEL_DIR/sshd.service.LoadState"
    printf enabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
    printf active > "$SSH_MODEL_DIR/sshd.service.ActiveState"
    for unit in ssh.socket sshd.socket; do
        printf no > "$SSH_MODEL_DIR/$unit.Accept"
        printf '%s.service' "${unit%.socket}" > "$SSH_MODEL_DIR/$unit.Triggers"
    done
}

cleanup_ssh_policy() { rm -rf -- "$SSH_TEST_ROOT"; }

ssh_model_unit() {
    local unit="$1"
    case "$unit" in ssh|sshd) unit="$unit.service" ;; esac
    if [ "$SSH_MODEL_OS" = ubuntu ] && [ "$unit" = sshd.service ]; then
        unit=ssh.service
    fi
    printf '%s' "$unit"
}

systemctl() {
    local action="$1" unit property
    printf '%s\n' "$*" >> "$SSH_TEST_ROOT/commands"
    [ "$*" != "$FAIL_COMMAND" ] || return 1
    case "$action" in
        enable|start) printf '%s\n' "$*" >> "$SSH_TEST_ROOT/activation-commands" ;;
    esac
    [ "$action" != daemon-reload ] || return 0
    if [ "$action" = is-active ]; then
        unit=$(ssh_model_unit "$3")
        [ "$(cat "$SSH_MODEL_DIR/$unit.ActiveState")" = active ]
        return
    fi
    unit=$(ssh_model_unit "$2")
    case "$action" in
        show)
            property="${3#--property=}"
            cat "$SSH_MODEL_DIR/$unit.$property"
            ;;
        cat) [ "$(cat "$SSH_MODEL_DIR/$unit.LoadState")" = loaded ] ;;
        stop) printf inactive > "$SSH_MODEL_DIR/$unit.ActiveState" ;;
        disable) printf disabled > "$SSH_MODEL_DIR/$unit.UnitFileState" ;;
        enable) printf enabled > "$SSH_MODEL_DIR/$unit.UnitFileState" ;;
        start)
            [ "$(cat "$SSH_MODEL_DIR/$unit.LoadState")" = loaded ] || return 1
            if [ -f "$SSH_POLICY_DIR/disabled" ] &&
                grep -qx "ConditionPathExists=!$SSH_POLICY_DIR/disabled" \
                    "$SSH_SYSTEMD_UNIT_DIR/$unit.d/90-aks-ssh-access.conf"; then
                return 0
            fi
            if [ -n "$EXPECTED_PUBKEY" ]; then
                grep -qx "PubkeyAuthentication $EXPECTED_PUBKEY" "$SSHD_CONFIG_FILE" || return 1
            fi
            printf active > "$SSH_MODEL_DIR/$unit.ActiveState"
            ;;
        unmask) printf loaded > "$SSH_MODEL_DIR/$unit.LoadState" ;;
        reload-or-restart) return 0 ;;
        *) return 1 ;;
    esac
}

timeout() { shift; "$@"; }
sleep() { :; }
sshd() {
    if [ "$1" = -T ]; then
        awk 'BEGIN { value="yes" } /^Match/ { exit } /^PubkeyAuthentication / { value=$2 }
            END { print "pubkeyauthentication " value }' "$3"
    fi
}
install() {
    [ "$1" != -d ] || return 0
    while [ "$#" -gt 2 ]; do shift; done
    command cp "$1" "$2"
}
