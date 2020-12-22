[Unit]
Description=Update Labels for Kubernetes nodes
After=kubelet.service
[Service]
Restart=always
RestartSec=60

Environment="NODE_LABELS=kubernetes.azure.com/role=agent,node-role.kubernetes.io/agent=,kubernetes.io/role=agent,agentpool=agent2,storageprofile=managed,storagetier=Premium_LRS,kubernetes.azure.com/cluster=resourceGroupName"

ExecStart=/bin/bash /opt/azure/containers/labels.sh
#EOF
