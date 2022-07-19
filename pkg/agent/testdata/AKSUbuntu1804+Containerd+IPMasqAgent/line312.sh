#!/bin/bash

# remove this if we are no longer using promiscuous bridge mode for containerd
# background: we get duplicated packets from pod to serviceIP if both are on the same node (one from the cbr0 bridge and one from the pod ip itself via kernel due to promiscuous mode being on)
# we should filter out the one from pod ip
# this is exactly what kubelet does for dockershim+kubenet
# https://github.com/kubernetes/kubernetes/pull/28717

ebtables -t filter -L AKS-DEDUP-PROMISC 2>/dev/null
if [[ $? -eq 0 ]]; then
    echo "AKS-DEDUP-PROMISC rule already set"
    exit 0
fi
if [[ ! -f /etc/cni/net.d/10-containerd-net.conflist ]]; then
    echo "cni config not up yet...exiting early"
    exit 1
fi

bridgeName=$(cat /etc/cni/net.d/10-containerd-net.conflist  | jq -r ".plugins[] | select(.type == \"bridge\") | .bridge")
promiscMode=$(cat /etc/cni/net.d/10-containerd-net.conflist  | jq -r ".plugins[] | select(.type == \"bridge\") | .promiscMode")
if [[ "${promiscMode}" != "true" ]]; then
    echo "bridge ${bridgeName} not in promiscuous mode...exiting early"
    exit 0
fi

if [[ ! -f /sys/class/net/${bridgeName}/address ]]; then
    echo "bridge ${bridgeName} not up yet...exiting early"
    exit 1
fi


bridgeIP=$(ip addr show ${bridgeName} | grep -Eo "inet ([0-9]*\.){3}[0-9]*" | grep -Eo "([0-9]*\.){3}[0-9]*")
if [[ -z "${bridgeIP}" ]]; then
    echo "bridge ${bridgeName} does not have an ipv4 address...exiting early"
    exit 1
fi

podSubnetAddr=$(cat /etc/cni/net.d/10-containerd-net.conflist  | jq -r ".plugins[] | select(.type == \"bridge\") | .ipam.subnet")
if [[ -z "${podSubnetAddr}" ]]; then
    echo "could not determine this node's pod ipam subnet range from 10-containerd-net.conflist...exiting early"
    exit 1
fi

bridgeMAC=$(cat /sys/class/net/${bridgeName}/address)

echo "adding AKS-DEDUP-PROMISC ebtable chain"
ebtables -t filter -N AKS-DEDUP-PROMISC # add new AKS-DEDUP-PROMISC chain
ebtables -t filter -A AKS-DEDUP-PROMISC -p IPv4 -s ${bridgeMAC} -o veth+ --ip-src ${bridgeIP} -j ACCEPT
ebtables -t filter -A AKS-DEDUP-PROMISC -p IPv4 -s ${bridgeMAC} -o veth+ --ip-src ${podSubnetAddr} -j DROP
ebtables -t filter -A OUTPUT -j AKS-DEDUP-PROMISC # add new rule to OUTPUT chain jump to AKS-DEDUP-PROMISC

echo "outputting newly added AKS-DEDUP-PROMISC rules:"
ebtables -t filter -L OUTPUT 2>/dev/null
ebtables -t filter -L AKS-DEDUP-PROMISC 2>/dev/null
exit 0
#EOF