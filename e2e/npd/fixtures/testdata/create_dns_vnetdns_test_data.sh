#!/bin/bash
# test/node-problem-detector/fixtures/testdata/create_dns_vnetdns_test_data.sh
# Creates test data for VNet DNS monitoring script: check_dns_to_vnetdns.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Create mock data for Azure DNS (VNet DNS) monitoring - healthy scenario
create_azuredns_healthy() {
    echo "Create healthy Azure DNS scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/azuredns-healthy"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/opt/azure/containers/localdns" "$dir/run/systemd/resolve"
    
    # Create VNet DNS mock data - Real LocalDNS corefile format
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

    # Mock systemd resolved config as fallback
    cat > "$dir/run/systemd/resolve/resolv.conf" <<'EOF'
nameserver 168.63.129.16
nameserver 8.8.8.8
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

# Create mock data for Azure DNS - LocalDNS disabled scenario
create_azuredns_no_localdns() {
    echo "Create Azure DNS no LocalDNS scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/azuredns-no-localdns"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/run/systemd/resolve"
    
    # No LocalDNS corefile present
    
    # Mock systemd resolved config - primary source when LocalDNS is disabled
    cat > "$dir/run/systemd/resolve/resolv.conf" <<'EOF'
nameserver 168.63.129.16
nameserver 8.8.8.8
search default.svc.cluster.local svc.cluster.local cluster.local
options ndots:5
EOF

    # Mock kubeconfig
    cp "$SCRIPT_DIR/mock-data/azuredns-healthy/var/lib/kubelet/kubeconfig" "$dir/var/lib/kubelet/kubeconfig"
}

# Create mock commands for DNS vnetdns
create_dns_vnetdns_mock_commands() {
    echo "Create DNS vnetdns mock commands"
    local cmd_dir="$SCRIPT_DIR/mock-commands/dns-vnetdns"
    mkdir -p "$cmd_dir"
    
    # Mock dig command - successful resolution
    cat > "$cmd_dir/dig-success" <<'EOF'
#!/bin/bash
# Mock dig for successful DNS resolution
case "$*" in
    *mcr.microsoft.com*|*kubernetes.default.svc.cluster.local*|*example.microsoft.com*)
        # Simulate successful DNS resolution
        cat <<END
; <<>> DiG 9.16.1 <<>> mcr.microsoft.com @168.63.129.16
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 12345
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; QUESTION SECTION:
;mcr.microsoft.com.             IN      A

;; ANSWER SECTION:
mcr.microsoft.com.      300     IN      A       20.118.95.41

;; Query time: 5 msec
;; SERVER: 168.63.129.16#53(168.63.129.16)
;; WHEN: Mon Jul 15 10:00:00 UTC 2025
;; MSG SIZE  rcvd: 62
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
exec /mock-commands/dns-vnetdns/command-not-found dig
EOF

    # Mock dig command - DNS resolution failure
    cat > "$cmd_dir/dig-failure" <<'EOF'
#!/bin/bash
# Mock dig for DNS resolution failure
case "$*" in
    *mcr.microsoft.com*|*kubernetes.default.svc.cluster.local*|*example.microsoft.com*)
        # Simulate DNS resolution failure
        cat <<END >&2
; <<>> DiG 9.16.1 <<>> mcr.microsoft.com @168.63.129.16
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: SERVFAIL, id: 12345
;; flags: qr rd ra; QUERY: 1, ANSWER: 0, AUTHORITY: 0, ADDITIONAL: 1

;; QUESTION SECTION:
;mcr.microsoft.com.             IN      A

;; Query time: 5000 msec
;; SERVER: 168.63.129.16#53(168.63.129.16)
;; WHEN: Mon Jul 15 10:00:00 UTC 2025
;; MSG SIZE  rcvd: 43
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
sleep 10  # This will be interrupted by timeout
exit 9
EOF

    # Mock dig command - success with retry
    cat > "$cmd_dir/dig-retry-success" <<'EOF'
#!/bin/bash
# Mock dig that fails first few times then succeeds
RETRY_FILE="/tmp/dig_retry_count_$$"
if [ -f "$RETRY_FILE" ]; then
    count=$(cat "$RETRY_FILE")
else
    count=0
fi
count=$((count + 1))
echo "$count" > "$RETRY_FILE"

if [ "$count" -lt 3 ]; then
    # Fail first 2 attempts
    echo "Temporary failure" >&2
    exit 9
else
    # Succeed on 3rd attempt
    rm -f "$RETRY_FILE"
    case "$*" in
        *mcr.microsoft.com*|*kubernetes.default.svc.cluster.local*|*example.microsoft.com*)
            cat <<END
; <<>> DiG 9.16.1 <<>> mcr.microsoft.com @168.63.129.16
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 12345
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; QUESTION SECTION:
;mcr.microsoft.com.             IN      A

;; ANSWER SECTION:
mcr.microsoft.com.      300     IN      A       20.118.95.41

;; Query time: 5 msec
;; SERVER: 168.63.129.16#53(168.63.129.16)
;; WHEN: Mon Jul 15 10:00:00 UTC 2025
;; MSG SIZE  rcvd: 62
END
            exit 0
            ;;
    esac
fi
EOF

    # Mock hostname command
    cat > "$cmd_dir/hostname" <<'EOF'
#!/bin/bash
# Mock hostname command
echo "aks-test-node-12345"
EOF

    # Mock kubectl for LocalDNS state label queries - enabled
    cat > "$cmd_dir/kubectl-localdns-enabled" <<'EOF'
#!/bin/bash
# Mock kubectl for LocalDNS enabled scenario
case "$*" in
    *"get node"*"jsonpath"*)
        echo "enabled"
        ;;
    *)
        echo "Mock kubectl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock kubectl for LocalDNS state label queries - disabled
    cat > "$cmd_dir/kubectl-localdns-disabled" <<'EOF'
#!/bin/bash
# Mock kubectl for LocalDNS disabled scenario
case "$*" in
    *"get node"*"jsonpath"*)
        echo "disabled"
        ;;
    *)
        echo "Mock kubectl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock kubectl for RBAC forbidden scenario
    cat > "$cmd_dir/kubectl-rbac-forbidden" <<'EOF'
#!/bin/bash
# Mock kubectl for RBAC forbidden scenario
case "$*" in
    *"get node"*)
        echo 'Error from server (Forbidden): nodes is forbidden: User "system:node:aks-test-node" cannot get resource "nodes" in API group "" at the cluster scope' >&2
        exit 1
        ;;
    *)
        echo "Mock kubectl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock systemctl for LocalDNS service checks - active
    cat > "$cmd_dir/systemctl-active" <<'EOF'
#!/bin/bash
# Mock systemctl for active LocalDNS service
case "$*" in
    *"is-active"*"localdns.service"*)
        echo "active"
        exit 0
        ;;
    *)
        echo "Mock systemctl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock systemctl for LocalDNS service checks - inactive
    cat > "$cmd_dir/systemctl-inactive" <<'EOF'
#!/bin/bash
# Mock systemctl for inactive LocalDNS service
case "$*" in
    *"is-active"*"localdns.service"*)
        echo "inactive"
        exit 3
        ;;
    *)
        echo "Mock systemctl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock systemctl that times out
    cat > "$cmd_dir/systemctl-timeout" <<'EOF'
#!/bin/bash
# Mock systemctl that simulates timeout
sleep 30  # This will be interrupted by timeout command
EOF


    # Create dispatchers based on scenario environment variables
    cat > "$cmd_dir/dig" <<'EOF'
#!/bin/bash
# dig dispatcher for DNS vnetdns tests
SCENARIO="${DIG_SCENARIO:-success}"

# Handle special scenarios
case "$SCENARIO" in
    "timeout")
        exec "/mock-commands/dns-vnetdns/dig-timeout" "$@"
        ;;
    "retry-success")
        exec "/mock-commands/dns-vnetdns/dig-retry-success" "$@"
        ;;
    *)
        # Look for scenario-specific mock
        if [ -f "/mock-commands/dns-vnetdns/dig-$SCENARIO" ]; then
            exec "/mock-commands/dns-vnetdns/dig-$SCENARIO" "$@"
        fi
        ;;
esac

# Default to success scenario
exec "/mock-commands/dns-vnetdns/dig-success" "$@"
EOF

    cat > "$cmd_dir/kubectl" <<'EOF'
#!/bin/bash
# kubectl dispatcher for DNS vnetdns tests
SCENARIO="${KUBECTL_SCENARIO:-localdns-enabled}"

# Look for scenario-specific mock
if [ -f "/mock-commands/dns-vnetdns/kubectl-$SCENARIO" ]; then
    exec "/mock-commands/dns-vnetdns/kubectl-$SCENARIO" "$@"
fi

# Default to LocalDNS enabled scenario
exec "/mock-commands/dns-vnetdns/kubectl-localdns-enabled" "$@"
EOF

    cat > "$cmd_dir/systemctl" <<'EOF'
#!/bin/bash
# systemctl dispatcher for DNS vnetdns tests
SCENARIO="${SYSTEMCTL_SCENARIO:-active}"

# Look for scenario-specific mock
if [ -f "/mock-commands/dns-vnetdns/systemctl-$SCENARIO" ]; then
    exec "/mock-commands/dns-vnetdns/systemctl-$SCENARIO" "$@"
fi

# Default to active scenario
exec "/mock-commands/dns-vnetdns/systemctl-active" "$@"
EOF

    # Mock command-not-found scenario for dependency checks
    cat > "$cmd_dir/command-not-found" <<'EOF'
#!/bin/bash
# Mock for missing command scenario
echo "command not found: $1" >&2
exit 127
EOF

    # Make all mock commands executable
    chmod +x "$cmd_dir"/*
    
    # Create symlinks for DNS scenario variations that map to existing commands
    ln -sf dig-failure "$cmd_dir/dig-dns-failure"
    ln -sf dig-failure "$cmd_dir/dig-dns-mixed-results"
    ln -sf dig-timeout "$cmd_dir/dig-dns-timeout"
    
    # Create top-level symlink for dig command to point to the dispatcher
    local parent_dir="$SCRIPT_DIR/mock-commands"
    mkdir -p "$parent_dir"
    ln -sf dns-vnetdns/dig "$parent_dir/dig"
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
    cp "$SCRIPT_DIR/mock-data/azuredns-healthy/var/lib/kubelet/kubeconfig" "$dir/var/lib/kubelet/kubeconfig"
}

# Create scenario with no VNet DNS IPs found
create_no_vnet_ips() {
    echo "Create no VNet DNS IPs scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/no-vnet-ips"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/etc" "$dir/run/systemd/resolve"
    
    # Create empty resolv.conf files that would result in no VNet DNS IPs
    cat > "$dir/etc/resolv.conf" <<'EOF'
# No nameservers
search default.svc.cluster.local svc.cluster.local cluster.local
options ndots:5
EOF

    cat > "$dir/run/systemd/resolve/resolv.conf" <<'EOF'
# No nameservers  
search default.svc.cluster.local svc.cluster.local cluster.local
options ndots:5
EOF
    
    # Mock kubeconfig
    cp "$SCRIPT_DIR/mock-data/azuredns-healthy/var/lib/kubelet/kubeconfig" "$dir/var/lib/kubelet/kubeconfig"
}

# Create error scenarios for edge case testing
create_error_scenarios() {
    echo "Create various error scenario mock data"
    
    # Scenario: Multiple VNet DNS IPs with mixed results
    local mixed_dir="$SCRIPT_DIR/mock-data/azuredns-mixed-results"
    mkdir -p "$mixed_dir/opt/azure/containers/localdns" "$mixed_dir/var/lib/kubelet"
    
    # LocalDNS corefile with multiple DNS servers in VNet section
    cat > "$mixed_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
# VnetDNS section with multiple forward IPs
.:53 {
    errors
    bind 169.254.10.10
    forward . 168.63.129.16 8.8.8.8 1.1.1.1 {
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600
    loop
    nsid localdns
}
cluster.local:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
        force_tcp
        policy sequential
    }
    cache 3600
    loop
    nsid localdns-pod
}
EOF

    cp "$SCRIPT_DIR/mock-data/azuredns-healthy/var/lib/kubelet/kubeconfig" "$mixed_dir/var/lib/kubelet/kubeconfig"
}

# Create test scenario for get_vnet_dns_ips function testing
create_vnet_dns_ips_test_scenarios() {
    echo "Create get_vnet_dns_ips function test scenarios"
    
    # Scenario: Real production LocalDNS corefile format
    local real_format_dir="$SCRIPT_DIR/mock-data/azuredns-real-format"
    mkdir -p "$real_format_dir/opt/azure/containers/localdns" "$real_format_dir/var/lib/kubelet"
    
    cat > "$real_format_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
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

    cp "$SCRIPT_DIR/mock-data/azuredns-healthy/var/lib/kubelet/kubeconfig" "$real_format_dir/var/lib/kubelet/kubeconfig"
    
    # Scenario: Only KubeDNS bind (no VNet DNS bind)
    local kubedns_only_dir="$SCRIPT_DIR/mock-data/kubedns-only"
    mkdir -p "$kubedns_only_dir/opt/azure/containers/localdns" "$kubedns_only_dir/var/lib/kubelet"
    
    cat > "$kubedns_only_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
# Only KubeDNS section - no VNet DNS bind
.:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
        policy sequential
    }
    cache 3600
    loop
    nsid localdns-pod
}
cluster.local:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
        force_tcp
        policy sequential
    }
    cache 3600
    loop
    nsid localdns-pod
}
EOF
    
    # Scenario: Malformed corefile with syntax errors
    local malformed_dir="$SCRIPT_DIR/mock-data/malformed-corefile"
    mkdir -p "$malformed_dir/opt/azure/containers/localdns" "$malformed_dir/var/lib/kubelet"
    
    cat > "$malformed_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
# Malformed corefile with missing closing braces and syntax errors
.:53 {
    bind 169.254.10.10
    forward . 168.63.129.16
    # Missing closing brace

cluster.local:53 
    bind 169.254.10.11
    forward . 10.0.0.10
}
EOF

    # Scenario: Very large corefile with many sections
    local large_corefile_dir="$SCRIPT_DIR/mock-data/large-corefile"
    mkdir -p "$large_corefile_dir/opt/azure/containers/localdns" "$large_corefile_dir/var/lib/kubelet"
    
    cat > "$large_corefile_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
# Large corefile with multiple VNet DNS sections and many rules
example1.com:53 {
    bind 169.254.10.11
    forward . 8.8.8.8
}

.:53 {
    bind 169.254.10.10
    forward . 168.63.129.16 8.8.8.8 1.1.1.1 {
        policy sequential
    }
    cache 30
}

example2.com:53 {
    bind 169.254.10.11  
    forward . 1.1.1.1
}

.:53 {
    bind 169.254.10.11
    forward . 10.0.0.10 {
        policy random
    }
    cache 60
}

cluster.local:53 {
    bind 169.254.10.10
    forward . 10.0.0.10
}
EOF

    # Scenario: Empty corefile
    local empty_corefile_dir="$SCRIPT_DIR/mock-data/empty-corefile"  
    mkdir -p "$empty_corefile_dir/opt/azure/containers/localdns" "$empty_corefile_dir/var/lib/kubelet"
    
    cat > "$empty_corefile_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
# Empty corefile with only comments
# No actual DNS configuration
EOF

    # Scenario: IPv6 DNS servers mixed with IPv4
    local ipv6_mixed_dir="$SCRIPT_DIR/mock-data/ipv6-mixed"
    mkdir -p "$ipv6_mixed_dir/opt/azure/containers/localdns" "$ipv6_mixed_dir/var/lib/kubelet"
    
    cat > "$ipv6_mixed_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
.:53 {
    bind 169.254.10.10
    forward . 168.63.129.16 2001:4860:4860::8888 8.8.8.8 {
        policy sequential
    }
    cache 30
}
EOF

    # Scenario: Comma-separated DNS servers 
    local comma_separated_dir="$SCRIPT_DIR/mock-data/comma-separated"
    mkdir -p "$comma_separated_dir/opt/azure/containers/localdns" "$comma_separated_dir/var/lib/kubelet"
    
    cat > "$comma_separated_dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
.:53 {
    bind 169.254.10.10
    forward . 168.63.129.16,8.8.8.8,1.1.1.1 {
        policy sequential
    }
    cache 30
}
EOF
}

# Copy kubeconfig to all scenario directories that need it
copy_kubeconfigs() {
    local base_kubeconfig="$SCRIPT_DIR/mock-data/azuredns-healthy/var/lib/kubelet/kubeconfig"
    local dirs=("azuredns-real-format" "kubedns-only" "malformed-corefile" "large-corefile" "empty-corefile" "ipv6-mixed" "comma-separated")
    
    for dir in "${dirs[@]}"; do
        cp "$base_kubeconfig" "$SCRIPT_DIR/mock-data/$dir/var/lib/kubelet/kubeconfig"
    done
}

# Run all setup functions
create_azuredns_healthy
create_azuredns_no_localdns
create_dependency_missing
create_no_vnet_ips
create_error_scenarios
create_vnet_dns_ips_test_scenarios
create_dns_vnetdns_mock_commands
copy_kubeconfigs

echo "VNet DNS mock data creation complete"