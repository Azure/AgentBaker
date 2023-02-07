#!/bin/bash
# Timeout waiting for a file
ERR_FILE_WATCH_TIMEOUT=6 
set -x
if [ -f /opt/azure/containers/provision.complete ]; then
      echo "Already ran to success exiting..."
      exit 0
fi

# Setup logs for upload to host
LOG_DIR=/var/log/azure/aks
mkdir -p ${LOG_DIR}
ln -s /var/log/azure/cluster-provision.log \
      /var/log/azure/cluster-provision-cse-output.log \
      /opt/azure/*.json \
      /opt/azure/cloud-init-files.paved \
      /opt/azure/vhd-install.complete \
      ${LOG_DIR}/

# Redact the necessary secrets from cloud-config.txt so we don't expose any sensitive information
# when cloud-config.txt gets included within log bundles
python3 /opt/azure/containers/provision_redact_cloud_config.py \
    --cloud-config-path /var/lib/cloud/instance/cloud-config.txt \
    --output-path ${LOG_DIR}/cloud-config.txt

UBUNTU_RELEASE=$(lsb_release -r -s)
if [[ ${UBUNTU_RELEASE} == "16.04" ]]; then
    sudo apt-get -y autoremove chrony
    echo $?
    sudo systemctl restart systemd-timesyncd
fi

echo $(date),$(hostname), startcustomscript>>/opt/m

for i in $(seq 1 3600); do
    if [ -s "${CSE_HELPERS_FILEPATH}" ]; then
        grep -Fq '#HELPERSEOF' "${CSE_HELPERS_FILEPATH}" && break
    fi
    if [ $i -eq 3600 ]; then
        exit $ERR_FILE_WATCH_TIMEOUT
    else
        sleep 1
    fi
done
sed -i "/#HELPERSEOF/d" "${CSE_HELPERS_FILEPATH}"
source "${CSE_HELPERS_FILEPATH}"

wait_for_file 3600 1 "${CSE_DISTRO_HELPERS_FILEPATH}" || exit $ERR_FILE_WATCH_TIMEOUT
source "${CSE_DISTRO_HELPERS_FILEPATH}"

wait_for_file 3600 1 "${CSE_INSTALL_FILEPATH}" || exit $ERR_FILE_WATCH_TIMEOUT
source "${CSE_INSTALL_FILEPATH}"

wait_for_file 3600 1 "${CSE_DISTRO_INSTALL_FILEPATH}" || exit $ERR_FILE_WATCH_TIMEOUT
source "${CSE_DISTRO_INSTALL_FILEPATH}"

wait_for_file 3600 1 "${CSE_CONFIG_FILEPATH}" || exit $ERR_FILE_WATCH_TIMEOUT
source "${CSE_CONFIG_FILEPATH}"

if [[ "${DISABLE_SSH}" == "true" ]]; then
    disableSSH || exit $ERR_DISABLE_SSH
fi

if [[ "${SHOULD_CONFIGURE_HTTP_PROXY_CA}" == "true" ]]; then
    configureHTTPProxyCA || exit $ERR_UPDATE_CA_CERTS
    configureEtcEnvironment
fi

if [[ "${SHOULD_CONFIGURE_CUSTOM_CA_TRUST}" == "true" ]]; then
    configureCustomCaCertificate || $ERR_UPDATE_CA_CERTS
fi

export NO_PROXY="localhost,127.0.0.1"; export HTTPS_PROXY="https://myproxy.server.com:8080/"; export http_proxy="http://myproxy.server.com:8080/"; retrycmd_if_failure() { r=$1; w=$2; t=$3; shift && shift && shift; for i in $(seq 1 $r); do timeout $t ${@}; [ $? -eq 0  ] && break || if [ $i -eq $r ]; then return 1; else sleep $w; fi; done }; ERR_OUTBOUND_CONN_FAIL=50; retrycmd_if_failure 50 1 5 curl -v --insecure --proxy-insecure https://mcr.microsoft.com/v2/ >> /var/log/azure/cluster-provision-cse-output.log 2>&1 || time curl -v --insecure --proxy-insecure https://mcr.microsoft.com/v2/ || exit $ERR_OUTBOUND_CONN_FAIL;

# Bring in OS-related vars
source /etc/os-release

# Mandb is not currently available on MarinerV1
if [[ ${ID} != "mariner" ]]; then
    echo "Removing man-db auto-update flag file..."
    logs_to_events "AKS.CSE.removeManDbAutoUpdateFlagFile" removeManDbAutoUpdateFlagFile
fi

if [[ "${GPU_NODE}" != "true" ]]; then
    logs_to_events "AKS.CSE.cleanUpGPUDrivers" cleanUpGPUDrivers
fi

logs_to_events "AKS.CSE.disableSystemdResolved" disableSystemdResolved

logs_to_events "AKS.CSE.configureAdminUser" configureAdminUser

VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
if [ -f $VHD_LOGS_FILEPATH ]; then
    echo "detected golden image pre-install"
    logs_to_events "AKS.CSE.cleanUpContainerImages" cleanUpContainerImages
    FULL_INSTALL_REQUIRED=false
else
    if [[ "${IS_VHD}" = true ]]; then
        echo "Using VHD distro but file $VHD_LOGS_FILEPATH not found"
        exit $ERR_VHD_FILE_NOT_FOUND
    fi
    FULL_INSTALL_REQUIRED=true
fi

if [[ $OS == $UBUNTU_OS_NAME ]] && [ "$FULL_INSTALL_REQUIRED" = "true" ]; then
    logs_to_events "AKS.CSE.installDeps" installDeps
else
    echo "Golden image; skipping dependencies installation"
fi

logs_to_events "AKS.CSE.installContainerRuntime" installContainerRuntime
if [ "${NEEDS_CONTAINERD}" == "true" ] && [ "${TELEPORT_ENABLED}" == "true" ]; then 
    logs_to_events "AKS.CSE.installTeleportdPlugin" installTeleportdPlugin
fi

setupCNIDirs

logs_to_events "AKS.CSE.installNetworkPlugin" installNetworkPlugin

if [ "${IS_KRUSTLET}" == "true" ]; then
    logs_to_events "AKS.CSE.downloadKrustlet" downloadContainerdWasmShims
fi

# By default, never reboot new nodes.
REBOOTREQUIRED=false

echo $(date),$(hostname), "Start configuring GPU drivers"
if [[ "${GPU_NODE}" = true ]]; then
    logs_to_events "AKS.CSE.ensureGPUDrivers" ensureGPUDrivers
    if [[ "${ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED}" = true ]]; then
        if [[ "${MIG_NODE}" == "true" ]] && [[ -f "/etc/systemd/system/nvidia-device-plugin.service" ]]; then
            logs_to_events "AKS.CSE.mig_strategy" "wait_for_file 3600 1 /etc/systemd/system/nvidia-device-plugin.service.d/10-mig_strategy.conf" || exit $ERR_FILE_WATCH_TIMEOUT
        fi
        logs_to_events "AKS.CSE.start.nvidia-device-plugin" "systemctlEnableAndStart nvidia-device-plugin" || exit $ERR_GPU_DEVICE_PLUGIN_START_FAIL
    else
        logs_to_events "AKS.CSE.stop.nvidia-device-plugin" "systemctlDisableAndStop nvidia-device-plugin"
    fi
fi

if [[ "${GPU_NEEDS_FABRIC_MANAGER}" == "true" ]]; then
    # fabric manager trains nvlink connections between multi instance gpus.
    # it appears this is only necessary for systems with *multiple cards*.
    # i.e., an A100 can be partitioned a maximum of 7 ways.
    # An NC24ads_A100_v4 has one A100.
    # An ND96asr_v4 has eight A100, for a maximum of 56 partitions.
    # ND96 seems to require fabric manager *even when not using mig partitions*
    # while it fails to install on NC24.
    logs_to_events "AKS.CSE.nvidia-fabricmanager" "systemctlEnableAndStart nvidia-fabricmanager" || exit $ERR_GPU_DRIVERS_START_FAIL
fi

# This will only be true for multi-instance capable VM sizes
# for which the user has specified a partitioning profile.
# it is valid to use mig-capable gpus without a partitioning profile.
if [[ "${MIG_NODE}" == "true" ]]; then
    # A100 GPU has a bit in the physical card (infoROM) to enable mig mode.
    # Changing this bit in either direction requires a VM reboot on Azure (hypervisor/plaform stuff).
    # Commands such as `nvidia-smi --gpu-reset` may succeed,
    # while commands such as `nvidia-smi -q` will show mismatched current/pending mig mode.
    # this will not be required per nvidia for next gen H100.
    REBOOTREQUIRED=true
    
    # this service applies the partitioning scheme with nvidia-smi.
    # we should consider moving to mig-parted which is simpler/newer.
    # we couldn't because of old drivers but that has long been fixed.
    logs_to_events "AKS.CSE.ensureMigPartition" ensureMigPartition
fi

echo $(date),$(hostname), "End configuring GPU drivers"

if [ "${NEEDS_DOCKER_LOGIN}" == "true" ]; then
    set +x
    docker login -u $SERVICE_PRINCIPAL_CLIENT_ID -p $SERVICE_PRINCIPAL_CLIENT_SECRET "${AZURE_PRIVATE_REGISTRY_SERVER}"
    set -x
fi

logs_to_events "AKS.CSE.installKubeletKubectlAndKubeProxy" installKubeletKubectlAndKubeProxy

createKubeManifestDir

if [ "${HAS_CUSTOM_SEARCH_DOMAIN}" == "true" ]; then
    wait_for_file 3600 1 "${CUSTOM_SEARCH_DOMAIN_FILEPATH}" || exit $ERR_FILE_WATCH_TIMEOUT
    "${CUSTOM_SEARCH_DOMAIN_FILEPATH}" > /opt/azure/containers/setup-custom-search-domain.log 2>&1 || exit $ERR_CUSTOM_SEARCH_DOMAINS_FAIL
fi

logs_to_events "AKS.CSE.configureK8s" configureK8s

logs_to_events "AKS.CSE.configureCNI" configureCNI

# configure and enable dhcpv6 for dual stack feature
if [ "${IPV6_DUAL_STACK_ENABLED}" == "true" ]; then
    logs_to_events "AKS.CSE.ensureDHCPv6" ensureDHCPv6
fi

if [ "${NEEDS_CONTAINERD}" == "true" ]; then
    # containerd should not be configured until cni has been configured first
    logs_to_events "AKS.CSE.ensureContainerd" ensureContainerd 
    logs_to_events "AKS.CSE.ensureContainerdMonitorService" ensureContainerdMonitorService
else
    logs_to_events "AKS.CSE.ensureDocker" ensureDocker
    logs_to_events "AKS.CSE.ensureDockerMonitorService" ensureDockerMonitorService
fi

# Start the service to synchronize tunnel logs so WALinuxAgent can pick them up
logs_to_events "AKS.CSE.sync-tunnel-logs" "systemctlEnableAndStart sync-tunnel-logs"

# must run before kubelet starts to avoid race in container status using wrong image
# https://github.com/kubernetes/kubernetes/issues/51017
# can remove when fixed
if [[ "${TARGET_CLOUD}" == "AzureChinaCloud" ]]; then
    retagMCRImagesForChina
fi

if [[ "${ENABLE_HOSTS_CONFIG_AGENT}" == "true" ]]; then
    logs_to_events "AKS.CSE.configPrivateClusterHosts" configPrivateClusterHosts
fi

if [ "${SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE}" == "true" ]; then
    logs_to_events "AKS.CSE.configureTransparentHugePage" configureTransparentHugePage
fi

if [ "${SHOULD_CONFIG_SWAP_FILE}" == "true" ]; then
    logs_to_events "AKS.CSE.configureSwapFile" configureSwapFile
fi

logs_to_events "AKS.CSE.ensureSysctl" ensureSysctl

logs_to_events "AKS.CSE.ensureKubelet" ensureKubelet
if [ "${ENSURE_NO_DUPE_PROMISCUOUS_BRIDGE}" == "true" ]; then
    logs_to_events "AKS.CSE.ensureNoDupOnPromiscuBridge" ensureNoDupOnPromiscuBridge
fi

if $FULL_INSTALL_REQUIRED; then
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        # mitigation for bug https://bugs.launchpad.net/ubuntu/+source/linux/+bug/1676635 
        echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind
        sed -i "13i\echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind\n" /etc/rc.local
    fi
fi

VALIDATION_ERR=0

# Edge case scenarios:
# high retry times to wait for new API server DNS record to replicate (e.g. stop and start cluster)
# high timeout to address high latency for private dns server to forward request to Azure DNS
# dns check will be done only if we use FQDN for API_SERVER_NAME
API_SERVER_CONN_RETRIES=50
if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
    API_SERVER_CONN_RETRIES=100
fi
if ! [[ ${API_SERVER_NAME} =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    API_SERVER_DNS_RETRIES=100
    if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
       API_SERVER_DNS_RETRIES=200
    fi
    if [[ "${ENABLE_HOSTS_CONFIG_AGENT}" != "true" ]]; then
        RES=$(retrycmd_if_failure ${API_SERVER_DNS_RETRIES} 1 10 nslookup ${API_SERVER_NAME})
        STS=$?
    else
        STS=0
    fi
    if [[ $STS != 0 ]]; then
        time nslookup ${API_SERVER_NAME}
        if [[ $RES == *"168.63.129.16"*  ]]; then
            VALIDATION_ERR=$ERR_K8S_API_SERVER_AZURE_DNS_LOOKUP_FAIL
        else
            VALIDATION_ERR=$ERR_K8S_API_SERVER_DNS_LOOKUP_FAIL
        fi
    else
        logs_to_events "AKS.CSE.apiserverNC" "retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 10 nc -vz ${API_SERVER_NAME} 443" || time nc -vz ${API_SERVER_NAME} 443 || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
    fi
else
    logs_to_events "AKS.CSE.apiserverNC" "retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 10 nc -vz ${API_SERVER_NAME} 443" || time nc -vz ${API_SERVER_NAME} 443 || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
fi

if [[ ${ID} != "mariner" ]]; then
    echo "Recreating man-db auto-update flag file and kicking off man-db update process at $(date)"
    createManDbAutoUpdateFlagFile
    /usr/bin/mandb && echo "man-db finished updates at $(date)" &
fi

if $REBOOTREQUIRED; then
    echo 'reboot required, rebooting node in 1 minute'
    /bin/bash -c "shutdown -r 1 &"
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        # logs_to_events should not be run on & commands
        aptmarkWALinuxAgent unhold &
    fi
else
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        # logs_to_events should not be run on & commands
        if [ "${ENABLE_UNATTENDED_UPGRADES}" == "true" ]; then
            systemctl unmask apt-daily.service apt-daily-upgrade.service
            systemctl enable apt-daily.service apt-daily-upgrade.service
            systemctl enable apt-daily.timer apt-daily-upgrade.timer
            systemctl restart --no-block apt-daily.timer apt-daily-upgrade.timer            
            # this is the DOWNLOAD service
            # meaning we are wasting IO without even triggering an upgrade 
            # -________________-
            systemctl restart --no-block apt-daily.service
        fi
        aptmarkWALinuxAgent unhold &
    fi
fi

echo "Custom script finished. API server connection check code:" $VALIDATION_ERR
echo $(date),$(hostname), endcustomscript>>/opt/m
mkdir -p /opt/azure/containers && touch /opt/azure/containers/provision.complete

exit $VALIDATION_ERR

#EOF
