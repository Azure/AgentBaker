apiVersion: v1
kind: Config
clusters:
- name: localcluster
    cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://:443
users:
- name: kubelet-bootstrap
    user:
    exec:
        apiVersion: client.authentication.k8s.io/v1
        command: /opt/azure/tlsbootstrap/tls-bootstrap-client
        args:
        - bootstrap
        - --next-proto=aks-tls-bootstrap
        - --aad-resource=6dae42f8-4368-4678-94ff-3960e28e3630
        interactiveMode: Never
        provideClusterInfo: true
contexts:
- context:
    cluster: localcluster
    user: kubelet-bootstrap
    name: bootstrap-context
current-context: bootstrap-context