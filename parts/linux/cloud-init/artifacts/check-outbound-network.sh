#!/bin/bash

set -o nounset
set -o pipefail

EVENTS_LOGGING_DIR="/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/"

logs_to_events() {
    # local vars here allow for nested function tracking
    # installContainerRuntime for example
    local task=$1; shift
    local eventsFileName=$(date +%s%3N)

    local startTime=$(date +"%F %T.%3N")
    ${@}
    ret=$?
    local endTime=$(date +"%F %T.%3N")

    # arg names are defined by GA and all these are required to be correctly read by GA
    # EventPid, EventTid are required to be int. No use case for them at this point.
    json_string=$( jq -n \
        --arg Timestamp   "${startTime}" \
        --arg OperationId "${endTime}" \
        --arg Version     "1.23" \
        --arg TaskName    "${task}" \
        --arg EventLevel  "Informational" \
        --arg Message     "Completed: ${@}" \
        --arg EventPid    "0" \
        --arg EventTid    "0" \
        '{Timestamp: $Timestamp, OperationId: $OperationId, Version: $Version, TaskName: $TaskName, EventLevel: $EventLevel, Message: $Message, EventPid: $EventPid, EventTid: $EventTid}'
    )
    echo ${json_string} > ${EVENTS_LOGGING_DIR}${eventsFileName}.json

    # this allows an error from the command at ${@} to be returned and correct code assigned in cse_main
    if [ "$ret" != "0" ]; then
      return $ret
    fi
}

# Check access to management.azure.com endpoint
azure_config=$(cat /etc/kubernetes/azure.json)
if [ -z $azure_config ]; then
    logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to read azure.json file. Are you running inside Kubernetes?'"
    exit 1
fi

aad_client_id=$(echo $azure_config | jq -r '.aadClientId')
aad_client_secret=$(echo $azure_config | jq -r '.aadClientSecret')
if [ $aad_client_id == "msi" ] && [ $aad_client_secret == "msi" ]; then
    client_id=$(echo $azure_config | jq -r '.userAssignedIdentityID')
    subscription_id=$(echo $azure_config | jq -r '.subscriptionId')
    location=$(echo $azure_config | jq -r '.location')
    node_resource_group=$(echo $azure_config | jq -r '.resourceGroup')
    resource_group=$(echo $node_resource_group | cut -d '_' -f 2)
    cluster_name=$(echo $node_resource_group | cut -d '_' -f 3)

    resource="https://management.azure.com"
    metadata_endpoint="http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&client_id=${client_id}&resource=${resource}/"
    tmp_file="/tmp/managedCluster.json"
    result=$(curl -s -o $tmp_file -w "%{http_code}" -H Metadata:true $metadata_endpoint)
    if [ $result -ne 200 ]; then
        logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to send metadata endpoint request with returned status code $result'" 
    fi

    access_token=$(cat $tmp_file | jq -r .access_token)
    clusterEndpoint="${resource}/subscriptions/${subscription_id}/resourceGroups/${resource_group}/providers/Microsoft.ContainerService/managedClusters/${cluster_name}?api-version=2023-04-01"
    res=$(curl -X GET -H "Authorization: Bearer $access_token" -H "Content-Type:application/json" -s -o $tmp_file -w "%{http_code}" $clusterEndpoint)
    if [ $res -eq 200 ]; then
        logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully pinged $resource with returned status code $res'"
        fqdn=$(cat $tmp_file | jq -r .properties.fqdn)
    else 
        logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to ping $resource with returned status code $res'" 
    fi
else 
    logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Unable to check access to management.azure.com endpoint'"
fi

# Set the URLs to ping
urlLists=("https://mcr.microsoft.com" "https://login.microsoftonline.com" "https://packages.microsoft.com")

# Set the number of times to retry before logging an error
retries=3

# Set the delay between retries in seconds
delay=5

for url in ${urlLists[@]};
do
    i=0
    while true;
    do
        # Ping the URLs and capture the response code
        response=$(curl -s -o /dev/null -w "%{http_code}" $url -L)

        if [ $response -eq 200 ]; then
            logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully pinged $url with returned status code $response'"
            break
        else 
            # If the response code is not 200, increment the error count
            i=$(( $i + 1 ))
        fi

        # If we have reached the maximum number of retries, log an error
        if [[ $i -eq $retries ]]; then
            logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to ping $url after $retries attempts with returned status code $response'" 
            break
        fi

        # Sleep for the specified delay before trying again
        sleep $delay
    done
done


