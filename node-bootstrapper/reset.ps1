stop-service kubelet
stop-service csi-proxy
stop-service kubeproxy
stop-service containerd

remove-service kubelet
remove-service csi-proxy
remove-service kubeproxy
remove-service containerd

rm c:\k\* -Recurse
rm c:\AzureData\* -Recurse
