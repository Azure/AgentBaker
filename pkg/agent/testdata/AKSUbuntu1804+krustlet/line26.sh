CSE_STARTTIME=$(date)
/bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1
EXIT_CODE=$?
systemctl --no-pager -l status kubelet >> /var/log/azure/cluster-provision-cse-output.log 2>&1
OUTPUT=$(head -c 3000 "/var/log/azure/cluster-provision-cse-output.log")
KUBELET_START_TIME=$(echo "$OUTPUT" | cut -d ',' -f -1 | head -1)
KERNEL_STARTTIME=$(systemctl show -p KernelTimestamp | sed -e  "s/KernelTimestamp=//g" || true)
GUEST_AGENT_STARTTIME=$(systemctl show walinuxagent.service -p ExecMainStartTimestamp | sed -e "s/ExecMainStartTimestamp=//g" || true)
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
                  '{ExitCode: $ec, Output: $op, Error: $er, ExecDuration: $ed, KernelStartTime: $ks, CSEStartTime: $cse, GuestAgentStartTime: $ga, SystemdSummary: $ss, BootDatapoints: { KubeletStartTime: $kubelet }}' )
echo $JSON_STRING
exit $EXIT_CODE