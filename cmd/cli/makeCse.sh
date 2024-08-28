#!/bin/bash
set -e

export subID=subID
export tenantId=tenantId
export orchestratorVersion=1.30.3
export WINDOWS_E2E_VMSIZE=Standard_DS1_v2
export KUBERNETES_VERSION=1.30.3

export WINDOWS_E2E_IMAGE=2019-containerd
export WINDOWS_DISTRO=aks-windows-2019-containerd

export WINDOWS_GPU_DRIVER_SUFFIX=
export WINDOWS_GPU_DRIVER_URL=""
export CONFIG_GPU_DRIVER_IF_NEEDED=false

export WINDOWS_PACKAGE_VERSION=$KUBERNETES_VERSION
export K8S_VERSION=${WINDOWS_PACKAGE_VERSION//./}

envsubst < percluster_template.json > _percluster_config.json

jq -s '.[0] * .[1]' nodebootstrapping_static.json _percluster_config.json  > _nodebootstrapping-config.json

go run main.go getCustomScript < _nodebootstrapping-config.json
#go run main.go getCustomScriptData < _nodebootstrapping-config.json
