package scenario

import (
	"context"
	"errors"
	"fmt"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

const serviceDiscoveryDNSIP = "172.16.0.53"

var clusterServiceDiscovery = cachedFunc(func(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model := getKubenetClusterModel("abe2e-custom-dns-v1", request.Location, request.K8sSystemPoolSKU)
	model.Properties.NetworkProfile.DNSServiceIP = to.Ptr(serviceDiscoveryDNSIP)
	return prepareCluster(ctx, model, false, false)
})

func init() {
	for _, tt := range []struct {
		name  string
		image *config.Image
	}{{"Ubuntu2204", config.VHDUbuntu2204Gen2Containerd}, {"AzureLinuxV3", config.VHDAzureLinuxV3Gen2}} {
		for _, localDNS := range []bool{true, false} {
			mode := "NoLocalDNS"
			if localDNS {
				mode = "LocalDNS"
			}
			Register(&Scenario{
				Name:        fmt.Sprintf("%s_CustomDNS_%s_ANC", tt.name, mode),
				Description: "Native ANC custom DNS: verify both LocalDNS listeners and ordinary pod service discovery",
				Tags:        Tags{Scriptless: true},
				Config: Config{
					Cluster:               clusterServiceDiscovery,
					VHD:                   tt.image,
					NativeANC:             true,
					SkipDefaultValidation: true,
					BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = localDNS
						nbc.KubeletConfig["--cluster-dns"] = podDNSForServiceDiscovery(localDNS)
					},
					AKSNodeConfigMutator: func(_ *Cluster, config *aksnodeconfigv1.Configuration) {
						config.DisableCustomData = false
						config.LocalDnsProfile.EnableLocalDns = localDNS
						config.KubeletConfig.KubeletFlags["--cluster-dns"] = podDNSForServiceDiscovery(localDNS)
						config.KubeletConfig.KubeletConfigFileConfig.ClusterDns = []string{podDNSForServiceDiscovery(localDNS)}
					},
					Validator: func(ctx context.Context, s *Scenario) error {
						nodeName, err := s.Runtime.Kube.WaitUntilNodeReady(ctx, s.Runtime.VMSSName)
						if err != nil {
							return err
						}
						s.Runtime.VM.KubeName = nodeName
						// Flags alone do not establish which provisioner actually ran.
						_, modeErr := execScriptOnVMForScenarioValidateExitCode(ctx, s, nativeANCProvisioningScript, 0, "expected native ANC provisioning")
						var listenerErr error
						if localDNS {
							listenerErr = ValidateLocalDNSServiceDiscovery(ctx, s)
						} else {
							listenerErr = ValidateLocalDNSService(ctx, s, "disabled")
						}
						return errors.Join(modeErr, listenerErr, validateDNSWorkload(ctx, s, podDNSForServiceDiscovery(localDNS)))
					},
				},
			})
		}
	}
}

func podDNSForServiceDiscovery(localDNS bool) string {
	if localDNS {
		return "169.254.10.11"
	}
	return serviceDiscoveryDNSIP
}

const nativeANCProvisioningScript = `set -eu
test ! -e /opt/azure/containers/aks-node-controller-nbc-cmd.sh
if grep -q 'Using NBC command for scriptless phase 2' /var/log/azure/aks-node-controller.output; then
  echo 'FAIL: native ANC scenario delegated to NBC'
  exit 1
fi
grep -q 'aks-node-controller finished successfully' /var/log/azure/aks-node-controller.output
`
