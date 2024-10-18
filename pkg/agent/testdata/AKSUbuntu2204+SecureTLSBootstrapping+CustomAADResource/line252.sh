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
        - --aad-resource=appID
        interactiveMode: Never
        provideClusterInfo: true
contexts:
- context:
    cluster: localcluster
    user: kubelet-bootstrap
    name: bootstrap-context
current-context: bootstrap-context