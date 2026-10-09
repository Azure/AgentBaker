package agent

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// CustomData size guard for the scriptless NBC CSE path.
//
// Background:
//
//	In Mode B (slim CustomData), the generated payload carries P (platform content) + H (hotfix-
//	injected scripts). R (customer certs) is moved to protectedSettings and is bounded by RP-side
//	per-field cert byte-size validation, so only H can overflow CustomData.
//
//	This guard asserts that the slim Mode B CustomData stays under MaxCustomDataLength (87,380).
//	In the hotfix CI it runs AFTER hotfix_generate.py injects the real H into nodecustomdata.yml,
//	so the //go:embed'd template already carries H — no synthetic injection needed.

// newScriptlessGuardConfig builds a minimal scriptless NBC config for the given distro.
func newScriptlessGuardConfig(distro datamodel.Distro) *datamodel.NodeBootstrappingConfiguration {
	agentPoolProfile := &datamodel.AgentPoolProfile{
		Name:   "nodepool1",
		OSType: datamodel.Linux,
		Distro: distro,
	}
	return &datamodel.NodeBootstrappingConfiguration{
		ContainerService: &datamodel.ContainerService{
			Location: "eastus",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorVersion: "1.29.0",
					OrchestratorType:    datamodel.Kubernetes,
					KubernetesConfig: &datamodel.KubernetesConfig{
						ContainerRuntimeConfig: map[string]string{},
					},
				},
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					FQDN: "test-cluster.hcp.eastus.azmk8s.io",
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{agentPoolProfile},
			},
		},
		AgentPoolProfile:          agentPoolProfile,
		CloudSpecConfig:           datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:             &datamodel.K8sComponents{},
		KubeletConfig:             map[string]string{},
		EnableScriptlessNBCCSECmd: true,
		// The hotfix flow sets ENABLE_PROVISIONING_HOTFIX=true, which adds enabled_features.sh
		// to the Mode B payload via renderEnabledFeatures. Include it so the guard measures
		// the same payload size as a real hotfix-enabled node.
		EnabledFeatures: map[string]string{"ENABLE_PROVISIONING_HOTFIX": "true"},
	}
}

// TestCustomDataSizeWithHotfix (Guard 1): the slim Mode B CustomData (base platform content P
// plus any hotfix-injected scripts H already embedded in nodecustomdata.yml) must stay under
// MaxCustomDataLength. Forcing ScriptlessCSEProvisionMode makes getScriptlessBoothook return the
// slim CustomData directly, so this measures exactly the Mode B CustomData branch.
//
// The template has distro-conditional blocks (Ubuntu vs Mariner/AzureLinux vs Flatcar vs OSGuard)
// and a cloud-conditional block (IsAKSCustomCloud inlines init-aks-cloud.sh), so we table-drive
// representative distros × cloud combinations to ensure no rendering branch exceeds the limit.
func TestCustomDataSizeWithHotfix(t *testing.T) {
	cases := []struct {
		name        string
		distro      datamodel.Distro
		customCloud bool
	}{
		{"Ubuntu2204", datamodel.AKSUbuntuContainerd2204Gen2, false},
		{"Ubuntu2204_CustomCloud", datamodel.AKSUbuntuContainerd2204Gen2, true},
		{"AzureLinuxV2", datamodel.AKSAzureLinuxV2Gen2, false},
		{"AzureLinuxV3", datamodel.AKSAzureLinuxV3Gen2, false},
		{"AzureLinuxV3OSGuard", datamodel.AKSAzureLinuxV3OSGuardGen2FIPSTL, false},
		{"Flatcar", datamodel.AKSFlatcarGen2, false},
		{"ACL", datamodel.AKSACLGen2TL, false},
	}

	templateGenerator := InitializeTemplateGenerator()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := newScriptlessGuardConfig(tc.distro)
			if tc.customCloud {
				config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
					Name: "akscustom",
				}
			}
			// Force Mode B so getScriptlessBoothook early-returns the slim CustomData (P + H).
			config.ScriptlessCSEProvisionMode = true

			payload := templateGenerator.getScriptlessBoothook(config)

			if len(payload) >= MaxCustomDataLength {
				t.Fatalf("slim Mode B CustomData (%s) is %d bytes, must be < MaxCustomDataLength (%d). "+
					"A hotfix injecting too many/too-large scripts (H) overflowed the CustomData limit; "+
					"there is no further fallback once in Mode B, so the node would fail to provision.",
					tc.name, len(payload), MaxCustomDataLength)
			}
			t.Logf("slim Mode B CustomData (%s): %d / %d bytes (%d headroom)",
				tc.name, len(payload), MaxCustomDataLength, MaxCustomDataLength-len(payload))
		})
	}
}
