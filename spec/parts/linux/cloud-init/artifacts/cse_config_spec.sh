#!/bin/bash

check_test_fstab_permissions() {
    printf "0%s" "$(getFileMode "$TEST_FSTAB_FILE")"
}

Describe 'cse_config.sh'
    CSE_CONFIG_GPU_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"
    CSE_CONFIG_LOCALDNS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_localdns.sh"
    CSE_CONFIG_KUBELET_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh"
    CSE_CONFIG_NETWORK_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_network.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Describe 'enableAndStartCgroupTelemetry'
        setup_cgroup_telemetry() {
            OS="UBUNTU"
            OS_VARIANT=""
            MOCK_CGROUP_VERSION="cgroup2fs"
            FAILED_TELEMETRY_COMMAND=""
            MISSING_TELEMETRY_UNITS=""
            memory_commands="systemctlEnableAndStartNoBlock cgroup-memory-telemetry.timer 30
systemctlEnableAndStartNoBlock cgroup-memory-telemetry.service 30"
            cpu_pressure_commands="systemctlEnableAndStartNoBlock cgroup-cpu-telemetry.timer 30
systemctlEnableAndStartNoBlock cgroup-cpu-telemetry.service 30
systemctlEnableAndStartNoBlock cgroup-pressure-telemetry.timer 30
systemctlEnableAndStartNoBlock cgroup-pressure-telemetry.service 30"
        }

        BeforeEach 'setup_cgroup_telemetry'

        stat() {
            if [ "$*" != "-fc %T /sys/fs/cgroup" ]; then
                echo "Unexpected stat arguments: $*" >&2
                return 1
            fi
            echo "$MOCK_CGROUP_VERSION"
        }

        systemctlEnableAndStartNoBlock() {
            echo "systemctlEnableAndStartNoBlock $*"
            [ "systemctlEnableAndStartNoBlock $*" != "$FAILED_TELEMETRY_COMMAND" ]
        }

        systemctl() {
            [ "$#" -eq 2 ] && [ "$1" = "cat" ] || return 1
            case " $MISSING_TELEMETRY_UNITS " in
                *" $2 "*) return 1 ;;
            esac
            return 0
        }

        Describe 'missing units'
            Parameters
                cgroup-memory-telemetry.timer
                cgroup-memory-telemetry.service
                cgroup-cpu-telemetry.timer
                cgroup-cpu-telemetry.service
                cgroup-pressure-telemetry.timer
                cgroup-pressure-telemetry.service
            End

            It "skips absent $1 and starts the remaining eligible units"
                MISSING_TELEMETRY_UNITS="$1"
                expected_commands="$memory_commands
$cpu_pressure_commands"
                expected_commands="${expected_commands/systemctlEnableAndStartNoBlock $1 30/$1 not found on this VHD, skipping}"

                When run enableAndStartCgroupTelemetry

                The status should be success
                The output should equal "$expected_commands"
            End
        End

        It 'succeeds without starting telemetry when all units are absent'
            MISSING_TELEMETRY_UNITS="cgroup-memory-telemetry.timer cgroup-memory-telemetry.service cgroup-cpu-telemetry.timer cgroup-cpu-telemetry.service cgroup-pressure-telemetry.timer cgroup-pressure-telemetry.service"

            When run enableAndStartCgroupTelemetry

            The status should be success
            The output should equal "cgroup-memory-telemetry.timer not found on this VHD, skipping
cgroup-memory-telemetry.service not found on this VHD, skipping
cgroup-cpu-telemetry.timer not found on this VHD, skipping
cgroup-cpu-telemetry.service not found on this VHD, skipping
cgroup-pressure-telemetry.timer not found on this VHD, skipping
cgroup-pressure-telemetry.service not found on this VHD, skipping"
        End

        Describe 'OS and cgroup eligibility'
            Parameters
                UBUNTU "" true
                MARINER "" false
                MARINERKATA "" false
                AZURELINUX "" false
                AZURELINUXKATA "" false
                AZURELINUX OSGUARD false
                AZURECONTAINERLINUX "" true
                AZURELINUX AZURECONTAINERLINUX true
            End

            It "configures eligible telemetry on cgroup v2 for $1 variant '$2'"
                OS="$1"
                OS_VARIANT="$2"
                expected_commands="$cpu_pressure_commands"
                if [ "$3" = "true" ]; then
                    expected_commands="$memory_commands
$cpu_pressure_commands"
                fi

                When run enableAndStartCgroupTelemetry

                The status should be success
                The output should equal "$expected_commands"
            End

            It "configures only eligible memory telemetry on cgroup v1 for $1 variant '$2'"
                OS="$1"
                OS_VARIANT="$2"
                MOCK_CGROUP_VERSION="tmpfs"
                expected_commands=""
                if [ "$3" = "true" ]; then
                    expected_commands="$memory_commands"
                fi

                When run enableAndStartCgroupTelemetry

                The status should be success
                The output should equal "$expected_commands"
            End
        End

        Describe 'startup failures'
            Parameters
                "systemctlEnableAndStartNoBlock cgroup-memory-telemetry.timer 30" 1
                "systemctlEnableAndStartNoBlock cgroup-memory-telemetry.service 30" 2
                "systemctlEnableAndStartNoBlock cgroup-cpu-telemetry.timer 30" 3
                "systemctlEnableAndStartNoBlock cgroup-cpu-telemetry.service 30" 4
                "systemctlEnableAndStartNoBlock cgroup-pressure-telemetry.timer 30" 5
                "systemctlEnableAndStartNoBlock cgroup-pressure-telemetry.service 30" 6
            End

            It "exits with the telemetry error and stops setup when '$1' fails"
                FAILED_TELEMETRY_COMMAND="$1"

                When run enableAndStartCgroupTelemetry

                The status should equal "$ERR_START_CGROUP_TELEMETRY"
                The lines of output should equal "$2"
                The line "$2" of output should equal "$1"
            End
        End

        It 'exits with the telemetry error when memory service startup fails on cgroup v1'
            MOCK_CGROUP_VERSION="tmpfs"
            FAILED_TELEMETRY_COMMAND="systemctlEnableAndStartNoBlock cgroup-memory-telemetry.service 30"

            When run enableAndStartCgroupTelemetry

            The status should equal "$ERR_START_CGROUP_TELEMETRY"
            The output should equal "$memory_commands"
        End
    End
    Describe 'configureTransparentHugePageSystemdService'
        setup_thp_service() {
            THP_ENABLED="never"
            THP_DEFRAG="madvise"
            unset FAIL_MKDIR FAIL_TEE_PATH FAIL_CHMOD FAIL_DAEMON_RELOAD FAIL_SYSTEMCTL_ENABLE
        }

        cleanup_thp_service() {
            unset THP_ENABLED THP_DEFRAG FAIL_MKDIR FAIL_TEE_PATH CAPTURE_TEE_PATH FAIL_CHMOD FAIL_DAEMON_RELOAD FAIL_SYSTEMCTL_ENABLE
        }

        mkdir() {
            echo "mkdir $*"
            [ "${FAIL_MKDIR:-}" = "true" ] && return 1
            return 0
        }

        tee() {
            if [ "${1:-}" = "${FAIL_TEE_PATH:-__none__}" ]; then
                return 1
            fi
            if [ "${1:-}" = "${CAPTURE_TEE_PATH:-__none__}" ]; then
                cat >&2
                return 0
            fi
            cat > /dev/null
        }

        chmod() {
            echo "chmod $*"
            [ "${FAIL_CHMOD:-}" = "true" ] && return 1
            return 0
        }

        systemctl() {
            echo "systemctl $*"
            [ "${1:-}" = "daemon-reload" ] && [ "${FAIL_DAEMON_RELOAD:-}" = "true" ] && return 1
            return 0
        }

        systemctlEnableAndStart() {
            echo "systemctlEnableAndStart $*"
            [ "${FAIL_SYSTEMCTL_ENABLE:-}" = "true" ] && return 1
            return 0
        }

        BeforeEach 'setup_thp_service'
        AfterEach 'cleanup_thp_service'

        It 'writes helper files, reloads systemd, and enables the service'
            When run configureTransparentHugePageSystemdService

            The status should be success
            The output should include "mkdir -p /opt/azure/containers /opt/azure/containers/aks-transparent-hugepage"
            The output should include "chmod 0755 /opt/azure/containers/aks-transparent-hugepage.sh"
            The output should include "systemctl daemon-reload"
            The output should include "systemctlEnableAndStart aks-transparent-hugepage 30"
        End

        It 'exits when helper directory creation fails'
            FAIL_MKDIR="true"

            When run configureTransparentHugePageSystemdService

            The status should equal "$ERR_SYSCTL_RELOAD"
            The output should include "mkdir -p /opt/azure/containers"
            The output should not include "chmod"
            The output should not include "systemctl daemon-reload"
        End

        It 'exits when helper script write fails'
            FAIL_TEE_PATH="/opt/azure/containers/aks-transparent-hugepage.sh"

            When run configureTransparentHugePageSystemdService

            The status should equal "$ERR_SYSCTL_RELOAD"
            The output should include "mkdir -p /opt/azure/containers"
            The output should not include "chmod"
            The output should not include "systemctl daemon-reload"
        End

        It 'exits when helper script chmod fails'
            FAIL_CHMOD="true"

            When run configureTransparentHugePageSystemdService

            The status should equal "$ERR_SYSCTL_RELOAD"
            The output should include "chmod 0755 /opt/azure/containers/aks-transparent-hugepage.sh"
            The output should not include "systemctl daemon-reload"
        End

        It 'exits when service unit write fails'
            FAIL_TEE_PATH="/etc/systemd/system/aks-transparent-hugepage.service"

            When run configureTransparentHugePageSystemdService

            The status should equal "$ERR_SYSCTL_RELOAD"
            The output should include "chmod 0755 /opt/azure/containers/aks-transparent-hugepage.sh"
            The output should not include "systemctl daemon-reload"
        End

        It 'exits when systemd daemon reload fails'
            FAIL_DAEMON_RELOAD="true"

            When run configureTransparentHugePageSystemdService

            The status should equal "$ERR_SYSTEMCTL_START_FAIL"
            The output should include "systemctl daemon-reload"
            The output should not include "systemctlEnableAndStart"
        End

        It 'does not embed raw THP values in the generated helper script'
            THP_ENABLED='never"; touch /tmp/aks-thp-injection #'
            CAPTURE_TEE_PATH="/opt/azure/containers/aks-transparent-hugepage.sh"

            When run configureTransparentHugePageSystemdService

            The status should be success
            The output should include "systemctlEnableAndStart aks-transparent-hugepage 30"
            The error should include 'cat "${thp_enabled_config}" > /sys/kernel/mm/transparent_hugepage/enabled'
            The error should not include "touch /tmp/aks-thp-injection"
        End
    End
    Describe 'swapFileIsActive'
        swapon() {
            if [ "$*" != "--show --noheadings" ]; then
                return 1
            fi
            printf '%b' "${SWAPON_OUTPUT}"
        }

        It 'matches an active swap file when swapon output has leading whitespace'
            SWAPON_OUTPUT='    /swapfile\n'

            When call swapFileIsActive "/swapfile"
            The status should be success
        End

        It 'matches only the exact swap file path'
            SWAPON_OUTPUT='    /swapfile-extra\n'

            When call swapFileIsActive "/swapfile"
            The status should be failure
        End
    End
    Describe 'ensureSwapFileFstabEntry'
        setup() {
            TEST_FSTAB_DIR="$(mktemp -d)"
            TEST_FSTAB_FILE="${TEST_FSTAB_DIR}/fstab"
            : > "${TEST_FSTAB_FILE}"
        }

        cleanup() {
            rm -rf "${TEST_FSTAB_DIR}"
            unset TEST_FSTAB_FILE
            unset TEST_FSTAB_DIR
            unset FAIL_MV
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'

        mv() {
            if [ "${FAIL_MV:-false}" = "true" ]; then
                return 1
            fi

            command mv "$@"
        }

        It 'replaces existing fstab entries for the same swap file'
            chmod 0644 "${TEST_FSTAB_FILE}"
            printf '/swapfile none swap sw 0 0\n/other none swap sw 0 0\n/swapfile none swap defaults 0 0\n' > "${TEST_FSTAB_FILE}"
            expected_fstab='/other none swap sw 0 0
/swapfile none swap noauto,nofail 0 0'

            When call ensureSwapFileFstabEntry "/swapfile" "${TEST_FSTAB_FILE}"

            The status should be success
            The contents of file "${TEST_FSTAB_FILE}" should equal "${expected_fstab}"
        End

        It 'preserves the fstab file mode when replacing entries'
            chmod 0640 "${TEST_FSTAB_FILE}"
            printf '/other none swap sw 0 0\n' > "${TEST_FSTAB_FILE}"

            When call ensureSwapFileFstabEntry "/swapfile" "${TEST_FSTAB_FILE}"

            The status should be success
            The path "${TEST_FSTAB_FILE}" should be file
            The result of function check_test_fstab_permissions should equal "0640"
        End

        It 'keeps one canonical fstab entry when it already exists'
            printf '/other none swap sw 0 0\n/swapfile none swap noauto,nofail 0 0\n' > "${TEST_FSTAB_FILE}"
            expected_fstab='/other none swap sw 0 0
/swapfile none swap noauto,nofail 0 0'

            When call ensureSwapFileFstabEntry "/swapfile" "${TEST_FSTAB_FILE}"

            The status should be success
            The contents of file "${TEST_FSTAB_FILE}" should equal "${expected_fstab}"
        End

        It 'leaves the existing fstab untouched when atomic replace fails'
            printf '/other none swap sw 0 0\n' > "${TEST_FSTAB_FILE}"
            FAIL_MV=true

            When call ensureSwapFileFstabEntry "/swapfile" "${TEST_FSTAB_FILE}"

            The status should be failure
            The contents of file "${TEST_FSTAB_FILE}" should equal '/other none swap sw 0 0'
        End
    End
    Describe 'disableSSH'
        setup_ssh_telemetry() {
            TEST_EVENTS_ROOT=$(mktemp -d)
            TEST_EVENTS_DIR="$TEST_EVENTS_ROOT/events"
            mkdir "$TEST_EVENTS_DIR"
            EVENTS_LOGGING_DIR=$TEST_EVENTS_DIR
        }

        cleanup_ssh_telemetry() {
            rm -rf "$TEST_EVENTS_ROOT"
        }

        ssh_events_match() {
            jq -se "$SSH_EVENT_FILTER" "$TEST_EVENTS_DIR"/*.json
        }

        BeforeEach setup_ssh_telemetry
        AfterEach cleanup_ssh_telemetry

        installSSHDisablePolicy() { :; }

        systemctl() {
            case "$1" in
                daemon-reload) return 0 ;;
                show)
                    if [ "$3" = --property=ActiveState ]; then
                        echo inactive
                    else
                        case " ${MISSING_UNITS:-} " in
                            *" $2 "*) echo not-found ;;
                            *) echo loaded ;;
                        esac
                    fi
                    ;;
                cat)
                    # MISSING_UNITS simulates a distro where the unit isn't installed.
                    case " ${MISSING_UNITS:-} " in
                        *" $2 "*) return 1 ;;
                    esac
                    return 0
                    ;;
                stop|disable)
                    echo "$*"
                    [ "$*" != "${FAIL_COMMAND:-}" ]
                    ;;
                *) return 1 ;;
            esac
        }

        timeout() {
            shift
            "$@"
        }

        sleep() {
            :
        }

        It 'stops and disables every SSH unit that is present'
            When call disableSSH
            The output should equal "stop ssh.socket
disable ssh.socket
stop sshd.socket
disable sshd.socket
stop ssh
disable ssh
stop sshd
disable sshd"
            The status should be success
            SSH_EVENT_FILTER='
                length == 10 and
                ([.[].Message | fromjson | .AttemptId] | unique | length == 1) and
                all(.[]; (.Message | type) == "string" and .EventLevel == "Informational") and
                ([.[] | select(.TaskName == "AKS.CSE.disableSSH") | .Message | fromjson |
                  select(.Phase == "Completed")] |
                                        length == 1 and .[0].ExitCode == 0) and
                ([.[] | select(.TaskName == "AKS.CSE.disableSSH.unit") | .Message | fromjson |
                  select(.Phase == "Completed")] |
                    length == 4 and all(.[];
                        .CatExitCode == 0 and .StopExitCode == 0 and .DisableExitCode == 0 and
                        .Skipped == false and .SchemaVersion == 1)) and
                    all(.[]; (.Timestamp | length) > 0 and (.OperationId | length) > 0) and
                ([.[].Message | fromjson | select(.Phase == "Started")] |
                    length == 5 and all(.[]; .ExitCode == null))
            '
            The result of function ssh_events_match should equal true
        End

        It 'skips units that are not installed'
            MISSING_UNITS="ssh sshd"
            When call disableSSH
            The output should equal "stop ssh.socket
disable ssh.socket
stop sshd.socket
disable sshd.socket"
            The status should be success
            SSH_EVENT_FILTER='
                [.[].Message | fromjson | select(.Phase == "Completed" and .Skipped == true)] |
                length == 2 and all(.[]; .CatExitCode == 1 and .StopExitCode == null and
                    .DisableExitCode == null and .ExitCode == 0)
            '
            The result of function ssh_events_match should equal true
        End

        It 'skips sshd.socket on distros without socket activation'
            MISSING_UNITS="ssh ssh.socket sshd.socket"
            When call disableSSH
            The output should equal "stop sshd
disable sshd"
            The status should be success
        End

        It 'exits with the CSE SSH error code when a unit cannot be disabled'
            FAIL_COMMAND="disable sshd.socket"
            When run disableSSH
            The output should include "sshd.socket could not be disabled"
            The status should equal 172
            SSH_EVENT_FILTER='
                [.[] | select(.EventLevel == "Error") | .Message | fromjson] |
                length == 2 and
                any(.[]; .Unit == "sshd.socket" and .StopExitCode == 0 and .DisableExitCode == 1) and
                any(.[]; .Unit == "" and .ExitCode == 172 and .Phase == "Completed")
            '
            The result of function ssh_events_match should equal true
        End

        It 'exits with the CSE SSH error code when a unit cannot be stopped'
            FAIL_COMMAND="stop sshd"
            When run disableSSH
            The output should include "sshd could not be stopped"
            The output should include "disable sshd"
            The output should include "disable sshd.socket"
            The status should equal 172
            SSH_EVENT_FILTER='
                [.[].Message | fromjson] |
                any(.[]; .Unit == "sshd" and .StopExitCode == 1 and .DisableExitCode == 0) and
                any(.[]; .Unit == "" and .ExitCode == 172 and .Phase == "Completed") and
                any(.[]; .Unit == "sshd.socket" and .StopExitCode == 0 and .DisableExitCode == 0)
            '
            The result of function ssh_events_match should equal true
        End

        It 'emits only the compact outcome schema'
            When call disableSSH
            The status should be success
            The output should include "disable sshd.socket"
            SSH_EVENT_FILTER='
                [.[].Message | fromjson | select(.Phase == "Completed" and .Unit != "") | keys] |
                length == 4 and all(.[]; . ==
                    ["AttemptId", "CatExitCode", "DisableExitCode", "ExitCode", "Phase",
                     "SchemaVersion", "Skipped", "StopExitCode", "Unit"])
            '
            The result of function ssh_events_match should equal true
        End

        It 'keeps a distinct denominator when called repeatedly'
            disable_ssh_twice() {
                disableSSH
                disableSSH
            }
            When call disable_ssh_twice
            The status should be success
            The output should include "disable sshd.socket"
            SSH_EVENT_FILTER='
                length == 20 and ([.[].Message | fromjson | .AttemptId] | unique | length == 2)
            '
            The result of function ssh_events_match should equal true
        End

        It 'publishes complete events despite guest-agent queue cleanup'
            mv() {
                if [ ! -f "$TEST_EVENTS_ROOT/cleaned" ]; then
                    find "$TEST_EVENTS_DIR" -maxdepth 1 -type f -delete
                    touch "$TEST_EVENTS_ROOT/cleaned"
                fi
                command mv "$@"
            }

            remaining_staging_files() {
                find "$TEST_EVENTS_ROOT" -name '.ssh-disable.*' -print
            }

            When call disableSSH
            The status should be success
            The output should include "disable sshd.socket"
            The stderr should equal ""
            The file "$TEST_EVENTS_ROOT/cleaned" should be exist
            SSH_EVENT_FILTER='length == 10 and all(.[]; (.Message | fromjson | .SchemaVersion) == 1)'
            The result of function ssh_events_match should equal true
            The result of function remaining_staging_files should equal ""
        End

        It 'leaves started events when an attempt is interrupted'
            systemctl_stop() { exit 124; }
            When run disableSSH
            The status should equal 124
            SSH_EVENT_FILTER='
                length == 2 and all(.[]; (.Message | fromjson | .Phase) == "Started") and
                any(.[]; .TaskName == "AKS.CSE.disableSSH") and
                any(.[]; .TaskName == "AKS.CSE.disableSSH.unit")
            '
            The result of function ssh_events_match should equal true
        End

        It 'warns without failing provisioning when telemetry cannot be written'
            touch "$TEST_EVENTS_DIR/not-a-directory"
            EVENTS_LOGGING_DIR="$TEST_EVENTS_DIR/not-a-directory"
            When call disableSSH
            The status should be success
            The output should include "disable sshd.socket"
            The stderr should include "WARNING: could not emit SSH disable"
        End

        It 'preserves success when telemetry serialization fails'
            jq() { return 1; }
            When call disableSSH
            The status should be success
            The output should include "disable sshd.socket"
            The stderr should include "WARNING: could not emit SSH disable"
            The stderr should include "WARNING: could not serialize SSH disable result"
        End

        It 'preserves exit 172 when telemetry serialization also fails'
            jq() { return 1; }
            FAIL_COMMAND="stop sshd"
            When run disableSSH
            The status should equal 172
            The output should include "sshd could not be stopped"
            The output should include "disable sshd"
            The stderr should include "WARNING: could not emit SSH disable"
            The stderr should include "WARNING: could not serialize SSH disable result"
        End
    End
    Describe 'disableSSHPubkeyAuth'
        setup() {
            SSHD_CONFIG_FILE="$(mktemp)"
            SSH_POLICY_DIR="$(mktemp -d)"
            SSH_SERVICE_ACTIVE="true"
            SSH_SERVICE_EXISTS="true"
            SSHD_OUTPUT_KEY=pubkeyauthentication
            DISABLE_SSH=false
            DISABLE_PUBKEY_AUTH=false
            SHOULD_RESTORE_SSH=false
        }

        cleanup() {
            rm -f "${SSHD_CONFIG_FILE}"
            rm -rf "${SSH_POLICY_DIR}"
            unset SSHD_CONFIG_FILE
            unset SSH_SERVICE_ACTIVE
            unset SSH_SERVICE_EXISTS
            unset SSHD_OUTPUT_KEY
        }

        systemctl() {
            echo "systemctl $*" >&2
            if [ "$1" = "cat" ] && [ "${SSH_SERVICE_EXISTS}" != "true" ]; then
                return 1
            fi
            if [ "$1" = "is-active" ]; then
                case "$3" in
                    *.socket) return 3 ;;
                esac
                [ "${SSH_SERVICE_ACTIVE}" = "true" ] || return 3
            fi
            return 0
        }

        sshd() {
            echo "sshd $*" >&2
            if [ "$1" = -T ]; then
                awk -v key="$SSHD_OUTPUT_KEY" 'BEGIN { value="yes" } /^Match/ { exit } /^PubkeyAuthentication / { value=$2 }
                    END { print key " " value }' "$3"
            fi
            return 0
        }

        install() {
            [ "$1" != -d ] || return 0
            while [ "$#" -gt 2 ]; do
                shift
            done
            command cp "$1" "$2"
        }

        run_disable_ssh_pubkey_auth() {
            disableSSHPubkeyAuth
            cat "${SSHD_CONFIG_FILE}"
        }

        restoreSSH() { return 0; }

        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'disables the global setting without changing a Match block'
            cat > "${SSHD_CONFIG_FILE}" <<'EOF'
PasswordAuthentication no
PubkeyAuthentication yes
Match User entra
  AuthenticationMethods publickey
  PubkeyAuthentication yes
EOF
            expected='PasswordAuthentication no
PubkeyAuthentication no
Match User entra
  AuthenticationMethods publickey
  PubkeyAuthentication yes'

            When call run_disable_ssh_pubkey_auth

            The status should be success
            The output should equal "${expected}"
            The stderr should include "sshd -t -f"
        End

        It 'inserts the global setting before the first Match block'
            cat > "${SSHD_CONFIG_FILE}" <<'EOF'
PasswordAuthentication no
Match User entra
  PubkeyAuthentication yes
EOF
            expected='PasswordAuthentication no
PubkeyAuthentication no
Match User entra
  PubkeyAuthentication yes'

            When call run_disable_ssh_pubkey_auth

            The status should be success
            The output should equal "${expected}"
            The stderr should include "sshd -t -f"
        End

        It 'appends the setting when the config has no Match block'
            echo "PasswordAuthentication no" > "${SSHD_CONFIG_FILE}"
            expected='PasswordAuthentication no
PubkeyAuthentication no'

            When call run_disable_ssh_pubkey_auth

            The status should be success
            The output should equal "${expected}"
            The stderr should include "sshd -t -f"
        End

        It 'starts ssh.service when the Ubuntu service is not active'
            echo "PasswordAuthentication no" > "${SSHD_CONFIG_FILE}"
            SSH_SERVICE_ACTIVE="false"

            When call run_disable_ssh_pubkey_auth

            The status should be success
            The output should equal "PasswordAuthentication no
PubkeyAuthentication no"
            The stderr should include "systemctl start ssh.service"
        End

        It 'starts sshd.service when ssh.service does not exist'
            echo "PasswordAuthentication no" > "${SSHD_CONFIG_FILE}"
            SSH_SERVICE_ACTIVE="false"
            SSH_SERVICE_EXISTS="false"

            When call run_disable_ssh_pubkey_auth

            The status should be success
            The output should equal "PasswordAuthentication no
PubkeyAuthentication no"
            The stderr should include "systemctl start sshd.service"
        End

        It 'does not start SSH when candidate validation fails'
            echo "PubkeyAuthentication yes" > "${SSHD_CONFIG_FILE}"
            SSH_SERVICE_ACTIVE=false
            sshd() {
                if [ "$1" = -T ]; then
                    echo "pubkeyauthentication yes"
                    return 0
                fi
                return 1
            }
            When run disableSSHPubkeyAuth
            The status should equal 178
            The contents of file "${SSHD_CONFIG_FILE}" should equal "PubkeyAuthentication yes"
            The stderr should not include "systemctl start"
        End

        It 'installs the restrictive config before starting SSH'
            echo "PubkeyAuthentication yes" > "${SSHD_CONFIG_FILE}"
            SSH_SERVICE_ACTIVE=false
            systemctl() {
                case "$1" in
                    cat) return 0 ;;
                    is-active) return 3 ;;
                    start) grep -qx 'PubkeyAuthentication no' "${SSHD_CONFIG_FILE}" ;;
                    reload-or-restart) return 0 ;;
                    *) return 1 ;;
                esac
            }
            When call disableSSHPubkeyAuth
            The status should be success
            The stderr should include "sshd -t -f"
        End

        Describe 'authentication restoration across sshd output formats'
            Parameters
                pubkeyauthentication
                PubkeyAuthentication
                PUBKEYAUTHENTICATION
            End

            It "restores only the owned global setting with $1 in sshd output"
                SSHD_OUTPUT_KEY="$1"
                cat > "${SSHD_CONFIG_FILE}" <<'EOF'
PubkeyAuthentication yes
Match User entra
  PubkeyAuthentication yes
EOF
                disable_then_restore_auth() {
                    disableSSHPubkeyAuth
                    SHOULD_RESTORE_SSH=true
                    applySSHPolicy
                    cat "${SSHD_CONFIG_FILE}"
                }
                When call disable_then_restore_auth
                The status should be success
                The output should equal "PubkeyAuthentication yes
Match User entra
  PubkeyAuthentication yes"
                The stderr should include "systemctl reload-or-restart ssh.service"
                The file "$SSH_POLICY_DIR/pubkey-auth" should not be exist
            End
        End

        It 'does not activate SSH when installing the authentication config fails'
            echo "PubkeyAuthentication yes" > "${SSHD_CONFIG_FILE}"
            SSH_SERVICE_ACTIVE=false
            install() { [ "$1" = -d ]; }
            When run disableSSHPubkeyAuth
            The status should equal 178
            The contents of file "${SSHD_CONFIG_FILE}" should equal "PubkeyAuthentication yes"
            The stderr should include "sshd -t -f"
            The stderr should not include "systemctl start"
        End

        It 'does not reopen SSH when an Include overrides the requested authentication policy'
            echo "Include /etc/ssh/sshd_config.d/*.conf" > "$SSHD_CONFIG_FILE"
            SSH_SERVICE_ACTIVE=false
            sshd() {
                [ "$1" != -T ] || echo 'pubkeyauthentication yes'
                return 0
            }
            When run disableSSHPubkeyAuth
            The status should equal 178
            The stderr should include "does not enforce the requested public-key policy"
            The stderr should not include "systemctl start"
            The contents of file "$SSHD_CONFIG_FILE" should equal "Include /etc/ssh/sshd_config.d/*.conf"
        End
    End
    Describe 'persistent SSH access policy'
        Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
        Include "./spec/support/ssh_policy.sh"
        BeforeEach setup_ssh_policy
        AfterEach cleanup_ssh_policy

        run_node_prep_ssh_reconciliation() {
            sed -n '/^function nodePrep {/,/^}/p' \
                parts/linux/cloud-init/artifacts/cse_main.sh |
                sed "s|/opt/azure/containers/base_prep.complete|$SSH_TEST_ROOT/base_prep.complete|g" \
                    > "$SSH_TEST_ROOT/node-prep.sh"
            # Load the actual entry point without executing CSE's top-level setup.
            . "$SSH_TEST_ROOT/node-prep.sh"
            nodePrep
        }

        It 'blocks a dependency start even after a package unmask'
            disable_and_unmask() {
                disableSSH
                systemctl unmask sshd.service
                systemctl start sshd.service
                cat "$SSH_MODEL_DIR/sshd.service.ActiveState"
            }
            When call disable_and_unmask
            The status should be success
            The output should equal inactive
        End

        It 'does not lose the original activation mode on repeated disable'
            disable_twice_and_restore() {
                disableSSH
                disableSSH
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                applySSHPolicy
                cat "$SSH_MODEL_DIR/sshd.service.ActiveState"
                cat "$SSH_MODEL_DIR/sshd.service.UnitFileState"
            }
            When call disable_twice_and_restore
            The status should be success
            The output should equal activeenabled
            The file "$SSH_POLICY_DIR/disabled" should not be exist
        End

        It 'can restore a unique endpoint disabled by legacy CSE without inventing boot links'
            restore_legacy_disabled() {
                printf disabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
                printf inactive > "$SSH_MODEL_DIR/sshd.service.ActiveState"
                disableSSH
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                applySSHPolicy
                cat "$SSH_MODEL_DIR/sshd.service.ActiveState"
                cat "$SSH_MODEL_DIR/sshd.service.UnitFileState"
            }
            When call restore_legacy_disabled
            The status should be success
            The output should equal activedisabled
        End

        It 'explicitly enables the unique legacy service when both endpoints are disabled'
            restore_ambiguous_legacy() {
                printf disabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
                printf inactive > "$SSH_MODEL_DIR/sshd.service.ActiveState"
                printf loaded > "$SSH_MODEL_DIR/sshd.socket.LoadState"
                disableSSH
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                applySSHPolicy
                cat "$SSH_MODEL_DIR/sshd.service.ActiveState"
                cat "$SSH_MODEL_DIR/sshd.service.UnitFileState"
                cat "$SSH_MODEL_DIR/sshd.socket.UnitFileState"
            }
            When call restore_ambiguous_legacy
            The status should be success
            The output should equal activeenableddisabled
            The file "$SSH_POLICY_DIR/disabled" should not be exist
        End

        It 'leaves a stopped service unchanged for LocalUser without an owned policy'
            DISABLE_SSH=false
            SHOULD_RESTORE_SSH=true
            printf disabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
            printf inactive > "$SSH_MODEL_DIR/sshd.service.ActiveState"
            When call applySSHPolicy
            The status should be success
            The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal inactive
            The contents of file "$SSH_MODEL_DIR/sshd.service.UnitFileState" should equal disabled
            The file "$SSH_TEST_ROOT/activation-commands" should not be exist
        End

        Describe 'legacy Entra authentication reconciliation without a disable marker'
            prepare_legacy_entra_auth() {
                DISABLE_SSH=false
                DISABLE_PUBKEY_AUTH=true
                SHOULD_RESTORE_SSH=true
                EXPECTED_PUBKEY=no
                printf disabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
                printf inactive > "$SSH_MODEL_DIR/sshd.service.ActiveState"
            }

            It 'enables boot activation after validating PubkeyAuthentication=no'
                prepare_legacy_entra_auth
                When call applySSHPolicy
                The status should be success
                The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal active
                The contents of file "$SSH_MODEL_DIR/sshd.service.UnitFileState" should equal enabled
                The contents of file "$SSHD_CONFIG_FILE" should include "PubkeyAuthentication no"
                The file "$SSH_POLICY_DIR/disabled" should not be exist
            End

            It 'preserves an external service mask with PubkeyAuthentication=no'
                prepare_legacy_entra_auth
                printf masked > "$SSH_MODEL_DIR/sshd.service.LoadState"
                When run applySSHPolicy
                The status should equal 178
                The stderr should include "external masks are not removed"
                The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal inactive
                The contents of file "$SSH_MODEL_DIR/sshd.service.LoadState" should equal masked
                The contents of file "$SSH_MODEL_DIR/sshd.service.UnitFileState" should equal disabled
            End
        End

        Describe 'LocalUser authentication restoration without a disable marker'
            Parameters
                loaded active
                loaded inactive
                masked inactive
            End

            It "restores authentication without changing a $1/$2 service"
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                mkdir -p "$SSH_POLICY_DIR"
                printf yes > "$SSH_POLICY_DIR/pubkey-auth"
                printf 'PubkeyAuthentication no\nMatch User entra\n  PubkeyAuthentication yes\n' > "$SSHD_CONFIG_FILE"
                printf '%s' "$1" > "$SSH_MODEL_DIR/sshd.service.LoadState"
                printf '%s' "$2" > "$SSH_MODEL_DIR/sshd.service.ActiveState"
                printf disabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
                When call applySSHPolicy
                The status should be success
                The contents of file "$SSHD_CONFIG_FILE" should equal "PubkeyAuthentication yes
Match User entra
  PubkeyAuthentication yes"
                The contents of file "$SSH_MODEL_DIR/sshd.service.LoadState" should equal "$1"
                The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal "$2"
                The contents of file "$SSH_MODEL_DIR/sshd.service.UnitFileState" should equal disabled
                The file "$SSH_TEST_ROOT/activation-commands" should not be exist
                The file "$SSH_POLICY_DIR/pubkey-auth" should not be exist
            End
        End

        It 'keeps the authentication backup when reloading LocalUser authentication fails'
            DISABLE_SSH=false
            SHOULD_RESTORE_SSH=true
            mkdir -p "$SSH_POLICY_DIR"
            printf yes > "$SSH_POLICY_DIR/pubkey-auth"
            printf 'PubkeyAuthentication no\n' > "$SSHD_CONFIG_FILE"
            FAIL_COMMAND="reload-or-restart sshd.service"
            When call applySSHPolicy
            The status should equal 178
            The contents of file "$SSH_POLICY_DIR/pubkey-auth" should equal yes
            The file "$SSH_TEST_ROOT/activation-commands" should not be exist
        End

        It 'restores authentication before activating an owned disabled service for LocalUser'
            restore_owned_localuser_auth() {
                disableSSHPubkeyAuth
                disableSSH
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                EXPECTED_PUBKEY=yes
                applySSHPolicy
            }
            When call restore_owned_localuser_auth
            The status should be success
            The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal active
            The contents of file "$SSH_MODEL_DIR/sshd.service.UnitFileState" should equal enabled
            The contents of file "$SSHD_CONFIG_FILE" should include "PubkeyAuthentication yes"
            The file "$SSH_POLICY_DIR/disabled" should not be exist
            The file "$SSH_POLICY_DIR/pubkey-auth" should not be exist
        End

        It 'deduplicates Ubuntu aliases when selecting the legacy service'
            restore_legacy_ubuntu() {
                SSH_MODEL_OS=ubuntu
                printf loaded > "$SSH_MODEL_DIR/ssh.service.LoadState"
                printf loaded > "$SSH_MODEL_DIR/ssh.socket.LoadState"
                disableSSH
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                applySSHPolicy
                cat "$SSH_MODEL_DIR/ssh.service.ActiveState"
                cat "$SSH_MODEL_DIR/ssh.service.UnitFileState"
                cat "$SSH_MODEL_DIR/ssh.socket.UnitFileState"
            }
            When call restore_legacy_ubuntu
            The status should be success
            The output should equal activeenableddisabled
        End

        It 'refuses a legacy restore with multiple distinct services'
            restore_distinct_legacy_services() {
                printf loaded > "$SSH_MODEL_DIR/ssh.service.LoadState"
                printf disabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
                printf inactive > "$SSH_MODEL_DIR/sshd.service.ActiveState"
                disableSSH
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                applySSHPolicy
            }
            When call restore_distinct_legacy_services
            The status should equal 172
            The stderr should include "multiple distinct services"
            The file "$SSH_POLICY_DIR/disabled" should be exist
        End

        It 'does not bypass an external service mask in the legacy fallback'
            restore_masked_legacy_service() {
                printf disabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
                printf inactive > "$SSH_MODEL_DIR/sshd.service.ActiveState"
                printf loaded > "$SSH_MODEL_DIR/sshd.socket.LoadState"
                disableSSH
                printf masked > "$SSH_MODEL_DIR/sshd.service.LoadState"
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                applySSHPolicy
            }
            When call restore_masked_legacy_service
            The status should equal 172
            The stderr should include "external masks are not removed"
            The contents of file "$SSH_MODEL_DIR/sshd.service.LoadState" should equal masked
            The file "$SSH_POLICY_DIR/disabled" should be exist
        End

        It 'keeps SSH blocked when the enabled policy is unspecified'
            disable_then_unspecified() {
                disableSSH
                DISABLE_SSH=false
                applySSHPolicy
                systemctl start sshd.service
                cat "$SSH_MODEL_DIR/sshd.service.ActiveState"
            }
            When call disable_then_unspecified
            The status should be success
            The output should equal inactive
            The file "$SSH_POLICY_DIR/disabled" should be exist
        End

        It 'stops an already running masked service without removing its mask'
            printf masked > "$SSH_MODEL_DIR/sshd.service.LoadState"
            When call disableSSH
            The status should be success
            The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal inactive
            The contents of file "$SSH_MODEL_DIR/sshd.service.LoadState" should equal masked
        End

        It 'reports an external mask conflict without reopening SSH'
            restore_externally_masked() {
                disableSSH
                printf masked > "$SSH_MODEL_DIR/sshd.service.LoadState"
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                applySSHPolicy
            }
            When call restore_externally_masked
            The status should equal 172
            The stderr should include "external masks are not removed"
            The file "$SSH_POLICY_DIR/disabled" should be exist
            The contents of file "$SSH_MODEL_DIR/sshd.service.LoadState" should equal masked
        End

        It 'preserves an administrator drop-in and fails provisioning'
            mkdir -p "$SSH_SYSTEMD_UNIT_DIR/sshd.service.d"
            echo '[Unit]' > "$SSH_SYSTEMD_UNIT_DIR/sshd.service.d/90-aks-ssh-access.conf"
            When run disableSSH
            The status should equal 172
            The stderr should include "Refusing to overwrite"
            The contents of file "$SSH_SYSTEMD_UNIT_DIR/sshd.service.d/90-aks-ssh-access.conf" should equal '[Unit]'
            The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal inactive
        End

        It 'leaves an external mask unchanged for LocalUser without an owned policy'
            DISABLE_SSH=false
            SHOULD_RESTORE_SSH=true
            printf masked > "$SSH_MODEL_DIR/sshd.service.LoadState"
            When call applySSHPolicy
            The status should be success
            The contents of file "$SSH_MODEL_DIR/sshd.service.LoadState" should equal masked
            The file "$SSH_TEST_ROOT/activation-commands" should not be exist
            The file "$SSH_POLICY_DIR/disabled" should not be exist
        End

        It 'does not release a disable marker when its ownership record is missing'
            restore_with_missing_record() {
                disableSSH
                rm "$SSH_POLICY_DIR/activation"
                printf enabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                applySSHPolicy
            }
            When call restore_with_missing_record
            The status should equal 172
            The stderr should include "owned activation record is missing"
            The file "$SSH_POLICY_DIR/disabled" should be exist
            The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal inactive
        End

        It 'fails provisioning when the persistent policy cannot be loaded'
            FAIL_COMMAND=daemon-reload
            When run disableSSH
            The status should equal 172
            The stderr should include "Failed to install persistent SSH disable policy"
            The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal inactive
        End

        It 'restores socket activation from a disabled PIS cache without enabling its service'
            restore_ubuntu_socket() {
                SSH_MODEL_OS=ubuntu
                printf loaded > "$SSH_MODEL_DIR/ssh.service.LoadState"
                printf loaded > "$SSH_MODEL_DIR/ssh.socket.LoadState"
                printf enabled > "$SSH_MODEL_DIR/ssh.socket.UnitFileState"
                printf active > "$SSH_MODEL_DIR/ssh.socket.ActiveState"
                disableSSH
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                applySSHPolicy
                cat "$SSH_MODEL_DIR/ssh.socket.ActiveState"
                cat "$SSH_MODEL_DIR/ssh.service.UnitFileState"
            }
            When call restore_ubuntu_socket
            The status should be success
            The output should equal activedisabled
        End

        It 'respects a package migration to a newly installed socket'
            restore_migrated_socket() {
                disableSSH
                printf loaded > "$SSH_MODEL_DIR/sshd.socket.LoadState"
                printf enabled > "$SSH_MODEL_DIR/sshd.socket.UnitFileState"
                systemctl start sshd.socket
                [ "$(cat "$SSH_MODEL_DIR/sshd.socket.ActiveState")" = inactive ] || return 1
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                applySSHPolicy
                cat "$SSH_MODEL_DIR/sshd.socket.ActiveState"
                cat "$SSH_MODEL_DIR/sshd.service.UnitFileState"
            }
            When call restore_migrated_socket
            The status should be success
            The output should equal activedisabled
        End

        It 'reinstates the persistent block if activation fails'
            fail_restore() {
                disableSSH
                FAIL_COMMAND="start sshd.service"
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                applySSHPolicy
            }
            When call fail_restore
            The status should equal 172
            The stderr should include "Failed to restore SSH activation"
            The file "$SSH_POLICY_DIR/disabled" should be exist
            The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal inactive
        End

        It 'configures Entra authentication before restoring a disabled service'
            restore_entra_auth() {
                disableSSH
                DISABLE_SSH=false
                DISABLE_PUBKEY_AUTH=true
                EXPECTED_PUBKEY=no
                applySSHPolicy
                cat "$SSH_MODEL_DIR/sshd.service.ActiveState"
            }
            When call restore_entra_auth
            The status should be success
            The output should equal active
            The contents of file "$SSHD_CONFIG_FILE" should include "PubkeyAuthentication no"
            The contents of file "$SSHD_CONFIG_FILE" should include "  PubkeyAuthentication yes"
        End

        Describe 'owned Entra activation failures'
            Parameters
                syntax
                install
            End

            It "keeps the owned block when authentication $1 fails"
                SSH_AUTH_FAILURE="$1"
                sshd() {
                    if [ "$1" = -T ]; then
                        awk 'BEGIN { value="yes" } /^Match/ { exit } /^PubkeyAuthentication / { value=$2 }
                            END { print "pubkeyauthentication " value }' "$3"
                    else
                        [ "$SSH_AUTH_FAILURE" != syntax ]
                    fi
                }
                install() {
                    [ "$1" != -d ] || return 0
                    [ "$SSH_AUTH_FAILURE" != install ] || return 1
                    while [ "$#" -gt 2 ]; do shift; done
                    command cp "$1" "$2"
                }
                fail_owned_entra() {
                    disableSSH
                    DISABLE_SSH=false
                    DISABLE_PUBKEY_AUTH=true
                    applySSHPolicy
                }
                When run fail_owned_entra
                The status should equal 178
                The file "$SSH_POLICY_DIR/disabled" should be exist
                The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal inactive
                The contents of file "$SSHD_CONFIG_FILE" should include "PubkeyAuthentication yes"
            End
        End

        It 'gives SSH Disabled precedence over the authentication request'
            DISABLE_PUBKEY_AUTH=true
            When call applySSHPolicy
            The status should be success
            The file "$SSH_POLICY_DIR/disabled" should be exist
            The file "$SSH_POLICY_DIR/pubkey-auth" should not be exist
            The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal inactive
        End

        It 'enforces Disabled at nodePrep even when basePrep was cached as enabled'
            touch "$SSH_TEST_ROOT/base_prep.complete"
            logs_to_events() {
                [ "$2" = applySSHPolicy ] || exit 1
                "$2"
                exit "$?"
            }
            When run run_node_prep_ssh_reconciliation
            The status should be success
            The file "$SSH_POLICY_DIR/disabled" should be exist
            The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal inactive
        End

        It 'restores explicitly enabled SSH at nodePrep from a disabled cache'
            prepare_disabled_cache() {
                touch "$SSH_TEST_ROOT/base_prep.complete"
                disableSSH
                DISABLE_SSH=false
                SHOULD_RESTORE_SSH=true
                run_node_prep_ssh_reconciliation
            }
            logs_to_events() {
                [ "$2" = applySSHPolicy ] || exit 1
                "$2"
                exit "$?"
            }
            When run prepare_disabled_cache
            The status should be success
            The file "$SSH_POLICY_DIR/disabled" should not be exist
            The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal active
        End

        Describe 'uncached Entra node preparation'
            Parameters
                legacy active
                socket inactive
            End

            It "does not activate a stopped service twice after restoring a $1 snapshot"
                prepare_uncached_entra() {
                    printf disabled > "$SSH_MODEL_DIR/sshd.service.UnitFileState"
                    printf inactive > "$SSH_MODEL_DIR/sshd.service.ActiveState"
                    if [ "$1" = socket ]; then
                        printf loaded > "$SSH_MODEL_DIR/sshd.socket.LoadState"
                        printf enabled > "$SSH_MODEL_DIR/sshd.socket.UnitFileState"
                        printf active > "$SSH_MODEL_DIR/sshd.socket.ActiveState"
                    fi
                    disableSSH
                    DISABLE_SSH=false
                    DISABLE_PUBKEY_AUTH=true
                    applySSHPolicy
                    [ ! -f "$SSH_POLICY_DIR/disabled" ] || return 1
                    [ "$(cat "$SSH_MODEL_DIR/sshd.service.ActiveState")" = "$2" ] || return 1
                    run_node_prep_ssh_reconciliation
                }
                logs_to_events() {
                    case "$2" in
                        applySSHPolicy|disableSSHPubkeyAuth)
                            printf '%s\n' "$2" >> "$SSH_TEST_ROOT/policy-events"
                            "$2"
                            ;;
                        configureAzureJson) exit 0 ;;
                        *) exit 1 ;;
                    esac
                }
                When run prepare_uncached_entra "$1" "$2"
                The status should be success
                The contents of file "$SSH_MODEL_DIR/sshd.service.ActiveState" should equal "$2"
                The contents of file "$SSH_TEST_ROOT/commands" should not include 'enable sshd.service'
                The contents of file "$SSH_TEST_ROOT/policy-events" should equal "disableSSHPubkeyAuth
applySSHPolicy
disableSSHPubkeyAuth"
            End
        End
    End
    Describe 'configureAzureJson'
        AZURE_JSON_PATH="azure.json"
        AKS_CUSTOM_CLOUD_JSON_PATH="customcloud.json"
        CLOUDPROVIDER_BACKOFF_EXPONENT="1"
        CLOUDPROVIDER_BACKOFF_JITTER="0.1"
        TARGET_CLOUD="AzurePublicCloud"
        TENANT_ID="tenant-id"
        SUBSCRIPTION_ID="subscription-id"
        RESOURCE_GROUP="resource-group"
        LOCATION="eastus"

        chmod () {
            echo "chmod $@"
        }
        chown() {
            echo "chown $@"
        }

        cleanup() {
            rm -f "$AZURE_JSON_PATH"
            rm -f "$AKS_CUSTOM_CLOUD_JSON_PATH"
        }

        AfterEach 'cleanup'

        It 'should configure the azure.json file'
            SERVICE_PRINCIPAL_CLIENT_ID="sp-client-id"
            SERVICE_PRINCIPAL_FILE_CONTENT="c3Atc2VjcmV0Cg==" # base64 encoding of "sp-secret"
            CLOUDPROVIDER_BACKOFF_MODE="v1"
            # using "run" instead of "call" since configureAzureJson modifies shell opts with set +/-x which conflicts with shellspec
            When run configureAzureJson
            The output should include "chmod 0600 azure.json"
            The output should include "chown root:root azure.json"
            The contents of file "azure.json" should include '"cloud": "AzurePublicCloud"'
            The contents of file "azure.json" should include '"aadClientId": "sp-client-id"'
            The contents of file "azure.json" should include '"aadClientSecret": "sp-secret"'
            The contents of file "azure.json" should include '"tenantId": "tenant-id"'
            The contents of file "azure.json" should include '"subscriptionId": "subscription-id"'
            The contents of file "azure.json" should include '"resourceGroup": "resource-group"'
            The contents of file "azure.json" should include '"location": "eastus"'
            The contents of file "azure.json" should include '"cloudProviderBackoffExponent": 1'
            The contents of file "azure.json" should include '"cloudProviderBackoffJitter": 0.1'
            The contents of file "azure.json" should include '"cloudProviderBackoffMode": "v1"'
            The stderr should not eq '' # since we're calling "set" with +/-x numerous times
            The status should be success
        End

        It 'should configure the azure.json file without a service principal secret if no service principal file content is supplied'
            SERVICE_PRINCIPAL_CLIENT_ID=""
            SERVICE_PRINCIPAL_FILE_CONTENT=""
            CLOUDPROVIDER_BACKOFF_MODE="v1"
            When run configureAzureJson
            The output should include "chmod 0600 azure.json"
            The output should include "chown root:root azure.json"
            The contents of file "azure.json" should include '"cloud": "AzurePublicCloud"'
            The contents of file "azure.json" should include '"aadClientId": ""'
            The contents of file "azure.json" should include '"aadClientSecret": ""'
            The contents of file "azure.json" should include '"tenantId": "tenant-id"'
            The contents of file "azure.json" should include '"subscriptionId": "subscription-id"'
            The contents of file "azure.json" should include '"resourceGroup": "resource-group"'
            The contents of file "azure.json" should include '"location": "eastus"'
            The contents of file "azure.json" should include '"cloudProviderBackoffExponent": 1'
            The contents of file "azure.json" should include '"cloudProviderBackoffJitter": 0.1'
            The contents of file "azure.json" should include '"cloudProviderBackoffMode": "v1"'
            The contents of file "azure.json" should not include "sp-secret"
            The stderr should not eq '' # since we're calling "set" with +/-x numerous times
            The status should be success
        End

        It 'should reconfigure azure json if cloud provider backoff mode is "v2"'
            SERVICE_PRINCIPAL_CLIENT_ID="sp-client-id"
            SERVICE_PRINCIPAL_FILE_CONTENT="c3Atc2VjcmV0Cg==" # base64 encoding of "sp-secret"
            CLOUDPROVIDER_BACKOFF_MODE="v2"
            When run configureAzureJson
            The output should include "chmod 0600 azure.json"
            The output should include "chown root:root azure.json"
            The contents of file "azure.json" should include '"cloud": "AzurePublicCloud"'
            The contents of file "azure.json" should include '"aadClientId": "sp-client-id"'
            The contents of file "azure.json" should include '"aadClientSecret": "sp-secret"'
            The contents of file "azure.json" should include '"tenantId": "tenant-id"'
            The contents of file "azure.json" should include '"subscriptionId": "subscription-id"'
            The contents of file "azure.json" should include '"resourceGroup": "resource-group"'
            The contents of file "azure.json" should include '"location": "eastus"'
            The contents of file "azure.json" should include '"cloudProviderBackoffMode": "v2"'
            The contents of file "azure.json" should not include "cloudProviderBackoffExponent"
            The contents of file "azure.json" should not include "cloudProviderBackoffJitter"
            The stderr should not eq '' # since we're calling "set" with +/-x numerous times
            The status should be success
        End

        It 'should create the AKS custom cloud json file if running in custom cloud environment'
            SERVICE_PRINCIPAL_CLIENT_ID="sp-client-id"
            SERVICE_PRINCIPAL_FILE_CONTENT="c3Atc2VjcmV0Cg==" # base64 encoding of "sp-secret"
            CLOUDPROVIDER_BACKOFF_MODE="v2"
            IS_CUSTOM_CLOUD="true"
            CUSTOM_ENV_JSON="eyJjdXN0b20iOnRydWV9Cg==" # base64 encoding of '{"custom":true}'
            When run configureAzureJson
            The output should include "chmod 0600 azure.json"
            The output should include "chown root:root azure.json"
            The output should include "chmod 0600 customcloud.json"
            The output should include "chown root:root customcloud.json"
            The contents of file "azure.json" should include '"cloud": "AzurePublicCloud"'
            The contents of file "azure.json" should include '"aadClientId": "sp-client-id"'
            The contents of file "azure.json" should include '"aadClientSecret": "sp-secret"'
            The contents of file "azure.json" should include '"tenantId": "tenant-id"'
            The contents of file "azure.json" should include '"subscriptionId": "subscription-id"'
            The contents of file "azure.json" should include '"resourceGroup": "resource-group"'
            The contents of file "azure.json" should include '"location": "eastus"'
            The contents of file "azure.json" should include '"cloudProviderBackoffMode": "v2"'
            The contents of file "azure.json" should not include "cloudProviderBackoffExponent"
            The contents of file "azure.json" should not include "cloudProviderBackoffJitter"
            The contents of file "customcloud.json" should include '"custom":true'
            The stderr should not eq '' # since we're calling "set" with +/-x numerous times
            The status should be success
        End
    End
    Describe 'ensureContainerd'
        It 'should not overwrite an existing NVIDIA containerd config'
            grep() {
                echo "grep $@"
                return 0
            }

            mkdir() {
                echo "mkdir $@"
            }

            rm() {
                echo "rm $@"
            }

            tee() {
                echo "tee $@"
                cat >/dev/null
            }

            retrycmd_if_failure() {
                echo "retrycmd_if_failure $@"
                return 0
            }

            systemctlEnableAndStartNoBlock() {
                echo "systemctlEnableAndStartNoBlock $@"
                return 0
            }

            should_e2e_mock_azure_china_cloud() {
                echo "false"
            }

            GPU_NODE="false"
            TARGET_CLOUD="AzurePublicCloud"
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER=""
            ERR_SYSCTL_RELOAD=1
            ERR_SYSTEMCTL_START_FAIL=1

            When call ensureContainerd

            The output should include 'grep -q BinaryName = "/usr/bin/nvidia-container-runtime" /etc/containerd/config.toml'
            The output should include "NVIDIA containerd config already exists at /etc/containerd/config.toml, skipping generation"
            The output should not include "rm -f /etc/containerd/config.toml"
            The output should not include "Generating containerd config"
            The output should not include "Generating GPU containerd config"
            The output should not include "Generating non-GPU containerd config"
            The output should include "systemctlEnableAndStartNoBlock containerd 30"
            The status should be success
        End
    End
    Describe 'configureContainerdRegistryHost'
        It 'should configure registry host correctly if MCR_REPOSITORY_BASE is unset'
            mkdir() {
                echo "mkdir $@"
            }
            touch() {
                echo "touch $@"
            }
            chmod() {
                echo "chmod $@"
            }
            tee() {
                echo "tee $@"
            }
            When call configureContainerdRegistryHost
            The variable CONTAINERD_CONFIG_REGISTRY_HOST_MCR should equal '/etc/containerd/certs.d/mcr.microsoft.com/hosts.toml'
            The output should include "mkdir -p /etc/containerd/certs.d/mcr.microsoft.com"
            The output should include "touch /etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"
            The output should include "chmod 0644 /etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"
            The output should not include "tee"
        End

        It 'should configure registry host correctly if MCR_REPOSITORY_BASE is set'
            mkdir() {
                echo "mkdir $@"
            }
            touch() {
                echo "touch $@"
            }
            chmod() {
                echo "chmod $@"
            }
            tee() {
                echo "tee $@"
            }
            MCR_REPOSITORY_BASE="fake.test.com"
            When call configureContainerdRegistryHost
            The variable CONTAINERD_CONFIG_REGISTRY_HOST_MCR should equal '/etc/containerd/certs.d/fake.test.com/hosts.toml'
            The output should include "mkdir -p /etc/containerd/certs.d/fake.test.com"
            The output should include "touch /etc/containerd/certs.d/fake.test.com/hosts.toml"
            The output should include "chmod 0644 /etc/containerd/certs.d/fake.test.com/hosts.toml"
            The output should not include "tee"
        End

        It 'should configure registry host correctly if MCR_REPOSITORY_BASE has the suffic "/"'
            mkdir() {
                echo "mkdir $@"
            }
            touch() {
                echo "touch $@"
            }
            chmod() {
                echo "chmod $@"
            }
            tee() {
                echo "tee $@"
            }
            MCR_REPOSITORY_BASE="fake.test.com/"
            When call configureContainerdRegistryHost
            The variable CONTAINERD_CONFIG_REGISTRY_HOST_MCR should equal '/etc/containerd/certs.d/fake.test.com/hosts.toml'
            The output should include "mkdir -p /etc/containerd/certs.d/fake.test.com"
            The output should include "touch /etc/containerd/certs.d/fake.test.com/hosts.toml"
            The output should include "chmod 0644 /etc/containerd/certs.d/fake.test.com/hosts.toml"
            The output should not include "tee"
        End

        It 'should configure registry host correctly if BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is abc.azurecr.io'
            mkdir() {
                echo "mkdir $@"
            }
            touch() {
                echo "touch $@"
            }
            chmod() {
                echo "chmod $@"
            }
            tee() {
                echo "tee $@"
            }
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="abc.azurecr.io"
            When call configureContainerdRegistryHost
            The variable CONTAINERD_CONFIG_REGISTRY_HOST_MCR should equal '/etc/containerd/certs.d/mcr.microsoft.com/hosts.toml'
            The variable CONTAINER_REGISTRY_URL should equal 'abc.azurecr.io/v2/'
            The output should include "mkdir -p /etc/containerd/certs.d/mcr.microsoft.com"
            The output should include "touch /etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"
            The output should include "chmod 0644 /etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"
            The output should not include "tee"
        End

        It 'should configure registry host correctly if BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is abc.azurecr.io/def'
            mkdir() {
                echo "mkdir $@"
            }
            touch() {
                echo "touch $@"
            }
            chmod() {
                echo "chmod $@"
            }
            tee() {
                echo "tee $@"
            }
            BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="abc.azurecr.io/def"
            When call configureContainerdRegistryHost
            The variable CONTAINERD_CONFIG_REGISTRY_HOST_MCR should equal '/etc/containerd/certs.d/mcr.microsoft.com/hosts.toml'
            The variable CONTAINER_REGISTRY_URL should equal 'abc.azurecr.io/v2/def/'
            The output should include "mkdir -p /etc/containerd/certs.d/mcr.microsoft.com"
            The output should include "touch /etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"
            The output should include "chmod 0644 /etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"
            The output should not include "tee"
        End
    End
    Describe 'configureContainerdLegacyMooncakeMcrHost'
        It 'should configure registry host correctly'
            mkdir() {
                echo "mkdir $@"
            }
            touch() {
                echo "touch $@"
            }
            chmod() {
                echo "chmod $@"
            }
            tee() {
                echo "tee $@"
            }
            When call configureContainerdLegacyMooncakeMcrHost
            The variable CONTAINERD_CONFIG_REGISTRY_HOST_MCR should equal '/etc/containerd/certs.d/mcr.azk8s.cn/hosts.toml'
            The output should include "mkdir -p /etc/containerd/certs.d/mcr.azk8s.cn"
            The output should include "touch /etc/containerd/certs.d/mcr.azk8s.cn/hosts.toml"
            The output should include "chmod 0644 /etc/containerd/certs.d/mcr.azk8s.cn/hosts.toml"
        End
    End
    Describe 'configureSSHPubkeyAuth CIS-compliant sshd_config permissions'
        # These are static assertions on cse_config.sh to guard against a
        # regression of the CIS Benchmark 5.1.1 fix, which requires
        # /etc/ssh/sshd_config to be mode 0600 (or more restrictive) and
        # owned by root:root. Previously, configureSSHPubkeyAuth used
        # `install -m 644 ...` which overwrote the VHD-hardened 0600 mode
        # from configureSsh() in cis.sh, causing CIS control 5.1.1 to fail
        # on Ubuntu 22.04 and 24.04 nodes.
        #
        # If configureSSHPubkeyAuth ever reverts to a non-compliant mode
        # (e.g. 644, 640, 660, 755) when replacing $SSHD_CONFIG, these
        # tests will fail and flag the regression before it ships.
        cse_config_path="./parts/linux/cloud-init/artifacts/cse_config.sh"

        It 'replaces sshd_config with mode 0600 (CIS Benchmark 5.1.1)'
            When call grep -E '^[[:space:]]*install[[:space:]]+-m[[:space:]]+0?600[[:space:]]+-o[[:space:]]+root[[:space:]]+-g[[:space:]]+root[[:space:]]+"\$TMP"[[:space:]]+"\$SSHD_CONFIG"' "$cse_config_path"
            The status should be success
            The output should include 'install'
            The output should include 'SSHD_CONFIG'
        End

        It 'does not replace sshd_config with a world/group-readable mode'
            When call grep -E '^[[:space:]]*install[[:space:]]+-m[[:space:]]+(0?(644|640|660|755|777))[[:space:]].*"\$SSHD_CONFIG"' "$cse_config_path"
            The status should be failure
        End
    End
    Describe 'configureSwapFile'
        SWAP_FILE_SIZE_MB=1

        function [ {
            if test "$1" = "-L" && test "$2" = "/dev/disk/azure/resource-part1"; then
                return 0
            fi
            local last_arg=""
            for last_arg in "$@"; do :; done
            if test "${last_arg}" = "]"; then
                command [ "$@"
            else
                command [ "$@" ]
            fi
        }

        readlink() {
            case "$2" in
                /dev/disk/azure/resource-part1) echo "/dev/sdb1" ;;
                /dev/disk/azure/root) echo "/dev/sda1" ;;
            esac
        }

        retrycmd_if_failure() {
            echo "retrycmd_if_failure $*"
        }

        chmod() {
            echo "chmod $*"
        }

        swapFileIsActive() {
            echo "swapFileIsActive $1"
        }

        reconcileSwapFilePersistence() {
            echo "reconcileSwapFilePersistence $1"
        }

        It 'falls back to OS disk when resource disk mountpoint cannot be determined'
            findmnt() {
                return 1
            }
            df() {
                printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/sda1 1000000 0 1000000 0%% /\n'
            }

            When call configureSwapFile

            The status should be success
            The output should include "Could not determine resource disk mountpoint, attempting to fall back to OS disk..."
            The output should include "Will use OS disk for swap file"
            The output should include "Swap file will be saved to: /swapfile"
            The output should include "reconcileSwapFilePersistence /swapfile"
        End

        It 'falls back to OS disk when resource disk free space cannot be determined'
            findmnt() {
                echo "/mnt/resource"
            }
            df() {
                case "$2" in
                    /mnt/resource)
                        printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
                        ;;
                    /)
                        printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/sda1 1000000 0 1000000 0%% /\n'
                        ;;
                esac
            }

            When call configureSwapFile

            The status should be success
            The output should include "Could not determine free space on resource disk, attempting to fall back to OS disk..."
            The output should include "Will use OS disk for swap file"
            The output should include "Swap file will be saved to: /swapfile"
            The output should include "reconcileSwapFilePersistence /swapfile"
        End

        It 'waits for the OS filesystem resize before creating the swap file'
            DISK_FREE_KB=500
            findmnt() {
                return 1
            }
            df() {
                printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/sda1 1000000 0 %s 0%% /\n' "${DISK_FREE_KB}"
            }
            sleep() {
                echo "sleep $1"
                DISK_FREE_KB=1000000
            }

            When call configureSwapFile

            The status should be success
            The output should include "waiting up to 30 seconds for filesystem resize"
            The output should include "sleep 1"
            The output should include "Will use OS disk for swap file"
            The output should include "Swap file will be saved to: /swapfile"
        End

        It 'fails after waiting 30 seconds when the OS disk remains too small'
            DISK_FREE_KB=500
            findmnt() {
                return 1
            }
            df() {
                printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/sda1 1000000 0 %s 0%% /\n' "${DISK_FREE_KB}"
            }
            sleep() {
                echo "sleep $1"
            }

            When run configureSwapFile

            The status should equal "$ERR_SWAP_CREATE_INSUFFICIENT_DISK_SPACE"
            The output should include "waiting up to 30 seconds for filesystem resize"
            The output should include "after waiting for filesystem resize"
            The output should not include "Swap file will be saved to"
        End
    End
    Describe 'reconcileSwapFilePersistence'
        findExistingSwapFileLocation() {
            return 1
        }

        configureSwapFile() {
            echo "createSwapFile"
        }

        ensureSwapFileFstabEntry() {
            echo "ensureSwapFileFstabEntry $1"
        }

        configureSwapFileSystemdService() {
            echo "configureSwapFileSystemdService $1"
        }

        It 'creates the requested swap file when no existing swap file is present'
            When call reconcileSwapFilePersistence

            The status should be success
            The output should include "No existing AKS swap file found; creating swap file for persistence reconciliation"
            The output should include "createSwapFile"
            The output should not include "ensureSwapFileFstabEntry"
            The output should not include "configureSwapFileSystemdService"
        End

        It 'reconciles persistence for an existing swap file without recreating it'
            When call reconcileSwapFilePersistence "/swapfile"

            The status should be success
            The output should include "ensureSwapFileFstabEntry /swapfile"
            The output should include "configureSwapFileSystemdService /swapfile"
            The output should not include "createSwapFile"
        End

        It 'exits when fstab reconciliation fails'
            ensureSwapFileFstabEntry() {
                return 1
            }

            When run reconcileSwapFilePersistence "/swapfile"

            The status should equal "$ERR_SWAP_CREATE_FAIL"
            The output should not include "configureSwapFileSystemdService"
        End

        It 'exits when swap systemd service reconciliation fails'
            configureSwapFileSystemdService() {
                return 1
            }

            When run reconcileSwapFilePersistence "/swapfile"

            The status should equal "$ERR_SWAP_CREATE_FAIL"
            The output should include "ensureSwapFileFstabEntry /swapfile"
        End
    End
    Describe 'configureK8s client key permissions'
        # Use a throwaway temp dir rather than the real /etc/kubernetes/certs path: when
        # ShellSpec is installed natively (spec/README.md's "Option 2") instead of via the
        # recommended container, tests run directly against the host filesystem, and this
        # test would otherwise delete/overwrite a real node's kubelet credential.
        setup_client_key() {
            CLIENT_KEY_TEST_DIR="$(mktemp -d "${SHELLSPEC_TMPBASE:-${TMPDIR:-/tmp}}/client-key.XXXXXX")"
            CLIENT_KEY_PATH="${CLIENT_KEY_TEST_DIR}/client.key"
            APISERVER_PUBLIC_KEY=""
            ENABLE_SECURE_TLS_BOOTSTRAPPING="false"
            unset TLS_BOOTSTRAP_TOKEN
            KUBELET_CLIENT_CONTENT="Y2xpZW50LWtleQ=="
            KUBELET_CLIENT_CERT_CONTENT=""
            KUBELET_CONFIG_FILE_ENABLED="false"
        }

        cleanup_client_key() {
            set +x
            command rm -rf "${CLIENT_KEY_TEST_DIR}"
        }

        invoke_configureK8s() {
            {
                configureK8s
                local status=$?
                set +x
                return "${status}"
            } 2>/dev/null
        }

        BeforeEach 'setup_client_key'
        AfterEach 'cleanup_client_key'

        It 'writes the kubelet client private key with mode 0600'
            When call invoke_configureK8s

            The status should be success
            The contents of file "${CLIENT_KEY_PATH}" should equal 'client-key'
            The value "$(stat -c '%a' "${CLIENT_KEY_PATH}")" should equal '600'
        End

        It 'creates the client key as 0600 at creation time, not only after a later chmod'
            # Stub chmod as a no-op so the only thing that can have set a restrictive mode
            # is the umask-scoped touch itself; this proves there is no world-readable window.
            chmod() { :; }

            When call invoke_configureK8s

            The status should be success
            The value "$(stat -c '%a' "${CLIENT_KEY_PATH}")" should equal '600'
        End

        It 'does not write the private key if chmod fails'
            chmod() { return 1; }

            When call invoke_configureK8s

            The status should equal "$ERR_KUBELET_CLIENT_KEY_PERM_FAIL"
            # touch still ran before the guarded chmod, so an empty file may exist;
            # what matters is the secret content was never written into it.
            The contents of file "${CLIENT_KEY_PATH}" should not equal 'client-key'
        End

        It 'does not write the private key if touch fails'
            touch() { return 1; }

            When call invoke_configureK8s

            The status should equal "$ERR_KUBELET_CLIENT_KEY_PERM_FAIL"
            The path "${CLIENT_KEY_PATH}" should not be exist
        End
    End
    Describe 'configureKMPP'
        setup_kmpp() {
            KMPP_TEST_DIR="$(mktemp -d)"
            KMPP_UNIT_DIR="${KMPP_TEST_DIR}/unit"
            KMPP_SYSTEMD_ETC_DIR="${KMPP_TEST_DIR}/etc"
            KMPP_CALLS="${KMPP_TEST_DIR}/calls"
            /bin/mkdir -p "${KMPP_UNIT_DIR}" "${KMPP_SYSTEMD_ETC_DIR}/dbus-com.microsoft.kmpp.service.d" "${KMPP_SYSTEMD_ETC_DIR}/dbus-com.microsoft.kmppctrl.service.d"
            touch "${KMPP_UNIT_DIR}/dbus-com.microsoft.kmpp.service"
            KUBELET_NODE_LABELS="kubernetes.azure.com/role=agent"
            unset FAIL_ON
        }
        cleanup_kmpp() {
            rm -rf "${KMPP_TEST_DIR}"
            unset KMPP_TEST_DIR KMPP_UNIT_DIR KMPP_SYSTEMD_ETC_DIR KMPP_CALLS KUBELET_NODE_LABELS FAIL_ON
        }
        record() {
            echo "$*" >> "${KMPP_CALLS}"
            case "$*" in *"${FAIL_ON:-__none__}"*) return 1 ;; esac
            return 0
        }
        retrycmd_if_failure() { shift 3; record "$@"; }
        systemctlEnableAndStart() { record "enable-start $1"; }
        systemctl() { record "systemctl $*"; }
        mkdir() { record "mkdir $*"; }
        chmod() { record "chmod $*"; }

        BeforeEach 'setup_kmpp'
        AfterEach 'cleanup_kmpp'

        It 'skips and adds no label when the VHD has no KMPP'
            rm -f "${KMPP_UNIT_DIR}/dbus-com.microsoft.kmpp.service"
            When call configureKMPP
            The status should be success
            The output should include "KMPP is not baked into this VHD"
            The path "${KMPP_CALLS}" should not be exist
            The variable KUBELET_NODE_LABELS should equal "kubernetes.azure.com/role=agent"
        End

        It 'moves KMPP under system.slice, starts it, bind-mounts the system bus, checks GetVersion and labels the node'
            When call configureKMPP
            The status should be success
            The output should include "adding label kubernetes.azure.com/kmpp=enabled"
            The contents of file "${KMPP_CALLS}" should equal "mkdir -p ${KMPP_SYSTEMD_ETC_DIR}/dbus-com.microsoft.kmpp.service.d ${KMPP_SYSTEMD_ETC_DIR}/dbus-com.microsoft.kmppctrl.service.d /run/kmpp/dbus
chmod 0644 ${KMPP_SYSTEMD_ETC_DIR}/system-kmpp.slice ${KMPP_SYSTEMD_ETC_DIR}/dbus-com.microsoft.kmpp.service.d/10-system-slice.conf ${KMPP_SYSTEMD_ETC_DIR}/dbus-com.microsoft.kmppctrl.service.d/10-system-slice.conf ${KMPP_SYSTEMD_ETC_DIR}/run-kmpp-dbus.mount
systemctl mask kmpp.slice
systemctl daemon-reload
systemctl unmask dbus-com.microsoft.kmpp.service dbus-com.microsoft.kmppctrl.service
enable-start dbus-com.microsoft.kmpp.service
systemctl start dbus-com.microsoft.kmppctrl.service
enable-start run-kmpp-dbus.mount
busctl --address=unix:path=/run/kmpp/dbus/system_bus_socket call com.microsoft.kmpp / com.microsoft.kmpp GetVersion"
            The contents of file "${KMPP_SYSTEMD_ETC_DIR}/system-kmpp.slice" should include "CPUQuota=50%"
            The contents of file "${KMPP_SYSTEMD_ETC_DIR}/system-kmpp.slice" should include "MemoryMax=20M"
            The contents of file "${KMPP_SYSTEMD_ETC_DIR}/dbus-com.microsoft.kmpp.service.d/10-system-slice.conf" should include "Slice=system-kmpp.slice"
            The contents of file "${KMPP_SYSTEMD_ETC_DIR}/dbus-com.microsoft.kmppctrl.service.d/10-system-slice.conf" should include "Slice=system-kmpp.slice"
            The contents of file "${KMPP_SYSTEMD_ETC_DIR}/run-kmpp-dbus.mount" should include "What=/run/dbus"
            The contents of file "${KMPP_SYSTEMD_ETC_DIR}/run-kmpp-dbus.mount" should include "Where=/run/kmpp/dbus"
            The contents of file "${KMPP_SYSTEMD_ETC_DIR}/run-kmpp-dbus.mount" should include "Options=bind"
            The variable KUBELET_NODE_LABELS should equal "kubernetes.azure.com/role=agent,kubernetes.azure.com/kmpp=enabled"
        End

        Parameters
            "masking kmpp.slice" "systemctl mask kmpp.slice"
            "unmask" "systemctl unmask"
            "the KMPP service start" "enable-start dbus-com.microsoft.kmpp.service"
            "the kmppctrl start" "systemctl start dbus-com.microsoft.kmppctrl.service"
            "daemon-reload" "systemctl daemon-reload"
            "the bind mount" "enable-start run-kmpp-dbus.mount"
            "the GetVersion check" "GetVersion"
        End
        It "fails with ERR_KMPP_START_FAIL and adds no label when $1 fails"
            FAIL_ON="$2"
            When call configureKMPP
            The status should equal "$ERR_KMPP_START_FAIL"
            The output should not include "adding label kubernetes.azure.com/kmpp=enabled"
            The variable KUBELET_NODE_LABELS should equal "kubernetes.azure.com/role=agent"
        End
    End
End
