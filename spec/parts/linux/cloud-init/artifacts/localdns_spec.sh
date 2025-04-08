#!/bin/bash

lsb_release() {
    echo "mock lsb_release"
}

Describe 'localdns.sh'

# This section tests - verify_localdns_corefile, verify_localdns_slicefile, verify_localdns_binary, replace_azurednsip_in_corefile.
# These functions are defined in parts/linux/cloud-init/artifacts/localdns.sh file.
#------------------------------------------------------------------------------------------------------------------------------------
    Describe 'verify_localdns_files'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns.sh" 

            TEST_DIR="/tmp/localdnstest"
            LOCALDNS_SCRIPT_PATH="${TEST_DIR}/opt/azure/containers/localdns"
            LOCALDNS_CORE_FILE="${LOCALDNS_SCRIPT_PATH}/localdns.corefile"
            mkdir -p "$LOCALDNS_SCRIPT_PATH"
            echo "forward . 168.63.129.16" >> "$LOCALDNS_CORE_FILE"

            LOCALDNS_SLICE_PATH="${TEST_DIR}/etc/systemd/system"
            LOCALDNS_SLICE_FILE="${LOCALDNS_SLICE_PATH}/localdns.slice"
            mkdir -p "$LOCALDNS_SLICE_PATH"
            echo "localdns slice file" >> "$LOCALDNS_SLICE_FILE"

            COREDNS_BINARY_PATH="${TEST_DIR}/opt/azure/containers/localdns/binary/coredns"
            mkdir -p "$(dirname "$COREDNS_BINARY_PATH")"
    cat <<EOF > "$COREDNS_BINARY_PATH"
#!/bin/bash
if [[ "\$1" == "--version" ]]; then
    echo "mock v1.12.0  linux/amd64"
    exit 0
fi
EOF
            chmod +x "$COREDNS_BINARY_PATH"
            RESOLV_CONF="${TEST_DIR}/run/systemd/resolve/resolv.conf"
            mkdir -p "$(dirname "$RESOLV_CONF")"
cat <<EOF > "$RESOLV_CONF"
nameserver 10.0.0.1
nameserver 10.0.0.2
EOF

        }
        cleanup() {
            rm -rf "$LOCALDNS_SCRIPT_PATH"
            rm -rf "$LOCALDNS_SLICE_PATH"
            rm -rf "$COREDNS_BINARY_PATH"
            rm -rf "$RESOLV_CONF"
        }
        BeforeEach 'setup'
        AfterEach 'cleanup'
        #------------------------ verify_localdns_corefile -------------------------------------------------
        It 'should return success if localdns corefile exists and is not empty'
            When run verify_localdns_corefile
            The status should be success
        End

        It 'should return failure if localdns corefile does not exist'
            rm -r "$LOCALDNS_CORE_FILE"
            When run verify_localdns_corefile
            The status should be failure
            The stdout should include "Localdns corefile either does not exist or is empty at $LOCALDNS_CORE_FILE."
        End

        It 'should return failure if localdns corefile is empty'
            > "$LOCALDNS_CORE_FILE"
            When run verify_localdns_corefile
            The status should be failure
            The stdout should include "Localdns corefile either does not exist or is empty at $LOCALDNS_CORE_FILE."
        End

        It 'should return failure if LOCALDNS_CORE_FILE is unset'
            unset LOCALDNS_CORE_FILE
            When run verify_localdns_corefile
            The status should be failure
            The stdout should include "LOCALDNS_CORE_FILE is not set or is empty."
        End

        #------------------------ verify_localdns_slicefile ------------------------------------------------
        It 'should return success if localdns slicefile exists and is not empty'
            When run verify_localdns_slicefile
            The status should be success
        End

        It 'should return failure if localdns slicefile does not exist'
            rm -f "$LOCALDNS_SLICE_FILE"
            When run verify_localdns_slicefile
            The status should be failure
            The stdout should include "Localdns slice file does not exist at $LOCALDNS_SLICE_FILE."
        End

        It 'should return failure if localdns slicefile is empty'
            > "$LOCALDNS_SLICE_FILE"
            When run verify_localdns_slicefile
            The status should be failure
            The stdout should include "Localdns slice file does not exist at $LOCALDNS_SLICE_FILE."
        End

        It 'should return failure if LOCALDNS_SLICE_FILE is unset'
            unset LOCALDNS_SLICE_FILE
            When run verify_localdns_slicefile
            The status should be failure
            The stdout should include "LOCALDNS_SLICE_FILE is not set or is empty."
        End

        #------------------------- verify_localdns_binary -----------------------------------------------------
        It 'should return success if coredns binary exists and is executable'
            When run verify_localdns_binary
            The status should be success
        End

        It 'should return failure if coredns binary does not exist'
            rm -f "$COREDNS_BINARY_PATH"
            When run verify_localdns_binary
            The status should be failure
            The stdout should include "Coredns binary either doesn't exist or isn't executable at $COREDNS_BINARY_PATH."
        End

        It 'should return failure if coredns binary is not executable'
            chmod -x "$COREDNS_BINARY_PATH"
            When run verify_localdns_binary
            The status should be failure
            The stdout should include "Coredns binary either doesn't exist or isn't executable at $COREDNS_BINARY_PATH."
        End

        It 'should return failure if coredns --version command fails'
            echo '#!/bin/bash' > "$COREDNS_BINARY_PATH"
            echo 'exit 1' >> "$COREDNS_BINARY_PATH"
            chmod +x "$COREDNS_BINARY_PATH"
            When run verify_localdns_binary
            The status should be failure
            The stdout should include "Failed to execute '--version'."
        End

        #------------------------- replace_azurednsip_in_corefile -----------------------------------------------
        It 'should replace 168.63.129.16 with UpstreamDNSIP if it is not same as AzureDNSIP'
            When run replace_azurednsip_in_corefile
            The status should be success
            The file "${LOCALDNS_CORE_FILE}" should exist
            The contents of file "${LOCALDNS_CORE_FILE}" should include "forward . 10.0.0.1 10.0.0.2"
        End

        It 'should fail if resolv.conf not found'
            rm -f "$RESOLV_CONF"
            When run replace_azurednsip_in_corefile
            The status should be failure
            The stdout should include ""$RESOLV_CONF" not found."
        End

        It 'should fail if UpstreamDNSIP is not found in resolv.conf'
cat <<EOF > "$RESOLV_CONF"
invalid
EOF
            When run replace_azurednsip_in_corefile
            The status should be failure
            The file "${LOCALDNS_CORE_FILE}" should exist
            The stdout should include "No Upstream VNET DNS servers found in "$RESOLV_CONF"."
            The contents of file "${LOCALDNS_CORE_FILE}" should include "forward . 168.63.129.16"
        End

        It 'should not replace 168.63.129.16 with UpstreamDNSIP if it is ""'
cat <<EOF > "$RESOLV_CONF"
nameserver ""
EOF
            When run replace_azurednsip_in_corefile
            The status should be failure
            The file "${LOCALDNS_CORE_FILE}" should exist
            The stdout should include "No Upstream VNET DNS servers found in "$RESOLV_CONF"."
            The contents of file "${LOCALDNS_CORE_FILE}" should include "forward . 168.63.129.16"
        End

        It 'should not replace 168.63.129.16 with UpstreamDNSIP if it is blank'
cat <<EOF > "$RESOLV_CONF"
nameserver  
EOF
            When run replace_azurednsip_in_corefile
            The status should be failure
            The file "${LOCALDNS_CORE_FILE}" should exist
            The stdout should include "No Upstream VNET DNS servers found in "$RESOLV_CONF"."
            The contents of file "${LOCALDNS_CORE_FILE}" should include "forward . 168.63.129.16"
        End

        It 'should not replace 168.63.129.16 with UpstreamDNSIP if it is same as AzureDNSIP'
cat <<EOF > "$RESOLV_CONF"
nameserver 168.63.129.16
EOF
            When run replace_azurednsip_in_corefile
            The status should be success
            The file "${LOCALDNS_CORE_FILE}" should exist
            The contents of file "${LOCALDNS_CORE_FILE}" should include "forward . 168.63.129.16"
        End

        It 'should return failure if AZURE_DNS_IP is unset'
            unset AZURE_DNS_IP
            When run replace_azurednsip_in_corefile
            The status should be failure
            The stdout should include "AZURE_DNS_IP is not set or is empty."
        End
    End


# This section tests - build_localdns_iptable_rules, verify_default_route_interface, verify_network_file, verify_network_dropin_dir.
# These functions are defined in parts/linux/cloud-init/artifacts/localdns.sh file.
#------------------------------------------------------------------------------------------------------------------------------------
    Describe 'build_localdns_iptable_rules_and_verify_network_file'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns.sh"
            TEST_DIR="/tmp/localdnstest"
            DEFAULT_ROUTE_INTERFACE="eth0"
            NETWORK_FILE_DIR="${TEST_DIR}/etc/systemd/network"
            NETWORK_FILE="${NETWORK_FILE_DIR}/eth0.network"
            mkdir -p "$NETWORK_FILE_DIR"
            echo "[Network]" >> "$NETWORK_FILE"

            NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
            mkdir -p "${NETWORK_DROPIN_DIR}"

            NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-localdns.conf"
            cat > "${NETWORK_DROPIN_FILE}" <<EOF
# Set DNS server to localdns cluster listernerIP.
[Network]
DNS=${LOCALDNS_NODE_LISTENER_IP}

# Disable DNS provided by DHCP to ensure local DNS is used.
[DHCP]
UseDNS=false
EOF
        }
        cleanup() {
            rm -rf "$NETWORK_FILE_DIR"
            rm -rf "$NETWORK_DROPIN_DIR"
        }
        BeforeEach 'setup'
        AfterEach 'cleanup'
        #------------------------- build_localdns_iptable_rules --------------------------------------------------
        It 'should build iptables rules correctly for OUTPUT and PREROUTING'
            When call build_localdns_iptable_rules
            The status should be success
            expected_rules=(
                "OUTPUT -p tcp -d 169.254.10.10 --dport 53 -j NOTRACK"
                "OUTPUT -p udp -d 169.254.10.10 --dport 53 -j NOTRACK"
                "OUTPUT -p tcp -d 169.254.10.11 --dport 53 -j NOTRACK"
                "OUTPUT -p udp -d 169.254.10.11 --dport 53 -j NOTRACK"
                "PREROUTING -p tcp -d 169.254.10.10 --dport 53 -j NOTRACK"
                "PREROUTING -p udp -d 169.254.10.10 --dport 53 -j NOTRACK"
                "PREROUTING -p tcp -d 169.254.10.11 --dport 53 -j NOTRACK"
                "PREROUTING -p udp -d 169.254.10.11 --dport 53 -j NOTRACK"
            )
            all_rules_found=true
            for expected_rule in "${expected_rules[@]}"; do
                found=false
                for actual_rule in "${IPTABLES_RULES[@]}"; do
                    if [[ "$actual_rule" == "$expected_rule" ]]; then
                        found=true
                        break
                    fi
                done

                if [[ "$found" == false ]]; then
                    all_rules_found=false
                    echo "Missing rule: $actual_rule"
                    exit 1
                fi
            done
            The value "${#IPTABLES_RULES[@]}" should equal "${#expected_rules[@]}"
        End

        #------------------------- verify_default_route_interface --------------------------------------------------
        It 'should succeed if default route interface is found'
            When call verify_default_route_interface
            The status should be success
            The variable DEFAULT_ROUTE_INTERFACE should equal "eth0"
        End

        It 'should fail if no default route interface is found'
            DEFAULT_ROUTE_INTERFACE=""
            When call verify_default_route_interface
            The status should be failure
            The stdout should include "Unable to determine the default route interface"
        End

        It 'should fail if default route interface variable is unset'
            unset DEFAULT_ROUTE_INTERFACE
            When call verify_default_route_interface
            The status should be failure
            The stdout should include "Unable to determine the default route interface"
        End

        #------------------------- verify_network_file --------------------------------------------------------------
        It 'should succeed if networkfile is found'
            When call verify_network_file
            The status should be success
            The variable NETWORK_FILE should equal "$NETWORK_FILE"
        End

        It 'should fail if no networkfile is found'
            rm -rf "$NETWORK_FILE"
            When call verify_network_file
            The status should be failure
            The stdout should include "Unable to determine network file for interface"
        End

        It 'should fail if NETWORK_FILE is unset'
            unset NETWORK_FILE
            When call verify_network_file
            The status should be failure
            The stdout should include "Unable to determine network file for interface"
        End

        #------------------------- verify_network_dropin_dir -------------------------------------------------------
        It 'should succeed if networkdir is found'
            When call verify_network_dropin_dir
            The status should be success
            The variable NETWORK_DROPIN_DIR should equal "$NETWORK_DROPIN_DIR"
        End

        It 'should fail if no networkdir is found'
            rm -rf "$NETWORK_DROPIN_DIR"
            When call verify_network_dropin_dir
            The status should be failure
            The stdout should include "Network drop-in directory does not exist."
        End
    End


# This section tests - start_localdns
# This function is defined in parts/linux/cloud-init/artifacts/localdns.sh file.
#------------------------------------------------------------------------------------------------------------------------------------
    Describe 'start_localdns'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns.sh"
            LOCALDNS_PID_FILE="/tmp/localdns.pid"
        }
        cleanup() {
            rm -f "${LOCALDNS_PID_FILE}"
            rm -f ./mock-coredns.sh
        }
        BeforeEach 'setup'
        AfterEach 'cleanup'
        #------------------------- start_localdns ------------------------------------------------------------------
        It 'should start localdns and create the PID file'
            MOCK_SCRIPT="./mock-coredns.sh"
            cat > "$MOCK_SCRIPT" <<EOF
#!/bin/bash
# Simulate a long-running process that creates the PID file.
echo \$\$ > "${LOCALDNS_PID_FILE}"
sleep 60
EOF
            chmod +x "$MOCK_SCRIPT"
            COREDNS_COMMAND="$MOCK_SCRIPT"
            When call start_localdns
            The status should be success
            The file "${LOCALDNS_PID_FILE}" should exist
            The output should include "Localdns PID is"
        End

        It 'should fail if PID file is not created in time'
            MOCK_SCRIPT="./mock-coredns.sh"
        cat > "$MOCK_SCRIPT" <<EOF
#!/bin/bash
# Simulate a long-running process that doesn't create the PID file.
sleep 60
EOF
            chmod +x "$MOCK_SCRIPT"
            COREDNS_COMMAND="$MOCK_SCRIPT"
            START_LOCALDNS_TIMEOUT=2
            When call start_localdns
            The status should be failure
            The output should include "Timed out waiting for CoreDNS to create PID file"
        End
    End


# This section tests - wait_for_localdns_ready
# These functions are defined in parts/linux/cloud-init/artifacts/localdns.sh file.
#------------------------------------------------------------------------------------------------------------------------------------
    Describe 'wait_for_localdns_ready'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns.sh"
        }
        BeforeEach 'setup'
    #------------------------- wait_for_localdns_ready -----------------------------------------------------------
        It 'should return success if localdns is ready'
            CURL_COMMAND="echo OK"
            MAX_ATTEMPTS=100
            TIMEOUT=5
            When call wait_for_localdns_ready $MAX_ATTEMPTS $TIMEOUT
            The status should be success
            The output should include "Waiting for localdns to start and be able to serve traffic."
            The output should include "Localdns is online and ready to serve traffic."
        End

        It 'should return failure if localdns is not ready, after timeout'
            CURL_COMMAND="echo NOTOK"
            MAX_ATTEMPTS=1000
            TIMEOUT=2
            When call wait_for_localdns_ready $MAX_ATTEMPTS $TIMEOUT
            The status should be failure
            The output should include "Localdns failed to come online after ${TIMEOUT} seconds (timeout)."
        End

        It 'should return failure if localdns is not ready, after max attempts'
            CURL_COMMAND="echo NOTOK"
            MAX_ATTEMPTS=2
            TIMEOUT=50
            When call wait_for_localdns_ready $MAX_ATTEMPTS $TIMEOUT
            The status should be failure
            The output should include "Localdns failed to come online after ${MAX_ATTEMPTS} attempts."
        End
    End


# This section tests - add_iptable_rules_to_skip_conntrack_from_pods
# This function is defined in parts/linux/cloud-init/artifacts/localdns.sh file.
#------------------------------------------------------------------------------------------------------------------------------------
    Describe 'add_iptable_rules_to_skip_conntrack_from_pods'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns.sh"
            LOCALDNS_NODE_LISTENER_IP="10.0.0.1"
            LOCALDNS_CLUSTER_LISTENER_IP="10.0.0.2"
            IPTABLES_RULES=("raw -t raw -p udp --dport 53 -j NOTRACK" "raw -t raw -p tcp --dport 53 -j NOTRACK")
            IPTABLES="echo iptables"
        }
        BeforeEach 'setup'
        #------------------------- add_iptable_rules_to_skip_conntrack_from_pods -------------------------------------
        It 'should create dummy localdns interface and set IPs, and add iptables rules'
            ip() {
                case "$1 $2" in
                    "link show")
                        return 1
                        ;;
                    "link add")
                        echo "Adding interface: $*"
                        ;;
                    "link set")
                        echo "Setting interface up: $*"
                        ;;
                    "addr add")
                        echo "Assigning IP: $*"
                        ;;
                    *)
                        echo "Unknown ip command: $*"
                        ;;
                esac
            }
            Path prepend "$(pwd)"
            When call add_iptable_rules_to_skip_conntrack_from_pods
            The output should include "Adding iptables rules to skip conntrack for queries to localdns."
            The output should include "iptables -A raw -t raw -p udp --dport 53 -j NOTRACK"
            The output should include "iptables -A raw -t raw -p tcp --dport 53 -j NOTRACK"
        End

        It 'should delete existing localdns interface'
            ip() {
                case "$1 $2" in
                    "link show")
                        return 0
                        ;;
                    "link delete")
                        echo "Deleting interface: $*"
                        ;;
                    *)
                        return 0
                        ;;
                esac
            }

            Path prepend "$(pwd)"
            When call add_iptable_rules_to_skip_conntrack_from_pods
            The output should include "Interface localdns already exists, deleting it."
            The output should include "Deleting interface: link delete localdns"
        End
    End


# This section tests - disable_dhcp_use_clusterlistener
# These functions are defined in parts/linux/cloud-init/artifacts/localdns.sh file.
#------------------------------------------------------------------------------------------------------------------------------------
    Describe 'disable_dhcp_use_clusterlistener'
        setup() {
            NETWORK_DROPIN_DIR="/tmp/test-systemd-network"
            NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/10-localdns.conf"
            LOCALDNS_NODE_LISTENER_IP="169.254.10.10"

            Include "./parts/linux/cloud-init/artifacts/localdns.sh"
        }
        cleanup() {
            rm -rf "$NETWORK_DROPIN_DIR"
        }
        BeforeEach 'setup'
        AfterEach 'cleanup'
        #------------------------- disable_dhcp_use_clusterlistener -------------------------------------------------
            It 'should update network configuration and reload networkctl'
                NETWORKCTL_RELOAD_CMD="true"
                When call disable_dhcp_use_clusterlistener
                The status should be success
                The file "${NETWORK_DROPIN_FILE}" should exist
                The contents of file "${NETWORK_DROPIN_FILE}" should include "UseDNS=false"
                The contents of file "${NETWORK_DROPIN_FILE}" should include "DNS=169.254.10.10"
            End

            It 'should fail if networkctl reload fails'
                NETWORKCTL_RELOAD_CMD="false"
                When call disable_dhcp_use_clusterlistener
                The status should be failure
                The output should include "Failed to reload networkctl."
            End
        End
    End


# This section tests - cleanup_localdns_configs
# These functions is defined in parts/linux/cloud-init/artifacts/localdns.sh file.
#------------------------------------------------------------------------------------------------------------------------------------
    Describe 'cleanup_localdns_configs'
        setup() {
            IPTABLES_RULES=("INPUT -p udp --dport 53 -j ACCEPT" "OUTPUT -p udp --sport 53 -j ACCEPT")
            NETWORK_DROPIN_FILE="/tmp/test-network-dropin.conf"
            COREDNS_PID="12345"
            mock_iptables() {
                echo "iptables -C $1"
                return 0
            }
            Include "./parts/linux/cloud-init/artifacts/localdns.sh"
            LOCALDNS_SHUTDOWN_DELAY=1
        }
        cleanup() {
            rm -rf "/tmp/test-network-dropin.conf"
        }
        BeforeEach 'setup'
        AfterEach 'cleanup'
        #------------------------- cleanup_localdns_configs ------------------------------------------------------------
        It "should clean up iptables rules"
            IPTABLES_RULES=(
            "OUTPUT -p tcp -d 169.254.10.10 --dport 53 -j NOTRACK"
            "OUTPUT -p udp -d 169.254.10.10 --dport 53 -j NOTRACK"
            "OUTPUT -p tcp -d 169.254.10.11 --dport 53 -j NOTRACK"
            "OUTPUT -p udp -d 169.254.10.11 --dport 53 -j NOTRACK"
            "PREROUTING -p tcp -d 169.254.10.10 --dport 53 -j NOTRACK"
            "PREROUTING -p udp -d 169.254.10.10 --dport 53 -j NOTRACK"
            "PREROUTING -p tcp -d 169.254.10.11 --dport 53 -j NOTRACK"
            "PREROUTING -p udp -d 169.254.10.11 --dport 53 -j NOTRACK"
            )
            IPTABLES="mock_iptables"
            When call cleanup_localdns_configs
            The stdout should include "Successfully removed iptables rule: OUTPUT -p tcp -d 169.254.10.10 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: OUTPUT -p udp -d 169.254.10.10 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: OUTPUT -p tcp -d 169.254.10.11 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: OUTPUT -p udp -d 169.254.10.11 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: PREROUTING -p tcp -d 169.254.10.10 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: PREROUTING -p udp -d 169.254.10.10 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: PREROUTING -p tcp -d 169.254.10.11 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: PREROUTING -p udp -d 169.254.10.11 --dport 53 -j NOTRACK."
            The stdout should include "Successfully cleanup localdns related configurations."
        End

        It 'should return failure if iptables rule removal fails'
            IPTABLES_RULES=("INPUT -p udp --dport 53 -j ACCEPT")
            IPTABLES="mock_failing_delete_iptables"
            mock_failing_delete_iptables() {
                if [[ "$1" == "-C" ]]; then return 0; fi
                if [[ "$1" == "-D" ]]; then return 1; fi
            }
            When call cleanup_localdns_configs
            The status should be failure
            The output should include "Failed to remove iptables rule"
        End

        It 'should return success if removing network drop-in file succeeds'
            NETWORKCTL_RELOAD_CMD="true"
            NETWORK_DROPIN_FILE="/tmp/test-network-dropin.conf"
            touch "$NETWORK_DROPIN_FILE"
            IPTABLES=""
            When call cleanup_localdns_configs
            The status should be success
            The output should include "Reverting DNS configuration by removing"
            The output should include "Successfully cleanup localdns related configurations."
            The file "${NETWORK_DROPIN_FILE}" should not exist
        End

        It 'should return failure if network reload fails'
            NETWORK_DROPIN_FILE="/tmp/test-network-dropin.conf"
            touch "$NETWORK_DROPIN_FILE"
            NETWORKCTL_RELOAD_CMD="false"
            IPTABLES=""
            When call cleanup_localdns_configs
            The status should be failure
            The output should include "Reverting DNS configuration by removing"
            The output should include "Failed to reload network after removing the DNS configuration."
        End

        It 'should return failure if SIGINT fails to send to CoreDNS'
            COREDNS_PID=$$
            kill() { return 1; }  # override kill
            ps() { return 0; }    # simulate process exists
            IPTABLES=""
            When call cleanup_localdns_configs
            The status should be failure
            The output should include "Sleeping ${LOCALDNS_SHUTDOWN_DELAY} seconds to allow connections to terminate."
            The output should include "Failed to send SIGINT to localdns"
        End

        It 'should return failure if localdns process does not terminate cleanly'
            COREDNS_PID=$$
            ps() { return 0; }
            kill() { return 0; }
            wait() { return 1; }
            IPTABLES=""
            When call cleanup_localdns_configs
            The status should be failure
            The output should include "Successfully sent SIGINT to localdns."
            The output should include "Localdns failed to terminate properly."
        End

        It 'should return success if localdns process terminates cleanly'
            COREDNS_PID=$$
            ps() { return 0; }
            kill() { return 0; }
            wait() { return 0; }
            IPTABLES=""
            When call cleanup_localdns_configs
            The status should be success
            The output should include "Successfully sent SIGINT to localdns."
            The output should include "Localdns terminated successfully."
            The output should include "Successfully cleanup localdns related configurations."
        End

        It 'should return failure if dummy interface cannot be removed'
            ip() {
                if [[ "$1" == "link" && "$2" == "show" ]]; then return 0; fi
                if [[ "$1" == "link" && "$2" == "del" ]]; then return 1; fi
            }
            IPTABLES=""
            When call cleanup_localdns_configs
            The status should be failure
            The output should include "Failed to remove localdns dummy interface."
        End

        It 'should return success if dummy interface was removed'
            ip() {
                if [[ "$1" == "link" && "$2" == "show" ]]; then return 0; fi
                if [[ "$1" == "link" && "$2" == "del" ]]; then return 0; fi
            }
            IPTABLES=""
            When call cleanup_localdns_configs
            The status should be success
            The output should include "Successfully removed localdns dummy interface."
            The output should include "Successfully cleanup localdns related configurations."
        End

        It 'should return success if none of the objects are present'
            IPTABLES=""
            When call cleanup_localdns_configs
            The status should be success
            The output should include "Successfully cleanup localdns related configurations."
        End


# This section tests - start_localdns_watchdog
# These functions is also defined in parts/linux/cloud-init/artifacts/localdns.sh file.
#------------------------------------------------------------------------------------------------------------------------------------
    Describe 'start_localdns_watchdog'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns.sh"
        }
        BeforeEach 'setup'
        #------------------------- start_localdns_watchdog ------------------------------------------------------------
        It 'should not do anything if NOTIFY_SOCKET and WATCHDOG_USEC are empty'
            NOTIFY_SOCKET=""
            WATCHDOG_USEC=""
            COREDNS_PID="12345"
            wait() { return 0; }
            When call start_localdns_watchdog
            The status should be success
        End
    End
End