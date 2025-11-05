#!/bin/bash
# test/node-problem-detector/fixtures/testdata/create_dns_coredns_test_data.sh
# Creates test data for CoreDNS monitoring script: check_dns_to_coredns.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Create mock data for CoreDNS monitoring - healthy scenario
create_coredns_healthy() {
    echo "Create healthy CoreDNS scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/coredns-healthy"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/opt/azure/containers/localdns" "$dir/run/systemd/resolve" "$dir/etc/default"
    
    # Create LocalDNS corefile with CoreDNS configuration
    cat > "$dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
# ***********************************************************************************
# WARNING: Changes to this file will be overwritten and not persisted.
# ***********************************************************************************
# whoami (used for health check of DNS)
health-check.localdns.local:53 {
    bind 169.254.10.10 169.254.10.11
    whoami
}
# VnetDNS overrides apply to DNS traffic from pods with dnsPolicy:default or kubelet (referred to as VnetDNS traffic).
.:53 {
    errors
    bind 169.254.10.10
    forward . 168.63.129.16 {
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s immediate
        servfail 0
    }
    loop
    nsid localdns
    prometheus :9253
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
    template ANY ANY reddog.microsoft.com {
        rcode NXDOMAIN
    }
}
cluster.local:53 {
    errors
    bind 169.254.10.10
    forward . 10.0.0.10 {
        force_tcp
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s immediate
        servfail 0
    }
    loop
    nsid localdns
    prometheus :9253
}
# KubeDNS overrides apply to DNS traffic from pods with dnsPolicy:ClusterFirst (referred to as KubeDNS traffic).
.:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.11:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s immediate
        servfail 0
    }
    loop
    nsid localdns-pod
    prometheus :9253
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
    template ANY ANY reddog.microsoft.com {
        rcode NXDOMAIN
    }
}
cluster.local:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
        force_tcp
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.11:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s immediate
        servfail 0
    }
    loop
    nsid localdns-pod
    prometheus :9253
}
EOF

    # Mock systemd resolved config
    cat > "$dir/run/systemd/resolve/resolv.conf" <<'EOF'
nameserver 168.63.129.16
nameserver 8.8.8.8
search default.svc.cluster.local svc.cluster.local cluster.local
options ndots:5
EOF

    # Mock kubelet default flags with real AKS configuration
    cat > "$dir/etc/default/kubelet" <<'EOF'
KUBELET_FLAGS=--address=0.0.0.0 --anonymous-auth=false --authentication-token-webhook=true --authorization-mode=Webhook --cgroups-per-qos=true --client-ca-file=/etc/kubernetes/certs/ca.crt --cloud-config=/etc/kubernetes/azure.json --cloud-provider=external --cluster-dns=100.20.20.10 --cluster-domain=cluster.local --container-log-max-size=50M --enforce-node-allocatable=pods --event-qps=0 --eviction-hard=memory.available<100Mi,nodefs.available<10%,nodefs.inodesFree<5%,pid.available<2000 --feature-gates=RotateKubeletServerCertificate=true --image-credential-provider-bin-dir=/var/lib/kubelet/credential-provider --image-credential-provider-config=/var/lib/kubelet/credential-provider-config.yaml --image-gc-high-threshold=85 --image-gc-low-threshold=80 --kube-reserved=cpu=180m,memory=2250Mi,pid=1000 --kubeconfig=/var/lib/kubelet/kubeconfig --max-pods=110 --node-status-update-frequency=10s --pod-infra-container-image=mcr.microsoft.com/oss/kubernetes/pause:3.6 --pod-manifest-path=/etc/kubernetes/manifests --protect-kernel-defaults=true --read-only-port=0 --rotate-certificates=true --rotate-server-certificates=true --serialize-image-pulls=false --streaming-connection-idle-timeout=4h --tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256 --node-ip=10.224.0.5
KUBELET_REGISTER_SCHEDULABLE=true
NETWORK_POLICY=
KUBELET_IMAGE=
KUBELET_NODE_LABELS=agentpool=userpool,kubernetes.azure.com/agentpool=userpool,agentpool=userpool,kubernetes.azure.com/agentpool=userpool,kubernetes.azure.com/azure-cni-overlay=true,kubernetes.azure.com/cluster=MC_sharshalocaldnstest_sharshalocaldnstest_westcentralus,kubernetes.azure.com/consolidated-additional-properties=7f6ddd83-9d56-11f0-ac7c-12f27c461584,kubernetes.azure.com/kubelet-identity-client-id=7c5119c6-1590-44aa-a6a1-a29d343e6450,kubernetes.azure.com/localdns-state=enabled,kubernetes.azure.com/mode=user,kubernetes.azure.com/network-name=aks-vnet-10914146,kubernetes.azure.com/network-policy=none,kubernetes.azure.com/network-resourcegroup=sharshalocaldnstest,kubernetes.azure.com/network-stateless-cni=false,kubernetes.azure.com/network-subnet=aks-subnet,kubernetes.azure.com/network-subscription=26fe00f8-9173-4872-9134-bb1d2e00343a,kubernetes.azure.com/node-image-version=AKSUbuntu-2204gen2containerd-202509.23.0,kubernetes.azure.com/nodenetwork-vnetguid=e3a60ba6-324e-4da0-9dec-cfbd9a951cc4,kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/os-sku=Ubuntu,kubernetes.azure.com/os-sku-effective=Ubuntu2204,kubernetes.azure.com/os-sku-requested=Ubuntu,kubernetes.azure.com/podnetwork-type=overlay,kubernetes.azure.com/role=agent,kubernetes.azure.com/kubelet-serving-ca=cluster
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

# Create mock data for CoreDNS - no LocalDNS scenario (iptables discovery)
create_coredns_no_localdns() {
    echo "Create CoreDNS no LocalDNS scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/coredns-no-localdns"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/run/systemd/resolve" "$dir/etc/default"
    
    # No LocalDNS corefile present
    
    # Mock systemd resolved config
    cat > "$dir/run/systemd/resolve/resolv.conf" <<'EOF'
nameserver 168.63.129.16
nameserver 8.8.8.8
search default.svc.cluster.local svc.cluster.local cluster.local
options ndots:5
EOF

    # Mock kubelet default flags with cluster-dns setting (no LocalDNS)
    cat > "$dir/etc/default/kubelet" <<'EOF'
KUBELET_FLAGS=--address=0.0.0.0 --cluster-dns=10.0.0.10 --cluster-domain=cluster.local --kubeconfig=/var/lib/kubelet/kubeconfig --pod-manifest-path=/etc/kubernetes/manifests
KUBELET_REGISTER_SCHEDULABLE=true
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

# Create mock commands for DNS CoreDNS
create_dns_coredns_mock_commands() {
    echo "Create DNS CoreDNS mock commands"
    local cmd_dir="$SCRIPT_DIR/mock-commands/dns-coredns"
    mkdir -p "$cmd_dir"
    
    # Mock dig command - successful resolution
    cat > "$cmd_dir/dig-success" <<'EOF'
#!/bin/bash
# Mock dig for successful DNS resolution
case "$*" in
    *kubernetes.default.svc.cluster.local*)
        # Simulate successful CoreDNS resolution
        cat <<END
; <<>> DiG 9.16.1 <<>> kubernetes.default.svc.cluster.local @10.0.0.10
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 54321
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; QUESTION SECTION:
;kubernetes.default.svc.cluster.local. IN A

;; ANSWER SECTION:
kubernetes.default.svc.cluster.local. 30 IN A 10.0.0.1

;; Query time: 2 msec
;; SERVER: 10.0.0.10#53(10.0.0.10)
;; WHEN: Mon Jul 15 10:00:00 UTC 2025
;; MSG SIZE  rcvd: 82
END
        exit 0
        ;;
    *)
        echo "Mock dig: unknown query: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock dig command - missing dependency (simulates command not found)
    cat > "$cmd_dir/dig-missing" <<'EOF'
#!/bin/bash
# Mock for missing dig command
exec /mock-commands/dns-coredns/command-not-found dig
EOF

    # Mock dig command - DNS resolution failure
    cat > "$cmd_dir/dig-failure" <<'EOF'
#!/bin/bash
# Mock dig for DNS resolution failure
case "$*" in
    *kubernetes.default.svc.cluster.local*)
        # Simulate DNS resolution failure
        cat <<END >&2
; <<>> DiG 9.16.1 <<>> kubernetes.default.svc.cluster.local @10.0.0.10
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: SERVFAIL, id: 54321
;; flags: qr rd ra; QUERY: 1, ANSWER: 0, AUTHORITY: 0, ADDITIONAL: 1

;; QUESTION SECTION:
;kubernetes.default.svc.cluster.local. IN A

;; Query time: 5000 msec
;; SERVER: 10.0.0.10#53(10.0.0.10)
;; WHEN: Mon Jul 15 10:00:00 UTC 2025
;; MSG SIZE  rcvd: 63
END
        exit 9
        ;;
    *)
        echo "Mock dig: unknown query: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock dig command - timeout scenario
    cat > "$cmd_dir/dig-timeout" <<'EOF'
#!/bin/bash
# Mock dig that simulates timeout
case "$*" in
    *kubernetes.default.svc.cluster.local*)
        # Simulate DNS timeout failure
        cat <<END >&2
; <<>> DiG 9.16.1 <<>> kubernetes.default.svc.cluster.local @10.0.0.10
;; global options: +cmd
;; connection timed out; no servers could be reached
END
        exit 9
        ;;
    *)
        echo "Mock dig: unknown query: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock dig command - success with retry
    cat > "$cmd_dir/dig-retry-success" <<'EOF'
#!/bin/bash
# Mock dig that fails first few times then succeeds
RETRY_FILE="/tmp/dig_retry_count_coredns_$$"
if [ -f "$RETRY_FILE" ]; then
    count=$(cat "$RETRY_FILE")
else
    count=0
fi
count=$((count + 1))
echo "$count" > "$RETRY_FILE"

if [ "$count" -le 2 ]; then
    # Fail first 2 attempts
    echo "Temporary failure" >&2
    exit 9
else
    # Succeed on 3rd attempt
    rm -f "$RETRY_FILE"
    case "$*" in
        *kubernetes.default.svc.cluster.local*)
            cat <<END
; <<>> DiG 9.16.1 <<>> kubernetes.default.svc.cluster.local @10.0.0.10
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 54321
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; QUESTION SECTION:
;kubernetes.default.svc.cluster.local. IN A

;; ANSWER SECTION:
kubernetes.default.svc.cluster.local. 30 IN A 10.0.0.1

;; Query time: 2 msec
;; SERVER: 10.0.0.10#53(10.0.0.10)
;; WHEN: Mon Jul 15 10:00:00 UTC 2025
;; MSG SIZE  rcvd: 82
END
            exit 0
            ;;
    esac
fi
EOF

    # Mock iptables-save for CoreDNS IP discovery
    cat > "$cmd_dir/iptables-save-success" <<'EOF'
#!/bin/bash
# Mock iptables-save for CoreDNS service IP discovery
case "$*" in
    *"-t nat"*)
        cat <<END
# Generated by iptables-save v1.8.4 on Mon Jul 15 10:00:00 2025
*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]
:KUBE-SERVICES - [0:0]
:KUBE-SVC-TCOU7JCQXEZGVUNU - [0:0]
-A PREROUTING -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A OUTPUT -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A KUBE-SERVICES -d 10.0.0.10/32 -p udp -m comment --comment "kube-system/kube-dns:dns-tcp cluster IP" -m udp --dport 53 -j KUBE-SVC-TCOU7JCQXEZGVUNU
-A KUBE-SERVICES -d 10.0.0.10/32 -p tcp -m comment --comment "kube-system/kube-dns:dns-tcp cluster IP" -m tcp --dport 53 -j KUBE-SVC-TCOU7JCQXEZGVUNU
-A KUBE-SVC-TCOU7JCQXEZGVUNU -m comment --comment "kube-system/kube-dns:dns-tcp"
COMMIT
# Completed on Mon Jul 15 10:00:00 2025
END
        ;;
    *)
        echo "Mock iptables-save: unknown args: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock iptables-save with no CoreDNS rules
    cat > "$cmd_dir/iptables-save-no-coredns" <<'EOF'
#!/bin/bash
# Mock iptables-save with no CoreDNS/kube-dns rules
case "$*" in
    *"-t nat"*)
        cat <<END
# Generated by iptables-save v1.8.4 on Mon Jul 15 10:00:00 2025
*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]
:KUBE-SERVICES - [0:0]
-A PREROUTING -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A OUTPUT -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
COMMIT
# Completed on Mon Jul 15 10:00:00 2025
END
        ;;
    *)
        echo "Mock iptables-save: unknown args: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock iptables-save with alternative CoreDNS rule format
    cat > "$cmd_dir/iptables-save-alternative" <<'EOF'
#!/bin/bash
# Mock iptables-save with alternative kube-dns rule format
case "$*" in
    *"-t nat"*)
        cat <<END
# Generated by iptables-save v1.8.4 on Mon Jul 15 10:00:00 2025
*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]
:KUBE-SERVICES - [0:0]
:KUBE-SVC-TCOU7JCQXEZGVUNU - [0:0]
-A PREROUTING -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A OUTPUT -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A KUBE-SERVICES -d 10.0.0.15/32 -p udp -m comment --comment "kube-system/kube-dns" -m udp --dport 53 -j KUBE-SVC-TCOU7JCQXEZGVUNU
-A KUBE-SVC-TCOU7JCQXEZGVUNU -m comment --comment "kube-system/kube-dns"
COMMIT
# Completed on Mon Jul 15 10:00:00 2025
END
        ;;
    *)
        echo "Mock iptables-save: unknown args: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock iptables-save with real AKS format (as provided by user)
    cat > "$cmd_dir/iptables-save-real-aks" <<'EOF'
#!/bin/bash
# Mock iptables-save with real AKS iptables output format
case "$*" in
    *"-t nat"*)
        cat <<END
# Generated by iptables-save v1.8.7 on Wed Oct  1 17:42:05 2025
*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [398301:30275106]
:KUBE-SERVICES - [0:0]
:KUBE-SVC-ERIFXISQEP7F7OF4 - [0:0]
:KUBE-SVC-TCOU7JCQXEZGVUNU - [0:0]
-A PREROUTING -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A OUTPUT -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A KUBE-SERVICES -d 10.0.0.10/32 -p udp -m comment --comment "kube-system/kube-dns:dns cluster IP" -j KUBE-SVC-TCOU7JCQXEZGVUNU
-A KUBE-SERVICES -d 10.0.0.10/32 -p tcp -m comment --comment "kube-system/kube-dns:dns-tcp cluster IP" -j KUBE-SVC-ERIFXISQEP7F7OF4
COMMIT
# Completed on Wed Oct  1 17:42:05 2025
END
        ;;
    *)
        echo "Mock iptables-save: unknown args: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock iptables-save with regex edge cases (single digit IPs)
    cat > "$cmd_dir/iptables-save-edge-cases" <<'EOF'
#!/bin/bash
# Mock iptables-save with edge case IP formats for regex testing
case "$*" in
    *"-t nat"*)
        cat <<END
# Generated by iptables-save v1.8.4 on Mon Jul 15 10:00:00 2025
*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]
:KUBE-SERVICES - [0:0]
-A PREROUTING -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A OUTPUT -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A KUBE-SERVICES -d 1.2.3.4/32 -p tcp -m comment --comment "kube-system/kube-dns:dns-tcp cluster IP" -j KUBE-SVC-TEST
-A KUBE-SERVICES -d 192.168.100.200/32 -p udp -m comment --comment "kube-system/kube-dns cluster IP" -j KUBE-SVC-TEST2
COMMIT
# Completed on Mon Jul 15 10:00:00 2025
END
        ;;
    *)
        echo "Mock iptables-save: unknown args: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock iptables-save with fallback cluster IP pattern
    cat > "$cmd_dir/iptables-save-fallback-pattern" <<'EOF'
#!/bin/bash
# Mock iptables-save with fallback pattern (any kube-dns cluster IP)
case "$*" in
    *"-t nat"*)
        cat <<END
# Generated by iptables-save v1.8.4 on Mon Jul 15 10:00:00 2025
*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]
:KUBE-SERVICES - [0:0]
-A PREROUTING -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A OUTPUT -m comment --comment "kubernetes service portals" -j KUBE-SERVICES
-A KUBE-SERVICES -d 10.0.0.25/32 -p udp -m comment --comment "kube-system/kube-dns:dns cluster IP" -j KUBE-SVC-TEST
-A KUBE-SERVICES -d 10.0.0.30/32 -p tcp -m comment --comment "kube-system/kube-dns cluster IP" -j KUBE-SVC-TEST2
COMMIT
# Completed on Mon Jul 15 10:00:00 2025
END
        ;;
    *)
        echo "Mock iptables-save: unknown args: $*" >&2
        exit 1
        ;;
esac
EOF

    # Create dispatchers based on scenario environment variables
    cat > "$cmd_dir/dig" <<'EOF'
#!/bin/bash
# dig dispatcher for DNS CoreDNS tests
SCENARIO="${DIG_SCENARIO:-success}"

# Handle special scenarios
case "$SCENARIO" in
    "timeout")
        exec "/mock-commands/dns-coredns/dig-timeout" "$@"
        ;;
    "retry-success")
        exec "/mock-commands/dns-coredns/dig-retry-success" "$@"
        ;;
    *)
        # Look for scenario-specific mock
        if [ -f "/mock-commands/dns-coredns/dig-$SCENARIO" ]; then
            exec "/mock-commands/dns-coredns/dig-$SCENARIO" "$@"
        fi
        ;;
esac

# Default to success scenario
exec "/mock-commands/dns-coredns/dig-success" "$@"
EOF

    cat > "$cmd_dir/iptables-save" <<'EOF'
#!/bin/bash
# iptables-save dispatcher for CoreDNS tests
SCENARIO="${IPTABLES_SCENARIO:-success}"

# Find the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Look for scenario-specific mock
if [ -f "$SCRIPT_DIR/iptables-save-$SCENARIO" ]; then
    exec "$SCRIPT_DIR/iptables-save-$SCENARIO" "$@"
fi

# Default to success scenario
exec "$SCRIPT_DIR/iptables-save-success" "$@"
EOF

    # Mock command-not-found scenario for dependency checks
    cat > "$cmd_dir/command-not-found" <<'EOF'
#!/bin/bash
# Mock for missing command scenario
echo "command not found: $1" >&2
exit 127
EOF

    # Create test wrapper script for get_vnet_dns_ips function
    cat > "$cmd_dir/test_get_vnet_dns_ips" <<'EOF'
#!/bin/bash
# Test wrapper script for get_vnet_dns_ips function
set -uo pipefail

# Source the common DNS functions
SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "/etc/node-problem-detector.d/plugin/check_dns_common.sh"

# Call the get_vnet_dns_ips function and output results
get_vnet_dns_ips
exit 0
EOF

    # Create test wrapper script for get_coredns_ip function
    cat > "$cmd_dir/test_get_coredns_ip_wrapper" <<'EOF'
#!/bin/bash
# Test wrapper script for get_coredns_ip function with dynamic kubelet mock creation
set -uo pipefail

# Function to create default kubelet mock files
create_default_kubelet_mock() {
    local cluster_dns_ip="${1:-10.0.0.10}"
    local temp_kubelet_file="/tmp/test_kubelet_$$.conf"
    local kubelet_dir="/tmp/etc_default_$$"
    
    # Create temporary directory structure
    mkdir -p "$kubelet_dir"
    
    # Create kubelet configuration with specified or default cluster DNS
    cat > "$temp_kubelet_file" <<KUBELET_EOF
KUBELET_FLAGS=--address=0.0.0.0 --cluster-dns=$cluster_dns_ip --cluster-domain=cluster.local --kubeconfig=/var/lib/kubelet/kubeconfig --pod-manifest-path=/etc/kubernetes/manifests
KUBELET_REGISTER_SCHEDULABLE=true
KUBELET_EOF
    
    # Copy to the temporary directory
    cp "$temp_kubelet_file" "$kubelet_dir/kubelet"
    
    # Create a symbolic link to the expected location if possible
    if [ -w "/etc" ] 2>/dev/null; then
        mkdir -p "/etc/default" 2>/dev/null || true
        ln -sf "$temp_kubelet_file" "/etc/default/kubelet" 2>/dev/null || cp "$temp_kubelet_file" "/etc/default/kubelet" 2>/dev/null || true
    fi
    
    # Export for cleanup
    export TEST_KUBELET_TEMP_FILE="$temp_kubelet_file"
    export TEST_KUBELET_DIR="$kubelet_dir"
}

# Function to set up mock data based on COREFILE_ROOT
setup_mock_data() {
    local corefile_root="${COREFILE_ROOT:-}"
    
    if [ -n "$corefile_root" ] && [ -d "$corefile_root" ]; then
        # Copy mock data to expected locations
        if [ -f "$corefile_root/opt/azure/containers/localdns/updated.localdns.corefile" ]; then
            mkdir -p "/opt/azure/containers/localdns"
            cp "$corefile_root/opt/azure/containers/localdns/updated.localdns.corefile" "/opt/azure/containers/localdns/updated.localdns.corefile" 2>/dev/null || true
        fi
        
        # Copy kubelet config if it exists in mock data
        if [ -f "$corefile_root/etc/default/kubelet" ]; then
            mkdir -p "/etc/default" 2>/dev/null || true
            cp "$corefile_root/etc/default/kubelet" "/etc/default/kubelet" 2>/dev/null || true
            export KUBELET_FROM_MOCK=true
        fi
    fi
}

# Function to determine cluster DNS IP from test scenario
get_expected_cluster_dns() {
    local scenario="${IPTABLES_SCENARIO:-success}"
    
    # Map scenarios to expected cluster DNS IPs for kubelet fallback
    case "$scenario" in
        "no-coredns")
            echo "10.0.0.20"  # For kubelet flags fallback test
            ;;
        *)
            echo "10.0.0.10"  # Default
            ;;
    esac
}

# Find the script directory and repository root
find_script_paths() {
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local repo_root=""
    
    # Try to find repository root by looking for the config directory
    local current_dir="$script_dir"
    while [ "$current_dir" != "/" ]; do
        if [ -f "$current_dir/config/node-problem-detector/plugin/check_dns_common.sh" ]; then
            repo_root="$current_dir"
            break
        fi
        current_dir=$(dirname "$current_dir")
    done
    
    echo "$script_dir|$repo_root"
}

# Main execution
main() {
    # Set up mock data first
    setup_mock_data
    
    # Determine cluster DNS IP based on scenario
    local cluster_dns_ip
    cluster_dns_ip=$(get_expected_cluster_dns)
    
    # Create kubelet mock only if not already provided by mock data
    if [ -z "${KUBELET_FROM_MOCK:-}" ]; then
        create_default_kubelet_mock "$cluster_dns_ip"
    fi
    
    # Find script paths
    local paths
    paths=$(find_script_paths)
    local script_dir="${paths%|*}"
    local repo_root="${paths#*|}"
    
    # Source the common DNS functions - try multiple locations
    local dns_common_script=""
    
    # Try Docker container paths first
    if [ -f "/etc/node-problem-detector.d/plugin/check_dns_common.sh" ]; then
        dns_common_script="/etc/node-problem-detector.d/plugin/check_dns_common.sh"
    elif [ -f "/config/node-problem-detector/plugin/check_dns_common.sh" ]; then
        dns_common_script="/config/node-problem-detector/plugin/check_dns_common.sh"
    # Try host paths relative to repository root
    elif [ -n "$repo_root" ] && [ -f "$repo_root/config/node-problem-detector/plugin/check_dns_common.sh" ]; then
        dns_common_script="$repo_root/config/node-problem-detector/plugin/check_dns_common.sh"
    else
        echo "Error: Could not find check_dns_common.sh" >&2
        echo "Debug: script_dir=$script_dir, repo_root=$repo_root" >&2
        exit 1
    fi
    
    source "$dns_common_script"
    
    # Call the get_coredns_ip function and output results
    get_coredns_ip
    local exit_code=$?
    
    # Cleanup
    rm -f "${TEST_KUBELET_TEMP_FILE:-}"
    rm -rf "${TEST_KUBELET_DIR:-}"
    rm -f "/etc/default/kubelet" 2>/dev/null || true
    rm -f "/opt/azure/containers/localdns/updated.localdns.corefile" 2>/dev/null || true
    rm -rf "/opt/azure/containers" 2>/dev/null || true
    unset TEST_KUBELET_TEMP_FILE TEST_KUBELET_DIR KUBELET_FROM_MOCK
    
    exit $exit_code
}

# Execute main function
main "$@"
EOF

    # Make all mock commands executable
    chmod +x "$cmd_dir"/*
}

# Create mock data for dependency missing scenarios
create_dependency_missing() {
    echo "Create dependency missing scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/dependency-missing"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/etc"
    
    # Create empty resolv.conf 
    cat > "$dir/etc/resolv.conf" <<'EOF'
# Empty resolv.conf - no nameservers  
EOF
    
    # Mock kubeconfig
    cat > "$dir/var/lib/kubelet/kubeconfig" <<'EOF'
apiVersion: v1
clusters:
- cluster:
    server: https://test-cluster-api.hcp.eastus.azmk8s.io:443
  name: test-cluster
kind: Config
EOF
}

# Create scenario with no CoreDNS IP discovery methods available
create_no_coredns_ip() {
    echo "Create no CoreDNS IP scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/no-coredns-ip"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/etc/default" "$dir/run/systemd/resolve"
    
    # No LocalDNS corefile
    # No kubelet flags file
    # Empty systemd resolv.conf
    cat > "$dir/run/systemd/resolve/resolv.conf" <<'EOF'
# No nameservers  
search default.svc.cluster.local svc.cluster.local cluster.local
options ndots:5
EOF
    
    # Mock kubeconfig
    cat > "$dir/var/lib/kubelet/kubeconfig" <<'EOF'
apiVersion: v1
clusters:
- cluster:
    server: https://test-cluster-api.hcp.eastus.azmk8s.io:443
  name: test-cluster
kind: Config
EOF
}

# Create error scenarios for edge case testing
create_error_scenarios() {
    echo "Create various CoreDNS error scenario mock data"
    
    # Scenario: Multiple CoreDNS IPs from different sources
    local multiple_sources_dir="$SCRIPT_DIR/mock-data/coredns-multiple-sources"
    mkdir -p "$multiple_sources_dir/opt/azure/containers/localdns" "$multiple_sources_dir/var/lib/kubelet" "$multiple_sources_dir/etc/default"
    
    # LocalDNS corefile with one CoreDNS IP
    cat > "$multiple_sources_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
cluster.local:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.12 {
        force_tcp
        policy sequential
    }
    cache 3600
    loop
    nsid localdns-pod
}
EOF

    # Kubelet flags with different CoreDNS IP
    cat > "$multiple_sources_dir/etc/default/kubelet" <<'EOF'
KUBELET_FLAGS=--cluster-dns=10.0.0.13 --cluster-domain=cluster.local --kubeconfig=/var/lib/kubelet/kubeconfig
KUBELET_REGISTER_SCHEDULABLE=true
EOF

    # Mock kubeconfig
    cat > "$multiple_sources_dir/var/lib/kubelet/kubeconfig" <<'EOF'
apiVersion: v1
clusters:
- cluster:
    server: https://test-cluster-api.hcp.eastus.azmk8s.io:443
  name: test-cluster
kind: Config
EOF
}

# Create test scenario for get_coredns_ip function testing
create_coredns_ip_test_scenarios() {
    echo "Create get_coredns_ip function test scenarios"
    
    # Scenario: Kubelet flags only (no LocalDNS, no iptables)
    local kubelet_only_dir="$SCRIPT_DIR/mock-data/coredns-kubelet-only"
    mkdir -p "$kubelet_only_dir/var/lib/kubelet" "$kubelet_only_dir/etc/default"
    
    cat > "$kubelet_only_dir/etc/default/kubelet" <<'EOF'
KUBELET_FLAGS=--kubeconfig=/var/lib/kubelet/kubeconfig --cluster-dns=10.0.0.20 --cluster-domain=cluster.local --node-labels=kubernetes.io/role=agent
KUBELET_REGISTER_SCHEDULABLE=true
EOF

    # Scenario: Different corefile format variations
    local corefile_variants_dir="$SCRIPT_DIR/mock-data/coredns-corefile-variants"
    mkdir -p "$corefile_variants_dir/opt/azure/containers/localdns" "$corefile_variants_dir/var/lib/kubelet"
    
    cat > "$corefile_variants_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
# Variant corefile format with different spacing and structure
cluster.local:53{
    errors
    bind 169.254.10.11
    forward . 10.0.0.25{
        force_tcp
        policy sequential
    }
    cache 3600
    loop
}
EOF

    # Scenario: Fallback to default IP (all discovery methods fail)
    local fallback_dir="$SCRIPT_DIR/mock-data/coredns-fallback"
    mkdir -p "$fallback_dir/var/lib/kubelet" "$fallback_dir/etc"
    
    cat > "$fallback_dir/etc/resolv.conf" <<'EOF'
# Fallback resolv.conf
search cluster.local
options ndots:5
EOF

    # Create additional test scenarios for get_coredns_ip function
    
    # Scenario: Real AKS iptables format
    local real_aks_iptables_dir="$SCRIPT_DIR/mock-data/coredns-real-aks-iptables"
    mkdir -p "$real_aks_iptables_dir/var/lib/kubelet"
    
    # Scenario: Edge case IP formats for regex testing
    local edge_cases_dir="$SCRIPT_DIR/mock-data/coredns-edge-cases"
    mkdir -p "$edge_cases_dir/var/lib/kubelet"
    
    # Scenario: Fallback pattern testing (any kube-dns cluster IP)
    local fallback_pattern_dir="$SCRIPT_DIR/mock-data/coredns-fallback-pattern"
    mkdir -p "$fallback_pattern_dir/var/lib/kubelet"
}

# Create test scenarios for get_vnet_dns_ips function testing
create_vnet_dns_ips_test_scenarios() {
    echo "Create get_vnet_dns_ips function test scenarios"
    
    # Scenario 1: Basic VNet DNS extraction from LocalDNS
    local basic_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-basic"
    mkdir -p "$basic_dir/opt/azure/containers/localdns"
    
    cat > "$basic_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
.:53 {
    errors
    bind 169.254.10.10
    forward . 168.63.129.16 {
        policy sequential
    }
    cache 3600
    loop
}
EOF

    # Scenario 2: Multiple VNet DNS IPs
    local multiple_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-multiple"
    mkdir -p "$multiple_dir/opt/azure/containers/localdns"
    
    cat > "$multiple_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
.:53 {
    errors
    bind 169.254.10.10
    forward . 168.63.129.16 8.8.8.8 {
        policy sequential
    }
    cache 3600
    loop
}
EOF

    # Scenario 3: Comma separated VNet DNS IPs
    local comma_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-comma"
    mkdir -p "$comma_dir/opt/azure/containers/localdns"
    
    cat > "$comma_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
.:53 {
    errors
    bind 169.254.10.10
    forward . 168.63.129.16,8.8.8.8,1.1.1.1 {
        policy sequential
    }
    cache 3600
    loop
}
EOF

    # Scenario 4: Complex corefile with multiple zones (only VNet should match)
    local complex_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-complex"
    mkdir -p "$complex_dir/opt/azure/containers/localdns"
    
    cat > "$complex_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
.:53 {
    errors
    bind 169.254.10.10
    forward . 168.63.129.16 8.8.4.4 {
        policy sequential
    }
    cache 3600
    loop
}
cluster.local:53 {
    errors
    bind 169.254.10.10
    forward . 10.0.0.10 {
        force_tcp
    }
    cache 3600
    loop
}
.:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
        policy sequential
    }
    cache 3600
    loop
}
EOF

    # Scenario 5: No VNet DNS section (bind to 169.254.10.11 only)
    local no_vnet_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-no-vnet"
    mkdir -p "$no_vnet_dir/opt/azure/containers/localdns"
    
    cat > "$no_vnet_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
.:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
        policy sequential
    }
    cache 3600
    loop
}
cluster.local:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
        force_tcp
    }
    cache 3600
    loop
}
EOF

    # Scenario 6: Malformed corefile
    local malformed_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-malformed"
    mkdir -p "$malformed_dir/opt/azure/containers/localdns"
    
    cat > "$malformed_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
.:53 {
    errors
    bind 169.254.10.10
    forward . 168.63.129.16 {
        policy sequential
    }
    # Missing closing brace - malformed
cluster.local:53 {
    errors
}
EOF

    # Scenario 7: Different formatting and spacing
    local formatting_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-formatting"
    mkdir -p "$formatting_dir/opt/azure/containers/localdns"
    
    cat > "$formatting_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
. : 53 {
    errors
    bind    169.254.10.10
    forward   .   168.63.129.16    8.8.8.8   {
        policy sequential
    }
    cache 3600
    loop
}
EOF

    # Scenario 8: No LocalDNS - systemd resolv.conf
    local no_localdns_systemd_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-no-localdns-systemd"
    mkdir -p "$no_localdns_systemd_dir/run/systemd/resolve"
    
    cat > "$no_localdns_systemd_dir/run/systemd/resolve/resolv.conf" <<'EOF'
nameserver 168.63.129.16
nameserver 8.8.8.8
search default.svc.cluster.local svc.cluster.local cluster.local
options ndots:5
EOF

    # Scenario 9: No LocalDNS - fallback to /etc/resolv.conf
    local no_localdns_fallback_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-no-localdns-fallback"
    mkdir -p "$no_localdns_fallback_dir/etc"
    
    # Create empty systemd resolv.conf to trigger fallback
    mkdir -p "$no_localdns_fallback_dir/run/systemd/resolve"
    touch "$no_localdns_fallback_dir/run/systemd/resolve/resolv.conf"
    
    cat > "$no_localdns_fallback_dir/etc/resolv.conf" <<'EOF'
nameserver 1.1.1.1
nameserver 8.8.4.4
search cluster.local
options ndots:2
EOF

    # Scenario 10: Empty resolv.conf files
    local empty_resolv_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-empty-resolv"
    mkdir -p "$empty_resolv_dir/run/systemd/resolve" "$empty_resolv_dir/etc"
    
    # Create empty files
    touch "$empty_resolv_dir/run/systemd/resolve/resolv.conf"
    touch "$empty_resolv_dir/etc/resolv.conf"

    # Scenario 11: IPv6 addresses mixed with IPv4 (should filter out IPv6)
    local ipv6_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-ipv6"
    mkdir -p "$ipv6_dir/opt/azure/containers/localdns"
    
    cat > "$ipv6_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
.:53 {
    errors
    bind 169.254.10.10
    forward . 168.63.129.16 2001:4860:4860::8888 {
        policy sequential
    }
    cache 3600
    loop
}
EOF

    # Scenario 12: Real AKS format (from existing healthy scenario)
    local real_aks_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-real-aks"
    mkdir -p "$real_aks_dir/opt/azure/containers/localdns"
    
    # Copy from the existing healthy scenario but focus on VNet section
    cat > "$real_aks_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
# VnetDNS overrides apply to DNS traffic from pods with dnsPolicy:default or kubelet (referred to as VnetDNS traffic).
.:53 {
    errors
    bind 169.254.10.10
    forward . 168.63.129.16 {
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s immediate
        servfail 0
    }
    loop
    nsid localdns
    prometheus :9253
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
}
# KubeDNS overrides apply to DNS traffic from pods with dnsPolicy:ClusterFirst (referred to as KubeDNS traffic).
.:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.11:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s immediate
        servfail 0
    }
    loop
    nsid localdns-pod
    prometheus :9253
}
EOF

    # Scenario 13: Multiple root zones with different binds
    local multiple_roots_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-multiple-roots"
    mkdir -p "$multiple_roots_dir/opt/azure/containers/localdns"
    
    cat > "$multiple_roots_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
.:53 {
    errors
    bind 169.254.10.10
    forward . 168.63.129.16 {
        policy sequential
    }
    cache 3600
    loop
}
.:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
        policy sequential
    }
    cache 3600
    loop
}
.:53 {
    errors
    bind 169.254.10.12
    forward . 8.8.8.8 {
        policy sequential
    }
    cache 3600
    loop
}
EOF

    # Scenario 14: Duplicate IPs test
    local duplicates_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-duplicates"
    mkdir -p "$duplicates_dir/opt/azure/containers/localdns"
    
    cat > "$duplicates_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
.:53 {
    errors
    bind 169.254.10.10
    forward . 168.63.129.16 8.8.8.8 168.63.129.16 8.8.8.8 {
        policy sequential
    }
    cache 3600
    loop
}
EOF

    # Scenario 15: No VNet bind (has bind but not to 169.254.10.10)
    local no_vnet_bind_dir="$SCRIPT_DIR/mock-data/vnet-dns-ips-no-vnet-bind"
    mkdir -p "$no_vnet_bind_dir/opt/azure/containers/localdns"
    
    cat > "$no_vnet_bind_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
.:53 {
    errors
    bind 192.168.1.10
    forward . 168.63.129.16 {
        policy sequential
    }
    cache 3600
    loop
}
EOF
}

# Copy kubeconfig to all scenario directories that need it
copy_kubeconfigs() {
    local base_kubeconfig="$SCRIPT_DIR/mock-data/coredns-healthy/var/lib/kubelet/kubeconfig"
    local dirs=("coredns-multiple-sources" "kubelet-only" "corefile-variants" "fallback")
    
    for dir in "${dirs[@]}"; do
        if [ -d "$SCRIPT_DIR/mock-data/$dir/var/lib/kubelet" ]; then
            cp "$base_kubeconfig" "$SCRIPT_DIR/mock-data/$dir/var/lib/kubelet/kubeconfig"
        fi
    done
}

# Run all setup functions
create_coredns_healthy
create_coredns_no_localdns
create_dependency_missing
create_no_coredns_ip
create_error_scenarios
create_coredns_ip_test_scenarios
create_vnet_dns_ips_test_scenarios
create_dns_coredns_mock_commands
copy_kubeconfigs

echo "CoreDNS mock data creation complete"