#!/bin/bash
NODE_INDEX=$(hostname | tail -c 2)
NODE_NAME=$(hostname)

configureAdminUser(){
    chage -E -1 -I -1 -m 0 -M 99999 "${ADMINUSER}"
    chage -l "${ADMINUSER}"
    chage -I -1 -M -1 root
}

configPrivateClusterHosts() {
    mkdir -p /etc/systemd/system/reconcile-private-hosts.service.d/
    touch /etc/systemd/system/reconcile-private-hosts.service.d/10-fqdn.conf
    tee /etc/systemd/system/reconcile-private-hosts.service.d/10-fqdn.conf > /dev/null <<EOF
[Service]
Environment="KUBE_API_SERVER_NAME=${API_SERVER_NAME}"
EOF
  systemctlEnableAndStart reconcile-private-hosts 30 || exit $ERR_SYSTEMCTL_START_FAIL
}
configureTransparentHugePage() {
    ETC_SYSFS_CONF="/etc/sysfs.conf"
    if [ -n "${THP_ENABLED}" ]; then
        echo "${THP_ENABLED}" > /sys/kernel/mm/transparent_hugepage/enabled
        echo "kernel/mm/transparent_hugepage/enabled=${THP_ENABLED}" >> ${ETC_SYSFS_CONF}
    fi
    if [ -n "${THP_DEFRAG}" ]; then
        echo "${THP_DEFRAG}" > /sys/kernel/mm/transparent_hugepage/defrag
        echo "kernel/mm/transparent_hugepage/defrag=${THP_DEFRAG}" >> ${ETC_SYSFS_CONF}
    fi
}

configureSystemdUseDomains() {
    NETWORK_CONFIG_FILE="/etc/systemd/networkd.conf"

    if awk '/^\[DHCPv4\]/{flag=1; next} /^\[/{flag=0} flag && /#UseDomains=no/' "$NETWORK_CONFIG_FILE"; then
        sed -i '/^\[DHCPv4\]/,/^\[/ s/#UseDomains=no/UseDomains=yes/' $NETWORK_CONFIG_FILE
    fi

    if [ "${IPV6_DUAL_STACK_ENABLED}" = "true" ]; then
        if awk '/^\[DHCPv6\]/{flag=1; next} /^\[/{flag=0} flag && /#UseDomains=no/' "$NETWORK_CONFIG_FILE"; then
            sed -i '/^\[DHCPv6\]/,/^\[/ s/#UseDomains=no/UseDomains=yes/' $NETWORK_CONFIG_FILE
        fi
    fi

    # Restart systemd networkd service
    systemctl restart systemd-networkd

    # Restart rsyslog service to display the correct hostname in log
    systemctl restart rsyslog
}

configureSwapFile() {
    # https://learn.microsoft.com/en-us/troubleshoot/azure/virtual-machines/troubleshoot-device-names-problems#identify-disk-luns
    swap_size_kb=$(expr ${SWAP_FILE_SIZE_MB} \* 1000)
    swap_location=""

    # Attempt to use the resource disk
    if [ -L /dev/disk/azure/resource-part1 ]; then
        resource_disk_path=$(findmnt -nr -o target -S $(readlink -f /dev/disk/azure/resource-part1))
        disk_free_kb=$(df ${resource_disk_path} | sed 1d | awk '{print $4}')
        if [ "${disk_free_kb}" -gt "${swap_size_kb}" ]; then
            echo "Will use resource disk for swap file"
            swap_location=${resource_disk_path}/swapfile
        else
            echo "Insufficient disk space on resource disk to create swap file: request ${swap_size_kb} free ${disk_free_kb}, attempting to fall back to OS disk..."
        fi
    fi

    # If we couldn't use the resource disk, attempt to use the OS disk
    if [ -z "${swap_location}" ]; then
        # Directly check size on the root directory since we can't rely on 'root-part1' always being the correct label
        os_device=$(readlink -f /dev/disk/azure/root)
        disk_free_kb=$(df -P / | sed 1d | awk '{print $4}')
        if [ "${disk_free_kb}" -gt "${swap_size_kb}" ]; then
            echo "Will use OS disk for swap file"
            swap_location=/swapfile
        else
            echo "Insufficient disk space on OS device ${os_device} to create swap file: request ${swap_size_kb} free ${disk_free_kb}"
            exit $ERR_SWAP_CREATE_INSUFFICIENT_DISK_SPACE
        fi
    fi

    echo "Swap file will be saved to: ${swap_location}"
    retrycmd_if_failure 24 5 25 fallocate -l ${swap_size_kb}K ${swap_location} || exit $ERR_SWAP_CREATE_FAIL
    chmod 600 ${swap_location}
    retrycmd_if_failure 24 5 25 mkswap ${swap_location} || exit $ERR_SWAP_CREATE_FAIL
    retrycmd_if_failure 24 5 25 swapon ${swap_location} || exit $ERR_SWAP_CREATE_FAIL
    retrycmd_if_failure 24 5 25 swapon --show | grep ${swap_location} || exit $ERR_SWAP_CREATE_FAIL
    echo "${swap_location} none swap sw 0 0" >> /etc/fstab
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

configureCNI() {
    # needed for bridge iptables rules and customer-configured conntrack sysctls
    retrycmd_if_failure 120 5 25 modprobe -a br_netfilter nf_conntrack || exit $ERR_MODPROBE_FAIL
    echo -n "br_netfilter" > /etc/modules-load.d/br_netfilter.conf
    echo -n "nf_conntrack" > /etc/modules-load.d/nf_conntrack.conf
    configureCNIIPTables
}

configureCNIIPTables() {
    if [ "${NETWORK_PLUGIN}" = "azure" ]; then
        mv $CNI_BIN_DIR/10-azure.conflist $CNI_CONFIG_DIR/
        chmod 600 $CNI_CONFIG_DIR/10-azure.conflist
        if [ "${NETWORK_POLICY}" = "calico" ]; then
          sed -i 's#"mode":"bridge"#"mode":"transparent"#g' $CNI_CONFIG_DIR/10-azure.conflist
        elif [ -n "${NETWORK_POLICY}" ] || [ "${NETWORK_POLICY}" = "none" ] && [ "${NETWORK_MODE}" = "transparent" ]; then
          sed -i 's#"mode":"bridge"#"mode":"transparent"#g' $CNI_CONFIG_DIR/10-azure.conflist
        fi
        /sbin/ebtables -t nat --list
    fi
}

disableSystemdResolved() {
    ls -ltr /etc/resolv.conf
    cat /etc/resolv.conf
    UBUNTU_RELEASE=$(lsb_release -r -s 2>/dev/null || echo "")
    if [ "${UBUNTU_RELEASE}" = "20.04" ] || [ "${UBUNTU_RELEASE}" = "22.04" ] || [ "${UBUNTU_RELEASE}" = "24.04" ]; then
        echo "Ingoring systemd-resolved query service but using its resolv.conf file"
        echo "This is the simplest approach to workaround resolved issues without completely uninstall it"
        [ -f /run/systemd/resolve/resolv.conf ] && ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
        ls -ltr /etc/resolv.conf
        cat /etc/resolv.conf
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

ensureNoDupOnPromiscuBridge() {
    systemctlEnableAndStart ensure-no-dup 30 || exit $ERR_SYSTEMCTL_START_FAIL
}

ensureArtifactStreaming() {
  waitForContainerdReady || exit $ERR_ARTIFACT_STREAMING_INSTALL
  retrycmd_if_failure 120 5 25 systemctl --quiet enable --now acr-mirror overlaybd-tcmu overlaybd-snapshotter
  /opt/acr/bin/acr-config --enable-containerd 'azurecr.io'
}

ensureDHCPv6() {
    systemctlEnableAndStart dhcpv6 30 || exit $ERR_SYSTEMCTL_START_FAIL
    retrycmd_if_failure 120 5 25 modprobe ip6_tables || exit $ERR_MODPROBE_FAIL
}

getPrimaryNicIP() {
    local ip=""
    export -f get_primary_nic_ip
    ip=$(get_primary_nic_ip)
    echo "${ip}"
}

generateSelfSignedKubeletServingCertificate() {
    mkdir -p "/etc/kubernetes/certs"

    KUBELET_SERVER_PRIVATE_KEY_PATH="/etc/kubernetes/certs/kubeletserver.key"
    KUBELET_SERVER_CERT_PATH="/etc/kubernetes/certs/kubeletserver.crt"

    openssl genrsa -out $KUBELET_SERVER_PRIVATE_KEY_PATH 2048
    openssl req -new -x509 -days 7300 -key $KUBELET_SERVER_PRIVATE_KEY_PATH -out $KUBELET_SERVER_CERT_PATH -subj "/CN=${NODE_NAME}" -addext "subjectAltName=DNS:${NODE_NAME}"
}

configureKubeletServing() {
    if [ "${ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION}" != "true" ]; then
        echo "kubelet serving certificate rotation is disabled, generating self-signed serving certificate with openssl"
        generateSelfSignedKubeletServingCertificate
        return 0
    fi

    KUBELET_SERVING_CERTIFICATE_ROTATION_LABEL="kubernetes.azure.com/kubelet-serving-ca=cluster"
    KUBELET_SERVER_PRIVATE_KEY_PATH="/etc/kubernetes/certs/kubeletserver.key"
    KUBELET_SERVER_CERT_PATH="/etc/kubernetes/certs/kubeletserver.crt"

    # check if kubelet serving certificate rotation is disabled by customer-specified nodepool tags
    export -f should_disable_kubelet_serving_certificate_rotation
    DISABLE_KUBELET_SERVING_CERTIFICATE_ROTATION=$(should_disable_kubelet_serving_certificate_rotation)
    if [ "$?" -ne 0 ]; then
        echo "failed to determine if kubelet serving certificate rotation should be disabled by nodepool tags"
        exit $ERR_LOOKUP_DISABLE_KUBELET_SERVING_CERTIFICATE_ROTATION_TAG
    fi

    if [ "${DISABLE_KUBELET_SERVING_CERTIFICATE_ROTATION}" = "true" ]; then
        echo "kubelet serving certificate rotation is disabled by nodepool tags"

        # set --rotate-server-certificates flag and serverTLSBootstrap config file field to false
        echo "reconfiguring kubelet flags and config as needed"
        KUBELET_FLAGS="${KUBELET_FLAGS/--rotate-server-certificates=true/--rotate-server-certificates=false}"
        if [ "${KUBELET_CONFIG_FILE_ENABLED}" = "true" ]; then
            set +x
            KUBELET_CONFIG_FILE_CONTENT=$(echo "$KUBELET_CONFIG_FILE_CONTENT" | base64 -d | jq 'if .serverTLSBootstrap == true then .serverTLSBootstrap = false else . end' | base64)
            set -x
        fi

        # manually generate kubelet's self-signed serving certificate
        echo "generating self-signed serving certificate with openssl"
        generateSelfSignedKubeletServingCertificate

        # make sure to eliminate the kubelet serving node label
        echo "removing node label $KUBELET_SERVING_CERTIFICATE_ROTATION_LABEL"
        removeKubeletNodeLabel $KUBELET_SERVING_CERTIFICATE_ROTATION_LABEL
    else
        echo "kubelet serving certificate rotation is enabled"

        # remove the --tls-cert-file and --tls-private-key-file flags, which are incompatible with serving certificate rotation
        # NOTE: this step will not be needed once these flags are no longer defaulted by the bootstrapper
        echo "removing --tls-cert-file and --tls-private-key-file from kubelet flags"
        removeKubeletFlag "--tls-cert-file=$KUBELET_SERVER_CERT_PATH"
        removeKubeletFlag "--tls-private-key-file=$KUBELET_SERVER_PRIVATE_KEY_PATH"
        if [ "${KUBELET_CONFIG_FILE_ENABLED}" = "true" ]; then
            set +x
            KUBELET_CONFIG_FILE_CONTENT=$(echo "$KUBELET_CONFIG_FILE_CONTENT" | base64 -d | jq 'del(.tlsCertFile)' | jq 'del(.tlsPrivateKeyFile)' | base64)
            set -x
        fi

        # make sure to add the kubelet serving node label
        echo "adding node label $KUBELET_SERVING_CERTIFICATE_ROTATION_LABEL if needed"
        addKubeletNodeLabel $KUBELET_SERVING_CERTIFICATE_ROTATION_LABEL
    fi
}

ensureKubeCACert() {
    KUBE_CA_FILE="/etc/kubernetes/certs/ca.crt"
    mkdir -p "$(dirname "${KUBE_CA_FILE}")"
    echo "${KUBE_CA_CRT}" | base64 -d > "${KUBE_CA_FILE}"
    chmod 0600 "${KUBE_CA_FILE}"
}

# file paths defined outside so configureAndEnableSecureTLSBootstrapping can be unit tested
SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE="/etc/default/secure-tls-bootstrap"
SECURE_TLS_BOOTSTRAPPING_DROP_IN="/etc/systemd/system/secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf"
configureAndEnableSecureTLSBootstrapping() {
    BOOTSTRAP_CLIENT_FLAGS="--aad-resource=${SECURE_TLS_BOOTSTRAPPING_AAD_RESOURCE:-$AKS_AAD_SERVER_APP_ID} --apiserver-fqdn=${API_SERVER_NAME} --cloud-provider-config=${AZURE_JSON_PATH}"
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_USER_ASSIGNED_IDENTITY_ID}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --user-assigned-identity-id=$SECURE_TLS_BOOTSTRAPPING_USER_ASSIGNED_IDENTITY_ID"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_VALIDATE_KUBECONFIG_TIMEOUT}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --validate-kubeconfig-timeout=${SECURE_TLS_BOOTSTRAPPING_VALIDATE_KUBECONFIG_TIMEOUT}"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_GET_ACCESS_TOKEN_TIMEOUT}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --get-access-token-timeout=${SECURE_TLS_BOOTSTRAPPING_GET_ACCESS_TOKEN_TIMEOUT}"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_GET_INSTANCE_DATA_TIMEOUT}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --get-instance-data-timeout=${SECURE_TLS_BOOTSTRAPPING_GET_INSTANCE_DATA_TIMEOUT}"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_GET_NONCE_TIMEOUT}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --get-nonce-timeout=${SECURE_TLS_BOOTSTRAPPING_GET_NONCE_TIMEOUT}"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_GET_ATTESTED_DATA_TIMEOUT}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --get-attested-data-timeout=${SECURE_TLS_BOOTSTRAPPING_GET_ATTESTED_DATA_TIMEOUT}"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_GET_CREDENTIAL_TIMEOUT}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --get-credential-timeout=${SECURE_TLS_BOOTSTRAPPING_GET_CREDENTIAL_TIMEOUT}"
    fi
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_DEADLINE}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --deadline=${SECURE_TLS_BOOTSTRAPPING_DEADLINE}"
    fi

    mkdir -p "$(dirname "${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE}")"
    touch "${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE}"
    chmod 0600 "${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE}"
    echo "BOOTSTRAP_FLAGS=${BOOTSTRAP_CLIENT_FLAGS}" > "${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE}"
    if [ -n "${AZURE_ENVIRONMENT_FILEPATH}" ]; then
        echo "AZURE_ENVIRONMENT_FILEPATH=${AZURE_ENVIRONMENT_FILEPATH}" >> "${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE}"
    fi

    mkdir -p "$(dirname "${SECURE_TLS_BOOTSTRAPPING_DROP_IN}")"
    touch "${SECURE_TLS_BOOTSTRAPPING_DROP_IN}"
    chmod 0600 "${SECURE_TLS_BOOTSTRAPPING_DROP_IN}"
    cat > "${SECURE_TLS_BOOTSTRAPPING_DROP_IN}" <<EOF
[Unit]
Before=kubelet.service
[Service]
EnvironmentFile=${SECURE_TLS_BOOTSTRAPPING_DEFAULT_FILE}
[Install]
# this configuration has secure-tls-bootstrap.service only start when kubelet.service is started
# once bootstrap tokens are no longer a fallback, kubelet.service needs to be a RequiredBy=
WantedBy=kubelet.service
EOF

    # enable the service so it runs ahead of kubelet on next boot; do not start it now
    if ! retrycmd_if_failure 120 5 25 systemctl enable secure-tls-bootstrap; then
        echo "secure-tls-bootstrap could not be enabled by systemctl"
        exit $ERR_SECURE_TLS_BOOTSTRAP_ENABLE_FAILURE
    fi

    # once bootstrap tokens are no longer a fallback, we can unset TLS_BOOTSTRAP_TOKEN here if needed
}

configureKubeletAndKubectl() {
    # Install kubelet and kubectl binaries from URL:
    # 1. For Custom Kube binary or Private Kube binary.
    # 2. If k8s version < 1.34.0, skip_bypass_k8s_version_check != true, and not Flatcar (which falls back to URL later).
    # 3. For Azure Linux v2 due to lack of PMC packages (if not network isolated).
    if [ -n "${CUSTOM_KUBE_BINARY_DOWNLOAD_URL}" ] || [ -n "${PRIVATE_KUBE_BINARY_DOWNLOAD_URL}" ] ||
       { ! isFlatcar && ! isACL && [ "${SHOULD_ENFORCE_KUBE_PMC_INSTALL}" != true ] && ! semverCompare "${KUBERNETES_VERSION:-0.0.0}" 1.34.0; } ||
       { isMarinerOrAzureLinux && [ "${OS_VERSION}" = 2.0 ] && [ -z "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; }
    then
        logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlFromURL" installKubeletKubectlFromURL
    elif [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
        logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlFromBootstrapProfileRegistry" "installKubeletKubectlFromBootstrapProfileRegistry ${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER} ${KUBERNETES_VERSION}"
    elif [ "$(type -t installKubeletKubectlFromPkg)" = function ]; then
        logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlFromPkg" "installKubeletKubectlFromPkg ${KUBERNETES_VERSION}"
    else
        echo "installKubeletKubectlFromPkg is not defined for this OS"
        exit $ERR_K8S_INSTALL_ERR
    fi
}

ensurePodInfraContainerImage() {
    POD_INFRA_CONTAINER_IMAGE_DOWNLOAD_DIR="/opt/pod-infra-container-image/downloads"
    POD_INFRA_CONTAINER_IMAGE_TAR="/opt/pod-infra-container-image/pod-infra-container-image.tar"

    waitForContainerdReady || exit $ERR_PULL_POD_INFRA_CONTAINER_IMAGE

    pod_infra_container_image=$(get_sandbox_image)

    if [ -z "${pod_infra_container_image}" ]; then
        echo "Failed to recognize pod infra container image"
        exit $ERR_PULL_POD_INFRA_CONTAINER_IMAGE
    fi

    echo "Checking if $pod_infra_container_image already exists locally..."
    if ctr -n k8s.io images list -q | grep -q "^${pod_infra_container_image}$"; then
        echo "Image $pod_infra_container_image already exists locally, skipping pull"
        return 0
    fi
    base_name="${pod_infra_container_image%@:*}"
    base_name="${pod_infra_container_image%:*}"
    tag="local"

    image="${pod_infra_container_image//mcr.microsoft.com/${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}}"
    acr_url=$(echo "$image" | cut -d/ -f1)

    mkdir -p ${POD_INFRA_CONTAINER_IMAGE_DOWNLOAD_DIR}

    echo "Pulling with authentication for $image"
    retrycmd_cp_oci_layout_with_oras 10 5 "${POD_INFRA_CONTAINER_IMAGE_DOWNLOAD_DIR}" "$tag" "$image" || exit $ERR_PULL_POD_INFRA_CONTAINER_IMAGE

    tar -cvf ${POD_INFRA_CONTAINER_IMAGE_TAR} -C ${POD_INFRA_CONTAINER_IMAGE_DOWNLOAD_DIR} .
    if ctr -n k8s.io image import --base-name $base_name ${POD_INFRA_CONTAINER_IMAGE_TAR}; then
        ctr -n k8s.io image tag "${base_name}:${tag}" "${pod_infra_container_image}"
        echo "Successfully imported $pod_infra_container_image"
        labelContainerImage "${pod_infra_container_image}" "io.cri-containerd.pinned" "pinned"
    else
        echo "Failed to import $pod_infra_container_image"
        exit $ERR_PULL_POD_INFRA_CONTAINER_IMAGE
    fi

    rm -rf ${POD_INFRA_CONTAINER_IMAGE_DOWNLOAD_DIR}
    rm -f ${POD_INFRA_CONTAINER_IMAGE_TAR}
}

validateKubeletNodeLabels() {
    local labels="$1"
    local validated_labels=""
    local delimiter=""

    # Return empty if no labels provided
    if [ -z "$labels" ]; then
        echo "No labels found in KUBELET_NODE_LABELS"
        return 0
    fi

    # Split labels by comma and process each
    IFS=',' read -ra LABEL_ARRAY <<< "$labels"
    for label in "${LABEL_ARRAY[@]}"; do
        # Split each label into key and value
        # shellcheck disable=SC3010
        if [[ "$label" == *"="* ]]; then
            key="${label%%=*}"
            value="${label#*=}"

            # Check if key length exceeds 63 characters
            if [ ${#key} -gt 63 ]; then
                echo "Warning: Label key '$key' exceeds 63 characters, truncating to 63 characters" >&2
                key="${key:0:63}"
            fi

            # Rebuild the label with potentially truncated key
            validated_labels="${validated_labels}${delimiter}${key}=${value}"
        fi

        # Set delimiter for subsequent labels
        delimiter=","
    done

    # Update the global variable with validated labels
    KUBELET_NODE_LABELS="$validated_labels"
}

ensureKubelet() {
    KUBELET_DEFAULT_FILE=/etc/default/kubelet
    mkdir -p /etc/default

    # In k8s >= 1.29 kubelet no longer sets node internalIP when using external cloud provider
    # https://github.com/kubernetes/kubernetes/pull/121028
    # This regresses node startup performance in Azure CNI Overlay and Podsubnet clusters, which require the node to be
    # assigned an internal IP before configuring pod networking.
    # To improve node startup performance, explicitly set `--node-ip` to the IP returned from IMDS so kubelet sets
    # the internal IP when it registers the node.
    # If this fails, skip setting --node-ip, which is safe because cloud-node-manager will assign it later anyway.
    if semverCompare ${KUBERNETES_VERSION:-"0.0.0"} "1.29.0"; then
        logs_to_events "AKS.CSE.ensureKubelet.setKubeletNodeIPFlag" setKubeletNodeIPFlag
    fi

    # systemd watchdog support was added in 1.32.0: https://github.com/kubernetes/kubernetes/pull/127566
    # This is needed to ensure kubelet is restarted if it becomes unresponsive
    if semverCompare ${KUBERNETES_VERSION:-"0.0.0"} "1.32.0"; then
        tee "/etc/systemd/system/kubelet.service.d/10-watchdog.conf" > /dev/null <<'EOF'
[Service]
WatchdogSec=60s
EOF
    fi

    echo "KUBELET_FLAGS=${KUBELET_FLAGS}" > "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_REGISTER_SCHEDULABLE=true" >> "${KUBELET_DEFAULT_FILE}"
    echo "NETWORK_POLICY=${NETWORK_POLICY}" >> "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_IMAGE=${KUBELET_IMAGE}" >> "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_NODE_LABELS=${KUBELET_NODE_LABELS}" >> "${KUBELET_DEFAULT_FILE}"
    if [ -n "${AZURE_ENVIRONMENT_FILEPATH}" ]; then
        echo "AZURE_ENVIRONMENT_FILEPATH=${AZURE_ENVIRONMENT_FILEPATH}" >> "${KUBELET_DEFAULT_FILE}"
    fi
    chmod 0600 "${KUBELET_DEFAULT_FILE}"

    BOOTSTRAP_KUBECONFIG_FILE=/var/lib/kubelet/bootstrap-kubeconfig

    # to ensure we don't expose bootstrap token secrets in provisioning logs
    set +x

    if [ -n "${TLS_BOOTSTRAP_TOKEN:-}" ]; then
        echo "using bootstrap token to generate a bootstrap-kubeconfig"

        CREDENTIAL_VALIDATION_DROP_IN="/etc/systemd/system/kubelet.service.d/10-credential-validation.conf"
        mkdir -p "$(dirname "${CREDENTIAL_VALIDATION_DROP_IN}")"
        touch "${CREDENTIAL_VALIDATION_DROP_IN}"
        chmod 0600 "${CREDENTIAL_VALIDATION_DROP_IN}"
        tee "${CREDENTIAL_VALIDATION_DROP_IN}" > /dev/null <<EOF
[Service]
Environment="CREDENTIAL_VALIDATION_KUBE_CA_FILE=/etc/kubernetes/certs/ca.crt"
Environment="CREDENTIAL_VALIDATION_APISERVER_URL=https://${API_SERVER_NAME}:443"
EOF

        KUBELET_TLS_DROP_IN="/etc/systemd/system/kubelet.service.d/10-tlsbootstrap.conf"
        mkdir -p "$(dirname "${KUBELET_TLS_DROP_IN}")"
        touch "${KUBELET_TLS_DROP_IN}"
        chmod 0600 "${KUBELET_TLS_DROP_IN}"
        tee "${KUBELET_TLS_DROP_IN}" > /dev/null <<EOF
[Service]
Environment="KUBELET_TLS_BOOTSTRAP_FLAGS=--kubeconfig /var/lib/kubelet/kubeconfig --bootstrap-kubeconfig /var/lib/kubelet/bootstrap-kubeconfig"
EOF
        mkdir -p "$(dirname "${BOOTSTRAP_KUBECONFIG_FILE}")"
        touch "${BOOTSTRAP_KUBECONFIG_FILE}"
        chmod 0644 "${BOOTSTRAP_KUBECONFIG_FILE}"
        tee "${BOOTSTRAP_KUBECONFIG_FILE}" > /dev/null <<EOF
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://${API_SERVER_NAME}:443
users:
- name: kubelet-bootstrap
  user:
    token: "${TLS_BOOTSTRAP_TOKEN:-}"
contexts:
- context:
    cluster: localcluster
    user: kubelet-bootstrap
  name: bootstrap-context
current-context: bootstrap-context
EOF
    else
        echo "generating kubeconfig referencing the provided kubelet client certificate"

        KUBECONFIG_FILE=/var/lib/kubelet/kubeconfig
        mkdir -p "$(dirname "${KUBECONFIG_FILE}")"
        touch "${KUBECONFIG_FILE}"
        chmod 0644 "${KUBECONFIG_FILE}"
        tee "${KUBECONFIG_FILE}" > /dev/null <<EOF
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://${API_SERVER_NAME}:443
users:
- name: client
  user:
    client-certificate: /etc/kubernetes/certs/client.crt
    client-key: /etc/kubernetes/certs/client.key
contexts:
- context:
    cluster: localcluster
    user: client
  name: localclustercontext
current-context: localclustercontext
EOF
    fi

    set -x

    KUBELET_RUNTIME_CONFIG_SCRIPT_FILE=/opt/azure/containers/kubelet.sh
    tee "${KUBELET_RUNTIME_CONFIG_SCRIPT_FILE}" > /dev/null <<EOF
#!/bin/bash
# Disallow container from reaching out to the special IP address 168.63.129.16
# for TCP protocol (which http uses)
#
# 168.63.129.16 contains protected settings that have priviledged info.
# HostGAPlugin (Host-GuestAgent-Plugin) is a web server process that runs on the physical host that serves the operational and diagnostic needs of the in-VM Guest Agent.
# IT listens on both port 80 and 32526 hence access is only needed for agent but not the containers.
#
# The host can still reach 168.63.129.16 because it goes through the OUTPUT chain, not FORWARD.
#
# Note: we should not block all traffic to 168.63.129.16. For example UDP traffic is still needed
# for DNS.
iptables -I FORWARD -d 168.63.129.16 -p tcp --dport 80 -j DROP
iptables -I FORWARD -d 168.63.129.16 -p tcp --dport 32526 -j DROP
EOF

    # As iptables rule will be cleaned every time the node is restarted, we need to ensure the rule is applied every time kubelet is started.
    primaryNicIP=$(logs_to_events "AKS.CSE.ensureKubelet.getPrimaryNicIP" getPrimaryNicIP)
    ENSURE_IMDS_RESTRICTION_DROP_IN="/etc/systemd/system/kubelet.service.d/10-ensure-imds-restriction.conf"
    mkdir -p "$(dirname "${ENSURE_IMDS_RESTRICTION_DROP_IN}")"
    touch "${ENSURE_IMDS_RESTRICTION_DROP_IN}"
    chmod 0600 "${ENSURE_IMDS_RESTRICTION_DROP_IN}"
    tee "${ENSURE_IMDS_RESTRICTION_DROP_IN}" > /dev/null <<EOF
[Service]
Environment="PRIMARY_NIC_IP=${primaryNicIP}"
Environment="ENABLE_IMDS_RESTRICTION=${ENABLE_IMDS_RESTRICTION}"
Environment="INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE=${INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE}"
EOF

    # check if kubelet flags contain image-credential-provider-config and image-credential-provider-bin-dir
    # shellcheck disable=SC3010
    if [[ $KUBELET_FLAGS == *"image-credential-provider-config"* && $KUBELET_FLAGS == *"image-credential-provider-bin-dir"* ]]; then
        echo "Configure credential provider for both image-credential-provider-config and image-credential-provider-bin-dir flags are specified in KUBELET_FLAGS"
        logs_to_events "AKS.CSE.ensureKubelet.configCredentialProvider" configCredentialProvider
        # Install credential provider from URL:
        # 1. If k8s version < 1.34.0, skip_bypass_k8s_version_check != true, and not Flatcar (which falls back to URL later).
        # 2. For Azure Linux v2 due to lack of PMC packages (if not network isolated).
        if { ! isFlatcar && ! isACL && [ "${SHOULD_ENFORCE_KUBE_PMC_INSTALL}" != true ] && ! semverCompare "${KUBERNETES_VERSION:-0.0.0}" 1.34.0; } ||
           { isMarinerOrAzureLinux && [ "${OS_VERSION}" = 2.0 ] && [ -z "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; }
        then
            logs_to_events "AKS.CSE.ensureKubelet.installCredentialProviderFromUrl" installCredentialProviderFromUrl
        elif [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
            # For network isolated clusters, try distro packages first and fallback to binary installation
            logs_to_events "AKS.CSE.ensureKubelet.installCredentialProviderFromBootstrapProfileRegistry" installCredentialProviderPackageFromBootstrapProfileRegistry ${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER} ${KUBERNETES_VERSION}
        elif [ "$(type -t installCredentialProviderFromPkg)" = function ]; then
            logs_to_events "AKS.CSE.ensureKubelet.installCredentialProviderFromPkg" "installCredentialProviderFromPkg ${KUBERNETES_VERSION}"
        else
            echo "installCredentialProviderFromPkg is not defined for this OS"
            exit $ERR_CREDENTIAL_PROVIDER_DOWNLOAD_TIMEOUT
        fi
    fi

    # kubelet cannot pull pause image from anonymous disabled registry during runtime
    if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
        logs_to_events "AKS.CSE.ensureKubelet.ensurePodInfraContainerImage" ensurePodInfraContainerImage
    fi

    local tls_bootstrapping_start_time_filepath="/opt/azure/containers/tls-bootstrap-start-time"
    date +"%F %T.%3N" > "${tls_bootstrapping_start_time_filepath}"

    # Node Memory Hardening (F2/F5): if the RP rendered --kube-reserved-cgroup or
    # --system-reserved-cgroup, ensure the corresponding systemd slices exist before
    # kubelet starts so its NodeAllocatable enforcement loop can find them. The
    # helper is a no-op when neither value is present (back-compat with non-hardened pools).
    resolveKubeletReservedCgroups
    if [ -n "${KUBE_RESERVED_CGROUP}" ] || [ -n "${SYSTEM_RESERVED_CGROUP}" ]; then
        if ! logs_to_events "AKS.CSE.ensureKubelet.ensureKubeletCgroupHierarchy" ensureKubeletCgroupHierarchy; then
            exit $ERR_KUBELET_START_FAIL
        fi
    fi

    # start kubelet.service without waiting for the main process to start, though check whether it has entered a failed state after enablement
    if ! systemctlEnableAndStartNoBlock kubelet 240; then
        # append kubelet status to CSE output to ensure we can see it
        rm -f "${tls_bootstrapping_start_time_filepath}"
        journalctl -u kubelet.service --no-pager || true
        exit $ERR_KUBELET_START_FAIL
    fi

    # start measure-tls-bootstrapping-latency.service without waiting for the main process to start, while ignoring any failures
    if ! systemctlEnableAndStartNoBlock measure-tls-bootstrapping-latency 30; then
        rm -f "${tls_bootstrapping_start_time_filepath}"
        echo "failed to start measure-tls-bootstrapping-latency.service"
    fi
}

ensureSnapshotUpdate() {
    systemctlEnableAndStartNoBlock snapshot-update.timer 30 || exit $ERR_SNAPSHOT_UPDATE_START_FAIL
}

ensureMigPartition(){
    mkdir -p /etc/systemd/system/mig-partition.service.d/
    touch /etc/systemd/system/mig-partition.service.d/10-mig-profile.conf
    tee /etc/systemd/system/mig-partition.service.d/10-mig-profile.conf > /dev/null <<EOF
[Service]
Environment="GPU_INSTANCE_PROFILE=${GPU_INSTANCE_PROFILE}"
EOF
    # this is expected to fail and work only on next reboot
    # it MAY succeed, only due to unreliability of systemd
    # service type=Simple, which does not exit non-zero
    # on failure if ExecStart failed to invoke.
    systemctlEnableAndStart mig-partition 300
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

ensureAzureNetworkConfig() {
    # Reload udev rules to pick up the new azure-network rules
    udevadm control --reload-rules

    # Trigger udev to detect and populate network interfaces
    echo "Triggering udev for network devices..."
    udevadm trigger --subsystem-match=net --action=add

    # Give udev time to process and trigger the systemd service
    udevadm settle --timeout=10
}

configureSecondaryNICs() {
    # Read NIC list from cached IMDS metadata.
    # IMDS reports all NICs attached to the VM, including secondary ones.
    # On a vanilla Azure VM cloud-init would configure these automatically,
    # but AKS VHDs disable cloud-init network management (apply_network_config: false
    # on Ubuntu, hardcoded Name=eth0 in networkd on AzureLinux). This function
    # fills the gap for Standard-type secondary NICs that need OS-level DHCP.
    local nic_count
    nic_count=$(jq -r '.network.interface | length' "$IMDS_INSTANCE_METADATA_CACHE_FILE") || {
        echo "Failed to parse NIC count from IMDS cache file: $IMDS_INSTANCE_METADATA_CACHE_FILE" >&2
        return $ERR_SECONDARY_NIC_CONFIG_FAIL
    }
    if ! [ "$nic_count" -eq "$nic_count" ] 2>/dev/null; then
        echo "Invalid NIC count '$nic_count' from IMDS cache file: $IMDS_INSTANCE_METADATA_CACHE_FILE" >&2
        return $ERR_SECONDARY_NIC_CONFIG_FAIL
    fi

    if [ "$nic_count" -le 1 ]; then
        echo "No secondary NICs detected, skipping"
        return 0
    fi

    echo "Detected $nic_count NICs, configuring secondary interfaces..."

    local is_netplan=false
    # Ubuntu netplan primary NIC default metric is ~100,
    # so secondary NICs use 200, 300, etc. (base 100).
    # AzureLinux/Mariner networkd primary NIC DHCP default metric is 1024,
    # so secondary NICs must use >1024 to avoid asymmetric routing.
    local metric_base=2000
    if isUbuntu; then
        is_netplan=true
        metric_base=100
    fi

    # Collect resolved interface names for networkctl up calls after reload.
    local secondary_ifaces=""

    for i in $(seq 1 $((nic_count - 1))); do
        local mac
        mac=$(jq -r ".network.interface[$i].macAddress" "$IMDS_INSTANCE_METADATA_CACHE_FILE")
        # IMDS returns MAC without colons (e.g. "7CED8D8A4DCE"), convert to colon-separated lowercase
        mac=$(echo "$mac" | sed 's/\(..\)/\1:/g; s/:$//' | tr '[:upper:]' '[:lower:]')
        local metric=$(( metric_base + i * 100 ))

        # Resolve the actual kernel interface name by matching the MAC address against
        # /sys/class/net/*/address. This is necessary because SR-IOV virtual functions
        # can claim eth1 before the secondary NIC is attached, so we cannot assume
        # secondary NIC $i is eth${i}.
        local iface_name=""
        for sys_path in /sys/class/net/*/address; do
            local sys_dir
            sys_dir=$(dirname "$sys_path")
            # Skip SR-IOV VFs (enslaved interfaces) — they share the MAC of their master
            # but don't hold an IP address themselves.
            [ -e "$sys_dir/master" ] && continue
            if [ "$(cat "$sys_path" 2>/dev/null | tr '[:upper:]' '[:lower:]')" = "$mac" ]; then
                iface_name=$(basename "$sys_dir")
                break
            fi
        done
        if [ -z "$iface_name" ]; then
            echo "Warning: could not find interface for MAC ${mac}, using eth${i} as fallback for logging"
            iface_name="eth${i}"
        fi

        # Check if this NIC has IPv6 addresses configured in IMDS
        local ipv6_count
        ipv6_count=$(jq -r ".network.interface[$i].ipv6.ipAddress | length" "$IMDS_INSTANCE_METADATA_CACHE_FILE")
        local has_ipv6=false
        if [ "$ipv6_count" -gt 0 ]; then
            has_ipv6=true
        fi

        if [ "$is_netplan" = true ]; then
            # Ubuntu: generate netplan config for the secondary NIC.
            # Match by MAC address so we configure the right device regardless of
            # kernel naming (SR-IOV VFs can shift ethN indices).
            local netplan_file="/etc/netplan/60-secondary-nic-${i}.yaml"
            {
                cat <<NETPLAN_EOF
network:
  ethernets:
    secondary-nic-${i}:
      match:
        macaddress: "${mac}"
      dhcp4: true
      dhcp4-overrides:
        route-metric: ${metric}
        use-dns: false
NETPLAN_EOF
                if [ "$has_ipv6" = true ]; then
                    cat <<NETPLAN_V6_EOF
      dhcp6: true
      dhcp6-overrides:
        route-metric: ${metric}
        use-dns: false
NETPLAN_V6_EOF
                fi
            } > "$netplan_file"
            chmod 600 "$netplan_file"
        else
            # AzureLinux/Mariner: generate networkd .network unit.
            # Prefix 10- so it takes precedence over the VHD's 99-dhcp-en.network.
            # Match by MAC address so we configure the right device regardless of
            # kernel naming (SR-IOV VFs can shift ethN indices).
            local networkd_file="/etc/systemd/network/10-secondary-nic-${i}.network"
            if [ "$has_ipv6" = true ]; then
                cat > "$networkd_file" <<NETWORKD_EOF
[Match]
MACAddress=${mac}

[Network]
DHCP=yes
IPv6AcceptRA=yes

[DHCPv4]
RouteMetric=${metric}
UseDNS=false
UseDomains=false
SendRelease=false

[DHCPv6]
RouteMetric=${metric}
UseDNS=false
UseDomains=false
NETWORKD_EOF
            else
                cat > "$networkd_file" <<NETWORKD_EOF
[Match]
MACAddress=${mac}

[Network]
DHCP=ipv4

[DHCPv4]
RouteMetric=${metric}
UseDNS=false
UseDomains=false
SendRelease=false
NETWORKD_EOF
            fi
        fi

        echo "Configured secondary NIC ${iface_name} (mac=${mac}, metric=${metric})"
        # Only track interfaces that actually exist and are not SR-IOV VFs for
        # the networkctl up loop. The .network files match by MAC and will
        # auto-activate once the real interface appears, so a missing or VF
        # interface should not block or fail the reload path.
        if [ -d "/sys/class/net/${iface_name}" ] && [ ! -e "/sys/class/net/${iface_name}/master" ]; then
            secondary_ifaces="${secondary_ifaces} ${iface_name}"
        fi
    done

    # Apply all configs in a single operation to avoid repeated network restarts.
    if [ "$is_netplan" = true ]; then
        if ! retrycmd_if_failure 5 3 10 netplan apply; then
            echo "Failed to apply netplan config for secondary NICs" >&2
            return $ERR_SECONDARY_NIC_CONFIG_FAIL
        fi
    else
        if isACL; then
            # On ACL (Flatcar-based), networkctl's control socket is often broken
            # ("Transport endpoint is not connected") because systemd-networkd's
            # varlink socket is torn down during the initrd→real-root pivot and may
            # never recover. Bypass the broken socket entirely by restarting the
            # systemd-networkd service, which talks to PID-1's (always-working)
            # D-Bus socket instead. The restart re-reads all .network files and
            # brings up matching interfaces automatically — no separate
            # networkctl up/reload calls needed.
            if ! retrycmd_if_failure 5 5 30 systemctl restart systemd-networkd; then
                echo "Failed to restart systemd-networkd for secondary NICs" >&2
                return $ERR_SECONDARY_NIC_CONFIG_FAIL
            fi
        else
            local reload_retries=5 reload_sleep=3
            if ! retrycmd_if_failure $reload_retries $reload_sleep 10 networkctl reload; then
                echo "Failed to reload networkd for secondary NICs" >&2
                return $ERR_SECONDARY_NIC_CONFIG_FAIL
            fi
            for iface in $secondary_ifaces; do
                if ! retrycmd_if_failure $reload_retries $reload_sleep 10 networkctl up "$iface"; then
                    echo "Failed to bring up ${iface}" >&2
                    return $ERR_SECONDARY_NIC_CONFIG_FAIL
                fi
            done
        fi
    fi
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

configClusterAutoscalerAddon() {
    CLUSTER_AUTOSCALER_ADDON_FILE=/etc/kubernetes/addons/cluster-autoscaler-deployment.yaml
    sed -i "s|<clientID>|$(echo $SERVICE_PRINCIPAL_CLIENT_ID | base64)|g" $CLUSTER_AUTOSCALER_ADDON_FILE
    sed -i "s|<clientSec>|$(echo $SERVICE_PRINCIPAL_CLIENT_SECRET | base64)|g" $CLUSTER_AUTOSCALER_ADDON_FILE
    sed -i "s|<subID>|$(echo $SUBSCRIPTION_ID | base64)|g" $CLUSTER_AUTOSCALER_ADDON_FILE
    sed -i "s|<tenantID>|$(echo $TENANT_ID | base64)|g" $CLUSTER_AUTOSCALER_ADDON_FILE
    sed -i "s|<rg>|$(echo $RESOURCE_GROUP | base64)|g" $CLUSTER_AUTOSCALER_ADDON_FILE
}

configACIConnectorAddon() {
    ACI_CONNECTOR_CREDENTIALS=$(printf "{\"clientId\": \"%s\", \"clientSecret\": \"%s\", \"tenantId\": \"%s\", \"subscriptionId\": \"%s\", \"activeDirectoryEndpointUrl\": \"https://login.microsoftonline.com\",\"resourceManagerEndpointUrl\": \"https://management.azure.com/\", \"activeDirectoryGraphResourceId\": \"https://graph.windows.net/\", \"sqlManagementEndpointUrl\": \"https://management.core.windows.net:8443/\", \"galleryEndpointUrl\": \"https://gallery.azure.com/\", \"managementEndpointUrl\": \"https://management.core.windows.net/\"}" "$SERVICE_PRINCIPAL_CLIENT_ID" "$SERVICE_PRINCIPAL_CLIENT_SECRET" "$TENANT_ID" "$SUBSCRIPTION_ID" | base64 -w 0)

    openssl req -newkey rsa:4096 -new -nodes -x509 -days 3650 -keyout /etc/kubernetes/certs/aci-connector-key.pem -out /etc/kubernetes/certs/aci-connector-cert.pem -subj "/C=US/ST=CA/L=virtualkubelet/O=virtualkubelet/OU=virtualkubelet/CN=virtualkubelet"
    ACI_CONNECTOR_KEY=$(base64 /etc/kubernetes/certs/aci-connector-key.pem -w0)
    ACI_CONNECTOR_CERT=$(base64 /etc/kubernetes/certs/aci-connector-cert.pem -w0)

    ACI_CONNECTOR_ADDON_FILE=/etc/kubernetes/addons/aci-connector-deployment.yaml
    sed -i "s|<creds>|$ACI_CONNECTOR_CREDENTIALS|g" $ACI_CONNECTOR_ADDON_FILE
    sed -i "s|<rgName>|$RESOURCE_GROUP|g" $ACI_CONNECTOR_ADDON_FILE
    sed -i "s|<cert>|$ACI_CONNECTOR_CERT|g" $ACI_CONNECTOR_ADDON_FILE
    sed -i "s|<key>|$ACI_CONNECTOR_KEY|g" $ACI_CONNECTOR_ADDON_FILE
}

configAzurePolicyAddon() {
    AZURE_POLICY_ADDON_FILE=/etc/kubernetes/addons/azure-policy-deployment.yaml
    sed -i "s|<resourceId>|/subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RESOURCE_GROUP|g" $AZURE_POLICY_ADDON_FILE
}

# Wrapped as functions so logs_to_events can time each step; the install's
# bash -c command can't be passed to logs_to_events inline (it word-splits args).
pullGPUDriverImage() {
    ctr -n k8s.io image pull $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
}

installGPUDriverImage() {
    local gpuInstallAction="${1:-install}"
    retrycmd_if_failure 5 10 600 bash -c "$CTR_GPU_INSTALL_CMD $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG gpuinstall /entrypoint.sh ${gpuInstallAction}"
}

configGPUDrivers() {
    if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
        waitForContainerdReady || exit $ERR_GPU_DRIVERS_START_FAIL
        mkdir -p /opt/{actions,gpu}
        # When the kernel module was pre-built into the VHD (build-only at image-bake time),
        # a marker is present. Ask aks-gpu to skip the ~100s DKMS recompile and run only the
        # device-dependent steps -- but ONLY when the marker's driver_kind matches THIS node's
        # driver (NVIDIA_GPU_DRIVER_TYPE). A CUDA-prebaked marker on a GRID node (or vice-versa)
        # must request a full "install": the other driver image may not even support
        # install-skip-build and would fail to stage its userspace files (e.g. /opt/gpu/config.sh).
        # aks-gpu still independently re-validates the marker (kernel + driver_version +
        # driver_kind) and falls back to a full build on any remaining mismatch (e.g. kernel drift).
        GPU_INSTALL_ACTION="install"
        GPU_DKMS_MARKER="${GPU_DKMS_MARKER_FILE:-/opt/azure/aks-gpu/dkms-marker}"
        if [ -f "$GPU_DKMS_MARKER" ] && \
           [ "$(sed -n 's/^driver_kind=//p' "$GPU_DKMS_MARKER" | head -n1)" = "$NVIDIA_GPU_DRIVER_TYPE" ]; then
            GPU_INSTALL_ACTION="install-skip-build"
        fi
        # The driver image is normally pre-pulled into the VHD; only hit the registry when it is
        # actually missing so provisioning doesn't pay a redundant manifest/layer round trip.
        # Use containerd's native exact-name filter rather than text-matching `images ls` output.
        if [ -z "$(ctr -n k8s.io images ls -q "name==${NVIDIA_DRIVER_IMAGE}:${NVIDIA_DRIVER_IMAGE_TAG}")" ]; then
            logs_to_events "AKS.CSE.configGPUDrivers.pullGPUDriverImage" pullGPUDriverImage
        fi
        logs_to_events "AKS.CSE.configGPUDrivers.installGPUDriverImage" installGPUDriverImage "$GPU_INSTALL_ACTION"
        ret=$?
        if [ "$ret" -ne 0 ]; then
            echo "Failed to install GPU driver, exiting..."
            exit $ERR_GPU_DRIVERS_START_FAIL
        fi
        # Drop the driver image reference so containerd can reclaim its space, but skip --sync so
        # garbage collection runs asynchronously instead of blocking node provisioning.
        ctr -n k8s.io images rm $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
    elif isMarinerOrAzureLinux "$OS" && ! isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
        logs_to_events "AKS.CSE.configGPUDrivers.downloadGPUDrivers" downloadGPUDrivers
        logs_to_events "AKS.CSE.configGPUDrivers.installNvidiaContainerToolkit" installNvidiaContainerToolkit
        enableNvidiaPersistenceMode
    elif isACL "$OS" "$OS_VARIANT"; then
        logs_to_events "AKS.CSE.configGPUDrivers.installNvidiaContainerToolkitSysext" installNvidiaContainerToolkitSysext
        logs_to_events "AKS.CSE.configGPUDrivers.installGPUDriverSysext" installGPUDriverSysext
        enableNvidiaPersistenceMode
    else
        echo "os $OS $OS_VARIANT not supported at this time. skipping configGPUDrivers"
        exit 1
    fi

    logs_to_events "AKS.CSE.configGPUDrivers.waitForNvidiaModprobe" "retrycmd_if_failure 120 5 25 nvidia-modprobe -u -c0" || exit $ERR_GPU_DRIVERS_START_FAIL
    logs_to_events "AKS.CSE.configGPUDrivers.waitForNvidiaSmi" "retrycmd_if_failure 120 5 30 nvidia-smi" || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 25 ldconfig || exit $ERR_GPU_DRIVERS_START_FAIL

    # Fix the NVIDIA /dev/char link issue (Mariner/AzureLinux only)
    if isMarinerOrAzureLinux "$OS"; then
        createNvidiaSymlinkToAllDeviceNodes
    fi

    # GRID vGPU licensing: start nvidia-gridd service to ensure license configuration
    if (isMarinerOrAzureLinux "$OS" || isACL "$OS" "$OS_VARIANT") && [ "$NVIDIA_GPU_DRIVER_TYPE" = "grid" ]; then
        systemctlEnableAndStart nvidia-gridd 300 || exit $ERR_SYSTEMCTL_START_FAIL
    fi

    systemctlEnableAndStart containerd 30 || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT

    # NPD is installed as a VM extension, which might happen before/after/during CSE, so this
    # line may fail. This will need to be updated when NPD is shipped in the VHD - we can control
    # the startup ordering in that case.
    systemctl restart node-problem-detector || true
}

validateGPUDrivers() {
    if [ "$(isARM64)" -eq 1 ]; then
        return
    fi

    retrycmd_if_failure 24 5 25 nvidia-modprobe -u -c0 && echo "gpu driver loaded" || configGPUDrivers || exit $ERR_GPU_DRIVERS_START_FAIL

    if which nvidia-smi; then
        SMI_RESULT=$(retrycmd_if_failure 24 5 30 nvidia-smi)
    else
        SMI_RESULT=$(retrycmd_if_failure 24 5 30 $GPU_DEST/bin/nvidia-smi)
    fi
    SMI_STATUS=$?
    if [ "$SMI_STATUS" -ne 0 ]; then
        # shellcheck disable=SC3010
        if [[ $SMI_RESULT == *"infoROM is corrupted"* ]]; then
            exit $ERR_GPU_INFO_ROM_CORRUPTED
        else
            exit $ERR_GPU_DRIVERS_START_FAIL
        fi
    else
        echo "gpu driver working fine"
    fi
}

ensureGPUDrivers() {
    if [ "$(isARM64)" -eq 1 ]; then
        return
    fi

    if [ "${CONFIG_GPU_DRIVER_IF_NEEDED}" = true ]; then
        logs_to_events "AKS.CSE.ensureGPUDrivers.configGPUDrivers" configGPUDrivers
    else
        logs_to_events "AKS.CSE.ensureGPUDrivers.validateGPUDrivers" validateGPUDrivers
    fi
    if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
        logs_to_events "AKS.CSE.ensureGPUDrivers.nvidia-modprobe" "systemctlEnableAndStart nvidia-modprobe 30" || exit $ERR_GPU_DRIVERS_START_FAIL
    fi
}

# Install AMD AMA core SW package for MA35D (Supernova GPU SKU)
# Note that this depends on access to download.microsoft.com, so network-isolated clusters are not supported
dnf_install_amd_ama_core() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        # RPM_FRONTEND env variable needed to disable license agreement prompt
        RPM_FRONTEND=noninteractive dnf install -y https://download.microsoft.com/download/16b04fa7-883e-4a94-88c2-801881a47b28/amd-ama-core_1.3.0-2503242033-amd64.rpm && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
            dnf_makecache
        fi
    done
    echo Executed dnf install AMD AMA core package $i times;
}

# Install AMD AMA drivers/SW for MA35D (Supernova GPU SKU)
# Note that this depends on access to download.microsoft.com, so network-isolated clusters are not supported
setupAmdAma() {
    if [ "$(isARM64)" -eq 1 ]; then
        return
    fi

    if isMarinerOrAzureLinux "$OS"; then
        # Install driver - currently version 1.3.0 is supported
        if ! dnf_install 30 1 600 azurelinux-repos-amd; then
          echo "Unable to install Azure Linux AMD package repo, exiting..."
          exit $ERR_AMDAMA_INSTALL_FAIL
        fi
        KERNEL_VERSION=$(uname -r | sed 's/-/./g')
        AMD_AMA_DRIVER_PACKAGE=$(dnf repoquery -y --available "amd-ama-driver-1.3.0*" | grep -E "amd-ama-driver-[0-9]+.*_$KERNEL_VERSION" | sort -V | tail -n 1)
        if [ -z "$AMD_AMA_DRIVER_PACKAGE" ]; then
            echo "Unable to find AMD AMA driver package for current kernel version, exiting..."
            exit $ERR_AMDAMA_DRIVER_NOT_FOUND
        fi
        if ! dnf_install 30 1 600 $AMD_AMA_DRIVER_PACKAGE; then
          echo "Unable to install AMD AMA driver package, exiting..."
          exit $ERR_AMDAMA_INSTALL_FAIL
        fi

        # Install core package
        if ! dnf_install 30 1 600 azurelinux-repos-extended libzip; then
          echo "Unable to install Azure Linux packages required for AMD AMA core package, exiting..."
          exit $ERR_AMDAMA_INSTALL_FAIL
        fi
        if ! dnf_install_amd_ama_core 30 1 600; then
          echo "Unable to install AMD AMA core package, exiting..."
          exit $ERR_AMDAMA_INSTALL_FAIL
        fi

        # Install AKS device plugin
        if ! dnf_install 30 1 600 amdama-device-plugin.x86_64; then
          echo "Unable to install AMD AMA AKS device plugin package, exiting..."
          exit $ERR_AMDAMA_INSTALL_FAIL
        fi
        # Configure huge pages
        sh -c "echo 'vm.nr_hugepages=4096' > /etc/sysctl.d/99-ama_transcoder.conf"
        sh -c "echo 4096 > /proc/sys/vm/nr_hugepages"
        if [ "$(systemctl is-active kubelet)" = "active" ]; then
            systemctl restart kubelet
        fi
    fi
}

disableSSH() {
    # On ubuntu, the ssh service is named "ssh.service"
    systemctlDisableAndStop ssh || exit $ERR_DISABLE_SSH
    # On AzureLinux, the ssh service is named "sshd.service"
    systemctlDisableAndStop sshd || exit $ERR_DISABLE_SSH
}

configureSSHPubkeyAuth() {
  local disable_pubkey_auth="$1"
  local ssh_use_pubkey_auth

  # Determine the desired pubkey auth setting
  if [ "${disable_pubkey_auth}" = "true" ]; then
    ssh_use_pubkey_auth="no"
  else
    ssh_use_pubkey_auth="yes"
  fi
  local SSHD_CONFIG="/etc/ssh/sshd_config"
  local TMP
  TMP="$(mktemp)"

  # AAD SSH extension will append following section to the end of sshd_config,
  # so we need to check the "Match" section, and only update "PubkeyAuthentication" outside of it.
  # Match User *@*,????????-????-????-????-???????????? # Added by aadsshlogin installer
  # AuthenticationMethods publickey
  # PubkeyAuthentication yes
  # AuthorizedKeysCommand /usr/sbin/aad_certhandler %u %k
  # AuthorizedKeysCommandUser root
  awk -v desired="$ssh_use_pubkey_auth" '
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

  # Validate the candidate config
  sshd -t -f "$TMP" || { rm -f "$TMP"; exit $ERR_CONFIG_PUBKEY_AUTH_SSH; }

  # Replace the original with the candidate (permissions 644, owned by root)
  install -m 644 -o root -g root "$TMP" "$SSHD_CONFIG"
  rm -f "$TMP"

  # Reload sshd
  systemctl reload sshd || systemctl restart sshd || exit $ERR_CONFIG_PUBKEY_AUTH_SSH
}

# Internal function that writes credential provider config to a specified path
# This function is extracted to allow unit testing without root permissions
# Usage: writeCredentialProviderConfig <config_file_path>
writeCredentialProviderConfig() {
    if [ -z "$1" ]; then
        echo "Error: writeCredentialProviderConfig requires config file path as argument"
        return 1
    fi
    local config_file_path="$1"
    mkdir -p "$(dirname "${config_file_path}")"
    touch "${config_file_path}"

    # Prepare identity binding configuration if enabled (including leading newlines)
    local ib_token_attributes=""
    local ib_args=""
    local ib_args_list=()
    if [ "${SERVICE_ACCOUNT_IMAGE_PULL_ENABLED}" = "true" ]; then
        ib_token_attributes="
    tokenAttributes:
      serviceAccountTokenAudience: api://AKSIdentityBinding
      requireServiceAccount: false
      cacheType: ServiceAccount
      optionalServiceAccountAnnotationKeys:
        - kubernetes.azure.com/acr-client-id"
        # Build identity binding args list using an array to avoid word splitting
        ib_args_list=( "--ib-sni-name=${IDENTITY_BINDINGS_LOCAL_AUTHORITY_SNI}" )
        [ -n "${SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID}" ] && ib_args_list+=( "--ib-default-client-id=${SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_CLIENT_ID}" )
        [ -n "${SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_TENANT_ID}" ] && ib_args_list+=( "--ib-default-tenant-id=${SERVICE_ACCOUNT_IMAGE_PULL_DEFAULT_TENANT_ID}" )
        ib_args_list+=( "--ib-apiserver-ip=${API_SERVER_NAME}" )
        # Format args as YAML list items with proper indentation
        for arg in "${ib_args_list[@]}"; do
            ib_args="${ib_args}
      - ${arg}"
        done
    fi

    if [ -n "$AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX" ]; then
        echo "configure credential provider for custom cloud"
        tee "${config_file_path}" > /dev/null <<EOF
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*$AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1${ib_token_attributes}
    args:
      - /etc/kubernetes/azure.json${ib_args}
EOF
    elif [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
        echo "configure credential provider for network isolated cluster"
        MCR_REPOSITORY_BASE="${MCR_REPOSITORY_BASE:=mcr.microsoft.com}"
        MCR_REPOSITORY_BASE="${MCR_REPOSITORY_BASE%/}"
        tee "${config_file_path}" > /dev/null <<EOF
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "${MCR_REPOSITORY_BASE}"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1${ib_token_attributes}
    args:
      - /etc/kubernetes/azure.json
      - --registry-mirror=${MCR_REPOSITORY_BASE}:$BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER${ib_args}
EOF
    else
        echo "configure credential provider with default settings"
        tee "${config_file_path}" > /dev/null <<EOF
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1${ib_token_attributes}
    args:
      - /etc/kubernetes/azure.json${ib_args}
EOF
    fi
}

configCredentialProvider() {
    writeCredentialProviderConfig "/var/lib/kubelet/credential-provider-config.yaml"
}

setKubeletNodeIPFlag() {
    local imdsOutput
    export -f get_imds_network_metadata
    imdsOutput=$(get_imds_network_metadata)
    nodeIPAddrs=()
    ipv4Addr=$(echo $imdsOutput | jq -r '.[0].ipv4.ipAddress[0].privateIpAddress // ""')
    [ -n "$ipv4Addr" ] && nodeIPAddrs+=("$ipv4Addr")
    ipv6Addr=$(echo $imdsOutput | jq -r '.[0].ipv6.ipAddress[0].privateIpAddress // ""')
    [ -n "$ipv6Addr" ] && nodeIPAddrs+=("$ipv6Addr")
    nodeIPArg=$(IFS=, ; echo "${nodeIPAddrs[*]}") # join, comma-separated
    if [ -n "$nodeIPArg" ]; then
        echo "Adding --node-ip=$nodeIPArg to kubelet flags"
        KUBELET_FLAGS="$KUBELET_FLAGS --node-ip=$nodeIPArg"
    fi
}

# localdns corefile used by localdns systemd unit.
LOCALDNS_CORE_FILE="/opt/azure/containers/localdns/localdns.corefile"
# localdns slice file used by localdns systemd unit.
LOCALDNS_SLICE_FILE="/etc/systemd/system/localdns.slice"
# This function is called from cse_main.sh.
# It creates the localdns corefile and slicefile, then enables and starts localdns.
# Both corefile variants are read from globals set in cse_cmd.sh:
#   LOCALDNS_COREFILE_BASE         — standard corefile without hosts plugin
#   LOCALDNS_COREFILE_WITH_HOSTS — corefile with hosts plugin
# The base variant is written as the initial active corefile.
# Both variants are saved to /etc/localdns/environment so localdns.sh
# can dynamically switch between them on restart.
generateLocalDNSFiles() {
    mkdir -p "$(dirname "${LOCALDNS_CORE_FILE}")"
    touch "${LOCALDNS_CORE_FILE}"
    chmod 0644 "${LOCALDNS_CORE_FILE}"

    # Determine the base corefile to use as the initial active corefile.
    # LOCALDNS_COREFILE_BASE is set by new CSE; fall back to LOCALDNS_GENERATED_COREFILE
    # for backward compatibility when this VHD runs with an older CSE that only sets
    # LOCALDNS_GENERATED_COREFILE.
    local corefile_base="${LOCALDNS_COREFILE_BASE:-${LOCALDNS_GENERATED_COREFILE:-}}"
    if [ -z "${corefile_base}" ]; then
        echo "Error: neither LOCALDNS_COREFILE_BASE nor LOCALDNS_GENERATED_COREFILE is set"
        exit $ERR_LOCALDNS_FAIL
    fi

    # Start with the base corefile as the initial active corefile.
    # localdns.sh will select the appropriate variant (BASE or WITH_HOSTS)
    # based on the SHOULD_ENABLE_HOSTS_PLUGIN feature flag on service start.
    base64 -d <<< "${corefile_base}" > "${LOCALDNS_CORE_FILE}" || exit $ERR_LOCALDNS_FAIL

    # Log whether the initial corefile includes hosts plugin.
    # This is the BASE corefile; localdns.sh may select the WITH_HOSTS variant at service start.
    if grep -q "hosts /etc/localdns/hosts" "${LOCALDNS_CORE_FILE}"; then
        echo "Initial corefile at ${LOCALDNS_CORE_FILE} INCLUDES hosts plugin"
    else
        echo "Initial corefile at ${LOCALDNS_CORE_FILE} DOES NOT include hosts plugin (localdns.sh selects variant at runtime)"
    fi

    # Create environment file for corefile regeneration.
    # This file will be referenced by localdns.service using EnvironmentFile directive.
    # Save BOTH corefile variants so localdns can dynamically choose on each restart.
    # All corefile values are base64-encoded; localdns.sh decodes them at runtime.
    # LOCALDNS_BASE64_ENCODED_COREFILE is the legacy key for old VHDs.
    # LOCALDNS_COREFILE_BASE is the new name ("BASE" = base variant without hosts plugin, not base64).
    # LOCALDNS_COREFILE_WITH_HOSTS is the variant WITH hosts plugin.
    LOCALDNS_ENV_FILE="/etc/localdns/environment"
    mkdir -p "$(dirname "${LOCALDNS_ENV_FILE}")"
    if [ "${SHOULD_ENABLE_HOSTS_PLUGIN:-false}" = "true" ] && [ -z "${LOCALDNS_COREFILE_WITH_HOSTS:-}" ]; then
        echo "WARNING: SHOULD_ENABLE_HOSTS_PLUGIN=true but LOCALDNS_COREFILE_WITH_HOSTS is empty. Hosts plugin will fall back to BASE corefile at runtime."
    fi
    cat > "${LOCALDNS_ENV_FILE}" <<EOF
LOCALDNS_BASE64_ENCODED_COREFILE=${corefile_base}
LOCALDNS_COREFILE_BASE=${corefile_base}
LOCALDNS_COREFILE_WITH_HOSTS=${LOCALDNS_COREFILE_WITH_HOSTS:-}
SHOULD_ENABLE_HOSTS_PLUGIN=${SHOULD_ENABLE_HOSTS_PLUGIN:-false}
LOCALDNS_CRITICAL_FQDNS=${LOCALDNS_CRITICAL_FQDNS:-}
EOF
    chmod 0644 "${LOCALDNS_ENV_FILE}"

	mkdir -p "$(dirname "${LOCALDNS_SLICE_FILE}")"
    touch "${LOCALDNS_SLICE_FILE}"
    chmod 0644 "${LOCALDNS_SLICE_FILE}"
    cat > "${LOCALDNS_SLICE_FILE}" <<EOF
[Unit]
Description=localdns Slice
DefaultDependencies=no
Before=slices.target
Requires=system.slice
After=system.slice
[Slice]
MemoryMax=${LOCALDNS_MEMORY_LIMIT}
CPUQuota=${LOCALDNS_CPU_LIMIT}
EOF
}

# enableLocalDNS creates localdns files and starts the service.
# Both corefile variants (with/without hosts plugin) are read from globals
# set in cse_cmd.sh. No parameters needed.
enableLocalDNS() {
    # Guard: Check if this VHD has localdns assets installed.
    # Older VHDs may not have localdns.service or the execution script.
    # This ensures backward compatibility when new CSE runs on old VHDs.
    if [ ! -f /etc/systemd/system/localdns.service ]; then
        echo "Warning: localdns.service not found on this VHD, skipping localdns setup"
        return 0
    fi
    if [ ! -f /opt/azure/containers/localdns/localdns.sh ]; then
        echo "Warning: localdns.sh not found on this VHD, skipping localdns setup"
        return 0
    fi

    echo "enableLocalDNS called, generating corefile..."
    generateLocalDNSFiles
    # Log corefile variant after it's been successfully written
    echo "Generated corefile: $(grep -q 'hosts /etc/localdns/hosts' "${LOCALDNS_CORE_FILE}" 2>/dev/null && echo 'WITH hosts plugin' || echo 'WITHOUT hosts plugin')"

    # Disable hosts plugin cleanup path: if the hosts plugin was previously enabled but is now
    # disabled (e.g. rollback), clean up the timer and hosts file. The enable path is handled
    # separately — enableAKSLocalDNSHostsSetup() is called earlier in basePrep() to give the
    # timer a head start on DNS resolution before enableLocalDNS() starts CoreDNS.
    if [ "${SHOULD_ENABLE_HOSTS_PLUGIN}" != "true" ]; then
        logs_to_events "AKS.CSE.enableLocalDNS.disableAKSLocalDNSHostsSetup" disableAKSLocalDNSHostsSetup
    fi

    echo "localdns should be enabled."
    systemctlEnableAndStart localdns 30 || exit $ERR_LOCALDNS_FAIL
    echo "Enable localdns succeeded."
    # Exporter socket setup is deferred to configureLocalDNSExporterSocket() (after ensureKubelet)
    # to avoid delaying kubelet start. The kubelet node label is added separately in cse_main.sh.
}

# Configures the localdns metrics exporter socket to listen on the node IP.
# Runs after ensureKubelet (the kubelet node label is added separately before ensureKubelet).
# The VHD default binds to 0.0.0.0 which already works for vmagent scraping.
# The drop-in narrows binding to the node IP for tighter scoping when available.
configureLocalDNSExporterSocket() {
    # Guard: skip everything if the socket unit doesn't exist (old VHD without exporter files).
    # This is a backward compatibility check for VHDs built before the exporter was added.
    # Without this guard, we'd create an orphaned drop-in directory and
    # systemctlEnableAndStartNoBlock would hit its retry loop (~100 retries × 5s) for a missing unit.
    if ! systemctl cat localdns-exporter.socket &>/dev/null; then
        echo "localdns-exporter: socket unit not found on this VHD, skipping"
        return 0
    fi

    # Create drop-in to narrow socket binding from 0.0.0.0 to the node IP.
    local node_ip
    node_ip=$(get_primary_nic_ip)
    if [ -n "${node_ip}" ]; then
        echo "localdns-exporter: creating socket drop-in to bind to ${node_ip}:9353"
        mkdir -p /etc/systemd/system/localdns-exporter.socket.d
        tee /etc/systemd/system/localdns-exporter.socket.d/10-listen-address.conf > /dev/null <<EOF
[Socket]
ListenStream=
ListenStream=${node_ip}:9353
EOF
        systemctl daemon-reload
    else
        echo "localdns-exporter: get_primary_nic_ip returned empty, using VHD default (0.0.0.0:9353)"
    fi

    # Enable localdns metrics exporter socket for Prometheus scraping.
    # This is optional observability — don't block provisioning if it fails.
    # Note: the kubelet node label is added separately in cse_main.sh before ensureKubelet.
    echo "Enabling localdns-exporter.socket for metrics collection."
    if systemctlEnableAndStartNoBlock localdns-exporter.socket 30; then
        echo "Enable localdns-exporter.socket succeeded."
    else
        echo "WARNING: Failed to enable localdns-exporter.socket. Metrics will not be available but continuing provisioning."
    fi
}

# This function enables and starts the aks-localdns-hosts-setup timer.
# The timer periodically resolves critical AKS FQDN DNS records and populates /etc/localdns/hosts.
# Called from basePrep() early in the boot sequence, before enableLocalDNS().
# This allows DNS resolution to begin while the rest of basePrep installs packages and configures the node.
# The timer's systemd service reads LOCALDNS_CRITICAL_FQDNS from /etc/localdns/environment,
# so this function writes a minimal environment file before starting the timer.
# generateLocalDNSFiles() (called later by enableLocalDNS) overwrites it with the full content.
enableAKSLocalDNSHostsSetup() {
    # Best-effort setup: log errors but never fail.
    # The corefile will fall back to the no-hosts variant if hosts file is empty.
    # Allow overriding paths for testing (via environment variables)
    local hosts_file="${AKS_LOCALDNS_HOSTS_FILE:-/etc/localdns/hosts}"
    local hosts_setup_script="${AKS_LOCALDNS_HOSTS_SETUP_SCRIPT:-/opt/azure/containers/aks-localdns-hosts-setup.sh}"
    local hosts_setup_service="${AKS_LOCALDNS_HOSTS_SETUP_SERVICE:-/etc/systemd/system/aks-localdns-hosts-setup.service}"
    local hosts_setup_timer="${AKS_LOCALDNS_HOSTS_SETUP_TIMER:-/etc/systemd/system/aks-localdns-hosts-setup.timer}"

    # Guard: verify required artifacts exist on this VHD.
    # Older VHDs (or certain build modes) may not include them.
    if [ ! -f "${hosts_setup_script}" ]; then
        echo "Warning: ${hosts_setup_script} not found on this VHD, skipping aks-localdns-hosts-setup"
        return 0
    fi
    if [ ! -x "${hosts_setup_script}" ]; then
        echo "Warning: ${hosts_setup_script} is not executable, skipping aks-localdns-hosts-setup"
        return 0
    fi
    if [ ! -f "${hosts_setup_service}" ]; then
        echo "Warning: ${hosts_setup_service} not found on this VHD, skipping aks-localdns-hosts-setup"
        return 0
    fi
    if [ ! -f "${hosts_setup_timer}" ]; then
        echo "Warning: ${hosts_setup_timer} not found on this VHD, skipping aks-localdns-hosts-setup"
        return 0
    fi

    # Verify LOCALDNS_CRITICAL_FQDNS is set before proceeding; if not, skip hosts setup.
    if [ -z "${LOCALDNS_CRITICAL_FQDNS:-}" ]; then
        echo "WARNING: LOCALDNS_CRITICAL_FQDNS is not set. RP did not pass critical FQDNs."
        echo "Skipping aks-localdns-hosts-setup. Corefile will fall back to version without hosts plugin."
        return 0
    fi

    # Write a minimal environment file so the systemd service (which reads from
    # /etc/localdns/environment via EnvironmentFile=) has LOCALDNS_CRITICAL_FQDNS available.
    # generateLocalDNSFiles() overwrites this later with the full content including corefiles.
    local env_file="/etc/localdns/environment"
    mkdir -p "$(dirname "${env_file}")"
    cat > "${env_file}" <<EOF
LOCALDNS_CRITICAL_FQDNS=${LOCALDNS_CRITICAL_FQDNS}
EOF
    chmod 0644 "${env_file}"

    # Create an empty hosts file so the localdns hosts plugin can start watching it
    # immediately. The file will be populated by aks-localdns-hosts-setup timer asynchronously.
    mkdir -p "$(dirname "${hosts_file}")"
    touch "${hosts_file}"
    chmod 0644 "${hosts_file}"

    # Enable the timer for periodic refresh (every 15 minutes)
    # This will update the hosts file with fresh IPs from live DNS
    echo "Enabling aks-localdns-hosts-setup timer..."
    if systemctlEnableAndStartNoBlock aks-localdns-hosts-setup.timer 30; then
        echo "aks-localdns-hosts-setup timer enabled successfully."
    else
        echo "Warning: Failed to enable aks-localdns-hosts-setup timer"
    fi
}

# disableAKSLocalDNSHostsSetup disables the hosts plugin on a node where it was previously enabled.
# Called from enableLocalDNS() when SHOULD_ENABLE_HOSTS_PLUGIN is not true.
# This handles the production rollback case where a customer disables the hosts plugin
# on an existing agentpool and AKS-RP re-runs CSE with SHOULD_ENABLE_HOSTS_PLUGIN=false.
# All operations are idempotent — safe to call when hosts plugin was never enabled.
disableAKSLocalDNSHostsSetup() {
    local hosts_file="${AKS_LOCALDNS_HOSTS_FILE:-/etc/localdns/hosts}"
    local hosts_setup_timer="${AKS_LOCALDNS_HOSTS_SETUP_TIMER:-/etc/systemd/system/aks-localdns-hosts-setup.timer}"

    echo "disableAKSLocalDNSHostsSetup called, cleaning up hosts plugin state..."

    # Stop and disable the hosts-setup timer if it exists and is active.
    # This prevents further updates to the hosts file.
    if [ -f "${hosts_setup_timer}" ]; then
        systemctl disable --now aks-localdns-hosts-setup.timer 2>/dev/null || true
        echo "Disabled and stopped aks-localdns-hosts-setup.timer"
    else
        echo "aks-localdns-hosts-setup.timer not found on this VHD, skipping"
    fi

    # Remove the hosts file to clean up stale data.
    # select_localdns_corefile() selects based on SHOULD_ENABLE_HOSTS_PLUGIN,
    # so removing the file isn't strictly needed for corefile selection, but
    # it prevents CoreDNS from serving stale host entries if the feature is re-enabled later.
    if [ -f "${hosts_file}" ]; then
        rm -f "${hosts_file}"
        echo "Removed ${hosts_file}"
    else
        echo "${hosts_file} does not exist, skipping"
    fi

    echo "disableAKSLocalDNSHostsSetup complete"
}

configureManagedGPUExperience() {
    if [ "${GPU_NODE}" != "true" ] || [ "${skip_nvidia_driver_install}" = "true" ]; then
        return
    fi
    local managed_gpu_marker="/opt/azure/containers/managed-gpu-experience.enabled"
    if [ "${ENABLE_MANAGED_GPU_EXPERIENCE}" = "true" ]; then
        logs_to_events "AKS.CSE.installNvidiaManagedExpPkgFromCache" "installNvidiaManagedExpPkgFromCache" || exit $ERR_NVIDIA_DCGM_INSTALL
        logs_to_events "AKS.CSE.startNvidiaManagedExpServices" "startNvidiaManagedExpServices" || exit $ERR_NVIDIA_DCGM_EXPORTER_FAIL
        addKubeletNodeLabel "kubernetes.azure.com/dcgm-exporter=enabled"
        mkdir -p "$(dirname "${managed_gpu_marker}")"
        touch "${managed_gpu_marker}"
    else
        # EnableManagedGPUExperience is mutable, so services may have been
        # installed on a previous CSE run. Stop them if they exist.
        logs_to_events "AKS.CSE.stop.nvidia-device-plugin" "systemctlDisableAndStop nvidia-device-plugin"
        logs_to_events "AKS.CSE.stop.nvidia-dcgm" "systemctlDisableAndStop nvidia-dcgm"
        logs_to_events "AKS.CSE.stop.nvidia-dcgm-exporter" "systemctlDisableAndStop nvidia-dcgm-exporter"
        rm -f "${managed_gpu_marker}"
    fi
}

startNvidiaManagedExpServices() {
    # 1. Start the nvidia-device-plugin service.
    # Create systemd override directory to configure device plugin
    NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR="/etc/systemd/system/nvidia-device-plugin.service.d"
    mkdir -p "${NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR}"

    if [ "${MIG_NODE}" = "true" ]; then
        # Configure with MIG strategy for MIG nodes.
        # MIG strategy controls how nvidia-device-plugin exposes MIG instances to Kubernetes:
        #   - "single": All MIG devices exposed as generic nvidia.com/gpu resources
        #   - "mixed": MIG devices exposed with specific types like nvidia.com/mig-1g.5gb
        #
        # We only use "mixed" when explicitly specified via NVIDIA_MIG_STRATEGY.
        # Otherwise, we default to "single" which is the safer/simpler option.
        # Note: NVIDIA_MIG_STRATEGY values from RP are "None", "Single", "Mixed".
        # "None" and "Single" both result in using the "single" strategy.
        if [ "${NVIDIA_MIG_STRATEGY}" = "Mixed" ]; then
            MIG_STRATEGY_FLAG="--mig-strategy mixed"
        else
            # Default to "single" for "Single", "None", empty, or any other value
            MIG_STRATEGY_FLAG="--mig-strategy single"
        fi

        tee "${NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR}/10-device-plugin-config.conf" > /dev/null <<EOF
[Service]
ExecStart=
ExecStart=/usr/bin/nvidia-device-plugin ${MIG_STRATEGY_FLAG} --pass-device-specs
EOF
    else
        # Configure with pass-device-specs for non-MIG nodes
        tee "${NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR}/10-device-plugin-config.conf" > /dev/null <<'EOF'
[Service]
ExecStart=
ExecStart=/usr/bin/nvidia-device-plugin --pass-device-specs
EOF
    fi

    # Reload systemd to pick up the override
    systemctl daemon-reload

    logs_to_events "AKS.CSE.start.nvidia-device-plugin" "systemctlEnableAndStart nvidia-device-plugin 30" || exit $ERR_GPU_DEVICE_PLUGIN_START_FAIL

    # 2. Start the nvidia-dcgm service.
    # DCGM is monitoring/telemetry and does not gate GPU workload scheduling, so start it without
    # blocking node provisioning and treat a slow/failed start as non-fatal.
    logs_to_events "AKS.CSE.start.nvidia-dcgm" "systemctlEnableAndStartNoBlock nvidia-dcgm 30" || echo "warning: nvidia-dcgm could not be enqueued; GPU monitoring will start asynchronously"

    # 3. Start the nvidia-dcgm-exporter service.
    # Create systemd drop-in directory for nvidia-dcgm-exporter service
    DCGM_EXPORTER_OVERRIDE_DIR="/etc/systemd/system/nvidia-dcgm-exporter.service.d"
    mkdir -p "${DCGM_EXPORTER_OVERRIDE_DIR}"

    # Create drop-in file to override service configuration
    tee "${DCGM_EXPORTER_OVERRIDE_DIR}/10-aks-override.conf" > /dev/null <<EOF
[Service]
# Remove file-based logging - let systemd handle logs
StandardOutput=journal
StandardError=journal
# Change default port from 9400 to 19400 so that it does not conflict with user installed dcgm-exporter
ExecStart=
ExecStart=/usr/bin/dcgm-exporter -f /etc/dcgm-exporter/default-counters.csv --address ":19400"
EOF

    # Reload systemd to apply the override configuration
    systemctl daemon-reload

    # Start the nvidia-dcgm-exporter service.
    # The exporter is telemetry only and does not gate scheduling, so start it off the critical
    # path and treat a slow/failed start as non-fatal.
    logs_to_events "AKS.CSE.start.nvidia-dcgm-exporter" "systemctlEnableAndStartNoBlock nvidia-dcgm-exporter 30" || echo "warning: nvidia-dcgm-exporter could not be enqueued; GPU metrics will start asynchronously"
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
