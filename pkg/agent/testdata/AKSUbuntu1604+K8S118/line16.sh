/bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1
EXIT_CODE=$?
systemctl --no-pager -l status kubelet >> /var/log/azure/cluster-provision-cse-output.log 2>&1
OUTPUT=$(cat /var/log/azure/cluster-provision-cse-output.log | head -n 30)
JSON_STRING=$( jq -n \
                  --arg ec "$EXIT_CODE" \
                  --arg op "$OUTPUT" \
                  --arg er "" \
                  '{ExitCode: $ec, Output: $op, Error: $er}' )
echo $JSON_STRING
exit $EXIT_CODE