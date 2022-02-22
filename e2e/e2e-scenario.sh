#!/bin/bash

set -euxo pipefail

source e2e-helper.sh

#kubectl apply -f deploy.yaml
kubectl rollout status deploy/debug

echo "scenario is $SCENARIO_NAME"
jq -s '.[0] * .[1]' nodebootstrapping_config.json scenarios/$SCENARIO_NAME/property-$SCENARIO_NAME.json > scenarios/$SCENARIO_NAME/nbc-$SCENARIO_NAME.json

go test -run TestE2EBasic

set +x
if [ ! -f ~/.ssh/id_rsa ]; then
    ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa -N ""
fi
set -x

msiResourceID=$(jq -r '.identityProfile.kubeletidentity.resourceId' < cluster_info.json)
echo $msiResourceID
echo "vm sku is $VM_SKU"
VMSS_NAME="$(mktemp -u abtest-XXXXXXX | tr '[:upper:]' '[:lower:]')"
tee vmss.json > /dev/null <<EOF
{
    "group": "${MC_RESOURCE_GROUP_NAME}",
    "vmss": "${VMSS_NAME}"
}
EOF

cat vmss.json

# Create a test VMSS with 1 instance 
# TODO 3: Discuss about the --image version, probably go with aks-ubuntu-1804-gen2-2021-q2:latest
#       However, how to incorporate chaning quarters?
log "Creating VMSS"
vmssStartTime=$(date +%s)
az vmss create -n ${VMSS_NAME} \
    -g $MC_RESOURCE_GROUP_NAME \
    --admin-username azureuser \
    --custom-data cloud-init.txt \
    --lb kubernetes --backend-pool-name aksOutboundBackendPool \
    --vm-sku $VM_SKU \
    --instance-count 1 \
    --assign-identity $msiResourceID \
    --image "microsoft-aks:aks:aks-ubuntu-1804-gen2-2021-q2:2021.05.19" \
    --upgrade-policy-mode Automatic \
    --ssh-key-values ~/.ssh/id_rsa.pub \
    -ojson