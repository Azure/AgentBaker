set -euo

set +x
echo "Downloading VHD..."
VHD_RELEASE_CANDIDATE="$(Build.SourcesDirectory)/${sku_name}_release_candidate.vhd"

export AZCOPY_AUTO_LOGIN_TYPE="MSI"
export AZCOPY_MSI_RESOURCE_STRING="$AZURE_MSI_RESOURCE_STRING"
export AZCOPY_CONCURRENCY_VALUE="AUTO"

# Dynamically determine url for checksum
azcopy copy "${checksum_url}${sas_token}" "$VHD_RELEASE_CANDIDATE" --recursive=true

echo "Calculating checksum..."

sha256sum "$VHD_RELEASE_CANDIDATE" > "${VHD_RELEASE_CANDIDATE}.sha256"

echo "Digitally signing sha256 file..."

gpg --armor --detach-sign --output "${VHD_RELEASE_CANDIDATE}.sha256.asc" "${VHD_RELEASE_CANDIDATE}.sha256"


echo "Printing checksum..."
cat mydisk.sha256   