#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

debug() {
    local retval
    retval=0
    mkdir -p $SCENARIO_NAME-logs
    INSTANCE_ID="$(az vmss list-instances --name $VMSS_NAME -g $MC_RESOURCE_GROUP_NAME | jq -r '.[0].instanceId')"
    PRIVATE_IP="$(az vmss nic list-vm-nics --vmss-name $VMSS_NAME -g $MC_RESOURCE_GROUP_NAME --instance-id $INSTANCE_ID | jq -r .[0].ipConfigurations[0].privateIpAddress)"
    set +x
    SSH_KEY=$(cat ~/.ssh/id_rsa)
    SSH_OPTS="-o PasswordAuthentication=no -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o ConnectTimeout=5"
    SSH_CMD="echo '$SSH_KEY' > sshkey && chmod 0600 sshkey && ssh -i sshkey $SSH_OPTS azureuser@$PRIVATE_IP sudo"
    exec_on_host "$SSH_CMD cat /var/log/azure/cluster-provision.log" $SCENARIO_NAME-logs/cluster-provision.log || retval=$?
    if [ "$retval" != "0" ]; then
        echo "failed cat cluster-provision"
    fi
    exec_on_host "$SSH_CMD cat /var/log/azure/cluster-provision-cse-output.log" $SCENARIO_NAME-logs/provision-cse-output.log || retval=$?
    if [ "$retval" != "0" ]; then
        echo "failed cat provision-cse-output"
    fi
    exec_on_host "$SSH_CMD systemctl status kubelet" $SCENARIO_NAME-logs/kubelet-status.txt  || retval=$?
    if [ "$retval" != "0" ]; then
        echo "failed systemctl status kubelet"
    fi
    exec_on_host "$SSH_CMD journalctl -u kubelet -r | head -n 500" $SCENARIO_NAME-logs/kubelet.log  || retval=$?
    if [ "$retval" != "0" ]; then
        echo "failed journalctl -u kubelet"
    fi
    exec_on_host "$SSH_CMD cat /var/log/syslog" $SCENARIO_NAME-logs/syslog || retval=$?
    if [ "$retval" != "0" ]; then
        echo "failed cat syslog"
    fi
    set -x
    echo "debug done"
}

KUBECONFIG=$(pwd)/kubeconfig
export KUBECONFIG
kubectl rollout status deploy/debug

echo "Scenario is $SCENARIO_NAME"
jq -s '.[0] * .[1]' nodebootstrapping_config.json scenarios/$SCENARIO_NAME/property-$SCENARIO_NAME.json > scenarios/$SCENARIO_NAME/nbc-$SCENARIO_NAME.json

go test -run TestE2EBasic

set +x
if [ ! -f ~/.ssh/id_rsa ]; then
    ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa -N ""
fi
set -x

msiResourceID=$(jq -r '.identityProfile.kubeletidentity.resourceId' < cluster_info.json)
MC_RESOURCE_GROUP_NAME="MC_${RESOURCE_GROUP_NAME}_${CLUSTER_NAME}_eastus"

VMSS_NAME="$(mktemp -u abtest-XXXXXXX | tr '[:upper:]' '[:lower:]')"
tee $SCENARIO_NAME-vmss.json > /dev/null <<EOF
{
    "group": "${MC_RESOURCE_GROUP_NAME}",
    "vmss": "${VMSS_NAME}"
}
EOF

cat $SCENARIO_NAME-vmss.json

# Create a test VMSS with 1 instance 
# TODO 3: Discuss about the --image version, probably go with aks-ubuntu-1804-gen2-2021-q2:latest
#       However, how to incorporate chaning quarters?
log "Creating VMSS"
vmssStartTime=$(date +%s)
az vmss create -n ${VMSS_NAME} \
    -g $MC_RESOURCE_GROUP_NAME \
    --admin-username azureuser \
    --custom-data scenarios/$SCENARIO_NAME/$SCENARIO_NAME-cloud-init.txt \
    --lb kubernetes --backend-pool-name aksOutboundBackendPool \
    --vm-sku $VM_SKU \
    --instance-count 1 \
    --assign-identity $msiResourceID \
    --image "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/1804Gen2/versions/1.1666631350.18026" \
    --upgrade-policy-mode Automatic \
    --ssh-key-values ~/.ssh/id_rsa.pub \
    -ojson

vmssEndTime=$(date +%s)
log "Created VMSS in $((vmssEndTime-vmssStartTime)) seconds"

# Get the name of the VM instance to later check with kubectl get nodes
vmInstanceName=$(az vmss list-instances \
                -n ${VMSS_NAME} \
                -g $MC_RESOURCE_GROUP_NAME \
                -ojson | \
                jq -r '.[].osProfile.computerName'
            )
export vmInstanceName

# Generate the extension from csecmd
jq -Rs '{commandToExecute: . }' scenarios/$SCENARIO_NAME/$SCENARIO_NAME-cseCmd > scenarios/$SCENARIO_NAME/$SCENARIO_NAME-settings.json

# Apply extension to the VM
log "Applying extensions to VMSS"
vmssExtStartTime=$(date +%s)
set +e
az vmss extension set --resource-group $MC_RESOURCE_GROUP_NAME \
    --name CustomScript \
    --vmss-name ${VMSS_NAME} \
    --publisher Microsoft.Azure.Extensions \
    --protected-settings scenarios/$SCENARIO_NAME/$SCENARIO_NAME-settings.json \
    --version 2.0 \
    -ojson
retval=$?
set -e

vmssExtEndTime=$(date +%s)
log "Applied extensions in $((vmssExtEndTime-vmssExtStartTime)) seconds"

FAILED=0
# Check if the node joined the cluster
if [[ "$retval" != "0" ]]; then
    err "cse failed to apply"
    debug
    tail -n 50 $SCENARIO_NAME-logs/cluster-provision.log || true
    exit 1
fi

# Sleep to let the automatic upgrade of the VM finish
waitForNodeStartTime=$(date +%s)
for i in $(seq 1 10); do
    set +e
    kubectl get nodes | grep $vmInstanceName
    # pipefail interferes with conditional.
    # shellcheck disable=SC2143
    if [ -z "$(kubectl get nodes | grep $vmInstanceName | grep -v "NotReady")" ]; then
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

debug
tail -n 50 $SCENARIO_NAME-logs/cluster-provision.log || true

if [ "$FAILED" == "1" ]; then
    echo "node join failed, dumping logs for debug"
    head -n 500 $SCENARIO_NAME-logs/kubelet.log || true
    cat $SCENARIO_NAME-logs/kubelet-status.txt || true
    exit 1
fi

# Run a nginx pod on the node to check if pod runs
podName=$(mktemp -u podName-XXXXXXX | tr '[:upper:]' '[:lower:]')
export podName
envsubst < pod-nginx-template.yaml > pod-nginx.yaml
sleep 5
kubectl apply -f pod-nginx.yaml

# Sleep to let Pod Status=Running
waitForPodStartTime=$(date +%s)
for i in $(seq 1 10); do
    set +e
    kubectl get pods -o wide | grep $podName
    kubectl get pods -o wide | grep $podName | grep 'Running'
    retval=$?
    set -e
    if [ "$retval" -ne 0 ]; then
        log "retrying attempt $i"
        sleep 10
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
    kubectl get pods -o wide | grep $podName
    kubectl describe pod $podName
    exit 1
fi

waitForDeleteStartTime=$(date +%s)

kubectl delete node $vmInstanceName

waitForDeleteEndTime=$(date +%s)
log "Waited $((waitForDeleteEndTime-waitForDeleteStartTime)) seconds to delete VMSS and node"
