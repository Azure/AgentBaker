# shellcheck shell=bash

# Windows base images for building VHD. To install Windows patches, please refer to configure-windows-vhd.ps1
# CLI example to get all available image version under a SKU:
#   az vm image list --all --publisher MicrosoftWindowsServer --offer WindowsServer --output table -s 2019-Datacenter-Core-smalldisk
# CLI example to get the latest image version:
#   az vm image show --urn MicrosoftWindowsServer:WindowsServer:2019-Datacenter-Core-smalldisk:latest
WINDOWS_2019_BASE_IMAGE_SKU=2019-Datacenter-Core-smalldisk
# TODO: update global:patch in generate-windows-vhd-configuration.ps1 and remove this comment when you bump 12B
# - but revert and bring back if open ssh fails when you build the VHD. This image is 9B.
WINDOWS_2019_BASE_IMAGE_VERSION=17763.6293.240905
#WINDOWS_2019_BASE_IMAGE_VERSION=$(az vm image show --urn "MicrosoftWindowsServer:WindowsServer:${WINDOWS_2019_BASE_IMAGE_SKU}:latest"  | jq -r .name)

WINDOWS_2022_BASE_IMAGE_SKU=2022-Datacenter-Core-smalldisk
WINDOWS_2022_BASE_IMAGE_VERSION=$(az vm image show --urn "MicrosoftWindowsServer:WindowsServer:${2022-Datacenter-Core-smalldisk}:latest"  | jq -r .name)

# CLI example to get all available image version under a SKU (suffix g2 for Gen 2):
#   az vm image list --all --publisher MicrosoftWindowsServer --offer WindowsServer --output table -s 2022-datacenter-core-smalldisk-g2

WINDOWS_2022_GEN2_BASE_IMAGE_SKU=2022-datacenter-core-smalldisk-g2
WINDOWS_2022_GEN2_BASE_IMAGE_VERSION=$(az vm image show --urn "MicrosoftWindowsServer:WindowsServer:${WINDOWS_2022_GEN2_BASE_IMAGE_SKU}:latest"  | jq -r .name)

WINDOWS_23H2_BASE_IMAGE_SKU=23h2-datacenter-core
WINDOWS_23H2_BASE_IMAGE_VERSION=$(az vm image show --urn "MicrosoftWindowsServer:${WINDOWS_23H2_BASE_IMAGE_SKU}:latest"  | jq -r .name)

# CLI example to get all available image version under a SKU (suffix g2 for Gen 2):
#   az vm image list --all --publisher MicrosoftWindowsServer --offer WindowsServer --output table -s 23h2-datacenter-core-g2
WINDOWS_23H2_GEN2_BASE_IMAGE_SKU=23h2-datacenter-core-g2
WINDOWS_23H2_GEN2_BASE_IMAGE_VERSION=$(az vm image show --urn "MicrosoftWindowsServer:WindowsServer:${WINDOWS_23H2_GEN2_BASE_IMAGE_SKU}:latest"  | jq -r .name)

# Please uncomment the following lines and set a larger os disk size that is at least 30GB when your PR check-in fails
# WINDOWS_2019_CONTAINERD_OS_DISK_SIZE_GB=30
WINDOWS_2022_CONTAINERD_OS_DISK_SIZE_GB=35
WINDOWS_23H2_OS_DISK_SIZE_GB=35
