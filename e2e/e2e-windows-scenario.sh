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
    VMSS_INSTANCE_ID="$(az vmss list-instances --name $DEPLOYMENT_VMSS_NAME -g $MC_RESOURCE_GROUP_NAME | jq -r '.[0].instanceId')"
    set +x
    expiryTime=$(date --date="2 day" +%Y-%m-%d)
    token=$(az storage container generate-sas --account-name $STORAGE_ACCOUNT_NAME --account-key $MAPPED_ACCOUNT_KEY --permissions 'rwacdl' --expiry $expiryTime --name $STORAGE_LOG_CONTAINER --https-only)
    # Use .ps1 file to run scripts since single quotes of parameters for --scripts would fail in check-shell
    az vmss run-command invoke --command-id RunPowerShellScript \
        --resource-group $MC_RESOURCE_GROUP_NAME \
        --name $DEPLOYMENT_VMSS_NAME \
        --instance-id $VMSS_INSTANCE_ID \
        --scripts @upload-cse-logs.ps1 \
        --parameters arg1=$STORAGE_ACCOUNT_NAME arg2=$STORAGE_LOG_CONTAINER arg3=$DEPLOYMENT_VMSS_NAME arg4=$token
    if [ "$retval" != "0" ]; then
        echo "failed in uploading cse logs"
    fi

    tokenWithoutQuote=${token//\"}
    # use array to pass shellcheck
    array=(azcopy_*)
    ${array[0]}/azcopy copy "https://${STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${STORAGE_LOG_CONTAINER}/${DEPLOYMENT_VMSS_NAME}-cse.log?${tokenWithoutQuote}" $SCENARIO_NAME-logs/CustomDataSetupScript.log
    if [ "$retval" != "0" ]; then
        echo "failed in downloading cse logs"
    else
        ${array[0]}/azcopy rm "https://${STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${STORAGE_LOG_CONTAINER}/${DEPLOYMENT_VMSS_NAME}-cse.log?${tokenWithoutQuote}"
        if [ "$retval" != "0" ]; then
            echo "failed in deleting cse logs in remote storage"
        fi
    fi
    set -x

    echo "collect cse logs done"
}

RESOURCE_GROUP_NAME="$RESOURCE_GROUP_NAME"-"$WINDOWS_E2E_IMAGE"

DEPLOYMENT_VMSS_NAME="$(mktemp -u winXXXXX | tr '[:upper:]' '[:lower:]')"
export DEPLOYMENT_VMSS_NAME

# download azcopy
wget https://aka.ms/downloadazcopy-v10-linux
tar -xvf downloadazcopy-v10-linux

# zip and upload cse package
timeStamp=$(date +%s)
cd ../staging/cse/windows
zip -r ../../../e2e/${timeStamp}-${DEPLOYMENT_VMSS_NAME}-aks-windows-cse-scripts.zip ./* -x ./*.tests.ps1 -x "*azurecnifunc.tests.suites*" -x README -x provisioningscripts/*.md -x debug/update-scripts.ps1
echo "zip cse packages done"

set +x
expiryTime=$(date --date="2 day" +%Y-%m-%d)
token=$(az storage container generate-sas --account-name $STORAGE_ACCOUNT_NAME --account-key $MAPPED_ACCOUNT_KEY --permissions 'rwacdl' --expiry $expiryTime --name $STORAGE_PACKAGE_CONTAINER)
tokenWithoutQuote=${token//\"}

csePackageURL="https://${STORAGE_ACCOUNT_NAME}.blob.core.windows.net/${STORAGE_PACKAGE_CONTAINER}/${timeStamp}-${DEPLOYMENT_VMSS_NAME}-aks-windows-cse-scripts.zip?${tokenWithoutQuote}"
export csePackageURL

cd ../../../e2e

array=(azcopy_*)
noExistStr="File count: 0"
listResult=$(${array[0]}/azcopy list $csePackageURL --running-tally)

for i in $(seq 1 10); do
    if [[ "$listResult" != *"$noExistStr"* ]]; then
        echo "Cse package with the same exists, retry $i to use new name..."
        timeStamp=$(date +%s)
        listResult=$(${array[0]}/azcopy list $csePackageURL --running-tally)
        continue
    fi
    ${array[0]}/azcopy copy ${timeStamp}-${DEPLOYMENT_VMSS_NAME}-aks-windows-cse-scripts.zip $csePackageURL
    break;
done

set -x

listResult=$(${array[0]}/azcopy list $csePackageURL --running-tally)
if [[ "$listResult" == *"$noExistStr"* ]]; then
    err "failed to upload cse package"
    exit 1
fi

echo "upload cse packages done"

envsubst < scenarios/$SCENARIO_NAME/property-$SCENARIO_NAME-template.json > scenarios/$SCENARIO_NAME/property-$SCENARIO_NAME.json

set +x
WINDOWS_PASSWORD=$({
    choose '0123456789'
    choose 'abcdefghijklmnopqrstuvwxyz'
    choose 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'
    for i in $(seq 1 16)
    do
        choose '#*0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ'
    done
} | sort -R | awk '{printf "%s", $1}')
set -x
echo $WINDOWS_PASSWORD

MC_RESOURCE_GROUP_NAME="MC_${RESOURCE_GROUP_NAME}_${CLUSTER_NAME}_eastus"

KUBECONFIG=$(pwd)/kubeconfig
export KUBECONFIG

clientCertificate=$(grep "client-certificate-data" $KUBECONFIG | awk '{print $2}')
kubectl rollout status deploy/debug

tee $SCENARIO_NAME-vmss.json > /dev/null <<EOF
{
    "group": "${MC_RESOURCE_GROUP_NAME}",
    "vmss": "${DEPLOYMENT_VMSS_NAME}"
}
EOF

echo "Scenario is $SCENARIO_NAME"
jq --arg clientCrt "$clientCertificate" --arg vmssName $DEPLOYMENT_VMSS_NAME 'del(.KubeletConfig."--pod-manifest-path") | del(.KubeletConfig."--pod-max-pids") | del(.KubeletConfig."--protect-kernel-defaults") | del(.KubeletConfig."--tls-cert-file") | del(.KubeletConfig."--tls-private-key-file") | .ContainerService.properties.certificateProfile += {"clientCertificate": $clientCrt} | .PrimaryScaleSetName=$vmssName' nodebootstrapping_config.json > nodebootstrapping_config_for_windows.json
jq -s '.[0] * .[1]' nodebootstrapping_config_for_windows.json scenarios/$SCENARIO_NAME/property-$SCENARIO_NAME.json > scenarios/$SCENARIO_NAME/nbc-$SCENARIO_NAME.json

go test -run TestE2EWindows

MC_WIN_VMSS_NAME=$(az vmss list -g $MC_RESOURCE_GROUP_NAME --query "[?contains(name, 'winnp')]" -ojson | jq -r '.[0].name')
VMSS_RESOURCE_Id=$(az resource show --resource-group $MC_RESOURCE_GROUP_NAME --name $MC_WIN_VMSS_NAME --resource-type Microsoft.Compute/virtualMachineScaleSets --query id --output tsv)

az group export --resource-group $MC_RESOURCE_GROUP_NAME --resource-ids $VMSS_RESOURCE_Id --include-parameter-default-value > test.json
# TODO: use prod image
# IMAGE_REFERENCE=$(jq -c '.resources[0].properties.virtualMachineProfile.storageProfile.imageReference.id' test.json)
IMAGE_REFERENCE="[concat(parameters('galleries_AKSWindows_externalid'), '/images/windows-e2e-test-$WINDOWS_E2E_IMAGE/versions/2023.02.07')]"
IMAGE_EXTERNALID="/subscriptions/$IMAGE_SUBSCRIPTION_ID/resourceGroups/akswinvhdbuilderrg/providers/Microsoft.Compute/galleries/AKSWindows"
WINDOWS_VNET=$(jq -c '.parameters | with_entries( select(.key|contains("vnet")))' test.json)
WINDOWS_LOADBALANCER=$(jq -c '.parameters | with_entries( select(.key|contains("loadBalancers")))' test.json)
WINDOWS_IDENTITY=$(jq -c '.resources[0] | with_entries( select(.key|contains("identity")))' test.json)
WINDOWS_SKU=$(jq -c '.resources[0] | with_entries( select(.key|contains("sku")))' test.json)
WINDOWS_OSDISK=$(jq -c '.resources[0].properties.virtualMachineProfile.storageProfile | with_entries( select(.key|contains("osDisk")))' test.json)
NETWORK_PROPERTIES=$(jq -c '.resources[0].properties.virtualMachineProfile.networkProfile.networkInterfaceConfigurations[0] | with_entries( select(.key|contains("properties")))' test.json)
CUSTOM_DATA=$(cat scenarios/$SCENARIO_NAME/$SCENARIO_NAME-cloud-init.txt)
CSE_CMD=$(cat scenarios/$SCENARIO_NAME/$SCENARIO_NAME-cseCmd)

jq --argjson JsonForVnet "$WINDOWS_VNET" \
    --argjson JsonForLB "$WINDOWS_LOADBALANCER" \
    --argjson JsonForIdentity "$WINDOWS_IDENTITY" \
    --argjson JsonForSKU "$WINDOWS_SKU" \
    --argjson JsonForNetwork "$NETWORK_PROPERTIES" \
    --argjson JsonForOSDisk "$WINDOWS_OSDISK" \
    --arg ValueForImageExternalID "$IMAGE_EXTERNALID" \
    --arg ValueForImageReference "$IMAGE_REFERENCE" \
    --arg ValueForAdminPassword "$WINDOWS_PASSWORD" \
    --arg ValueForCustomData "$CUSTOM_DATA" \
    --arg ValueForCSECmd "$CSE_CMD" \
    --arg ValueForVMSS "$DEPLOYMENT_VMSS_NAME" \
    '.parameters += $JsonForVnet | .parameters += $JsonForLB | .parameters.galleries_AKSWindows_externalid.defaultValue=$ValueForImageExternalID | .resources[0] += $JsonForIdentity | .resources[0] += $JsonForSKU | .resources[0].properties.virtualMachineProfile.storageProfile+=$JsonForOSDisk | .resources[0].properties.virtualMachineProfile.networkProfile.networkInterfaceConfigurations[0] += $JsonForNetwork | .resources[0].properties.virtualMachineProfile.storageProfile.imageReference.id=$ValueForImageReference | .resources[0].properties.virtualMachineProfile.osProfile.adminPassword=$ValueForAdminPassword | .resources[0].properties.virtualMachineProfile.osProfile.customData=$ValueForCustomData | .resources[0].properties.virtualMachineProfile.extensionProfile.extensions[0].properties.settings.commandToExecute=$ValueForCSECmd | .parameters.virtualMachineScaleSets_akswin30_name.defaultValue=$ValueForVMSS' \
    windows_vmss_template.json > $DEPLOYMENT_VMSS_NAME-deployment.json

set +e
az deployment group create --resource-group $MC_RESOURCE_GROUP_NAME \
         --template-file $DEPLOYMENT_VMSS_NAME-deployment.json
retval=$?
set -e

# delete cse package in storage account
${array[0]}/azcopy rm $csePackageURL
retval=$?

if [[ "$retval" != "0" ]]; then
    err "failed to delete cse package in storage account"
else
    echo "delete cse package in storage account done"
fi

log "Collect cse log"
collect-logs

if [[ "$retval" != "0" ]]; then
    err "failed to deploy windows vmss"
    exit 1
fi

cat $SCENARIO_NAME-vmss.json

VMSS_INSTANCE_NAME=$(az vmss list-instances \
                    -n ${DEPLOYMENT_VMSS_NAME} \
                    -g $MC_RESOURCE_GROUP_NAME \
                    -ojson | \
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
        log "retrying attempt $i"
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
        log "retrying attempt $i"
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
    err "Pod pending/not running"
    kubectl get pods -o wide | grep $POD_NAME
    kubectl describe pod $POD_NAME
    exit 1
fi

if [ "$FAILED" == "1" ] || [ "$retval" -eq 1 ]; then
    log "Reserve vmss and node for failed pipeline"
else
    waitForDeleteStartTime=$(date +%s)

    # Only delete node and vmss since we reuse the resource group and cluster now
    kubectl delete node $VMSS_INSTANCE_NAME
    az vmss delete -g $(jq -r .group $SCENARIO_NAME-vmss.json) -n $(jq -r .vmss $SCENARIO_NAME-vmss.json)

    waitForDeleteEndTime=$(date +%s)
    log "Waited $((waitForDeleteEndTime-waitForDeleteStartTime)) seconds to delete VMSS and node"   
fi