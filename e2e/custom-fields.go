package e2e

// TODO 1: Rename some variables better?

type customDataFields struct {
	Cloud                             string  `json:"cloud"`
	TenantId                          string  `json:"tenantId"`
	SubscriptionId                    string  `json:"subscriptionId"`
	AadClientId                       string  `json:"aadClientId"`
	AadClientSecret                   string  `json:"aadClientSecret"`
	ResourceGroup                     string  `json:"resourceGroup"`
	Location                          string  `json:"location"`
	VmType                            string  `json:"vmType"`
	SubnetName                        string  `json:"subnetName"`
	SecurityGroupName                 string  `json:"securityGroupName"`
	VnetName                          string  `json:"vnetName"`
	VnetResourceGroup                 string  `json:"vnetResourceGroup"`
	RouteTableName                    string  `json:"routeTableName"`
	PrimaryAvailabilitySetName        string  `json:"primaryAvailabilitySetName"`
	PrimaryScaleSetName               string  `json:"primaryScaleSetName"`
	CloudProviderBackoffMode          string  `json:"cloudProviderBackoffMode"`
	CloudProviderBackoff              bool    `json:"cloudProviderBackoff"`
	CloudProviderBackoffRetries       int     `json:"cloudProviderBackoffRetries"`
	CloudProviderBackoffDuration      int     `json:"cloudProviderBackoffDuration"`
	CloudProviderRateLimit            bool    `json:"cloudProviderRateLimit"`
	CloudProviderRateLimitQPS         float64 `json:"cloudProviderRateLimitQPS"`
	CloudProviderRateLimitBucket      int     `json:"cloudProviderRateLimitBucket"`
	CloudProviderRateLimitQPSWrite    float64 `json:"cloudProviderRateLimitQPSWrite"`
	CloudProviderRateLimitBucketWrite int     `json:"cloudProviderRateLimitBucketWrite"`
	UseManagedIdentityExtension       bool    `json:"useManagedIdentityExtension"`
	UserAssignedIdentityID            string  `json:"userAssignedIdentityID"`
	UseInstanceMetadata               bool    `json:"useInstanceMetadata"`
	LoadBalancerSku                   string  `json:"loadBalancerSku"`
	DisableOutboundSNAT               bool    `json:"disableOutboundSNAT"`
	ExcludeMasterFromStandardLB       bool    `json:"excludeMasterFromStandardLB"`
	ProviderVaultName                 string  `json:"providerVaultName"`
	MaximumLoadBalancerRuleCount      int     `json:"maximumLoadBalancerRuleCount"`
	ProviderKeyName                   string  `json:"providerKeyName"`
	ProviderKeyVersion                string  `json:"providerKeyVersion"`
	Apiservercert                     string  `json:"apiserver.crt"`
	Cacert                            string  `json:"ca.crt"`
	Clientkey                         string  `json:"client.key"`
	Clientcert                        string  `json:"client.crt"`
	Fqdn                              string  `json:"fqdn"`
	Mode                              string  `json:"mode"`
	Name		                      string  `json:"nodepoolname"`
	NodeImageVersion                  string  `json:"nodeImageVersion"`
	TenantID                          string  `json:"tenantID"`
	MCRGName                          string  `json:"mcRGName"`
	ClusterID                         string  `json:"clusterID"`
	SubID                             string  `json:"subID"`
	TLSBootstrapToken                 string  `json:"tlsbootstraptoken"`
}