#!/usr/bin/env bash

set -eu -o pipefail
shopt -s failglob

[ $# -ne 1 ] && { echo "Usage: $0 NODE"; exit 1; }

NODE=$1

# TODO: need to clean up flexvol stuff too?
echo "Removing Calico CNI binaries and conflist"
kubectl delete pods cleanup || true
cat cleanup.yaml | sed s/NODE/$NODE/g | kubectl apply -f -
sleep 10

CALICOPOD=$(kubectl get pods -n calico-system -l k8s-app=calico-node -owide | grep $NODE | cut -d ' ' -f 1)
echo "Deleting $CALICOPOD"
kubectl delete pod -n calico-system $CALICOPOD

date --utc
