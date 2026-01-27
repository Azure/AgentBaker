package nodeconfigutils

import (
	"encoding/base64"
	"fmt"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	cloudConfigTemplate = `#cloud-config
write_files:
- path: /opt/azure/containers/aks-node-controller-config.json
  permissions: "0755"
  owner: root
  content: !!binary |
   %s`
	CSE = "/opt/azure/containers/aks-node-controller provision-wait"
)

func CustomData(cfg *aksnodeconfigv1.Configuration) (string, error) {
	aksNodeConfigJSON, err := MarshalConfigurationV1(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal nbc, error: %w", err)
	}
	encodedAksNodeConfigJSON := base64.StdEncoding.EncodeToString(aksNodeConfigJSON)
	customDataYAML := fmt.Sprintf(cloudConfigTemplate, encodedAksNodeConfigJSON)
	return base64.StdEncoding.EncodeToString([]byte(customDataYAML)), nil
}

func MarshalConfigurationV1(cfg *aksnodeconfigv1.Configuration) ([]byte, error) {
	options := protojson.MarshalOptions{
		UseEnumNumbers: false,
		UseProtoNames:  true,
		Indent:         "  ",
	}
	return options.Marshal(cfg)
}

func UnmarshalConfigurationV1(data []byte) (*aksnodeconfigv1.Configuration, error) {
	cfg := &aksnodeconfigv1.Configuration{}
	options := protojson.UnmarshalOptions{
		DiscardUnknown: true, // ignore unknown fields to allow forward compatibility
	}
	err := options.Unmarshal(data, cfg)
	return cfg, err
}

func Validate(cfg *aksnodeconfigv1.Configuration) error {
	requiredStrings := map[string]string{
		"version":                                           cfg.GetVersion(),
		"auth_config.subscription_id":                       cfg.GetAuthConfig().GetSubscriptionId(),
		"cluster_config.resource_group":                     cfg.GetClusterConfig().GetResourceGroup(),
		"cluster_config.location":                           cfg.GetClusterConfig().GetLocation(),
		"cluster_config.cluster_network_config.vnet_name":   cfg.GetClusterConfig().GetClusterNetworkConfig().GetVnetName(),
		"cluster_config.cluster_network_config.route_table": cfg.GetClusterConfig().GetClusterNetworkConfig().GetRouteTable(),
		"api_server_config.api_server_name":                 cfg.GetApiServerConfig().GetApiServerName(),
	}

	for field, value := range requiredStrings {
		if value == "" {
			return fmt.Errorf("required field %v is missing", field)
		}
	}
	return nil
}
