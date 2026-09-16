#!/bin/bash
NODE_INDEX=$(hostname | tail -c 2)
NODE_NAME=$(hostname)

# Older provisioners may not set module paths; default to siblings of this script.
source "${CSE_CONFIG_GPU_FILEPATH:-${BASH_SOURCE[0]%.sh}_gpu.sh}"
source "${CSE_CONFIG_LOCALDNS_FILEPATH:-${BASH_SOURCE[0]%.sh}_localdns.sh}"
source "${CSE_CONFIG_KUBELET_FILEPATH:-${BASH_SOURCE[0]%.sh}_kubelet.sh}"
source "${CSE_CONFIG_NETWORK_FILEPATH:-${BASH_SOURCE[0]%.sh}_network.sh}"
source "${CSE_CONFIG_ADDONS_FILEPATH:-${BASH_SOURCE[0]%.sh}_addons.sh}"

configureAdminUser(){
    chage -E -1 -I -1 -m 0 -M 99999 "${ADMINUSER}"
    chage -l "${ADMINUSER}"
    chage -I -1 -M -1 root
}

configureTransparentHugePage() {
    local etc_sysfs_conf="/etc/sysfs.conf"

    applyTransparentHugePageValues

    if [ -n "${THP_ENABLED}" ]; then
        printf 'kernel/mm/transparent_hugepage/enabled=%s\n' "${THP_ENABLED}" >> "${etc_sysfs_conf}" || exit "$ERR_SYSCTL_RELOAD"
    fi
    if [ -n "${THP_DEFRAG}" ]; then
        printf 'kernel/mm/transparent_hugepage/defrag=%s\n' "${THP_DEFRAG}" >> "${etc_sysfs_conf}" || exit "$ERR_SYSCTL_RELOAD"
    fi
    reconcileTransparentHugePagePersistence
}

applyTransparentHugePageValues() {
    local thp_enabled_path="/sys/kernel/mm/transparent_hugepage/enabled"
    local thp_defrag_path="/sys/kernel/mm/transparent_hugepage/defrag"

    if [ -n "${THP_ENABLED}" ]; then
        printf '%s\n' "${THP_ENABLED}" > "${thp_enabled_path}" || exit "$ERR_SYSCTL_RELOAD"
    fi
    if [ -n "${THP_DEFRAG}" ]; then
        printf '%s\n' "${THP_DEFRAG}" > "${thp_defrag_path}" || exit "$ERR_SYSCTL_RELOAD"
    fi
}

reconcileTransparentHugePagePersistence() {
    if { [ -n "${THP_ENABLED}" ] || [ -n "${THP_DEFRAG}" ]; } && isMarinerOrAzureLinux "$OS" "$OS_VARIANT"; then
        configureTransparentHugePageSystemdService
    fi
}

configureTransparentHugePageSystemdService() {
    local service_name="aks-transparent-hugepage"
    local script_path="/opt/azure/containers/aks-transparent-hugepage.sh"
    local config_dir="/opt/azure/containers/aks-transparent-hugepage"
    local service_path="/etc/systemd/system/${service_name}.service"

    mkdir -p "$(dirname "${script_path}")" "${config_dir}" || exit "$ERR_SYSCTL_RELOAD"
    if [ -n "${THP_ENABLED}" ]; then
        printf '%s\n' "${THP_ENABLED}" | tee "${config_dir}/enabled" > /dev/null || exit "$ERR_SYSCTL_RELOAD"
    else
        rm -f "${config_dir}/enabled" || exit "$ERR_SYSCTL_RELOAD"
    fi
    if [ -n "${THP_DEFRAG}" ]; then
        printf '%s\n' "${THP_DEFRAG}" | tee "${config_dir}/defrag" > /dev/null || exit "$ERR_SYSCTL_RELOAD"
    else
        rm -f "${config_dir}/defrag" || exit "$ERR_SYSCTL_RELOAD"
    fi

    if ! tee "${script_path}" > /dev/null <<'EOF'
#!/bin/bash
set -e
config_dir="/opt/azure/containers/aks-transparent-hugepage"
thp_enabled_config="${config_dir}/enabled"
thp_defrag_config="${config_dir}/defrag"

if [ -s "${thp_enabled_config}" ]; then
    cat "${thp_enabled_config}" > /sys/kernel/mm/transparent_hugepage/enabled
fi
if [ -s "${thp_defrag_config}" ]; then
    cat "${thp_defrag_config}" > /sys/kernel/mm/transparent_hugepage/defrag
fi
EOF
    then
        exit "$ERR_SYSCTL_RELOAD"
    fi
    chmod 0755 "${script_path}" || exit "$ERR_SYSCTL_RELOAD"

    if ! tee "${service_path}" > /dev/null <<EOF
[Unit]
Description=Apply AKS transparent huge page settings
After=systemd-sysctl.service
Before=kubelet.service
ConditionPathExists=/sys/kernel/mm/transparent_hugepage

[Service]
Type=oneshot
ExecStart=${script_path}

[Install]
WantedBy=multi-user.target
EOF
    then
        exit "$ERR_SYSCTL_RELOAD"
    fi

    systemctl daemon-reload || exit "$ERR_SYSTEMCTL_START_FAIL"
    systemctlEnableAndStart "${service_name}" 30 || exit "$ERR_SYSTEMCTL_START_FAIL"
}

swapFileIsActive() {
    local swap_location="$1"

    swapon --show --noheadings | awk '{print $1}' | grep -Fxq "${swap_location}"
}

getDiskFreeKB() {
    local disk_path="$1"

    df -P "${disk_path}" | awk 'NR == 2 {print $4}'
}

hasSufficientDiskSpace() {
    local disk_path="$1"
    local required_kb="$2"
    local disk_free_kb

    disk_free_kb="$(getDiskFreeKB "${disk_path}")"
    case "${disk_free_kb}" in
        ''|*[!0-9]*) return 1 ;;
        *) [ "${disk_free_kb}" -gt "${required_kb}" ] ;;
    esac
}

waitForDiskSpace() {
    local disk_path="$1"
    local required_kb="$2"
    local timeout_seconds="$3"
    local poll_interval_seconds="$4"
    local elapsed_seconds=0

    while [ "${elapsed_seconds}" -lt "${timeout_seconds}" ]; do
        sleep "${poll_interval_seconds}"
        elapsed_seconds=$((elapsed_seconds + poll_interval_seconds))
        if hasSufficientDiskSpace "${disk_path}" "${required_kb}"; then
            return 0
        fi
    done

    return 1
}

getFileMode() {
    local file="$1"

    stat -c "%a" "${file}" 2>/dev/null || stat -f "%Lp" "${file}" 2>/dev/null
}

ensureSwapFileFstabEntry() {
    local swap_location="$1"
    local fstab_entry="${swap_location} none swap noauto,nofail 0 0"
    local fstab_file="${2:-/etc/fstab}"
    local fstab_dir
    local fstab_mode
    local temp_fstab

    fstab_dir="$(dirname "${fstab_file}")"
    temp_fstab="$(mktemp "${fstab_dir}/fstab.XXXXXX")" || return 1
    fstab_mode="$(getFileMode "${fstab_file}")" || {
        rm -f "${temp_fstab}"
        return 1
    }
    chmod "${fstab_mode}" "${temp_fstab}" || {
        rm -f "${temp_fstab}"
        return 1
    }
    awk -v swap_location="${swap_location}" '$1 != swap_location { print }' "${fstab_file}" > "${temp_fstab}" || {
        rm -f "${temp_fstab}"
        return 1
    }
    echo "${fstab_entry}" >> "${temp_fstab}" || {
        rm -f "${temp_fstab}"
        return 1
    }
    mv "${temp_fstab}" "${fstab_file}" || {
        rm -f "${temp_fstab}"
        return 1
    }
}

findExistingSwapFileLocation() {
    local resource_disk_path
    local swap_location

    if [ -L /dev/disk/azure/resource-part1 ]; then
        resource_disk_path=$(findmnt -nr -o target -S "$(readlink -f /dev/disk/azure/resource-part1)" || true)
        swap_location="${resource_disk_path}/swapfile"
        if [ -n "${resource_disk_path}" ] && [ -f "${swap_location}" ]; then
            echo "${swap_location}"
            return 0
        fi
    fi

    if [ -f /swapfile ]; then
        echo "/swapfile"
        return 0
    fi

    return 1
}

reconcileSwapFilePersistence() {
    local swap_location="${1:-}"

    if [ -z "${swap_location}" ]; then
        swap_location="$(findExistingSwapFileLocation || true)"
    fi

    if [ -z "${swap_location}" ]; then
        echo "No existing AKS swap file found; creating swap file for persistence reconciliation"
        configureSwapFile
        return 0
    fi

    ensureSwapFileFstabEntry "${swap_location}" || exit "$ERR_SWAP_CREATE_FAIL"
    configureSwapFileSystemdService "${swap_location}" || exit "$ERR_SWAP_CREATE_FAIL"
}

configureSwapFile() {
    # https://learn.microsoft.com/en-us/troubleshoot/azure/virtual-machines/troubleshoot-device-names-problems#identify-disk-luns
    swap_size_kb=$(expr "${SWAP_FILE_SIZE_MB}" \* 1000)
    swap_location=""

    # Attempt to use the resource disk
    if [ -L /dev/disk/azure/resource-part1 ]; then
        resource_disk_path=$(findmnt -nr -o target -S "$(readlink -f /dev/disk/azure/resource-part1)" || true)
        if [ -n "${resource_disk_path}" ]; then
            disk_free_kb=$(df -P "${resource_disk_path}" | sed 1d | awk '{print $4}')
            case "${disk_free_kb}" in
                ''|*[!0-9]*)
                    echo "Could not determine free space on resource disk, attempting to fall back to OS disk..."
                    ;;
                *)
                    if [ "${disk_free_kb}" -gt "${swap_size_kb}" ]; then
                        echo "Will use resource disk for swap file"
                        swap_location=${resource_disk_path}/swapfile
                    else
                        echo "Insufficient disk space on resource disk to create swap file: request ${swap_size_kb} free ${disk_free_kb}, attempting to fall back to OS disk..."
                    fi
                    ;;
            esac
        else
            echo "Could not determine resource disk mountpoint, attempting to fall back to OS disk..."
        fi
    fi

    # If we couldn't use the resource disk, attempt to use the OS disk
    if [ -z "${swap_location}" ]; then
        # Directly check size on the root directory since we can't rely on 'root-part1' always being the correct label
        os_device=$(readlink -f /dev/disk/azure/root)
        disk_free_kb=$(getDiskFreeKB /)
        if ! hasSufficientDiskSpace / "${swap_size_kb}"; then
            echo "Insufficient disk space on OS device ${os_device}, waiting up to 30 seconds for filesystem resize: request ${swap_size_kb} free ${disk_free_kb}"
            if ! waitForDiskSpace / "${swap_size_kb}" 30 1; then
                disk_free_kb=$(getDiskFreeKB /)
                echo "Insufficient disk space on OS device ${os_device} to create swap file after waiting for filesystem resize: request ${swap_size_kb} free ${disk_free_kb}"
                exit $ERR_SWAP_CREATE_INSUFFICIENT_DISK_SPACE
            fi
        fi
        echo "Will use OS disk for swap file"
        swap_location=/swapfile
    fi

    echo "Swap file will be saved to: ${swap_location}"
    retrycmd_if_failure 24 5 25 fallocate -l "${swap_size_kb}K" "${swap_location}" || exit "$ERR_SWAP_CREATE_FAIL"
    chmod 600 "${swap_location}"
    retrycmd_if_failure 24 5 25 mkswap "${swap_location}" || exit "$ERR_SWAP_CREATE_FAIL"
    retrycmd_if_failure 24 5 25 swapon "${swap_location}" || exit "$ERR_SWAP_CREATE_FAIL"
    swapFileIsActive "${swap_location}" || exit "$ERR_SWAP_CREATE_FAIL"
    reconcileSwapFilePersistence "${swap_location}" || exit "$ERR_SWAP_CREATE_FAIL"
}

configureSwapFileSystemdService() {
    local swap_location="$1"
    local service_name="aks-swapfile"
    local script_path="/opt/azure/containers/aks-swapfile.sh"
    local service_path="/etc/systemd/system/${service_name}.service"
    local swap_mount_path

    swap_mount_path="$(dirname "${swap_location}")"

    mkdir -p "$(dirname "${script_path}")" || exit "$ERR_SWAP_CREATE_FAIL"
    if ! tee "${script_path}" > /dev/null <<EOF
#!/bin/bash
set -e
if ! swapon --show --noheadings | awk '{print \$1}' | grep -Fxq "${swap_location}"; then
    swapon "${swap_location}"
fi
EOF
    then
        exit "$ERR_SWAP_CREATE_FAIL"
    fi
    chmod 0755 "${script_path}" || exit "$ERR_SWAP_CREATE_FAIL"

    if ! tee "${service_path}" > /dev/null <<EOF
[Unit]
Description=Activate AKS swap file
After=local-fs.target
Before=kubelet.service
RequiresMountsFor=${swap_mount_path}
ConditionPathExists=${swap_location}

[Service]
Type=oneshot
ExecStart=${script_path}
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF
    then
        exit "$ERR_SWAP_CREATE_FAIL"
    fi

    systemctl daemon-reload || exit "$ERR_SYSTEMCTL_START_FAIL"
    systemctlEnableAndStart "${service_name}" 30 || exit "$ERR_SYSTEMCTL_START_FAIL"
}

configureEtcEnvironment() {
    mkdir -p /etc/systemd/system.conf.d/
    touch /etc/systemd/system.conf.d/proxy.conf
    chmod 0644 /etc/systemd/system.conf.d/proxy.conf

    mkdir -p  /etc/apt/apt.conf.d
    touch /etc/apt/apt.conf.d/95proxy
    chmod 0644 /etc/apt/apt.conf.d/95proxy

    echo "[Manager]" >> /etc/systemd/system.conf.d/proxy.conf
    if [ -n "${HTTP_PROXY_URLS}" ]; then
        echo "HTTP_PROXY=${HTTP_PROXY_URLS}" >> /etc/environment
        echo "http_proxy=${HTTP_PROXY_URLS}" >> /etc/environment
        echo "Acquire::http::proxy \"${HTTP_PROXY_URLS}\";" >> /etc/apt/apt.conf.d/95proxy
        echo "DefaultEnvironment=\"HTTP_PROXY=${HTTP_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
        echo "DefaultEnvironment=\"http_proxy=${HTTP_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
    fi
    if [ -n "${HTTPS_PROXY_URLS}" ]; then
        echo "HTTPS_PROXY=${HTTPS_PROXY_URLS}" >> /etc/environment
        echo "https_proxy=${HTTPS_PROXY_URLS}" >> /etc/environment
        echo "Acquire::https::proxy \"${HTTPS_PROXY_URLS}\";" >> /etc/apt/apt.conf.d/95proxy
        echo "DefaultEnvironment=\"HTTPS_PROXY=${HTTPS_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
        echo "DefaultEnvironment=\"https_proxy=${HTTPS_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
    fi
    if [ -n "${NO_PROXY_URLS}" ]; then
        echo "NO_PROXY=${NO_PROXY_URLS}" >> /etc/environment
        echo "no_proxy=${NO_PROXY_URLS}" >> /etc/environment
        echo "DefaultEnvironment=\"NO_PROXY=${NO_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
        echo "DefaultEnvironment=\"no_proxy=${NO_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
    fi

    mkdir -p "/etc/systemd/system/kubelet.service.d"
    tee "/etc/systemd/system/kubelet.service.d/10-httpproxy.conf" > /dev/null <<'EOF'
[Service]
EnvironmentFile=/etc/environment
EOF
}

configureHTTPProxyCA() {
    suffix="crt"
    if isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
        cert_dest="/etc/pki/ca-trust/source/anchors"
        update_cmd="update-ca-trust"
    elif isMarinerOrAzureLinux "$OS"; then
        cert_dest="/usr/share/pki/ca-trust-source/anchors"
        update_cmd="update-ca-trust"
    elif isACL "$OS" "$OS_VARIANT"; then
        # ACL is Flatcar-based but uses Azure Linux internals for CA trust.
        cert_dest="/etc/pki/ca-trust/source/anchors"
        update_cmd="update-ca-trust"
    elif isFlatcar "$OS"; then
        cert_dest="/etc/ssl/certs"
        update_cmd="update-ca-certificates"
        # c_rehash inside update-ca-certificates only handles *.pem in /etc/ssl/certs
        suffix="pem"
    else
        cert_dest="/usr/local/share/ca-certificates"
        update_cmd="update-ca-certificates"
    fi
    HTTP_PROXY_TRUSTED_CA=$(echo "${HTTP_PROXY_TRUSTED_CA}" | xargs)
    echo "${HTTP_PROXY_TRUSTED_CA}" | base64 -d > "${cert_dest}/proxyCA.${suffix}" || exit $ERR_UPDATE_CA_CERTS
    $update_cmd || exit $ERR_UPDATE_CA_CERTS
}

configureCustomCaCertificate() {
    mkdir -p /opt/certs
    # This path is used by the Custom CA Trust feature only
    chmod 755 /opt/certs
    for i in $(seq 0 $((${CUSTOM_CA_TRUST_COUNT} - 1))); do
        # declare dynamically and use "!" to avoid bad substition errors
        declare varname=CUSTOM_CA_CERT_${i}
        echo "${!varname}" | base64 -d > /opt/certs/00000000000000cert${i}.crt
    done
    # blocks until svc is considered active, which will happen when ExecStart command terminates with code 0
    systemctl restart update_certs.service || exit $ERR_UPDATE_CA_CERTS
}

configureContainerdUlimits() {
  CONTAINERD_ULIMIT_DROP_IN_FILE_PATH="/etc/systemd/system/containerd.service.d/set_ulimits.conf"
  mkdir -p "$(dirname "${CONTAINERD_ULIMIT_DROP_IN_FILE_PATH}")"
  touch "${CONTAINERD_ULIMIT_DROP_IN_FILE_PATH}"
  chmod 0600 "${CONTAINERD_ULIMIT_DROP_IN_FILE_PATH}"
  tee "${CONTAINERD_ULIMIT_DROP_IN_FILE_PATH}" > /dev/null <<EOF
$(echo "$CONTAINERD_ULIMITS" | tr ' ' '\n')
EOF
}

# file paths defined outside so configureAzureJson can be unit tested
# TODO: move common file path definitions to cse_helpers.sh
AZURE_JSON_PATH="/etc/kubernetes/azure.json"
AKS_CUSTOM_CLOUD_JSON_PATH="/etc/kubernetes/${TARGET_ENVIRONMENT}.json"
configureAzureJson() {
    mkdir -p "$(dirname "${AZURE_JSON_PATH}")"
    touch "${AZURE_JSON_PATH}"
    chmod 0600 "${AZURE_JSON_PATH}"
    chown root:root "${AZURE_JSON_PATH}"

    set +x
    if [ -n "${SERVICE_PRINCIPAL_FILE_CONTENT}" ]; then
        SERVICE_PRINCIPAL_CLIENT_SECRET="$(base64 -d <<< "${SERVICE_PRINCIPAL_FILE_CONTENT}")"
    fi
    SERVICE_PRINCIPAL_CLIENT_SECRET=${SERVICE_PRINCIPAL_CLIENT_SECRET//\\/\\\\}
    SERVICE_PRINCIPAL_CLIENT_SECRET=${SERVICE_PRINCIPAL_CLIENT_SECRET//\"/\\\"}

    cat << EOF > "${AZURE_JSON_PATH}"
{
    "cloud": "${TARGET_CLOUD}",
    "tenantId": "${TENANT_ID}",
    "subscriptionId": "${SUBSCRIPTION_ID}",
    "aadClientId": "${SERVICE_PRINCIPAL_CLIENT_ID}",
    "aadClientSecret": "${SERVICE_PRINCIPAL_CLIENT_SECRET}",
    "resourceGroup": "${RESOURCE_GROUP}",
    "location": "${LOCATION}",
    "vmType": "${VM_TYPE}",
    "subnetName": "${SUBNET}",
    "securityGroupName": "${NETWORK_SECURITY_GROUP}",
    "vnetName": "${VIRTUAL_NETWORK}",
    "vnetResourceGroup": "${VIRTUAL_NETWORK_RESOURCE_GROUP}",
    "routeTableName": "${ROUTE_TABLE}",
    "primaryAvailabilitySetName": "${PRIMARY_AVAILABILITY_SET}",
    "primaryScaleSetName": "${PRIMARY_SCALE_SET}",
    "cloudProviderBackoffMode": "${CLOUDPROVIDER_BACKOFF_MODE}",
    "cloudProviderBackoff": ${CLOUDPROVIDER_BACKOFF},
    "cloudProviderBackoffRetries": ${CLOUDPROVIDER_BACKOFF_RETRIES},
    "cloudProviderBackoffExponent": ${CLOUDPROVIDER_BACKOFF_EXPONENT},
    "cloudProviderBackoffDuration": ${CLOUDPROVIDER_BACKOFF_DURATION},
    "cloudProviderBackoffJitter": ${CLOUDPROVIDER_BACKOFF_JITTER},
    "cloudProviderRateLimit": ${CLOUDPROVIDER_RATELIMIT},
    "cloudProviderRateLimitQPS": ${CLOUDPROVIDER_RATELIMIT_QPS},
    "cloudProviderRateLimitBucket": ${CLOUDPROVIDER_RATELIMIT_BUCKET},
    "cloudProviderRateLimitQPSWrite": ${CLOUDPROVIDER_RATELIMIT_QPS_WRITE},
    "cloudProviderRateLimitBucketWrite": ${CLOUDPROVIDER_RATELIMIT_BUCKET_WRITE},
    "useManagedIdentityExtension": ${USE_MANAGED_IDENTITY_EXTENSION},
    "userAssignedIdentityID": "${USER_ASSIGNED_IDENTITY_ID}",
    "useInstanceMetadata": ${USE_INSTANCE_METADATA},
    "loadBalancerSku": "${LOAD_BALANCER_SKU}",
    "disableOutboundSNAT": ${LOAD_BALANCER_DISABLE_OUTBOUND_SNAT},
    "excludeMasterFromStandardLB": ${EXCLUDE_MASTER_FROM_STANDARD_LB},
    "providerVaultName": "${KMS_PROVIDER_VAULT_NAME}",
    "maximumLoadBalancerRuleCount": ${MAXIMUM_LOADBALANCER_RULE_COUNT},
    "providerKeyName": "k8s",
    "providerKeyVersion": ""
}
EOF
    set -x

    if [ "${CLOUDPROVIDER_BACKOFF_MODE}" = "v2" ]; then
        sed -i "/cloudProviderBackoffExponent/d" $AZURE_JSON_PATH
        sed -i "/cloudProviderBackoffJitter/d" $AZURE_JSON_PATH
    fi

    if [ "${IS_CUSTOM_CLOUD}" = "true" ]; then
        set +x
        touch "${AKS_CUSTOM_CLOUD_JSON_PATH}"
        chmod 0600 "${AKS_CUSTOM_CLOUD_JSON_PATH}"
        chown root:root "${AKS_CUSTOM_CLOUD_JSON_PATH}"

        echo "${CUSTOM_ENV_JSON}" | base64 -d > "${AKS_CUSTOM_CLOUD_JSON_PATH}"
        set -x
    fi
}

configureK8s() {
    mkdir -p "/etc/kubernetes/certs"
    mkdir -p "/etc/systemd/system/kubelet.service.d"

    if [ -n "${APISERVER_PUBLIC_KEY}" ]; then
        APISERVER_PUBLIC_KEY_PATH="/etc/kubernetes/certs/apiserver.crt"
        touch "${APISERVER_PUBLIC_KEY_PATH}"
        chmod 0644 "${APISERVER_PUBLIC_KEY_PATH}"
        chown root:root "${APISERVER_PUBLIC_KEY_PATH}"

        set +x
        echo "${APISERVER_PUBLIC_KEY}" | base64 --decode > "${APISERVER_PUBLIC_KEY_PATH}"
        set -x
    fi

    set +x
    if [ "${ENABLE_SECURE_TLS_BOOTSTRAPPING}" = "false" ] && [ -z "${TLS_BOOTSTRAP_TOKEN:-}" ]; then
        # only create the client cert and key if we're not using vanilla/secure TLS bootstrapping
        if [ -n "${KUBELET_CLIENT_CONTENT}" ]; then
            echo "${KUBELET_CLIENT_CONTENT}" | base64 -d > /etc/kubernetes/certs/client.key
        fi
        if [ -n "${KUBELET_CLIENT_CERT_CONTENT}" ]; then
            echo "${KUBELET_CLIENT_CERT_CONTENT}" | base64 -d > /etc/kubernetes/certs/client.crt
        fi
    fi
    set -x

    if [ "${KUBELET_CONFIG_FILE_ENABLED}" = "true" ]; then
        set +x
        KUBELET_CONFIG_JSON_PATH="/etc/default/kubeletconfig.json"
        touch "${KUBELET_CONFIG_JSON_PATH}"
        chmod 0600 "${KUBELET_CONFIG_JSON_PATH}"
        chown root:root "${KUBELET_CONFIG_JSON_PATH}"
        echo "${KUBELET_CONFIG_FILE_CONTENT}" | base64 -d > "${KUBELET_CONFIG_JSON_PATH}"
        set -x
        KUBELET_CONFIG_DROP_IN="/etc/systemd/system/kubelet.service.d/10-componentconfig.conf"
        touch "${KUBELET_CONFIG_DROP_IN}"
        chmod 0600 "${KUBELET_CONFIG_DROP_IN}"
        tee "${KUBELET_CONFIG_DROP_IN}" > /dev/null <<EOF
[Service]
Environment="KUBELET_CONFIG_FILE_FLAGS=--config /etc/default/kubeletconfig.json"
EOF
    fi
}

ensureContainerd() {
  mkdir -p "/etc/systemd/system/containerd.service.d"
  # Explicitly set LimitNOFILE=1048576 (the value that 'infinity' resolves to on Ubuntu 22.04) for both Ubuntu and Mariner/AzureLinux.
  # On Ubuntu 24.04 (Containerd 2.0), LimitNOFILE is removed upstream and systemd falls back to an implicit soft:hard limit
  # (for example 1024:524288), so containerd inherits a very low soft file descriptor limit (1024) unless we override it here.
  # On Mariner/AzureLinux this is redundant with the base containerd.service unit but harmless.
  # Not removing LimitNOFILE from parts/linux/cloud-init/artifacts/containerd.service,
  # to avoid compatibility issues between new VHDs and old CSE scripts.
  tee "/etc/systemd/system/containerd.service.d/exec_start.conf" > /dev/null <<EOF
[Service]
ExecStartPost=/sbin/iptables -P FORWARD ACCEPT
LimitNOFILE=1048576
EOF

  mkdir -p /etc/containerd

  if grep -q 'BinaryName = "/usr/bin/nvidia-container-runtime"' /etc/containerd/config.toml 2>/dev/null; then
    echo "NVIDIA containerd config already exists at /etc/containerd/config.toml, skipping generation"
  else
    # Remove in case this is an existing symlink or non-NVIDIA config
    rm -f /etc/containerd/config.toml
    if [ "${GPU_NODE}" = "true" ]; then
      # Check VM tag directly to determine if GPU drivers should be skipped
      export -f should_skip_nvidia_drivers
      should_skip=$(should_skip_nvidia_drivers)
      if [ "$?" -eq 0 ] && [ "${should_skip}" = "true" ]; then
        echo "Generating non-GPU containerd config for GPU node due to VM tags"
        echo "${CONTAINERD_CONFIG_NO_GPU_CONTENT}" | base64 -d > /etc/containerd/config.toml || exit $ERR_FILE_WATCH_TIMEOUT
      else
        echo "Generating GPU containerd config..."
        echo "${CONTAINERD_CONFIG_CONTENT}" | base64 -d > /etc/containerd/config.toml || exit $ERR_FILE_WATCH_TIMEOUT
      fi
    else
      echo "Generating containerd config..."
      echo "${CONTAINERD_CONFIG_CONTENT}" | base64 -d > /etc/containerd/config.toml || exit $ERR_FILE_WATCH_TIMEOUT
    fi
  fi

  export -f should_e2e_mock_azure_china_cloud
  E2EMockAzureChinaCloud=$(should_e2e_mock_azure_china_cloud)
  if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
    logs_to_events "AKS.CSE.ensureContainerd.configureContainerdRegistryHost" configureContainerdRegistryHost
  elif [ "${TARGET_CLOUD}" = "AzureChinaCloud" ] || [ "${E2EMockAzureChinaCloud}" = "true" ]; then
    logs_to_events "AKS.CSE.ensureContainerd.configureContainerdLegacyMooncakeMcrHost" configureContainerdLegacyMooncakeMcrHost
  fi

  tee "/etc/sysctl.d/99-force-bridge-forward.conf" > /dev/null <<EOF
net.ipv4.ip_forward = 1
net.ipv4.conf.all.forwarding = 1
net.ipv6.conf.all.forwarding = 1
net.bridge.bridge-nf-call-iptables = 1
EOF
  retrycmd_if_failure 120 5 25 sysctl --system || exit $ERR_SYSCTL_RELOAD

  # Node Memory Hardening: create kubereserved.slice and drop-ins BEFORE starting
  # containerd/kubelet so both services start in the correct slice from the
  # beginning — avoids needing a disruptive restart after the fact.
  resolveKubeletReservedCgroups
  if [ -n "${KUBE_RESERVED_CGROUP}" ] || [ -n "${SYSTEM_RESERVED_CGROUP}" ]; then
      if ! logs_to_events "AKS.CSE.ensureKubelet.ensureKubeletCgroupHierarchy" ensureKubeletCgroupHierarchy; then
          exit $ERR_KUBELET_START_FAIL
      fi
  fi

  systemctlEnableAndStartNoBlock containerd 30 || exit $ERR_SYSTEMCTL_START_FAIL
}

configureContainerdRegistryHost() {
  MCR_REPOSITORY_BASE="${MCR_REPOSITORY_BASE:=mcr.microsoft.com}"
  MCR_REPOSITORY_BASE="${MCR_REPOSITORY_BASE%/}"
  CONTAINERD_CONFIG_REGISTRY_HOST_MCR="/etc/containerd/certs.d/${MCR_REPOSITORY_BASE}/hosts.toml"
  mkdir -p "$(dirname "${CONTAINERD_CONFIG_REGISTRY_HOST_MCR}")"
  touch "${CONTAINERD_CONFIG_REGISTRY_HOST_MCR}"
  chmod 0644 "${CONTAINERD_CONFIG_REGISTRY_HOST_MCR}"
  CONTAINER_REGISTRY_URL=$(sed 's@/@/v2/@1' <<< "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}/")
  tee "${CONTAINERD_CONFIG_REGISTRY_HOST_MCR}" > /dev/null <<EOF
[host."https://${CONTAINER_REGISTRY_URL%/}"]
  capabilities = ["pull", "resolve"]
  override_path = true
EOF
}

# this function craetes containerd host config to map mcr.azk8s.cn host to mcr.azure.cn
# containerd will resolve mcr.azk8s.cn as mcr.azure.cn and pull the image. If failed, it will fallback to mcr.azk8s.cn
# https://github.com/containerd/containerd/blob/main/docs/hosts.md#registry-configuration---examples
# TODO(xinhl): remove when aks rp fully deprecates mcr.azk8s.cn
configureContainerdLegacyMooncakeMcrHost() {
    LEGACY_MCR_REPOSITORY_BASE="mcr.azk8s.cn"
    CONTAINERD_CONFIG_REGISTRY_HOST_MCR="/etc/containerd/certs.d/${LEGACY_MCR_REPOSITORY_BASE}/hosts.toml"
    mkdir -p "$(dirname "${CONTAINERD_CONFIG_REGISTRY_HOST_MCR}")"
    touch "${CONTAINERD_CONFIG_REGISTRY_HOST_MCR}"
    chmod 0644 "${CONTAINERD_CONFIG_REGISTRY_HOST_MCR}"

    TARGET_MCR_REPOSITORY_BASE="mcr.azure.cn"
    tee "${CONTAINERD_CONFIG_REGISTRY_HOST_MCR}" > /dev/null <<EOF
[host."https://${TARGET_MCR_REPOSITORY_BASE}"]
  capabilities = ["pull", "resolve"]
[host."https://${TARGET_MCR_REPOSITORY_BASE}".header]
    X-Forwarded-For = ["${LEGACY_MCR_REPOSITORY_BASE}"]
EOF
}

ensureArtifactStreaming() {
  waitForContainerdReady || exit $ERR_ARTIFACT_STREAMING_INSTALL
  retrycmd_if_failure 120 5 25 systemctl --quiet enable --now acr-mirror overlaybd-tcmu overlaybd-snapshotter || exit $ERR_ARTIFACT_STREAMING_INSTALL

  local acr_mirror_setup="${ACR_MIRROR_SETUP_SCRIPT:-/opt/acr/tools/mirror/setup.sh}"
  if [ -x "$acr_mirror_setup" ]; then
    "$acr_mirror_setup" aks
  else
    echo "Older acr-mirror package is detected, using old acr-config enablement"
    # setup.sh is only available in acr-mirror 1.0.0 and above
    "${ACR_CONFIG_BIN:-/opt/acr/bin/acr-config}" --enable-containerd 'azurecr.io'
  fi
}

ensureSnapshotUpdate() {
    systemctlEnableAndStartNoBlock snapshot-update.timer 30 || exit $ERR_SNAPSHOT_UPDATE_START_FAIL
}

configureNodeExporter() {
    echo "Configuring Node Exporter"
    # Check for skip file to determine if node-exporter was installed on this VHD
    if [ ! -f /etc/node-exporter.d/skip_vhd_node_exporter ]; then
        echo "Node Exporter assets not found on this VHD (missing /etc/node-exporter.d/skip_vhd_node_exporter); skipping configuration."
        return 0
    fi

    if ! systemctlEnableAndStart node-exporter 30; then
        echo "Failed to start node-exporter service"
        return $ERR_NODE_EXPORTER_START_FAIL
    fi
    if ! systemctlEnableAndStart node-exporter-restart.path 30; then
        echo "Failed to start node-exporter-restart.path"
        return $ERR_NODE_EXPORTER_START_FAIL
    fi
    echo "Node Exporter started successfully"
}

ensureSysctl() {
    SYSCTL_CONFIG_FILE=/etc/sysctl.d/999-sysctl-aks.conf
    mkdir -p "$(dirname "${SYSCTL_CONFIG_FILE}")"
    touch "${SYSCTL_CONFIG_FILE}"
    chmod 0644 "${SYSCTL_CONFIG_FILE}"
    echo "${SYSCTL_CONTENT}" | base64 -d > "${SYSCTL_CONFIG_FILE}"
    retrycmd_if_failure 24 5 25 sysctl --system
}

ensureK8sControlPlane() {
    if $REBOOTREQUIRED || [ "$NO_OUTBOUND" = "true" ]; then
        return
    fi
    retrycmd_if_failure 120 5 25 $KUBECTL 2>/dev/null cluster-info || exit $ERR_K8S_RUNNING_TIMEOUT
}

createKubeManifestDir() {
    KUBEMANIFESTDIR=/etc/kubernetes/manifests
    mkdir -p $KUBEMANIFESTDIR
}

writeKubeConfig() {
    KUBECONFIGDIR=/home/$ADMINUSER/.kube
    KUBECONFIGFILE=$KUBECONFIGDIR/config
    mkdir -p $KUBECONFIGDIR
    touch $KUBECONFIGFILE
    chown $ADMINUSER:$ADMINUSER $KUBECONFIGDIR
    chown $ADMINUSER:$ADMINUSER $KUBECONFIGFILE
    chmod 700 $KUBECONFIGDIR
    chmod 600 $KUBECONFIGFILE
    set +x
    echo "
---
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: \"$CA_CERTIFICATE\"
    server: $KUBECONFIG_SERVER
  name: \"$MASTER_FQDN\"
contexts:
- context:
    cluster: \"$MASTER_FQDN\"
    user: \"$MASTER_FQDN-admin\"
  name: \"$MASTER_FQDN\"
current-context: \"$MASTER_FQDN\"
kind: Config
users:
- name: \"$MASTER_FQDN-admin\"
  user:
    client-certificate-data: \"$KUBECONFIG_CERTIFICATE\"
    client-key-data: \"$KUBECONFIG_KEY\"
" > $KUBECONFIGFILE
    set -x
}

# Wrapped as functions so logs_to_events can time each step; the install's
# bash -c command can't be passed to logs_to_events inline (it word-splits args).
disableSSH() {
    # On ubuntu, the ssh service is named "ssh.service"
    systemctlDisableAndStop ssh || exit $ERR_DISABLE_SSH
    # On AzureLinux, the ssh service is named "sshd.service"
    systemctlDisableAndStop sshd || exit $ERR_DISABLE_SSH
}

disableSSHPubkeyAuth() {
  local SSHD_CONFIG="${SSHD_CONFIG_FILE:-/etc/ssh/sshd_config}"
  local TMP
  TMP="$(mktemp)"

  # AAD SSH extension will append following section to the end of sshd_config,
  # so we need to check the "Match" section, and only update "PubkeyAuthentication" outside of it.
  # Match User *@*,????????-????-????-????-???????????? # Added by aadsshlogin installer
  # AuthenticationMethods publickey
  # PubkeyAuthentication yes
  # AuthorizedKeysCommand /usr/sbin/aad_certhandler %u %k
  # AuthorizedKeysCommandUser root
  awk -v desired="no" '
    BEGIN { in_match=0; replaced=0; inserted=0 }
    /^Match([[:space:]]|$)/ {
      if (!replaced && !inserted) { print "PubkeyAuthentication " desired; inserted=1 }
      in_match=1; print; next
    }
    (!in_match) && /^[[:space:]]*PubkeyAuthentication[[:space:]]+/ {
      print "PubkeyAuthentication " desired; replaced=1; next
    }
    { print }
    END { if (!replaced && !inserted) print "PubkeyAuthentication " desired }
  ' "$SSHD_CONFIG" > "$TMP"

  local ssh_service="sshd.service"
  if systemctl cat ssh.service >/dev/null 2>&1; then
    ssh_service="ssh.service"
  fi
  systemctl is-active --quiet "$ssh_service" || systemctl start "$ssh_service" || exit $ERR_CONFIG_PUBKEY_AUTH_SSH

  # Validate the candidate config
  sshd -t -f "$TMP" || { rm -f "$TMP"; exit $ERR_CONFIG_PUBKEY_AUTH_SSH; }

  # Replace the original with the candidate (permissions 600, owned by root)
  # Mode 0600 is required by CIS Benchmark control 5.1.1
  install -m 0600 -o root -g root "$TMP" "$SSHD_CONFIG"
  rm -f "$TMP"

  # Reload or restart ssh service
  systemctl reload-or-restart "$ssh_service" || exit $ERR_CONFIG_PUBKEY_AUTH_SSH
}

get_compute_sku() {
    # Retrieves the VM SKU (size) from the cached IMDS instance metadata.
    local vm_sku=""
    if [ ! -f "$IMDS_INSTANCE_METADATA_CACHE_FILE" ]; then
        echo "IMDS cache file not found: $IMDS_INSTANCE_METADATA_CACHE_FILE" >&2
        return 1
    fi
    vm_sku=$(jq -r '.compute.vmSize // empty' "$IMDS_INSTANCE_METADATA_CACHE_FILE")
    if [ -z "$vm_sku" ]; then
        echo "Failed to retrieve VM SKU from IMDS cache" >&2
        return 1
    fi
    echo "$vm_sku"
}

#EOF
