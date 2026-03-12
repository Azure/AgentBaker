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

mkdir -p /opt/azure/containers

cat <<'EOF' | base64 -d >/opt/azure/containers/aks-node-controller-config.json
%s
EOF
chmod 0644 /opt/azure/containers/aks-node-controller-config.json

cat <<'EOF' >/etc/systemd/system/aks-node-controller.service
[Unit]
Description=Parse contract and run csecmd
ConditionPathExists=/opt/azure/containers/aks-node-controller-config.json
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/opt/azure/containers/aks-node-controller-wrapper.sh
RemainAfterExit=yes
EOF

systemctl daemon-reload
systemctl enable aks-node-controller.service
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
     },
     "systemd": {
       "units": [{
         "name": "aks-node-controller.service",
         "enabled": true,
         "contents": "[Unit]\nConditionPathExists=/opt/azure/containers/aks-node-controller-config.json\nAfter=network-online.target\nWants=network-online.target\n\n[Service]\nType=oneshot\nExecStart=/opt/azure/containers/aks-node-controller-wrapper.sh\nRemainAfterExit=yes\n\n[Install]\nWantedBy=multi-user.target"
       }]
     }
   }`
)

func CustomData(cfg *aksnodeconfigv1.Configuration) (string, error) {
	aksNodeConfigJSON, err := MarshalConfigurationV1(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal nbc, error: %w", err)
	}

	encodedAksNodeConfigJSON := base64.StdEncoding.EncodeToString(aksNodeConfigJSON)
	boothook := fmt.Sprintf(boothookTemplate, encodedAksNodeConfigJSON)

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

func CustomDataFlatcar(cfg *aksnodeconfigv1.Configuration) (string, error) {
	aksNodeConfigJSON, err := MarshalConfigurationV1(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal nbc, error: %w", err)
	}

	encodedAksNodeConfigJSON := base64.StdEncoding.EncodeToString(aksNodeConfigJSON)
	customDataYAML := fmt.Sprintf(flatcarTemplate, encodedAksNodeConfigJSON)
	return base64.StdEncoding.EncodeToString([]byte(customDataYAML)), nil
}

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
