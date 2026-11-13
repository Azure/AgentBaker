echo $(date),$(hostname) > ${PROVISION_OUTPUT};
if [ -f /run/aks/init-aks-cloud.status ]; then
	read -r INIT_AKS_CLOUD_STATUS < /run/aks/init-aks-cloud.status;
	[ "${INIT_AKS_CLOUD_STATUS}" = 0 ] || exit "${INIT_AKS_CLOUD_STATUS:-1}";
elif [ -f "${INIT_AKS_CLOUD_FILEPATH}" ]; then
	"${INIT_AKS_CLOUD_FILEPATH}" >> /var/log/azure/cluster-provision.log 2>&1 || exit $?;
fi;
/usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"
