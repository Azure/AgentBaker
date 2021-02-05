/bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1
EXIT_CODE=$?
systemctl --no-pager -l status kubelet >> /var/log/azure/cluster-provision-cse-output.log 2>&1
OUTPUT=$(cat /var/log/azure/cluster-provision-cse-output.log | head -n 30)
START_TIME=$(echo "$OUTPUT" | cut -d ',' -f -1 | head -1)
CSE_EXECUTION_DURATION=$(echo $(($(date +%s.%3N) - $(date -d "$START_TIME" +%s.%3N))))
echo "CSE Total Execution Duration:$CSE_EXECUTION_DURATION  seconds" >> /var/log/azure/cluster-provision-execution-durations.log 2>&1
EXECUTION_DURATIONS=$(cat /var/log/azure/cluster-provision-execution-durations.log)

JSON_STRING=$( jq -n \
                  --arg ec "$EXIT_CODE" \
                  --arg op "$OUTPUT" \
                  --arg er "" \
                  --arg ed "$EXECUTION_DURATIONS" \
                  '{ExitCode: $ec, Output: $op, Error: $er, CSEExecutionDurations: $ed}' )
echo $JSON_STRING
exit $EXIT_CODE