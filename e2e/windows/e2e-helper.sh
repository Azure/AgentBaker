#!/bin/bash

log() {
    printf "\\033[1;33m%s\\033[0m\\n" "$*"
}

ok() {
    printf "\\033[1;32m%s\\033[0m\\n" "$*"
}

err() {
    printf "\\033[1;31m%s\\033[0m\\n" "$*"
}

exec_on_host() {
    kubectl exec $(kubectl get pod -l app=debug -o jsonpath="{.items[0].metadata.name}") -- bash -c "nsenter -t 1 -m bash -c \"$1\"" > $2
}

create_storage_container() {
    set +x
    # check if the storage container exists and create one if not
    exists=$(az storage container exists --account-name $STORAGE_ACCOUNT_NAME --account-key $MAPPED_ACCOUNT_KEY --name $WINDOWS_E2E_STORAGE_CONTAINER)
    if [[ $exists == *false* ]]; then
        az storage container create -n $WINDOWS_E2E_STORAGE_CONTAINER --account-name $STORAGE_ACCOUNT_NAME --account-key $MAPPED_ACCOUNT_KEY
        echo "Created storage container $WINDOWS_E2E_STORAGE_CONTAINER in $STORAGE_ACCOUNT_NAME"
    else
        # check if the storage container is empty and delete the blobs within one by one if not
        blob_list=$(az storage blob list --account-name $STORAGE_ACCOUNT_NAME --container-name $WINDOWS_E2E_STORAGE_CONTAINER --account-key $MAPPED_ACCOUNT_KEY -o json | jq -r '.[] | .name')
        if [[ -n $blob_list ]]; then
            for blob in $blob_list; do
                az storage blob delete --account-name $STORAGE_ACCOUNT_NAME --container-name $WINDOWS_E2E_STORAGE_CONTAINER --account-key $MAPPED_ACCOUNT_KEY --name $blob
                echo "Deleted blob $blob from storage container $WINDOWS_E2E_STORAGE_CONTAINER in the storage account $STORAGE_ACCOUNT_NAME"
            done
        fi
    fi
    set -x
}

upload_linux_file_to_storage_account() {
    local retval=0
    MC_RESOURCE_GROUP_NAME="MC_${RESOURCE_GROUP_NAME}_${CLUSTER_NAME}_$LOCATION"
    MC_VMSS_NAME=$(az vmss list -g $MC_RESOURCE_GROUP_NAME --query "[?contains(name, 'nodepool')]" -ojson | jq -r '.[0].name')
    VMSS_INSTANCE_ID="$(az vmss list-instances --name $MC_VMSS_NAME -g $MC_RESOURCE_GROUP_NAME | jq -r '.[0].instanceId')"

    set +x
    expiryTime=$(date --date="2 day" +%Y-%m-%d)
    token=$(az storage container generate-sas --account-name $STORAGE_ACCOUNT_NAME --account-key $MAPPED_ACCOUNT_KEY --permissions 'w' --expiry $expiryTime --name $WINDOWS_E2E_STORAGE_CONTAINER)
    linuxFileURL="https://${STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${WINDOWS_E2E_STORAGE_CONTAINER}/${MC_VMSS_NAME}-linux-file.zip?${token}"

    az vmss run-command invoke --command-id RunShellScript \
        --resource-group $MC_RESOURCE_GROUP_NAME \
        --name $MC_VMSS_NAME \
        --instance-id $VMSS_INSTANCE_ID \
        --scripts "cat /etc/kubernetes/azure.json > /home/fields.json; cat /etc/kubernetes/certs/apiserver.crt | base64 -w 0 > /home/apiserver.crt; cat /etc/kubernetes/certs/ca.crt | base64 -w 0 > /home/ca.crt; cat /etc/kubernetes/certs/client.key | base64 -w 0 > /home/client.key; cat /var/lib/kubelet/bootstrap-kubeconfig > /home/bootstrap-kubeconfig; cd /home; zip file.zip fields.json apiserver.crt ca.crt client.key bootstrap-kubeconfig; wget https://aka.ms/downloadazcopy-v10-linux; tar -xvf downloadazcopy-v10-linux; cd ./azcopy_*; ./azcopy copy /home/file.zip $linuxFileURL" || retval=$?
    
    set -x
    if [ "$retval" -eq 0 ]; then
        log "Upload linux file successfully"
    else
        err "Failed to upload linux file. Error code is $retval."
    fi
}

download_linux_file_from_storage_account() {
    wget https://aka.ms/downloadazcopy-v10-linux
    tar -xvf downloadazcopy-v10-linux

    expiryTime=$(date --date="2 day" +%Y-%m-%d)

    set +x

    token=$(az storage container generate-sas --account-name $STORAGE_ACCOUNT_NAME --account-key $MAPPED_ACCOUNT_KEY --permissions 'rl' --expiry $expiryTime --name $WINDOWS_E2E_STORAGE_CONTAINER)
    tokenWithoutQuote=${token//\"}
    linuxFileURL="https://${STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${WINDOWS_E2E_STORAGE_CONTAINER}/${MC_VMSS_NAME}-linux-file.zip?${tokenWithoutQuote}"

    array=(azcopy_*)
    noExistStr="File count: 0"
    local fileExist="false"
    for i in $(seq 1 20); do
        listResult=$(${array[0]}/azcopy list $linuxFileURL --running-tally)
        if [[ "$listResult" == *"$noExistStr"* ]]; then
            log "Linux file has not been uploaded, waiting..."
            sleep 10
            continue
        fi
        fileExist="true"
        break;
    done
    set -x

    if [ "$fileExist" == "false" ]; then
        err "File does not exist in storage account."
        exit 1
    fi

    set +x
    ${array[0]}/azcopy copy $linuxFileURL file.zip
    set -x

    unzip file.zip
}

addJsonToFile() {
    k=$1; v=$2
    jq -r --arg key $k --arg value $v '. + { ($key) : $value}' < fields.json > dummy.json && mv dummy.json fields.json
}

getAgentPoolProfileValues() {
    declare -a properties=("mode" "name" "nodeImageVersion")

    for property in "${properties[@]}"; do
        value=$(jq -r .agentPoolProfiles[].${property} < cluster_info.json)
        addJsonToFile $property $value
    done
}

getFQDN() {
    fqdn=$(jq -r '.fqdn' < cluster_info.json)
    addJsonToFile "fqdn" $fqdn
}

getMSIResourceID() {
    msiResourceID=$(jq -r '.identityProfile.kubeletidentity.resourceId' < cluster_info.json)
    addJsonToFile "msiResourceID" $msiResourceID
}

getTenantID() {
    tenantID=$(jq -r '.identity.tenantId' < cluster_info.json)
    addJsonToFile "tenantID" $tenantID
}

cleanupOutdatedFiles() {
    # delete blobs created 1 day ago
    EXPIRATION_IN_HOURS=24
    (( expirationInSecs = ${EXPIRATION_IN_HOURS} * 60 * 60 ))
    (( deadline=$(date +%s)-${expirationInSecs%.*} ))
    dateOfdeadline=$(date -d @${deadline} +"%Y-%m-%dT%H:%M:%S+00:00")

    # two containers need to be cleaned up now
    CONTAINER_LIST=("cselogs" "csepackages")

    for CONTAINER_NAME in "${CONTAINER_LIST[@]}"
    do 
        result=$(az storage blob list -c $CONTAINER_NAME --account-name $STORAGE_ACCOUNT_NAME --account-key $MAPPED_ACCOUNT_KEY -o json \
        | jq -r --arg time "$dateOfdeadline" '.[] | select(.properties.creationTime < $time)' \
        | jq -r '.name')

        for item in $result
        do
            az storage blob delete -c $CONTAINER_NAME --account-name $STORAGE_ACCOUNT_NAME --account-key $MAPPED_ACCOUNT_KEY -n $item
            echo "Deleted $item in $CONTAINER_NAME"
        done
    done
}