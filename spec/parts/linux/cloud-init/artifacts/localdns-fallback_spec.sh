#!/bin/bash

Describe 'localdns-fallback.sh'
# Tests the kernel-NAT fallback in
# parts/linux/cloud-init/artifacts/localdns-fallback.sh.
#
# The script shells out to iptables, systemctl, curl and jq. Each Describe below
# replaces the ones it needs with functions, so the real control flow runs
# against synthetic output rather than the node's networking.
#------------------------------------------------------------------------------
    FALLBACK="./parts/linux/cloud-init/artifacts/localdns-fallback.sh"

    Describe 'wait_for_vnet_dns'
        setup() {
            TMPD=$(mktemp -d)
            RESOLV="$TMPD/resolv.conf"
            RESOLV_WAIT_SECONDS=1
            __SOURCED__=1 . "$FALLBACK"
            RESOLV="$TMPD/resolv.conf"
            RESOLV_WAIT_SECONDS=1
        }
        cleanup() { rm -rf "$TMPD"; }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'returns the first nameserver that is not the node listener'
            printf 'nameserver 169.254.10.10\nnameserver 168.63.129.16\nnameserver 10.0.0.53\n' > "$RESOLV"
            When call wait_for_vnet_dns
            The output should equal "168.63.129.16"
        End

        # Until networkctl reload lands, the file still holds the drop-in state:
        # DNS=169.254.10.10 with UseDNS=false, i.e. our own listener and nothing
        # else. Reading it once here would yield no upstream at all.
        It 'does not accept the node listener as an upstream'
            printf 'nameserver 169.254.10.10\n' > "$RESOLV"
            When call wait_for_vnet_dns
            The status should be failure
            The output should equal ""
        End

        It 'treats an empty resolv.conf as not yet converged'
            : > "$RESOLV"
            When call wait_for_vnet_dns
            The status should be failure
        End

        # The reason this function polls at all: ExecStopPost removes the drop-in
        # and runs 'networkctl reload', which is asynchronous, so the upstream can
        # appear after the wait has already started. Reading once would return
        # failure here and leave the node with no fallback at all.
        It 'picks up an upstream that appears after the wait has started'
            RESOLV_WAIT_SECONDS=5
            printf 'nameserver 169.254.10.10\n' > "$RESOLV"
            # Rename rather than write in place so the poll can never observe a
            # half-written file.
            ( sleep 1
              printf 'nameserver 169.254.10.10\nnameserver 168.63.129.16\n' > "${RESOLV}.new"
              mv "${RESOLV}.new" "$RESOLV" ) &
            When call wait_for_vnet_dns
            The output should equal "168.63.129.16"
        End
    End

    Describe 'kube_dns_chain'
        setup() {
            __SOURCED__=1 . "$FALLBACK"
            iptables() {
                cat <<'EOF'
-A KUBE-SERVICES -d 172.16.0.1/32 -p tcp -m comment --comment "default/kubernetes:https cluster IP" -m tcp --dport 443 -j KUBE-SVC-NPX46M4PTMTKRN6Y
-A KUBE-SERVICES -d 172.16.0.10/32 -p udp -m comment --comment "kube-system/kube-dns:dns cluster IP" -m udp --dport 53 -j KUBE-SVC-TCOU7JCQXEZGVUNU
-A KUBE-SERVICES -d 172.16.0.10/32 -p tcp -m comment --comment "kube-system/kube-dns:dns-tcp cluster IP" -m tcp --dport 53 -j KUBE-SVC-ERIFXISQEP7F7OF4
EOF
            }
        }
        BeforeEach 'setup'

        It 'reads the udp kube-dns chain off KUBE-SERVICES'
            When call kube_dns_chain udp dns
            The output should equal "KUBE-SVC-TCOU7JCQXEZGVUNU"
        End

        It 'reads the tcp kube-dns chain off KUBE-SERVICES'
            When call kube_dns_chain tcp dns-tcp
            The output should equal "KUBE-SVC-ERIFXISQEP7F7OF4"
        End

        # The name is read rather than recomputed from a hash, so an upstream
        # rename surfaces as "not found" -- and apply refuses -- instead of a
        # silently wrong jump.
        It 'returns nothing when the service is absent'
            When call kube_dns_chain udp nonexistent
            The output should equal ""
        End
    End

    Describe 'add_spread'
        setup() {
            __SOURCED__=1 . "$FALLBACK"
            iptables() { echo "iptables $*"; }
        }
        BeforeEach 'setup'

        # The 1/(n-i) ladder kube-proxy uses: 1/3, then 1/2, then the last rule
        # takes whatever the earlier draws did not. That yields an even split.
        It 'gives each of three backends an equal share'
            When call add_spread udp 10.244.0.5 10.244.1.7 10.244.2.9
            The output should include "--probability 0.33333"
            The output should include "--probability 0.50000"
            The output should include "DNAT --to-destination 10.244.2.9:53"
        End

        It 'emits a single unconditional rule for one backend'
            When call add_spread udp 10.244.0.5
            The output should not include "--probability"
            The output should include "DNAT --to-destination 10.244.0.5:53"
        End
    End

    Describe 'clear_rules'
        setup() {
            __SOURCED__=1 . "$FALLBACK"
            CALLS="$(mktemp)"
            # -D succeeds once per hook, then fails, so the delete loop terminates.
            iptables() {
                echo "$*" >> "$CALLS"
                case "$*" in
                    *"-D PREROUTING"*|*"-D OUTPUT"*)
                        grep -c -- "$*" "$CALLS" | grep -q '^1$' && return 0
                        return 1 ;;
                esac
                return 0
            }
        }
        cleanup() { rm -f "$CALLS"; }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'removes the jumps from both hooks, then flushes and deletes the chain'
            When call clear_rules
            The contents of file "$CALLS" should include "-D PREROUTING -j LOCALDNS-FALLBACK"
            The contents of file "$CALLS" should include "-D OUTPUT -j LOCALDNS-FALLBACK"
            The contents of file "$CALLS" should include "-F LOCALDNS-FALLBACK"
            The contents of file "$CALLS" should include "-X LOCALDNS-FALLBACK"
        End
    End

    Describe 'apply_rules gating'
        setup() {
            TMPD=$(mktemp -d)
            __SOURCED__=1 . "$FALLBACK"
            RESOLV="$TMPD/resolv.conf"
            RESOLV_WAIT_SECONDS=1
            CILIUM_SOCK="$TMPD/absent.sock"
            printf 'nameserver 168.63.129.16\n' > "$RESOLV"
            LOCALDNS_STATE=failed
            systemctl() { echo "$LOCALDNS_STATE"; }
            iptables() { case "$*" in *"-D "*) return 1 ;; esac; return 0; }
        }
        cleanup() { rm -rf "$TMPD"; }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        # Redirecting while localdns is serving would silently take its traffic
        # away. Only terminal 'failed' is a safe moment.
        It 'refuses while localdns is active'
            LOCALDNS_STATE=active
            When run apply_rules
            The status should be failure
            The stderr should include "not failed; not redirecting"
        End

        It 'refuses while localdns is still activating'
            LOCALDNS_STATE=activating
            When run apply_rules
            The status should be failure
            The stderr should include "not failed; not redirecting"
        End

        It 'refuses when no usable upstream is present'
            printf 'nameserver 169.254.10.10\n' > "$RESOLV"
            When run apply_rules
            The status should be failure
            The stderr should include "no upstream nameserver"
        End

        It 'refuses when neither kube-proxy chains nor a cilium socket are found'
            When run apply_rules
            The status should be failure
            The stderr should include "neither kube-proxy kube-dns chains nor a Cilium agent socket"
        End
    End

    Describe 'apply_rules on a kube-proxy node'
        setup() {
            TMPD=$(mktemp -d)
            __SOURCED__=1 . "$FALLBACK"
            RESOLV="$TMPD/resolv.conf"
            RESOLV_WAIT_SECONDS=1
            CILIUM_SOCK="$TMPD/absent.sock"
            RULES="$TMPD/rules"
            printf 'nameserver 168.63.129.16\n' > "$RESOLV"
            systemctl() { echo failed; }
            iptables() {
                echo "$*" >> "$RULES"
                case "$*" in
                    # clear_rules deletes until -D fails; real iptables fails once
                    # the rule is gone, so the mock must too or the loop spins.
                    *"-D "*) return 1 ;;
                    *"-S KUBE-SERVICES"*)
                        echo '-A KUBE-SERVICES -d 172.16.0.10/32 -p udp -m comment --comment "kube-system/kube-dns:dns cluster IP" -m udp --dport 53 -j KUBE-SVC-TCOU7JCQXEZGVUNU'
                        echo '-A KUBE-SERVICES -d 172.16.0.10/32 -p tcp -m comment --comment "kube-system/kube-dns:dns-tcp cluster IP" -m tcp --dport 53 -j KUBE-SVC-ERIFXISQEP7F7OF4'
                        ;;
                esac
                return 0
            }
        }
        cleanup() { rm -rf "$TMPD"; }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'redirects the node listener at the VNet DNS over udp and tcp'
            When call apply_rules
            The stderr should include "applied (kube-proxy)"
            The contents of file "$RULES" should include "-d 169.254.10.10/32 -p udp --dport 53 -j DNAT --to-destination 168.63.129.16:53"
            The contents of file "$RULES" should include "-d 169.254.10.10/32 -p tcp --dport 53 -j DNAT --to-destination 168.63.129.16:53"
        End

        # The ClusterIP is deliberately not a target: the nat table is traversed
        # once per connection and DNAT is terminal, so rewriting .11 to the
        # ClusterIP would consume the traversal KUBE-SERVICES needed to translate
        # it into a pod IP. Jump straight into kube-proxy's service chain instead.
        It 'jumps the cluster listener into kube-proxy own kube-dns chains'
            When call apply_rules
            The stderr should include "applied"
            The contents of file "$RULES" should include "-d 169.254.10.11/32 -p udp --dport 53 -j KUBE-SVC-TCOU7JCQXEZGVUNU"
            The contents of file "$RULES" should include "-d 169.254.10.11/32 -p tcp --dport 53 -j KUBE-SVC-ERIFXISQEP7F7OF4"
        End

        It 'never DNATs the cluster listener at a ClusterIP'
            When call apply_rules
            The stderr should include "applied"
            The contents of file "$RULES" should not include "-d 169.254.10.11/32 -p udp --dport 53 -j DNAT"
        End

        It 'inserts the chain ahead of everything else in nat'
            When call apply_rules
            The stderr should include "applied"
            The contents of file "$RULES" should include "-I PREROUTING 1 -j LOCALDNS-FALLBACK"
            The contents of file "$RULES" should include "-I OUTPUT 1 -j LOCALDNS-FALLBACK"
        End
    End

    Describe 'apply_rules on a cilium node'
        setup() {
            TMPD=$(mktemp -d)
            __SOURCED__=1 . "$FALLBACK"
            RESOLV="$TMPD/resolv.conf"
            RESOLV_WAIT_SECONDS=1
            CILIUM_SOCK="$TMPD/cilium.sock"
            RULES="$TMPD/rules"
            printf 'nameserver 168.63.129.16\n' > "$RESOLV"
            # [ -S ] needs a real unix socket, so stand in for the probe itself.
            cilium_agent_present() { return 0; }
            HOST_ROUTING=true
            BACKENDS="10.244.0.5
10.244.1.7"
            systemctl() { echo failed; }
            iptables() {
                echo "$*" >> "$RULES"
                case "$*" in *"-D "*) return 1 ;; esac
                return 0
            }
            kube_dns_chain() { echo ""; }
            cilium_legacy_host_routing() { [ "$HOST_ROUTING" = true ]; }
            cilium_kube_dns_backends() { echo "$BACKENDS"; }
        }
        cleanup() { rm -rf "$TMPD"; }
        BeforeEach 'setup'
        AfterEach 'cleanup'

        # eBPF host routing redirects pod egress past netfilter entirely, so a
        # nat rule would never see the packet. Refusing is the honest answer.
        It 'refuses when cilium uses eBPF host routing'
            HOST_ROUTING=false
            When run apply_rules
            The status should be failure
            The stderr should include "bypasses netfilter"
        End

        It 'refuses when the agent reports no active backends'
            BACKENDS=""
            When run apply_rules
            The status should be failure
            The stderr should include "no active kube-dns backends"
        End

        It 'spreads the cluster listener across the reported backends'
            When call apply_rules
            The stderr should include "applied (cilium)"
            The contents of file "$RULES" should include "DNAT --to-destination 10.244.0.5:53"
            The contents of file "$RULES" should include "DNAT --to-destination 10.244.1.7:53"
        End

        It 'still redirects the node listener at the VNet DNS'
            When call apply_rules
            The stderr should include "applied (cilium)"
            The contents of file "$RULES" should include "-d 169.254.10.10/32 -p udp --dport 53 -j DNAT --to-destination 168.63.129.16:53"
        End
    End
End
