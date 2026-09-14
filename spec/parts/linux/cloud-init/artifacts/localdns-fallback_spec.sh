#!/bin/bash

Describe 'localdns-fallback.sh'
# Tests the pod-DNS fallback helper functions defined in
# parts/linux/cloud-init/artifacts/localdns-fallback.sh.
#------------------------------------------------------------------------------

    Describe 'resolve_upstream'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns-fallback.sh"
        }
        BeforeEach 'setup'

        It 'returns COREDNS_SERVICE_IP when set'
            COREDNS_SERVICE_IP="172.16.0.10"
            When call resolve_upstream
            The output should equal "172.16.0.10"
        End

        It 'falls back to the default when COREDNS_SERVICE_IP is empty'
            COREDNS_SERVICE_IP=""
            When call resolve_upstream
            The output should equal "10.0.0.10"
            The stderr should include "using default 10.0.0.10"
        End

        It 'falls back to the default when COREDNS_SERVICE_IP is unset'
            unset COREDNS_SERVICE_IP
            When call resolve_upstream
            The output should equal "10.0.0.10"
            The stderr should include "using default 10.0.0.10"
        End
    End

    Describe 'generate_fallback_corefile'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns-fallback.sh"
            TMP_DIR=$(mktemp -d)
            FALLBACK_DIR="${TMP_DIR}/fallback"
            FALLBACK_COREFILE="${FALLBACK_DIR}/fallback.corefile"
        }
        cleanup() { rm -rf "${TMP_DIR}"; }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'binds .11 and forwards to the configured ClusterIP'
            COREDNS_SERVICE_IP="10.0.0.10"
            When call generate_fallback_corefile
            The status should be success
            The stderr should include "forwarding 169.254.10.11 -> 10.0.0.10"
            The path "${FALLBACK_COREFILE}" should be file
            The contents of file "${FALLBACK_COREFILE}" should include "bind 169.254.10.11"
            The contents of file "${FALLBACK_COREFILE}" should include "forward . 10.0.0.10"
        End

        It 'honors a custom ClusterIP verbatim'
            COREDNS_SERVICE_IP="172.16.0.10"
            When call generate_fallback_corefile
            The status should be success
            The stderr should include "forwarding 169.254.10.11 -> 172.16.0.10"
            The contents of file "${FALLBACK_COREFILE}" should include "forward . 172.16.0.10"
            The contents of file "${FALLBACK_COREFILE}" should not include "forward . 10.0.0.10"
        End
    End

    Describe 'ensure_cluster_listener_interface (idempotency)'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns-fallback.sh"
            IP_LOG=$(mktemp)
            : > "${IP_LOG}"
            # Mock 'ip'. IFACE_EXISTS and ADDR_EXISTS control the two branches.
            ip() {
                echo "ip $*" >> "${IP_LOG}"
                case "$*" in
                    "link show localdns")
                        [ "${IFACE_EXISTS:-0}" = "1" ] && return 0 || return 1 ;;
                    "addr show dev localdns")
                        [ "${ADDR_EXISTS:-0}" = "1" ] && echo "inet 169.254.10.11/32" || echo "" ;;
                    *) return 0 ;;
                esac
            }
        }
        cleanup() { rm -f "${IP_LOG}"; }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'creates the dummy interface when it is absent'
            IFACE_EXISTS=0 ADDR_EXISTS=0
            When call ensure_cluster_listener_interface
            The status should be success
            The stderr should include "creating it"
            The contents of file "${IP_LOG}" should include "ip link add name localdns type dummy"
            The contents of file "${IP_LOG}" should include "ip addr add 169.254.10.11/32 dev localdns"
        End

        It 'does not re-add .11 when it is already present'
            IFACE_EXISTS=1 ADDR_EXISTS=1
            When call ensure_cluster_listener_interface
            The status should be success
            The stderr should include "already present"
            The contents of file "${IP_LOG}" should not include "ip link add name localdns type dummy"
            The contents of file "${IP_LOG}" should not include "ip addr add 169.254.10.11/32 dev localdns"
        End

        It 'adds .11 when the interface exists but the address does not'
            IFACE_EXISTS=1 ADDR_EXISTS=0
            When call ensure_cluster_listener_interface
            The status should be success
            The stderr should include "assigning cluster listener"
            The contents of file "${IP_LOG}" should not include "ip link add name localdns type dummy"
            The contents of file "${IP_LOG}" should include "ip addr add 169.254.10.11/32 dev localdns"
        End
    End

    Describe 'cleanup_fallback (localdns-aware, no address removal)'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns-fallback.sh"
            # Mock systemctl is-active for localdns.
            systemctl() {
                if [ "$1" = "is-active" ]; then
                    [ "${LOCALDNS_ACTIVE:-0}" = "1" ] && return 0 || return 3
                fi
                return 0
            }
        }
        BeforeEach 'setup'

        It 'leaves .11 in place when localdns is active'
            LOCALDNS_ACTIVE=1
            When call cleanup_fallback
            The status should be success
            The stderr should include "leaving 169.254.10.11 in place"
        End

        It 'is a no-op (no address removal) when localdns is not active'
            LOCALDNS_ACTIVE=0
            When call cleanup_fallback
            The status should be success
            The stderr should include "no address removal"
        End
    End
End
