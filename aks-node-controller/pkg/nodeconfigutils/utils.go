package nodeconfigutils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net/textproto"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	CSE = "/opt/azure/containers/aks-node-controller provision-wait"

	boothookTemplate = `#cloud-boothook
#!/bin/bash
set -euo pipefail

logger -t aks-boothook "boothook start $(date -Ins)"

mkdir -p /opt/azure/containers

cat <<'EOF' | base64 -d >/opt/azure/containers/aks-node-controller-config.json
%s
EOF
chmod 0644 /opt/azure/containers/aks-node-controller-config.json

cat <<'EOF' | base64 -d >/opt/azure/containers/aks-node-controller-nbc-cmd.sh
%s
EOF
chmod 0755 /opt/azure/containers/aks-node-controller-nbc-cmd.sh

logger -t aks-boothook "launching aks-node-controller service $(date -Ins)"
systemctl start --no-block aks-node-controller.service
`

	cloudConfigTemplate = `#cloud-config
runcmd:
- echo "AKS Node Controller cloud-init completed at $(date)"
`

	flatcarTemplate = `{
     "ignition": { "version": "3.4.0" },
     "storage": {
       "files": [{
         "path": "/opt/azure/containers/aks-node-controller-config.json",
         "mode": 420,
         "contents": { "source": "data:;base64,%s" }
       }]
     }
    }`
)

// CustomData builds a base64-encoded MIME multipart document to be used as VM custom data for cloud-init.
// It encodes the node configuration as JSON, embeds it in a cloud-boothook script that writes the config
// to disk and starts the aks-node-controller service, then pairs it with a cloud-config part. Cloud-init
// processes each MIME part according to its Content-Type during the VM's first boot.
func CustomData(cfg *aksnodeconfigv1.Configuration, nbcCMD string) (string, error) {
	aksNodeConfigJSON, err := MarshalConfigurationV1(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal nbc, error: %w", err)
	}

	encodedAksNodeConfigJSON := base64.StdEncoding.EncodeToString(aksNodeConfigJSON)
	boothook := fmt.Sprintf(boothookTemplate, encodedAksNodeConfigJSON, nbcCMD)

	var customData bytes.Buffer
	writer := multipart.NewWriter(&customData)

	fmt.Fprintf(&customData, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&customData, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", writer.Boundary())

	if err := writeMIMEPart(writer, "text/cloud-boothook", boothook); err != nil {
		return "", fmt.Errorf("failed to write boothook part: %w", err)
	}
	if err := writeMIMEPart(writer, "text/cloud-config", cloudConfigTemplate); err != nil {
		return "", fmt.Errorf("failed to write cloud-config part: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize multipart custom data: %w", err)
	}

	return base64.StdEncoding.EncodeToString(customData.Bytes()), nil
}

// CustomDataFlatcar builds base64-encoded custom data for Flatcar Container Linux nodes.
// Unlike Ubuntu/Azure Linux which use cloud-init and expect MIME multipart custom data,
// Flatcar uses Ignition (configured via Butane) to process machine configuration. Ignition
// consumes a JSON document that declaratively specifies files to write to disk, so we embed
// the node config directly as a base64 data URI in an Ignition storage entry instead of
// wrapping it in a MIME multipart boothook script.
func CustomDataFlatcar(cfg *aksnodeconfigv1.Configuration) (string, error) {
	aksNodeConfigJSON, err := MarshalConfigurationV1(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal nbc, error: %w", err)
	}

	encodedAksNodeConfigJSON := base64.StdEncoding.EncodeToString(aksNodeConfigJSON)
	customDataYAML := fmt.Sprintf(flatcarTemplate, encodedAksNodeConfigJSON)
	return base64.StdEncoding.EncodeToString([]byte(customDataYAML)), nil
}

// writeMIMEPart writes a single part to a MIME multipart message. Cloud-init expects custom data
// as a MIME multipart document where each part carries a Content-Type that tells cloud-init how to
// process it (e.g. "text/cloud-boothook" for early-boot scripts, "text/cloud-config" for declarative
// cloud-config YAML). This helper creates one such part with the appropriate headers.
func writeMIMEPart(writer *multipart.Writer, contentType, content string) error {
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", contentType)
	header.Set("MIME-Version", "1.0")
	header.Set("Content-Transfer-Encoding", "7bit")

	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}

	_, err = part.Write([]byte(content))
	return err
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
