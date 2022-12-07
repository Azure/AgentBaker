#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

choose() {
    echo ${1:RANDOM%${#1}:1} $RANDOM;
}

set +x
WINDOWS_PASSWORD=$({
    choose '#*-+.;'
    choose '0123456789'
    choose 'abcdefghijklmnopqrstuvwxyz'
    choose 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'
    for i in $(seq 1 16)
    do
        choose '#*-+.;0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ'
    done
} | sort -R | awk '{printf "%s", $1}')
set -x
echo $WINDOWS_PASSWORD

KUBECONFIG=$(pwd)/kubeconfig
export KUBECONFIG
kubectl rollout status deploy/debug

echo "Scenario is $SCENARIO_NAME"
jq 'del(.KubeletConfig."--pod-manifest-path") | del(.KubeletConfig."--pod-max-pids") | del(.KubeletConfig."--protect-kernel-defaults") | del(.KubeletConfig."--tls-cert-file") | del(.KubeletConfig."--tls-private-key-file")' nodebootstrapping_config.json > nodebootstrapping_config_for_windows.json
jq -s '.[0] * .[1]' nodebootstrapping_config_for_windows.json scenarios/$SCENARIO_NAME/property-$SCENARIO_NAME.json > scenarios/$SCENARIO_NAME/nbc-$SCENARIO_NAME.json

go test -run TestE2EWindows

MC_RESOURCE_GROUP_NAME="MC_${RESOURCE_GROUP_NAME}_${CLUSTER_NAME}_eastus"
MC_WIN_VMSS_NAME=$(az vmss list -g $MC_RESOURCE_GROUP_NAME --query "[?contains(name, '$WINDOWS_NODEPOOL')]" -ojson | jq -r '.[0].name')
VMSS_RESOURCE_Id=$(az resource show --resource-group $MC_RESOURCE_GROUP_NAME --name $MC_WIN_VMSS_NAME --resource-type Microsoft.Compute/virtualMachineScaleSets --query id --output tsv)

az group export --resource-group $MC_RESOURCE_GROUP_NAME --resource-ids $VMSS_RESOURCE_Id --include-parameter-default-value > test.json
WINDOWS_VNET=$(jq -c '.parameters | with_entries( select(.key|contains("vnet")))' test.json)
WINDOWS_LOADBALANCER=$(jq -c '.parameters | with_entries( select(.key|contains("loadBalancers")))' test.json)
WINDOWS_IDENTITY=$(jq -c '.resources[0] | with_entries( select(.key|contains("identity")))' test.json)
NETWORK_PROPERTIES=$(jq -c '.resources[0].properties.virtualMachineProfile.networkProfile.networkInterfaceConfigurations[0] | with_entries( select(.key|contains("properties")))' test.json)
CUSTOM_DATA=$(cat scenarios/$SCENARIO_NAME/$SCENARIO_NAME-cloud-init.txt)
CSE_CMD=$(cat scenarios/$SCENARIO_NAME/$SCENARIO_NAME-cseCmd)

jq --argjson JsonForVnet "$WINDOWS_VNET" \
    --argjson JsonForLB "$WINDOWS_LOADBALANCER" \
    --argjson JsonForIdentity "$WINDOWS_IDENTITY" \
    --argjson JsonForNetwork "$NETWORK_PROPERTIES" \
    --arg ValueForAdminPassword "$WINDOWS_PASSWORD" \
    --arg ValueForCustomData "$CUSTOM_DATA" \
    --arg ValueForCSECmd "$CSE_CMD" \
    '.parameters += $JsonForVnet | .parameters += $JsonForLB | .resources[0] += $JsonForIdentity | .resources[0].properties.virtualMachineProfile.networkProfile.networkInterfaceConfigurations[0] += $JsonForNetwork | .resources[0].properties.virtualMachineProfile.osProfile.adminPassword=$ValueForAdminPassword | .resources[0].properties.virtualMachineProfile.osProfile.customData=$ValueForCustomData | .resources[0].properties.virtualMachineProfile.extensionProfile.extensions[0].properties.settings.commandToExecute=$ValueForCSECmd' \
    template.json > deployment.json

set +e
az deployment group create --resource-group $MC_RESOURCE_GROUP_NAME \
         --template-file deployment.json
retval=$?
set -e

DEPLOYMENT_VMSS_NAME="akswin30"

tee $SCENARIO_NAME-vmss.json > /dev/null <<EOF
{
    "group": "${MC_RESOURCE_GROUP_NAME}",
    "vmss": "${DEPLOYMENT_VMSS_NAME}"
}
EOF

cat $SCENARIO_NAME-vmss.json

VMSS_INSTANCE_NAME=$(az vmss list-instances \
                    -n ${DEPLOYMENT_VMSS_NAME} \
                    -g $MC_RESOURCE_GROUP_NAME \
                    -ojson | \
                    jq -r '.[].osProfile.computerName')
export DEPLOYMENT_VMSS_NAME
export VMSS_INSTANCE_NAME


# FAILED=0
# # Check if the node joined the cluster
# if [[ "$retval" != "0" ]]; then
#     err "cse failed to apply"
#     debug
#     tail -n 50 $SCENARIO_NAME-logs/cluster-provision.log || true
#     exit 1
# fi

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
if [[ "$retval" -eq 0 ]]; then
    ok "Test succeeded, node joined the cluster"
    kubectl get nodes -o wide | grep $VMSS_INSTANCE_NAME
else
    err "Node did not join cluster"
    FAILED=1
fi

# Run a windows servercore pod on the node to check if pod runs
POD_NAME=$(mktemp -u podName-XXXXXXX | tr '[:upper:]' '[:lower:]')
export POD_NAME
envsubst < pod-windows-template.yaml > pod-windows.yaml
sleep 5
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

if [[ "$retval" -eq 0 ]]; then
    ok "Pod ran successfully"
else
    err "Pod pending/not running"
    kubectl get pods -o wide | grep $POD_NAME
    kubectl describe pod $POD_NAME
    exit 1
fi

# debug
retval=0
mkdir -p $SCENARIO_NAME-logs
kubectl cp $POD_NAME:AzureData/CustomDataSetupScript.log $SCENARIO_NAME-logs/CustomDataSetupScript.log
kubectl cp $POD_NAME:AzureData/CustomDataSetupScript.ps1 $SCENARIO_NAME-logs/CustomDataSetupScript.ps1

waitForDeleteStartTime=$(date +%s)

kubectl delete node $VMSS_INSTANCE_NAME

waitForDeleteEndTime=$(date +%s)
log "Waited $((waitForDeleteEndTime-waitForDeleteStartTime)) seconds to delete VMSS and node"   