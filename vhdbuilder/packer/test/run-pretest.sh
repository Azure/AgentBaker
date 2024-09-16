#!/usr/bin/env bash
set -euo pipefail
set -x

components=$(jq .ContainerImages[] --monochrome-output --compact-output < parts/linux/cloud-init/artifacts/components.json)
for component in "${components[*]}"; do
	downloadURL=$(echo "${component}" | jq .downloadURL)
	downloadURL=$(echo ${downloadURL//\*/} | jq 'sub(".com/" ; ".com/v2/") | sub(":" ; "/tags/list")' -r)
	amd64OnlyVersionsStr=$(echo "${component}" | jq .amd64OnlyVersions -r)
	amd64OnlyVersions=""
	if [[ ${amd64OnlyVersionsStr} != null ]]; then
		amd64OnlyVersions=$(echo "${amd64OnlyVersionsStr}" | jq -r ".[]")
	fi
	latestVersions=(echo "${component}" | jq -r ".multiArchVersionsV2[] | select(.latestVersion != null) | .latestVersion")
    previousLatestVersions=(echo "${component}" | jq -r ".multiArchVersionsV2[] | select(.previousLatestVersion != null) | .previousLatestVersion")
    for version in "${latestVersions[@]}"; do
      multiArchVersionsV2+=("${version}")
    done
    for version in "${previousLatestVersions[@]}"; do
      multiArchVersionsV2+=("${version}")
    done

	arch=$(uname -m)
	if [[ ${arch,,} == "aarch64" || ${arch,,} == "arm64"  ]]; then
		versionsToBeDownloaded="${multiArchVersionsV2}"
	else
		versionsToBeDownloaded="${amd64OnlyVersions} ${multiArchVersionsV2}"
	fi

	validVersions=$(curl -sL https://$downloadURL | jq .tags[])
	
	for versionToBeDownloaded in ${versionsToBeDownloaded[*]}; do
		[[ ${validVersions[*]}  =~  ${versionToBeDownloaded} ]] || (echo "${versionToBeDownloaded} does not exist in ${downloadURL}" && exit 1)
	done
done
