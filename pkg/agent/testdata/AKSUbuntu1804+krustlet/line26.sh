CSE_STARTTIME=$(date)
timeout --preserve-status -k5s 15m /bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1
EXIT_CODE=$?
systemctl --no-pager -l status kubelet >> /var/log/azure/cluster-provision-cse-output.log 2>&1
OUTPUT=$(tail -c 3000 "/var/log/azure/cluster-provision.log")
KERNEL_STARTTIME=$(systemctl show -p KernelTimestamp | sed -e  "s/KernelTimestamp=//g" || true)
GUEST_AGENT_STARTTIME=$(systemctl show walinuxagent.service -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
KUBELET_START_TIME=$(systemctl show kubelet.service -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
SYSTEMD_SUMMARY=$(systemd-analyze || true)
EXECUTION_DURATION=$(echo $(($(date +%s) - $(date -d "$CSE_STARTTIME" +%s))))

JSON_STRING=$( jq -n \
                  --arg ec "$EXIT_CODE" \
                  --arg op "$OUTPUT" \
                  --arg er "" \
                  --arg ed "$EXECUTION_DURATION" \
                  --arg ks "$KERNEL_STARTTIME" \
                  --arg cse "$CSE_STARTTIME" \
                  --arg ga "$GUEST_AGENT_STARTTIME" \
                  --arg ss "$SYSTEMD_SUMMARY" \
                  --arg kubelet "$KUBELET_START_TIME" \
                  '{ExitCode: $ec, Output: $op, Error: $er, ExecDuration: $ed, KernelStartTime: $ks, CSEStartTime: $cse, GuestAgentStartTime: $ga, SystemdSummary: $ss, BootDatapoints: { KernelStartTime: $ks, CSEStartTime: $cse, GuestAgentStartTime: $ga, KubeletStartTime: $kubelet }}' )
mkdir -p /var/log/azure/aks
echo $JSON_STRING | tee /var/log/azure/aks/provision.json

# force a log upload to the host after the provisioning script finishes
# if we failed, wait for the upload to complete so that we don't remove
# the VM before it finishes. if we succeeded, upload in the background
# so that the provisioning script returns success more quickly
upload_logs() {
    # find the most recent version of WALinuxAgent and use it to collect logs per
    # https://supportability.visualstudio.com/AzureIaaSVM/_wiki/wikis/AzureIaaSVM/495009/Log-Collection_AGEX?anchor=manually-collect-logs
    PYTHONPATH=$(find /var/lib/waagent -name WALinuxAgent\*.egg | sort -rV | head -n1)
    python3 $PYTHONPATH -collect-logs -full >/dev/null 2>&1
    python3 /opt/azure/containers/provision_send_logs.py >/dev/null 2>&1
}
if [ $EXIT_CODE -ne 0 ]; then
    upload_logs
else
    upload_logs &
fi

exit $EXIT_CODE