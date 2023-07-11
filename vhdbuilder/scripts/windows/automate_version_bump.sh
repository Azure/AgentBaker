set -euxo pipefail
az login --identity

# This function finds the latest windows VHD base Image version from the command az vm image show
find_latest_image_version() {
    latest_image_version_2019=$(az vm image show --urn MicrosoftWindowsServer:WindowsServer:2019-Datacenter-Core-smalldisk:latest --query 'id' -o tsv | awk -F '/' '{print $NF}')
    latest_image_version_2022=$(az vm image show --urn MicrosoftWindowsServer:WindowsServer:2022-Datacenter-Core-smalldisk:latest --query 'id' -o tsv | awk -F '/' '{print $NF}')
    echo "Latest windows 2019 base image version is: ${latest_image_version_2019}"
    echo "Latest windows 2022 base image version is: ${latest_image_version_2022}"
}

find_latest_image_version