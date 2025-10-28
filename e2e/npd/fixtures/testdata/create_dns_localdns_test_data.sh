#!/bin/bash
# test/node-problem-detector/fixtures/testdata/create_dns_localdns_test_data.sh
# Creates test data for LocalDNS monitoring script: check_dns_to_localdns.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Create mock data for LocalDNS monitoring - healthy scenario
create_localdns_healthy() {
    echo "Create healthy LocalDNS scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/localdns-healthy"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/opt/azure/containers/localdns" "$dir/run/systemd/resolve" "$dir/etc/default"
    
    # Create LocalDNS corefile with full configuration
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

    # Mock kubelet default flags with LocalDNS enabled
    cat > "$dir/etc/default/kubelet" <<'EOF'
KUBELET_FLAGS=--address=0.0.0.0 --anonymous-auth=false --authentication-token-webhook=true --authorization-mode=Webhook --cgroups-per-qos=true --client-ca-file=/etc/kubernetes/certs/ca.crt --cloud-config=/etc/kubernetes/azure.json --cloud-provider=external --cluster-dns=169.254.10.11 --cluster-domain=cluster.local --container-log-max-size=50M --enforce-node-allocatable=pods --event-qps=0 --eviction-hard=memory.available<100Mi,nodefs.available<10%,nodefs.inodesFree<5%,pid.available<2000 --feature-gates=RotateKubeletServerCertificate=true --image-credential-provider-bin-dir=/var/lib/kubelet/credential-provider --image-credential-provider-config=/var/lib/kubelet/credential-provider-config.yaml --image-gc-high-threshold=85 --image-gc-low-threshold=80 --kube-reserved=cpu=180m,memory=2250Mi,pid=1000 --kubeconfig=/var/lib/kubelet/kubeconfig --max-pods=110 --node-status-update-frequency=10s --pod-infra-container-image=mcr.microsoft.com/oss/kubernetes/pause:3.6 --pod-manifest-path=/etc/kubernetes/manifests --protect-kernel-defaults=true --read-only-port=0 --rotate-certificates=true --rotate-server-certificates=true --serialize-image-pulls=false --streaming-connection-idle-timeout=4h --tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256 --node-ip=10.224.0.5
KUBELET_REGISTER_SCHEDULABLE=true
NETWORK_POLICY=
KUBELET_IMAGE=
KUBELET_NODE_LABELS=agentpool=userpool,kubernetes.azure.com/agentpool=userpool,kubernetes.azure.com/azure-cni-overlay=true,kubernetes.azure.com/cluster=MC_sharshalocaldnstest_sharshalocaldnstest_westcentralus,kubernetes.azure.com/consolidated-additional-properties=7f6ddd83-9d56-11f0-ac7c-12f27c461584,kubernetes.azure.com/kubelet-identity-client-id=7c5119c6-1590-44aa-a6a1-a29d343e6450,kubernetes.azure.com/localdns-state=enabled,kubernetes.azure.com/mode=user,kubernetes.azure.com/network-name=aks-vnet-10914146,kubernetes.azure.com/network-policy=none,kubernetes.azure.com/network-resourcegroup=sharshalocaldnstest,kubernetes.azure.com/network-stateless-cni=false,kubernetes.azure.com/network-subnet=aks-subnet,kubernetes.azure.com/network-subscription=26fe00f8-9173-4872-9134-bb1d2e00343a,kubernetes.azure.com/node-image-version=AKSUbuntu-2204gen2containerd-202509.23.0,kubernetes.azure.com/nodenetwork-vnetguid=e3a60ba6-324e-4da0-9dec-cfbd9a951cc4,kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/os-sku=Ubuntu,kubernetes.azure.com/os-sku-effective=Ubuntu2204,kubernetes.azure.com/os-sku-requested=Ubuntu,kubernetes.azure.com/podnetwork-type=overlay,kubernetes.azure.com/role=agent,kubernetes.azure.com/kubelet-serving-ca=cluster
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

# Create mock data for LocalDNS disabled scenario
create_localdns_disabled() {
    echo "Create LocalDNS disabled scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/localdns-disabled"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/run/systemd/resolve" "$dir/etc/default"
    
    # No LocalDNS corefile present
    
    # Mock systemd resolved config
    cat > "$dir/run/systemd/resolve/resolv.conf" <<'EOF'
nameserver 168.63.129.16
nameserver 8.8.8.8
search default.svc.cluster.local svc.cluster.local cluster.local
options ndots:5
EOF

    # Mock kubelet default flags with LocalDNS disabled
    cat > "$dir/etc/default/kubelet" <<'EOF'
KUBELET_FLAGS=--address=0.0.0.0 --cluster-dns=10.0.0.10 --cluster-domain=cluster.local --kubeconfig=/var/lib/kubelet/kubeconfig --pod-manifest-path=/etc/kubernetes/manifests
KUBELET_REGISTER_SCHEDULABLE=true
NETWORK_POLICY=
KUBELET_IMAGE=
KUBELET_NODE_LABELS=agentpool=userpool,kubernetes.azure.com/agentpool=userpool,kubernetes.azure.com/localdns-state=disabled,kubernetes.azure.com/mode=user,kubernetes.azure.com/role=agent
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

# Create mock data for LocalDNS service down scenario
create_localdns_service_down() {
    echo "Create LocalDNS service down scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/localdns-service-down"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/opt/azure/containers/localdns" "$dir/run/systemd/resolve" "$dir/etc/default"
    
    # Create LocalDNS corefile (exists but service is down)
    cat > "$dir/opt/azure/containers/localdns/updated.localdns.corefile" <<'EOF'
# LocalDNS corefile exists but service is down
.:53 {
    errors
    bind 169.254.10.10
    forward . 168.63.129.16
}
cluster.local:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10
}
EOF

    # Mock kubelet flags with LocalDNS enabled
    cat > "$dir/etc/default/kubelet" <<'EOF'
KUBELET_FLAGS=--cluster-dns=169.254.10.11 --cluster-domain=cluster.local --kubeconfig=/var/lib/kubelet/kubeconfig
KUBELET_REGISTER_SCHEDULABLE=true
KUBELET_NODE_LABELS=kubernetes.azure.com/localdns-state=enabled
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

# Create mock data for mismatch scenario (corefile missing but label enabled)
create_localdns_mismatch() {
    echo "Create LocalDNS mismatch scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/localdns-mismatch"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/run/systemd/resolve" "$dir/etc/default"
    
    # No LocalDNS corefile present
    
    # Mock kubelet flags with LocalDNS enabled in label but no corefile
    cat > "$dir/etc/default/kubelet" <<'EOF'
KUBELET_FLAGS=--cluster-dns=10.0.0.10 --cluster-domain=cluster.local --kubeconfig=/var/lib/kubelet/kubeconfig
KUBELET_REGISTER_SCHEDULABLE=true
KUBELET_NODE_LABELS=kubernetes.azure.com/localdns-state=enabled,kubernetes.azure.com/role=agent
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

# Create mock data for missing node labels scenario
create_localdns_no_labels() {
    echo "Create LocalDNS no labels scenario mock data"
    local dir="$SCRIPT_DIR/mock-data/localdns-no-labels"
    
    mkdir -p "$dir/proc/net" "$dir/var/lib/kubelet" "$dir/run/systemd/resolve" "$dir/etc/default"
    
    # No LocalDNS corefile present
    
    # Mock kubelet flags without KUBELET_NODE_LABELS
    cat > "$dir/etc/default/kubelet" <<'EOF'
KUBELET_FLAGS=--cluster-dns=10.0.0.10 --cluster-domain=cluster.local --kubeconfig=/var/lib/kubelet/kubeconfig
KUBELET_REGISTER_SCHEDULABLE=true
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

# Create mock commands for DNS LocalDNS
create_dns_localdns_mock_commands() {
    echo "Create DNS LocalDNS mock commands"
    local cmd_dir="$SCRIPT_DIR/mock-commands/dns-localdns"
    mkdir -p "$cmd_dir"
    
    # Mock dig command - successful resolution
    cat > "$cmd_dir/dig-success" <<'EOF'
#!/bin/bash
# Mock dig for successful DNS resolution
case "$*" in
    *kubernetes.default.svc.cluster.local*169.254.10.10*)
        # Simulate successful LocalDNS node listener resolution
        cat <<END
; <<>> DiG 9.16.1 <<>> kubernetes.default.svc.cluster.local @169.254.10.10
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 12345
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; QUESTION SECTION:
;kubernetes.default.svc.cluster.local. IN A

;; ANSWER SECTION:
kubernetes.default.svc.cluster.local. 30 IN A 10.0.0.1

;; Query time: 3 msec
;; SERVER: 169.254.10.10#53(169.254.10.10)
;; WHEN: Mon Jul 15 10:00:00 UTC 2025
;; MSG SIZE  rcvd: 82
END
        exit 0
        ;;
    *kubernetes.default.svc.cluster.local*169.254.10.11*)
        # Simulate successful LocalDNS cluster listener resolution
        cat <<END
; <<>> DiG 9.16.1 <<>> kubernetes.default.svc.cluster.local @169.254.10.11
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 12346
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; QUESTION SECTION:
;kubernetes.default.svc.cluster.local. IN A

;; ANSWER SECTION:
kubernetes.default.svc.cluster.local. 30 IN A 10.0.0.1

;; Query time: 2 msec
;; SERVER: 169.254.10.11#53(169.254.10.11)
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

    # Mock dig command - DNS resolution failure
    cat > "$cmd_dir/dig-failure" <<'EOF'
#!/bin/bash
# Mock dig for DNS resolution failure
case "$*" in
    *kubernetes.default.svc.cluster.local*)
        # Simulate DNS resolution failure
        cat <<END >&2
; <<>> DiG 9.16.1 <<>> kubernetes.default.svc.cluster.local @169.254.10.10
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: SERVFAIL, id: 12345
;; flags: qr rd ra; QUERY: 1, ANSWER: 0, AUTHORITY: 0, ADDITIONAL: 1

;; QUESTION SECTION:
;kubernetes.default.svc.cluster.local. IN A

;; Query time: 5000 msec
;; SERVER: 169.254.10.10#53(169.254.10.10)
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

    # Mock dig command - cluster listener failure only
    cat > "$cmd_dir/dig-cluster-fail" <<'EOF'
#!/bin/bash
# Mock dig that fails for cluster listener but succeeds for node listener
case "$*" in
    *kubernetes.default.svc.cluster.local*169.254.10.11*)
        # Simulate cluster listener failure
        cat <<END >&2
; <<>> DiG 9.16.1 <<>> kubernetes.default.svc.cluster.local @169.254.10.11
;; global options: +cmd
;; connection timed out; no servers could be reached
END
        exit 9
        ;;
    *kubernetes.default.svc.cluster.local*169.254.10.10*)
        # Simulate successful node listener resolution
        cat <<END
; <<>> DiG 9.16.1 <<>> kubernetes.default.svc.cluster.local @169.254.10.10
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 12345
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; QUESTION SECTION:
;kubernetes.default.svc.cluster.local. IN A

;; ANSWER SECTION:
kubernetes.default.svc.cluster.local. 30 IN A 10.0.0.1

;; Query time: 3 msec
;; SERVER: 169.254.10.10#53(169.254.10.10)
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

    # Mock dig command - node listener failure only
    cat > "$cmd_dir/dig-node-fail" <<'EOF'
#!/bin/bash
# Mock dig that fails for node listener but succeeds for cluster listener
case "$*" in
    *kubernetes.default.svc.cluster.local*169.254.10.10*)
        # Simulate node listener failure
        cat <<END >&2
; <<>> DiG 9.16.1 <<>> kubernetes.default.svc.cluster.local @169.254.10.10
;; global options: +cmd
;; connection timed out; no servers could be reached
END
        exit 9
        ;;
    *kubernetes.default.svc.cluster.local*169.254.10.11*)
        # Simulate successful cluster listener resolution
        cat <<END
; <<>> DiG 9.16.1 <<>> kubernetes.default.svc.cluster.local @169.254.10.11
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 12346
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; QUESTION SECTION:
;kubernetes.default.svc.cluster.local. IN A

;; ANSWER SECTION:
kubernetes.default.svc.cluster.local. 30 IN A 10.0.0.1

;; Query time: 2 msec
;; SERVER: 169.254.10.11#53(169.254.10.11)
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

    # Mock systemctl command - service active
    cat > "$cmd_dir/systemctl-active" <<'EOF'
#!/bin/bash
# Mock systemctl for active service
case "$*" in
    *"is-active localdns.service"*)
        echo "active"
        exit 0
        ;;
    *)
        echo "Mock systemctl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock systemctl command - service inactive
    cat > "$cmd_dir/systemctl-inactive" <<'EOF'
#!/bin/bash
# Mock systemctl for inactive service
case "$*" in
    *"is-active localdns.service"*)
        echo "inactive"
        exit 3
        ;;
    *)
        echo "Mock systemctl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock systemctl command - service failed
    cat > "$cmd_dir/systemctl-failed" <<'EOF'
#!/bin/bash
# Mock systemctl for failed service
case "$*" in
    *"is-active localdns.service"*)
        echo "failed"
        exit 3
        ;;
    *)
        echo "Mock systemctl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Mock systemctl command - timeout scenario
    cat > "$cmd_dir/systemctl-timeout" <<'EOF'
#!/bin/bash
# Mock systemctl that hangs (simulates timeout)
case "$*" in
    *"is-active localdns.service"*)
        # Simulate hanging command
        sleep 15
        echo "active"
        exit 0
        ;;
    *)
        echo "Mock systemctl: unknown command: $*" >&2
        exit 1
        ;;
esac
EOF

    # Create dispatchers based on scenario environment variables
    cat > "$cmd_dir/dig" <<'EOF'
#!/bin/bash
# dig dispatcher for DNS LocalDNS tests
SCENARIO="${DIG_SCENARIO:-success}"

# Look for scenario-specific mock
if [ -f "/mock-commands/dns-localdns/dig-$SCENARIO" ]; then
    exec "/mock-commands/dns-localdns/dig-$SCENARIO" "$@"
fi

# Default to success scenario
exec "/mock-commands/dns-localdns/dig-success" "$@"
EOF

    cat > "$cmd_dir/systemctl" <<'EOF'
#!/bin/bash
# systemctl dispatcher for LocalDNS tests
SCENARIO="${SYSTEMCTL_SCENARIO:-active}"

# Look for scenario-specific mock
if [ -f "/mock-commands/dns-localdns/systemctl-$SCENARIO" ]; then
    exec "/mock-commands/dns-localdns/systemctl-$SCENARIO" "$@"
fi

# Default to active scenario
exec "/mock-commands/dns-localdns/systemctl-active" "$@"
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

# Copy kubeconfig to all scenario directories that need it
copy_kubeconfigs() {
    local base_kubeconfig="$SCRIPT_DIR/mock-data/localdns-healthy/var/lib/kubelet/kubeconfig"
    local dirs=("localdns-disabled" "localdns-service-down" "localdns-mismatch" "localdns-no-labels")
    
    for dir in "${dirs[@]}"; do
        if [ -d "$SCRIPT_DIR/mock-data/$dir/var/lib/kubelet" ]; then
            cp "$base_kubeconfig" "$SCRIPT_DIR/mock-data/$dir/var/lib/kubelet/kubeconfig"
        fi
    done
}

# Run all setup functions
create_localdns_healthy
create_localdns_disabled
create_localdns_service_down
create_localdns_mismatch
create_localdns_no_labels
create_dependency_missing
create_dns_localdns_mock_commands
copy_kubeconfigs

echo "LocalDNS mock data creation complete"