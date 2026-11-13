echo $(date),$(hostname) > ${PROVISION_OUTPUT};
if [ -f "${INIT_AKS_CLOUD_FILEPATH}" ] && [ ! -f /run/aks/init-aks-cloud.done ]; then
	"${INIT_AKS_CLOUD_FILEPATH}" >> /var/log/azure/cluster-provision.log 2>&1 || exit $?;
fi;
/usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"
