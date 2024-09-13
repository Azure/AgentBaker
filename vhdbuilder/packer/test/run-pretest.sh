#!/usr/bin/env bash
set -euo pipefail

components=$(jq .ContainerImages[] --monochrome-output --compact-output < parts/linux/cloud-init/artifacts/components.json)
for component in ${components[*]}; do
	downloadURL=$(echo ${component} | jq .downloadURL)
	downloadURL=$(echo ${downloadURL//\*/} | jq 'sub(".com/" ; ".com/v2/") | sub(":" ; "/tags/list")' -r)
	amd64OnlyVersionsStr=$(echo "${component}" | jq .amd64OnlyVersions -r)
	multiArchVersionsStr=$(echo "${component}" | jq .multiArchVersions -r)
	amd64OnlyVersions=""
	if [[ ${amd64OnlyVersionsStr} != null ]]; then
		amd64OnlyVersions=$(echo "${amd64OnlyVersionsStr}" | jq -r ".[]")
	fi
	multiArchVersions=""
	if [[ ${multiArchVersionsStr} != null ]]; then
		multiArchVersions=$(echo "${multiArchVersionsStr}" | jq -r ".[]")
	fi
	arch=$(uname -m)
	if [[ ${arch,,} == "aarch64" || ${arch,,} == "arm64"  ]]; then
		versionsToBeDownloaded="${multiArchVersions}"
	else
		versionsToBeDownloaded="${amd64OnlyVersions} ${multiArchVersions}"
	fi

	validVersions=$(curl -sL https://$downloadURL | jq .tags[])
	
	for versionToBeDownloaded in ${versionsToBeDownloaded[*]}; do
		[[ ${validVersions[*]}  =~  ${versionToBeDownloaded} ]] || (echo "${versionToBeDownloaded} does not exist in ${downloadURL}" && exit 1)
	done
done
