#!/bin/bash
# test/node-problem-detector/fixtures/testdata/create_dns_test_data.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Create mock data for DNS checks - healthy CoreDNS scenario
create_dns_healthy() {
    echo "Create healthy DNS scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/dns-healthy"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/var/run/npd/check_dns_issues_state"
    
    # /proc/net/snmp - normal UDP statistics (no errors)
    cat > "$dir/proc/net/snmp" <<'EOF'
Ip: Forwarding DefaultTTL InReceives InHdrErrors InAddrErrors ForwDatagrams InUnknownProtos InDiscards InDelivers OutRequests OutDiscards OutNoRoutes ReasmTimeout ReasmReqds ReasmOKs ReasmFails FragOKs FragFails FragCreates
Ip: 2 64 1000 0 0 0 0 0 1000 1000 0 0 0 0 0 0 0 0 0
Icmp: InMsgs InErrors InCsumErrors InDestUnreachs InTimeExcds InParmProbs InSrcQuenchs InRedirects InEchos InEchoReps InTimestamps InTimestampReps InAddrMasks InAddrMaskReps OutMsgs OutErrors OutDestUnreachs OutTimeExcds OutParmProbs OutSrcQuenchs OutRedirects OutEchos OutEchoReps OutTimestamps OutTimestampReps OutAddrMasks OutAddrMaskReps
Icmp: 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
IcmpMsg:
Tcp: RtoAlgorithm RtoMin RtoMax MaxConn ActiveOpens PassiveOpens AttemptFails EstabResets CurrEstab InSegs OutSegs RetransSegs InErrs OutRsts InCsumErrors
Tcp: 1 200 120000 -1 100 50 0 0 5 2000 2000 0 0 0 0
Udp: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti MemErrors
Udp: 500 0 0 500 0 0 0 0 0
UdpLite: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti
UdpLite: 0 0 0 0 0 0 0 0
EOF

    # Mock kubeconfig
    cat > "$dir/var/lib/kubelet/kubeconfig" <<'EOF'
apiVersion: v1
clusters:
- cluster:
    server: https://test-cluster-api.hcp.eastus.azmk8s.io:443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: clusterUser_test-rg_test-cluster
  name: test-cluster
current-context: test-cluster
kind: Config
preferences: {}
users:
- name: clusterUser_test-rg_test-cluster
  user:
    client-certificate-data: LS0tLS1CRUdJTi...
    client-key-data: LS0tLS1CRUdJTi...
EOF
}

# Create mock data for DNS checks - unhealthy CoreDNS scenario
create_dns_unhealthy() {
    echo "Create unhealthy DNS scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/dns-unhealthy"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/var/run/npd/check_dns_issues_state"
    
    # /proc/net/snmp - showing UDP errors
    cat > "$dir/proc/net/snmp" <<'EOF'
Ip: Forwarding DefaultTTL InReceives InHdrErrors InAddrErrors ForwDatagrams InUnknownProtos InDiscards InDelivers OutRequests OutDiscards OutNoRoutes ReasmTimeout ReasmReqds ReasmOKs ReasmFails FragOKs FragFails FragCreates
Ip: 2 64 1000 0 0 0 0 0 1000 1000 0 0 0 0 0 0 0 0 0
Icmp: InMsgs InErrors InCsumErrors InDestUnreachs InTimeExcds InParmProbs InSrcQuenchs InRedirects InEchos InEchoReps InTimestamps InTimestampReps InAddrMasks InAddrMaskReps OutMsgs OutErrors OutDestUnreachs OutTimeExcds OutParmProbs OutSrcQuenchs OutRedirects OutEchos OutEchoReps OutTimestamps OutTimestampReps OutAddrMasks OutAddrMaskReps
Icmp: 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
IcmpMsg:
Tcp: RtoAlgorithm RtoMin RtoMax MaxConn ActiveOpens PassiveOpens AttemptFails EstabResets CurrEstab InSegs OutSegs RetransSegs InErrs OutRsts InCsumErrors
Tcp: 1 200 120000 -1 100 50 0 0 5 2000 2000 0 0 0 0
Udp: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti MemErrors
Udp: 500 5 10 500 2 1 3 0 1
UdpLite: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti
UdpLite: 0 0 0 0 0 0 0 0
EOF

    # Create state file showing previous lower UDP InErrors count (5 errors vs current 10)
    echo "1721000000 5" > "$dir/var/run/npd/check_dns_issues_state/udp_in_errors.state"

    # Same kubeconfig as healthy scenario
    cp "$SCRIPT_DIR/mock-data/dns-healthy/var/lib/kubelet/kubeconfig" "$dir/var/lib/kubelet/kubeconfig"
}

# Create mock data for DNS checks - leading spaces bug scenario
create_dns_leading_spaces_bug() {
    echo "Create DNS leading spaces bug scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/dns-leading-spaces-bug"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/var/run/npd/check_dns_issues_state"
    
    # Same as healthy scenario
    cp -r "$SCRIPT_DIR/mock-data/dns-healthy/proc" "$dir/"
    cp -r "$SCRIPT_DIR/mock-data/dns-healthy/var" "$dir/"
}

# Create mock data for DNS checks - no leading spaces (working) scenario
create_dns_no_leading_spaces() {
    echo "Create DNS no leading spaces scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/dns-no-leading-spaces"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/var/run/npd/check_dns_issues_state"
    
    # Same as healthy scenario
    cp -r "$SCRIPT_DIR/mock-data/dns-healthy/proc" "$dir/"
    cp -r "$SCRIPT_DIR/mock-data/dns-healthy/var" "$dir/"
}

# Create mock data for DNS checks - IP address endpoint scenario  
create_dns_ip_endpoint() {
    echo "Create DNS IP address endpoint scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/dns-ip-endpoint"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/var/run/npd/check_dns_issues_state"
    
    # Same proc/net/snmp as healthy scenario
    cp -r "$SCRIPT_DIR/mock-data/dns-healthy/proc" "$dir/"
    cp -r "$SCRIPT_DIR/mock-data/dns-healthy/var/run" "$dir/var/"
    
    # Create kubeconfig with IP address instead of FQDN
    mkdir -p "$dir/var/lib/kubelet"
    cat > "$dir/var/lib/kubelet/kubeconfig" <<'EOF'
apiVersion: v1
clusters:
- cluster:
    server: https://10.0.0.1:443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: clusterUser_test-rg_test-cluster
  name: test-cluster
current-context: test-cluster
kind: Config
preferences: {}
users:
- name: clusterUser_test-rg_test-cluster
  user:
    client-certificate-data: LS0tLS1CRUdJTi...
    client-key-data: LS0tLS1CRUdJTi...
EOF
}

# Create mock commands for DNS tests
create_mock_commands() {
    echo "Create DNS mock commands"
    local cmd_dir="$SCRIPT_DIR/mock-commands/dns"
    mkdir -p "$cmd_dir"
    
    # Mock kubectl for healthy CoreDNS pods
    cat > "$cmd_dir/kubectl-healthy" <<'EOF'
#!/bin/bash
# Mock kubectl for healthy CoreDNS scenario
case "$*" in
    *"get pods"*)
        echo "coredns-7968c94f89-5f2m9:10.240.0.150"
        echo "coredns-7968c94f89-gs585:10.240.1.239"
        ;;
    *"config view"*)
        echo "https://test-cluster-api.hcp.eastus.azmk8s.io:443"
        ;;
    *)
        echo "Mock kubectl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock kubectl for unhealthy CoreDNS pods
    cat > "$cmd_dir/kubectl-unhealthy" <<'EOF'
#!/bin/bash
# Mock kubectl for unhealthy CoreDNS scenario
case "$*" in
    *"get pods"*)
        echo "coredns-7968c94f89-5f2m9:10.240.0.150"
        echo "coredns-7968c94f89-gs585:10.240.1.239"
        ;;
    *"config view"*)
        echo "https://test-cluster-api.hcp.eastus.azmk8s.io:443"
        ;;
    *)
        echo "Mock kubectl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock kubectl for IP endpoint scenario
    cat > "$cmd_dir/kubectl-ip-endpoint" <<'EOF'
#!/bin/bash
# Mock kubectl for IP endpoint scenario
case "$*" in
    *"get pods"*)
        echo "coredns-7968c94f89-5f2m9:10.240.0.150"
        echo "coredns-7968c94f89-gs585:10.240.1.239"
        ;;
    *"config view"*)
        echo "https://10.0.0.1:443"
        ;;
    *)
        echo "Mock kubectl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock kubectl for RBAC permission failure
    cat > "$cmd_dir/kubectl-rbac-forbidden" <<'EOF'
#!/bin/bash
# Mock kubectl that returns RBAC permission error
case "$*" in
    *"get pods"*)
        echo 'Error from server (Forbidden): pods is forbidden: User "system:node:aks-le64i4f-29427988-vmss000028" cannot list resource "pods" in API group "" in the namespace "kube-system": can only list/watch pods with spec.nodeName field selector' >&2
        exit 1
        ;;
    *"config view"*)
        echo "https://test-cluster-api.hcp.eastus.azmk8s.io:443"
        ;;
    *)
        echo "Mock kubectl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock wget with HTTP/2 response (reproduces the actual bug!)
    cat > "$cmd_dir/wget-http2-bug" <<'EOF'
#!/bin/bash
# Mock wget that outputs HTTP/2 response - this reproduces the real issue!
echo "  HTTP/2 200"
echo "  Date: Mon, 14 Jul 2025 18:48:48 GMT"
echo "  Content-Length: 2"
echo "  Content-Type: text/plain; charset=utf-8"
exit 0
EOF

    # Mock wget that actually fails to match regex (reproduces the mystery!)
    cat > "$cmd_dir/wget-failing-case" <<'EOF'
#!/bin/bash
# Mock wget that should reproduce the actual production failure
# Let's try some edge cases that might cause regex failure
echo "  HTTP/1.1 200 OK"
echo "  Date: Mon, 14 Jul 2025 18:48:48 GMT"
echo "  Content-Length: 2"
echo "  Content-Type: text/plain; charset=utf-8"
exit 0
EOF

    # Mock wget with leading spaces (for edge case testing)
    cat > "$cmd_dir/wget-leading-spaces" <<'EOF'
#!/bin/bash
# Mock wget that outputs leading spaces like real wget -S
echo "  HTTP/1.1 200 OK"
echo "  Date: Mon, 14 Jul 2025 18:48:48 GMT"
echo "  Content-Length: 2"
echo "  Content-Type: text/plain; charset=utf-8"
exit 0
EOF

    # Mock wget without leading spaces (works correctly)
    cat > "$cmd_dir/wget-no-leading-spaces" <<'EOF'
#!/bin/bash
# Mock wget that outputs without leading spaces
echo "HTTP/1.1 200 OK"
echo "Date: Mon, 14 Jul 2025 18:48:48 GMT"
echo "Content-Length: 2"
echo "Content-Type: text/plain; charset=utf-8"
exit 0
EOF

    # Mock wget for unhealthy response (non-200)
    cat > "$cmd_dir/wget-unhealthy" <<'EOF'
#!/bin/bash
# Mock wget that returns unhealthy response
echo "  HTTP/1.1 503 Service Unavailable"
echo "  Date: Mon, 14 Jul 2025 18:48:48 GMT"
echo "  Content-Length: 0"
echo "  Content-Type: text/plain; charset=utf-8"
exit 0
EOF

    # Mock wget timeout/failure
    cat > "$cmd_dir/wget-timeout" <<'EOF'
#!/bin/bash
# Mock wget that simulates timeout
echo "wget: unable to resolve host address 'nonexistent.example.com'" >&2
exit 4
EOF

    # Mock nslookup successful
    cat > "$cmd_dir/nslookup-success" <<'EOF'
#!/bin/bash
# Mock nslookup successful resolution
cat <<END
Server:    10.0.0.10
Address:   10.0.0.10#53

Non-authoritative answer:
Name: test-cluster-api.hcp.eastus.azmk8s.io
Address: 20.231.239.123
END
exit 0
EOF

    # Mock nslookup failure
    cat > "$cmd_dir/nslookup-failure" <<'EOF'
#!/bin/bash
# Mock nslookup failure
cat <<END
Server:    10.0.0.10
Address:   10.0.0.10#53

** server can't find test-cluster-api.hcp.eastus.azmk8s.io: NXDOMAIN
END
exit 1
EOF

    # Mock iptables-save for CoreDNS IP discovery
    cat > "$cmd_dir/iptables-save" <<'EOF'
#!/bin/bash
# Mock iptables-save for CoreDNS IP discovery
cat <<END
# Generated by iptables-save v1.6.1 on Mon Jul 14 18:48:48 2025
*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]
:KUBE-SERVICES - [0:0]
-A KUBE-SERVICES -d 10.0.0.10/32 -p tcp -m comment --comment "kube-system/kube-dns:dns-tcp cluster IP" -m tcp --dport 53 -j KUBE-SVC-TCOU7JCQXEZGVUNU
-A KUBE-SERVICES -d 10.0.0.10/32 -p udp -m comment --comment "kube-system/kube-dns:dns cluster IP" -m udp --dport 53 -j KUBE-SVC-ERIFXISQEP7F7OF4
COMMIT
# Completed on Mon Jul 14 18:48:48 2025
END
EOF

    # Create cat mock to intercept /proc/net/snmp reads
    cat > "$cmd_dir/cat" <<'EOF'
#!/bin/bash
# cat dispatcher for DNS tests - intercepts reads to /proc/net/snmp
SCENARIO="${DNS_SCENARIO:-healthy}"

# If reading /proc/net/snmp, return our mock data
if [[ "$*" == *"/proc/net/snmp"* ]]; then
    # Return mock data based on scenario
    case "$SCENARIO" in
        "healthy")
            # Healthy scenario - low UDP errors
            cat <<'SNMP'
Ip: Forwarding DefaultTTL InReceives InHdrErrors InAddrErrors ForwDatagrams InUnknownProtos InDiscards InDelivers OutRequests OutDiscards OutNoRoutes ReasmTimeout ReasmReqds ReasmOKs ReasmFails FragOKs FragFails FragCreates
Ip: 2 64 1000 0 0 0 0 0 1000 1000 0 0 0 0 0 0 0 0 0
Icmp: InMsgs InErrors InCsumErrors InDestUnreachs InTimeExcds InParmProbs InSrcQuenchs InRedirects InEchos InEchoReps InTimestamps InTimestampReps InAddrMasks InAddrMaskReps OutMsgs OutErrors OutDestUnreachs OutTimeExcds OutParmProbs OutSrcQuenchs OutRedirects OutEchos OutEchoReps OutTimestamps OutTimestampReps OutAddrMasks OutAddrMaskReps
Icmp: 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
IcmpMsg:
Tcp: RtoAlgorithm RtoMin RtoMax MaxConn ActiveOpens PassiveOpens AttemptFails EstabResets CurrEstab InSegs OutSegs RetransSegs InErrs OutRsts InCsumErrors
Tcp: 1 200 120000 -1 100 50 0 0 5 2000 2000 0 0 0 0
Udp: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti MemErrors
Udp: 500 5 0 500 0 0 0 0 0
UdpLite: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti
UdpLite: 0 0 0 0 0 0 0 0
SNMP
            ;;
        "unhealthy")
            # Unhealthy scenario - high UDP errors
            cat <<'SNMP'
Ip: Forwarding DefaultTTL InReceives InHdrErrors InAddrErrors ForwDatagrams InUnknownProtos InDiscards InDelivers OutRequests OutDiscards OutNoRoutes ReasmTimeout ReasmReqds ReasmOKs ReasmFails FragOKs FragFails FragCreates
Ip: 2 64 1000 0 0 0 0 0 1000 1000 0 0 0 0 0 0 0 0 0
Icmp: InMsgs InErrors InCsumErrors InDestUnreachs InTimeExcds InParmProbs InSrcQuenchs InRedirects InEchos InEchoReps InTimestamps InTimestampReps InAddrMasks InAddrMaskReps OutMsgs OutErrors OutDestUnreachs OutTimeExcds OutParmProbs OutSrcQuenchs OutRedirects OutEchos OutEchoReps OutTimestamps OutTimestampReps OutAddrMasks OutAddrMaskReps
Icmp: 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
IcmpMsg:
Tcp: RtoAlgorithm RtoMin RtoMax MaxConn ActiveOpens PassiveOpens AttemptFails EstabResets CurrEstab InSegs OutSegs RetransSegs InErrs OutRsts InCsumErrors
Tcp: 1 200 120000 -1 100 50 0 0 5 2000 2000 0 0 0 0
Udp: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti MemErrors
Udp: 500 5 10 500 2 1 3 0 1
UdpLite: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti
UdpLite: 0 0 0 0 0 0 0 0
SNMP
            ;;
        *)
            # Default/other scenarios - no UDP errors
            cat <<'SNMP'
Ip: Forwarding DefaultTTL InReceives InHdrErrors InAddrErrors ForwDatagrams InUnknownProtos InDiscards InDelivers OutRequests OutDiscards OutNoRoutes ReasmTimeout ReasmReqds ReasmOKs ReasmFails FragOKs FragFails FragCreates
Ip: 2 64 1000 0 0 0 0 0 1000 1000 0 0 0 0 0 0 0 0 0
Icmp: InMsgs InErrors InCsumErrors InDestUnreachs InTimeExcds InParmProbs InSrcQuenchs InRedirects InEchos InEchoReps InTimestamps InTimestampReps InAddrMasks InAddrMaskReps OutMsgs OutErrors OutDestUnreachs OutTimeExcds OutParmProbs OutSrcQuenchs OutRedirects OutEchos OutEchoReps OutTimestamps OutTimestampReps OutAddrMasks OutAddrMaskReps
Icmp: 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
IcmpMsg:
Tcp: RtoAlgorithm RtoMin RtoMax MaxConn ActiveOpens PassiveOpens AttemptFails EstabResets CurrEstab InSegs OutSegs RetransSegs InErrs OutRsts InCsumErrors
Tcp: 1 200 120000 -1 100 50 0 0 5 2000 2000 0 0 0 0
Udp: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti MemErrors
Udp: 500 5 0 500 0 0 0 0 0
UdpLite: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti
UdpLite: 0 0 0 0 0 0 0 0
SNMP
            ;;
    esac
else
    # Pass through to real cat for all other files
    exec /bin/cat "$@"
fi
EOF

    # Create dispatchers based on scenario environment variables
    cat > "$cmd_dir/kubectl" <<'EOF'
#!/bin/bash
# kubectl dispatcher for DNS tests
SCENARIO="${DNS_SCENARIO:-healthy}"

# Look for scenario-specific mock in dns directory
if [ -f "/mock-commands/dns/kubectl-$SCENARIO" ]; then
    exec "/mock-commands/dns/kubectl-$SCENARIO" "$@"
fi

# Default to healthy scenario
exec "/mock-commands/dns/kubectl-healthy" "$@"
EOF

    cat > "$cmd_dir/wget" <<'EOF'
#!/bin/bash
# wget dispatcher for DNS tests
SCENARIO="${WGET_SCENARIO:-no-leading-spaces}"

# Look for scenario-specific mock in dns directory
if [ -f "/mock-commands/dns/wget-$SCENARIO" ]; then
    exec "/mock-commands/dns/wget-$SCENARIO" "$@"
fi

# Default to no leading spaces scenario
exec "/mock-commands/dns/wget-no-leading-spaces" "$@"
EOF

    cat > "$cmd_dir/nslookup" <<'EOF'
#!/bin/bash
# nslookup dispatcher for DNS tests
SCENARIO="${NSLOOKUP_SCENARIO:-success}"

# Look for scenario-specific mock in dns directory
if [ -f "/mock-commands/dns/nslookup-$SCENARIO" ]; then
    exec "/mock-commands/dns/nslookup-$SCENARIO" "$@"
fi

# Default to success scenario
exec "/mock-commands/dns/nslookup-success" "$@"
EOF

    # Make all mock commands executable
    chmod +x "$cmd_dir"/*
}

# Create mock data for dedicated UDP error scenarios
create_dns_udp_errors() {
    echo "Create dedicated UDP error detection scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/dns-udp-errors"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/var/run/npd/check_dns_issues_state"
    
    # /proc/net/snmp - showing UDP errors (10 InErrors)
    cat > "$dir/proc/net/snmp" <<'EOF'
Ip: Forwarding DefaultTTL InReceives InHdrErrors InAddrErrors ForwDatagrams InUnknownProtos InDiscards InDelivers OutRequests OutDiscards OutNoRoutes ReasmTimeout ReasmReqds ReasmOKs ReasmFails FragOKs FragFails FragCreates
Ip: 2 64 1000 0 0 0 0 0 1000 1000 0 0 0 0 0 0 0 0 0
Icmp: InMsgs InErrors InCsumErrors InDestUnreachs InTimeExcds InParmProbs InSrcQuenchs InRedirects InEchos InEchoReps InTimestamps InTimestampReps InAddrMasks InAddrMaskReps OutMsgs OutErrors OutDestUnreachs OutTimeExcds OutParmProbs OutSrcQuenchs OutRedirects OutEchos OutEchoReps OutTimestamps OutTimestampReps OutAddrMasks OutAddrMaskReps
Icmp: 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
IcmpMsg:
Tcp: RtoAlgorithm RtoMin RtoMax MaxConn ActiveOpens PassiveOpens AttemptFails EstabResets CurrEstab InSegs OutSegs RetransSegs InErrs OutRsts InCsumErrors
Tcp: 1 200 120000 -1 100 50 0 0 5 2000 2000 0 0 0 0
Udp: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti MemErrors
Udp: 500 5 10 500 2 1 3 0 1
UdpLite: InDatagrams NoPorts InErrors OutDatagrams RcvbufErrors SndbufErrors InCsumErrors IgnoredMulti
UdpLite: 0 0 0 0 0 0 0 0
EOF

    # Create state file showing previous lower UDP InErrors count (5 errors vs current 10)
    echo "1721000000 5" > "$dir/var/run/npd/check_dns_issues_state/udp_in_errors.state"

    # Same kubeconfig as healthy scenario - we're focused on UDP error detection
    cp "$SCRIPT_DIR/mock-data/dns-healthy/var/lib/kubelet/kubeconfig" "$dir/var/lib/kubelet/kubeconfig"
}

# Create mock state data directories for stateful testing
create_mock_state_directories() {
    # Scenario 1: UDP errors baseline (first run) - no prior state
    local baseline_dir="$SCRIPT_DIR/mock-data/dns-udp-errors-baseline"
    mkdir -p "$baseline_dir/proc/net" "$baseline_dir/var/lib/kubelet" "$baseline_dir/var/run/npd/check_dns_issues_state"
    
    # Copy proc and kubeconfig from healthy scenario (only state differs)
    cp "$SCRIPT_DIR/mock-data/dns-healthy/proc/net/snmp" "$baseline_dir/proc/net/snmp"
    cp "$SCRIPT_DIR/mock-data/dns-healthy/var/lib/kubelet/kubeconfig" "$baseline_dir/var/lib/kubelet/kubeconfig"
    
    # Ensure no prior state exists for baseline test - remove any existing state file
    rm -f "$baseline_dir/var/run/npd/check_dns_issues_state/udp_in_errors.state"
            
    # Scenario 2: UDP errors detected (second run) - with prior state
    local detected_dir="$SCRIPT_DIR/mock-data/dns-udp-errors-detected"
    mkdir -p "$detected_dir/proc/net" "$detected_dir/var/lib/kubelet" "$detected_dir/var/run/npd/check_dns_issues_state"
    
    # Copy proc and kubeconfig from unhealthy scenario (same UDP error pattern)
    cp "$SCRIPT_DIR/mock-data/dns-unhealthy/proc/net/snmp" "$detected_dir/proc/net/snmp"
    cp "$SCRIPT_DIR/mock-data/dns-unhealthy/var/lib/kubelet/kubeconfig" "$detected_dir/var/lib/kubelet/kubeconfig"
    
    # Create state file with previous UDP InErrors count of 5 at timestamp 1721000000
    # This will result in a delta when combined with current count of 10 from mock cat
    echo "1721000000 5" > "$detected_dir/var/run/npd/check_dns_issues_state/udp_in_errors.state"
}

# Create isolated mock data directories for integration tests
create_integration_test_directories() {
    echo "Create integration test specific mock data directories"
    
    # RBAC test directory - based on healthy scenario with no UDP errors
    local rbac_dir="$SCRIPT_DIR/mock-data/dns-healthy-rbac"
    mkdir -p "$rbac_dir/proc/net" "$rbac_dir/var/lib/kubelet" "$rbac_dir/var/run/npd/check_dns_issues_state"
    
    # Copy healthy scenario data
    cp "$SCRIPT_DIR/mock-data/dns-healthy/proc/net/snmp" "$rbac_dir/proc/net/snmp"
    cp "$SCRIPT_DIR/mock-data/dns-healthy/var/lib/kubelet/kubeconfig" "$rbac_dir/var/lib/kubelet/kubeconfig"
    # Create baseline state file for RBAC test (no prior runs expected)
    echo "1721000000 0" > "$rbac_dir/var/run/npd/check_dns_issues_state/udp_in_errors.state"
    
    # DNS failure test directory - based on unhealthy scenario with UDP errors to detect
    local failure_dir="$SCRIPT_DIR/mock-data/dns-unhealthy-failure"
    mkdir -p "$failure_dir/proc/net" "$failure_dir/var/lib/kubelet" "$failure_dir/var/run/npd/check_dns_issues_state"
    
    # Copy unhealthy scenario data
    cp "$SCRIPT_DIR/mock-data/dns-unhealthy/proc/net/snmp" "$failure_dir/proc/net/snmp"  
    cp "$SCRIPT_DIR/mock-data/dns-unhealthy/var/lib/kubelet/kubeconfig" "$failure_dir/var/lib/kubelet/kubeconfig"
    # Create state file showing previous lower UDP InErrors count (5 vs current 10)
    # This ensures UDP error delta will be detected (5 new errors)
    echo "1721000000 5" > "$failure_dir/var/run/npd/check_dns_issues_state/udp_in_errors.state"
}

# Run all setup
create_dns_healthy
create_dns_unhealthy  
create_dns_leading_spaces_bug
create_dns_no_leading_spaces
create_dns_ip_endpoint
create_dns_udp_errors
create_mock_commands
create_mock_state_directories
create_integration_test_directories

echo "DNS mock data creation complete"