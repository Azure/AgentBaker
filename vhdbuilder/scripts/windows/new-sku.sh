#!/bin/bash -e

required_env_vars=(
    "IMAGE_SKU"
    "CONTAINER_RUNTIME"
)

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

# As Marketpalce(Platform Image Repository) demonstrates the SKU title prominently,  
# ensuring each SKU title unique helps users locate the correct SKU
case "${IMAGE_SKU}" in
"2019-Datacenter-Core-smalldisk") 
    WINDOWS_VERSION_SHORT="windows-2019"
    ;;
"datacenter-core-2004-with-containers-smalldisk")
    WINDOWS_VERSION_SHORT="windows-2004"
    ;;
*)  
    echo "unsupported windows sku: ${IMAGE_SKU}" 
    exit 1
    ;; 
esac

valid_container_runtimes=("docker" "containerd")
if [[ ! " ${valid_container_runtimes[@]} " =~ " ${CONTAINER_RUNTIME} " ]]; then
    echo "CONTAINER_RUNTIME should be among [${valid_container_runtimes[*]}]"
    exit 1
fi

short_date=$(date +"%y%m")
pretty_date=$(date +"%b %Y")
sku_id="${IMAGE_SKU}-${short_date}"

BASEDIR=$(dirname "$0")
SKU_TEMPLATE_FILE_NAME="sku-template.json"
SKU_TEMPLATE_FILE=$BASEDIR/$SKU_TEMPLATE_FILE_NAME

cat $SKU_TEMPLATE_FILE |sed s/{{ID}}/"$sku_id"/ | sed s/{{MONTH-YEAR}}/"$pretty_date/" | sed s/{{CONTAINER_RUNTIME}}/"$CONTAINER_RUNTIME/" | sed s/{{WINDOWS_VERSION}}/"$WINDOWS_VERSION_SHORT/"
