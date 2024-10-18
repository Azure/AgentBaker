package nbcontractv1

const (
	// Follow the semantic versioning format. <Major>.<Minor>.<Patch>
	// Major version is for breaking changes, which should only be updated by the contract owner.
	// Minor version is for minor changes that doesn't break the API. Feature owner should update this when adding a new feature.
	// Patch version is for bug fixes. This should be updated when a patch is released.
	contractVersion = "1.0.0"
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
