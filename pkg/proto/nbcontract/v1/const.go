package nbcontractv1

const (
	// Follow the semantic versioning format. V<Major>.<Minor>.<Patch>
	// Major version is for breaking changes, which should only be updated by the contract owner.
	// Minor version is for minor changes that doesn't break the API. Feature owner should update this when adding a new feature.
	// Patch version is for bug fixes. This should be updated when a patch is released.
	contractVersion = "1.0.0"

	// AzureChinaCloud is the cloud name for Azure China Cloud.
	AzureChinaCloud = "AzureChinaCloud"

	// DefaultCloudName is the default cloud name.
	DefaultCloudName = "AzurePublicCloud"

	// VMTypeStandard is the standard VM type for the AKS cluster.
	VMTypeStandard = "standard"

	// VMTypeVMSS is the VMSS (Virtual Machine Scale Set) VM type for the AKS cluster.
	VMTypeVMSS = "vmss"

	// NetworkPluginAzure is the Azure network plugin for Kubernetes.
	NetworkPluginAzure = "azure"

	// NetworkPluginkubenet is the default network plugin for Kubernetes.
	NetworkPluginkubenet = "kubenet"

	// NetworkPolicyAzure is the Azure network policy for Kubernetes.
	NetworkPolicyAzure = "azure"

	// NetworkPolicyCalico is the Calico network policy for the AKS cluster.
	NetworkPolicyCalico = "calico"

	// LoadBalancerBasic is the basic tier load balancer for the AKS cluster.
	LoadBalancerBasic = "Basic"

	// LoadBalancerStandard is the standard tier load balancer for the AKS cluster.
	LoadBalancerStandard = "Standard"

	// VMSizeStandardDc2s is the standard_dc2s VM size for the AKS cluster.
	VMSizeStandardDc2s = "Standard_DC2s"

	// VMSizeStandardDc4s is the standard_dc4s VM size for the AKS cluster.
	VMSizeStandardDc4s = "Standard_DC4s"

	// DefaultLinuxUser is the default Linux user name for each node in the AKS cluster.
	DefaultLinuxUser = "azureuser"
)
