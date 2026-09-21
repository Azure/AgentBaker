#!/bin/bash

check_file_permissions() {
    # Use printf to ensure leading zero (0644 format)
    printf "0%s" "$(stat -c "%a" "$LOCALDNS_ENV_FILE")"
}

check_cloud_env_permissions() {
    printf "0%s" "$(stat -c "%a" "$AKS_CLOUD_ENV_FILE")"
}

check_hosts_file_permissions() {
    stat -c '%a' "$AKS_LOCALDNS_HOSTS_FILE"
}

Describe 'cse_config_localdns.sh'
    CSE_CONFIG_GPU_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"
    CSE_CONFIG_LOCALDNS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_localdns.sh"
    CSE_CONFIG_KUBELET_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh"
    CSE_CONFIG_NETWORK_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_network.sh"
    CSE_CONFIG_ADDONS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_addons.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"
    Describe 'enableLocalDNS'
        setup() {
            TMP_DIR=$(mktemp -d)
            LOCALDNS_CORE_FILE="$TMP_DIR/localdns.corefile"
            KUBELET_NODE_LABELS=""
            LOCALDNS_SLICE_FILE="$TMP_DIR/localdns.slice"
            LOCALDNS_COREFILE_WITH_HOSTS=$(echo -n "localdns corefile with hosts" | base64)
            LOCALDNS_COREFILE_BASE=$(echo -n "localdns corefile" | base64)
            LOCALDNS_MEMORY_LIMIT="128M"
            LOCALDNS_CPU_LIMIT="200.0%"
            # Create mock localdns assets that would be present on VHD
            mkdir -p /etc/systemd/system
            mkdir -p /opt/azure/containers/localdns
            touch /etc/systemd/system/localdns.service
            touch /opt/azure/containers/localdns/localdns.sh

            # enableLocalDNS's retry loop calls check_cse_timeout, which warns on stderr
            # when this is unset. Set it so the real guard is exercised (elapsed ~0s, well
            # under CSE_MAX_DURATION_SECONDS) instead of taking its unset short-circuit.
            CSE_STARTTIME_SECONDS=$(date +%s)

            # enableLocalDNS drives systemd directly rather than going through
            # systemctlEnableAndStart, so it can clear the StartLimit budget
            # between attempts. Mock the primitives it actually calls.
            systemctl() {
                echo "systemctl $*"
                return 0
            }
            timeout() {
                shift
                "$@"
            }
            retrycmd_if_failure() {
                echo "retrycmd_if_failure $*"
                return 0
            }
            # enableLocalDNS captures the journal alongside 'systemctl status' on the failure
            # paths. Mock it so the assertions are deterministic and no real journal is read.
            journalctl() {
                echo "journalctl $*"
                return 0
            }
            # The give-up and enable-failure paths redirect into this directory. Without it the
            # redirect fails before the command runs and the '|| true' hides it, so the tests
            # would pass without ever exercising the capture.
            mkdir -p /var/log/azure
            rm -f /var/log/azure/localdns-status.log
            sleep() {
                :
            }
            systemctlEnableAndStartNoBlock() {
                echo "systemctlEnableAndStartNoBlock $@"
                return 0
            }
            addKubeletNodeLabel() {
                echo "addKubeletNodeLabel $1"
                if [[ -z "$KUBELET_NODE_LABELS" ]]; then
                    KUBELET_NODE_LABELS="$1"
                else
                    KUBELET_NODE_LABELS="$KUBELET_NODE_LABELS,$1"
                fi
            }
        }
        cleanup() {
            rm -rf "$TMP_DIR"
            # Clean up mock VHD assets
            rm -f /etc/systemd/system/localdns.service
            rm -f /opt/azure/containers/localdns/localdns.sh
        }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'should enable localdns successfully when VHD has required assets'
            When run enableLocalDNS
            The status should be success
            The output should include "localdns should be enabled."
            The output should include "Enable localdns succeeded."
            # Note: exporter socket setup is now in configureLocalDNSExporterSocket (called separately in cse_main.sh)
            The output should not include "localdns-exporter"
        End

        It 'should skip localdns when localdns.service is missing on old VHD'
            rm -f /etc/systemd/system/localdns.service
            When run enableLocalDNS
            The status should be success
            The output should include "Warning: localdns.service not found on this VHD, skipping localdns setup"
            The output should not include "localdns should be enabled."
        End

        It 'should skip localdns when localdns.sh is missing on old VHD'
            rm -f /opt/azure/containers/localdns/localdns.sh
            When run enableLocalDNS
            The status should be success
            The output should include "Warning: localdns.sh not found on this VHD, skipping localdns setup"
            The output should not include "localdns should be enabled."
        End

        # Record an ordered trace of the calls that matter: R for reset-failed, S for a
        # start attempt. Asserting on the trace is what pins "before *each* attempt" -- a
        # test that only checks both strings appear somewhere would still pass if
        # reset-failed were hoisted out of the loop.
        tracing_systemctl() {
            systemctl() {
                case "$1" in
                    reset-failed) printf 'R' >> "$TMP_DIR/trace" ;;
                    restart)      printf 'S' >> "$TMP_DIR/trace" ;;
                esac
                echo "systemctl $*"
                if [ "$1" = "restart" ]; then
                    restart_calls=$((restart_calls + 1))
                    if [ "$restart_calls" -lt "$restart_failures_before_success" ]; then
                        return 1
                    fi
                fi
                return 0
            }
            restart_calls=0
        }

        It 'should reset the StartLimit budget before every attempt, not only the first'
            # Two failed restarts then a success, so the loop runs three times.
            restart_failures_before_success=3
            tracing_systemctl
            When run enableLocalDNS
            The status should be success
            The output should include "Enable localdns succeeded."
            # R before every S, three times over -- not RSSS.
            The contents of file "$TMP_DIR/trace" should equal "RSRSRS"
        End

        It 'should return error when systemctl fails to start localdns'
            # Never succeeds, so the loop exhausts and takes the give-up path.
            restart_failures_before_success=99999
            tracing_systemctl
            When run enableLocalDNS
            The status should equal 216
            The output should include "localdns should be enabled."
            The output should include "systemctl reset-failed localdns"
            # The give-up path deliberately does not reset, so the unit is left in 'failed'
            # for NPD: the trace must end on a start attempt, never on a reset.
            The contents of file "$TMP_DIR/trace" should end with "S"
        End

        It 'should say why it gave up and capture the journal, not just the unit state'
            # systemctlEnableAndStart logged 'systemctl status' plus 'journalctl -u' on every
            # failed attempt; inlining the loop dropped all of it. The snapshot alone shows a
            # unit mid-restart-cycle and does not explain any of the failures, so the journal
            # has to come with it.
            restart_failures_before_success=99999
            tracing_systemctl
            When run enableLocalDNS
            The status should equal 216
            The output should include "localdns could not be started: exhausted the restart attempts."
            The contents of file /var/log/azure/localdns-status.log should include "journalctl -u localdns"
        End

        It 'should report a CSE budget give-up differently from exhausting the attempts'
            # The two give-up reasons need different messages: one means localdns is broken,
            # the other means provisioning ran out of time and never finished trying.
            restart_failures_before_success=99999
            tracing_systemctl
            check_cse_timeout() { return 1; }
            When run enableLocalDNS
            The status should equal 216
            The output should include "CSE provisioning budget exhausted at attempt 1"
            The output should not include "exhausted the restart attempts"
        End

        It 'should sample diagnostics during the retries without dumping on every attempt'
            # Bounded and periodic on purpose: the old helper dumped status plus an unbounded
            # journal on all 99 failed attempts, measured at 6-8s per iteration, which ate the
            # provisioning window it was retrying inside.
            restart_failures_before_success=11
            tracing_systemctl
            When run enableLocalDNS
            The status should be success
            The output should include "localdns restart attempt 10 failed"
            The output should include "journalctl -u localdns --no-pager -n 50"
            The output should not include "localdns restart attempt 9 failed"
            The output should include "Enable localdns succeeded."
        End

        It 'should log and capture status when systemctl enable fails'
            # systemctlEnableAndStart wrote a status log on the enable-failure path as well as
            # the start-failure path. Inlining the loop kept the first and dropped the second,
            # so an enable failure exited with nothing but the code.
            retrycmd_if_failure() { return 1; }
            When run enableLocalDNS
            The status should equal 216
            The output should include "localdns could not be enabled by systemctl."
            The output should not include "Enable localdns succeeded."
            The contents of file /var/log/azure/localdns-status.log should include "journalctl -u localdns"
        End

        It 'should distinguish a CSE budget timeout from a genuine enable failure'
            # retrycmd_if_failure returns 2 when check_cse_timeout trips and 1 when it burns
            # all its attempts. 'if ! retrycmd ...' would throw that away -- '!' inverts before
            # $? is read -- and report a budget timeout as a systemd failure, sending the
            # on-call after the wrong thing.
            retrycmd_if_failure() { return 2; }
            When run enableLocalDNS
            The status should equal 216
            The output should include "localdns could not be enabled: CSE provisioning budget exhausted."
            The output should not include "could not be enabled by systemctl"
        End
    End
    Describe 'enableLocalDNSForScriptless'
        setup() {
            TMP_DIR=$(mktemp -d)
            LOCALDNS_CORE_FILE="$TMP_DIR/localdns.corefile"
            LOCALDNS_SLICE_FILE="$TMP_DIR/localdns.slice"
            LOCALDNS_COREFILE_BASE=$(echo "bG9jYWxkbnMgY29yZWZpbGU=") # "localdns corefile" base64
            LOCALDNS_COREFILE_WITH_HOSTS=$(echo "bG9jYWxkbnMgY29yZWZpbGU=") # "localdns corefile" base64
            LOCALDNS_MEMORY_LIMIT="512M"
            LOCALDNS_CPU_LIMIT="250%"
            # Create mock localdns assets that would be present on VHD
            mkdir -p /etc/systemd/system
            mkdir -p /opt/azure/containers/localdns
            touch /etc/systemd/system/localdns.service
            touch /opt/azure/containers/localdns/localdns.sh

            # enableLocalDNS's retry loop calls check_cse_timeout, which warns on stderr
            # when this is unset. Set it so the real guard is exercised (elapsed ~0s, well
            # under CSE_MAX_DURATION_SECONDS) instead of taking its unset short-circuit.
            CSE_STARTTIME_SECONDS=$(date +%s)

            # enableLocalDNS drives systemd directly rather than going through
            # systemctlEnableAndStart, so it can clear the StartLimit budget
            # between attempts. Mock the primitives it actually calls.
            systemctl() {
                echo "systemctl $*"
                return 0
            }
            timeout() {
                shift
                "$@"
            }
            retrycmd_if_failure() {
                echo "retrycmd_if_failure $*"
                return 0
            }
            sleep() {
                :
            }
            systemctlEnableAndStartNoBlock() {
                echo "systemctlEnableAndStartNoBlock $@"
                return 0
            }
        }
        cleanup() {
            rm -rf "$TMP_DIR"
            # Clean up mock VHD assets
            rm -f /etc/systemd/system/localdns.service
            rm -f /opt/azure/containers/localdns/localdns.sh
        }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        # Success case.
        It 'should enable localdns successfully'
            When call enableLocalDNS
            The status should be success
            The output should include "localdns should be enabled."
            The output should include "Enable localdns succeeded."
        End

        # Corefile file creation.
        It 'should create localdns.corefile with correct data'
            When call enableLocalDNS
            The status should be success
            The output should include "localdns should be enabled."
            The path "$LOCALDNS_CORE_FILE" should be file
            The contents of file "$LOCALDNS_CORE_FILE" should include "localdns corefile"
            The output should include "localdns should be enabled."
            The output should include "Enable localdns succeeded."
        End

        # Corefile already exists (idempotency).
        It 'should overwrite existing localdns.corefile'
            echo "wrong data" > "$LOCALDNS_CORE_FILE"
            When call enableLocalDNS
            The status should be success
            The path "$LOCALDNS_CORE_FILE" should be file
            The contents of file "$LOCALDNS_CORE_FILE" should include "localdns corefile"
            The output should include "localdns should be enabled."
            The output should include "Enable localdns succeeded."
        End

        # Slice file creation.
        It 'should create localdns.slice with correct CPU and Memory limits'
            When call enableLocalDNS
            The status should be success
            The output should include "localdns should be enabled."
            The path "$LOCALDNS_SLICE_FILE" should be file
            The contents of file "$LOCALDNS_SLICE_FILE" should include "MemoryMax=${LOCALDNS_MEMORY_LIMIT}"
            The contents of file "$LOCALDNS_SLICE_FILE" should include "CPUQuota=${LOCALDNS_CPU_LIMIT}"
            The output should include "localdns should be enabled."
            The output should include "Enable localdns succeeded."
        End

        # Environment file creation with both corefile variants.
        It 'should create environment file with all corefile variants for dynamic selection'
            # Set up both corefile variants
            LOCALDNS_COREFILE_WITH_HOSTS=$(echo -n "corefile with hosts plugin" | base64)
            LOCALDNS_COREFILE_BASE=$(echo -n "corefile without hosts plugin" | base64)
            SHOULD_ENABLE_HOSTS_PLUGIN="true"
            LOCALDNS_ENV_FILE="$TMP_DIR/environment"

            When call enableLocalDNS
            The status should be success
            The stdout should include "enableLocalDNS called, generating corefile..."
            The stdout should include "localdns should be enabled."
            The stdout should include "Enable localdns succeeded."
            The path "$LOCALDNS_ENV_FILE" should be file
            The contents of file "$LOCALDNS_ENV_FILE" should include "LOCALDNS_BASE64_ENCODED_COREFILE="
            The contents of file "$LOCALDNS_ENV_FILE" should include "LOCALDNS_COREFILE_BASE="
            The contents of file "$LOCALDNS_ENV_FILE" should include "LOCALDNS_COREFILE_WITH_HOSTS=${LOCALDNS_COREFILE_WITH_HOSTS}"
            The contents of file "$LOCALDNS_ENV_FILE" should include "SHOULD_ENABLE_HOSTS_PLUGIN=true"
        End

        # Old CSE + new VHD backward compatibility.
        # An old AgentBaker service only sets LOCALDNS_GENERATED_COREFILE (not LOCALDNS_COREFILE_BASE).
        # The new VHD's generateLocalDNSFiles must fall back to the legacy variable.
        It 'should fall back to LOCALDNS_GENERATED_COREFILE when LOCALDNS_COREFILE_BASE is unset (old CSE + new VHD)'
            unset LOCALDNS_COREFILE_BASE
            LOCALDNS_GENERATED_COREFILE=$(echo -n "legacy corefile from old CSE" | base64)
            LOCALDNS_ENV_FILE="$TMP_DIR/environment"

            When call enableLocalDNS
            The status should be success
            The stdout should include "localdns should be enabled."
            The stdout should include "Enable localdns succeeded."
            The path "$LOCALDNS_CORE_FILE" should be file
            The contents of file "$LOCALDNS_CORE_FILE" should include "legacy corefile from old CSE"
            The path "$LOCALDNS_ENV_FILE" should be file
            The contents of file "$LOCALDNS_ENV_FILE" should include "LOCALDNS_BASE64_ENCODED_COREFILE="
            The contents of file "$LOCALDNS_ENV_FILE" should include "LOCALDNS_COREFILE_BASE="
        End

        # Environment file permissions.
        It 'should set correct permissions on environment file'
            LOCALDNS_ENV_FILE="$TMP_DIR/environment"
            When call enableLocalDNS
            The status should be success
            The path "$LOCALDNS_ENV_FILE" should be file
            # Check permissions are 0644 (owner read/write, group read, others read)
            The result of function check_file_permissions should equal "0644"
        End
    End
    Describe 'enableAKSLocalDNSHostsSetup'
        setup() {
            # Create temporary test directories and files
            TEST_TEMP_DIR=$(mktemp -d)
            AKS_LOCALDNS_HOSTS_FILE="${TEST_TEMP_DIR}/hosts"
            AKS_LOCALDNS_HOSTS_SETUP_SCRIPT="${TEST_TEMP_DIR}/aks-localdns-hosts-setup.sh"
            AKS_LOCALDNS_HOSTS_SETUP_SERVICE="${TEST_TEMP_DIR}/aks-localdns-hosts-setup.service"
            AKS_LOCALDNS_HOSTS_SETUP_TIMER="${TEST_TEMP_DIR}/aks-localdns-hosts-setup.timer"
            AKS_CLOUD_ENV_FILE="${TEST_TEMP_DIR}/cloud-env"

            # Create fake script that simulates successful hosts file creation
            cat > "$AKS_LOCALDNS_HOSTS_SETUP_SCRIPT" << 'SETUP_EOF'
#!/bin/bash
echo "# test hosts file" > "${AKS_LOCALDNS_HOSTS_FILE}"
SETUP_EOF
            chmod +x "$AKS_LOCALDNS_HOSTS_SETUP_SCRIPT"

            # Create dummy service and timer files
            touch "$AKS_LOCALDNS_HOSTS_SETUP_SERVICE"
            cat > "$AKS_LOCALDNS_HOSTS_SETUP_TIMER" <<'TIMER_EOF'
[Unit]
Description=Run AKS LocalDNS hosts setup periodically

[Timer]
OnUnitActiveSec=15min
TIMER_EOF

            # Set up test environment
            TARGET_CLOUD="AzurePublicCloud"
            LOCALDNS_CRITICAL_FQDNS="mcr.microsoft.com,packages.microsoft.com,management.azure.com,login.microsoftonline.com,acs-mirror.azureedge.net,packages.aks.azure.com"
            LOCALDNS_HOSTS_PLUGIN_REFRESH_INTERVAL_IN_SECONDS=""

            # Mock systemctl function
            systemctlEnableAndStartNoBlock() {
                echo "systemctlEnableAndStartNoBlock $@"
                return 0
            }
            systemctl() {
                echo "systemctl $@"
                return 0
            }

            # Export variables so the real function can use them
            export AKS_LOCALDNS_HOSTS_FILE AKS_LOCALDNS_HOSTS_SETUP_SCRIPT AKS_LOCALDNS_HOSTS_SETUP_SERVICE
            export AKS_LOCALDNS_HOSTS_SETUP_TIMER AKS_CLOUD_ENV_FILE TARGET_CLOUD LOCALDNS_CRITICAL_FQDNS LOCALDNS_HOSTS_PLUGIN_REFRESH_INTERVAL_IN_SECONDS
        }

        cleanup() {
            rm -rf "$TEST_TEMP_DIR"
            unset AKS_LOCALDNS_HOSTS_FILE AKS_LOCALDNS_HOSTS_SETUP_SCRIPT AKS_LOCALDNS_HOSTS_SETUP_SERVICE
            unset AKS_LOCALDNS_HOSTS_SETUP_TIMER AKS_CLOUD_ENV_FILE TARGET_CLOUD LOCALDNS_CRITICAL_FQDNS LOCALDNS_HOSTS_PLUGIN_REFRESH_INTERVAL_IN_SECONDS
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'should enable aks-localdns-hosts-setup timer successfully'
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "Enabling aks-localdns-hosts-setup timer..."
            The output should include "systemctlEnableAndStartNoBlock aks-localdns-hosts-setup.timer 30"
            The output should include "aks-localdns-hosts-setup timer enabled successfully."
        End

        It 'should call systemctlEnableAndStartNoBlock with correct parameters'
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "systemctlEnableAndStartNoBlock aks-localdns-hosts-setup.timer 30"
        End

        It 'should update the timer refresh interval when provided'
            LOCALDNS_HOSTS_PLUGIN_REFRESH_INTERVAL_IN_SECONDS="30"
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "Configured aks-localdns-hosts-setup timer refresh interval to 30s."
            The output should include "systemctl daemon-reload"
            The contents of file "${AKS_LOCALDNS_HOSTS_SETUP_TIMER}.d/10-refresh-interval.conf" should include "OnUnitActiveSec=30s"
            The contents of file "${AKS_LOCALDNS_HOSTS_SETUP_TIMER}.d/10-refresh-interval.conf" should include "AccuracySec=1s"
            The contents of file "$AKS_LOCALDNS_HOSTS_SETUP_TIMER" should include "OnUnitActiveSec=15min"
        End

        It 'should restore the default timer when refresh interval is unset'
            mkdir -p "${AKS_LOCALDNS_HOSTS_SETUP_TIMER}.d"
            cat > "${AKS_LOCALDNS_HOSTS_SETUP_TIMER}.d/10-refresh-interval.conf" <<'OVERRIDE_EOF'
[Timer]
OnUnitActiveSec=30s
AccuracySec=1s
OVERRIDE_EOF
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "Restored default aks-localdns-hosts-setup timer refresh interval."
            The output should include "systemctl daemon-reload"
            The contents of file "$AKS_LOCALDNS_HOSTS_SETUP_TIMER" should include "OnUnitActiveSec=15min"
            The file "${AKS_LOCALDNS_HOSTS_SETUP_TIMER}.d/10-refresh-interval.conf" should not be exist
        End

        It 'should keep the default timer when refresh interval is invalid'
            LOCALDNS_HOSTS_PLUGIN_REFRESH_INTERVAL_IN_SECONDS="abc"
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "must be an integer, got 'abc'. Using default timer interval."
            The contents of file "$AKS_LOCALDNS_HOSTS_SETUP_TIMER" should include "OnUnitActiveSec=15min"
            The file "${AKS_LOCALDNS_HOSTS_SETUP_TIMER}.d/10-refresh-interval.conf" should not be exist
        End

        It 'should clamp the timer refresh interval when below minimum'
            LOCALDNS_HOSTS_PLUGIN_REFRESH_INTERVAL_IN_SECONDS="1"
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "must be >= 5, got '1'. Clamping to 5s."
            The output should include "Configured aks-localdns-hosts-setup timer refresh interval to 5s."
            The contents of file "$AKS_LOCALDNS_HOSTS_SETUP_TIMER" should include "OnUnitActiveSec=15min"
            The contents of file "${AKS_LOCALDNS_HOSTS_SETUP_TIMER}.d/10-refresh-interval.conf" should include "OnUnitActiveSec=5s"
            The contents of file "${AKS_LOCALDNS_HOSTS_SETUP_TIMER}.d/10-refresh-interval.conf" should include "AccuracySec=1s"
        End

        It 'should skip when setup script is missing'
            rm -f "$AKS_LOCALDNS_HOSTS_SETUP_SCRIPT"
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "not found on this VHD, skipping aks-localdns-hosts-setup"
        End

        It 'should skip when timer unit is missing'
            rm -f "$AKS_LOCALDNS_HOSTS_SETUP_TIMER"
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "not found on this VHD, skipping aks-localdns-hosts-setup"
        End

        It 'should print warning when systemctlEnableAndStartNoBlock fails'
            systemctlEnableAndStartNoBlock() {
                echo "systemctlEnableAndStartNoBlock $@"
                return 1
            }
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "Enabling aks-localdns-hosts-setup timer..."
            The output should include "Warning: Failed to enable aks-localdns-hosts-setup timer"
            The output should not include "aks-localdns-hosts-setup timer enabled successfully."
        End

        It 'should skip when service unit is missing'
            rm -f "$AKS_LOCALDNS_HOSTS_SETUP_SERVICE"
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "not found on this VHD, skipping aks-localdns-hosts-setup"
        End

        It 'should skip when setup script is not executable'
            chmod -x "$AKS_LOCALDNS_HOSTS_SETUP_SCRIPT"
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "is not executable, skipping aks-localdns-hosts-setup"
        End

        It 'should create empty hosts file with correct permissions'
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "aks-localdns-hosts-setup timer enabled successfully."
            The file "$AKS_LOCALDNS_HOSTS_FILE" should be exist
        End

        It 'should succeed with China FQDNs from RP'
            TARGET_CLOUD="AzureChinaCloud"
            LOCALDNS_CRITICAL_FQDNS="mcr.azure.cn,mcr.azk8s.cn,login.partner.microsoftonline.cn,management.chinacloudapi.cn,packages.microsoft.com"
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "aks-localdns-hosts-setup timer enabled successfully."
        End

        It 'should succeed with US Gov FQDNs from RP'
            TARGET_CLOUD="AzureUSGovernmentCloud"
            LOCALDNS_CRITICAL_FQDNS="mcr.microsoft.com,login.microsoftonline.us,management.usgovcloudapi.net,packages.aks.azure.com"
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "aks-localdns-hosts-setup timer enabled successfully."
        End

        It 'should create hosts file with correct permissions'
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "aks-localdns-hosts-setup timer enabled successfully."
            The file "$AKS_LOCALDNS_HOSTS_FILE" should be exist
            The result of function check_hosts_file_permissions should equal "644"
        End

        It 'should skip when LOCALDNS_CRITICAL_FQDNS is unset'
            unset LOCALDNS_CRITICAL_FQDNS
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "WARNING: LOCALDNS_CRITICAL_FQDNS is not set"
            The output should include "Skipping aks-localdns-hosts-setup"
        End

        It 'should skip when LOCALDNS_CRITICAL_FQDNS is empty string'
            LOCALDNS_CRITICAL_FQDNS=""
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "WARNING: LOCALDNS_CRITICAL_FQDNS is not set"
            The output should include "Skipping aks-localdns-hosts-setup"
        End

        It 'should work with any cloud as long as FQDNs are provided'
            TARGET_CLOUD="USNatCloud"
            LOCALDNS_CRITICAL_FQDNS="mcr.microsoft.com,login.microsoftonline.com"
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "aks-localdns-hosts-setup timer enabled successfully."
        End

        It 'should succeed and enable timer when LOCALDNS_CRITICAL_FQDNS is set'
            When call enableAKSLocalDNSHostsSetup
            The status should be success
            The output should include "Enabling aks-localdns-hosts-setup timer..."
            The output should include "aks-localdns-hosts-setup timer enabled successfully."
        End
    End
End
