CSE_STARTTIME=$(date)
CSE_STARTTIME_FORMATTED=$(date +"%F %T.%3N")
timeout -k5s 15m /bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1
EXIT_CODE=$?
systemctl --no-pager -l status kubelet >> /var/log/azure/cluster-provision-cse-output.log 2>&1
OUTPUT=$(tail -c 3000 "/var/log/azure/cluster-provision.log")
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
message_string=$(echo $message_string | sed 's/\\//g' | sed 's/^.\(.*\).$/\1/')

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

upload_logs() {
    if test -x /opt/azure/containers/aks-log-collector.sh; then
        /opt/azure/containers/aks-log-collector.sh >/var/log/azure/aks/cse-aks-log-collector.log 2>&1
    else
        PYTHONPATH=$(find /var/lib/waagent -name WALinuxAgent\*.egg | sort -rV | head -n1)
        python3 $PYTHONPATH -collect-logs -full >/dev/null 2>&1
        python3 /opt/azure/containers/provision_send_logs.py >/dev/null 2>&1
    fi
}
if [ $EXIT_CODE -ne 0 ]; then
    upload_logs
else
    upload_logs &
fi

exit $EXIT_CODE