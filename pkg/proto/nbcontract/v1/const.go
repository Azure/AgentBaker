package nbcontractv1

const (
	// Follow the semantic versioning format. <Major>.<Minor>.<Patch> //nolint: godot // no period is required after patch.
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
