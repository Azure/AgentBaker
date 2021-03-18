#!/bin/bash
set -euo pipefail

components=$(jq .ContainerImages[] --monochrome-output --compact-output < vhdbuilder/packer/components.json)
for component in ${components[*]}; do
	downloadURL=$(echo ${component} | jq .downloadURL)
	downloadURL=$(echo ${downloadURL//\*/} | jq 'sub(".com/" ; ".com/v2/") | sub(":" ; "/tags/list")' -r)
	versionsToBeDownloaded=$(echo "${component}" | jq .versions[])

	validVersions=$(curl -sL https://$downloadURL | jq .tags[])
	
	for versionToBeDownloaded in ${versionsToBeDownloaded[*]}; do
		[[ ${validVersions[*]}  =~  ${versionToBeDownloaded} ]] || (echo "${versionToBeDownloaded} does not exist in ${downloadURL}" && exit 1)
	done
done
