#!/bin/bash
# Timeout waiting for a file
ERR_FILE_WATCH_TIMEOUT=6 
set -x
if [ -f /opt/azure/containers/provision.complete ]; then
      echo "Already ran to success exiting..."
      exit 0
fi

aptmarkWALinuxAgent hold &

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

source "${CSE_DISTRO_HELPERS_FILEPATH}"
source "${CSE_INSTALL_FILEPATH}"
source "${CSE_DISTRO_INSTALL_FILEPATH}"
source "${CSE_CONFIG_FILEPATH}"

if [[ "${DISABLE_SSH}" == "true" ]]; then
    disableSSH || exit $ERR_DISABLE_SSH
fi

# This involves using proxy, log the config before fetching packages
echo "private egress proxy address is '${PRIVATE_EGRESS_PROXY_ADDRESS}'"
# TODO update to use proxy

if [[ "${SHOULD_CONFIGURE_HTTP_PROXY}" == "true" ]]; then
    if [[ "${SHOULD_CONFIGURE_HTTP_PROXY_CA}" == "true" ]]; then
        configureHTTPProxyCA || exit $ERR_UPDATE_CA_CERTS
    fi
    configureEtcEnvironment
fi


if [[ "${SHOULD_CONFIGURE_CUSTOM_CA_TRUST}" == "true" ]]; then
    configureCustomCaCertificate || exit $ERR_UPDATE_CA_CERTS
fi

if [[ -n "${OUTBOUND_COMMAND}" ]]; then
    if [[ -n "${PROXY_VARS}" ]]; then
        eval $PROXY_VARS
    fi
    retrycmd_if_failure 50 1 5 $OUTBOUND_COMMAND >> /var/log/azure/cluster-provision-cse-output.log 2>&1 || exit $ERR_OUTBOUND_CONN_FAIL;
fi

logs_to_events "AKS.CSE.setCPUArch" setCPUArch
source /etc/os-release

if [[ ${ID} != "mariner" ]] && [[ ${ID} != "azurelinux" ]]; then
    echo "Removing man-db auto-update flag file..."
    logs_to_events "AKS.CSE.removeManDbAutoUpdateFlagFile" removeManDbAutoUpdateFlagFile
fi

export -f should_skip_nvidia_drivers
skip_nvidia_driver_install=$(retrycmd_if_failure_no_stats 10 1 10 bash -cx should_skip_nvidia_drivers)
ret=$?
if [[ "$ret" != "0" ]]; then
    echo "Failed to determine if nvidia driver install should be skipped"
    exit $ERR_NVIDIA_DRIVER_INSTALL
fi

if [[ "${GPU_NODE}" != "true" ]] || [[ "${skip_nvidia_driver_install}" == "true" ]]; then
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
    local versionsWasm=$(jq -r '.Packages[] | select(.name == "containerd-wasm-shims") | .downloadURIs.default.current.versionsV2[].latestVersion' "$COMPONENTS_FILEPATH")
    local downloadLocationWasm=$(jq -r '.Packages[] | select(.name == "containerd-wasm-shims") | .downloadLocation' "$COMPONENTS_FILEPATH")
    local downloadURLWasm=$(jq -r '.Packages[] | select(.name == "containerd-wasm-shims") | .downloadURIs.default.current.downloadURL' "$COMPONENTS_FILEPATH")
    logs_to_events "AKS.CSE.installContainerdWasmShims" installContainerdWasmShims "$downloadLocationWasm" "$downloadURLWasm" "$versionsWasm"

    local versionsSpinKube=$(jq -r '.Packages[] | select(.name == spinkube") | .downloadURIs.default.current.versionsV2[].latestVersion' "$COMPONENTS_FILEPATH")
    local downloadLocationSpinKube=$(jq -r '.Packages[] | select(.name == "spinkube) | .downloadLocation' "$COMPONENTS_FILEPATH")
    local downloadURLSpinKube=$(jq -r '.Packages[] | select(.name == "spinkube") | .downloadURIs.default.current.downloadURL' "$COMPONENTS_FILEPATH")
    logs_to_events "AKS.CSE.installSpinKube" installSpinKube "$downloadURSpinKube" "$downloadLocationSpinKube" "$versionsSpinKube"
fi

if [ "${ENABLE_SECURE_TLS_BOOTSTRAPPING}" == "true" ]; then
    logs_to_events "AKS.CSE.downloadSecureTLSBootstrapKubeletExecPlugin" downloadSecureTLSBootstrapKubeletExecPlugin
fi

# By default, never reboot new nodes.
REBOOTREQUIRED=false

echo $(date),$(hostname), "Start configuring GPU drivers"
if [[ "${GPU_NODE}" = true ]] && [[ "${skip_nvidia_driver_install}" != "true" ]]; then
    logs_to_events "AKS.CSE.ensureGPUDrivers" ensureGPUDrivers
    if [[ "${ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED}" = true ]]; then
        if [[ "${MIG_NODE}" == "true" ]] && [[ -f "/etc/systemd/system/nvidia-device-plugin.service" ]]; then
            mkdir -p "/etc/systemd/system/nvidia-device-plugin.service.d"
            tee "/etc/systemd/system/nvidia-device-plugin.service.d/10-mig_strategy.conf" > /dev/null <<'EOF'
[Service]
Environment="MIG_STRATEGY=--mig-strategy single"
ExecStart=
ExecStart=/usr/local/nvidia/bin/nvidia-device-plugin $MIG_STRATEGY    
EOF
        fi
        logs_to_events "AKS.CSE.start.nvidia-device-plugin" "systemctlEnableAndStart nvidia-device-plugin" || exit $ERR_GPU_DEVICE_PLUGIN_START_FAIL
    else
        logs_to_events "AKS.CSE.stop.nvidia-device-plugin" "systemctlDisableAndStop nvidia-device-plugin"
    fi

    if [[ "${GPU_NEEDS_FABRIC_MANAGER}" == "true" ]]; then
        # fabric manager trains nvlink connections between multi instance gpus.
        # it appears this is only necessary for systems with *multiple cards*.
        # i.e., an A100 can be partitioned a maximum of 7 ways.
        # An NC24ads_A100_v4 has one A100.
        # An ND96asr_v4 has eight A100, for a maximum of 56 partitions.
        # ND96 seems to require fabric manager *even when not using mig partitions*
        # while it fails to install on NC24.
        if isMarinerOrAzureLinux "$OS"; then
            logs_to_events "AKS.CSE.installNvidiaFabricManager" installNvidiaFabricManager
        fi
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
    "${CUSTOM_SEARCH_DOMAIN_FILEPATH}" > /opt/azure/containers/setup-custom-search-domain.log 2>&1 || exit $ERR_CUSTOM_SEARCH_DOMAINS_FAIL
fi


# for drop ins, so they don't all have to check/create the dir
mkdir -p "/etc/systemd/system/kubelet.service.d"

# we do this here since this function has the potential to mutate kubelet flags, 
# kubelet config file, and node labels if a special tag has been added to the underlying VM. 
# kubelet config file content is decoded and written to disk by configureK8s, thus we need to make sure the content is correct beforehand.
logs_to_events "AKS.CSE.disableKubeletServingCertificateRotationForTags" disableKubeletServingCertificateRotationForTags

logs_to_events "AKS.CSE.configureK8s" configureK8s

logs_to_events "AKS.CSE.configureCNI" configureCNI

# configure and enable dhcpv6 for dual stack feature
if [ "${IPV6_DUAL_STACK_ENABLED}" == "true" ]; then
    logs_to_events "AKS.CSE.ensureDHCPv6" ensureDHCPv6
fi

# For systemd in Azure Linux, UseDomains= is by default disabled for security purposes. Enable this
# configuration within Azure Linux AKS that operates on trusted networks to support hostname resolution
if isMarinerOrAzureLinux "$OS"; then
    logs_to_events "AKS.CSE.configureSystemdUseDomains" configureSystemdUseDomains
fi

if [ "${NEEDS_CONTAINERD}" == "true" ]; then
    # containerd should not be configured until cni has been configured first
    logs_to_events "AKS.CSE.ensureContainerd" ensureContainerd 
else
    logs_to_events "AKS.CSE.ensureDocker" ensureDocker
fi

if [[ "${MESSAGE_OF_THE_DAY}" != "" ]]; then
    echo "${MESSAGE_OF_THE_DAY}" | base64 -d > /etc/motd
fi

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

if [ "${NEEDS_CGROUPV2}" == "true" ]; then
    tee "/etc/systemd/system/kubelet.service.d/10-cgroupv2.conf" > /dev/null <<EOF
[Service]
Environment="KUBELET_CGROUP_FLAGS=--cgroup-driver=systemd"
EOF
fi

if [ "${NEEDS_CONTAINERD}" == "true" ]; then
    # gross, but the backticks make it very hard to do in Go
    # TODO: move entirely into vhd.
    # alternatively, can we verify this is safe with docker?
    # or just do it even if not because docker is out of support?
    mkdir -p /etc/containerd
    echo "${KUBENET_TEMPLATE}" | base64 -d > /etc/containerd/kubenet_template.conf

    # In k8s 1.27, the flag --container-runtime was removed.
    # We now have 2 drop-in's, one with the still valid flags that will be applied to all k8s versions,
    # the flags are --runtime-request-timeout, --container-runtime-endpoint, --runtime-cgroups
    # For k8s >= 1.27, the flag --container-runtime will not be passed.
    tee "/etc/systemd/system/kubelet.service.d/10-containerd-base-flag.conf" > /dev/null <<'EOF'
[Service]
Environment="KUBELET_CONTAINERD_FLAGS=--runtime-request-timeout=15m --container-runtime-endpoint=unix:///run/containerd/containerd.sock --runtime-cgroups=/system.slice/containerd.service"
EOF
    
    if ! semverCompare ${KUBERNETES_VERSION:-"0.0.0"} "1.27.0"; then
        tee "/etc/systemd/system/kubelet.service.d/10-container-runtime-flag.conf" > /dev/null <<'EOF'
[Service]
Environment="KUBELET_CONTAINER_RUNTIME_FLAG=--container-runtime=remote"
EOF
    fi
fi

if [ "${HAS_KUBELET_DISK_TYPE}" == "true" ]; then
    tee "/etc/systemd/system/kubelet.service.d/10-bindmount.conf" > /dev/null <<EOF
[Unit]
Requires=bind-mount.service
After=bind-mount.service
EOF
fi

logs_to_events "AKS.CSE.ensureSysctl" ensureSysctl

if [ "${NEEDS_CONTAINERD}" == "true" ] &&  [ "${SHOULD_CONFIG_CONTAINERD_ULIMITS}" == "true" ]; then
  logs_to_events "AKS.CSE.setContainerdUlimits" configureContainerdUlimits
fi

logs_to_events "AKS.CSE.ensureKubelet" ensureKubelet
if [ "${ENSURE_NO_DUPE_PROMISCUOUS_BRIDGE}" == "true" ]; then
    logs_to_events "AKS.CSE.ensureNoDupOnPromiscuBridge" ensureNoDupOnPromiscuBridge
fi

if [[ $OS == $UBUNTU_OS_NAME ]] || isMarinerOrAzureLinux "$OS"; then
    logs_to_events "AKS.CSE.ubuntuSnapshotUpdate" ensureSnapshotUpdate
fi

if $FULL_INSTALL_REQUIRED; then
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        # mitigation for bug https://bugs.launchpad.net/ubuntu/+source/linux/+bug/1676635 
        echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind
        sed -i "13i\echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind\n" /etc/rc.local
    fi
fi

VALIDATION_ERR=0

# TODO(djsly): Look at leveraging the `aks-check-network.sh` script for this validation instead of duplicating the logic here

# Edge case scenarios:
# high retry times to wait for new API server DNS record to replicate (e.g. stop and start cluster)
# high timeout to address high latency for private dns server to forward request to Azure DNS
# dns check will be done only if we use FQDN for API_SERVER_NAME
API_SERVER_CONN_RETRIES=50
if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
    API_SERVER_CONN_RETRIES=100
fi
if ! [[ ${API_SERVER_NAME} =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    API_SERVER_DNS_RETRY_TIMEOUT=300
    if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
       API_SERVER_DNS_RETRY_TIMEOUT=600
    fi
    if [[ "${ENABLE_HOSTS_CONFIG_AGENT}" != "true" ]]; then
        RES=$(logs_to_events "AKS.CSE.apiserverNslookup" "retrycmd_nslookup 1 15 ${API_SERVER_DNS_RETRY_TIMEOUT} ${API_SERVER_NAME}")
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
        if [ "${UBUNTU_RELEASE}" == "18.04" ]; then
            #TODO (djsly): remove this once 18.04 isn't supported anymore
            logs_to_events "AKS.CSE.apiserverNC" "retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 10 nc -vz ${API_SERVER_NAME} 443" || time nc -vz ${API_SERVER_NAME} 443 || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
        else
            logs_to_events "AKS.CSE.apiserverCurl" "retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 10 curl -v --cacert /etc/kubernetes/certs/ca.crt https://${API_SERVER_NAME}:443" || time curl -v --cacert /etc/kubernetes/certs/ca.crt "https://${API_SERVER_NAME}:443" || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
        fi
    fi
else
    # an IP address is provided for the API server, skip the DNS lookup
    if [ "${UBUNTU_RELEASE}" == "18.04" ]; then
        #TODO (djsly): remove this once 18.04 isn't supported anymore
        logs_to_events "AKS.CSE.apiserverNC" "retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 10 nc -vz ${API_SERVER_NAME} 443" || time nc -vz ${API_SERVER_NAME} 443 || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
    else
        logs_to_events "AKS.CSE.apiserverCurl" "retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 10 curl -v --cacert /etc/kubernetes/certs/ca.crt https://${API_SERVER_NAME}:443" || time curl -v --cacert /etc/kubernetes/certs/ca.crt "https://${API_SERVER_NAME}:443" || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
    fi
fi

if [[ ${ID} != "mariner" ]] && [[ ${ID} != "azurelinux" ]]; then
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
            UU_CONFIG_DIR="/etc/apt/apt.conf.d/99periodic"
            mkdir -p "$(dirname "${UU_CONFIG_DIR}")"
            touch "${UU_CONFIG_DIR}"
            chmod 0644 "${UU_CONFIG_DIR}"
            echo 'APT::Periodic::Update-Package-Lists "1";' >> "${UU_CONFIG_DIR}"
            echo 'APT::Periodic::Unattended-Upgrade "1";' >> "${UU_CONFIG_DIR}"
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
    elif isMarinerOrAzureLinux "$OS"; then
        if [ "${ENABLE_UNATTENDED_UPGRADES}" == "true" ]; then
            if [ "${IS_KATA}" == "true" ]; then
                # Currently kata packages must be updated as a unit (including the kernel which requires a reboot). This can
                # only be done reliably via image updates as of now so never enable automatic updates.
                echo 'EnableUnattendedUpgrade is not supported by kata images, will not be enabled'
            else
                # By default the dnf-automatic is service is notify only in Mariner.
                # Enable the automatic install timer and the check-restart timer.
                # Stop the notify only dnf timer since we've enabled the auto install one.
                # systemctlDisableAndStop adds .service to the end which doesn't work on timers.
                systemctl disable dnf-automatic-notifyonly.timer
                systemctl stop dnf-automatic-notifyonly.timer
                # At 6:00:00 UTC (1 hour random fuzz) download and install package updates.
                systemctl unmask dnf-automatic-install.service || exit $ERR_SYSTEMCTL_START_FAIL
                systemctl unmask dnf-automatic-install.timer || exit $ERR_SYSTEMCTL_START_FAIL
                systemctlEnableAndStart dnf-automatic-install.timer || exit $ERR_SYSTEMCTL_START_FAIL
                # The check-restart service which will inform kured of required restarts should already be running
            fi
        fi
    fi
fi

echo "Custom script finished. API server connection check code:" $VALIDATION_ERR
echo $(date),$(hostname), endcustomscript>>/opt/m
mkdir -p /opt/azure/containers && touch /opt/azure/containers/provision.complete

exit $VALIDATION_ERR


#EOF