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

deleteCluster() {
    name=$1; rg=$2
    log "Deleting cluster $name"
    clusterDeleteStartTime=$(date +%s)
    az aks delete -n $name -g $rg --yes
    clusterDeleteEndTime=$(date +%s)
    log "Deleted cluster $name in $((clusterDeleteEndTime-clusterDeleteStartTime)) seconds"
    create_cluster="true"
}

# waitForCluster() {
#     name=$1; rg=$2
#     cluster_provisioning_state=$(az aks show -n $name -g $rg | jq '.provisioningState')

#     while [[ "$cluster_provisioning_state" == "\"Creating\"" ]] || [[ "$cluster_provisioning_state" == "\"Stopping\"" ]]; do
#         log "Cluster $name is currently in provisioning state $cluster_provisioning_state, waiting for \"Succeeded\", \"Failed\", \"Stopped\"states"
#         sleep 10
#         cluster_provisioning_state=$(az aks show -n $name -g $rg | jq '.provisioningState')
#     done

#     if [[ "$cluster_provisioning_state" == "\"Succeeded\"" ]]; then
#         return 0
#     elif [[ "$cluster_provisioning_state" == "\"Failed\"" ]]; then
#         return 1
#     else
#         log "Cluster in unrecognized provisioning state: $cluster_provisioning_state"
#         return 1
#     fi
# }