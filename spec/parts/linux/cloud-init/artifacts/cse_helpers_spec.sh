#!/bin/bash

lsb_release() {
    echo "mock lsb_release"
}

Describe 'localdns.sh'

# Verify the required files exists.
# --------------------------------------------------------------------------------------------------------------------
    Describe 'verify_localdns_files'
        setup() {
            AZURE_DNS_IP="168.63.129.16"

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

            verify_localdns_files() {
                # This file contains generated corefile used by localdns systemd unit.
                if [ ! -f "${LOCALDNS_CORE_FILE}" ] || [ ! -s "${LOCALDNS_CORE_FILE}" ]; then
                    printf "Localdns corefile either does not exist or is empty at %s.\n" "${LOCALDNS_CORE_FILE}"
                    return 1
                fi

                # This is slice file used by localdns systemd unit.
                if [ ! -f "${LOCALDNS_SLICE_FILE}" ] || [ ! -s "${LOCALDNS_SLICE_FILE}" ]; then
                    printf "Localdns slice file does not exist at %s.\n" "${LOCALDNS_SLICE_FILE}"
                    return 1
                fi

                # Check if coredns binary is cached in VHD and is executable.
                # ----------------------------------------------------------------------------------------------------------------
                # Coredns binary is extracted from cached coredns image and pre-installed in the VHD -
                # /opt/azure/containers/localdns/binary/coredns.
                if [[ ! -f "${COREDNS_BINARY_PATH}" || ! -x "${COREDNS_BINARY_PATH}" ]]; then
                    printf "Coredns binary either doesn't exist or isn't executable at %s.\n" "${COREDNS_BINARY_PATH}"
                    return 1
                fi

                if ! timeout 5s "${COREDNS_BINARY_PATH}" --version >/dev/null 2>&1; then
                    printf "Failed to execute '%s --version'.\n" "${COREDNS_BINARY_PATH}"
                    return 1
                fi

                # Replace Vnet_DNS_Server in corefile with VNET DNS Server IPs.
                # -----------------------------------------------------------------------------------------------------------------
                if [[ ! -f "$RESOLV_CONF" ]]; then
                    printf "%s not found.\n" "$RESOLV_CONF"
                    return 1
                fi
                # Get the upstream VNET DNS servers from /run/systemd/resolve/resolv.conf.
                UPSTREAM_VNET_DNS_SERVERS=$(awk '/nameserver/ {print $2}' "$RESOLV_CONF" | paste -sd' ')
                if [[ -z "${UPSTREAM_VNET_DNS_SERVERS}" ]]; then
                    printf "No Upstream VNET DNS servers found in %s.\n" "$RESOLV_CONF"
                    return 1
                fi

                # Based on customer input, corefile was generated in pkg/agent/baker.go.
                # Replace 168.63.129.16 with VNET DNS ServerIPs only if VNET DNS ServerIPs is not equal to 168.63.129.16.
                # Corefile will have 168.63.129.16 when user input has VnetDNS value for forwarddestination.
                # Note - For root domain under VnetDNSOverrides, all DNS traffic should be forwarded to VnetDNS.
                if [[ "${UPSTREAM_VNET_DNS_SERVERS}" != "${AZURE_DNS_IP}" ]]; then
                    sed -i -e "s|${AZURE_DNS_IP}|${UPSTREAM_VNET_DNS_SERVERS}|g" "${LOCALDNS_CORE_FILE}" || {
                        printf "Updating corefile failed."
                        return 1
                    }
                fi
            }
            verify_localdns_files
        }
        cleanup() {
            rm -rf "$LOCALDNS_SCRIPT_PATH"
            rm -rf "$LOCALDNS_SLICE_PATH"
            rm -rf "$COREDNS_BINARY_PATH"
            rm -rf "$RESOLV_CONF"
        }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        #------------------------ Test localdns corefile -------------------------------------------------
        It 'should return failure if localdns corefile is missing'
            rm -r "$LOCALDNS_CORE_FILE"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Localdns corefile either does not exist or is empty at $LOCALDNS_CORE_FILE."
        End

        It 'should return failure if localdns corefile is empty'
            > "$LOCALDNS_CORE_FILE"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Localdns corefile either does not exist or is empty at $LOCALDNS_CORE_FILE."
        End

        #------------------------ Test localdns slice file ------------------------------------------------
        It 'should return failure if localdns slicefile is missing'
            rm -f "$LOCALDNS_SLICE_FILE"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Localdns slice file does not exist at $LOCALDNS_SLICE_FILE."
        End

        It 'should return failure if localdns slicefile is empty'
            > "$LOCALDNS_SLICE_FILE"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Localdns slice file does not exist at $LOCALDNS_SLICE_FILE."
        End

        #------------------------- Test coredns binary -----------------------------------------------------
        It 'should return failure if coredns binary is missing'
            rm -f "$COREDNS_BINARY_PATH"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Coredns binary either doesn't exist or isn't executable at $COREDNS_BINARY_PATH."
        End

        It 'should return failure if coredns binary is not executable'
            chmod -x "$COREDNS_BINARY_PATH"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Coredns binary either doesn't exist or isn't executable at $COREDNS_BINARY_PATH."
        End

        It 'should return failure if coredns --version command fails'
            echo '#!/bin/bash' > "$COREDNS_BINARY_PATH"
            echo 'exit 1' >> "$COREDNS_BINARY_PATH"
            chmod +x "$COREDNS_BINARY_PATH"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "Failed to execute '$COREDNS_BINARY_PATH --version'."
        End

        #------------------------- Test systemd resolv -----------------------------------------------------
        It 'should fail if /run/systemd/resolve/resolv.conf not found'
            rm -f "$RESOLV_CONF"
            When run verify_localdns_files
            The status should be failure
            The stdout should include "/run/systemd/resolve/resolv.conf not found."
        End

        #------------------------- Test AzureDNS replace in corefile ---------------------------------------
        It 'should replace 168.63.129.16 with UpstreamDNS IPs if upstream VNET DNS server is not same as AzureDNS'
            When run verify_localdns_files
            The status should be success
            The file "${LOCALDNS_CORE_FILE}" should exist
            The contents of file "${LOCALDNS_CORE_FILE}" should include "forward . 10.0.0.1 10.0.0.2"
        End

        #------------------------- All success path in verify_localdns_files -------------------------------
        It 'should return success - all successful path'
            When run verify_localdns_files
            The status should be success
            The file "${LOCALDNS_CORE_FILE}" should exist
            The contents of file "${LOCALDNS_CORE_FILE}" should include "forward . 10.0.0.1 10.0.0.2"
        End
    End


# Iptables: build rules and Information variables.
# --------------------------------------------------------------------------------------------------------------------
    Describe 'setup_localdns_iptables_and_network_info'
        setup() {
            LOCALDNS_NODE_LISTENER_IP="169.254.10.10"
            LOCALDNS_CLUSTER_LISTENER_IP="169.254.10.11"
            AZURE_DNS_IP="168.63.129.16"
            ERR_LOCALDNS_FAIL=216

            IPTABLES='iptables -w -t raw -m comment --comment "localdns: skip conntrack"'
            IPTABLES_RULES=()

            mock_ip_command() {
                echo '[{"dev": "eth0"}]'
            }

            mock_networkctl_command() {
                echo '{"NetworkFile": "/etc/systemd/network/eth0.network"}'
            }

            setup_localdns_iptables() {
                IPTABLES_RULES=()

                for CHAIN in OUTPUT PREROUTING; do
                    for IP in ${LOCALDNS_NODE_LISTENER_IP} ${LOCALDNS_CLUSTER_LISTENER_IP}; do
                        for PROTO in tcp udp; do
                            IPTABLES_RULES+=("${CHAIN} -p ${PROTO} -d ${IP} --dport 53 -j NOTRACK")
                        done
                    done
                done
                
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
                for rule in "${expected_rules[@]}"; do
                    found=false
                    for existing_rule in "${IPTABLES_RULES[@]}"; do
                        if [[ "$existing_rule" == "$rule" ]]; then
                            found=true
                            break
                        fi
                    done

                    if [[ "$found" == false ]]; then
                        all_rules_found=false
                    fi
                done

                if [[ "$all_rules_found" == true ]]; then
                    return 0
                else
                    return 1
                fi
            }
            setup_localdns_iptables

            setup_localdns_network_info() {
                DEFAULT_ROUTE_INTERFACE="$(mock_ip_command)"
                DEFAULT_ROUTE_INTERFACE=$(echo "$DEFAULT_ROUTE_INTERFACE" | jq -r 'if type == "array" and length > 0 then .[0].dev else empty end')

                if [[ -z "${DEFAULT_ROUTE_INTERFACE}" ]]; then
                    echo "Unable to determine the default route interface for ${AZURE_DNS_IP}."
                    return 1
                fi

                NETWORK_FILE="$(mock_networkctl_command)"
                NETWORK_FILE=$(echo "$NETWORK_FILE" | jq -r '.NetworkFile')

                if [[ -z "${NETWORK_FILE}" ]]; then
                    echo "Unable to determine network file for interface ${DEFAULT_ROUTE_INTERFACE}."
                    return 1
                fi

                NETWORK_DROPIN_DIR="${NETWORK_FILE}.d"
                NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/70-localdns.conf"

                export IPTABLES IPTABLES_RULES DEFAULT_ROUTE_INTERFACE NETWORK_FILE NETWORK_DROPIN_DIR NETWORK_DROPIN_FILE
            }
            setup_localdns_network_info
        }

        cleanup() {
            unset IPTABLES IPTABLES_RULES DEFAULT_ROUTE_INTERFACE NETWORK_FILE NETWORK_DROPIN_DIR NETWORK_DROPIN_FILE
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'

        #------------------------- Test IPTABLES rules ------------------------------------------------------
        It 'should build iptables rules correctly for OUTPUT and PREROUTING'
            When call setup
            The status should be success
        End

        #------------------------- Test Network Info ------------------------------------------------------
        It 'should correctly set network-related variables'
            When call setup_localdns_network_info
            The variable DEFAULT_ROUTE_INTERFACE should equal "eth0"
            The variable NETWORK_FILE should equal "/etc/systemd/network/eth0.network"
            The variable NETWORK_DROPIN_DIR should equal "/etc/systemd/network/eth0.network.d"
            The variable NETWORK_DROPIN_FILE should equal "/etc/systemd/network/eth0.network.d/70-localdns.conf"
        End

        It 'should fail if no default route interface is found'
            mock_ip_command() {
                echo '[]'
            }
            When call setup_localdns_network_info
            The status should be failure
            The stdout should include "Unable to determine the default route interface for 168.63.129.16."
        End
    
        It 'should fail if no network file is found'
            mock_networkctl_command() {
                echo '{"NetworkFile": ""}'
            }
            When call setup_localdns_network_info
            The status should be failure
            The stdout should include "Unable to determine network file for interface eth0."
        End
    End


# Cleanup function will be run on script exit/crash to revert config.
# --------------------------------------------------------------------------------------------------------------------
    Describe "cleanup function"
        cleanup() {
            # Disable error handling so that we don't get into a recursive loop.
            set +e

            # Remove iptables rules to stop forwarding DNS traffic.
            for RULE in "${IPTABLES_RULES[@]}"; do
                if eval "${IPTABLES}" -C "${RULE}" 2>/dev/null; then
                    eval "${IPTABLES}" -D "${RULE}"
                    if [ $? -eq 0 ]; then
                        printf "Successfully removed iptables rule: %s.\n" "${RULE}"
                    else
                        printf "Failed to remove iptables rule: %s.\n"
                        return 1
                    fi
                fi
            done

            # Revert the changes made to the DNS configuration if present.
            if [ -f "${NETWORK_DROPIN_FILE}" ]; then
                printf "Reverting DNS configuration by removing %s.\n" "${NETWORK_DROPIN_FILE}"
                if /bin/rm -f "${NETWORK_DROPIN_FILE}"; then
                    networkctl reload || {
                        printf "Failed to reload network after removing the DNS configuration.\n"
                        return 1
                    }
                else
                    printf "Failed to remove %s.\n" "${NETWORK_DROPIN_FILE}"
                    return 1
                fi
            fi

            # Trigger localdns shutdown, if running.
            if [[ -n "${COREDNS_PID}" ]] && [[ "${COREDNS_PID}" =~ ^[0-9]+$ ]]; then
                if mock_ps; then
                    if [[ "${LOCALDNS_SHUTDOWN_DELAY}" -gt 0 ]]; then
                        printf "Sleeping %d seconds to allow connections to terminate.\n" "${LOCALDNS_SHUTDOWN_DELAY}"
                        sleep "${LOCALDNS_SHUTDOWN_DELAY}"
                    fi
                    printf "Sending SIGINT to localdns and waiting for it to terminate.\n"
                    mock_kill "${COREDNS_PID}"
                    kill_status=$?
                    if [ $kill_status -eq 0 ]; then
                        printf "Successfully sent SIGINT to localdns.\n"
                    else
                        printf "Failed to send SIGINT to localdns. Exit status: %s.\n" "$kill_status"
                        return 1
                    fi

                    if mock_wait "${COREDNS_PID}"; then
                        printf "Localdns terminated successfully.\n"
                    else
                        printf "Localdns failed to terminate properly.\n"
                        return 1
                    fi
                fi
            fi

            # Delete the dummy interface if present.
            if mock_ip_link_show >/dev/null 2>&1; then
                printf "Removing localdns dummy interface.\n"
                mock_ip_link_del $LOCALDNS_INTERFACE
                if [ $? -eq 0 ]; then
                    printf "Successfully removed localdns dummy interface.\n"
                else
                    printf "Failed to remove localdns dummy interface.\n"
                    return 1
                fi
            fi

            # Indicate successful cleanup.
            printf "Successfully cleanup localdns related configurations.\n"
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
            When call cleanup
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
            When call cleanup
            The stdout should include "Successfully cleanup localdns related configurations."
        End

        It "should successfully cleanup"
            COREDNS_PID="12345"
            When call cleanup
            The status should be success
            The stdout should include "Sending SIGINT to localdns and waiting for it to terminate."
            The stdout should include "Successfully sent SIGINT to localdns."
            The stdout should include "Localdns terminated successfully."
            The stdout should include "Successfully cleanup localdns related configurations."
        End

        It "should fail cleanup"
            COREDNS_PID="54321"
            When call cleanup
            The status should be failure
            The stdout should include "Sending SIGINT to localdns and waiting for it to terminate."
            The stdout should include "Failed to send SIGINT to localdns. Exit status: 1."
        End

        It "should remove dummy interface if exists"
            When call cleanup
            The stdout should include "Successfully removed localdns dummy interface"
        End

        It "should handle errors in iptables deletion"
            LOCALDNS_INTERFACE="name localdns"
            When call cleanup
            The stdout should include "Removing localdns dummy interface."
            The stdout should include "Successfully removed localdns dummy interface."
        End
    End


# Start localdns.
# --------------------------------------------------------------------------------------------------------------------
    Describe 'wait_for_localdns_ready'
        LOCALDNS_NODE_LISTENER_IP="169.254.10.10"
        mock_curl() {
            echo "OK"
        }
        
        wait_for_localdns_ready() {
            declare -i ATTEMPTS=0
            local START_TIME=$(date +%s)

            #printf "Waiting for localdns to start and be able to serve traffic.\n"
            until [ "$(mock_curl)" == "OK" ]; do
                if [ $ATTEMPTS -ge $MAX_ATTEMPTS ]; then
                    printf "Localdns failed to come online after %d attempts.\n" "$MAX_ATTEMPTS"
                    return 1
                fi
                # Check for timeout based on elapsed time.
                local CURRENT_TIME=$(date +%s)
                local ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
                if [ $ELAPSED_TIME -ge $TIMEOUT ]; then
                    printf "Localdns failed to come online after %d seconds (timeout).\n" "$TIMEOUT"
                    return 1
                fi
                sleep 1
                ((ATTEMPTS++))
            done
            #printf "Localdns is online and ready to serve traffic.\n"
            return 0
        }
        wait_for_localdns_ready

        It 'should return success'
            MAX_ATTEMPTS=100
            TIMEOUT=5
            When call wait_for_localdns_ready
            The status should be success
        End

        It 'should return failure, after timeout'
            mock_curl() {
                echo "NOTOK"
            }
            MAX_ATTEMPTS=1000
            TIMEOUT=5
            When call wait_for_localdns_ready
            The status should be failure
            The output should include "Localdns failed to come online after 5 seconds (timeout)."
        End

        It 'should return failure, after max attempts'
            mock_curl() {
                echo "NOTOK"
            }
            MAX_ATTEMPTS=10
            TIMEOUT=50
            When call wait_for_localdns_ready
            The status should be failure
            The output should include "Localdns failed to come online after 10 attempts."
        End

        It 'should retry and succeed after 5 attempts'
            MAX_ATTEMPTS=60
            TIMEOUT=10
            mock_curl() {
                if [ "$ATTEMPTS" -lt 5 ]; then
                    echo "FAIL"
                else
                    echo "OK"
                fi
            }
            When call wait_for_localdns_ready
            The status should be success
        End
    End


# Disable DNS from DHCP and point the system at localdns.
# --------------------------------------------------------------------------------------------------------------------
    Describe 'disable_dhcp_use_clusterlistener'
        NETWORK_DROPIN_DIR="/etc/systemd/network"
        NETWORK_DROPIN_FILE="${NETWORK_DROPIN_DIR}/10-localdns.conf"
        LOCALDNS_NODE_LISTENER_IP="169.254.10.10"
        ERR_LOCALDNS_FAIL=1
        mock_networkctl_reload() {
            return 0
        }

        disable_dhcp_use_clusterlistener() {
            #printf "Updating network DNS configuration to point to localdns via %s.\n" "${NETWORK_DROPIN_FILE}"
            mkdir -p "${NETWORK_DROPIN_DIR}"

cat > "${NETWORK_DROPIN_FILE}" <<EOF
# Set DNS server to localdns cluster listernerIP.
[Network]
DNS=${LOCALDNS_NODE_LISTENER_IP}

# Disable DNS provided by DHCP to ensure local DNS is used.
[DHCP]
UseDNS=false
EOF
            # Set permissions on the drop-in directory and file.
            chmod -R ugo+rX "${NETWORK_DROPIN_DIR}"

            mock_networkctl_reload
            if [[ $? -ne 0 ]]; then
                echo "Failed to reload networkctl."
                return 1
            fi
            #printf "Startup complete - serving node and pod DNS traffic.\n"
        }
        disable_dhcp_use_clusterlistener

        It 'should update network configuration and reload networkctl'
            When call disable_dhcp_use_clusterlistener
            The status should be success
            The file "${NETWORK_DROPIN_FILE}" should exist
            The contents of file "${NETWORK_DROPIN_FILE}" should include "UseDNS=false"
            The contents of file "${NETWORK_DROPIN_FILE}" should include "DNS=169.254.10.10"
        End

        It 'should fail if networkctl reload fails'
            mock_networkctl_reload() {
                return 1
            }
            When call disable_dhcp_use_clusterlistener
            The status should be failure
            The output should include "Failed to reload networkctl."
        End
    End
End