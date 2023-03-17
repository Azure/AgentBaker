#!/usr/bin/env bash
set -euxo pipefail

TRIVY_REPORT_PATH=/opt/azure/containers/trivy-report.json
TRIVY_VERSION="0.38.2"
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

mkdir -p "$(dirname "${TRIVY_REPORT_PATH}")"

wget "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
tar -xvzf "trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
rm "trivy_${TRIVY_VERSION}_${TRIVY_ARCH}.tar.gz"
chmod a+x trivy 

./trivy --security-checks vuln rootfs -f json --ignore-unfixed --severity HIGH,CRITICAL -o "${TRIVY_REPORT_PATH}" /

rm ./trivy 

chmod a+r "${TRIVY_REPORT_PATH}"
