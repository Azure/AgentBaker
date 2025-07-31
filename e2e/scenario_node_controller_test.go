package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"
)

// test aks-node-controller binary without rebuilding VHD images.
// it compiles the aks-node-controller binary and uploads it to Azure Storage.
// the binary is then downloaded and executed on the VM
// the test results are unreliable, as there can be a version mismatch between the binary and the rest content of VHD image
// it's intended to be used for quick testing without rebuilding VHD images
// mostly executed locally
func Test_Ubuntu2204AKSNodeController(t *testing.T) {
	location := config.Config.DefaultLocation
	ctx := newTestCtx(t, location)
	if !config.Config.EnableAKSNodeControllerTest {
		t.Skip("ENABLE_AKS_NODE_CONTROLLER_TEST is not set")
	}
	// TODO: figure out how to properly parallelize test, maybe move t.Parallel to the top of each test?
	cluster, err := ClusterKubenet(ctx, location, t)
	require.NoError(t, err)
	t.Cleanup(func() {
		log, err := os.ReadFile("./scenario-logs/" + t.Name() + "/aks-node-controller.stdout.txt")
		if err != nil {
			t.Logf("failed to read aks-node-controller log: %v", err)
		}
		t.Logf("aks-node-controller log: %s", string(log))
	})
	identity, err := config.Azure.CreateVMManagedIdentity(ctx, location)
	require.NoError(t, err)
	binary := compileAKSNodeController(t)
	url, err := config.Azure.UploadAndGetLink(ctx, time.Now().UTC().Format("2006-01-02-15-04-05")+"/aks-node-controller", binary)
	require.NoError(t, err)

	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD can be properly bootstrapped",
		Config: Config{
			//NodeBootstrappingType: Scriptless,
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			VMConfigMutator: func(model *armcompute.VirtualMachineScaleSet) {
				model.Identity = &armcompute.VirtualMachineScaleSetIdentity{
					Type: to.Ptr(armcompute.ResourceIdentityTypeSystemAssignedUserAssigned),
					UserAssignedIdentities: map[string]*armcompute.UserAssignedIdentitiesValue{
						config.Config.VMIdentityResourceID(location): {},
					},
				}
				model.Properties.VirtualMachineProfile.ExtensionProfile = &armcompute.VirtualMachineScaleSetExtensionProfile{
					Extensions: []*armcompute.VirtualMachineScaleSetExtension{
						{
							Name: to.Ptr("vmssCSE"),
							Properties: &armcompute.VirtualMachineScaleSetExtensionProperties{
								Publisher:               to.Ptr("Microsoft.Azure.Extensions"),
								Type:                    to.Ptr("CustomScript"),
								TypeHandlerVersion:      to.Ptr("2.0"),
								AutoUpgradeMinorVersion: to.Ptr(true),
								Settings:                map[string]any{},
								ProtectedSettings: map[string]any{
									"fileUris":         []string{url},
									"commandToExecute": CSEAKSNodeController(t, cluster, config.VHDUbuntu2204Gen2Containerd),
									"managedIdentity": map[string]any{
										"clientId": identity,
									},
								},
							},
						},
					},
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", getExpectedPackageVersions("containerd", "ubuntu", "r2204")[0])
				ValidateInstalledPackageVersion(ctx, s, "moby-runc", getExpectedPackageVersions("runc", "ubuntu", "r2204")[0])
				ValidateFileHasContent(ctx, s, "/var/log/azure/aks-node-controller.log", "aks-node-controller finished successfully")
			},
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {},
		},
	})
}

func CSEAKSNodeController(t *testing.T, cluster *Cluster, vhd *config.Image) string {
	nbc := getBaseNBC(t, cluster, vhd)
	agent.ValidateAndSetLinuxNodeBootstrappingConfiguration(nbc)
	configContent := nbcToAKSNodeConfigV1(nbc)
	configJSON, err := json.Marshal(configContent)
	require.NoError(t, err)

	return fmt.Sprintf(`sh -c "(mkdir -p /etc/aks-node-controller && echo '%s' | base64 -d > /etc/aks-node-controller/config.json && ./aks-node-controller provision --provision-config=/etc/aks-node-controller/config.json)"`, base64.StdEncoding.EncodeToString(configJSON))
}

func compileAKSNodeController(t *testing.T) *os.File {
	cmd := exec.Command("go", "build", "-o", "aks-node-controller", "-v")
	cmd.Dir = filepath.Join("..", "aks-node-controller")
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS=linux",
		"GOARCH=amd64",
	)
	log, err := cmd.CombinedOutput()
	require.NoError(t, err, string(log))
	t.Logf("Compiled aks-node-controller")
	f, err := os.Open(filepath.Join("..", "aks-node-controller", "aks-node-controller"))
	require.NoError(t, err)
	return f
}
