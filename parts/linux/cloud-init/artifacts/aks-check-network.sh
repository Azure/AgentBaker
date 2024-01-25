#!/bin/bash
# 
# AKS Check Network
#
# This script is used to check network connectivity from the node to certain required AKS endpoints 
# and log the results to the events directory. For now, this script has to be triggered manually to
# collect the log. In the future, we will run it periodically to check and alert any issue.

set -o nounset
set -o pipefail

APISERVER_FQDN=$1
CUSTOM_ENDPOINT=${2:-''}

EVENTS_LOGGING_PATH="/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/"
AZURE_CONFIG_PATH="/etc/kubernetes/azure.json"
RESOLV_CONFIG_PATH="/etc/resolv.conf"
SYSTEMD_RESOLV_CONFIG_PATH="/run/systemd/resolve/resolv.conf"

ARM_ENDPOINT="management.azure.com"
METADATA_ENDPOINT="http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://${ARM_ENDPOINT}/"
AKS_ENDPOINT="https://${resource}/providers/Microsoft.ContainerService/operations?api-version=2023-05-01"

TEMP_DIR=$(mktemp -d)


function logs_to_events {
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
    echo ${json_string} > ${EVENTS_LOGGING_PATH}${eventsFileName}.json

    # this allows an error from the command at ${@} to be returned and correct code assigned in cse_main
    if [ "$ret" != "0" ]; then
      return $ret
    fi
}


if ! [ -e "${AZURE_CONFIG_PATH}" ]; then
    logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to find azure.json file. Are you running inside Kubernetes?'"
    exit 1
fi
azure_config=$(cat $AZURE_CONFIG_PATH)

# check DNS resolution to ARM endpoint
nslookup $ARM_ENDPOINT > ${TEMP_DIR}/nslookup.log
if [ $? -eq 0 ]; then
    logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested DNS resolution to endpoint $ARM_ENDPOINT'"
else
    error_log=$(cat ${TEMP_DIR}/nslookup.log)
    logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to test DNS resolution to endpoint $ARM_ENDPOINT with error $error_log'"

    # perform basic DNS troubleshooting
    nameserver=$(cat ${TEMP_DIR}/nslookup.log | grep "Server" | awk '{print $2}')
    echo "Checking resolv.conf for nameserver $nameserver"
    cat $RESOLV_CONFIG_PATH | grep $nameserver 
    if [ $? -ne 0 ]; then
        logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - FAILURE: Nameserver $nameserver wasn't found in $RESOLV_CONFIG_PATH'"
    fi
    cat $SYSTEMD_RESOLV_CONFIG_PATH | grep $nameserver 
    if [ $? -ne 0 ]; then
        logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - FAILURE: Nameserver $nameserver wasn't found in $SYSTEMD_RESOLV_CONFIG_PATH'"
    fi
    exit 1
fi

# check access to ARM endpoint
token_file="${TEMP_DIR}/access_token.json"
result=$(curl -s -o $token_file -w "%{http_code}" -H Metadata:true $METADATA_ENDPOINT)
if [ $result -eq 200 ]; then
    logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully sent metadata endpoint request with returned status code $result'"
    access_token=$(cat $token_file | jq -r .access_token)
    res=$(curl -X GET -H "Authorization: Bearer $access_token" -H "Content-Type:application/json" -s -o /dev/null -w "%{http_code}" $AKS_ENDPOINT)
    if [ $res -ge 200 ] && [ $res -lt 400 ]; then
        logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully curled $resource with returned status code $res'"
    else 
        logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to curl $resource with returned status code $res'" 
    fi
else
    logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to send metadata endpoint request with returned status code $result. Can't check access to $resource'" 
fi

# Check access to apiserver
nslookup $FQDN > /dev/null
if [ $? -eq 0 ]; then
    logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested DNS resolution to endpoint $FQDN'"
    fullUrl="https://${FQDN}/healthz"
    res=$(curl -s -o /dev/null -w "%{http_code}" --cacert /etc/kubernetes/certs/apiserver.crt --cert /etc/kubernetes/certs/client.crt --key /etc/kubernetes/certs/client.key $fullUrl)
    if [ $res -ge 200 ] && [ $res -lt 400 ]; then
        logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully curled apiserver $FQDN with returned status code $res'"
    else 
        logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to curl apiserver $FQDN with returned status code $res'" 
    fi
else
    logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to test DNS resolution to endpoint $url'"
fi

# Set the URLs to ping
urlLists=("mcr.microsoft.com" "login.microsoftonline.com" "packages.microsoft.com" "acs-mirror.azureedge.net")

# Set the number of times to retry before logging an error
retries=3

# Set the delay between retries in seconds
delay=5

for url in ${urlLists[@]};
do
    # Check DNS 
    nslookup $url > /dev/null
    if [ $? -eq 0 ]; then
        logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested DNS resolution to endpoint $url'"
    else
        logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to test DNS resolution to endpoint $url'"
        continue
    fi

    i=0
    while true;
    do
        # Ping the URLs and capture the response code
        if [ $url == "acs-mirror.azureedge.net" ]; then
            fullUrl="https://${url}/azure-cni/v1.4.43/binaries/azure-vnet-cni-linux-amd64-v1.4.43.tgz"
            response=$(curl -I -s -o /dev/null -w "%{http_code}" $fullUrl -L)
        else
            response=$(curl -s -o /dev/null -w "%{http_code}" "https://$url" -L)
        fi

        if [ $response -ge 200 ] && [ $response -lt 400 ]; then
            logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully curled $url with returned status code $response'"
            break
        fi

        # If the response code is not 200, increment the error count
        i=$(( $i + 1 ))
        # If we have reached the maximum number of retries, log an error
        if [[ $i -eq $retries ]]; then
            logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to curl $url after $retries attempts with returned status code $response'" 
            break
        fi

        # Sleep for the specified delay before trying again
        sleep $delay
    done
done

if [ ! -z "$CUSTOM_ENDPOINT" ]; then
    echo "Checking additional endpoints ..."  
    extraUrlList=($(echo $CUSTOM_ENDPOINT | tr "," "\n"))
    for url in "${extraUrlList[@]}"
    do
        nslookup $url > /dev/null
        if [ $? -eq 0 ]; then
            logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested DNS resolution to endpoint $url'"
        else
            logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to test DNS resolution to endpoint $url'"
            continue
        fi

        i=0
        while true;
        do
            response=$(curl -s -o /dev/null -w "%{http_code}" "https://$url" -L)
            if [ $response -ge 200 ] && [ $response -lt 400 ]; then
                logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully curled $url with returned status code $response'"
                break
            fi

            i=$(( $i + 1 ))
            if [[ $i -eq $retries ]]; then
                logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to curl $url after $retries attempts with returned status code $response'" 
                break
            fi

            sleep $delay
        done
    done
fi