#!/usr/bin/env bash
set -euo pipefail

components=$(jq .ContainerImages[] --monochrome-output --compact-output < parts/common/components.json)
i=0
while IFS= read -r component; do
	downloadURL=$(echo "${component}" | jq .downloadURL)
	downloadURL=$(echo ${downloadURL//\*/} | jq 'sub(".com/" ; ".com/v2/") | sub(":" ; "/tags/list")' -r)
	amd64OnlyVersionsStr=$(echo "${component}" | jq .amd64OnlyVersions -r)
	amd64OnlyVersions=""
	if [ "${amd64OnlyVersionsStr}" != "null" ]; then
		amd64OnlyVersions=$(echo "${amd64OnlyVersionsStr}" | jq -r ".[]")
	fi
	multiArchVersionsV2=()
	mapfile -t latestVersions < <(echo "${component}" | jq -r '.multiArchVersionsV2[] | select(.latestVersion != null) | .latestVersion')
    mapfile -t previousLatestVersions < <(echo "${component}" | jq -r '.multiArchVersionsV2[] | select(.previousLatestVersion != null) | .previousLatestVersion')
    for version in "${latestVersions[@]}"; do
        multiArchVersionsV2+=("${version}")
    done
    for version in "${previousLatestVersions[@]}"; do
        multiArchVersionsV2+=("${version}")
    done
	multiArchVersionsV2String=""
	if [ -n "${multiArchVersionsV2[*]}" ]; then
		IFS=' ' read -r -a multiArchVersionsV2String <<< "${multiArchVersionsV2[*]}"
	fi
	
	arch=$(uname -m)
	# shellcheck disable=SC3010
	if [[ ${arch,,} == "aarch64" || ${arch,,} == "arm64"  ]]; then
		versionsToBeDownloaded="${multiArchVersionsV2String}"
	else
		versionsToBeDownloaded="${amd64OnlyVersions} ${multiArchVersionsV2String}"
	fi

	validVersions=$(curl -sL https://$downloadURL | jq .tags[])
	
	for versionToBeDownloaded in ${versionsToBeDownloaded[*]}; do
	    # shellcheck disable=SC3010
		[[ ${validVersions[*]}  =~  ${versionToBeDownloaded} ]] || (echo "${versionToBeDownloaded} does not exist in ${downloadURL}" && exit 1)
	done
done <<< "$components"
