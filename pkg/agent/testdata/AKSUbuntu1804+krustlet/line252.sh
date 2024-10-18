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
    token: "07401b.f395accd246ae52d"
contexts:
- context:
    cluster: localcluster
    user: kubelet-bootstrap
    name: bootstrap-context
current-context: bootstrap-context