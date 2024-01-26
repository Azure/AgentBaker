#!/bin/bash
# 
# AKS Check Network
#
# This script is used to check network connectivity from the node to certain required AKS endpoints 
# and log the results to the events directory. For now, this script has to be triggered manually to
# collect the log. In the future, we will run it periodically to check and alert any issue.

APISERVER_FQDN=${1:-''}
CUSTOM_ENDPOINT=${2:-''}

EVENTS_LOGGING_PATH="/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/"
AZURE_CONFIG_PATH="/etc/kubernetes/azure.json"
AKS_CA_CERT_PATH="/etc/kubernetes/certs/apiserver.crt"
AKS_CERT_PATH="/etc/kubernetes/certs/client.crt"
AKS_KEY_PATH="/etc/kubernetes/certs/client.key"
RESOLV_CONFIG_PATH="/etc/resolv.conf"
SYSTEMD_RESOLV_CONFIG_PATH="/run/systemd/resolve/resolv.conf"

ARM_ENDPOINT="management.azure.com"
METADATA_ENDPOINT="http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://${ARM_ENDPOINT}/"
API_VERSION="2023-11-01"
AKS_ENDPOINT="https://${ARM_ENDPOINT}/providers/Microsoft.ContainerService/operations?api-version=${API_VERSION}"
APISERVER_ENDPOINT="https://${APISERVER_FQDN}/healthz"
ACS_BINARY_ENDPOINT="acs-mirror.azureedge.net/azure-cni/v1.4.43/binaries/azure-vnet-cni-linux-amd64-v1.4.43.tgz"

TEMP_DIR=$(mktemp -d)
NSLOOKUP_FILE="${TEMP_DIR}/nslookup.log"
TOKEN_FILE="${TEMP_DIR}/access_token.json"

URL_LIST=("mcr.microsoft.com" "login.microsoftonline.com" "packages.microsoft.com" "acs-mirror.azureedge.net")
MAX_RETRY=3
DELAY=5

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
    # echo ${json_string} > ${EVENTS_LOGGING_PATH}${eventsFileName}.json

    # this allows an error from the command at ${@} to be returned and correct code assigned in cse_main
    if [ "$ret" != "0" ]; then
      return $ret
    fi
}

function dns_trace {
    local endpoint=$1

    echo "Trace DNS request for $endpoint"
    dig $endpoint
    host -a $endpoint
}

function check_and_curl {
    local url=$1
    local error_msg=$2

    # Check DNS 
    nslookup $url > /dev/null
    if [ $? -eq 0 ]; then
        logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested DNS resolution to $url'"
    else
        dns_trace $url
        logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to test DNS resolution to $url'"
        continue
    fi

    i=0
    while true;
    do
        # Curl the url and capture the response code
        if [ $url == "acs-mirror.azureedge.net" ]; then
            response=$(curl -I -s -o /dev/null -w "%{http_code}" "https://${ACS_BINARY_ENDPOINT}" -L)
        else
            response=$(curl -s -o /dev/null -w "%{http_code}" "https://${url}" -L)
        fi

        if [ $response -ge 200 ] && [ $response -lt 400 ]; then
            logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully curled $url with returned status code $response'"
            break
        fi

        # If the response code is not within successful range, increment the error count
        i=$(( $i + 1 ))
        # If we have reached the maximum number of retries, log an error
        if [[ $i -eq $MAX_RETRY ]]; then
            logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to curl $url after $MAX_RETRY attempts with returned status code $response. $error_msg'" 
            break
        fi
        
        # Sleep for the specified delay before trying again
        sleep $DELAY
    done
}


if ! [ -e "${AZURE_CONFIG_PATH}" ]; then
    logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - WARNING: Failed to find $AZURE_CONFIG_PATH file. Are you running inside Kubernetes?'"
fi

# check DNS resolution to ARM endpoint
nslookup $ARM_ENDPOINT > $NSLOOKUP_FILE
if [ $? -eq 0 ]; then
    logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested DNS resolution to $ARM_ENDPOINT'"
else
    error_log=$(cat $NSLOOKUP_FILE)
    logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to test DNS resolution to $ARM_ENDPOINT with error $error_log'"

    # check resolv.conf
    nameserver=$(cat $NSLOOKUP_FILE | grep "Server" | awk '{print $2}')
    echo "Checking resolv.conf for nameserver $nameserver"
    cat $RESOLV_CONFIG_PATH | grep $nameserver 
    if [ $? -ne 0 ]; then
        logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - FAILURE: Nameserver $nameserver wasn't found in $RESOLV_CONFIG_PATH'"
    fi
    cat $SYSTEMD_RESOLV_CONFIG_PATH | grep $nameserver 
    if [ $? -ne 0 ]; then
        logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - FAILURE: Nameserver $nameserver wasn't found in $SYSTEMD_RESOLV_CONFIG_PATH'"
    fi

    # trace request
    dns_trace $ARM_ENDPOINT

    exit 1
fi

# check access to ARM endpoint
result=$(curl -s -o $TOKEN_FILE -w "%{http_code}" -H Metadata:true $METADATA_ENDPOINT)
if [ $result -eq 200 ]; then
    logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully retrieved access token'"
    access_token=$(cat $TOKEN_FILE | jq -r .access_token)
    res=$(curl -X GET -H "Authorization: Bearer $access_token" -H "Content-Type:application/json" -s -o /dev/null -w "%{http_code}" $AKS_ENDPOINT)
    if [ $res -ge 200 ] && [ $res -lt 400 ]; then
        logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully curled $ARM_ENDPOINT with returned status code $res'"
    else 
        logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to curl $ARM_ENDPOINT with returned status code $res. This endpoint is required for Kubernetes operations against the Azure API'" 
    fi
else
    logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to retrieve access token with returned status code $result. Can't check access to $ARM_ENDPOINT'" 
fi

# check access to apiserver
if [ -z "$APISERVER_FQDN" ]; then
    logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - WARNING: No apiserver FQDN provided. Skipping apiserver check.'"
else
    nslookup $APISERVER_FQDN > /dev/null
    if [ $? -eq 0 ]; then
        logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested DNS resolution to $APISERVER_FQDN'"
        res=$(curl -s -o /dev/null -w "%{http_code}" --cacert $AKS_CA_CERT_PATH --cert $AKS_CERT_PATH --key $AKS_KEY_PATH $APISERVER_ENDPOINT)
        if [ $res -ge 200 ] && [ $res -lt 400 ]; then
            logs_to_events "AKS.CSE.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully curled apiserver $APISERVER_FQDN with returned status code $res'"
        else 
            logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to curl $APISERVER_FQDN with returned status code $res. Node can't connect to the apiserver'" 
        fi
    else
        logs_to_events "AKS.CSE.testingTraffic.failure" "echo '$(date) - ERROR: Failed to test DNS resolution to $APISERVER_FQDN'"
        dns_trace $APISERVER_FQDN
    fi
fi

for url in ${URL_LIST[@]};
do
    check_and_curl $url ""
done

if [ ! -z "$CUSTOM_ENDPOINT" ]; then
    echo "Checking additional endpoints ..."  
    extra_urls=($(echo $CUSTOM_ENDPOINT | tr "," "\n"))
    for url in "${extra_urls[@]}"
    do
        check_and_curl $url ""
    done
fi