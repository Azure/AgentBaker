#!/usr/bin/env bash

# Check if the azure cni config is there... no need to run this script if not
# Also don't want to run this when not using azure-cni
[ ! -f /etc/cni/net.d/10-azure.conflist ] && exit 0

# CNI team mentions that this is not needed for calico network policy to run this script
export NETWORK_POLICY = $1
if [[ "${NETWORK_POLICY}" == "calico" ]]; then
    exit 0
fi

# Check if the azure0 bridge is already configured
# We don't need to run if so.
ip link show azure0 && exit 0

run_plugin() {
        export CNI_COMMAND=$1
        cat /etc/cni/net.d/10-azure.conflist | jq '.name as $name | .cniVersion as $version | .plugins[]+= {name: $name, cniVersion: $version} | .plugins[0]' | /opt/cni/bin/azure-vnet
}

export CNI_ARGS='K8S_POD_NAMESPACE=default;K8S_POD_NAME=configureAzureCNI'
export CNI_CONTAINERID=9999
export CNI_NETNS=/run/netns/configureazcni
export CNI_IFNAME=eth9999
export CNI_PATH=/opt/cni/bin

ip netns add $(basename ${CNI_NETNS})
run_plugin ADD

if [ $? -gt 0 ]; then
        ip netns del "$(basename ${CNI_NETNS})"
        exit 1
fi

run_plugin DEL
ip netns del $(basename ${CNI_NETNS})
