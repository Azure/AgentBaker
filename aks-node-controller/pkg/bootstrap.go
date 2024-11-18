package pkg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
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
