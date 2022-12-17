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

retrycmd_if_failure() { r=$1; w=$2; t=$3; shift && shift && shift; for i in $(seq 1 $r); do timeout $t ${@}; [ $? -eq 0  ] && break || if [ $i -eq $r ]; then return 1; else sleep $w; fi; done }; ERR_OUTBOUND_CONN_FAIL=50; retrycmd_if_failure 50 1 5 nc -vz mcr.microsoft.com 443 >> /var/log/azure/cluster-provision-cse-output.log 2>&1 || time nc -vz mcr.microsoft.com 443 || exit $ERR_OUTBOUND_CONN_FAIL;

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

if [[ "false" == "true" ]]; then
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


logs_to_events "AKS.CSE.installKubeletKubectlAndKubeProxy" installKubeletKubectlAndKubeProxy

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

logs_to_events "AKS.CSE.ensureKubelet" ensureKubelet

if $FULL_INSTALL_REQUIRED; then
    if [[ $OS == $UBUNTU_OS_NAME ]]; then
        
        echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind
        sed -i "13i\echo 2dd1ce17-079e-403c-b352-a1921ee207ee > /sys/bus/vmbus/drivers/hv_util/unbind\n" /etc/rc.local
    fi
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
        systemctl restart --no-block apt-daily.timer apt-daily-upgrade.timer
        # this is the DOWNLOAD service
        # meaning we are wasting IO without even triggering an upgrade 
        # -________________-
        systemctl restart --no-block apt-daily.service
        aptmarkWALinuxAgent unhold &
    fi
fi

echo "Custom script finished. API server connection check code:" $VALIDATION_ERR
echo $(date),$(hostname), endcustomscript>>/opt/m
mkdir -p /opt/azure/containers && touch /opt/azure/containers/provision.complete

timeout 60s grep -q 'NodeReady' <(journalctl -u kubelet -f) || exit 1  

exit $VALIDATION_ERR

KERNEL_STARTTIME=$(systemctl show -p KernelTimestamp | sed -e  "s/KernelTimestamp=//g" || true)
KERNEL_STARTTIME_FORMATTED=$(date -d "${KERNEL_STARTTIME}" +"%F %T.%3N" )
CLOUDINITLOCAL_STARTTIME=$(systemctl show cloud-init-local -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
CLOUDINITLOCAL_STARTTIME_FORMATTED=$(date -d "${CLOUDINITLOCAL_STARTTIME}" +"%F %T.%3N" )
CLOUDINIT_STARTTIME=$(systemctl show cloud-init -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
CLOUDINIT_STARTTIME_FORMATTED=$(date -d "${CLOUDINIT_STARTTIME}" +"%F %T.%3N" )
CLOUDINITFINAL_STARTTIME=$(systemctl show cloud-final -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
CLOUDINITFINAL_STARTTIME_FORMATTED=$(date -d "${CLOUDINITFINAL_STARTTIME}" +"%F %T.%3N" )
NETWORKD_STARTTIME=$(systemctl show systemd-networkd -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
NETWORKD_STARTTIME_FORMATTED=$(date -d "${NETWORKD_STARTTIME}" +"%F %T.%3N" )
GUEST_AGENT_STARTTIME=$(systemctl show walinuxagent.service -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
GUEST_AGENT_STARTTIME_FORMATTED=$(date -d "${GUEST_AGENT_STARTTIME}" +"%F %T.%3N" )
KUBELET_START_TIME=$(systemctl show kubelet.service -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
KUBELET_START_TIME_FORMATTED=$(date -d "${KUBELET_START_TIME}" +"%F %T.%3N" )
KUBELET_READY_TIME_FORMATTED="$(date -d "$(journalctl -u kubelet | grep NodeReady | cut -d' ' -f1-3)" +"%F %T.%3N")"
SYSTEMD_SUMMARY=$(systemd-analyze || true)
CSE_ENDTIME_FORMATTED=$(date +"%F %T.%3N")
EVENTS_LOGGING_DIR=/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/
EVENTS_FILE_NAME=$(date +%s%3N)
EXECUTION_DURATION=$(echo $(($(date +%s) - $(date -d "$CSE_STARTTIME" +%s))))

JSON_STRING=$( jq -n \
                  --arg ec "$EXIT_CODE" \
                  --arg op "$OUTPUT" \
                  --arg er "" \
                  --arg ed "$EXECUTION_DURATION" \
                  --arg ks "$KERNEL_STARTTIME" \
                  --arg cinitl "$CLOUDINITLOCAL_STARTTIME" \
                  --arg cinit "$CLOUDINIT_STARTTIME" \
                  --arg cf "$CLOUDINITFINAL_STARTTIME" \
                  --arg ns "$NETWORKD_STARTTIME" \
                  --arg cse "$CSE_STARTTIME" \
                  --arg ga "$GUEST_AGENT_STARTTIME" \
                  --arg ss "$SYSTEMD_SUMMARY" \
                  --arg kubelet "$KUBELET_START_TIME" \
                  '{ExitCode: $ec, Output: $op, Error: $er, ExecDuration: $ed, KernelStartTime: $ks, CloudInitLocalStartTime: $cinitl, CloudInitStartTime: $cinit, CloudFinalStartTime: $cf, NetworkdStartTime: $ns, CSEStartTime: $cse, GuestAgentStartTime: $ga, SystemdSummary: $ss, BootDatapoints: { KernelStartTime: $ks, CSEStartTime: $cse, GuestAgentStartTime: $ga, KubeletStartTime: $kubelet }}' )
mkdir -p /var/log/azure/aks
echo $JSON_STRING | tee /var/log/azure/aks/provision.json

# messsage_string is here because GA only accepts strings in Message.
message_string=$( jq -n \
--arg EXECUTION_DURATION                  "${EXECUTION_DURATION}" \
--arg EXIT_CODE                           "${EXIT_CODE}" \
--arg KERNEL_STARTTIME_FORMATTED          "${KERNEL_STARTTIME_FORMATTED}" \
--arg CLOUDINITLOCAL_STARTTIME_FORMATTED  "${CLOUDINITLOCAL_STARTTIME_FORMATTED}" \
--arg CLOUDINIT_STARTTIME_FORMATTED       "${CLOUDINIT_STARTTIME_FORMATTED}" \
--arg CLOUDINITFINAL_STARTTIME_FORMATTED  "${CLOUDINITFINAL_STARTTIME_FORMATTED}" \
--arg NETWORKD_STARTTIME_FORMATTED        "${NETWORKD_STARTTIME_FORMATTED}" \
--arg GUEST_AGENT_STARTTIME_FORMATTED     "${GUEST_AGENT_STARTTIME_FORMATTED}" \
--arg KUBELET_START_TIME_FORMATTED        "${KUBELET_START_TIME_FORMATTED}" \
--arg KUBELET_READY_TIME_FORMATTED       "${KUBELET_READY_TIME_FORMATTED}" \
'{ExitCode: $EXIT_CODE, E2E: $EXECUTION_DURATION, KernelStartTime: $KERNEL_STARTTIME_FORMATTED, CloudInitLocalStartTime: $CLOUDINITLOCAL_STARTTIME_FORMATTED, CloudInitStartTime: $CLOUDINIT_STARTTIME_FORMATTED, CloudFinalStartTime: $CLOUDINITFINAL_STARTTIME_FORMATTED, NetworkdStartTime: $NETWORKD_STARTTIME_FORMATTED, GuestAgentStartTime: $GUEST_AGENT_STARTTIME_FORMATTED, KubeletStartTime: $KUBELET_START_TIME_FORMATTED, KubeletReadyTime: $KUBELET_READY_TIME_FORMATTED } | tostring'
)
# this clean up brings me no joy, but removing extra "\" and then removing quotes at the end of the string
# allows parsing to happening without additional manipulation
message_string=$(echo $message_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

# arg names are defined by GA and all these are required to be correctly read by GA
# EventPid, EventTid are required to be int. No use case for them at this point.
EVENT_JSON=$( jq -n \
    --arg Timestamp     "${CSE_STARTTIME_FORMATTED}" \
    --arg OperationId   "${CSE_ENDTIME_FORMATTED}" \
    --arg Version       "1.23" \
    --arg TaskName      "AKS.CSE.cse_start" \
    --arg EventLevel    "${eventlevel}" \
    --arg Message       "${message_string}" \
    --arg EventPid      "0" \
    --arg EventTid      "0" \
    '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
)
echo ${EVENT_JSON} > ${EVENTS_LOGGING_DIR}${EVENTS_FILE_NAME}.json

#EOF
