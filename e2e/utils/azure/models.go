package azure

type listVMSSVMNetworkInterfaceResult struct {
	Value []struct {
		Name       string `json:"name,omitempty"`
		ID         string `json:"id,omitempty"`
		Properties struct {
			ProvisioningState string `json:"provisioningState,omitempty"`
			IPConfigurations  []struct {
				Name       string `json:"name,omitempty"`
				ID         string `json:"id,omitempty"`
				Properties struct {
					ProvisioningState         string `json:"provisioningState,omitempty"`
					PrivateIPAddress          string `json:"privateIPAddress,omitempty"`
					PrivateIPAllocationMethod string `json:"privateIPAllocationMethod,omitempty"`
					PublicIPAddress           struct {
						ID string `json:"id,omitempty"`
					} `json:"publicIPAddress,omitempty"`
					Subnet struct {
						ID string `json:"id,omitempty"`
					} `json:"subnet,omitempty"`
					Primary                         bool   `json:"primary,omitempty"`
					PrivateIPAddressVersion         string `json:"privateIPAddressVersion,omitempty"`
					LoadBalancerBackendAddressPools []struct {
						ID string `json:"id,omitempty"`
					} `json:"loadBalancerBackendAddressPools,omitempty"`
					LoadBalancerInboundNatRules []struct {
						ID string `json:"id,omitempty"`
					} `json:"loadBalancerInboundNatRules,omitempty"`
				} `json:"properties,omitempty"`
			} `json:"ipConfigurations,omitempty"`
			DNSSettings struct {
				DNSServers               []interface{} `json:"dnsServers,omitempty"`
				AppliedDNSServers        []interface{} `json:"appliedDnsServers,omitempty"`
				InternalDomainNameSuffix string        `json:"internalDomainNameSuffix,omitempty"`
			} `json:"dnsSettings,omitempty"`
			MacAddress                  string `json:"macAddress,omitempty"`
			EnableAcceleratedNetworking bool   `json:"enableAcceleratedNetworking,omitempty"`
			EnableIPForwarding          bool   `json:"enableIPForwarding,omitempty"`
			NetworkSecurityGroup        struct {
				ID string `json:"id,omitempty"`
			} `json:"networkSecurityGroup,omitempty"`
			Primary        bool `json:"primary,omitempty"`
			VirtualMachine struct {
				ID string `json:"id,omitempty"`
			} `json:"virtualMachine,omitempty"`
		} `json:"properties,omitempty"`
	} `json:"value,omitempty"`
}
