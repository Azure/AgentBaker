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

configureSwapFile() {
    swap_size_kb=$(expr ${SWAP_FILE_SIZE_MB} \* 1000)
    swap_location=""
    
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

    if [[ -z "${swap_location}" ]]; then
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
    if [[ $OS == $MARINER_OS_NAME ]]; then
        cert_dest="/usr/share/pki/ca-trust-source/anchors"
        update_cmd="update-ca-trust"
    else
        cert_dest="/usr/local/share/ca-certificates"
        update_cmd="update-ca-certificates"
    fi
    echo "${HTTP_PROXY_TRUSTED_CA}" | base64 -d > "${cert_dest}/proxyCA.crt" || exit $ERR_UPDATE_CA_CERTS
    $update_cmd || exit $ERR_UPDATE_CA_CERTS
}

configureCustomCaCertificate() {
    mkdir -p /opt/certs
    for i in $(seq 0 $((${CUSTOM_CA_TRUST_COUNT} - 1))); do
        declare varname=CUSTOM_CA_CERT_${i} 
        echo "${!varname}" | base64 -d > /opt/certs/00000000000000cert${i}.crt
    done
    systemctl restart update_certs.service || exit $ERR_UPDATE_CA_CERTS
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
    if [ -n "${KUBELET_CLIENT_CONTENT}" ]; then
        echo "${KUBELET_CLIENT_CONTENT}" | base64 -d > /etc/kubernetes/certs/client.key
    fi
    if [ -n "${KUBELET_CLIENT_CERT_CONTENT}" ]; then
        echo "${KUBELET_CLIENT_CERT_CONTENT}" | base64 -d > /etc/kubernetes/certs/client.crt
    fi
    if [ -n "${SERVICE_PRINCIPAL_FILE_CONTENT}" ]; then
        echo "${SERVICE_PRINCIPAL_FILE_CONTENT}" | base64 -d > /etc/kubernetes/sp.txt
    fi

    set +x
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

    configureKubeletServerCert
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
    if [[ "${UBUNTU_RELEASE}" == "18.04" || "${UBUNTU_RELEASE}" == "20.04" || "${UBUNTU_RELEASE}" == "22.04" ]]; then
        echo "Ingorings systemd-resolved query service but using its resolv.conf file"
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

ensureKubelet() {
    KUBELET_DEFAULT_FILE=/etc/default/kubelet
    mkdir -p /etc/default
    echo "KUBELET_FLAGS=${KUBELET_FLAGS}" > "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_REGISTER_SCHEDULABLE=true" >> "${KUBELET_DEFAULT_FILE}"
    echo "NETWORK_POLICY=${NETWORK_POLICY}" >> "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_IMAGE=${KUBELET_IMAGE}" >> "${KUBELET_DEFAULT_FILE}"
    echo "KUBELET_NODE_LABELS=${KUBELET_NODE_LABELS}" >> "${KUBELET_DEFAULT_FILE}"
    if [ -n "${AZURE_ENVIRONMENT_FILEPATH}" ]; then
        echo "AZURE_ENVIRONMENT_FILEPATH=${AZURE_ENVIRONMENT_FILEPATH}" >> "${KUBELET_DEFAULT_FILE}"
    fi
    
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
#
#
#
iptables -I FORWARD -d 168.63.129.16 -p tcp --dport 80 -j DROP
EOF
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
    elif [[ $OS == $MARINER_OS_NAME ]]; then
        downloadGPUDrivers
        installNvidiaContainerRuntime
        enableNvidiaPersistenceMode
    else 
        echo "os $OS not supported at this time. skipping configGPUDrivers"
        exit 1
    fi

    retrycmd_if_failure 120 5 25 nvidia-modprobe -u -c0 || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 300 nvidia-smi || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 25 ldconfig || exit $ERR_GPU_DRIVERS_START_FAIL

    if [[ $OS == $MARINER_OS_NAME ]]; then
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

#EOF