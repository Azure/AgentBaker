/bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1
EXIT_CODE=$?
systemctl --no-pager -l status kubelet >> /var/log/azure/cluster-provision-cse-output.log 2>&1
OUTPUT=$(head -c 3000 "/var/log/azure/cluster-provision-cse-output.log")
START_TIME=$(echo "$OUTPUT" | cut -d ',' -f -1 | head -1)
KERNEL_STARTTIME=$(systemctl show -p KernelTimestamp | sed -e  "s/KernelTimestamp=//g")
SYSTEMD_SUMMARY=$(systemd-analyze)
EXECUTION_DURATION=$(echo $(($(date +%s) - $(date -d "$START_TIME" +%s))))

JSON_STRING=$( jq -n \
                  --arg ec "$EXIT_CODE" \
                  --arg op "$OUTPUT" \
                  --arg er "" \
                  --arg ed "$EXECUTION_DURATION" \
                  --arg ks "$KERNEL_STARTTIME" \
                  --arg ss "$SYSTEMD_SUMMARY" \
                  '{ExitCode: $ec, Output: $op, Error: $er, ExecDuration: $ed, KernelStartTime: $ks, SystemdSummary: $ss}' )
echo $JSON_STRING
exit $EXIT_CODE