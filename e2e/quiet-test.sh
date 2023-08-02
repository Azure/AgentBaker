#!/usr/bin/env bash

set -eu -o pipefail
shopt -s failglob

[ $# -ne 1 ] && { echo "Usage: $0 NODE"; exit 1; }

NODE=$1

echo "Removing Calico CNI binaries and conflist"
kubectl run cleanup --rm --image=alpine -it --privileged=true --overrides="{\"spec\": {\"nodeSelector\": {\"kubernetes.io/hostname\": \"$NODE\"}, \"hostNetwork\":true, \"hostPID\":true}}" -- nsenter --target "1" --mount --uts --ipc --net --pid -- bash -c "rm -f /opt/cni/bin/{calico-ipam,calico,flannel,install} /etc/cni/net.d/{10-calico.conflist,calico-kubeconfig}"

CALICOPOD=$(kubectl get pods -n calico-system -l k8s-app=calico-node -owide | grep $NODE | cut -d ' ' -f 1)
echo "Deleting $CALICOPOD"
kubectl delete pod -n calico-system $CALICOPOD

date --utc
