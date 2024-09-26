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
  systemctlEnableAndStart reconcile-private-hosts || exit $ERR_SYSTEMCTL_START_FAIL
}
configureTransparentHugePage() {
    ETC_SYSFS_CONF="/etc/sysfs.conf"
    if [[ "${THP_ENABLED}" != "" ]]; then
        echo "${THP_ENABLED}" > /sys/kernel/mm/transparent_hugepage/enabled
        echo "kernel/mm/transparent_hugepage/enabled=${THP_ENABLED}" >> ${ETC_SYSFS_CONF}
    fi
    if [[ "${THP_DEFRAG}" != "" ]]; then
        echo "${THP_DEFRAG}" > /sys/kernel/mm/transparent_hugepage/defrag
        echo "kernel/mm/transparent_hugepage/defrag=${THP_DEFRAG}" >> ${ETC_SYSFS_CONF}
    fi
}

configureSystemdUseDomains() {
    NETWORK_CONFIG_FILE="/etc/systemd/networkd.conf"

    if awk '/^\[DHCPv4\]/{flag=1; next} /^\[/{flag=0} flag && /#UseDomains=no/' "$NETWORK_CONFIG_FILE"; then
        sed -i '/^\[DHCPv4\]/,/^\[/ s/#UseDomains=no/UseDomains=yes/' $NETWORK_CONFIG_FILE
    fi

    if [ "${IPV6_DUAL_STACK_ENABLED}" == "true" ]; then
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
    if [[ -L /dev/disk/azure/resource-part1 ]]; then
        resource_disk_path=$(findmnt -nr -o target -S $(readlink -f /dev/disk/azure/resource-part1))
        disk_free_kb=$(df ${resource_disk_path} | sed 1d | awk '{print $4}')
        if [[ ${disk_free_kb} -gt ${swap_size_kb} ]]; then
            echo "Will use resource disk for swap file"
            swap_location=${resource_disk_path}/swapfile
        else
            echo "Insufficient disk space on resource disk to create swap file: request ${swap_size_kb} free ${disk_free_kb}, attempting to fall back to OS disk..."
        fi
    fi

    # If we couldn't use the resource disk, attempt to use the OS disk
    if [[ -z "${swap_location}" ]]; then
        # Directly check size on the root directory since we can't rely on 'root-part1' always being the correct label
        os_device=$(readlink -f /dev/disk/azure/root)
        disk_free_kb=$(df -P / | sed 1d | awk '{print $4}')
        if [[ ${disk_free_kb} -gt ${swap_size_kb} ]]; then
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
    if [ "${HTTP_PROXY_URLS}" != "" ]; then
        echo "HTTP_PROXY=${HTTP_PROXY_URLS}" >> /etc/environment
        echo "http_proxy=${HTTP_PROXY_URLS}" >> /etc/environment
        echo "Acquire::http::proxy \"${HTTP_PROXY_URLS}\";" >> /etc/apt/apt.conf.d/95proxy
        echo "DefaultEnvironment=\"HTTP_PROXY=${HTTP_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
        echo "DefaultEnvironment=\"http_proxy=${HTTP_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
    fi
    if [ "${HTTPS_PROXY_URLS}" != "" ]; then
        echo "HTTPS_PROXY=${HTTPS_PROXY_URLS}" >> /etc/environment
        echo "https_proxy=${HTTPS_PROXY_URLS}" >> /etc/environment
        echo "Acquire::https::proxy \"${HTTPS_PROXY_URLS}\";" >> /etc/apt/apt.conf.d/95proxy
        echo "DefaultEnvironment=\"HTTPS_PROXY=${HTTPS_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
        echo "DefaultEnvironment=\"https_proxy=${HTTPS_PROXY_URLS}\"" >> /etc/systemd/system.conf.d/proxy.conf
    fi
    if [ "${NO_PROXY_URLS}" != "" ]; then
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
    if isMarinerOrAzureLinux "$OS"; then
        cert_dest="/usr/share/pki/ca-trust-source/anchors"
        update_cmd="update-ca-trust"
    else
        cert_dest="/usr/local/share/ca-certificates"
        update_cmd="update-ca-certificates"
    fi
    HTTP_PROXY_TRUSTED_CA=$(echo "${HTTP_PROXY_TRUSTED_CA}" | xargs)
    echo "${HTTP_PROXY_TRUSTED_CA}" | base64 -d > "${cert_dest}/proxyCA.crt" || exit $ERR_UPDATE_CA_CERTS
    $update_cmd || exit $ERR_UPDATE_CA_CERTS
}

configureCustomCaCertificate() {
    mkdir -p /opt/certs
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

# this simply generates a self-signed certificate used for serving by the kubelet
configureKubeletServerCert() {
    KUBELET_SERVER_PRIVATE_KEY_PATH="/etc/kubernetes/certs/kubeletserver.key"
    KUBELET_SERVER_CERT_PATH="/etc/kubernetes/certs/kubeletserver.crt"

    openssl genrsa -out $KUBELET_SERVER_PRIVATE_KEY_PATH 2048
    openssl req -new -x509 -days 7300 -key $KUBELET_SERVER_PRIVATE_KEY_PATH -out $KUBELET_SERVER_CERT_PATH -subj "/CN=${NODE_NAME}" -addext "subjectAltName=DNS:${NODE_NAME}"
}

configureK8s() {
    APISERVER_PUBLIC_KEY_PATH="/etc/kubernetes/certs/apiserver.crt"
    touch "${APISERVER_PUBLIC_KEY_PATH}"
    chmod 0644 "${APISERVER_PUBLIC_KEY_PATH}"
    chown root:root "${APISERVER_PUBLIC_KEY_PATH}"

    AZURE_JSON_PATH="/etc/kubernetes/azure.json"
    touch "${AZURE_JSON_PATH}"
    chmod 0600 "${AZURE_JSON_PATH}"
    chown root:root "${AZURE_JSON_PATH}"

    mkdir -p "/etc/kubernetes/certs"

    set +x
    if [ -n "${KUBELET_CLIENT_CONTENT}" ]; then
        echo "${KUBELET_CLIENT_CONTENT}" | base64 -d > /etc/kubernetes/certs/client.key
    fi
    if [ -n "${KUBELET_CLIENT_CERT_CONTENT}" ]; then
        echo "${KUBELET_CLIENT_CERT_CONTENT}" | base64 -d > /etc/kubernetes/certs/client.crt
    fi
    if [ -n "${SERVICE_PRINCIPAL_FILE_CONTENT}" ]; then
        echo "${SERVICE_PRINCIPAL_FILE_CONTENT}" | base64 -d > /etc/kubernetes/sp.txt
    fi

    echo "${APISERVER_PUBLIC_KEY}" | base64 --decode > "${APISERVER_PUBLIC_KEY_PATH}"
    SP_FILE="/etc/kubernetes/sp.txt"
    SERVICE_PRINCIPAL_CLIENT_SECRET="$(cat "$SP_FILE")"
    SERVICE_PRINCIPAL_CLIENT_SECRET=${SERVICE_PRINCIPAL_CLIENT_SECRET//\\/\\\\}
    SERVICE_PRINCIPAL_CLIENT_SECRET=${SERVICE_PRINCIPAL_CLIENT_SECRET//\"/\\\"}
    rm "$SP_FILE"
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
    if [[ "${CLOUDPROVIDER_BACKOFF_MODE}" = "v2" ]]; then
        sed -i "/cloudProviderBackoffExponent/d" /etc/kubernetes/azure.json
        sed -i "/cloudProviderBackoffJitter/d" /etc/kubernetes/azure.json
    fi

    # generate a kubelet serving certificate if we aren't relying on TLS bootstrapping to generate one for us.
    # NOTE: in the case where ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION is true but 
    # the customer has disabled serving certificate rotation via nodepool tags,
    # the self-signed serving certificate will be bootstrapped by the kubelet instead of this function
    # TODO(cameissner): remove configureKubeletServerCert altogether
    if [ "${ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION}" != "true" ]; then
        configureKubeletServerCert
    fi

    if [ "${IS_CUSTOM_CLOUD}" == "true" ]; then
        set +x
        AKS_CUSTOM_CLOUD_JSON_PATH="/etc/kubernetes/${TARGET_ENVIRONMENT}.json"
        touch "${AKS_CUSTOM_CLOUD_JSON_PATH}"
        chmod 0600 "${AKS_CUSTOM_CLOUD_JSON_PATH}"
        chown root:root "${AKS_CUSTOM_CLOUD_JSON_PATH}"

        echo "${CUSTOM_ENV_JSON}" | base64 -d > "${AKS_CUSTOM_CLOUD_JSON_PATH}"
        set -x
    fi

    if [ "${KUBELET_CONFIG_FILE_ENABLED}" == "true" ]; then
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
    if [[ "${NETWORK_PLUGIN}" = "azure" ]]; then
        mv $CNI_BIN_DIR/10-azure.conflist $CNI_CONFIG_DIR/
        chmod 600 $CNI_CONFIG_DIR/10-azure.conflist
        if [[ "${NETWORK_POLICY}" == "calico" ]]; then
          sed -i 's#"mode":"bridge"#"mode":"transparent"#g' $CNI_CONFIG_DIR/10-azure.conflist
        elif [[ "${NETWORK_POLICY}" == "" || "${NETWORK_POLICY}" == "none" ]] && [[ "${NETWORK_MODE}" == "transparent" ]]; then
          sed -i 's#"mode":"bridge"#"mode":"transparent"#g' $CNI_CONFIG_DIR/10-azure.conflist
        fi
        /sbin/ebtables -t nat --list
    fi
}

disableSystemdResolved() {
    ls -ltr /etc/resolv.conf
    cat /etc/resolv.conf
    UBUNTU_RELEASE=$(lsb_release -r -s)
    if [[ "${UBUNTU_RELEASE}" == "18.04" || "${UBUNTU_RELEASE}" == "20.04" || "${UBUNTU_RELEASE}" == "22.04" || "${UBUNTU_RELEASE}" == "24.04" ]]; then
        echo "Ingoring systemd-resolved query service but using its resolv.conf file"
        echo "This is the simplest approach to workaround resolved issues without completely uninstall it"
        [ -f /run/systemd/resolve/resolv.conf ] && sudo ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
        ls -ltr /etc/resolv.conf
        cat /etc/resolv.conf
    fi
}

ensureContainerd() {
  if [ "${TELEPORT_ENABLED}" == "true" ]; then
    ensureTeleportd
  fi
  mkdir -p "/etc/systemd/system/containerd.service.d" 
  tee "/etc/systemd/system/containerd.service.d/exec_start.conf" > /dev/null <<EOF
[Service]
ExecStartPost=/sbin/iptables -P FORWARD ACCEPT
EOF

  if [ "${ARTIFACT_STREAMING_ENABLED}" == "true" ]; then
    logs_to_events "AKS.CSE.ensureContainerd.ensureArtifactStreaming" ensureArtifactStreaming || exit $ERR_ARTIFACT_STREAMING_INSTALL
  fi

  mkdir -p /etc/containerd
  if [[ "${GPU_NODE}" = true ]] && [[ "${skip_nvidia_driver_install}" == "true" ]]; then
    echo "Generating non-GPU containerd config for GPU node due to VM tags"
    echo "${CONTAINERD_CONFIG_NO_GPU_CONTENT}" | base64 -d > /etc/containerd/config.toml || exit $ERR_FILE_WATCH_TIMEOUT
  else
    echo "Generating containerd config..."
    echo "${CONTAINERD_CONFIG_CONTENT}" | base64 -d > /etc/containerd/config.toml || exit $ERR_FILE_WATCH_TIMEOUT
  fi

  tee "/etc/sysctl.d/99-force-bridge-forward.conf" > /dev/null <<EOF 
net.ipv4.ip_forward = 1
net.ipv4.conf.all.forwarding = 1
net.ipv6.conf.all.forwarding = 1
net.bridge.bridge-nf-call-iptables = 1
EOF
  retrycmd_if_failure 120 5 25 sysctl --system || exit $ERR_SYSCTL_RELOAD
  systemctl is-active --quiet docker && (systemctl_disable 20 30 120 docker || exit $ERR_SYSTEMD_DOCKER_STOP_FAIL)
  systemctlEnableAndStart containerd || exit $ERR_SYSTEMCTL_START_FAIL
}

ensureNoDupOnPromiscuBridge() {
    systemctlEnableAndStart ensure-no-dup || exit $ERR_SYSTEMCTL_START_FAIL
}

ensureTeleportd() {
    systemctlEnableAndStart teleportd || exit $ERR_SYSTEMCTL_START_FAIL
}

ensureArtifactStreaming() {
  systemctl enable acr-mirror.service
  systemctl start acr-mirror.service
  sudo /opt/acr/tools/overlaybd/install.sh
  sudo /opt/acr/tools/overlaybd/enable-http-auth.sh
  sudo /opt/acr/tools/overlaybd/config.sh download.enable false
  sudo /opt/acr/tools/overlaybd/config.sh cacheConfig.cacheSizeGB 32
  sudo /opt/acr/tools/overlaybd/config.sh exporterConfig.enable true
  sudo /opt/acr/tools/overlaybd/config.sh exporterConfig.port 9863
  modprobe target_core_user
  curl -X PUT 'localhost:8578/config?ns=_default&enable_suffix=azurecr.io&stream_format=overlaybd' -O
  systemctl enable /opt/overlaybd/overlaybd-tcmu.service
  systemctl enable /opt/overlaybd/snapshotter/overlaybd-snapshotter.service
  systemctl start overlaybd-tcmu
  systemctl start overlaybd-snapshotter
  systemctl start acr-nodemon
}

ensureDocker() {
    DOCKER_SERVICE_EXEC_START_FILE=/etc/systemd/system/docker.service.d/exec_start.conf
    usermod -aG docker ${ADMINUSER}
    DOCKER_MOUNT_FLAGS_SYSTEMD_FILE=/etc/systemd/system/docker.service.d/clear_mount_propagation_flags.conf
    DOCKER_JSON_FILE=/etc/docker/daemon.json
    for i in $(seq 1 1200); do
        if [ -s $DOCKER_JSON_FILE ]; then
            jq '.' < $DOCKER_JSON_FILE && break
        fi
        if [ $i -eq 1200 ]; then
            exit $ERR_FILE_WATCH_TIMEOUT
        else
            sleep 1
        fi
    done
    systemctl is-active --quiet containerd && (systemctl_disable 20 30 120 containerd || exit $ERR_SYSTEMD_CONTAINERD_STOP_FAIL)
    systemctlEnableAndStart docker || exit $ERR_DOCKER_START_FAIL

}

ensureDHCPv6() {
    systemctlEnableAndStart dhcpv6 || exit $ERR_SYSTEMCTL_START_FAIL
    retrycmd_if_failure 120 5 25 modprobe ip6_tables || exit $ERR_MODPROBE_FAIL
}

getPrimaryNicIP() {
    local sleepTime=1
    local maxRetries=10
    local i=0
    local ip=""

    while [[ $i -lt $maxRetries ]]; do
        ip=$(curl -sSL -H "Metadata: true" "http://169.254.169.254/metadata/instance/network/interface?api-version=2021-02-01" | jq -r '.[].ipv4.ipAddress[0].privateIpAddress')
        if [[ -n "$ip" && $? -eq 0 ]]; then
            break
        fi
        sleep $sleepTime
        i=$((i+1))
    done
    echo "$ip"
}

# removes the specified LABEL_STRING (which should be in the form of 'label=value') from KUBELET_NODE_LABELS
clearKubeletNodeLabel() {
    local LABEL_STRING=$1
    if echo "$KUBELET_NODE_LABELS" | grep -e ",${LABEL_STRING}"; then
        KUBELET_NODE_LABELS="${KUBELET_NODE_LABELS/,${LABEL_STRING}/}"
    elif echo "$KUBELET_NODE_LABELS" | grep -e "${LABEL_STRING},"; then
        KUBELET_NODE_LABELS="${KUBELET_NODE_LABELS/${LABEL_STRING},/}"
    elif echo "$KUBELET_NODE_LABELS" | grep -e "${LABEL_STRING}"; then
        KUBELET_NODE_LABELS="${KUBELET_NODE_LABELS/${LABEL_STRING}/}"
    fi
}

disableKubeletServingCertificateRotationForTags() {
    if [[ "${ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION}" != "true"  ]]; then
        echo "kubelet serving certificate rotation is already disabled"
        return 0
    fi

    # check if kubelet serving certificate rotation is disabled by customer-specified nodepool tags
    export -f should_disable_kubelet_serving_certificate_rotation
    DISABLE_KUBELET_SERVING_CERTIFICATE_ROTATION=$(retrycmd_if_failure_no_stats 10 1 10 bash -cx should_disable_kubelet_serving_certificate_rotation)
    if [ $? -ne 0 ]; then
        echo "failed to determine if kubelet serving certificate rotation should be disabled by nodepool tags"
        exit $ERR_LOOKUP_DISABLE_KUBELET_SERVING_CERTIFICATE_ROTATION_TAG
    fi

    if [ "${DISABLE_KUBELET_SERVING_CERTIFICATE_ROTATION,,}" != "true" ]; then
        echo "nodepool tag \"aks-disable-kubelet-serving-certificate-rotation\" is not true, nothing to disable"
        return 0
    fi

    echo "kubelet serving certificate rotation is disabled by nodepool tags, reconfiguring kubelet flags and node labels..."

    # set the --rotate-server-certificates flag to false if needed
    KUBELET_FLAGS="${KUBELET_FLAGS/--rotate-server-certificates=true/--rotate-server-certificates=false}"

    if [ "${KUBELET_CONFIG_FILE_ENABLED,,}" == "true" ]; then
        set +x
        # set the serverTLSBootstrap property to false if needed
        KUBELET_CONFIG_FILE_CONTENT=$(echo "$KUBELET_CONFIG_FILE_CONTENT" | base64 -d | jq 'if .serverTLSBootstrap == true then .serverTLSBootstrap = false else . end' | base64)
        set -x
    fi
    
    # remove the "kubernetes.azure.com/kubelet-serving-ca=cluster" label if needed
    clearKubeletNodeLabel "kubernetes.azure.com/kubelet-serving-ca=cluster"
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

    echo "KUBELET_FLAGS=${KUBELET_FLAGS}" > "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_REGISTER_SCHEDULABLE=true" >> "${KUBELET_DEFAULT_FILE}"
    echo "NETWORK_POLICY=${NETWORK_POLICY}" >> "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_IMAGE=${KUBELET_IMAGE}" >> "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_NODE_LABELS=${KUBELET_NODE_LABELS}" >> "${KUBELET_DEFAULT_FILE}"
    if [ -n "${AZURE_ENVIRONMENT_FILEPATH}" ]; then
        echo "AZURE_ENVIRONMENT_FILEPATH=${AZURE_ENVIRONMENT_FILEPATH}" >> "${KUBELET_DEFAULT_FILE}"
    fi
    chmod 0600 "${KUBELET_DEFAULT_FILE}"
    
    KUBE_CA_FILE="/etc/kubernetes/certs/ca.crt"
    mkdir -p "$(dirname "${KUBE_CA_FILE}")"
    echo "${KUBE_CA_CRT}" | base64 -d > "${KUBE_CA_FILE}"
    chmod 0600 "${KUBE_CA_FILE}"

    if [ "${ENABLE_SECURE_TLS_BOOTSTRAPPING}" == "true" ] || [ "${ENABLE_TLS_BOOTSTRAPPING}" == "true" ]; then
        KUBELET_TLS_DROP_IN="/etc/systemd/system/kubelet.service.d/10-tlsbootstrap.conf"
        mkdir -p "$(dirname "${KUBELET_TLS_DROP_IN}")"
        touch "${KUBELET_TLS_DROP_IN}"
        chmod 0600 "${KUBELET_TLS_DROP_IN}"
        tee "${KUBELET_TLS_DROP_IN}" > /dev/null <<EOF
[Service]
Environment="KUBELET_TLS_BOOTSTRAP_FLAGS=--kubeconfig /var/lib/kubelet/kubeconfig --bootstrap-kubeconfig /var/lib/kubelet/bootstrap-kubeconfig"
EOF
    fi

    if [ "${ENABLE_SECURE_TLS_BOOTSTRAPPING}" == "true" ]; then
        AAD_RESOURCE="6dae42f8-4368-4678-94ff-3960e28e3630"
        if [ -n "$CUSTOM_SECURE_TLS_BOOTSTRAP_AAD_SERVER_APP_ID" ]; then
            AAD_RESOURCE=$CUSTOM_SECURE_TLS_BOOTSTRAP_AAD_SERVER_APP_ID
        fi
        SECURE_BOOTSTRAP_KUBECONFIG_FILE=/var/lib/kubelet/bootstrap-kubeconfig
        mkdir -p "$(dirname "${SECURE_BOOTSTRAP_KUBECONFIG_FILE}")"
        touch "${SECURE_BOOTSTRAP_KUBECONFIG_FILE}"
        chmod 0644 "${SECURE_BOOTSTRAP_KUBECONFIG_FILE}"
        tee "${SECURE_BOOTSTRAP_KUBECONFIG_FILE}" > /dev/null <<EOF
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
    exec:
        apiVersion: client.authentication.k8s.io/v1
        command: /opt/azure/tlsbootstrap/tls-bootstrap-client
        args:
        - bootstrap
        - --next-proto=aks-tls-bootstrap
        - --aad-resource=${AAD_RESOURCE}
        interactiveMode: Never
        provideClusterInfo: true
contexts:
- context:
    cluster: localcluster
    user: kubelet-bootstrap
  name: bootstrap-context
current-context: bootstrap-context
EOF
    elif [ "${ENABLE_TLS_BOOTSTRAPPING}" == "true" ]; then
        BOOTSTRAP_KUBECONFIG_FILE=/var/lib/kubelet/bootstrap-kubeconfig
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
    token: "${TLS_BOOTSTRAP_TOKEN}"
contexts:
- context:
    cluster: localcluster
    user: kubelet-bootstrap
  name: bootstrap-context
current-context: bootstrap-context
EOF
    else
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
    if [[ $KUBELET_FLAGS == *"image-credential-provider-config"* && $KUBELET_FLAGS == *"image-credential-provider-bin-dir"* ]]; then
        echo "Configure credential provider for both image-credential-provider-config and image-credential-provider-bin-dir flags are specified in KUBELET_FLAGS"
        logs_to_events "AKS.CSE.ensureKubelet.configCredentialProvider" configCredentialProvider
        logs_to_events "AKS.CSE.ensureKubelet.installCredentialProvider" installCredentialProvider
    fi

    systemctlEnableAndStart kubelet || exit $ERR_KUBELET_START_FAIL
}

ensureSnapshotUpdate() {
    systemctlEnableAndStart snapshot-update.timer || exit $ERR_SNAPSHOT_UPDATE_START_FAIL
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
    systemctlEnableAndStart mig-partition
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
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        mkdir -p /opt/{actions,gpu}
        if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
            ctr image pull $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
            retrycmd_if_failure 5 10 600 bash -c "$CTR_GPU_INSTALL_CMD $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG gpuinstall /entrypoint.sh install"
            ret=$?
            if [[ "$ret" != "0" ]]; then
                echo "Failed to install GPU driver, exiting..."
                exit $ERR_GPU_DRIVERS_START_FAIL
            fi
            ctr images rm --sync $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
        else
            bash -c "$DOCKER_GPU_INSTALL_CMD $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG install" 
            ret=$?
            if [[ "$ret" != "0" ]]; then
                echo "Failed to install GPU driver, exiting..."
                exit $ERR_GPU_DRIVERS_START_FAIL
            fi
            docker rmi $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
        fi
    elif isMarinerOrAzureLinux "$OS"; then
        downloadGPUDrivers
        installNvidiaContainerToolkit
        enableNvidiaPersistenceMode
    else 
        echo "os $OS not supported at this time. skipping configGPUDrivers"
        exit 1
    fi

    retrycmd_if_failure 120 5 25 nvidia-modprobe -u -c0 || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 300 nvidia-smi || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 25 ldconfig || exit $ERR_GPU_DRIVERS_START_FAIL

    # Fix the NVIDIA /dev/char link issue
    if isMarinerOrAzureLinux "$OS"; then
        createNvidiaSymlinkToAllDeviceNodes
    fi
    
    if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
        retrycmd_if_failure 120 5 25 pkill -SIGHUP containerd || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    else
        retrycmd_if_failure 120 5 25 pkill -SIGHUP dockerd || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    fi
}

validateGPUDrivers() {
    if [[ $(isARM64) == 1 ]]; then
        return
    fi

    retrycmd_if_failure 24 5 25 nvidia-modprobe -u -c0 && echo "gpu driver loaded" || configGPUDrivers || exit $ERR_GPU_DRIVERS_START_FAIL
    which nvidia-smi
    if [[ $? == 0 ]]; then
        SMI_RESULT=$(retrycmd_if_failure 24 5 300 nvidia-smi)
    else
        SMI_RESULT=$(retrycmd_if_failure 24 5 300 $GPU_DEST/bin/nvidia-smi)
    fi
    SMI_STATUS=$?
    if [[ $SMI_STATUS != 0 ]]; then
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
    if [[ $(isARM64) == 1 ]]; then
        return
    fi

    if [[ "${CONFIG_GPU_DRIVER_IF_NEEDED}" = true ]]; then
        logs_to_events "AKS.CSE.ensureGPUDrivers.configGPUDrivers" configGPUDrivers
    else
        logs_to_events "AKS.CSE.ensureGPUDrivers.validateGPUDrivers" validateGPUDrivers
    fi
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        logs_to_events "AKS.CSE.ensureGPUDrivers.nvidia-modprobe" "systemctlEnableAndStart nvidia-modprobe" || exit $ERR_GPU_DRIVERS_START_FAIL
    fi
}

disableSSH() {
    systemctlDisableAndStop ssh || exit $ERR_DISABLE_SSH
}

configCredentialProvider() {
    CREDENTIAL_PROVIDER_CONFIG_FILE=/var/lib/kubelet/credential-provider-config.yaml
    mkdir -p "$(dirname "${CREDENTIAL_PROVIDER_CONFIG_FILE}")"
    touch "${CREDENTIAL_PROVIDER_CONFIG_FILE}"
    if [[ -n "$AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX" ]]; then
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
    else
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
    if [[ $? -eq 0 ]]; then
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

#EOF