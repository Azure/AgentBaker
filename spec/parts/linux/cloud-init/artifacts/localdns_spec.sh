#!/bin/bash

lsb_release() {
    echo "mock lsb_release"
}

Describe 'localdns.sh'
    Describe 'verify_localdns_files'
        setup() {
            LOCALDNS_SCRIPT_PATH="/opt/azure/containers/localdns"
            LOCALDNS_CORE_FILE="${LOCALDNS_SCRIPT_PATH}/localdns.corefile"
            mkdir -p "$LOCALDNS_SCRIPT_PATH"
            echo "forward . 168.63.129.16" >> "$LOCALDNS_CORE_FILE"

            LOCALDNS_SLICE_PATH="/etc/systemd/system"
            LOCALDNS_SLICE_FILE="${LOCALDNS_SLICE_PATH}/localdns.slice"
            mkdir -p "$LOCALDNS_SLICE_PATH"
            echo "localdns slice file" >> "$LOCALDNS_SLICE_FILE"

            COREDNS_BINARY_PATH="/opt/azure/containers/localdns/binary/coredns"
            mkdir -p "$(dirname "$COREDNS_BINARY_PATH")"
    cat <<EOF > "$COREDNS_BINARY_PATH"
#!/bin/bash
if [[ "\$1" == "--version" ]]; then
    echo "mock v1.12.0  linux/amd64"
    exit 0
fi
EOF
            chmod +x "$COREDNS_BINARY_PATH"

            RESOLV_CONF="/run/systemd/resolve/resolv.conf"
            mkdir -p "$(dirname "$RESOLV_CONF")"
cat <<EOF > "$RESOLV_CONF"
nameserver 10.0.0.1
nameserver 10.0.0.2
EOF

            Include "./parts/linux/cloud-init/artifacts/localdns.sh" 
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

#------------------------- replace_azurednsip_in_corefile -----------------------------------------------------
        It 'should replace 168.63.129.16 with UpstreamDNSIP if it is not same as AzureDNSIP'
            When run replace_azurednsip_in_corefile
            The status should be success
            The file "${LOCALDNS_CORE_FILE}" should exist
            The contents of file "${LOCALDNS_CORE_FILE}" should include "forward . 10.0.0.1 10.0.0.2"
        End

        It 'should fail if /run/systemd/resolve/resolv.conf not found'
            rm -f "$RESOLV_CONF"
            When run replace_azurednsip_in_corefile
            The status should be failure
            The stdout should include "/run/systemd/resolve/resolv.conf not found."
        End

        It 'should fail if UpstreamDNSIP is not found in /run/systemd/resolve/resolv.conf'
cat <<EOF > "$RESOLV_CONF"
invalid
EOF
            When run replace_azurednsip_in_corefile
            The status should be failure
            The file "${LOCALDNS_CORE_FILE}" should exist
            The stdout should include "No Upstream VNET DNS servers found in /run/systemd/resolve/resolv.conf."
            The contents of file "${LOCALDNS_CORE_FILE}" should include "forward . 168.63.129.16"
        End

        It 'should not replace 168.63.129.16 with UpstreamDNSIP if it is ""'
cat <<EOF > "$RESOLV_CONF"
nameserver ""
EOF
            When run replace_azurednsip_in_corefile
            The status should be failure
            The file "${LOCALDNS_CORE_FILE}" should exist
            The stdout should include "No Upstream VNET DNS servers found in /run/systemd/resolve/resolv.conf."
            The contents of file "${LOCALDNS_CORE_FILE}" should include "forward . 168.63.129.16"
        End

        It 'should not replace 168.63.129.16 with UpstreamDNSIP if it is blank'
cat <<EOF > "$RESOLV_CONF"
nameserver  
EOF
            When run replace_azurednsip_in_corefile
            The status should be failure
            The file "${LOCALDNS_CORE_FILE}" should exist
            The stdout should include "No Upstream VNET DNS servers found in /run/systemd/resolve/resolv.conf."
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

        # It 'should fail if unable to replace AzureDNSIP in corefile'
        #     sudo chmod 444 "${LOCALDNS_CORE_FILE}"
        #     When run replace_azurednsip_in_corefile
        #     The status should be failure
        #     The file "${LOCALDNS_CORE_FILE}" should exist
        #     The contents of file "${LOCALDNS_CORE_FILE}" should include "forward . 168.63.129.16"
        #     The stdout should include "Updating corefile failed."
        # End

        It 'should return failure if AZURE_DNS_IP is unset'
            unset AZURE_DNS_IP
            When run replace_azurednsip_in_corefile
            The status should be failure
            The stdout should include "AZURE_DNS_IP is not set or is empty."
        End
    End

    Describe 'build_localdns_iptable_rules_and_verify_network_file'
        setup() {
            DEFAULT_ROUTE_INTERFACE="eth0"
            NETWORK_FILE_DIR="/etc/systemd/network"
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

            Include "./parts/linux/cloud-init/artifacts/localdns.sh"
        }

        cleanup() {
            rm -rf "$NETWORK_FILE_DIR"
            rm -rf "$NETWORK_DROPIN_DIR"
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'

#------------------------- build_localdns_iptable_rules ------------------------------------------------------
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

#------------------------- verify_default_route_interface ------------------------------------------------------
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

#------------------------- verify_network_file -----------------------------------------------------------------
        It 'should succeed if networkfile is found'
            When call verify_network_file
            The status should be success
            The variable NETWORK_FILE should equal "/etc/systemd/network/eth0.network"
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

#------------------------- verify_network_dropin_dir -----------------------------------------------------------------
        It 'should succeed if networkdir is found'
            When call verify_network_dropin_dir
            The status should be success
            The variable NETWORK_DROPIN_DIR should equal "/etc/systemd/network/eth0.network.d"
        End

        It 'should fail if no networkdir is found'
            rm -rf "$NETWORK_DROPIN_DIR"
            When call verify_network_dropin_dir
            The status should be failure
            The stdout should include "Network drop-in directory does not exist."
        End
    End

    Describe 'wait_for_localdns_ready'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns.sh"
        }

        BeforeEach 'setup'

        It 'wait_for_localdns_ready1'
            CURL_COMMAND="echo OK"
            MAX_ATTEMPTS=100
            TIMEOUT=5
            When call wait_for_localdns_ready $MAX_ATTEMPTS $TIMEOUT "$CURL_COMMAND"
            The status should be success
        End

        It 'should return failure, after timeout'
            CURL_COMMAND="echo NOTOK"
            MAX_ATTEMPTS=1000
            TIMEOUT=5
            When call wait_for_localdns_ready $MAX_ATTEMPTS $TIMEOUT "$CURL_COMMAND"
            The status should be failure
            The output should include "Localdns failed to come online after 5 seconds (timeout)."
        End

        It 'should return failure, after max attempts'
            CURL_COMMAND="echo NOTOK"
            MAX_ATTEMPTS=10
            TIMEOUT=50
            When call wait_for_localdns_ready $MAX_ATTEMPTS $TIMEOUT "$CURL_COMMAND"
            The status should be failure
            The output should include "Localdns failed to come online after 10 attempts."
        End
    End

    Describe "cleanup_localdns_configs"
        cleanup_localdns_configs() {
            # Disable error handling so that we don't get into a recursive loop.
            set +e

            # Remove iptables rules to stop forwarding DNS traffic.
            for RULE in "${IPTABLES_RULES[@]}"; do
                if eval "${IPTABLES}" -C "${RULE}" 2>/dev/null; then
                    eval "${IPTABLES}" -D "${RULE}"
                    if [ $? -eq 0 ]; then
                        echo "Successfully removed iptables rule: ${RULE}."
                    else
                        echo "Failed to remove iptables rule: ${RULE}."
                        return 1
                    fi
                fi
            done

            # Revert the changes made to the DNS configuration if present.
            if [ -f "${NETWORK_DROPIN_FILE}" ]; then
                echo "Reverting DNS configuration by removing ${NETWORK_DROPIN_FILE}."
                if /bin/rm -f "${NETWORK_DROPIN_FILE}"; then
                    networkctl reload || {
                        echo "Failed to reload network after removing the DNS configuration."
                        return 1
                    }
                else
                    echo "Failed to remove ${NETWORK_DROPIN_FILE}."
                    return 1
                fi
            fi

            # Trigger localdns shutdown, if running.
            if [[ -n "${COREDNS_PID}" ]] && [[ "${COREDNS_PID}" =~ ^[0-9]+$ ]]; then
                if mock_ps; then
                    if [[ "${LOCALDNS_SHUTDOWN_DELAY}" -gt 0 ]]; then
                        echo "Sleeping ${LOCALDNS_SHUTDOWN_DELAY} seconds to allow connections to terminate."
                        sleep "${LOCALDNS_SHUTDOWN_DELAY}"
                    fi
                    echo "Sending SIGINT to localdns and waiting for it to terminate."

                    mock_kill "${COREDNS_PID}"
                    kill_status=$?
                    if [ $kill_status -eq 0 ]; then
                        echo "Successfully sent SIGINT to localdns."
                    else
                        echo "Failed to send SIGINT to localdns. Exit status: $kill_status."
                        return 1
                    fi

                    if mock_wait "${COREDNS_PID}"; then
                        echo "Localdns terminated successfully."
                    else
                        echo "Localdns failed to terminate properly."
                        return 1
                    fi
                fi
            fi

            # Delete the dummy interface if present.
            if mock_ip_link_show >/dev/null 2>&1; then
                echo "Removing localdns dummy interface."
                mock_ip_link_del $LOCALDNS_INTERFACE
                if [ $? -eq 0 ]; then
                    echo "Successfully removed localdns dummy interface."
                else
                    echo "Failed to remove localdns dummy interface."
                    return 1
                fi
            fi

            # Indicate successful cleanup.
            echo "Successfully cleanup localdns related configurations."
            return 0
        }

        mock_iptables() {
            echo "iptables -C $1"
            return 0
        }

        mock_kill() {
            local pid=$1
            if [[ "$pid" == "12345" ]]; then
                return 0
            else
                return 1
            fi
        }

        mock_ps() {
            return 0
        }

        mock_wait() {
            local pid=$1
            if [[ "$pid" == "12345" ]]; then
                return 0
            else
                return 1
            fi
        }

        mock_ip_link_show() {
            return 0
        }

        mock_ip_link_del() {
            return 0
        }

        mock_networkdropin_command() {
            echo '{"NetworkFile": "/etc/systemd/network/eth0.network.d/70-localdns.conf"}'
        }

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
            LOCALDNS_INTERFACE="name localdns"
            When call cleanup_localdns_configs
            The stdout should include "Successfully removed iptables rule: OUTPUT -p tcp -d 169.254.10.10 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: OUTPUT -p udp -d 169.254.10.10 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: OUTPUT -p tcp -d 169.254.10.11 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: OUTPUT -p udp -d 169.254.10.11 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: PREROUTING -p tcp -d 169.254.10.10 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: PREROUTING -p udp -d 169.254.10.10 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: PREROUTING -p tcp -d 169.254.10.11 --dport 53 -j NOTRACK."
            The stdout should include "Successfully removed iptables rule: PREROUTING -p udp -d 169.254.10.11 --dport 53 -j NOTRACK."
            The stdout should include "Removing localdns dummy interface."
            The stdout should include "Successfully cleanup localdns related configurations."
        End

        It "should not fail if DNS configuration file doesn't exist"
            NETWORK_DROPIN_FILE=""
            When call cleanup_localdns_configs
            The stdout should include "Successfully cleanup localdns related configurations."
        End

        It "should successfully cleanup"
            COREDNS_PID="12345"
            When call cleanup_localdns_configs
            The status should be success
            The stdout should include "Sending SIGINT to localdns and waiting for it to terminate."
            The stdout should include "Successfully sent SIGINT to localdns."
            The stdout should include "Localdns terminated successfully."
            The stdout should include "Successfully cleanup localdns related configurations."
        End

        It "should fail cleanup"
            COREDNS_PID="54321"
            When call cleanup_localdns_configs
            The status should be failure
            The stdout should include "Sending SIGINT to localdns and waiting for it to terminate."
            The stdout should include "Failed to send SIGINT to localdns. Exit status: 1."
        End

        It "should remove dummy interface if exists"
            When call cleanup_localdns_configs
            The stdout should include "Successfully removed localdns dummy interface"
        End

        It "should handle errors in iptables deletion"
            LOCALDNS_INTERFACE="name localdns"
            When call cleanup_localdns_configs
            The stdout should include "Removing localdns dummy interface."
            The stdout should include "Successfully removed localdns dummy interface."
        End
    End
End