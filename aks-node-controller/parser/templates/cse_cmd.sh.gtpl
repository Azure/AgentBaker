echo $(date),$(hostname) > ${PROVISION_OUTPUT};
if [ -f "${INIT_AKS_CLOUD_FILEPATH}" ]; then
	REPO_DEPOT_ENDPOINT="${REPO_DEPOT_ENDPOINT}" LOCATION="${LOCATION}" "${INIT_AKS_CLOUD_FILEPATH}" >> /var/log/azure/cluster-provision.log 2>&1 || exit $?;
fi;
{{/*
Keep LOCATION inline with nohup; BuildCSECmd flattens this template into one shell command,
so inserting runtime control flow or command separators here can stop LOCATION from reaching provision_start.sh.
*/}}LOCATION="${LOCATION}" /usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"
