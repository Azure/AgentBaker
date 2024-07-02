#!/usr/bin/env bash
set -euxo pipefail

TRIVY_REPORT_DIRNAME=/opt/azure/containers
TRIVY_REPORT_ROOTFS_JSON_PATH=${TRIVY_REPORT_DIRNAME}/trivy-report-rootfs.json
TRIVY_VERSION="0.40.0"
TRIVY_ARCH=""

arch="$(uname -m)"
if [ "${arch,,}" == "arm64" ] || [ "${arch,,}" == "aarch64" ]; then
    TRIVY_ARCH="Linux-ARM64"
elif [ "${arch,,}" == "x86_64" ]; then
    TRIVY_ARCH="Linux-64bit"
else
    echo "invalid architecture ${arch,,}"
    exit 1
fi

mkdir -p "$(dirname "${TRIVY_REPORT_DIRNAME}")"

wget "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
tar -xvzf "trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
rm "trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
chmod a+x trivy

./trivy --scanners vuln rootfs -f json --skip-dirs /var/lib/containerd --ignore-unfixed --severity HIGH,CRITICAL -o "${TRIVY_REPORT_ROOTFS_JSON_PATH}" /

IMAGE_LIST=$(ctr -n k8s.io image list -q | grep -v sha256)

for CONTAINER_IMAGE in $IMAGE_LIST; do
    BASE_CONTAINER_IMAGE=$(basename ${CONTAINER_IMAGE})
    TRIVY_REPORT_IMAGE_JSON_PATH=${TRIVY_REPORT_DIRNAME}/trivy-report-image-${BASE_CONTAINER_IMAGE}.json
    ./trivy --scanners vuln image -f json --ignore-unfixed --severity HIGH,CRITICAL -o ${TRIVY_REPORT_IMAGE_JSON_PATH} $CONTAINER_IMAGE  || true
    chmod a+r "${TRIVY_REPORT_IMAGE_JSON_PATH}"
done

rm ./trivy

chmod a+r "${TRIVY_REPORT_ROOTFS_JSON_PATH}"
