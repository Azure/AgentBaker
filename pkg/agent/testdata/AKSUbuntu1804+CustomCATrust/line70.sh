#!/bin/bash
NODE_INDEX=$(hostname | tail -c 2)
NODE_NAME=$(hostname)

configureAdminUser(){
    chage -E -1 -I -1 -m 0 -M 99999 "${ADMINUSER}"
    chage -l "${ADMINUSER}"
}

ensureRPC() {
    systemctlEnableAndStart rpcbind || exit $ERR_SYSTEMCTL_START_FAIL
    systemctlEnableAndStart rpc-statd || exit $ERR_SYSTEMCTL_START_FAIL
}

configureHTTPProxyCA() {
    wait_for_file 1200 1 /usr/local/share/ca-certificates/proxyCA.crt || exit $ERR_FILE_WATCH_TIMEOUT
    update-ca-certificates || exit $ERR_UPDATE_CA_CERTS
}

configureCustomCaCertificate() {
    wait_for_file 1200 1 /opt/certs/00000000000000cert0.crt || exit $ERR_FILE_WATCH_TIMEOUT
    wait_for_file 1200 1 /opt/certs/00000000000000cert1.crt || exit $ERR_FILE_WATCH_TIMEOUT
    wait_for_file 1200 1 /opt/certs/00000000000000cert2.crt || exit $ERR_FILE_WATCH_TIMEOUT
    systemctl restart update_certs.service || exit $ERR_UPDATE_CA_CERTS
}


configureKubeletServerCert() {
    KUBELET_SERVER_PRIVATE_KEY_PATH="/etc/kubernetes/certs/kubeletserver.key"
    KUBELET_SERVER_CERT_PATH="/etc/kubernetes/certs/kubeletserver.crt"

    openssl genrsa -out $KUBELET_SERVER_PRIVATE_KEY_PATH 2048
    openssl req -new -x509 -days 7300 -key $KUBELET_SERVER_PRIVATE_KEY_PATH -out $KUBELET_SERVER_CERT_PATH -subj "/CN=${NODE_NAME}"
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

    SP_FILE="/etc/kubernetes/sp.txt"

    wait_for_file 1200 1 /etc/kubernetes/certs/client.key || exit $ERR_FILE_WATCH_TIMEOUT
    wait_for_file 1200 1 "$SP_FILE" || exit $ERR_FILE_WATCH_TIMEOUT

    set +x
    echo "${APISERVER_PUBLIC_KEY}" | base64 --decode > "${APISERVER_PUBLIC_KEY_PATH}"
    
    SERVICE_PRINCIPAL_CLIENT_SECRET="$(cat "$SP_FILE")"
    SERVICE_PRINCIPAL_CLIENT_SECRET=${SERVICE_PRINCIPAL_CLIENT_SECRET//\\/\\\\}
    SERVICE_PRINCIPAL_CLIENT_SECRET=${SERVICE_PRINCIPAL_CLIENT_SECRET//\"/\\\"}
    rm "$SP_FILE" # unneeded after reading from disk.
    cat << EOF > "${AZURE_JSON_PATH}"
{
    "cloud": "AzurePublicCloud",
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
}

configureCNI() {
    
    retrycmd_if_failure 120 5 25 modprobe br_netfilter || exit $ERR_MODPROBE_FAIL
    echo -n "br_netfilter" > /etc/modules-load.d/br_netfilter.conf
    configureCNIIPTables
    configureCNINFTables
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

configureCNINFTables() {
    if [[ "${IS_IPV6}" = "true" ]]; then
        # Delete the table in a subshell so that we can eat the failed return code
        (nft -na -- list table ip6 azureSLBProbe >/dev/null 2>&1 && nft -- delete table ip6 azureSLBProbe; exit 0)

        # Create a custom table
        nft -- add table ip6 azureSLBProbe

        # Map packets from the LB probe LLA to a SLA IP instead
        nft -- add chain ip6 azureSLBProbe prerouting {type filter hook prerouting priority -300\;}
        nft -- add rule ip6 azureSLBProbe prerouting iifname eth0 ip6 saddr fe80::1234:5678:9abc ip6 saddr set 2603:1062:0:1:fe80:1234:5678:9abc counter

        # Reverse the modification on the way back out
        nft -- add chain ip6 azureSLBProbe postrouting {type filter hook postrouting priority -300\;}
        nft -- add rule ip6 azureSLBProbe postrouting oifname eth0 ip6 daddr 2603:1062:0:1:fe80:1234:5678:9abc ip6 daddr set fe80::1234:5678:9abc counter

        # Display the rules
        nft -na -- list table ip6 azureSLBProbe
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
ensureDocker() {
    DOCKER_SERVICE_EXEC_START_FILE=/etc/systemd/system/docker.service.d/exec_start.conf
    wait_for_file 1200 1 $DOCKER_SERVICE_EXEC_START_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    usermod -aG docker ${ADMINUSER}
    DOCKER_MOUNT_FLAGS_SYSTEMD_FILE=/etc/systemd/system/docker.service.d/clear_mount_propagation_flags.conf
    wait_for_file 1200 1 $DOCKER_MOUNT_FLAGS_SYSTEMD_FILE || exit $ERR_FILE_WATCH_TIMEOUT
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
ensureMonitorService() {
    
    DOCKER_MONITOR_SYSTEMD_TIMER_FILE=/etc/systemd/system/docker-monitor.timer
    wait_for_file 1200 1 $DOCKER_MONITOR_SYSTEMD_TIMER_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    DOCKER_MONITOR_SYSTEMD_FILE=/etc/systemd/system/docker-monitor.service
    wait_for_file 1200 1 $DOCKER_MONITOR_SYSTEMD_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    systemctlEnableAndStart docker-monitor.timer || exit $ERR_SYSTEMCTL_START_FAIL
}



ensureKubelet() {
    KUBELET_DEFAULT_FILE=/etc/default/kubelet
    wait_for_file 1200 1 $KUBELET_DEFAULT_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    KUBECONFIG_FILE=/var/lib/kubelet/kubeconfig
    wait_for_file 1200 1 $KUBECONFIG_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    KUBELET_RUNTIME_CONFIG_SCRIPT_FILE=/opt/azure/containers/kubelet.sh
    wait_for_file 1200 1 $KUBELET_RUNTIME_CONFIG_SCRIPT_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    systemctlEnableAndStart kubelet || exit $ERR_KUBELET_START_FAIL
    
    
}

ensureMigPartition(){
    systemctlEnableAndStart mig-partition || exit $ERR_SYSTEMCTL_START_FAIL
}

ensureSysctl() {
    SYSCTL_CONFIG_FILE=/etc/sysctl.d/999-sysctl-aks.conf
    wait_for_file 1200 1 $SYSCTL_CONFIG_FILE || exit $ERR_FILE_WATCH_TIMEOUT
    retrycmd_if_failure 24 5 25 sysctl --system
}

ensureJournal() {
    {
        echo "Storage=persistent"
        echo "SystemMaxUse=1G"
        echo "RuntimeMaxUse=1G"
        echo "ForwardToSyslog=yes"
    } >> /etc/systemd/journald.conf
    systemctlEnableAndStart systemd-journald || exit $ERR_SYSTEMCTL_START_FAIL
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
    wait_for_file 1200 1 $CLUSTER_AUTOSCALER_ADDON_FILE || exit $ERR_FILE_WATCH_TIMEOUT
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
    wait_for_file 1200 1 $ACI_CONNECTOR_ADDON_FILE || exit $ERR_FILE_WATCH_TIMEOUT
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
    # install gpu driver
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        mkdir -p /opt/{actions,gpu}
        if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
            ctr image pull $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG
            bash -c "$CTR_GPU_INSTALL_CMD $NVIDIA_DRIVER_IMAGE:$NVIDIA_DRIVER_IMAGE_TAG gpuinstall /entrypoint.sh install" 
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
    else 
        echo "os $OS not supported at this time. skipping configGPUDrivers"
        exit 1
    fi

    # validate on host, already done inside container.
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        retrycmd_if_failure 120 5 25 nvidia-modprobe -u -c0 || exit $ERR_GPU_DRIVERS_START_FAIL
    fi
    retrycmd_if_failure 120 5 25 nvidia-smi || exit $ERR_GPU_DRIVERS_START_FAIL
    retrycmd_if_failure 120 5 25 ldconfig || exit $ERR_GPU_DRIVERS_START_FAIL
    
    # reload containerd/dockerd
    if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
        retrycmd_if_failure 120 5 25 pkill -SIGHUP containerd || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    else
        retrycmd_if_failure 120 5 25 pkill -SIGHUP dockerd || exit $ERR_GPU_DRIVERS_INSTALL_TIMEOUT
    fi
}

validateGPUDrivers() {
    if [[ $(isARM64) == 1 ]]; then
        # no GPU on ARM64
        return
    fi

    retrycmd_if_failure 24 5 25 nvidia-modprobe -u -c0 && echo "gpu driver loaded" || configGPUDrivers || exit $ERR_GPU_DRIVERS_START_FAIL
    which nvidia-smi
    if [[ $? == 0 ]]; then
        SMI_RESULT=$(retrycmd_if_failure 24 5 25 nvidia-smi)
    else
        SMI_RESULT=$(retrycmd_if_failure 24 5 25 $GPU_DEST/bin/nvidia-smi)
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
        # no GPU on ARM64
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

#EOF
