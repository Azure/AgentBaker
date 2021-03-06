#!/bin/bash

# remove this if we are no longer using promiscuous bridge mode for containerd
# background: we get duplicated packets from pod to serviceIP if both are on the same node (one from the cbr0 bridge and one from the pod ip itself via kernel due to promiscuous mode being on)
# we should filter out the one from pod ip
# this is exactly what kubelet does for dockershim+kubenet
# https://github.com/kubernetes/kubernetes/pull/28717
for try in {1..10}; do
    ebtables -t filter -L AKS-DEDUP 2>/dev/null
    if [[ $? -eq 0 ]]; then
        echo "AKS-DEDUP rule already set"
        exit 0
    fi
    if [[ ! -f /etc/cni/net.d/10-containerd-net.conflist ]]; then
        echo "cni config not up yet...checking again in 5s"
        sleep 5
        continue
    fi
    podSubnetAddr=$(cat /etc/cni/net.d/10-containerd-net.conflist  | jq -r ".plugins[] | select(.type == \"bridge\") | .ipam.subnet")

    if [[ ! -f /sys/class/net/cbr0/address ]]; then
        echo "cbr0 bridge not up yet...checking again in 5s"
        sleep 5
        continue
    fi
    cbr0MAC=$(cat /sys/class/net/cbr0/address)

    cbr0IP=$(ip addr show cbr0 | grep -Eo "inet ([0-9]*\.){3}[0-9]*" | grep -Eo "([0-9]*\.){3}[0-9]*")
    if [[ -z "${cbr0IP}" ]]; then
        echo "cbr0 bridge does not have an ipv4 address...checking again in 5s"
        sleep 5
        continue
    fi

    ebtables -t filter -N AKS-DEDUP # add new AKS-DEDUP chain
    ebtables -t filter -A AKS-DEDUP -p IPv4 -s ${cbr0MAC} -o veth+ --ip-src ${cbr0IP} -j ACCEPT
    ebtables -t filter -A AKS-DEDUP -p IPv4 -s ${cbr0MAC} -o veth+ --ip-src ${podSubnetAddr} -j DROP
    ebtables -t filter -A OUTPUT -j AKS-DEDUP # add new rule to OUTPUT chain jump to AKS-DEDUP
    exit 0
done
exit 1
#EOF