#!/usr/bin/env bash
set -euxo pipefail

TRIVY_REPORT_JSON_PATH=/opt/azure/containers/trivy-report.json
TRIVY_REPORT_TABLE_PATH=/opt/azure/containers/trivy-images-table.txt
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

mkdir -p "$(dirname "${TRIVY_REPORT_JSON_PATH}")"
mkdir -p "$(dirname "${TRIVY_REPORT_TABLE_PATH}")"

wget "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
tar -xvzf "trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
rm "trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
chmod a+x trivy 

./trivy --scanners vuln rootfs -f json --skip-dirs /var/lib/containerd --ignore-unfixed --severity HIGH,CRITICAL -o "${TRIVY_REPORT_JSON_PATH}" /

IMAGE_LIST=$(ctr -n k8s.io image list -q | grep -v sha256)

echo "This contains the list of images with high and critical level CVEs (if present), that are present in the node. 
Note: images without CVEs are also listed" >> "${TRIVY_REPORT_TABLE_PATH}"

for image in $IMAGE_LIST; do
    ./trivy --scanners vuln image --ignore-unfixed --severity HIGH,CRITICAL --parallel 13 -f table $image >> ${TRIVY_REPORT_TABLE_PATH} || true
done

rm ./trivy 

chmod a+r "${TRIVY_REPORT_JSON_PATH}"
chmod a+r "${TRIVY_REPORT_TABLE_PATH}"
