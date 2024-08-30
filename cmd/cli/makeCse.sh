#!/bin/bash
set -e

export subID=subID
export tenantId=tenantId

export fqdn=https://aks-timmy-wrightt-resource-82acd5-zr3wop33.hcp.australiaeast.azmk8s.io
export WINDOWS_E2E_VMSIZE=Standard_DS1_v2
export KUBERNETES_VERSION=1.30.3
export csePackageURL=https://acs-mirror.azureedge.net/aks/windows/cse/

## From vhdbuilder/packer/generate-windows-vhd-configuration.ps1 line 176
#        "https://acs-mirror.azureedge.net/kubernetes/v1.27.14-hotfix.20240712/windowszip/v1.27.14-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.27.15-hotfix.20240712/windowszip/v1.27.15-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.27.16/windowszip/v1.27.16-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.28.5-hotfix.20240712/windowszip/v1.28.5-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.28.9-hotfix.20240712/windowszip/v1.28.9-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.28.10-hotfix.20240712/windowszip/v1.28.10-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.28.11-hotfix.20240712/windowszip/v1.28.11-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.28.12/windowszip/v1.28.12-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.28.13/windowszip/v1.28.13-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.29.2-hotfix.20240712/windowszip/v1.29.2-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.29.4-hotfix.20240712/windowszip/v1.29.4-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.29.5-hotfix.20240712/windowszip/v1.29.5-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.29.6-hotfix.20240712/windowszip/v1.29.6-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.29.7/windowszip/v1.29.7-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.29.8/windowszip/v1.29.8-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.30.1-hotfix.20240712/windowszip/v1.30.1-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.30.2-hotfix.20240712/windowszip/v1.30.2-hotfix.20240712-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.30.3/windowszip/v1.30.3-1int.zip",
#        "https://acs-mirror.azureedge.net/kubernetes/v1.30.4/windowszip/v1.30.4-1int.zip"
export kubeBinariesSASURL=https://acs-mirror.azureedge.net/kubernetes/v1.30.3/windowszip/v1.30.3-1int.zip

export WINDOWS_E2E_IMAGE=2019-containerd
export WINDOWS_DISTRO=aks-windows-2019-containerd

export WINDOWS_GPU_DRIVER_SUFFIX=
export WINDOWS_GPU_DRIVER_URL=""
export CONFIG_GPU_DRIVER_IF_NEEDED=false

export adminUserName=tim
export adminPublicKeyData="public key data"

envsubst < percluster_template.json | jq -s '.[0] * .[1]' nodebootstrapping_static.json - | go run main.go getCustomScript  > CustomScriptExtension.bat
envsubst < percluster_template.json | jq -s '.[0] * .[1]' nodebootstrapping_static.json - | go run main.go getCustomScriptData | base64 --decode > CustomData.bin

#scp CustomScriptExtension.bat CustomData.bin tim@timmy-win-vm.australiaeast.cloudapp.azure.com:/AzureData/
