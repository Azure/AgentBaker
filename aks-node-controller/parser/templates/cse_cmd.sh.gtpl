echo $(date),$(hostname) > ${PROVISION_OUTPUT};
{{if getIsAksCustomCloud .CustomCloudConfig}}
REPO_DEPOT_ENDPOINT="{{.CustomCloudConfig.RepoDepotEndpoint}}"
{{end}}
LOCATION="{{getCloudLocation .}}"
{{getInitAKSCustomCloudFilepath}} >> /var/log/azure/cluster-provision.log 2>&1;
/usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"
