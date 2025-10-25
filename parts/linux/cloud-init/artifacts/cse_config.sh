#!/bin/bash
NODE_INDEX=$(hostname | tail -c 2)
NODE_NAME=$(hostname)

configureAdminUser(){
    chage -E -1 -I -1 -m 0 -M 99999 "${ADMINUSER}"
    chage -l "${ADMINUSER}"
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
    # containerd has to be restarted after new certs are added to the trust store, otherwise they will not be used until restart happens
    systemctl restart containerd
}

configureContainerdUlimits() {
  CONTAINERD_ULIMIT_DROP_IN_FILE_PATH="/etc/systemd/system/containerd.service.d/set_ulimits.conf"
  touch "${CONTAINERD_ULIMIT_DROP_IN_FILE_PATH}"
  chmod 0600 "${CONTAINERD_ULIMIT_DROP_IN_FILE_PATH}"
  tee "${CONTAINERD_ULIMIT_DROP_IN_FILE_PATH}" > /dev/null <<EOF
$(echo "$CONTAINERD_ULIMITS" | tr ' ' '\n')
EOF

  systemctl daemon-reload
  systemctl restart containerd
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
    # needed for the iptables rules to work on bridges
    retrycmd_if_failure 120 5 25 modprobe br_netfilter || exit $ERR_MODPROBE_FAIL
    echo -n "br_netfilter" > /etc/modules-load.d/br_netfilter.conf
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
    if [ "${UBUNTU_RELEASE}" = "18.04" ] || [ "${UBUNTU_RELEASE}" = "20.04" ] || [ "${UBUNTU_RELEASE}" = "22.04" ] || [ "${UBUNTU_RELEASE}" = "24.04" ]; then
        echo "Ingoring systemd-resolved query service but using its resolv.conf file"
        echo "This is the simplest approach to workaround resolved issues without completely uninstall it"
        [ -f /run/systemd/resolve/resolv.conf ] && sudo ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
        ls -ltr /etc/resolv.conf
        cat /etc/resolv.conf
    fi
}

ensureContainerd() {
  if [ "${TELEPORT_ENABLED}" = "true" ]; then
    ensureTeleportd
  fi
  mkdir -p "/etc/systemd/system/containerd.service.d"
  tee "/etc/systemd/system/containerd.service.d/exec_start.conf" > /dev/null <<EOF
[Service]
ExecStartPost=/sbin/iptables -P FORWARD ACCEPT
EOF

  mkdir -p /etc/containerd
  if [ "${GPU_NODE}" = "true" ]; then
    # Check VM tag directly to determine if GPU drivers should be skipped
    export -f should_skip_nvidia_drivers
    should_skip=$(retrycmd_silent 10 1 10 bash -cx should_skip_nvidia_drivers)
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

  if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
    logs_to_events "AKS.CSE.ensureContainerd.configureContainerdRegistryHost" configureContainerdRegistryHost
  fi

  tee "/etc/sysctl.d/99-force-bridge-forward.conf" > /dev/null <<EOF
net.ipv4.ip_forward = 1
net.ipv4.conf.all.forwarding = 1
net.ipv6.conf.all.forwarding = 1
net.bridge.bridge-nf-call-iptables = 1
EOF
  retrycmd_if_failure 120 5 25 sysctl --system || exit $ERR_SYSCTL_RELOAD
  systemctlEnableAndStart containerd 30 || exit $ERR_SYSTEMCTL_START_FAIL
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

ensureNoDupOnPromiscuBridge() {
    systemctlEnableAndStart ensure-no-dup 30 || exit $ERR_SYSTEMCTL_START_FAIL
}

ensureTeleportd() {
    systemctlEnableAndStart teleportd 30 || exit $ERR_SYSTEMCTL_START_FAIL
}

ensureArtifactStreaming() {
  retrycmd_if_failure 120 5 25 time systemctl --quiet enable --now  acr-mirror overlaybd-tcmu overlaybd-snapshotter
  time /opt/acr/bin/acr-config --enable-containerd 'azurecr.io'
}

ensureDHCPv6() {
    systemctlEnableAndStart dhcpv6 30 || exit $ERR_SYSTEMCTL_START_FAIL
    retrycmd_if_failure 120 5 25 modprobe ip6_tables || exit $ERR_MODPROBE_FAIL
}

getPrimaryNicIP() {
    local sleepTime=1
    local maxRetries=10
    local i=0
    local ip=""

    while [ "$i" -lt "$maxRetries" ]; do
        ip=$(curl -sSL -H "Metadata: true" "http://169.254.169.254/metadata/instance/network/interface?api-version=2021-02-01")
        if [ "$?" -eq 0 ]; then
            ip=$(echo "$ip" | jq -r '.[0].ipv4.ipAddress[0].privateIpAddress')
            if [ -n "$ip" ]; then
                break
            fi
        fi
        sleep $sleepTime
        i=$((i+1))
    done
    echo "$ip"
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
    DISABLE_KUBELET_SERVING_CERTIFICATE_ROTATION=$(retrycmd_silent 10 1 10 bash -cx should_disable_kubelet_serving_certificate_rotation)
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

# drop-in path defined outside so configureAndStartSecureTLSBootstrapping can be unit tested
SECURE_TLS_BOOTSTRAPPING_DROP_IN="/etc/systemd/system/secure-tls-bootstrap.service.d/10-securetlsbootstrap.conf"
configureAndStartSecureTLSBootstrapping() {
    BOOTSTRAP_CLIENT_FLAGS="--deadline=${SECURE_TLS_BOOTSTRAPPING_DEADLINE:-"2m0s"} --aad-resource=${SECURE_TLS_BOOTSTRAPPING_AAD_RESOURCE:-$AKS_AAD_SERVER_APP_ID} --apiserver-fqdn=${API_SERVER_NAME} --cloud-provider-config=${AZURE_JSON_PATH}"
    if [ -n "${SECURE_TLS_BOOTSTRAPPING_USER_ASSIGNED_IDENTITY_ID}" ]; then
        BOOTSTRAP_CLIENT_FLAGS="${BOOTSTRAP_CLIENT_FLAGS} --user-assigned-identity-id=$SECURE_TLS_BOOTSTRAPPING_USER_ASSIGNED_IDENTITY_ID"
    fi

    mkdir -p "$(dirname "${SECURE_TLS_BOOTSTRAPPING_DROP_IN}")"
    touch "${SECURE_TLS_BOOTSTRAPPING_DROP_IN}"
    chmod 0600 "${SECURE_TLS_BOOTSTRAPPING_DROP_IN}"
    cat > "${SECURE_TLS_BOOTSTRAPPING_DROP_IN}" <<EOF
[Unit]
Before=kubelet.service
[Service]
Environment="BOOTSTRAP_FLAGS=${BOOTSTRAP_CLIENT_FLAGS}"
[Install]
# once bootstrap tokens are no longer a fallback, kubelet.service needs to be a RequiredBy=
WantedBy=kubelet.service
EOF

    # explicitly start secure TLS bootstrapping ahead of kubelet
    systemctlEnableAndStartNoBlock secure-tls-bootstrap 30 || exit $ERR_SECURE_TLS_BOOTSTRAP_START_FAILURE

    # once bootstrap tokens are no longer a fallback, we can unset TLS_BOOTSTRAP_TOKEN here if needed
}

configureKubeletAndKubectl() {
    # Install kubelet and kubectl binaries from URL for Custom Kube binary and Private Kube binary
    if [ -n "${CUSTOM_KUBE_BINARY_DOWNLOAD_URL}" ] || [ -n "${PRIVATE_KUBE_BINARY_DOWNLOAD_URL}" ]; then
        logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlFromURL" installKubeletKubectlFromURL
    # only install kube pkgs from pmc if k8s version >= 1.34.0 or skip_bypass_k8s_version_check is true
    elif [ "${SHOULD_ENFORCE_KUBE_PMC_INSTALL}" != "true" ] && ! semverCompare ${KUBERNETES_VERSION:-"0.0.0"} "1.34.0"; then
        logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlFromURL" installKubeletKubectlFromURL
    else
        if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ] ; then
            logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlFromBootstrapProfileRegistry" "installKubeletKubectlFromBootstrapProfileRegistry ${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER} ${KUBERNETES_VERSION}"
        elif isMarinerOrAzureLinux "$OS"; then
            if [ "$OS_VERSION" = "2.0" ]; then
                # we do not publish packages to PMC for azurelinux V2
                logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlFromURL" installKubeletKubectlFromURL
            else
                logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlPkgFromPMC" "installKubeletKubectlPkgFromPMC ${KUBERNETES_VERSION}"
            fi
        elif [ "${OS}" = "${UBUNTU_OS_NAME}" ]; then
            logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlPkgFromPMC" "installKubeletKubectlPkgFromPMC ${KUBERNETES_VERSION}"
        elif [ "${OS}" = "${FLATCAR_OS_NAME}" ]; then
            logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlFromURL" installKubeletKubectlFromURL
        fi
    fi
}

ensurePodInfraContainerImage() {
    POD_INFRA_CONTAINER_IMAGE_DOWNLOAD_DIR="/opt/pod-infra-container-image/downloads"
    POD_INFRA_CONTAINER_IMAGE_TAR="/opt/pod-infra-container-image/pod-infra-container-image.tar"

    pod_infra_container_image=$(get_sandbox_image)

    echo "Checking if $pod_infra_container_image already exists locally..."
    if ctr -n k8s.io images list -q | grep -q "^${pod_infra_container_image}$"; then
        echo "Image $pod_infra_container_image already exists locally, skipping pull"
        echo "Cached image details:"
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
        if { [ "${SHOULD_ENFORCE_KUBE_PMC_INSTALL}" != "true" ] && ! semverCompare ${KUBERNETES_VERSION:-"0.0.0"} "1.34.0"; }; then
            logs_to_events "AKS.CSE.ensureKubelet.installCredentialProviderFromUrl" installCredentialProviderFromUrl
        else
            if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ] ; then
                # For network isolated clusters, try distro packages first and fallback to binary installation
                logs_to_events "AKS.CSE.ensureKubelet.installCredentialProviderFromBootstrapProfileRegistry" installCredentialProviderPackageFromBootstrapProfileRegistry ${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER} ${KUBERNETES_VERSION}
            elif isMarinerOrAzureLinux "$OS"; then
                if [ "$OS_VERSION" = "2.0" ]; then # PMC package installation not supported for AzureLinux V2, only V3
                    logs_to_events "AKS.CSE.ensureKubelet.installCredentialProviderFromUrl" installCredentialProviderFromUrl
                else
                    logs_to_events "AKS.CSE.ensureKubelet.installCredentialProviderFromPMC" "installCredentialProviderFromPMC ${KUBERNETES_VERSION}"
                fi
            else
                logs_to_events "AKS.CSE.ensureKubelet.installCredentialProviderFromPMC" "installCredentialProviderFromPMC ${KUBERNETES_VERSION}"
            fi
        fi
    fi

    # kubelet cannot pull pause image from anonymous disabled registry during runtime
    if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
        logs_to_events "AKS.CSE.ensureKubelet.ensurePodInfraContainerImage" ensurePodInfraContainerImage
    fi

    # start measure-tls-bootstrapping-latency.service without waiting for the main process to start, while ignoring any failures
    if ! systemctlEnableAndStartNoBlock measure-tls-bootstrapping-latency 30; then
        echo "failed to start measure-tls-bootstrapping-latency.service"
    fi

    # start kubelet.service without waiting for the main process to start, though check whether it has entered a failed state after enablement
    if ! systemctlEnableAndStartNoBlock kubelet 240; then
        # append kubelet status to CSE output to ensure we can see it
        journalctl -u kubelet.service --no-pager || true
        exit $ERR_KUBELET_START_FAIL
    fi
}

ensureSnapshotUpdate() {
    systemctlEnableAndStart snapshot-update.timer 30 || exit $ERR_SNAPSHOT_UPDATE_START_FAIL
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

configGPUDrivers() {
    if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
        mkdir -p /opt/{actions,gpu}
        ctr -n k8s.io image pull $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
        retrycmd_if_failure 5 10 600 bash -c "$CTR_GPU_INSTALL_CMD $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG gpuinstall /entrypoint.sh install"
        ret=$?
        if [ "$ret" -ne 0 ]; then
            echo "Failed to install GPU driver, exiting..."
            exit $ERR_GPU_DRIVERS_START_FAIL
        fi
        ctr -n k8s.io images rm --sync $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
    elif isMarinerOrAzureLinux "$OS" && ! isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
        downloadGPUDrivers
        installNvidiaContainerToolkit
        enableNvidiaPersistenceMode
    else
        echo "os $OS $OS_VARIANT not supported at this time. skipping configGPUDrivers"
        exit 1
    fi

    retrycmd_if_failure 120 5 25 nvidia-modprobe -u -c0 || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 300 nvidia-smi || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 25 ldconfig || exit $ERR_GPU_DRIVERS_START_FAIL

    # Fix the NVIDIA /dev/char link issue
    if isMarinerOrAzureLinux "$OS"; then
        createNvidiaSymlinkToAllDeviceNodes
    fi

    retrycmd_if_failure 120 5 25 pkill -SIGHUP containerd || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
}

validateGPUDrivers() {
    if [ "$(isARM64)" -eq 1 ]; then
        return
    fi

    retrycmd_if_failure 24 5 25 nvidia-modprobe -u -c0 && echo "gpu driver loaded" || configGPUDrivers || exit $ERR_GPU_DRIVERS_START_FAIL

    if which nvidia-smi; then
        SMI_RESULT=$(retrycmd_if_failure 24 5 300 nvidia-smi)
    else
        SMI_RESULT=$(retrycmd_if_failure 24 5 300 $GPU_DEST/bin/nvidia-smi)
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
  # Match User *@*,????????-????-????-????-????????????    # Added by aadsshlogin installer
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
  sudo sshd -t -f "$TMP" || { rm -f "$TMP"; exit $ERR_CONFIG_PUBKEY_AUTH_SSH; }

  # Replace the original with the candidate (permissions 644, owned by root)
  sudo install -m 644 -o root -g root "$TMP" "$SSHD_CONFIG"
  rm -f "$TMP"

  # Reload sshd
  sudo systemctl reload sshd || sudo systemctl restart sshd || exit $ERR_CONFIG_PUBKEY_AUTH_SSH
}

configCredentialProvider() {
    CREDENTIAL_PROVIDER_CONFIG_FILE=/var/lib/kubelet/credential-provider-config.yaml
    mkdir -p "$(dirname "${CREDENTIAL_PROVIDER_CONFIG_FILE}")"
    touch "${CREDENTIAL_PROVIDER_CONFIG_FILE}"
    if [ -n "$AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX" ]; then
        echo "configure credential provider for custom cloud"
        tee "${CREDENTIAL_PROVIDER_CONFIG_FILE}" > /dev/null <<EOF
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
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    args:
      - /etc/kubernetes/azure.json
EOF
    elif [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
        echo "configure credential provider for network isolated cluster"
        tee "${CREDENTIAL_PROVIDER_CONFIG_FILE}" > /dev/null <<EOF
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "mcr.microsoft.com"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    args:
      - /etc/kubernetes/azure.json
      - --registry-mirror=mcr.microsoft.com:$BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER
EOF
    else
        echo "configure credential provider with default settings"
        tee "${CREDENTIAL_PROVIDER_CONFIG_FILE}" > /dev/null <<EOF
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
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    args:
      - /etc/kubernetes/azure.json
EOF
    fi
}

setKubeletNodeIPFlag() {
    imdsOutput=$(curl -s -H Metadata:true --noproxy "*" --max-time 5 "http://169.254.169.254/metadata/instance/network/interface?api-version=2021-02-01" 2> /dev/null)
    if [ "$?" -eq 0 ]; then
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
    fi
}

# localdns corefile is created only when localdns profile has state enabled.
# This should match with 'path' defined in parts/linux/cloud-init/nodecustomdata.yml.
LOCALDNS_CORE_FILE="/opt/azure/containers/localdns/localdns.corefile"
# This function is called from cse_main.sh.
# It first checks if localdns should be enabled by checking for existence of corefile.
# It returns 0 if localdns is enabled successfully or if it should not be enabled.
# It returns a non-zero value if localdns should be enabled but there was a failure in enabling it.
enableLocalDNS() {
    # Check if the localdns corefile exists and is not empty.
    # If the corefile exists and is not empty, localdns should be enabled.
    # If the corefile does not exist or is empty, localdns should not be enabled.
    if [ ! -f "${LOCALDNS_CORE_FILE}" ] || [ ! -s "${LOCALDNS_CORE_FILE}" ]; then
        echo "localdns should not be enabled."
        return 0
    fi

    # If the corefile exists and is not empty, attempt to enable localdns.
    echo "localdns should be enabled."

    if ! systemctlEnableAndStart localdns 30; then
      echo "Enable localdns failed."
      return $ERR_LOCALDNS_FAIL
    fi

    # Enabling localdns succeeded.
    echo "Enable localdns succeeded."
}

# localdns corefile used by localdns systemd unit.
LOCALDNS_COREFILE="/opt/azure/containers/localdns/localdns.corefile"
# localdns slice file used by localdns systemd unit.
LOCALDNS_SLICEFILE="/etc/systemd/system/localdns.slice"
# This function is called from cse_main.sh.
# It creates the localdns corefile and slicefile, then enables and starts localdns.
# In this function, generated base64 encoded localdns corefile is decoded and written to the corefile path.
# This function also creates the localdns slice file with memory and cpu limits, that will be used by localdns systemd unit.
shouldEnableLocalDns() {
    mkdir -p "$(dirname "${LOCALDNS_COREFILE}")"
    touch "${LOCALDNS_COREFILE}"
    chmod 0644 "${LOCALDNS_COREFILE}"
    echo "${LOCALDNS_GENERATED_COREFILE}" | base64 -d > "${LOCALDNS_COREFILE}" || exit $ERR_LOCALDNS_FAIL

	mkdir -p "$(dirname "${LOCALDNS_SLICEFILE}")"
    touch "${LOCALDNS_SLICEFILE}"
    chmod 0644 "${LOCALDNS_SLICEFILE}"
    cat > "${LOCALDNS_SLICEFILE}" <<EOF
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

    echo "localdns should be enabled."
    systemctlEnableAndStart localdns 30 || exit $ERR_LOCALDNS_FAIL
    echo "Enable localdns succeeded."
}

startNvidiaManagedExpServices() {
    # 1. Start the nvidia-device-plugin service.
    # Create systemd override directory to configure device plugin
    NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR="/etc/systemd/system/nvidia-device-plugin.service.d"
    mkdir -p "${NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR}"

    if [ "${MIG_NODE}" = "true" ]; then
        # Configure with MIG strategy for MIG nodes
        tee "${NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR}/10-device-plugin-config.conf" > /dev/null <<'EOF'
[Service]
Environment="MIG_STRATEGY=--mig-strategy single"
ExecStart=
ExecStart=/usr/bin/nvidia-device-plugin $MIG_STRATEGY --pass-device-specs
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
    logs_to_events "AKS.CSE.start.nvidia-dcgm" "systemctlEnableAndStart nvidia-dcgm 30" || exit $ERR_NVIDIA_DCGM_FAIL

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
    logs_to_events "AKS.CSE.start.nvidia-dcgm-exporter" "systemctlEnableAndStart nvidia-dcgm-exporter 30" || exit $ERR_NVIDIA_DCGM_EXPORTER_FAIL
}

#EOF
