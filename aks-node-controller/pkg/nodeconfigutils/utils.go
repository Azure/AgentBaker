package nodeconfigutils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	scriptlessCustomDataTemplate = `#cloud-config
write_files:
- path: /opt/azure/containers/aks-node-controller-config.json
  permissions: "0755"
  owner: root
  content: !!binary |
   %s`
	scriptlessBootstrapStatusCSE = "/opt/azure/containers/aks-node-controller provision-wait"
)

func CustomData(cfg *aksnodeconfigv1.Configuration) (string, error) {
	nbcJSON, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal nbc, error: %w", err)
	}
	encodedNBCJson := base64.StdEncoding.EncodeToString(nbcJSON)
	customDataYAML := fmt.Sprintf(scriptlessCustomDataTemplate, encodedNBCJson)
	return base64.StdEncoding.EncodeToString([]byte(customDataYAML)), nil
}

func CSE(cfg *aksnodeconfigv1.Configuration) (string, error) {
	return scriptlessBootstrapStatusCSE, nil
}

func Marshal(cfg *aksnodeconfigv1.Configuration) ([]byte, error) {
	options := protojson.MarshalOptions{
		UseEnumNumbers: false,
		UseProtoNames:  true,
		Indent:         "  ",
	}
	return options.Marshal(cfg)
}

func Unmarshal(data []byte) (*aksnodeconfigv1.Configuration, error) {
	cfg := &aksnodeconfigv1.Configuration{}
	options := protojson.UnmarshalOptions{}
	err := options.Unmarshal(data, cfg)
	return cfg, err
}

func Validate(cfg *aksnodeconfigv1.Configuration) error {
	requiredStrings := map[string]string{
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
