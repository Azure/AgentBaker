#!/bin/bash -e

required_env_vars=(
    "SKU_PREFIX"
    "SKU_TEMPLATE_FILE"
    "AZURE_TENANT_ID"
    "AZURE_CLIENT_ID"
    "AZURE_CLIENT_SECRET"
    "PUBLISHER"
    "OFFER"
    "CONTAINER_RUNTIME"
)

(set -x; ls -lhR artifacts )

for v in "${required_env_vars[@]}"
do
    if [ -z "${!v}" ]; then
        echo "$v was not set!"
        exit 1
    fi
done

if [ ! -f "$SKU_TEMPLATE_FILE" ]; then
    echo "Could not find sku template file: ${SKU_TEMPLATE_FILE}!"
    exit 1
fi

(set -x; ls -lhR artifacts )
vhd_artifacts_path="publishing-info-2019"
if [[ ${CONTAINER_RUNTIME} = "containerd" ]]; then
    vhd_artifacts_path="publishing-info-2019-containerd"
fi
VHD_INFO="artifacts/vhd/${vhd_artifacts_path}/vhd-publishing-info.json"
if [ ! -f "$VHD_INFO" ]; then
    echo "Could not find VHD info file: ${VHD_INFO}!"
    exit 1
fi

short_date=$(date +"%y%m")
pretty_date=$(date +"%b %Y")
sku_id="${SKU_PREFIX}-${short_date}"
echo "Checking if offer contains SKU: $sku_id"
# Check if SKU already exists in offer
(set -x; hack/tools/bin/pub skus list -p $PUBLISHER -o $OFFER | jq ".[] | .planId" | tr -d '"' | tee skus.txt)
echo ""
if grep -q $sku_id skus.txt; then
    echo "Offer already has SKU"
else
    echo "Creating new SKU"
    < $SKU_TEMPLATE_FILE sed s/{{ID}}/"$sku_id"/ | sed s/{{MONTH-YEAR}}/"$pretty_date/" | sed s/{{CONTAINER_RUNTIME}}/"$CONTAINER_RUNTIME/" > sku.json
    echo "" ; cat sku.json ; echo ""
    (set -x ; hack/tools/bin/pub skus put -p $PUBLISHER -o $OFFER -f sku.json ; echo "")
fi

echo "Vhd publishing info:"
cat $VHD_INFO
echo

# Get VHD version info for vhd-publishing-info.json produced by previous pipeline stage
vhd_url=$(< $VHD_INFO jq -r ".vhd_url")
image_version=$(< $VHD_INFO jq -r ".image_version")

# generate media name
# Media name must be under 63 characters
media_name="${SKU_PREFIX}-${image_version}"
if [ "${#media_name}" -ge 63 ]; then
	echo "$media_name should be undr 63 characters"
	exit 1
fi

published_date=$(date +"%m/%d/%Y")
published_image_label="AKS Base Image for Windows"
published_image_description="AKS Base Image for Windows"
echo "publish image version to Marketplace"
echo "pubilsher: $PUBLISHER"
echo "offer: $OFFER"
echo "sku id: $sku_id"
echo "image version: $image_version"
echo "vhd url: $vhd_url"
echo "media name: $media_name"
echo "published date: $published_date"
echo "image label: $published_image_label"
echo "image description: $published_image_description"
echo ""
(set -x ; hack/tools/bin/pub versions put corevm -p $PUBLISHER -o $OFFER -s $sku_id --version $image_version --vhd-uri $vhd_url --media-name $media_name --label "AKS Base Image for Windows" --desc "AKS Base Image for Windows" --published-date "$published_date")