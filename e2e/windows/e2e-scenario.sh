#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

choose() {
    echo ${1:RANDOM%${#1}:1} $RANDOM;
}

collect-logs() {
    local retval
    retval=0
    mkdir -p $SCENARIO_NAME-logs
    VMSS_INSTANCE_ID="$(az vmss list-instances --name $DEPLOYMENT_VMSS_NAME -g $E2E_MC_RESOURCE_GROUP_NAME | jq -r '.[0].instanceId')"

    # Use .ps1 file to run scripts since single quotes of parameters for --scripts would fail in check-shell
    az vmss run-command invoke --command-id RunPowerShellScript \
        --resource-group $E2E_MC_RESOURCE_GROUP_NAME \
        --name $DEPLOYMENT_VMSS_NAME \
        --instance-id $VMSS_INSTANCE_ID \
        --scripts @upload-cse-logs.ps1 \
        --parameters arg1=$AZURE_E2E_STORAGE_ACCOUNT_NAME arg2=$AZURE_E2E_STORAGE_LOG_CONTAINER arg3=$DEPLOYMENT_VMSS_NAME arg4=$AZURE_MSI_RESOURCE_STRING || retval=$?
    if [ "$retval" -ne 0 ]; then
        err "Failed in uploading cse logs. Error code is $retval."
    fi

    az storage blob download --auth-mode login --blob-url "https://${AZURE_E2E_STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${AZURE_E2E_STORAGE_LOG_CONTAINER}/${DEPLOYMENT_VMSS_NAME}-cse.log" --file $SCENARIO_NAME-logs/$WINDOWS_E2E_IMAGE$WINDOWS_GPU_DRIVER_SUFFIX-CustomDataSetupScript.log || retval=$?
    if [ "$retval" -ne 0 ]; then
        err "Failed in downloading cse logs. Error code is $retval."
    else
        log "Collect cse logs done. Deleting the remote cse logs"
        az storage blob delete --auth-mode login --blob-url "https://${AZURE_E2E_STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${AZURE_E2E_STORAGE_LOG_CONTAINER}/${DEPLOYMENT_VMSS_NAME}-cse.log" || retval=$?
        if [ "$retval" -ne 0 ]; then
            err "Failed in deleting cse logs in remote storage. Error code is $retval."
        fi
    fi

    az storage blob download --auth-mode login --blob-url "https://${AZURE_E2E_STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${AZURE_E2E_STORAGE_LOG_CONTAINER}/${DEPLOYMENT_VMSS_NAME}-provision.complete" --file $SCENARIO_NAME-logs/$WINDOWS_E2E_IMAGE$WINDOWS_GPU_DRIVER_SUFFIX-provision.complete || retval=$?
    if [ "$retval" -ne 0 ]; then
        err "Failed in downloading provision.complete. Error code is $retval."
    else
        log "provision.complete is generated. Deleting the remote provision.complete"
        az storage blob delete --auth-mode login --blob-url "https://${AZURE_E2E_STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${AZURE_E2E_STORAGE_LOG_CONTAINER}/${DEPLOYMENT_VMSS_NAME}-provision.complete" || retval=$?
        if [ "$retval" -ne 0 ]; then
            err "Failed in deleting provision.complete in remote storage. Error code is $retval."
        fi
    fi

    az storage blob download --auth-mode login --blob-url "https://${AZURE_E2E_STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${AZURE_E2E_STORAGE_LOG_CONTAINER}/${DEPLOYMENT_VMSS_NAME}-collected-node-logs.zip" --file $SCENARIO_NAME-logs/$WINDOWS_E2E_IMAGE$WINDOWS_GPU_DRIVER_SUFFIX-collected-node-logs.zip || retval=$?
    if [ "$retval" -ne 0 ]; then
        err "Failed in downloading collected node logs. Error code is $retval."
    else
        log "collected-node-logs.zip is generated. Deleting the remote collected-node-logs.zip"
        az storage blob delete --auth-mode login --blob-url "https://${AZURE_E2E_STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${AZURE_E2E_STORAGE_LOG_CONTAINER}/${DEPLOYMENT_VMSS_NAME}-collected-node-logs.zip" || retval=$?
        if [ "$retval" -ne 0 ]; then
            err "Failed in deleting collected node logs in remote storage. Error code is $retval."
        fi
    fi
}

declare -l E2E_RESOURCE_GROUP_NAME="$AZURE_E2E_RESOURCE_GROUP_NAME-$WINDOWS_E2E_IMAGE$WINDOWS_GPU_DRIVER_SUFFIX-$K8S_VERSION"

DEPLOYMENT_VMSS_NAME="$(mktemp -u winXXXXX | tr '[:upper:]' '[:lower:]')"
export DEPLOYMENT_VMSS_NAME

# zip and upload cse package
cd ../staging/cse/windows
zip -r ../../../$WINDOWS_E2E_IMAGE/$WINDOWS_E2E_IMAGE-aks-windows-cse-scripts.zip ./* -x ./*.tests.ps1 -x "*azurecnifunc.tests.suites*" -x README -x provisioningscripts/*.md -x debug/update-scripts.ps1
log "Zip cse packages done"

cd ../../../$WINDOWS_E2E_IMAGE

for i in $(seq 0 10); do
    timeStamp=$(date +%s)
    csePackageName="${timeStamp}-${DEPLOYMENT_VMSS_NAME}-aks-windows-cse-scripts.zip"
    cseBlobUrlForUploading="https://${AZURE_E2E_STORAGE_ACCOUNT_NAME}.blob.core.windows.net/\$web/${AZURE_E2E_STORAGE_PACKAGE_CONTAINER}/${csePackageName}"

    # `az storage blob exists` returns {"exists": true} or {"exists": false} and returnss nothing when it fails in authenticaion
    #     ERROR: Operation returned an invalid status 'Server failed to authenticate the request. Please refer to the information in the www-authenticate header.'
    #     ErrorCode:NoAuthenticationInformation
    isRemoteCsePackageExist=$(az storage blob exists --auth-mode login --blob-url $cseBlobUrlForUploading | jq -r '.exists')
    if  [[ "$isRemoteCsePackageExist" == "false" ]]; then
        log "retry $i : Cse package with the same does not exist. Starting to upload cse package $cseBlobUrlForUploading"
        retval=0
        az storage blob upload --auth-mode login --file $WINDOWS_E2E_IMAGE-aks-windows-cse-scripts.zip --blob-url $cseBlobUrlForUploading || retval=$?
        if [ "$retval" -ne "0" ]; then
            retval=0
            err "retry $i : Failed to upload cse package"
            continue
        fi
        break;
    elif [[ "$isRemoteCsePackageExist" == "true" ]]; then
        log "retry $i : Cse package with the same exists. Generating a new name..."
        continue
    else
        err "Failed to check cse package existence. Please check whether the MSI has the required permission."
        exit 1
    fi
done

# Use website url to allow anonymous download
csePackageURL="${AZURE_E2E_STORAGE_CSE_PACKAGE_ENDPOINT}/${AZURE_E2E_STORAGE_PACKAGE_CONTAINER}/${csePackageName}"
export csePackageURL

log "Upload cse packages done"

log "Scenario is $SCENARIO_NAME"
log "Windows package version is $WINDOWS_PACKAGE_VERSION"

# Generate vmss cse deployment config for windows nodepool testing
export orchestratorVersion=$WINDOWS_PACKAGE_VERSION
envsubst < scenarios/$SCENARIO_NAME/property-$SCENARIO_NAME-template.json > scenarios/$SCENARIO_NAME/$WINDOWS_E2E_IMAGE-property-$SCENARIO_NAME.json

set +x
WINDOWS_PASSWORD=$({
    choose '0123456789'
    choose 'abcdefghijklmnopqrstuvwxyz'
    choose 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'
    choose '#@'
    for i in $(seq 1 16)
    do
        choose '#@0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ'
    done
} | sort -R | awk '{printf "%s", $1}')
set -x
echo $WINDOWS_PASSWORD

E2E_MC_RESOURCE_GROUP_NAME="MC_${E2E_RESOURCE_GROUP_NAME}_${AZURE_E2E_CLUSTER_NAME}_eastus"

KUBECONFIG=$(pwd)/kubeconfig
export KUBECONFIG

clientCertificate=$(grep "client-certificate-data" $KUBECONFIG | awk '{print $2}')

tee $SCENARIO_NAME-vmss.json > /dev/null <<EOF
{
    "group": "${E2E_MC_RESOURCE_GROUP_NAME}",
    "vmss": "${DEPLOYMENT_VMSS_NAME}"
}
EOF

# Removed the "network-plugin" tag o.w. kubelet error for 1.24.0+ contains "failed to parse kubelet flag: unknown flag: --network-plugin"
# "network-plugin" works for 1.23.15 and below (you won't see this parsing error in kubelet.err.log)
jq --arg clientCrt "$clientCertificate" --arg vmssName $DEPLOYMENT_VMSS_NAME 'del(.KubeletConfig."--pod-manifest-path") | del(.KubeletConfig."--pod-max-pids") | del(.KubeletConfig."--protect-kernel-defaults") | del(.KubeletConfig."--tls-cert-file") | del(.KubeletConfig."--tls-private-key-file") | del(.KubeletConfig."--network-plugin") | .ContainerService.properties.certificateProfile += {"clientCertificate": $clientCrt} | .PrimaryScaleSetName=$vmssName' nodebootstrapping_config.json > $WINDOWS_E2E_IMAGE-nodebootstrapping_config_for_windows.json
jq -s '.[0] * .[1]' $WINDOWS_E2E_IMAGE-nodebootstrapping_config_for_windows.json scenarios/$SCENARIO_NAME/$WINDOWS_E2E_IMAGE-property-$SCENARIO_NAME.json > scenarios/$SCENARIO_NAME/$WINDOWS_E2E_IMAGE-nbc-$SCENARIO_NAME.json

go test -tags bash_e2e -run TestE2EWindows

MC_WIN_VMSS_NAME=$(az vmss list -g $E2E_MC_RESOURCE_GROUP_NAME --query "[?contains(name, 'winnp')]" -ojson | jq -r '.[0].name')
VMSS_RESOURCE_Id=$(az resource show --resource-group $E2E_MC_RESOURCE_GROUP_NAME --name $MC_WIN_VMSS_NAME --resource-type Microsoft.Compute/virtualMachineScaleSets --query id --output tsv)

az group export --resource-group $E2E_MC_RESOURCE_GROUP_NAME --resource-ids $VMSS_RESOURCE_Id --include-parameter-default-value > test.json
IMAGE_REFERENCE="/subscriptions/$AZURE_E2E_IMAGE_SUBSCRIPTION_ID/resourceGroups/$AZURE_E2E_IMAGE_RESOURCE_GROUP_NAME/providers/Microsoft.Compute/galleries/$AZURE_E2E_IMAGE_GALLERY_NAME/images/windows-$WINDOWS_E2E_IMAGE/versions/latest"
WINDOWS_VNET=$(jq -c '.parameters | with_entries( select(.key|contains("vnet")))' test.json)
WINDOWS_LOADBALANCER=$(jq -c '.parameters | with_entries( select(.key|contains("loadBalancers")))' test.json)
WINDOWS_IDENTITY=$(jq -c '.resources[0] | with_entries( select(.key|contains("identity")))' test.json)
WINDOWS_SKU=$(jq -c '.resources[0] | with_entries( select(.key|contains("sku")))' test.json)
WINDOWS_OSDISK=$(jq -c '.resources[0].properties.virtualMachineProfile.storageProfile | with_entries( select(.key|contains("osDisk")))' test.json)
NETWORK_PROPERTIES=$(jq -c '.resources[0].properties.virtualMachineProfile.networkProfile.networkInterfaceConfigurations[0] | with_entries( select(.key|contains("properties")))' test.json)
CUSTOM_DATA=$(cat scenarios/$SCENARIO_NAME/$WINDOWS_E2E_IMAGE-$SCENARIO_NAME-cloud-init.txt)
CSE_CMD=$(cat scenarios/$SCENARIO_NAME/$WINDOWS_E2E_IMAGE-$SCENARIO_NAME-cseCmd)

jq --argjson JsonForVnet "$WINDOWS_VNET" \
    --argjson JsonForLB "$WINDOWS_LOADBALANCER" \
    --argjson JsonForIdentity "$WINDOWS_IDENTITY" \
    --argjson JsonForSKU "$WINDOWS_SKU" \
    --argjson JsonForNetwork "$NETWORK_PROPERTIES" \
    --argjson JsonForOSDisk "$WINDOWS_OSDISK" \
    --arg ValueForImageReference "$IMAGE_REFERENCE" \
    --arg ValueForAdminPassword "$WINDOWS_PASSWORD" \
    --arg ValueForCustomData "$CUSTOM_DATA" \
    --arg ValueForCSECmd "$CSE_CMD" \
    --arg ValueForVMSS "$DEPLOYMENT_VMSS_NAME" \
    '.parameters += $JsonForVnet | .parameters += $JsonForLB | .resources[0] += $JsonForIdentity | .resources[0] += $JsonForSKU | .resources[0].properties.virtualMachineProfile.storageProfile+=$JsonForOSDisk | .resources[0].properties.virtualMachineProfile.networkProfile.networkInterfaceConfigurations[0] += $JsonForNetwork | .resources[0].properties.virtualMachineProfile.storageProfile.imageReference.id=$ValueForImageReference | .resources[0].properties.virtualMachineProfile.osProfile.adminPassword=$ValueForAdminPassword | .resources[0].properties.virtualMachineProfile.osProfile.customData=$ValueForCustomData | .resources[0].properties.virtualMachineProfile.extensionProfile.extensions[0].properties.settings.commandToExecute=$ValueForCSECmd | .parameters.virtualMachineScaleSets_akswin30_name.defaultValue=$ValueForVMSS' \
    windows_vmss_template.json > $DEPLOYMENT_VMSS_NAME-deployment.json

retval=0
set +e
az deployment group create --resource-group $E2E_MC_RESOURCE_GROUP_NAME \
         --template-file $DEPLOYMENT_VMSS_NAME-deployment.json || retval=$?
set -e
log "Deployment of windows vmss succeeded."

# delete cse package in storage account
retnval=0
az storage blob delete --auth-mode login --blob-url $cseBlobUrlForUploading || retnval=$?

if [ "$retnval" -ne 0 ]; then
    err "Failed to delete cse package in storage account. Error code is $retnval."
else
    log "Delete cse package in storage account done"
fi

if [ "$retval" -ne 0 ]; then
    err "Failed to deploy windows vmss. Error code is $retval."
    exit 1
fi

log "Collect cse log"
collect-logs

cat $SCENARIO_NAME-vmss.json

VMSS=$(az vmss list-instances \
        -n ${DEPLOYMENT_VMSS_NAME} \
        -g $E2E_MC_RESOURCE_GROUP_NAME \
        -ojson)
VMSS_IMAGE_REFERNCE_ID=$(echo $VMSS | \
                    jq -r '.[].storageProfile.imageReference.id')
VMSS_IMAGE_EXACT_VERSION=$(echo $VMSS | \
                    jq -r '.[].storageProfile.imageReference.exactVersion')
log "imageReference.id is $VMSS_IMAGE_REFERNCE_ID"
log "imageReference.exactVersion is $VMSS_IMAGE_EXACT_VERSION"

VMSS_INSTANCE_NAME=$(echo $VMSS | \
                    jq -r '.[].osProfile.computerName')
export VMSS_INSTANCE_NAME

# Sleep to let the automatic upgrade of the VM finish
waitForNodeStartTime=$(date +%s)
for i in $(seq 1 10); do
    set +e
    kubectl get nodes | grep $VMSS_INSTANCE_NAME
    retval=$?
    # pipefail interferes with conditional.
    # shellcheck disable=SC2143
    if [ -z "$(kubectl get nodes | grep $VMSS_INSTANCE_NAME | grep -v "NotReady")" ]; then
        log "Retrying attempt $i"
        sleep 10
        continue
    fi
    break;
done
waitForNodeEndTime=$(date +%s)
log "Waited $((waitForNodeEndTime-waitForNodeStartTime)) seconds for node to join"

FAILED=0
# Check if the node joined the cluster
if [ "$retval" -eq 0 ]; then
    ok "Test succeeded, node joined the cluster"
    kubectl get nodes -o wide | grep $VMSS_INSTANCE_NAME
else
    err "Node did not join cluster"
    FAILED=1
fi

# Run a windows servercore pod on the node to check if pod runs
POD_NAME=$(mktemp -u podName-XXXXXXX | tr '[:upper:]' '[:lower:]')
export POD_NAME
if [[ "$WINDOWS_E2E_IMAGE" == *"2019"* ]]; then
    WINDOWS_POD_IMAGE="2019"
else
    WINDOWS_POD_IMAGE="2022"
fi
export WINDOWS_POD_IMAGE
envsubst < pod-windows-template.yaml > pod-windows.yaml

kubectl apply -f pod-windows.yaml

# Sleep to let Pod Status=Running
waitForPodStartTime=$(date +%s)
for i in $(seq 1 20); do
    set +e
    kubectl get pods -o wide | grep $POD_NAME
    kubectl get pods -o wide | grep $POD_NAME | grep 'Running'
    retval=$?
    set -e
    if [ "$retval" -ne 0 ]; then
        log "Retrying attempt $i"
        sleep 15
        continue
    fi
    break;
done
waitForPodEndTime=$(date +%s)
log "Waited $((waitForPodEndTime-waitForPodStartTime)) seconds for pod to come up"

if [ "$retval" -eq 0 ]; then
    ok "Pod ran successfully"
else
    err "Pod pending/not running. Error code is $retval."
    kubectl get pods -o wide | grep $POD_NAME
    kubectl describe pod $POD_NAME
    exit 1
fi

if [ "$FAILED" -ne 0  ] || [ "$retval" -ne 0 ]; then
    log "Reserve vmss and node for failed pipeline"
else
    waitForDeleteStartTime=$(date +%s)

    # Only delete node and vmss since we reuse the resource group and cluster now
    kubectl delete node $VMSS_INSTANCE_NAME
    az vmss delete -g $(jq -r .group $SCENARIO_NAME-vmss.json) -n $(jq -r .vmss $SCENARIO_NAME-vmss.json)

    waitForDeleteEndTime=$(date +%s)
    log "Waited $((waitForDeleteEndTime-waitForDeleteStartTime)) seconds to delete VMSS and node"   
fi