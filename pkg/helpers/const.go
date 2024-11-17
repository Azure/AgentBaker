package helpers

const (
	contractVersion = "v0"
)

const (
	VMTypeStandard       = "standard"
	VMTypeVmss           = "vmss"
	NetworkPluginAzure   = "azure"
	NetworkPluginKubenet = "kubenet"
	NetworkPolicyAzure   = "azure"
	NetworkPolicyCalico  = "calico"
	LoadBalancerBasic    = "basic"
	LoadBalancerStandard = "Standard"
	VMSizeStandardDc2s   = "Standard_DC2s"
	VMSizeStandardDc4s   = "Standard_DC4s"
	DefaultLinuxUser     = "azureuser"
	DefaultCloudName     = "AzurePublicCloud"
	AksCustomCloudName   = "akscustom"
	AzureStackCloud      = "AzureStackCloud"
)

const (
	scriptlessBootstrapStatusCSE = "/opt/azure/containers/aks-node-controller provision-wait"
	scriptlessCustomDataTemplate = `#cloud-config
write_files:
- path: /opt/azure/containers/aks-node-controller-config.json
  permissions: "0755"
  owner: root
  content: !!binary |
   %s`
)
