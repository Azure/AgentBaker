#! /bin/bash
# 
# AKS Check Network
#
# This script is used to check network connectivity from the node to certain required AKS endpoints 
# and log the results to the events directory. For now, this script has to be triggered manually to
# collect the log. In the future, we will run it periodically to check and alert any issue.

CUSTOM_ENDPOINT=${1:-''}

EVENTS_LOGGING_PATH="/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/"
AZURE_CONFIG_PATH="/etc/kubernetes/azure.json"
AKS_CA_CERT_PATH="/etc/kubernetes/certs/ca.crt"
AKS_CERT_PATH="/etc/kubernetes/certs/client.crt"
AKS_KEY_PATH="/etc/kubernetes/certs/client.key"
AKS_KUBECONFIG_PATH="/var/lib/kubelet/kubeconfig"
RESOLV_CONFIG_PATH="/etc/resolv.conf"
SYSTEMD_RESOLV_CONFIG_PATH="/run/systemd/resolve/resolv.conf"

ARM_ENDPOINT="management.azure.com"
METADATA_ENDPOINT="http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://${ARM_ENDPOINT}/"
AKS_ENDPOINT="https://${ARM_ENDPOINT}/providers/Microsoft.ContainerService/operations?api-version=2023-11-01"

TEMP_DIR=$(mktemp -d)
NSLOOKUP_FILE="${TEMP_DIR}/nslookup.log"
TOKEN_FILE="${TEMP_DIR}/access_token.json"

MAX_RETRY=3
MAX_TIME=10
DELAY=5

declare -A URL_LIST=( 
    ["mcr.microsoft.com"]="This is required to access images in Microsoft Container Registry (MCR). These images are required for the correct creation and functioning of the cluster, including scale and upgrade operations."\
    ["eastus.data.mcr.microsoft.com"]="FQDN *.data.mcr.microsoft.com is required for MCR storage backed by the Azure content delivery network (CDN)."\
    ["login.microsoftonline.com"]="This is equired for Microsoft Entra authentication."\
    ["packages.microsoft.com"]="This is required to download packages (like Moby, PowerShell, and Azure CLI) using cached apt-get operations."\
    ["acs-mirror.azureedge.net"]="This is required to download and install required binaries like kubenet and Azure CNI."\
)

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

function dns_trace {
    local endpoint=$1

    echo "Trace DNS request for $endpoint"
    dig $endpoint
    host -a $endpoint
}

function check_and_curl {
    local url=$1
    local error_msg=$2

    # check DNS 
    nslookup $url > /dev/null
    if [ $? -eq 0 ]; then
        logs_to_events "AKS.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested DNS resolution to $url'"
    else
        logs_to_events "AKS.testingTraffic.failure" "echo '$(date) - ERROR: Failed to test DNS resolution to $url. $error_msg'"
        dns_trace $url
        return 1
    fi

    local i=0
    while true;
    do
        # curl the url and capture the response code
        response=$(curl -s -m $MAX_TIME -o /dev/null -w "%{http_code}" "https://${url}" -L)

        if [ $response -ge 200 ] && [ $response -lt 400 ]; then
            logs_to_events "AKS.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested $url with returned status code $response'"
            break
        elif [ $response -eq 400 ] && ([ $url == "acs-mirror.azureedge.net" ] || [ $url == "eastus.data.mcr.microsoft.com" ]); then
            logs_to_events "AKS.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested $url with returned status code $response. This is expected since $url is a repository endpoint which requires a full package path to get 200 status code.'"
            break
        else
            # if the response code is not within successful range, increment the error count
            i=$(( $i + 1 ))
            # if we have reached the maximum number of retries, log an error
            if [[ $i -eq $MAX_RETRY ]]; then
                logs_to_events "AKS.testingTraffic.failure" "echo '$(date) - ERROR: Failed to curl $url after $MAX_RETRY attempts with returned status code $response. $error_msg'" 
                break
            fi
            
            # sleep for the specified delay before trying again
            sleep $DELAY
        fi
    done
}

logs_to_events "AKS.testingTraffic.start" "echo '$(date) - INFO: Starting network connectivity check'"

if ! [ -f "${AZURE_CONFIG_PATH}" ]; then
    logs_to_events "AKS.testingTraffic.failure" "echo '$(date) - WARNING: Failed to find $AZURE_CONFIG_PATH file. Are you running inside Kubernetes?'"
fi

# check DNS resolution to ARM endpoint
nslookup $ARM_ENDPOINT > $NSLOOKUP_FILE
if [ $? -eq 0 ]; then
    logs_to_events "AKS.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested DNS resolution to $ARM_ENDPOINT'"
else
    error_log=$(cat $NSLOOKUP_FILE)
    logs_to_events "AKS.testingTraffic.failure" "echo '$(date) - ERROR: Failed to test DNS resolution to $ARM_ENDPOINT with error $error_log'"

    # check resolv.conf
    nameserver=$(cat $NSLOOKUP_FILE | grep "Server" | awk '{print $2}')
    echo "Checking resolv.conf for nameserver $nameserver"
    cat $RESOLV_CONFIG_PATH | grep $nameserver 
    if [ $? -ne 0 ]; then
        logs_to_events "AKS.testingTraffic.failure" "echo '$(date) - FAILURE: Nameserver $nameserver wasn't found in $RESOLV_CONFIG_PATH'"
    fi
    cat $SYSTEMD_RESOLV_CONFIG_PATH | grep $nameserver 
    if [ $? -ne 0 ]; then
        logs_to_events "AKS.testingTraffic.failure" "echo '$(date) - FAILURE: Nameserver $nameserver wasn't found in $SYSTEMD_RESOLV_CONFIG_PATH'"
    fi

    # trace request
    dns_trace $ARM_ENDPOINT

    exit 1
fi

# check access to ARM endpoint
result=$(curl -m $MAX_TIME -s -o $TOKEN_FILE -w "%{http_code}" -H Metadata:true $METADATA_ENDPOINT)
if [ $result -eq 200 ]; then
    logs_to_events "AKS.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully retrieved access token'"
    access_token=$(cat $TOKEN_FILE | jq -r .access_token)
    res=$(curl -m $MAX_TIME -X GET -H "Authorization: Bearer $access_token" -H "Content-Type:application/json" -s -o /dev/null -w "%{http_code}" $AKS_ENDPOINT)
    if [ $res -ge 200 ] && [ $res -lt 400 ]; then
        logs_to_events "AKS.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested $ARM_ENDPOINT with returned status code $res'"
    else 
        logs_to_events "AKS.testingTraffic.failure" "echo '$(date) - ERROR: Failed to test $ARM_ENDPOINT with returned status code $res. This endpoint is required for Kubernetes operations against the Azure API'" 
    fi
else
    logs_to_events "AKS.testingTraffic.failure" "echo '$(date) - ERROR: Failed to retrieve access token with returned status code $result. Can't check access to $ARM_ENDPOINT'" 
fi

# check access to apiserver
if ! [ -f "${AKS_KUBECONFIG_PATH}" ]; then
    logs_to_events "AKS.testingTraffic.warning" "echo '$(date) - WARNING: Kubeconfig file not found. Skipping apiserver check.'"
else
    APISERVER_FQDN=$(grep server $AKS_KUBECONFIG_PATH | awk -F"server: https://" '{print $2}' | cut -d : -f 1)
    APISERVER_ENDPOINT="https://${APISERVER_FQDN}/healthz"
    nslookup $APISERVER_FQDN > /dev/null
    if [ $? -eq 0 ]; then
        logs_to_events "AKS.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested DNS resolution to $APISERVER_FQDN'"
        res=$(curl -m $MAX_TIME -s -o /dev/null -w "%{http_code}" --cacert $AKS_CA_CERT_PATH --cert $AKS_CERT_PATH --key $AKS_KEY_PATH $APISERVER_ENDPOINT)
        if [ $res -ge 200 ] && [ $res -lt 400 ]; then
            logs_to_events "AKS.testingTraffic.success" "echo '$(date) - SUCCESS: Successfully tested apiserver $APISERVER_FQDN with returned status code $res'"
        else 
            logs_to_events "AKS.testingTraffic.failure" "echo '$(date) - ERROR: Failed to test $APISERVER_FQDN with returned status code $res. Node might not be able to connect to the apiserver'" 
        fi
    else
        logs_to_events "AKS.testingTraffic.failure" "echo '$(date) - ERROR: Failed to test DNS resolution to $APISERVER_FQDN. Node might not be able to connect to the apiserver'"
        dns_trace $APISERVER_FQDN
    fi
fi

# check access to required endpoints
for url in "${!URL_LIST[@]}"
do
    check_and_curl $url "${URL_LIST[$url]}"
done

# check access to additional endpoints
if [ ! -z "$CUSTOM_ENDPOINT" ]; then
    echo "Checking additional endpoints ..."  
    extra_urls=($(echo $CUSTOM_ENDPOINT | tr "," "\n"))
    for url in "${extra_urls[@]}"
    do
        check_and_curl $url ""
    done
fi

logs_to_events "AKS.testingTraffic.end" "echo '$(date) - INFO: Network connectivity check completed'"