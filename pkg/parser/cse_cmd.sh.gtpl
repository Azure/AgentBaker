ADMINUSER="{{.LinuxAdminUsername}}"
PROVISION_OUTPUT="{{.ProvisionOutput}}"
{{if .CustomCloudConfig.IsCustomCloud}}
REPO_DEPOT_ENDPOINT="{{.RepoDepotEndpoint}}"
{{/* need to properly add {{GetInitAKSCustomCloudFilepath}} >> /var/log/azure/cluster-provision.log 2>&1; */}}
{{end}}
MOBY_VERSION="{{.MobyVersion}}"
TENANT_ID="{{.TenantId}}"
KUBERNETES_VERSION="{{.KubernetesVersion}}"
HYPERKUBE_URL="{{.HyperkubeUrl}}"
KUBE_BINARY_URL="{{.KubeBinaryUrl}}"
CUSTOM_KUBE_BINARY_URL="{{.CustomKubeBinaryUrl}}"
KUBEPROXY_URL="{{.KubeproxyUrl}}"

DISABLE_SSH="{{getInverseBoolStr .SshStatus}}"
IS_CUSTOM_CLOUD={{.CustomCloudConfig.IsCustomCloud}}