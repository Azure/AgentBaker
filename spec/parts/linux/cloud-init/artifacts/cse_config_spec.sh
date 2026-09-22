#!/bin/bash

check_test_fstab_permissions() {
    printf "0%s" "$(getFileMode "$TEST_FSTAB_FILE")"
}

Describe 'cse_config.sh'
    CSE_CONFIG_GPU_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"
    CSE_CONFIG_LOCALDNS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_localdns.sh"
    CSE_CONFIG_KUBELET_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh"
    CSE_CONFIG_NETWORK_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_network.sh"
    CSE_CONFIG_ADDONS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_addons.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
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
            TEST_EVENTS_DIR=$(mktemp -d)
            EVENTS_LOGGING_DIR=$TEST_EVENTS_DIR
        }

        cleanup_ssh_telemetry() {
            rm -f "$TEST_EVENTS_DIR"/*
            rmdir "$TEST_EVENTS_DIR"
        }

        ssh_events_match() {
            jq -se "$SSH_EVENT_FILTER" "$TEST_EVENTS_DIR"/*.json
        }

        BeforeEach setup_ssh_telemetry
        AfterEach cleanup_ssh_telemetry

        systemctl() {
            case "$1" in
                daemon-reload) return 0 ;;
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
                show)
                    if [ "${FAIL_STATE_QUERY:-false}" = true ]; then
                        return 124
                    fi
                    case " ${MISSING_UNITS:-} " in
                        *" $2 "*)
                            printf 'LoadState=not-found\nActiveState=inactive\nUnitFileState=\n'
                            return 4
                            ;;
                    esac
                    printf 'LoadState=loaded\nActiveState=%s\nUnitFileState=%s\n' \
                        "${ACTIVE_STATE:-inactive}" "${UNIT_FILE_STATE:-disabled}"
                    ;;
                *) return 1 ;;
            esac
        }

        timeout() {
            if [ "${3:-}" = show ] && [ "$1" != 2 ]; then
                return 99
            fi
            shift
            "$@"
        }

        sleep() {
            :
        }

        It 'stops and disables every SSH unit that is present'
            When call disableSSH
            The output should equal "stop ssh
disable ssh
stop sshd
disable sshd
stop sshd.socket
disable sshd.socket"
            The status should be success
            SSH_EVENT_FILTER='
                length == 8 and
                ([.[].Message | fromjson | .AttemptId] | unique | length == 1) and
                all(.[]; (.Message | type) == "string" and .EventLevel == "Informational") and
                ([.[] | select(.TaskName == "AKS.CSE.disableSSH") | .Message | fromjson |
                  select(.Phase == "Completed")] |
                    length == 1 and .[0].ExitCode == 0 and .[0].DurationSeconds >= 0) and
                ([.[] | select(.TaskName == "AKS.CSE.disableSSH.unit") | .Message | fromjson |
                  select(.Phase == "Completed")] |
                    length == 3 and all(.[];
                        .CatExitCode == 0 and .StopExitCode == 0 and .DisableExitCode == 0 and
                        .Skipped == false and .StateQueryExitCode == 0 and
                        .LoadState == "loaded" and .ActiveState == "inactive" and
                        .UnitFileState == "disabled" and .SchemaVersion == 1)) and
                ([.[].Message | fromjson | select(.Phase == "Started")] |
                    length == 4 and all(.[]; .ExitCode == null))
            '
            The result of function ssh_events_match should equal true
        End

        It 'skips units that are not installed'
            MISSING_UNITS="ssh sshd"
            When call disableSSH
            The output should equal "stop sshd.socket
disable sshd.socket"
            The status should be success
            SSH_EVENT_FILTER='
                [.[].Message | fromjson | select(.Phase == "Completed" and .Skipped == true)] |
                length == 2 and all(.[]; .CatExitCode == 1 and .StopExitCode == null and
                    .DisableExitCode == null and .LoadState == "not-found" and .ExitCode == 0)
            '
            The result of function ssh_events_match should equal true
        End

        It 'skips sshd.socket on distros without socket activation'
            MISSING_UNITS="ssh sshd.socket"
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
            The status should equal 172
            SSH_EVENT_FILTER='
                [.[].Message | fromjson] |
                any(.[]; .Unit == "sshd" and .StopExitCode == 1 and .DisableExitCode == 0) and
                any(.[]; .Unit == "" and .ExitCode == 172 and .Phase == "Completed") and
                all(.[]; .Unit != "sshd.socket")
            '
            The result of function ssh_events_match should equal true
        End

        It 'records observed states without changing successful command results'
            ACTIVE_STATE=active
            UNIT_FILE_STATE=enabled
            PRE_PROVISION_ONLY=true
            OS=AZURELINUX
            OS_VERSION=3.0
            OS_VARIANT=AZURECONTAINERLINUX
            When call disableSSH
            The status should be success
            The output should include "disable sshd.socket"
            SSH_EVENT_FILTER='
                [.[].Message | fromjson | select(.Phase == "Completed" and .Unit == "sshd.socket")] |
                length == 1 and .[0].ActiveState == "active" and .[0].UnitFileState == "enabled" and
                .[0].ExitCode == 0 and .[0].PreProvisionOnly == true and
                .[0].OS == "AZURELINUX" and .[0].OSVersion == "3.0" and .[0].OSVariant == "AZURECONTAINERLINUX"
            '
            The result of function ssh_events_match should equal true
        End

        It 'records a bounded state-query failure without failing provisioning'
            FAIL_STATE_QUERY=true
            When call disableSSH
            The status should be success
            The output should include "disable sshd.socket"
            SSH_EVENT_FILTER='
                [.[].Message | fromjson | select(.Phase == "Completed" and .Unit != "")] |
                length == 3 and all(.[]; .StateQueryExitCode == 124 and .ActiveState == "" and .ExitCode == 0)
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
                length == 16 and ([.[].Message | fromjson | .AttemptId] | unique | length == 2)
            '
            The result of function ssh_events_match should equal true
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
            SSH_SERVICE_ACTIVE="true"
            SSH_SERVICE_EXISTS="true"
        }

        cleanup() {
            rm -f "${SSHD_CONFIG_FILE}"
            unset SSHD_CONFIG_FILE
            unset SSH_SERVICE_ACTIVE
            unset SSH_SERVICE_EXISTS
        }

        systemctl() {
            echo "systemctl $*" >&2
            if [ "$1" = "cat" ] && [ "${SSH_SERVICE_EXISTS}" != "true" ]; then
                return 1
            fi
            if [ "$1" = "is-active" ] && [ "${SSH_SERVICE_ACTIVE}" != "true" ]; then
                return 3
            fi
            return 0
        }

        sshd() {
            echo "sshd $*" >&2
            return 0
        }

        install() {
            while [ "$#" -gt 2 ]; do
                shift
            done
            command cp "$1" "$2"
        }

        run_disable_ssh_pubkey_auth() {
            disableSSHPubkeyAuth
            cat "${SSHD_CONFIG_FILE}"
        }

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
End
