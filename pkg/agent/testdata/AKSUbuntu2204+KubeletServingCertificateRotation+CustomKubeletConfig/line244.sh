KUBELET_FLAGS=--azure-container-registry-config=/etc/kubernetes/azure.json --cloud-config=/etc/kubernetes/azure.json --cloud-provider=azure 
KUBELET_REGISTER_SCHEDULABLE=true
NETWORK_POLICY=
KUBELET_NODE_LABELS=agentpool=agent2,kubernetes.azure.com/agentpool=agent2,kubernetes.azure.com/kubelet-serving-ca=cluster