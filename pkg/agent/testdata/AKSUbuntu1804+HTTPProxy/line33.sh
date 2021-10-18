#!/bin/bash
ERR_FILE_WATCH_TIMEOUT=6 
set -x
if [ -f /opt/azure/containers/provision.complete ]; then
      echo "Already ran to success exiting..."
      exit 0
fi

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
cleanUpContainerd

if [[ "${GPU_NODE}" != "true" ]]; then
    cleanUpGPUDrivers
fi
configureHTTPProxyCA

disable1804SystemdResolved

if [ -f /var/run/reboot-required ]; then
    REBOOTREQUIRED=true
else
    REBOOTREQUIRED=false
fi

configureAdminUser

VHD_LOGS_FILEPATH=/opt/azure/vhd-install.complete
if [ -f $VHD_LOGS_FILEPATH ]; then
    echo "detected golden image pre-install"
    cleanUpContainerImages
    FULL_INSTALL_REQUIRED=false
else
    if [[ "${IS_VHD}" = true ]]; then
        echo "Using VHD distro but file $VHD_LOGS_FILEPATH not found"
        exit $ERR_VHD_FILE_NOT_FOUND
    fi
    FULL_INSTALL_REQUIRED=true
fi

if [[ $OS == $UBUNTU_OS_NAME ]] && [ "$FULL_INSTALL_REQUIRED" = "true" ]; then
    installDeps
else
    echo "Golden image; skipping dependencies installation"
fi

installContainerRuntime
installNetworkPlugin


installKubeletKubectlAndKubeProxy

ensureRPC

createKubeManifestDir

configureK8s
configureCNI



ensureDocker

ensureMonitorService

ensureSysctl
ensureJournal
ensureKubelet

if $FULL_INSTALL_REQUIRED; then
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        
        echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind
        sed -i "13i\echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind\n" /etc/rc.local
    fi
fi
rm -f /etc/apt/apt.conf.d/99periodic

if [[ $OS == $UBUNTU_OS_NAME ]]; then
    apt_get_purge 20 30 120 apache2-utils &
fi

VALIDATION_ERR=0
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
    API_SERVER_CONN_RETRIES=50
    if [[ $API_SERVER_NAME == *.privatelink.* ]]; then
        API_SERVER_CONN_RETRIES=100
    fi
    retrycmd_if_failure ${API_SERVER_CONN_RETRIES} 1 10 nc -vz ${API_SERVER_NAME} 443 || time nc -vz ${API_SERVER_NAME} 443 || VALIDATION_ERR=$ERR_K8S_API_SERVER_CONN_FAIL
fi

# If it is a MIG Node, enable mig-partition systemd service to create MIG instances
if [[ "${MIG_NODE}" == "true" ]]; then
    REBOOTREQUIRED=true
    ensureMigPartition
fi

if $REBOOTREQUIRED; then
    echo 'reboot required, rebooting node in 1 minute'
    /bin/bash -c "shutdown -r 1 &"
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        aptmarkWALinuxAgent unhold &
    fi
else
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        /usr/lib/apt/apt.systemd.daily &
        aptmarkWALinuxAgent unhold &
    fi
fi

echo "Custom script finished. API server connection check code:" $VALIDATION_ERR
echo $(date),$(hostname), endcustomscript>>/opt/m
mkdir -p /opt/azure/containers && touch /opt/azure/containers/provision.complete
ps auxfww > /opt/azure/provision-ps.log &

exit $VALIDATION_ERR

#EOF
