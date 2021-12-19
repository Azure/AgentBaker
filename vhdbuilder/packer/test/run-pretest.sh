#!/usr/bin/env bash
set -euo pipefail

components=$(jq .ContainerImages[] --monochrome-output --compact-output < vhdbuilder/packer/components.json)
for component in ${components[*]}; do
	downloadURL=$(echo ${component} | jq .downloadURL)
	downloadURL=$(echo ${downloadURL//\*/} | jq 'sub(".com/" ; ".com/v2/") | sub(":" ; "/tags/list")' -r)
	amd64OnlyVersionsStr=$(echo "${imageToBePulled}" | jq .amd64OnlyVersions -r)
	multiArchVersionsStr=$(echo "${imageToBePulled}" | jq .multiArchVersions -r)
	amd64OnlyVersions=""
	if [[ ${amd64OnlyVersionsStr} != null ]]; then
		amd64OnlyVersions=$(echo "${amd64OnlyVersionsStr}" | jq -r ".[]")
	fi
	multiArchVersions=""
	if [[ ${multiArchVersionsStr} != null ]]; then
		multiArchVersions=$(echo "${multiArchVersionsStr}" | jq -r ".[]")
	fi
	versionsToBeDownloaded="${amd64OnlyVersions} ${multiArchVersions}"

	validVersions=$(curl -sL https://$downloadURL | jq .tags[])
	
	for versionToBeDownloaded in ${versionsToBeDownloaded[*]}; do
		[[ ${validVersions[*]}  =~  ${versionToBeDownloaded} ]] || (echo "${versionToBeDownloaded} does not exist in ${downloadURL}" && exit 1)
	done
done
