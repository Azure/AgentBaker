package e2e

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/barkimedes/go-deepcopy"
	"github.com/stretchr/testify/require"
)

// test node-bootstrapper binary without rebuilding VHD images.
// it compiles the node-bootstrapper binary and uploads it to Azure Storage.
// the binary is then downloaded and executed on the VM
// the test results are unreliable, as there can be a version mismatch between the binary and the rest content of VHD image
// it's intended to be used for quick testing without rebuilding VHD images
// mostly executed locally
func Test_ubuntu2204NodeBootstrapper(t *testing.T) {
	ctx := newTestCtx(t)
	if !config.Config.EnableNodeBootstrapperTest {
		t.Skip("ENABLE_NODE_BOOTSTRAPPER_TEST is not set")
	}
	// TODO: figure out how to properly parallelize test, maybe move t.Parallel to the top of each test?
	cluster, err := ClusterKubenet(ctx, t)
	require.NoError(t, err)
	t.Cleanup(func() {
		log, err := os.ReadFile("./scenario-logs/" + t.Name() + "/node-bootstrapper.stdout.txt")
		if err != nil {
			t.Logf("failed to read node-bootstrapper log: %v", err)
		}
		t.Logf("node-bootstrapper log: %s", string(log))
	})
	identity, err := config.Azure.CreateVMManagedIdentity(ctx)
	require.NoError(t, err)
	binary := compileNodeBootstrapper(t)
	url, err := config.Azure.UploadAndGetLink(ctx, time.Now().Format("2006-01-02-15-04-05")+"/node-bootstrapper", binary)
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
						fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ManagedIdentity/userAssignedIdentities/%s", config.Config.SubscriptionID, config.ResourceGroupName, config.VMIdentityName): {},
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
									"commandToExecute": CSENodeBootstrapper(t, cluster),
									"managedIdentity": map[string]any{
										"clientId": identity,
									},
								},
							},
						},
					},
				}
			},
			LiveVMValidators: []*LiveVMValidator{
				mobyComponentVersionValidator("containerd", getExpectedPackageVersions("containerd", "ubuntu", "r2204")[0], "apt"),
				mobyComponentVersionValidator("runc", getExpectedPackageVersions("runc", "ubuntu", "r2204")[0], "apt"),
				FileHasContentsValidator("/var/log/azure/node-bootstrapper.log", "node-bootstrapper finished successfully"),
			},
			AKSNodeConfigMutator: func(config *nbcontractv1.Configuration) {},
		},
	})
}

func CSENodeBootstrapper(t *testing.T, cluster *Cluster) string {
	nbcAny, err := deepcopy.Anything(cluster.NodeBootstrappingConfiguration)
	require.NoError(t, err)
	nbc := nbcAny.(*datamodel.NodeBootstrappingConfiguration)
	agent.ValidateAndSetLinuxNodeBootstrappingConfiguration(nbc)

	configContent := nbcToNbcContractV1(nbc)

	configJSON, err := json.Marshal(configContent)
	require.NoError(t, err)

	return fmt.Sprintf(`sh -c "(mkdir -p /etc/node-bootstrapper && echo '%s' | base64 -d > /etc/node-bootstrapper/config.json && ./node-bootstrapper provision --provision-config=/etc/node-bootstrapper/config.json)"`, base64.StdEncoding.EncodeToString(configJSON))
}

func compileNodeBootstrapper(t *testing.T) *os.File {
	cmd := exec.Command("go", "build", "-o", "node-bootstrapper", "-v")
	cmd.Dir = filepath.Join("..", "node-bootstrapper")
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS=linux",
		"GOARCH=amd64",
	)
	log, err := cmd.CombinedOutput()
	require.NoError(t, err, string(log))
	t.Logf("Compiled node-bootstrapper")
	f, err := os.Open(filepath.Join("..", "node-bootstrapper", "node-bootstrapper"))
	require.NoError(t, err)
	return f
}
