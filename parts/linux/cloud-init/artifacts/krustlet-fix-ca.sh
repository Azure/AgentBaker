#!/usr/bin/env bash
set -o nounset
set -o pipefail
set -x

DONE="$(grep certificate-authority-data /var/lib/kubelet/bootstrap-kubeconfig)"

if [ -n "$DONE" ]; then
    echo "Found certificate-authority-data, will not modify bootstrap-kubeconfig"
    exit 0
fi

# we don't want to fail if the above grep fails, only after that.
set -o errexit

# TODO(ace): always use /etc/kubernetes/certs/ca.crt?
CA_FILE=$(grep certificate-authority /var/lib/kubelet/bootstrap-kubeconfig | cut -d" " -f6)
CA_CONTENT="$(cat $CA_FILE | base64 -w 0)"
sed -i "s~certificate-authority: /etc/kubernetes/certs/ca.crt~certificate-authority-data: $CA_CONTENT~g" /var/lib/kubelet/bootstrap-kubeconfig 