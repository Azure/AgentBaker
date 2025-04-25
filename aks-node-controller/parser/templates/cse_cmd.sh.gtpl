echo $(date),$(hostname) > ${PROVISION_OUTPUT};
{{if not .GetDisableCustomData}}
cloud-init status --wait > /dev/null 2>&1;
[ "$?" -ne 0 ] && echo 'cloud-init failed' >> ${PROVISION_OUTPUT} && exit 1;
echo "cloud-init succeeded" >> ${PROVISION_OUTPUT};
{{end}}
{{if getIsAksCustomCloud .CustomCloudConfig}}
REPO_DEPOT_ENDPOINT="{{.CustomCloudConfig.RepoDepotEndpoint}}"
{{getInitAKSCustomCloudFilepath}} >> /var/log/azure/cluster-provision.log 2>&1;
{{end}}
/usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"