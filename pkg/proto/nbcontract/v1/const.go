package nbcontractv1

const (
	// Follow the semantic versioning format. V<Major>.<Minor>.<Patch>
	// Major version is for breaking changes, which should only be updated by the contract owner.
	// Minor version is for minor changes that doesn't break the API. Feature owner should update this when adding a new feature.
	// Patch version is for bug fixes. This should be updated when a patch is released.
	contractVersion = "v1.0.0"
)

const (
	VmTypeStandard       = "standard"
	VmTypeVmss           = "vmss"
	NetworkPluginAzure   = "azure"
	NetworkPluginKubenet = "kubenet"
	NetworkPolicyAzure   = "azure"
	NetworkPolicyCalico  = "calico"
	LoadBalancerBasic    = "basic"
	LoadBalancerStandard = "Standard"
	VmSizeStandardDc2s   = "Standard_DC2s"
	VmSizeStandardDc4s   = "Standard_DC4s"
	DefaultLinuxUser     = "azureuser"
	DefaultCloudName     = "AzurePublicCloud"
	AksCustomCloudName   = "akscustom"
	AzureStackCloud      = "AzureStackCloud"
)
