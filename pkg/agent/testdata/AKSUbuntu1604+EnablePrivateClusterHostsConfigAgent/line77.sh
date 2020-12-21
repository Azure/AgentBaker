[Unit]
Description=Update Labels for Kubernetes nodes
After=kubelet.service
[Service]
Restart=always
RestartSec=60

NODE_LABELS=kubernetes.azure.com/role=agent,agentpool=agent2,storageprofile=managed,storagetier=Premium_LRS,kubernetes.azure.com/cluster=resourceGroupName

ExecStart=/bin/bash /opt/azure/containers/labels.sh
#EOF
