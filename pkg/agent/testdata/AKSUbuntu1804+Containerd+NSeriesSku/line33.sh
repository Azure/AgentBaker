#!/bin/bash
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
    if [ -s /opt/azure/containers/provision_source.sh ]; then
        grep -Fq '#HELPERSEOF' /opt/azure/containers/provision_source.sh && break
    fi
    if [ $i -eq 3600 ]; then
        exit $ERR_FILE_WATCH_TIMEOUT
    else
        sleep 1
    fi
done
sed -i "/#HELPERSEOF/d" /opt/azure/containers/provision_source.sh
source /opt/azure/containers/provision_source.sh

wait_for_file 3600 1 /opt/azure/containers/provision_source_distro.sh || exit $ERR_FILE_WATCH_TIMEOUT
source /opt/azure/containers/provision_source_distro.sh

wait_for_file 3600 1 /opt/azure/containers/provision_installs.sh || exit $ERR_FILE_WATCH_TIMEOUT
source /opt/azure/containers/provision_installs.sh

wait_for_file 3600 1 /opt/azure/containers/provision_installs_distro.sh || exit $ERR_FILE_WATCH_TIMEOUT
source /opt/azure/containers/provision_installs_distro.sh

wait_for_file 3600 1 /opt/azure/containers/provision_configs.sh || exit $ERR_FILE_WATCH_TIMEOUT
source /opt/azure/containers/provision_configs.sh

retrycmd_if_failure() { r=$1; w=$2; t=$3; shift && shift && shift; for i in $(seq 1 $r); do timeout $t ${@}; [ $? -eq 0  ] && break || if [ $i -eq $r ]; then return 1; else sleep $w; fi; done }; ERR_OUTBOUND_CONN_FAIL=50; retrycmd_if_failure 100 1 10 nc -vz mcr.microsoft.com 443 >> /var/log/azure/cluster-provision-cse-output.log 2>&1 || time nc -vz mcr.microsoft.com 443 || exit $ERR_OUTBOUND_CONN_FAIL;

# Bring in OS-related vars
source /etc/os-release

# Mandb is not currently available on MarinerV1
if [[ ${ID} != "mariner" ]]; then
    echo "Removing man-db auto-update flag file..."
    logs_to_events "AKS.CSE.removeManDbAutoUpdateFlagFile" removeManDbAutoUpdateFlagFile
fi

if [[ "${GPU_NODE}" != "true" ]]; then
    cleanUpGPUDrivers
fi

logs_to_events "AKS.CSE.disableSystemdResolved" disableSystemdResolved

logs_to_events "AKS.CSE.configureAdminUser" configureAdminUser
# If crictl gets installed then use it as the cri cli instead of ctr
# crictl is not a critical component so continue with boostrapping if the install fails
# CLI_TOOL is by default set to "ctr"
logs_to_events "AKS.CSE.installCrictl" 'installCrictl && CLI_TOOL="crictl"'

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

setupCNIDirs

logs_to_events "AKS.CSE.installNetworkPlugin" installNetworkPlugin
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
# If it is a MIG Node, enable mig-partition systemd service to create MIG instances
if [[ "${MIG_NODE}" == "true" ]]; then
    REBOOTREQUIRED=true
    logs_to_events "AKS.CSE.nvidia-fabricmanager" "systemctlEnableAndStart nvidia-fabricmanager" || exit $ERR_GPU_DRIVERS_START_FAIL
    logs_to_events "AKS.CSE.ensureMigPartition" ensureMigPartition
fi

echo $(date),$(hostname), "End configuring GPU drivers"


logs_to_events "AKS.CSE.installKubeletKubectlAndKubeProxy" installKubeletKubectlAndKubeProxy

logs_to_events "AKS.CSE.ensureRPC" ensureRPC

createKubeManifestDir

logs_to_events "AKS.CSE.configureK8s" configureK8s

logs_to_events "AKS.CSE.configureCNI" configureCNI


logs_to_events "AKS.CSE.ensureContainerd" ensureContainerd 

# Start the service to synchronize tunnel logs so WALinuxAgent can pick them up
logs_to_events "AKS.CSE.sync-tunnel-logs" "systemctlEnableAndStart sync-tunnel-logs"

logs_to_events "AKS.CSE.ensureMonitorService" ensureMonitorService
# must run before kubelet starts to avoid race in container status using wrong image
# https://github.com/kubernetes/kubernetes/issues/51017
# can remove when fixed
if [[ "AzurePublicCloud" == "AzureChinaCloud" ]]; then
    retagMCRImagesForChina
fi

logs_to_events "AKS.CSE.ensureSysctl" ensureSysctl
logs_to_events "AKS.CSE.ensureJournal" ensureJournal

logs_to_events "AKS.CSE.ensureKubelet" ensureKubelet

if $FULL_INSTALL_REQUIRED; then
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        
        echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind
        sed -i "13i\echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind\n" /etc/rc.local
    fi
fi

if [[ $OS == $UBUNTU_OS_NAME ]]; then
    # logs_to_events should not be run on & commands
    apt_get_purge 20 30 120 apache2-utils &
fi

VALIDATION_ERR=0
API_SERVER_CONN_RETRIES=50
if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
    API_SERVER_CONN_RETRIES=100
fi
if ! [[ ${API_SERVER_NAME} =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    API_SERVER_DNS_RETRIES=100
    if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
       API_SERVER_DNS_RETRIES=200
    fi
    RES=$(retrycmd_if_failure ${API_SERVER_DNS_RETRIES} 1 10 nslookup ${API_SERVER_NAME})
    STS=$?
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

# handle starting the logrotate-aks service responsible for log rotation
if [ -f /etc/systemd/system/logrotate-aks.timer ] && [ -f /etc/systemd/system/logrotate-aks.service ]; then
    # we're on a VHD that has definitions for the logrotate-aks service
    logs_to_events "AKS.CSE.logrotate-aks" "systemctlEnableAndStart logrotate-aks.timer"
    if [ -f /etc/cron.daily/logrotate ]; then
        rm -f /etc/cron.daily/logrotate
        echo "Removed /etc/cron.daily/logrotate"
    fi
    echo "Enabled and started logrotate-aks.timer, removed"
else
    # we're not on a VHD with the logrotate-aks service definition, 
    # fall back to using the default daily logrotate cron
    echo "Unable to start logrotate-aks.timer, falling back to default logrotate cron"
fi

# Ace: Basically the hypervisor blocks gpu reset which is required after enabling mig mode for the gpus to be usable
REBOOTREQUIRED=false
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
        systemctl unmask apt-daily.service apt-daily-upgrade.service
        systemctl enable apt-daily.service apt-daily-upgrade.service
        systemctl enable apt-daily.timer apt-daily-upgrade.timer
        # this is the DOWNLOAD service
        # meaning we are wasting IO without even triggering an upgrade 
        # -________________-
        systemctl restart apt-daily.service
        aptmarkWALinuxAgent unhold &
    fi
fi

echo "Custom script finished. API server connection check code:" $VALIDATION_ERR
echo $(date),$(hostname), endcustomscript>>/opt/m
mkdir -p /opt/azure/containers && touch /opt/azure/containers/provision.complete

exit $VALIDATION_ERR

#EOF
