echo $(date),$(hostname) > ${PROVISION_OUTPUT};
{{if not .GetDisableCustomData}}
. /does/not/exist.sh
/bin/bash -c "source /opt/azure/containers/cloud-init-status-check.sh; handleCloudInitStatus \"${PROVISION_OUTPUT}\"; returnStatus=\$?; echo \"Cloud init status check exit code: \$returnStatus\" >> ${PROVISION_OUTPUT}; exit \$returnStatus" >> ${PROVISION_OUTPUT} 2>&1;
cloudInitExitCode=$?;
[ "$cloudInitExitCode" -ne 0 ] && echo "cloud-init failed with exit code ${cloudInitExitCode}" >> ${PROVISION_OUTPUT} && exit ${cloudInitExitCode};
{{end}}
{{if getIsAksCustomCloud .CustomCloudConfig}}
REPO_DEPOT_ENDPOINT="{{.CustomCloudConfig.RepoDepotEndpoint}}"
{{getInitAKSCustomCloudFilepath}} >> /var/log/azure/cluster-provision.log 2>&1;
{{end}}
/usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"