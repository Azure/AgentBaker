#!/bin/bash

# The shared cleanup script also collects unrelated old images, galleries and
# resource groups. This manual pipeline cleans only its recorded Packer group.
function cleanup_amd_gpu_build_resource_group() {
	local settings_file="$1" resource_group="$2" build_id="$3"
	local subscription group
	case "${resource_group}" in
	'' | *"\$("*)
		echo 'No recorded Packer resource group; skipping AMD build cleanup'
		return 0
		;;
	esac
	case "${build_id}" in
	'' | *[!0-9]*)
		echo 'No valid build ID; skipping AMD build cleanup'
		return 0
		;;
	esac
	subscription=$(jq -er '.subscription_id | select(type == "string" and length > 0)' "${settings_file}") || {
		echo 'No recorded build subscription; skipping AMD build cleanup'
		return 0
	}
	case "${subscription}" in
	*"\$("*)
		echo 'Unresolved build subscription; skipping AMD build cleanup'
		return 0
		;;
	esac
	group=$(az group show --name "${resource_group}" --subscription "${subscription}" --output json) || {
		echo 'Packer resource group is absent or could not be read; skipping AMD build cleanup'
		return 0
	}
	if ! jq -e --arg build_id "${build_id}" --arg resource_group "${resource_group}" --arg subscription "${subscription}" '
      .tags.buildId == $build_id and .tags.createdBy == "aks-vhd-pipeline" and
      (.name | ascii_downcase) == ($resource_group | ascii_downcase) and
      (.id | ascii_downcase) ==
        ("/subscriptions/" + $subscription + "/resourceGroups/" + $resource_group | ascii_downcase)
    ' <<<"${group}" >/dev/null; then
		echo 'Packer resource group ownership does not match this build; skipping AMD build cleanup'
		return 0
	fi
	echo "Deleting verified Packer resource group ${resource_group} for build ${build_id}"
	az group delete --name "${resource_group}" --subscription "${subscription}" --yes --only-show-errors
}
