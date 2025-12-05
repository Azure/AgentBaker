echo $(date),$(hostname) > ${PROVISION_OUTPUT};
{{if not .GetDisableCustomData}}
CLOUD_INIT_STATUS_SCRIPT="/opt/azure/containers/cloud-init-status-check.sh";
cloudInitExitCode=0;
if [ -f "${CLOUD_INIT_STATUS_SCRIPT}" ]; then
	/bin/bash -c "source ${CLOUD_INIT_STATUS_SCRIPT}; handleCloudInitStatus \"${PROVISION_OUTPUT}\"; returnStatus=\$?; echo \"Cloud init status check exit code: \$returnStatus\" >> ${PROVISION_OUTPUT}; exit \$returnStatus" >> ${PROVISION_OUTPUT} 2>&1;
else
    cloud-init status --wait > /dev/null 2>&1;
fi;
cloudInitExitCode=$?;
if [ "$cloudInitExitCode" -eq 0 ]; then
	echo "cloud-init succeeded" >> ${PROVISION_OUTPUT};
else
	echo "cloud-init failed with exit code ${cloudInitExitCode}" >> ${PROVISION_OUTPUT};
	exit ${cloudInitExitCode};
fi;
{{end}}
{{if getIsAksCustomCloud .CustomCloudConfig}}
REPO_DEPOT_ENDPOINT="{{.CustomCloudConfig.RepoDepotEndpoint}}"
{{getInitAKSCustomCloudFilepath}} >> /var/log/azure/cluster-provision.log 2>&1;
{{end}}
/usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"
