#!/bin/bash -e

CDIR=$(dirname "${BASH_SOURCE}")

SETTINGS_JSON="${SETTINGS_JSON:-./packer/settings.json}"
SP_JSON="${SP_JSON:-./packer/sp.json}"
SUBSCRIPTION_ID="${SUBSCRIPTION_ID:-$(az account show -o json --query="id" | tr -d '"')}"
CREATE_TIME="$(date +%s)"
STORAGE_ACCOUNT_NAME="aksimages${CREATE_TIME}$RANDOM"


echo "Subscription ID: ${SUBSCRIPTION_ID}"
echo "Service Principal Path: ${SP_JSON}"

if [ -a "${SP_JSON}" ]; then
	echo "Existing credentials file found."
	exit 0
elif [ -z "${CLIENT_ID}" ]; then
	echo "Service principal not found! Generating one @ ${SP_JSON}"
	az ad sp create-for-rbac -n aks-images-packer${CREATE_TIME} -o json > ${SP_JSON}
	CLIENT_ID=$(jq -r .appId ${SP_JSON})
	CLIENT_SECRET=$(jq -r .password ${SP_JSON})
	TENANT_ID=$(jq -r .tenant ${SP_JSON})
fi

rg_id=$(az group show --name $AZURE_RESOURCE_GROUP_NAME) || rg_id=""
if [ -z "$rg_id" ]; then
	echo "Creating resource group $AZURE_RESOURCE_GROUP_NAME, location ${AZURE_LOCATION}"
	az group create --name $AZURE_RESOURCE_GROUP_NAME --location ${AZURE_LOCATION}
fi

avail=$(az storage account check-name -n ${STORAGE_ACCOUNT_NAME} -o json | jq -r .nameAvailable)
if $avail ; then
	echo "creating new storage account ${STORAGE_ACCOUNT_NAME}"
	az storage account create -n $STORAGE_ACCOUNT_NAME -g $AZURE_RESOURCE_GROUP_NAME --sku "Standard_RAGRS" --tags "now=${CREATE_TIME}"
	echo "creating new container system"
	key=$(az storage account keys list -n $STORAGE_ACCOUNT_NAME -g $AZURE_RESOURCE_GROUP_NAME | jq -r '.[0].value')
	az storage container create --name system --account-key=$key --account-name=$STORAGE_ACCOUNT_NAME
else
	echo "storage account ${STORAGE_ACCOUNT_NAME} already exists."
fi

if [ -z "${CLIENT_ID}" ]; then
	echo "CLIENT_ID was not set! Something happened when generating the service principal or when trying to read the sp file!"
	exit 1
fi

if [ -z "${CLIENT_SECRET}" ]; then
	echo "CLIENT_SECRET was not set! Something happened when generating the service principal or when trying to read the sp file!"
	exit 1
fi

if [ -z "${TENANT_ID}" ]; then
	echo "TENANT_ID was not set! Something happened when generating the service principal or when trying to read the sp file!"
	exit 1
fi

echo "storage name: ${STORAGE_ACCOUNT_NAME}"

# If SIG_IMAGE_NAME hasnt been provided in Gen2 mode, set it to the default value
if [[ "$MODE" == "gen2Mode" ]]; then
	if 	[[ -z "$SIG_GALLERY_NAME" ]]; then
		SIG_GALLERY_NAME="PackerSigGalleryEastUS"
	fi
	if 	[[ -z "$SIG_IMAGE_NAME" ]]; then
		if [[ "$OS_SKU" == "Ubuntu" ]]; then
			SIG_IMAGE_NAME=${OS_VERSION//./}Gen2
		fi
		if [[ "$OS_SKU" == "CBLMariner" ]]; then
			SIG_IMAGE_NAME=${OS_SKU}${OS_VERSION//./}Gen2
		fi
		echo "No input SIG_IMAGE_NAME for Packer build output. Setting to `${SIG_IMAGE_NAME}`"
	fi
fi


if [[ "$MODE" == "sigMode" || "$MODE" == "gen2Mode" ]]; then
	echo "SIG existence checking for $MODE"
	id=$(az sig show --resource-group ${AZURE_RESOURCE_GROUP_NAME} --gallery-name ${SIG_GALLERY_NAME}) || id=""
	if [ -z "$id" ]; then
		echo "Creating gallery ${SIG_GALLERY_NAME} in the resource group ${AZURE_RESOURCE_GROUP_NAME} location ${AZURE_LOCATION}"
		az sig create --resource-group ${AZURE_RESOURCE_GROUP_NAME} --gallery-name ${SIG_GALLERY_NAME} --location ${AZURE_LOCATION}
	else
		echo "Gallery ${SIG_GALLERY_NAME} exists in the resource group ${AZURE_RESOURCE_GROUP_NAME} location ${AZURE_LOCATION}"
	fi

	id=$(az sig image-definition show \
		--resource-group ${AZURE_RESOURCE_GROUP_NAME} \
		--gallery-name ${SIG_GALLERY_NAME} \
		--gallery-image-definition ${SIG_IMAGE_NAME}) || id=""
	if [ -z "$id" ]; then
		echo "Creating image definition ${SIG_IMAGE_NAME} in gallery ${SIG_GALLERY_NAME} resource group ${AZURE_RESOURCE_GROUP_NAME}"
		az sig image-definition create \
			--resource-group ${AZURE_RESOURCE_GROUP_NAME} \
			--gallery-name ${SIG_GALLERY_NAME} \
			--gallery-image-definition ${SIG_IMAGE_NAME} \
			--publisher microsoft-aks \
			--offer ${SIG_GALLERY_NAME} \
			--sku ${SIG_IMAGE_NAME} \
			--os-type ${OS_TYPE} \
			--hyper-v-generation ${HYPERV_GENERATION} \
			--location ${AZURE_LOCATION}
	else
		echo "Image definition ${SIG_IMAGE_NAME} existing in gallery ${SIG_GALLERY_NAME} resource group ${AZURE_RESOURCE_GROUP_NAME}"
	fi
else
	echo "Skipping SIG check for $MODE"
fi




#Tien: this is important
# Image import from storage account. Required to build CBLMariner images.
if [[ "$OS_SKU" == "CBLMariner" ]]; then
	if [[ $HYPERV_GENERATION == "V2" ]]; then
		IMPORT_IMAGE_URL=${IMPORT_IMAGE_URL_GEN2}
	elif [[ $HYPERV_GENERATION == "V1" ]]; then
		IMPORT_IMAGE_URL=${IMPORT_IMAGE_URL_GEN1}
	fi

	expiry_date=$(date -u -d "10 minutes" '+%Y-%m-%dT%H:%MZ')
	sas_token=$(az storage account generate-sas --account-name $STORAGE_ACCOUNT_NAME --permissions rcw --resource-types o --services b --expiry ${expiry_date} | tr -d '"')

	IMPORTED_IMAGE_NAME=imported-$CREATE_TIME-$RANDOM
	IMPORTED_IMAGE_URL="https://${STORAGE_ACCOUNT_NAME}.blob.core.windows.net/system/$IMPORTED_IMAGE_NAME.vhd"
	DESTINATION_WITH_SAS="${IMPORTED_IMAGE_URL}?${sas_token}"
	echo "Tien is here2"
	echo "IMPORT_IMAGE_URL: $IMPORT_IMAGE_URL"
	echo "DESTINATION_WITH_SAS: $DESTINATION_WITH_SAS"
	echo Importing VHD from $IMPORT_IMAGE_URL
	#azcopy-preview copy $IMPORT_IMAGE_URL$IMPORT_IMAGE_SAS $DESTINATION_WITH_SAS
	azcopy-preview copy $IMPORT_IMAGE_URL $DESTINATION_WITH_SAS
	echo "Tien is here3"



# # Generation 2 Packer builds require that the imported image is hosted in a SIG
# 	if [[ $HYPERV_GENERATION == "V2" ]]; then
# 		echo "Creating new image for imported vhd ${IMPORTED_IMAGE_URL}"
# 		az image create \
# 			--resource-group $AZURE_RESOURCE_GROUP_NAME \
# 			--name $IMPORTED_IMAGE_NAME \
# 			--source $IMPORTED_IMAGE_URL \
# 			--hyper-v-generation V2 \
# 			--os-type Linux

# 		echo "Creating new image-definition for imported image ${IMPORTED_IMAGE_NAME}"
# 		az sig image-definition create \
# 			--resource-group $AZURE_RESOURCE_GROUP_NAME \
# 			--gallery-name $SIG_GALLERY_NAME \
# 			--gallery-image-definition $IMPORTED_IMAGE_NAME \
# 			--location $AZURE_LOCATION \
# 			--os-type Linux \
# 			--publisher microsoft-aks \
# 			--offer $IMPORTED_IMAGE_NAME \
# 			--sku $OS_SKU \
# 			--hyper-v-generation V2 \
# 			--os-state generalized \
# 			--description "Imported image for AKS Packer build"

# 		echo "Creating new image-version for imported image ${IMPORTED_IMAGE_NAME}"
# 		az sig image-version create \
# 			--location $AZURE_LOCATION \
# 			--resource-group $AZURE_RESOURCE_GROUP_NAME \
# 			--gallery-name $SIG_GALLERY_NAME \
# 			--gallery-image-definition $IMPORTED_IMAGE_NAME \
# 			--gallery-image-version 1.0.0 \
# 			--managed-image $IMPORTED_IMAGE_NAME
# 	fi



fi

# considerations to also add the windows support here instead of an extra script to initialize windows variables:
# 1. we can demonstrate the whole user defined parameters all at once
# 2. help us keep in mind that changes of these variables will influence both windows and linux VHD building

# windows image sku and windows image version are recorded in code instead of pipeline variables
# because a pr gives a better chance to take a review of the version changes.
WINDOWS_IMAGE_SKU=""
WINDOWS_IMAGE_VERSION=""
# shellcheck disable=SC2236
if [ ! -z "${WINDOWS_SKU}" ]; then
	source $CDIR/windows-image.env
	case "${WINDOWS_SKU}" in
	"2019"|"2019-containerd")
		WINDOWS_IMAGE_SKU=$WINDOWS_2019_BASE_IMAGE_SKU
		WINDOWS_IMAGE_VERSION=$WINDOWS_2019_BASE_IMAGE_VERSION
		;;
	*)
		echo "unsupported windows sku: ${WINDOWS_SKU}"
		exit 1
		;;
	esac
fi

echo "Tien is here4"
echo "Tien is here4 CLIENT_ID: $CLIENT_ID"
echo "Tien is here4 TENANT_ID: $TENANT_ID"
echo "Tien is here4 AZURE_CLIENT_ID: $AZURE_CLIENT_ID"
echo "Tien is here4 AZURE_TENANT_ID: $AZURE_TENANT_ID"
echo "Tien is here4 AZURE_SUBSCRIPTION_ID: $AZURE_SUBSCRIPTION_ID"



cat <<EOF > vhdbuilder/packer/settings.json
{
  "subscription_id":  "${SUBSCRIPTION_ID}",
  "client_id": "${CLIENT_ID}",
  "client_secret": "${CLIENT_SECRET}",
  "tenant_id":      "${TENANT_ID}",
  "resource_group_name": "${AZURE_RESOURCE_GROUP_NAME}",
  "location": "${AZURE_LOCATION}",
  "storage_account_name": "${STORAGE_ACCOUNT_NAME}",
  "vm_size": "${AZURE_VM_SIZE}",
  "create_time": "${CREATE_TIME}",
  "windows_image_sku": "${WINDOWS_IMAGE_SKU}",
  "windows_image_version": "${WINDOWS_IMAGE_VERSION}",
  "imported_image_name": "${IMPORTED_IMAGE_NAME}",
  "sig_image_name":  "${SIG_IMAGE_NAME}",
  "sig_gallery_name": "${SIG_GALLERY_NAME}"
}
EOF

cat vhdbuilder/packer/settings.json
