package pkg

import (
	"fmt"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
)

func Validate(cfg *aksnodeconfigv1.Configuration) error {
	requiredStrings := map[string]string{
		"AuthConfig.SubscriptionId":                     cfg.GetAuthConfig().GetSubscriptionId(),
		"ClusterConfig.ResourceGroup":                   cfg.GetClusterConfig().GetResourceGroup(),
		"ClusterConfig.Location":                        cfg.GetClusterConfig().GetLocation(),
		"ClusterConfig.ClusterNetworkConfig.VnetName":   cfg.GetClusterConfig().GetClusterNetworkConfig().GetVnetName(),
		"ClusterConfig.ClusterNetworkConfig.RouteTable": cfg.GetClusterConfig().GetClusterNetworkConfig().GetRouteTable(),
		"ApiServerConfig.ApiServerName":                 cfg.GetApiServerConfig().GetApiServerName(),
	}

	for field, value := range requiredStrings {
		if value == "" {
			return fmt.Errorf("required field %v is missing", field)
		}
	}
	return nil
}
