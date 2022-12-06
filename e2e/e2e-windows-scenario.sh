#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

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
VMSS_INSTANCE_NAME=$(az vmss list-instances \
                    -n ${DEPLOYMENT_VMSS_NAME} \
                    -g $MC_RESOURCE_GROUP_NAME \
                    -ojson | \
                    jq -r '.[].osProfile.computerName')
VMSS_INSTANCE_ID=$(az vmss list-instances \ 
                    -n ${DEPLOYMENT_VMSS_NAME} \
                    -g $MC_RESOURCE_GROUP_NAME \
                    -OJSON | \
                    JQ -R '.[].instanceId'
                )
export DEPLOYMENT_VMSS_NAME
export DEPLOYMENT_VMSS_INSTANCE_ID

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
    kubectl get nodes -o wide | grep $vmInstanceName
else
    err "Node did not join cluster"
    FAILED=1
fi
