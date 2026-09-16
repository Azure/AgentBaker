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

    Describe 'localdns_is_mid_restart_cycle'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns-fallback.sh"
            # Mock 'systemctl show ... -p SubState --value'.
            systemctl() { echo "${SUBSTATE:-}"; }
        }
        BeforeEach 'setup'

        It 'reports true while systemd is holding the unit for an auto-restart'
            SUBSTATE="auto-restart"
            When call localdns_is_mid_restart_cycle
            The status should be success
        End

        It 'reports true for the queued auto-restart substate (systemd 255)'
            SUBSTATE="auto-restart-queued"
            When call localdns_is_mid_restart_cycle
            The status should be success
        End

        It 'reports false once the unit has settled in failed'
            SUBSTATE="failed"
            When call localdns_is_mid_restart_cycle
            The status should be failure
        End

        It 'reports false for a cleanly stopped unit'
            SUBSTATE="dead"
            When call localdns_is_mid_restart_cycle
            The status should be failure
        End

        It 'reports false for a running (possibly wedged) unit'
            SUBSTATE="running"
            When call localdns_is_mid_restart_cycle
            The status should be failure
        End

        It 'reports false when the substate cannot be read'
            SUBSTATE=""
            When call localdns_is_mid_restart_cycle
            The status should be failure
        End
    End

    Describe 'start_fallback (restart-cycle guard)'
        setup() {
            Include "./parts/linux/cloud-init/artifacts/localdns-fallback.sh"
            MARKER=$(mktemp)
            : > "${MARKER}"
            systemctl() { echo "${SUBSTATE:-}"; }
            # Any of these running means the guard did not short-circuit.
            verify_coredns_binary()           { echo "verify" >> "${MARKER}"; }
            ensure_cluster_listener_interface() { echo "iface" >> "${MARKER}"; }
            generate_fallback_corefile()      { echo "corefile" >> "${MARKER}"; }
        }
        cleanup() { rm -f "${MARKER}"; }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        # Regression: systemd fires OnFailure= on every failed start attempt, not
        # only on entry to terminal 'failed'. Binding .11 on those early
        # invocations made the fallback flap instead of covering the outage.
        It 'does not take over .11 while localdns is between restart attempts'
            SUBSTATE="auto-restart"
            When call start_fallback
            The status should be success
            The stderr should include "not taking over 169.254.10.11 yet"
            The contents of file "${MARKER}" should not include "iface"
            The contents of file "${MARKER}" should not include "corefile"
        End

        It 'proceeds once localdns has settled in failed'
            SUBSTATE="failed"
            # start_fallback ends in 'exec', which replaces the process. 'When run'
            # isolates that in a subshell; /bin/true stands in for the coredns
            # binary so the exec succeeds instead of aborting the example.
            COREDNS_BINARY_PATH="/bin/true"
            unset SYSTEMD_EXEC_PID
            When run start_fallback
            The status should be success
            The stderr should include "starting fallback coredns bound to 169.254.10.11"
            The contents of file "${MARKER}" should include "verify"
            The contents of file "${MARKER}" should include "iface"
            The contents of file "${MARKER}" should include "corefile"
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
