#!/bin/bash
# Create NPD startup mock data for testing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Remove existing startup mock data
rm -rf "$SCRIPT_DIR/mock-data/startup-"*

# =============================================================================
# GPU VM with working drivers and NVLink support
# =============================================================================
create_gpu_with_nvlink() {
    echo "Create GPU with NVLink mock data"
    local dir="$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink"

    mkdir -p "$dir/mock-commands"
    mkdir -p "$dir/etc/node-problem-detector.d/plugin"
    mkdir -p "$dir/etc/node-problem-detector.d/custom-plugin-monitor/gpu_checks"
    mkdir -p "$dir/etc/node-problem-detector.d/system-log-monitor"
    mkdir -p "$dir/etc/node-problem-detector.d/system-stats-monitor"
    mkdir -p "$dir/home/azureuser/.kube"
    mkdir -p "$dir/var/lib/kubelet"

    # IMDS response for GPU VM
    cat > "$dir/mock-commands/curl" <<'EOF'
#!/bin/bash
if [[ "$*" == *"169.254.169.254"* && "$*" == *"instance/compute"* ]]; then
    cat <<'IMDS_EOF'
{
  "vmSize": "Standard_ND96isr_H100_v5"
}
IMDS_EOF
else
    exec /usr/bin/curl "$@"
fi
EOF

    # Working nvidia-smi with NVLink support
    cat > "$dir/mock-commands/nvidia-smi" <<'EOF'
#!/bin/bash
if [[ "$*" == *"nvlink"* && "$*" == *"--status"* ]]; then
    cat <<'NVLINK_EOF'
GPU 0: Tesla V100-SXM2-32GB (UUID: GPU-12345678-1234-5678-9012-123456789012)
	 Link 0: 25.781 GB/s
	 Link 1: 25.781 GB/s
NVLINK_EOF
else
    cat <<'NVIDIA_EOF'
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 525.125.06   Driver Version: 525.125.06   CUDA Version: 12.0     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
|   0  Tesla V100-SXM2...  On   | 00000000:00:1E.0 Off |                    0 |
+-----------------------------------------------------------------------------+
NVIDIA_EOF
fi
exit 0
EOF

    # Working systemctl
    cat > "$dir/mock-commands/systemctl" <<'EOF'
#!/bin/bash
if [[ "$*" == *"cat kubelet"* ]]; then
    cat <<'SYSTEMCTL_EOF'
[Service]
ExecStart=/usr/local/bin/kubelet \
  --container-runtime-endpoint="unix:///run/containerd/containerd.sock" \
  --v=2
SYSTEMCTL_EOF
else
    exit 0
fi
EOF

    # Standard hostname
    cat > "$dir/mock-commands/hostname" <<'EOF'
#!/bin/bash
echo "aks-nodepool1-12345678-vmss000001"
EOF

    # Make all commands executable
    chmod +x "$dir/mock-commands"/*

    # Public settings with GPU checks enabled
    cat > "$dir/etc/node-problem-detector.d/public-settings.json" <<'EOF'
{
  "enable-npd-gpu-checks": "true",
  "npd-validate-in-prod": "true"
}
EOF

    # GPU health check configuration files
    cat > "$dir/etc/node-problem-detector.d/custom-plugin-monitor/gpu_checks/custom-plugin-gpu-count.json" <<'EOF'
{
  "plugin": "custom",
  "pluginConfig": {
    "invoke_interval": "60s",
    "timeout": "30s",
    "max_output_length": 80,
    "concurrency": 1
  },
  "source": "gpu-count-check",
  "conditions": [
    {
      "type": "GPUProblem",
      "reason": "GPUCountMismatch",
      "message": "GPU count does not match expected count"
    }
  ],
  "rules": [
    {
      "type": "permanent",
      "condition": "GPUProblem",
      "reason": "GPUCountMismatch",
      "path": "/etc/node-problem-detector.d/plugin/check_gpu_count.sh"
    }
  ]
}
EOF

    cat > "$dir/etc/node-problem-detector.d/custom-plugin-monitor/gpu_checks/custom-plugin-nvlink-status.json" <<'EOF'
{
  "plugin": "custom",
  "pluginConfig": {
    "invoke_interval": "60s",
    "timeout": "30s",
    "max_output_length": 80,
    "concurrency": 1
  },
  "source": "nvlink-status-check",
  "conditions": [
    {
      "type": "NVLinkProblem",
      "reason": "NVLinkError",
      "message": "NVLink hardware error detected"
    }
  ],
  "rules": [
    {
      "type": "permanent",
      "condition": "NVLinkProblem",
      "reason": "NVLinkError",
      "path": "/etc/node-problem-detector.d/plugin/check_nvlink_status.sh"
    }
  ]
}
EOF

    # Kubeconfig file
    cat > "$dir/home/azureuser/.kube/config" <<'EOF'
apiVersion: v1
clusters:
- cluster:
    server: https://test-cluster-12345678.hcp.eastus.azmk8s.io:443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: clusterUser_test-rg_test-cluster
  name: test-cluster
current-context: test-cluster
kind: Config
users:
- name: clusterUser_test-rg_test-cluster
  user:
    client-certificate-data: LS0tLS1CRUdJTi...
    client-key-data: LS0tLS1CRUdJTi...
    token: eyJhbGciOiJSUzI1NiIsImtpZCI6...
EOF
}

# =============================================================================
# GPU VM without NVLink support
# =============================================================================
create_gpu_without_nvlink() {
    echo "Create GPU without NVLink mock data"
    local dir="$SCRIPT_DIR/mock-data/startup-gpu-without-nvlink"

    mkdir -p "$dir/mock-commands"
    mkdir -p "$dir/etc/node-problem-detector.d/plugin"
    mkdir -p "$dir/etc/node-problem-detector.d/custom-plugin-monitor/gpu_checks"
    mkdir -p "$dir/etc/node-problem-detector.d/system-log-monitor"
    mkdir -p "$dir/etc/node-problem-detector.d/system-stats-monitor"
    mkdir -p "$dir/home/azureuser/.kube"
    mkdir -p "$dir/var/lib/kubelet"

    # IMDS response for GPU VM
    cat > "$dir/mock-commands/curl" <<'EOF'
#!/bin/bash
if [[ "$*" == *"169.254.169.254"* && "$*" == *"instance/compute"* ]]; then
    cat <<'IMDS_EOF'
{
  "vmSize": "Standard_NC24s_v3"
}
IMDS_EOF
else
    exec /usr/bin/curl "$@"
fi
EOF

    # nvidia-smi without NVLink support
    cat > "$dir/mock-commands/nvidia-smi" <<'EOF'
#!/bin/bash
if [[ "$*" == *"nvlink"* && "$*" == *"--status"* ]]; then
    # Output the "not supported" message to stdout (not stderr) so it can be captured
    echo "NVLink is not supported on device"
    exit 0  # Exit successfully so the output can be processed
else
    # Regular nvidia-smi works but no NVLink
    cat <<'NVIDIA_EOF'
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 525.125.06   Driver Version: 525.125.06   CUDA Version: 12.0     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
|   0  GeForce RTX 3070    Off  | 00000000:01:00.0  On |                  N/A |
+-----------------------------------------------------------------------------+
NVIDIA_EOF
fi
exit 0
EOF

    # Copy other mock commands from nvlink scenario
    cp "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink/mock-commands/systemctl" "$dir/mock-commands/"
    cp "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink/mock-commands/hostname" "$dir/mock-commands/"
    chmod +x "$dir/mock-commands"/*

    # Same configuration as nvlink scenario (GPU checks enabled)
    cp -r "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink/etc" "$dir/"
    cp -r "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink/home" "$dir/"
}

# =============================================================================
# Non-GPU VM
# =============================================================================
create_non_gpu() {
    echo "Create non-GPU VM mock data"
    local dir="$SCRIPT_DIR/mock-data/startup-non-gpu"

    mkdir -p "$dir/mock-commands"
    mkdir -p "$dir/etc/node-problem-detector.d/plugin"
    mkdir -p "$dir/etc/node-problem-detector.d/system-log-monitor"
    mkdir -p "$dir/etc/node-problem-detector.d/system-stats-monitor"
    mkdir -p "$dir/var/lib/kubelet"

    # IMDS response for non-GPU VM
    cat > "$dir/mock-commands/curl" <<'EOF'
#!/bin/bash
if [[ "$*" == *"169.254.169.254"* && "$*" == *"instance/compute"* ]]; then
    cat <<'IMDS_EOF'
{
  "vmSize": "Standard_D4s_v3"
}
IMDS_EOF
else
    exec /usr/bin/curl "$@"
fi
EOF

    # No nvidia-smi available
    cat > "$dir/mock-commands/nvidia-smi" <<'EOF'
#!/bin/bash
echo "nvidia-smi: command not found" >&2
exit 127
EOF

    # Copy other commands
    cp "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink/mock-commands/systemctl" "$dir/mock-commands/"
    cp "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink/mock-commands/hostname" "$dir/mock-commands/"
    chmod +x "$dir/mock-commands"/*

    # Public settings with GPU checks enabled (to test VM capability detection)
    cat > "$dir/etc/node-problem-detector.d/public-settings.json" <<'EOF'
{
  "enable-npd-gpu-checks": "true",
  "npd-validate-in-prod": "true"
}
EOF

    # Kubeconfig in worker node location
    cat > "$dir/var/lib/kubelet/kubeconfig" <<'EOF'
apiVersion: v1
clusters:
- cluster:
    server: https://worker-cluster-87654321.hcp.westus.azmk8s.io:443
  name: worker-cluster
contexts:
- context:
    cluster: worker-cluster
    user: system:node:aks-nodepool1-12345678-vmss000001
  name: worker-cluster
current-context: worker-cluster
kind: Config
users:
- name: system:node:aks-nodepool1-12345678-vmss000001
  user:
    client-certificate-data: LS0tLS1CRUdJTi...
    client-key-data: LS0tLS1CRUdJTi...
EOF
}

# =============================================================================
# GPU VM with missing/inaccessible drivers
# =============================================================================
create_gpu_missing_drivers() {
    echo "Create GPU with missing drivers mock data"
    local dir="$SCRIPT_DIR/mock-data/startup-gpu-missing-drivers"

    # Copy the base GPU structure
    cp -r "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink" "$dir"

    # Replace nvidia-smi with missing version
    cat > "$dir/mock-commands/nvidia-smi" <<'EOF'
#!/bin/bash
echo "nvidia-smi: command not found" >&2
exit 127
EOF
    chmod +x "$dir/mock-commands/nvidia-smi"
}


# =============================================================================
# GPU VM with driver failure
# =============================================================================
create_gpu_driver_failure() {
    echo "Create GPU with driver failure mock data"
    local dir="$SCRIPT_DIR/mock-data/startup-gpu-driver-failure"

    # Copy the base GPU structure
    cp -r "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink" "$dir"

    # Replace nvidia-smi with failing version
    cat > "$dir/mock-commands/nvidia-smi" <<'EOF'
#!/bin/bash
echo "NVIDIA-SMI has failed because it couldn't communicate with the NVIDIA driver. Make sure that the latest NVIDIA driver is installed and running." >&2
exit 255
EOF
    chmod +x "$dir/mock-commands/nvidia-smi"
}

# =============================================================================
# IMDS empty response scenario
# =============================================================================
create_imds_empty() {
    echo "Create IMDS empty response mock data"
    local dir="$SCRIPT_DIR/mock-data/startup-imds-empty"

    # Copy the base GPU structure
    cp -r "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink" "$dir"

    # Replace curl with empty response version
    cat > "$dir/mock-commands/curl" <<'EOF'
#!/bin/bash
if [[ "$*" == *"169.254.169.254"* && "$*" == *"instance/compute"* ]]; then
    # Return empty response
    exit 0
else
    exec /usr/bin/curl "$@"
fi
EOF
    chmod +x "$dir/mock-commands/curl"
}

# =============================================================================
# Hostname case conversion scenario
# =============================================================================
create_hostname_uppercase() {
    echo "Create hostname uppercase mock data"
    local dir="$SCRIPT_DIR/mock-data/startup-hostname-uppercase"

    # Copy the base GPU structure
    cp -r "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink" "$dir"

    # Replace hostname with uppercase version
    cat > "$dir/mock-commands/hostname" <<'EOF'
#!/bin/bash
echo "AKS-NODEPOOL1-12345678-VMSS000001"
EOF
    chmod +x "$dir/mock-commands/hostname"
}

# =============================================================================
# Long hostname scenario
# =============================================================================
create_hostname_long() {
    echo "Create long hostname mock data"
    local dir="$SCRIPT_DIR/mock-data/startup-hostname-long"

    # Copy the base GPU structure
    cp -r "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink" "$dir"

    # Replace hostname with long version
    cat > "$dir/mock-commands/hostname" <<'EOF'
#!/bin/bash
echo "very-long-hostname-that-exceeds-normal-limits-and-could-cause-issues-with-kubernetes-node-naming-conventions-vmss000001"
EOF
    chmod +x "$dir/mock-commands/hostname"
}

# =============================================================================
# Special characters in kubelet config scenario
# =============================================================================
create_kubelet_special_chars() {
    echo "Create kubelet special chars mock data"
    local dir="$SCRIPT_DIR/mock-data/startup-kubelet-special-chars"

    # Copy the base GPU structure
    cp -r "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink" "$dir"

    # Replace systemctl with special chars version
    cat > "$dir/mock-commands/systemctl" <<'EOF'
#!/bin/bash
if [[ "$*" == *"cat kubelet"* ]]; then
    cat <<'SYSTEMCTL_EOF'
[Service]
ExecStart=/usr/local/bin/kubelet \
  --bootstrap-kubeconfig="/etc/kubernetes/bootstrap-kubelet.conf" \
  --kubeconfig="/var/lib/kubelet/kubeconfig" \
  --config="/var/lib/kubelet/config.yaml" \
  --container-runtime-endpoint="unix:///run/containerd/containerd.sock" \
  --pod-infra-container-image="mcr.microsoft.com/oss/kubernetes/pause:3.6" \
  --node-labels="special=value with spaces,kubernetes.azure.com/role=agent" \
  --v=2
SYSTEMCTL_EOF
else
    exit 0
fi
EOF
    chmod +x "$dir/mock-commands/systemctl"
}

# =============================================================================
# IMDS timeout scenario
# =============================================================================
create_imds_timeout() {
    echo "Create IMDS timeout mock data"
    local dir="$SCRIPT_DIR/mock-data/startup-imds-timeout"

    # Copy the base GPU structure
    cp -r "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink" "$dir"

    # Replace curl with timeout version
    cat > "$dir/mock-commands/curl" <<'EOF'
#!/bin/bash
if [[ "$*" == *"169.254.169.254"* ]]; then
    # Simulate timeout
    sleep 1
    exit 28  # curl timeout exit code
else
    exec /usr/bin/curl "$@"
fi
EOF
    chmod +x "$dir/mock-commands/curl"
}

# =============================================================================
# Invalid JSON scenarios
# =============================================================================
create_invalid_json() {
    echo "Create invalid JSON mock data"
    local dir="$SCRIPT_DIR/mock-data/startup-invalid-json"

    # Copy the base structure
    cp -r "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink" "$dir"

    # Replace curl with malformed response
    cat > "$dir/mock-commands/curl" <<'EOF'
#!/bin/bash
if [[ "$*" == *"169.254.169.254"* && "$*" == *"instance/compute"* ]]; then
    echo "{"notVmSize": "unexpected_field"}"
    exit 0
else
    exec /usr/bin/curl "$@"
fi
EOF
    chmod +x "$dir/mock-commands/curl"

    # Create invalid VM capabilities JSON
    cat > "$dir/etc/node-problem-detector.d/plugin/gpu_vms_capabilities.json" <<'EOF'
{
  "invalid": "json structure"
  "missing": "closing brace"
EOF

    # Create invalid public settings JSON
    cat > "$dir/etc/node-problem-detector.d/public-settings.json" <<'EOF'
{
  "enable-npd-gpu-checks": true,  // Invalid: should be string
  "npd-validate-in-prod": "true",
  "extra-comma": "value",
}
EOF
}

# =============================================================================
# Missing configuration scenarios
# =============================================================================
create_missing_config() {
    echo "Create missing config mock data"
    local dir="$SCRIPT_DIR/mock-data/startup-missing-config"

    mkdir -p "$dir/mock-commands"
    mkdir -p "$dir/etc/node-problem-detector.d/plugin"
    mkdir -p "$dir/etc/node-problem-detector.d/system-log-monitor"
    mkdir -p "$dir/etc/node-problem-detector.d/system-stats-monitor"

    # Copy basic mock commands
    cp "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink/mock-commands/curl" "$dir/mock-commands/"
    cp "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink/mock-commands/nvidia-smi" "$dir/mock-commands/"
    cp "$SCRIPT_DIR/mock-data/startup-gpu-with-nvlink/mock-commands/hostname" "$dir/mock-commands/"

    # Create systemctl with missing container runtime endpoint
    cat > "$dir/mock-commands/systemctl" <<'EOF'
#!/bin/bash
if [[ "$*" == *"cat kubelet"* ]]; then
    cat <<'SYSTEMCTL_EOF'
[Service]
ExecStart=/usr/local/bin/kubelet \
  --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf \
  --kubeconfig=/var/lib/kubelet/kubeconfig \
  --config=/var/lib/kubelet/config.yaml \
  --pod-infra-container-image=mcr.microsoft.com/oss/kubernetes/pause:3.6 \
  --v=2
SYSTEMCTL_EOF
else
    exit 0
fi
EOF
    chmod +x "$dir/mock-commands"/*

    # Only create minimal directory structure - missing key files like public-settings.json
    # This tests error handling when required files are not present
}

# Create all scenarios
create_gpu_with_nvlink
create_gpu_without_nvlink
create_non_gpu
create_gpu_missing_drivers
create_gpu_driver_failure
create_imds_empty
create_hostname_uppercase
create_hostname_long
create_kubelet_special_chars
create_imds_timeout
create_invalid_json
create_missing_config
