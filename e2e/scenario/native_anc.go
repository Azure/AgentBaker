package scenario

import (
	"encoding/base64"
	"fmt"
	"strings"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
)

// Use the same native ANC CustomData producer as RP, rather than the NBC
// producer, which always includes a command that causes the launcher to delegate.
func nativeANCCustomData(config *aksnodeconfigv1.Configuration, binaryURL string) (string, error) {
	if config == nil || config.DisableCustomData {
		return "", fmt.Errorf("native ANC requires a configuration that enables custom data rendering")
	}
	data, err := nodeconfigutils.CustomData(config)
	if err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}
	const launch = `logger -t aks-boothook "launching aks-node-controller`
	if strings.Count(string(decoded), launch) != 1 {
		return "", fmt.Errorf("expected one native ANC launch point")
	}
	// Download this checkout's ANC before the launcher starts it, as the existing
	// NBC harness does at its hotfix marker. Preserve the MIME payload and JSON.
	quotedURL := "'" + strings.ReplaceAll(binaryURL, "'", "'\"'\"'") + "'"
	download := fmt.Sprintf("curl -fSL --retry 10 --retry-delay 2 --retry-connrefused %s -o /opt/azure/containers/aks-node-controller-hotfix && chmod +x /opt/azure/containers/aks-node-controller-hotfix\n", quotedURL)
	return base64.StdEncoding.EncodeToString([]byte(strings.Replace(string(decoded), launch, download+launch, 1))), nil
}
