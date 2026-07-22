echo $(date),$(hostname) > ${PROVISION_OUTPUT};
if [ -f "${INIT_AKS_CLOUD_FILEPATH}" ]; then
	REPO_DEPOT_ENDPOINT="${REPO_DEPOT_ENDPOINT}" LOCATION="${LOCATION}" "${INIT_AKS_CLOUD_FILEPATH}" >> /var/log/azure/cluster-provision.log 2>&1 || exit $?;
fi;
{{/* Keep LOCATION inline with nohup. */ -}}
{{/* BuildCSECmd flattens this template into one shell command, so this assignment is passed to nohup. */ -}}
{{/* Be careful not to add runtime control flow or command separators that break the flattening logic. */ -}}
LOCATION="${LOCATION}" /usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"
