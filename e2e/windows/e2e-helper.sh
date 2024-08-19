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

backfill_clean_storage_container() {
    set +x
    # Get supported kubernetes versions
    versions=$(az aks get-versions --location $AZURE_BUILD_LOCATION -o table | tail -n +3 | awk '{print $1}')
    k8s_versions=${versions//./}

    # Get container names e.g. akswinstore2022-1256 (for this $container_version would be "1256")
    container_names=$(az storage container list --account-name $AZURE_E2E_STORAGE_ACCOUNT_NAME --auth-mode "login" --query "[?starts_with(name, 'akswinstore')].name" -o tsv)
    # Check if the container version is still supported and delete the container if not
    for container_name in $container_names; do
        container_version=$(echo $container_name | cut -d '-' -f 2- | awk '{print tolower($0)}')
        echo "container version is $container_version"
        if [[ $k8s_versions == *"$container_version"* ]]; then
            echo "The version $container_version is available in the $AZURE_BUILD_LOCATION region."
        else
            echo "The version $container_version is not available in the $AZURE_BUILD_LOCATION region."
            echo "Deleting the container."
            az storage container delete --name $container_name --account-name $AZURE_E2E_STORAGE_ACCOUNT_NAME --auth-mode "login"
            echo "Deletion completed."
        fi
    done

    set -x
}

create_storage_container() {
    set +x

    # check if the storage container exists and create one if not
    exists=$(az storage container exists --account-name $AZURE_E2E_STORAGE_ACCOUNT_NAME --auth-mode "login" --name $WINDOWS_E2E_STORAGE_CONTAINER)
    if [[ $exists == *false* ]]; then
        az storage container create -n $WINDOWS_E2E_STORAGE_CONTAINER --account-name $AZURE_E2E_STORAGE_ACCOUNT_NAME --auth-mode "login"
        echo "Created storage container $WINDOWS_E2E_STORAGE_CONTAINER in $AZURE_E2E_STORAGE_ACCOUNT_NAME"
    else
        # check if the storage container is empty and delete the blobs within one by one if not
        blob_list=$(az storage blob list --account-name $AZURE_E2E_STORAGE_ACCOUNT_NAME --container-name $WINDOWS_E2E_STORAGE_CONTAINER --auth-mode "login" -o json | jq -r '.[] | .name')
        if [[ -n $blob_list ]]; then
            for blob in $blob_list; do
                az storage blob delete --account-name $AZURE_E2E_STORAGE_ACCOUNT_NAME --container-name $WINDOWS_E2E_STORAGE_CONTAINER --auth-mode "login" --name $blob
                echo "Deleted blob $blob from storage container $WINDOWS_E2E_STORAGE_CONTAINER in the storage account $AZURE_E2E_STORAGE_ACCOUNT_NAME"
            done
        fi
    fi
    set -x
}

upload_linux_file_to_storage_account() {
    local retval=0
    declare -l E2E_RESOURCE_GROUP_NAME="$AZURE_E2E_RESOURCE_GROUP_NAME-$WINDOWS_E2E_IMAGE$WINDOWS_GPU_DRIVER_SUFFIX-$K8S_VERSION"
    E2E_MC_RESOURCE_GROUP_NAME="MC_${E2E_RESOURCE_GROUP_NAME}_${AZURE_E2E_CLUSTER_NAME}_$AZURE_BUILD_LOCATION"
    MC_VMSS_NAME=$(az vmss list -g $E2E_MC_RESOURCE_GROUP_NAME --query "[?contains(name, 'nodepool')]" -ojson | jq -r '.[0].name')
    VMSS_INSTANCE_ID="$(az vmss list-instances --name $MC_VMSS_NAME -g $E2E_MC_RESOURCE_GROUP_NAME | jq -r '.[0].instanceId')"
    
    linuxFileURL="https://${AZURE_E2E_STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${WINDOWS_E2E_STORAGE_CONTAINER}/${MC_VMSS_NAME}-linux-file.zip"

    az vmss run-command invoke --command-id RunShellScript \
        --resource-group $E2E_MC_RESOURCE_GROUP_NAME \
        --name $MC_VMSS_NAME \
        --instance-id $VMSS_INSTANCE_ID \
        --scripts "cat /etc/kubernetes/azure.json > /home/fields.json; cat /etc/kubernetes/certs/apiserver.crt | base64 -w 0 > /home/apiserver.crt; cat /etc/kubernetes/certs/ca.crt | base64 -w 0 > /home/ca.crt; cat /etc/kubernetes/certs/client.key | base64 -w 0 > /home/client.key; cat /var/lib/kubelet/bootstrap-kubeconfig > /home/bootstrap-kubeconfig; cd /home; zip file.zip fields.json apiserver.crt ca.crt client.key bootstrap-kubeconfig; wget https://aka.ms/downloadazcopy-v10-linux; tar -xvf downloadazcopy-v10-linux; cd ./azcopy_*; export AZCOPY_AUTO_LOGIN_TYPE=\"MSI\"; export AZCOPY_MSI_RESOURCE_STRING=\"${AZURE_MSI_RESOURCE_STRING}\"; ./azcopy copy /home/file.zip $linuxFileURL" || retval=$?
    
    if [ "$retval" -eq 0 ]; then
        log "Upload linux file successfully"
    else
        err "Failed to upload linux file. Error code is $retval."
        exit 1
    fi
}

download_linux_file_from_storage_account() {
    local retval
    retval=0
    if [[ "$(check_linux_file_exists_in_storage_account)" == *"Linux file already exists in storage account."* ]]; then
        linuxFileURL="https://${AZURE_E2E_STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${WINDOWS_E2E_STORAGE_CONTAINER}/${MC_VMSS_NAME}-linux-file.zip"
        az storage blob download --auth-mode login --blob-url $linuxFileURL --file file.zip || retval=$?
        if [ "$retval" -ne 0 ]; then
            err "Failed in downloading linux files. Error code is $retval."
            exit 1
        fi
        unzip file.zip
    else
        exit 1
    fi
}

check_linux_file_exists_in_storage_account() {
    linuxFileURL="https://${AZURE_E2E_STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${WINDOWS_E2E_STORAGE_CONTAINER}/${MC_VMSS_NAME}-linux-file.zip"
    local fileExist="false"
    for i in $(seq 1 20); do
        isRemoteLinuxFileExist=$(az storage blob exists --auth-mode login --blob-url $linuxFileURL)
        if [[ "$isRemoteLinuxFileExist" == "false" ]]; then
            log "Linux file has not been uploaded, waiting..."
            sleep 10
            continue
        fi
        fileExist="true"
        break;
    done

    if [ "$fileExist" == "false" ]; then
        err "Linux file does not exist in storage account."
        return
    fi

    log "Linux file already exists in storage account."
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
    CONTAINER_LIST=("cselogs" "\$web")

    for CONTAINER_NAME in "${CONTAINER_LIST[@]}"
    do 
        result=$(az storage blob list -c $CONTAINER_NAME --account-name $AZURE_E2E_STORAGE_ACCOUNT_NAME --auth-mode "login" -o json \
        | jq -r --arg time "$dateOfdeadline" '.[] | select(.properties.creationTime < $time)' \
        | jq -r '.name')

        for item in $result
        do
            az storage blob delete -c $CONTAINER_NAME --account-name $AZURE_E2E_STORAGE_ACCOUNT_NAME --auth-mode "login" -n $item
            echo "Deleted $item in $CONTAINER_NAME"
        done
    done
}