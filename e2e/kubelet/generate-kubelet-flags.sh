#!/usr/bin/env bash
set -euox pipefail

retrycmd_get_tarball() {
    tar_retries=$1; wait_sleep=$2; tarball=$3; url=$4
    echo "${tar_retries} retries"
    for i in $(seq 1 $tar_retries); do
        tar -tzf $tarball && break || \
        if [ $i -eq $tar_retries ]; then
            return 1
        else
            timeout 60 curl -fsSL $url -o $tarball
            sleep $wait_sleep
        fi
    done
}

sudo swapoff -a
sudo ctr version
sudo systemctl status containerd

sed '$ d' parts/linux/cloud-init/artifacts/manifest.json > temporary_manifest.json
KUBE_BINARY_VERSIONS="$(jq -r .kubernetes.versions[] temporary_manifest.json)"
for KUBE_BINARY_VERSION in $KUBE_BINARY_VERSIONS; do
    K8S_DOWNLOADS_DIR="/opt/kubernetes/downloads"
    KUBE_BINARY_URL="https://packages.aks.azure.com/kubernetes/v${KUBE_BINARY_VERSION}/binaries/kubernetes-node-linux-amd64.tar.gz"
    KUBE_BINARY_URL=$(update_base_url $KUBE_BINARY_URL)
    mkdir -p ${K8S_DOWNLOADS_DIR}
    K8S_TGZ_TMP=${KUBE_BINARY_URL##*/}
    retrycmd_get_tarball 120 5 "$K8S_DOWNLOADS_DIR/${K8S_TGZ_TMP}" ${KUBE_BINARY_URL} || exit 120
    tar --transform="s|.*|&-${KUBE_BINARY_VERSION}|" --show-transformed-names -xzvf "$K8S_DOWNLOADS_DIR/${K8S_TGZ_TMP}" \
        --strip-components=3 -C /opt/bin kubernetes/node/bin/kubelet kubernetes/node/bin/kubectl
    rm -f "$K8S_DOWNLOADS_DIR/${K8S_TGZ_TMP}"
    export KUBE_BINARY_VERSION
    pushd e2e || exit 1
    go run -ldflags="-X main.KUBE_BINARY_VERSION=${KUBE_BINARY_VERSION}" kubelet/main.go
    cat kubelet/${KUBE_BINARY_VERSION}-flags.json
    popd || exit 1
done
rm -f temporary_manifest.json