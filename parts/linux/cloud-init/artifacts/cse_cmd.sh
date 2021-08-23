echo $(date),$(hostname) > /var/log/azure/cluster-provision-cse-output.log;
/usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"