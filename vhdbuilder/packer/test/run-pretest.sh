#!/bin/bash
set -euo pipefail

components=$(jq .ContainerImages[] --monochrome-output --compact-output < vhdbuilder/packer/components.json)
for component in ${components[*]}; do
	downloadURL=$(echo ${component} | jq .downloadURL)
	downloadURL=$(echo ${downloadURL//\*/} | jq 'sub(".com/" ; ".com/v2/") | sub(":" ; "/tags/list")' -r)
	toDownloadVersions=$(echo "${component}" | jq .versions[])

	validVersions=$(curl -L https://$downloadURL | jq .tags[])
	
	for toDownloadVersion in ${toDownloadVersions[*]}; do
		[[ ${validVersions[*]}  =~  ${toDownloadVersion} ]] || (echo "${toDownloadVersion} does not exist in ${validVersions}" && exit 1)
	done
done
