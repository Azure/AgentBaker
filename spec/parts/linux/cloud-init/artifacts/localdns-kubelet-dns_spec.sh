#!/bin/bash

Describe 'localdns-kubelet-dns.sh'
# Tests the kubelet --cluster-dns repoint helper defined in
# parts/linux/cloud-init/artifacts/localdns-kubelet-dns.sh.
#
# The invariant under test throughout: kubelet is restarted ONLY when the file
# actually changed. A flapping localdns must never flap kubelet.
#------------------------------------------------------------------------------

    setup() {
        Include "./parts/linux/cloud-init/artifacts/localdns-kubelet-dns.sh"
        TMP_DIR=$(mktemp -d)
        KUBELET_DEFAULT_FILE="${TMP_DIR}/kubelet"
        ORIGINAL_STATE_FILE="${TMP_DIR}/kubelet-cluster-dns.orig"
        RESTART_LOG="${TMP_DIR}/restarts"
        : > "${RESTART_LOG}"
        systemctl() { echo "systemctl $*" >> "${RESTART_LOG}"; }
        # A realistic KUBELET_FLAGS line: --cluster-dns is one flag among many, and
        # the rewrite must not disturb its neighbours.
        write_kubelet_file() {
            cat > "${KUBELET_DEFAULT_FILE}" <<EOF
KUBELET_FLAGS=--address=0.0.0.0 --cluster-dns=$1 --cluster-domain=cluster.local --pod-max-pids=-1
KUBELET_REGISTER_SCHEDULABLE=true
KUBELET_NODE_LABELS=agentpool=ldnp
EOF
        }
    }
    cleanup() { rm -rf "${TMP_DIR}"; }
    BeforeEach 'setup'
    AfterEach 'cleanup'

    Describe 'current_cluster_dns'
        It 'extracts the value from the KUBELET_FLAGS line'
            write_kubelet_file "169.254.10.11"
            When call current_cluster_dns
            The output should equal "169.254.10.11"
        End

        It 'fails when the file does not exist'
            KUBELET_DEFAULT_FILE="${TMP_DIR}/absent"
            When call current_cluster_dns
            The status should be failure
        End
    End

    Describe 'point_to_coredns'
        It 'repoints from the localdns cluster listener and restarts kubelet'
            write_kubelet_file "169.254.10.11"
            COREDNS_SERVICE_IP="10.0.0.10"
            When call point_to_coredns
            The status should be success
            The stderr should include "169.254.10.11 -> 10.0.0.10"
            The contents of file "${KUBELET_DEFAULT_FILE}" should include "--cluster-dns=10.0.0.10"
            The contents of file "${RESTART_LOG}" should include "restart --no-block kubelet.service"
        End

        It 'preserves every other kubelet flag'
            write_kubelet_file "169.254.10.11"
            COREDNS_SERVICE_IP="10.0.0.10"
            When call point_to_coredns
            The status should be success
            The stderr should include "->"
            The contents of file "${KUBELET_DEFAULT_FILE}" should include "--address=0.0.0.0"
            The contents of file "${KUBELET_DEFAULT_FILE}" should include "--cluster-domain=cluster.local"
            The contents of file "${KUBELET_DEFAULT_FILE}" should include "--pod-max-pids=-1"
            The contents of file "${KUBELET_DEFAULT_FILE}" should include "KUBELET_NODE_LABELS=agentpool=ldnp"
        End

        It 'honours a custom ClusterIP'
            write_kubelet_file "169.254.10.11"
            COREDNS_SERVICE_IP="172.16.0.10"
            When call point_to_coredns
            The status should be success
            The stderr should include "-> 172.16.0.10"
            The contents of file "${KUBELET_DEFAULT_FILE}" should include "--cluster-dns=172.16.0.10"
        End

        It 'records the original value so restore can put it back'
            write_kubelet_file "169.254.10.11"
            COREDNS_SERVICE_IP="10.0.0.10"
            When call point_to_coredns
            The status should be success
            The stderr should include "->"
            The contents of file "${ORIGINAL_STATE_FILE}" should equal "169.254.10.11"
        End

        # Idempotency: this is what stops a flapping localdns from flapping kubelet.
        It 'does not restart kubelet when already pointed at CoreDNS'
            write_kubelet_file "10.0.0.10"
            COREDNS_SERVICE_IP="10.0.0.10"
            When call point_to_coredns
            The status should be success
            The stderr should include "already 10.0.0.10"
            The contents of file "${RESTART_LOG}" should equal ""
        End

        It 'leaves an unrelated --cluster-dns value alone'
            write_kubelet_file "10.1.2.3"
            COREDNS_SERVICE_IP="10.0.0.10"
            When call point_to_coredns
            The status should be success
            The stderr should include "not the localdns cluster listener"
            The contents of file "${KUBELET_DEFAULT_FILE}" should include "--cluster-dns=10.1.2.3"
            The contents of file "${RESTART_LOG}" should equal ""
        End

        It 'is a no-op when there is no --cluster-dns flag at all'
            echo "KUBELET_FLAGS=--address=0.0.0.0" > "${KUBELET_DEFAULT_FILE}"
            COREDNS_SERVICE_IP="10.0.0.10"
            When call point_to_coredns
            The status should be success
            The stderr should include "nothing to repoint"
            The contents of file "${RESTART_LOG}" should equal ""
        End
    End

    Describe 'restore'
        It 'puts back the recorded original and restarts kubelet'
            write_kubelet_file "10.0.0.10"
            echo -n "169.254.10.11" > "${ORIGINAL_STATE_FILE}"
            When call restore
            The status should be success
            The stderr should include "10.0.0.10 -> 169.254.10.11"
            The contents of file "${KUBELET_DEFAULT_FILE}" should include "--cluster-dns=169.254.10.11"
            The contents of file "${RESTART_LOG}" should include "restart --no-block kubelet.service"
        End

        It 'clears the state file once restored'
            write_kubelet_file "10.0.0.10"
            echo -n "169.254.10.11" > "${ORIGINAL_STATE_FILE}"
            When call restore
            The status should be success
            The stderr should include "->"
            The path "${ORIGINAL_STATE_FILE}" should not be exist
        End

        # Regression. restore() runs from localdns.service ExecStartPost, i.e. on
        # every localdns start including every boot. An earlier version treated
        # "not the localdns listener" as licence to rewrite, which would silently
        # overwrite RP-provided configuration and bounce kubelet on a healthy node.
        It 'leaves an unrelated --cluster-dns alone when nothing was ever repointed'
            write_kubelet_file "10.1.2.3"
            rm -f "${ORIGINAL_STATE_FILE}"
            COREDNS_SERVICE_IP="10.0.0.10"
            When call restore
            The status should be success
            The stderr should equal ""
            The contents of file "${KUBELET_DEFAULT_FILE}" should include "--cluster-dns=10.1.2.3"
            The contents of file "${RESTART_LOG}" should equal ""
        End

        # If the recorded original is lost but the value on disk is exactly the
        # CoreDNS ClusterIP we would have written, it is safe to hand it back.
        It 'restores when the state file is lost but the value is one we would have set'
            write_kubelet_file "10.0.0.10"
            rm -f "${ORIGINAL_STATE_FILE}"
            COREDNS_SERVICE_IP="10.0.0.10"
            When call restore
            The status should be success
            The stderr should include "no recorded original"
            The contents of file "${KUBELET_DEFAULT_FILE}" should include "--cluster-dns=169.254.10.11"
            The contents of file "${RESTART_LOG}" should include "restart --no-block kubelet.service"
        End

        It 'does not guess when the state file is lost and COREDNS_SERVICE_IP is unset'
            write_kubelet_file "10.0.0.10"
            rm -f "${ORIGINAL_STATE_FILE}"
            unset COREDNS_SERVICE_IP
            When call restore
            The status should be success
            The stderr should equal ""
            The contents of file "${KUBELET_DEFAULT_FILE}" should include "--cluster-dns=10.0.0.10"
            The contents of file "${RESTART_LOG}" should equal ""
        End

        # This runs from localdns.service ExecStartPost on EVERY start, including
        # every normal boot, so the untouched case must be silent and free.
        It 'is a silent no-op when nothing was ever repointed'
            write_kubelet_file "169.254.10.11"
            When call restore
            The status should be success
            The stderr should equal ""
            The contents of file "${RESTART_LOG}" should equal ""
        End
    End
End
