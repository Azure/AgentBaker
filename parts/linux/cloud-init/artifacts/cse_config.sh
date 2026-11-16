#!/bin/bash
NODE_INDEX=$(hostname | tail -c 2)
NODE_NAME=$(hostname)

# Older provisioners may not set module paths; default to siblings of this script.
source "${CSE_CONFIG_GPU_FILEPATH:-${BASH_SOURCE[0]%.sh}_gpu.sh}"
source "${CSE_CONFIG_LOCALDNS_FILEPATH:-${BASH_SOURCE[0]%.sh}_localdns.sh}"
source "${CSE_CONFIG_KUBELET_FILEPATH:-${BASH_SOURCE[0]%.sh}_kubelet.sh}"
source "${CSE_CONFIG_NETWORK_FILEPATH:-${BASH_SOURCE[0]%.sh}_network.sh}"
source "${CSE_CONFIG_CHRONY_FILEPATH:-${BASH_SOURCE[0]%.sh}_chrony.sh}"

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
            # client.key is the kubelet client private key; path is overridable for testing.
            # Create under a restrictive umask so the file is never briefly world-readable
            # between creation and chmod (avoids a TOCTOU window), then chmod again to cover
            # the case where the file already existed with looser permissions.
            local client_key_path="${CLIENT_KEY_PATH:-/etc/kubernetes/certs/client.key}"
            # Guard both operations: if either fails, stop before writing the private key
            # rather than silently falling through and writing it with whatever (possibly
            # world-readable) permissions the file ended up with.
            (umask 077 && touch "${client_key_path}") || return "$ERR_KUBELET_CLIENT_KEY_PERM_FAIL"
            chmod 0600 "${client_key_path}" || return "$ERR_KUBELET_CLIENT_KEY_PERM_FAIL"
            echo "${KUBELET_CLIENT_CONTENT}" | base64 -d > "${client_key_path}"
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

configureContainerdMountGuard() {
  local dropin="/etc/systemd/system/containerd.service.d/10-mount-guard.conf"
  if [ "${HAS_KUBELET_DISK_TYPE}" = "true" ]; then
    if ! retrycmd_if_failure 120 1 5 mountpoint -q /mnt; then
      echo "Timed out waiting for /mnt before configuring containerd" >&2
      return 1
    fi
    # Persist a live check for reboot/PIS starts that do not rerun basePrep.
    if ! tee "$dropin" > /dev/null <<'EOF'
[Service]
ExecStartPre=/bin/sh -c 'timeout 120 /bin/sh -c "until mountpoint -q /mnt; do sleep 1; done" || { echo "Unable to confirm /mnt mount before starting containerd" >&2; exit 1; }'
TimeoutStartSec=150
EOF
    then
      echo "Unable to configure the containerd temporary-disk mount guard" >&2
      return 1
    fi
  else
    if ! rm -f "$dropin"; then
      echo "Unable to remove the containerd mount guard for an OS disk" >&2
      return 1
    fi
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

  configureContainerdMountGuard || exit "$ERR_SYSTEMCTL_START_FAIL"

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
  local acr_mirror_setup="${ACR_MIRROR_SETUP_SCRIPT:-/opt/acr/tools/mirror/setup.sh}"
  if isACL "$OS" "$OS_VARIANT"; then
    validateACLArtifactStreamingIPECompatibility || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    local services=(acr-mirror.service overlaybd-tcmu.service overlaybd-snapshotter.service)
    # DALEC's Upholds= starts unmasked units as soon as the manager reloads.
    systemctl mask "${services[@]}" artifact-streaming-setup.service || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    installArtifactStreamingSysext || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    systemctl daemon-reload || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    systemctl unmask artifact-streaming-setup.service || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    systemctl start artifact-streaming-setup.service || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    # acr-config rejects ACL's absolute symlink, including a preserved GPU config.
    if test -L /etc/containerd/config.toml; then
      cp --remove-destination "$(readlink -f /etc/containerd/config.toml)" /etc/containerd/config.toml || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    fi
    "$acr_mirror_setup" aks || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    systemctl unmask "${services[@]}" || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    systemctl enable "${services[@]}" || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    retrycmd_if_failure 120 5 25 systemctl restart "${services[@]}" || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    systemctl restart containerd || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    waitForContainerdReady || exit "$ERR_ARTIFACT_STREAMING_INSTALL"
    return
  fi
  waitForContainerdReady || exit "$ERR_ARTIFACT_STREAMING_INSTALL"

  if [ -x "$acr_mirror_setup" ]; then
    "$acr_mirror_setup" aks || exit $ERR_ARTIFACT_STREAMING_INSTALL
  else
    echo "Older acr-mirror package is detected, using old acr-config enablement"
    # setup.sh is only available in acr-mirror 1.0.0 and above
    "${ACR_CONFIG_BIN:-/opt/acr/bin/acr-config}" --enable-containerd 'azurecr.io' || exit $ERR_ARTIFACT_STREAMING_INSTALL
  fi

  retrycmd_if_failure 120 5 25 systemctl --quiet enable --now acr-mirror overlaybd-tcmu overlaybd-snapshotter || exit $ERR_ARTIFACT_STREAMING_INSTALL
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

configureNodeProblemDetector() {
    local skip_file="${NPD_SKIP_FILE:-/etc/node-problem-detector.d/skip_vhd_npd}"
    if ! { isUbuntu || isAzureLinux; } || [ ! -f "${skip_file}" ]; then
        echo "Skipping baked NPD configuration: this node remains extension-managed"
        return 0
    fi
    # The MAI image path also uses the skip marker; only manage the PMC package
    # installed by AgentBaker here.
    local installed
    if isUbuntu; then
        installed="$(dpkg-query -W -f='${db:Status-Status}' node-problem-detector-aks-config 2>/dev/null)"
    else
        rpm -q node-problem-detector-aks-config >/dev/null 2>&1 && installed=installed
    fi
    if [ "${installed:-}" != installed ]; then
        echo "Skipping NPD activation: AgentBaker config package is not installed"
        return 0
    fi
    systemctlEnableAndStart node-problem-detector 30 || return $ERR_SYSTEMCTL_START_FAIL
}

# configureKMPP starts the KMPP service baked (masked) into Azure Linux 3.0 VHDs and exposes
# the host system bus to the KMPP CSI provider at /run/kmpp/dbus (hostPath /var/run/kmpp/dbus).
# The bind mount is unfiltered. Restricting which callers and methods it allows is planned separately.
#
# The RPM runs both KMPP services in kmpp.slice, which sits at the cgroup root, outside
# system.slice where AKS reserves CPU and memory for system daemons (SYSTEM_RESERVED_CGROUP).
# Drop-ins move them to system-kmpp.slice, a child of system.slice, with the RPM's limits.
# kmpp.slice is masked so that any unit still pointing at it fails to start instead of
# silently running outside the system-reserved budget.
configureKMPP() {
    local unit_dir="${KMPP_UNIT_DIR:-/usr/lib/systemd/system}"
    local etc_dir="${KMPP_SYSTEMD_ETC_DIR:-/etc/systemd/system}"
    local kmpp_service="dbus-com.microsoft.kmpp.service"
    local kmppctrl_service="dbus-com.microsoft.kmppctrl.service"
    local service

    if [ ! -f "${unit_dir}/${kmpp_service}" ]; then
        echo "KMPP is not baked into this VHD, skipping KMPP configuration"
        return 0
    fi

    mkdir -p "${etc_dir}/${kmpp_service}.d" "${etc_dir}/${kmppctrl_service}.d" /run/kmpp/dbus || return "$ERR_KMPP_START_FAIL"
    # Same limits as the RPM's kmpp.slice, shared by both services.
    cat > "${etc_dir}/system-kmpp.slice" <<EOF || return "$ERR_KMPP_START_FAIL"
[Unit]
Description=KMPP key isolation service

[Slice]
CPUAccounting=true
CPUQuota=50%
MemoryAccounting=true
MemoryMax=20M
EOF
    for service in "${kmpp_service}" "${kmppctrl_service}"; do
        cat > "${etc_dir}/${service}.d/10-system-slice.conf" <<EOF || return "$ERR_KMPP_START_FAIL"
[Service]
Slice=system-kmpp.slice
EOF
    done
    cat > "${etc_dir}/run-kmpp-dbus.mount" <<EOF || return "$ERR_KMPP_START_FAIL"
[Unit]
Description=Expose the host system bus to KMPP clients
DefaultDependencies=no
Requires=dbus.socket
After=dbus.socket
Conflicts=umount.target
Before=umount.target

[Mount]
What=/run/dbus
Where=/run/kmpp/dbus
Type=none
Options=bind

[Install]
WantedBy=multi-user.target
EOF
    chmod 0644 "${etc_dir}/system-kmpp.slice" "${etc_dir}/${kmpp_service}.d/10-system-slice.conf" \
        "${etc_dir}/${kmppctrl_service}.d/10-system-slice.conf" "${etc_dir}/run-kmpp-dbus.mount" || return "$ERR_KMPP_START_FAIL"
    retrycmd_if_failure 10 2 30 systemctl mask kmpp.slice || return "$ERR_KMPP_START_FAIL"
    systemctl daemon-reload || return "$ERR_KMPP_START_FAIL"

    retrycmd_if_failure 10 2 30 systemctl unmask "${kmpp_service}" "${kmppctrl_service}" || return "$ERR_KMPP_START_FAIL"
    systemctlEnableAndStart "${kmpp_service}" 30 || return "$ERR_KMPP_START_FAIL"
    retrycmd_if_failure 10 2 30 systemctl start "${kmppctrl_service}" || return "$ERR_KMPP_START_FAIL"
    systemctlEnableAndStart run-kmpp-dbus.mount 30 || return "$ERR_KMPP_START_FAIL"

    if ! retrycmd_if_failure 10 2 10 busctl --address=unix:path=/run/kmpp/dbus/system_bus_socket \
        call com.microsoft.kmpp / com.microsoft.kmpp GetVersion; then
        echo "KMPP is not reachable through /run/kmpp/dbus/system_bus_socket"
        return "$ERR_KMPP_START_FAIL"
    fi

    addKubeletNodeLabel "kubernetes.azure.com/kmpp=enabled"
}

ensureSysctl() {
    SYSCTL_CONFIG_FILE=/etc/sysctl.d/999-sysctl-aks.conf
    mkdir -p "$(dirname "${SYSCTL_CONFIG_FILE}")"
    touch "${SYSCTL_CONFIG_FILE}"
    chmod 0644 "${SYSCTL_CONFIG_FILE}"
    echo "${SYSCTL_CONTENT}" | base64 -d > "${SYSCTL_CONFIG_FILE}"
    retrycmd_if_failure 24 5 25 sysctl --system
}

createKubeManifestDir() {
    KUBEMANIFESTDIR=/etc/kubernetes/manifests
    mkdir -p $KUBEMANIFESTDIR
}

# Prefer socket activation when the distribution enables it; aliases resolve to Id.
getSSHActivation() {
    local unit load_state enabled_state active_state id candidate=""
    for unit in ssh.socket sshd.socket ssh.service sshd.service; do
        load_state=$(systemctl show "$unit" --property=LoadState --value) || return 1
        case "$load_state" in
            not-found|masked) continue ;;
            loaded) ;;
            *) echo "Cannot inspect SSH unit $unit: $load_state" >&2; return 1 ;;
        esac
        enabled_state=$(systemctl show "$unit" --property=UnitFileState --value) || return 1
        if [ "$enabled_state" = enabled ]; then
            id=$(systemctl show "$unit" --property=Id --value) || return 1
            printf '%s enabled\n' "$id"
            return 0
        fi
    done
    for unit in ssh.socket sshd.socket ssh.service sshd.service; do
        active_state=$(systemctl show "$unit" --property=ActiveState --value) || return 1
        if [ "$active_state" = active ]; then
            id=$(systemctl show "$unit" --property=Id --value) || return 1
            printf '%s disabled\n' "$id"
            return 0
        fi
    done
    [ "${1:-false}" = true ] || return 0
    # Legacy stop+disable leaves no enabled or active endpoint. Capture only
    # an unambiguous unmasked unit, without inventing enabled boot links.
    for unit in ssh.socket sshd.socket ssh.service sshd.service; do
        load_state=$(systemctl show "$unit" --property=LoadState --value) || return 1
        [ "$load_state" = loaded ] || continue
        id=$(systemctl show "$unit" --property=Id --value) || return 1
        if [ -n "$candidate" ] && [ "$candidate" != "$id" ]; then
            return 0
        fi
        candidate="$id"
    done
    [ -z "$candidate" ] || printf '%s disabled\n' "$candidate"
}

installSSHDisablePolicy() {
    local policy_dir="${SSH_POLICY_DIR:-/etc/aks/ssh-policy}"
    local unit_dir="${SSH_SYSTEMD_UNIT_DIR:-/etc/systemd/system}"
    local unit dropin content result=0
    mkdir -p "$policy_dir" || return 1
    chmod 0700 "$policy_dir" || return 1
    if [ ! -f "$policy_dir/activation" ]; then
        getSSHActivation true > "$policy_dir/activation.tmp" &&
            mv "$policy_dir/activation.tmp" "$policy_dir/activation" || return 1
    fi
    touch "$policy_dir/disabled" || return 1
    # Unlike disable (or a mask undone by openssh package migration), this
    # condition also blocks dependency and socket starts after a reboot.
    content="[Unit]
ConditionPathExists=!$policy_dir/disabled"
    for unit in ssh.socket sshd.socket ssh.service sshd.service; do
        dropin="$unit_dir/$unit.d/90-aks-ssh-access.conf"
        if [ -e "$dropin" ] && [ "$(cat "$dropin")" != "$content" ]; then
            echo "Refusing to overwrite an existing SSH policy: $dropin" >&2
            result=1
            continue
        fi
        mkdir -p "$unit_dir/$unit.d" &&
            printf '%s\n' "$content" > "$dropin.tmp" &&
            chmod 0644 "$dropin.tmp" &&
            mv "$dropin.tmp" "$dropin" || result=1
    done
    systemctl daemon-reload || result=1
    return "$result"
}

validateSSHActivation() {
    local unit="$1" load_state accept triggers service
    local -a services=()
    load_state=$(systemctl show "$unit" --property=LoadState --value) || return 1
    if [ "$load_state" != loaded ]; then
        echo "Cannot restore SSH: $unit is $load_state; external masks are not removed" >&2
        return 1
    fi
    case "$unit" in
        *.service) return 0 ;;
        *.socket)
            accept=$(systemctl show "$unit" --property=Accept --value) || return 1
            case "$accept" in
                yes)
                    # Inspect a template instance without starting a connection handler.
                    services=("${unit%.socket}@internal.service")
                    ;;
                no)
                    triggers=$(systemctl show "$unit" --property=Triggers --value) || return 1
                    read -r -a services <<< "$triggers"
                    ;;
                *) echo "Cannot restore SSH: invalid Accept state for $unit: $accept" >&2; return 1 ;;
            esac
            ;;
        *) echo "Cannot restore SSH: unsupported activation unit $unit" >&2; return 1 ;;
    esac
    if [ "${#services[@]}" -eq 0 ]; then
        echo "Cannot restore SSH: no backing service reported for $unit" >&2
        return 1
    fi
    for service in "${services[@]}"; do
        case "$service" in
            *.service) validateSSHActivation "$service" || return 1 ;;
            *) echo "Cannot restore SSH: invalid backing service for $unit: $service" >&2; return 1 ;;
        esac
    done
}

restoreSSH() {
    local policy_dir="${SSH_POLICY_DIR:-/etc/aks/ssh-policy}"
    local unit activation load_state id candidate="" owned_policy=false legacy_activation=false
    [ ! -f "$policy_dir/disabled" ] || owned_policy=true
    if [ "$owned_policy" = true ] && [ ! -f "$policy_dir/activation" ]; then
        echo "Cannot restore SSH: the owned activation record is missing" >&2
        return 1
    fi
    # Package upgrades may have migrated service activation to a socket while
    # our condition kept it blocked. Respect that mode before the saved mode.
    activation=$(getSSHActivation) || return 1
    if [ -z "$activation" ] && [ "$owned_policy" = true ]; then
        activation=$(cat "$policy_dir/activation") || return 1
    fi
    # Legacy CSE discarded the original service/socket mode. An explicit enable
    # may select a unique unmasked service, but never invent the previous mode.
    if [ -z "$activation" ]; then
        for unit in ssh.service sshd.service; do
            load_state=$(systemctl show "$unit" --property=LoadState --value) || return 1
            case "$load_state" in
                not-found) continue ;;
                loaded) ;;
                *) echo "Cannot restore SSH: $unit is $load_state; external masks are not removed" >&2; return 1 ;;
            esac
            id=$(systemctl show "$unit" --property=Id --value) || return 1
            if [ -n "$candidate" ] && [ "$candidate" != "$id" ]; then
                echo "Cannot restore SSH: multiple distinct services are installed" >&2
                return 1
            fi
            candidate="$id"
        done
        if [ -n "$candidate" ]; then
            activation="$candidate enabled"
            legacy_activation=true
        fi
    fi
    read -r unit activation <<< "$activation"
    case "$unit:$activation" in
        ssh.service:enabled|sshd.service:enabled|ssh.socket:enabled|sshd.socket:enabled|\
        ssh.service:disabled|sshd.service:disabled|ssh.socket:disabled|sshd.socket:disabled) ;;
        *) echo "Cannot restore SSH: no valid saved activation mode" >&2; return 1 ;;
    esac
    validateSSHActivation "$unit" || return 1
    if [ "$owned_policy" = true ] || [ "$legacy_activation" = true ]; then
        [ "$activation" != enabled ] || systemctl enable "$unit" || return 1
    fi
    if [ "$owned_policy" = true ]; then
        rm -f "$policy_dir/disabled" || return 1
    fi
    # Keep the owned drop-ins: without the marker their conditions allow starts.
    if ! systemctl start "$unit" ||
        ! systemctl is-active --quiet "$unit"; then
        if [ "$owned_policy" = true ]; then
            touch "$policy_dir/disabled" || return 1
            systemctl stop "$unit" || return 1
        fi
        echo "Failed to restore SSH activation through $unit" >&2
        return 1
    fi
    [ "$owned_policy" != true ] || rm -f "$policy_dir/activation"
}

applySSHPolicy() {
    local policy_dir="${SSH_POLICY_DIR:-/etc/aks/ssh-policy}"
    if [ "${DISABLE_SSH:-}" = true ]; then
        disableSSH
    elif [ "${DISABLE_PUBKEY_AUTH:-}" = true ]; then
        logs_to_events "AKS.CSE.disableSSHPubkeyAuth" disableSSHPubkeyAuth
    elif [ "${SHOULD_RESTORE_SSH:-}" = true ]; then
        if [ -f "$policy_dir/pubkey-auth" ]; then
            local pubkey_auth
            pubkey_auth=$(cat "$policy_dir/pubkey-auth") || return "$ERR_CONFIG_PUBKEY_AUTH_SSH"
            configureSSHPubkeyAuth "$pubkey_auth" || return "$ERR_CONFIG_PUBKEY_AUTH_SSH"
            if [ -f "$policy_dir/disabled" ]; then
                restoreSSH || return "$ERR_DISABLE_SSH"
            fi
            reloadSSHAuthentication false || return "$ERR_CONFIG_PUBKEY_AUTH_SSH"
            rm -f "$policy_dir/pubkey-auth" || return "$ERR_CONFIG_PUBKEY_AUTH_SSH"
        elif [ -f "$policy_dir/disabled" ]; then
            restoreSSH || return "$ERR_DISABLE_SSH"
        fi
    fi
}

disableSSH() {
    local start_time attempt_id unit result=0
    start_time=$(date -u +"%F %T.%3N")
    attempt_id="$(date +%s%N)-$RANDOM"
    emitSSHDisableEvent "$attempt_id" Started "" "$start_time" null '{}' ||
        echo "WARNING: SSH disable telemetry failed" >&2
    installSSHDisablePolicy || { echo "Failed to install persistent SSH disable policy" >&2; result=$ERR_DISABLE_SSH; }
    # Ubuntu uses ssh, Azure Linux uses sshd, and ACL also uses sshd.socket.
    # Close the listening socket first so no new connection is accepted mid-teardown.
    # Keep going after a failure so a later unit still gets disabled, on ACL the
    # socket owns port 22 and skipping it would leave SSH reachable.
    local load_state active_state
    for unit in ssh.socket sshd.socket ssh sshd; do
        if ! load_state=$(systemctl show "$unit" --property=LoadState --value); then
            echo "Failed to inspect SSH unit $unit" >&2
            result=$ERR_DISABLE_SSH
            continue
        fi
        case "$load_state" in
            loaded|not-found)
                systemctlDisableAndStop "$unit" "$attempt_id" || result=$ERR_DISABLE_SSH
                ;;
            masked)
                # A masked unit can still be running. Never remove external masks.
                systemctl_stop 20 5 25 "$unit" || result=$ERR_DISABLE_SSH
                ;;
            *)
                echo "Cannot disable SSH unit $unit: $load_state" >&2
                result=$ERR_DISABLE_SSH
                ;;
        esac
        [ "$load_state" != not-found ] || continue
        active_state=$(systemctl show "$unit" --property=ActiveState --value) || result=$ERR_DISABLE_SSH
        if [ "$active_state" != inactive ]; then
            echo "SSH unit $unit is not inactive: $active_state" >&2
            result=$ERR_DISABLE_SSH
        fi
    done
    emitSSHDisableEvent "$attempt_id" Completed "" "$start_time" "$result" '{}' ||
        echo "WARNING: SSH disable telemetry failed" >&2
    [ "$result" -eq 0 ] || exit "$result"
}

configureSSHPubkeyAuth() {
  local desired="$1"
  case "$desired" in
    yes|no) ;;
    *) echo "Invalid SSH public-key authentication policy: $desired" >&2; return 1 ;;
  esac
  local SSHD_CONFIG="${SSHD_CONFIG_FILE:-/etc/ssh/sshd_config}"
  local TMP
  TMP="$(mktemp)" || return 1

  # AAD SSH extension will append following section to the end of sshd_config,
  # so we need to check the "Match" section, and only update "PubkeyAuthentication" outside of it.
  # Match User *@*,????????-????-????-????-???????????? # Added by aadsshlogin installer
  # AuthenticationMethods publickey
  # PubkeyAuthentication yes
  # AuthorizedKeysCommand /usr/sbin/aad_certhandler %u %k
  # AuthorizedKeysCommandUser root
  awk -v desired="$desired" '
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
  ' "$SSHD_CONFIG" > "$TMP" || { rm -f "$TMP"; return 1; }

  # Ubuntu removes this runtime directory when ssh.service stops. Validation
  # must not start the server merely to recreate its privilege-separation path.
  install -d -m 0755 -o root -g root /run/sshd || { rm -f "$TMP"; return 1; }

  # Validate syntax and the effective global policy, including Include files.
  sshd -t -f "$TMP" || { rm -f "$TMP"; return 1; }
  local effective
  effective=$(sshd -T -f "$TMP") || { rm -f "$TMP"; return 1; }
  if [ "$(awk 'tolower($1) == "pubkeyauthentication" { print $2 }' <<< "$effective")" != "$desired" ]; then
    echo "SSH configuration does not enforce the requested public-key policy" >&2
    rm -f "$TMP"
    return 1
  fi

  # Replace the original with the candidate (permissions 600, owned by root)
  # Mode 0600 is required by CIS Benchmark control 5.1.1
  install -m 0600 -o root -g root "$TMP" "$SSHD_CONFIG" || { rm -f "$TMP"; return 1; }
  rm -f "$TMP"
}

reloadSSHAuthentication() {
  local start_if_inactive="${1:-true}"
  local ssh_service=sshd.service
  if systemctl cat ssh.service >/dev/null 2>&1; then
    ssh_service=ssh.service
  fi
  if ! systemctl is-active --quiet "$ssh_service"; then
    [ "$start_if_inactive" != false ] || return 0
    if ! systemctl is-active --quiet ssh.socket &&
        ! systemctl is-active --quiet sshd.socket; then
      systemctl start "$ssh_service" || return 1
    else
      return 0
    fi
  fi
  systemctl reload-or-restart "$ssh_service"
}

disableSSHPubkeyAuth() {
  local policy_dir="${SSH_POLICY_DIR:-/etc/aks/ssh-policy}"
  local original
  mkdir -p "$policy_dir" || exit "$ERR_CONFIG_PUBKEY_AUTH_SSH"
  chmod 0700 "$policy_dir" || exit "$ERR_CONFIG_PUBKEY_AUTH_SSH"
  if [ ! -f "$policy_dir/pubkey-auth" ]; then
    install -d -m 0755 -o root -g root /run/sshd || exit "$ERR_CONFIG_PUBKEY_AUTH_SSH"
    original=$(sshd -T -f "${SSHD_CONFIG_FILE:-/etc/ssh/sshd_config}") || exit "$ERR_CONFIG_PUBKEY_AUTH_SSH"
    original=$(awk 'tolower($1) == "pubkeyauthentication" { print $2 }' <<< "$original")
    case "$original" in
      yes|no) ;;
      *) echo "Cannot determine the original SSH public-key policy" >&2; exit "$ERR_CONFIG_PUBKEY_AUTH_SSH" ;;
    esac
    printf '%s\n' "$original" > "$policy_dir/pubkey-auth" || exit "$ERR_CONFIG_PUBKEY_AUTH_SSH"
  fi
  configureSSHPubkeyAuth no || exit "$ERR_CONFIG_PUBKEY_AUTH_SSH"
  restoreSSH || exit "$ERR_CONFIG_PUBKEY_AUTH_SSH"
  reloadSSHAuthentication || exit "$ERR_CONFIG_PUBKEY_AUTH_SSH"
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

enableAndStartCgroupTelemetry() {
    local units=() unit

    # Don't enable cgroup-memory-telemetry on AzureLinux due to incompatibility with cgroup2fs driver and absence of required azure.slice directory
    if ! isMarinerOrAzureLinux "$OS"; then
        units+=(cgroup-memory-telemetry.timer cgroup-memory-telemetry.service)
    fi

    if [ "$(stat -fc %T /sys/fs/cgroup)" = "cgroup2fs" ]; then
        units+=(cgroup-cpu-telemetry.timer cgroup-cpu-telemetry.service
            cgroup-pressure-telemetry.timer cgroup-pressure-telemetry.service)
    fi

    for unit in "${units[@]}"; do
        if systemctl cat "$unit" &>/dev/null; then
            systemctlEnableAndStartNoBlock "$unit" 30 || exit $ERR_START_CGROUP_TELEMETRY
        else
            echo "$unit not found on this VHD, skipping"
        fi
    done
}
