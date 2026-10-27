package scenario

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCloudInitializationScenariosSelectActualExecutionMode(t *testing.T) {
	for _, mode := range []struct {
		name          string
		native, optIn bool
	}{
		{"native opt-in", true, true},
		{"native opt-out", true, false},
		{"NBC precedence with config present", false, true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			s := newCloudInitializationScenario("test", config.VHDUbuntu2204Gen2Containerd, mode.native, mode.optIn)
			require.True(t, s.Tags.NativeCloudInit)
			require.True(t, s.Tags.RCV1PCertMode)
			require.True(t, s.CloudInitializationFixture)
			require.False(t, s.SkipDefaultValidation, "Node Ready and pod checks must remain enabled")
			require.Empty(t, s.CustomDataWriteFiles, "write_files would disable native provisioning")
			nbc := &datamodel.NodeBootstrappingConfiguration{
				ContainerService: &datamodel.ContainerService{Properties: &datamodel.Properties{}},
			}
			s.BootstrapConfigMutator(nil, nbc)
			require.Equal(t, mode.native, nbc.EnableScriptlessAKSNodeConfig)
			require.Equal(t, !mode.native, nbc.EnableScriptlessNBCCSECmd)
			require.False(t, nbc.EnableScriptlessCSECmd)
			require.True(t, nbc.ContainerService.IsAKSCustomCloud())
			require.Equal(t, cloudInitRepositoryEndpoint, nbc.ContainerService.Properties.CustomCloudEnv.RepoDepotEndpoint)
			cfg := &aksnodeconfigv1.Configuration{}
			s.AKSNodeConfigMutator(nil, cfg)
			require.Equal(t, helpers.AksCustomCloudName, cfg.CustomCloudConfig.CustomCloudEnvName)
			require.Equal(t, cloudInitRepositoryEndpoint, cfg.CustomCloudConfig.RepoDepotEndpoint)
			var environment datamodel.CustomCloudEnv
			require.NoError(t, json.Unmarshal([]byte(cfg.CustomCloudConfig.CustomEnvJsonContent), &environment))
			require.Equal(t, datamodel.AzureStackCloud, environment.Name)
			require.Equal(t, "https://management.azure.com/", environment.ResourceManagerEndpoint)
			require.Equal(t, "mcr.microsoft.com", environment.McrURL)
			vmss := &armcompute.VirtualMachineScaleSet{}
			if mode.optIn {
				require.NotNil(t, s.VMConfigMutator)
				s.VMConfigMutator(vmss)
				require.Equal(t, "true", *vmss.Tags[rcv1pOptInTag])
			} else {
				require.Nil(t, s.VMConfigMutator, "opt-out must not accidentally set the ARM opt-in tag")
			}
		})
	}
}

func TestCloudInitializationFixtureRendersRealBootstrap(t *testing.T) {
	for _, native := range []bool{true, false} {
		t.Run(map[bool]string{true: "native", false: "NBC"}[native], func(t *testing.T) {
			s := newCloudInitializationScenario("test", config.VHDUbuntu2204Gen2Containerd, native, true)
			nbc, err := baseTemplateLinux("westus3", "1.35.0", "amd64")
			require.NoError(t, err)
			nbc.SubscriptionID = "00000000-0000-0000-0000-000000000000"
			nbc.ResourceGroupName = "cloud-integration-test"
			s.BootstrapConfigMutator(nil, nbc)
			cfg, err := nbcToAKSNodeConfigV1(nbc)
			require.NoError(t, err)
			s.AKSNodeConfigMutator(nil, cfg)
			cfg.ApiServerConfig.ApiServerName = "api.example.test"
			if native {
				require.NoError(t, nodeconfigutils.Validate(cfg))
			}
			data, err := nodeconfigutils.MarshalConfigurationV1(cfg)
			require.NoError(t, err)
			nbc.AKSNodeConfigJSON = string(data)
			baker, err := agent.NewAgentBaker()
			require.NoError(t, err)
			bootstrap, err := baker.GetNodeBootstrapping(context.Background(), nbc)
			require.NoError(t, err)
			original, err := base64.StdEncoding.DecodeString(bootstrap.CustomData)
			require.NoError(t, err)
			require.Contains(t, string(original), ">/opt/azure/containers/aks-node-controller-config.json")
			require.Equal(t, !native, strings.Contains(string(original), ">/opt/azure/containers/aks-node-controller-nbc-cmd.sh"))
			customData, err := customDataWithCloudInitializationFixture(bootstrap.CustomData, cloudInitializationArtifacts{
				ANC: "https://example.test/anc", Proxy: "https://example.test/proxy", Tests: "https://example.test/tests",
				SHA256: strings.Repeat("a", 64),
			})
			require.NoError(t, err)
			decoded, err := base64.StdEncoding.DecodeString(customData)
			require.NoError(t, err)
			require.LessOrEqual(t, len(decoded), 64*1024, "fixture must fit the Azure CustomData limit")
			command := exec.Command("bash", "-n")
			command.Stdin = strings.NewReader(string(decoded))
			output, err := command.CombinedOutput()
			require.NoError(t, err, string(output))
		})
	}
}

func TestCloudInitializationStrictGateDoesNotSkipMissingPrerequisites(t *testing.T) {
	original := config.Config
	config.Config = config.DefaultConfiguration()
	t.Cleanup(func() { config.Config = original })
	config.Config.DisableScriptLessCompilation = true
	s := newCloudInitializationScenario("test", config.VHDUbuntu2204Gen2Containerd, true, true)
	require.Contains(t, s.SkipIf(context.Background()), "PR compilation")
	t.Setenv("REQUIRE_NATIVE_CLOUD_INTEGRATION", "true")
	require.Empty(t, s.SkipIf(context.Background()))
	require.ErrorContains(t, s.BootstrapConfigMutatorWithError(context.Background(), nil, nil), "PR compilation")
	config.Config.DisableScriptLessCompilation = false
	t.Setenv("RCV1P_TAGS_AUTO_INJECTED", "true")
	require.ErrorContains(t, cloudInitializationPrerequisites(context.Background(), false), "without auto-injected")
}

func TestCloudInitializationFixtureRequiresExactMarkerAndBinaryIdentity(t *testing.T) {
	artifacts := cloudInitializationArtifacts{
		ANC: "https://example.test/anc", Proxy: "https://example.test/proxy", Tests: "https://example.test/tests",
		SHA256: strings.Repeat("a", 64),
	}
	encode := func(value string) string { return base64.StdEncoding.EncodeToString([]byte(value)) }
	for _, data := range []string{"", "#hotfix-marker\n#hotfix-marker\n"} {
		_, err := customDataWithCloudInitializationFixture(encode(data), artifacts)
		require.ErrorContains(t, err, "exactly one hotfix marker")
	}
	bad := artifacts
	bad.SHA256 = "not-a-digest"
	_, err := customDataWithCloudInitializationFixture(encode(hotfixMarker), bad)
	require.ErrorContains(t, err, "SHA256")
	bad = artifacts
	bad.Proxy = "https://example.test/$(must-not-execute)"
	_, err = customDataWithCloudInitializationFixture(encode(hotfixMarker), bad)
	require.ErrorContains(t, err, "artifact URL")
	_, err = customDataWithCloudInitializationFixture(encode(hotfixMarker), artifacts)
	require.ErrorContains(t, err, "one rendered nodecustomdata payload")
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err = writer.Write([]byte("write_files:\n- path: /opt/azure/containers/init-aks-cloud.sh\n  content: real legacy initializer\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	nodeData := cloudInitNodeDataHeader + base64.StdEncoding.EncodeToString(compressed.Bytes()) + "\nEOF\n"
	encoded, err := customDataWithCloudInitializationFixture(
		encode("#!/bin/bash\n"+nodeData+hotfixMarker+"\necho real-provisioning-continues\n"), artifacts)
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	script := string(decoded)
	require.NotContains(t, script, hotfixMarker)
	require.NotContains(t, script, "@ANC_")
	require.Contains(t, script, "sha256sum --check -")
	require.Contains(t, script, "systemctl start anc-cloud-repository-fixture.service")
	require.Contains(t, script, "cloud initialization integration setup failed")
	require.Contains(t, script, "echo real-provisioning-continues")
	command := exec.Command("bash", "-n")
	command.Stdin = strings.NewReader(script)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestCloudInitializationObserverPreservesActualScriptAndOtherUserdata(t *testing.T) {
	originalScript := "#!/bin/bash\nprintf 'actual legacy initialization\\n'\n"
	raw := "write_files:\n- path: " + cloudInitLegacyPath +
		"\n  permissions: '0744'\n  encoding: base64\n  content: " +
		base64.StdEncoding.EncodeToString([]byte(originalScript)) +
		"\n- path: /etc/unrelated-config\n  content: retained\nruncmd:\n- echo retained\n"
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write([]byte(raw))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	boothook := cloudInitNodeDataHeader + base64.StdEncoding.EncodeToString(compressed.Bytes()) + "\nEOF\n" + hotfixMarker
	result, err := instrumentCloudInitializationNodeData([]byte(boothook))
	require.NoError(t, err)
	start := len(cloudInitNodeDataHeader)
	end := strings.Index(string(result[start:]), "\nEOF") + start
	data, err := base64.StdEncoding.DecodeString(string(result[start:end]))
	require.NoError(t, err)
	reader, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	data, err = io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	var document yaml.Node
	require.NoError(t, yaml.Unmarshal(data, &document))
	root := document.Content[0]
	files := cloudFixtureMappingValue(root, "write_files")
	require.Len(t, files.Content, 3)
	require.Equal(t, cloudInitLegacyOriginalPath, cloudFixtureMappingValue(files.Content[0], "path").Value)
	actualScript, err := base64.StdEncoding.DecodeString(cloudFixtureMappingValue(files.Content[0], "content").Value)
	require.NoError(t, err)
	require.Equal(t, originalScript, string(actualScript))
	require.Equal(t, "/etc/unrelated-config", cloudFixtureMappingValue(files.Content[1], "path").Value)
	require.Equal(t, "echo retained", cloudFixtureMappingValue(root, "runcmd").Content[0].Value)
	require.Equal(t, cloudInitLegacyPath, cloudFixtureMappingValue(files.Content[2], "path").Value)
	observer, err := base64.StdEncoding.DecodeString(cloudFixtureMappingValue(files.Content[2], "content").Value)
	require.NoError(t, err)
	require.Equal(t, cloudInitLegacyWrapper, string(observer))
	_, err = instrumentCloudInitializationNodeData(result)
	require.ErrorContains(t, err, "already instrumented")
}

func TestCloudInitializationPipelineRequiresStrictManualGate(t *testing.T) {
	root, err := findRepoRoot()
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(root, ".pipelines/e2e-anc-cloud-init.yaml"))
	require.NoError(t, err)
	var pipeline struct {
		Trigger   string         `yaml:"trigger"`
		PR        string         `yaml:"pr"`
		Variables map[string]any `yaml:"variables"`
		Jobs      []struct {
			Template   string         `yaml:"template"`
			Parameters map[string]any `yaml:"parameters"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(data, &pipeline))
	require.Equal(t, "none", pipeline.Trigger)
	require.Equal(t, "none", pipeline.PR)
	require.Subset(t, pipeline.Variables, map[string]any{
		"TAGS_TO_RUN":                        "nativecloudinit=true",
		"REQUIRE_NATIVE_CLOUD_INTEGRATION":   "true",
		"SKIP_E2E_TESTS":                     false,
		"SKIP_TESTS_WITH_SKU_CAPACITY_ISSUE": false,
		"DISABLE_SCRIPTLESS":                 false,
		"DISABLE_SCRIPT_LESS_COMPILATION":    false,
	})
	require.Len(t, pipeline.Jobs, 1)
	require.Equal(t, "./templates/e2e-template.yaml", pipeline.Jobs[0].Template)
	require.Subset(t, pipeline.Jobs[0].Parameters, map[string]any{
		"IgnoreScenariosWithMissingVhd": false,
		"rcv1pTagsAutoInjected":         false,
		"failedTestsRetryCount":         0,
		"subscriptionId":                "${{ parameters.subscriptionId }}",
		"variableGroup":                 "${{ parameters.variableGroup }}",
	})
}
