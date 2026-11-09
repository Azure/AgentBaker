package scenario

import (
	"encoding/base64"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/parser"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/gpu"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/stretchr/testify/require"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

func TestNBCANCConfigurationParityRegressions(t *testing.T) {
	original := *config.Config
	config.Config.DisableScriptless = false
	t.Cleanup(func() { *config.Config = original })

	components, err := parts.Templates.ReadFile("common/components.json")
	require.NoError(t, err)
	gpuConfig, err := gpu.LoadConfig(components)
	require.NoError(t, err)

	for _, tc := range []struct {
		name       string
		containerd string
	}{
		{"Ubuntu2204_NodeHardening_KubeReservedSlice_CLIFlags", "1.7.22"},
		{"Ubuntu2604Minimal_NodeHardening_KubeReservedSlice_CLIFlags", "2.2.4"},
		{"Ubuntu2404_TemporaryKubeletDisk", "2.2.4"},
		{"AzureLinuxV3_TemporaryKubeletDisk", "2.2.4"},
		{"Ubuntu2204_CustomNodeConfig", "1.7.22"},
		{"Ubuntu2604Minimal_CustomNodeConfig", "2.2.4"},
		{"AzureLinuxV3Gen2Kata", "1.7.22"},
		{"Ubuntu2204_NodeHardening_KubeReservedSlice_ConfigFile", "1.7.22"},
		{"Ubuntu2604Minimal_NodeHardening_KubeReservedSlice_ConfigFile", "2.2.4"},
		{"Ubuntu2204_CustomCA", "1.7.22"},
		{"Ubuntu2204_HTTPSProxy_PrivateDNS", "1.7.22"},
		{"Ubuntu2204_GPUA10", "1.7.22"},
		{"Ubuntu2204_GPUNC", "1.7.22"},
		{"Ubuntu2204ARM64", "1.7.22"},
		{"Ubuntu2204_ArtifactStreaming", "1.7.22"},
		{"Ubuntu2204_DisableSSH", "1.7.22"},
		{"Ubuntu2204Gen2_ImagePullIdentityBinding_Enabled", "1.7.22"},
		{"Ubuntu2404_SecondaryNIC_DualStack", "2.2.4"},
		{"LocalDNS/Ubuntu2404", "2.2.4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var registered *Scenario
			for _, s := range List() {
				if s.Name == tc.name {
					registered = s
					break
				}
			}

			require.NotNil(t, registered)
			s := freshScenario(registered)
			s.ForceScriptlessNBCCSECmd = true
			s.Runtime = &ScenarioRuntime{
				EnableScriptlessNBCCSECmd: true,
				Cluster: &Cluster{
					ProxyURL:      "http://proxy.example.test:3128",
					ClusterParams: &ClusterParams{FQDN: "api.example.test"},
				},
			}

			nbc, err := baseTemplateLinux("westus2", "1.36.5", s.VHD.Arch)
			require.NoError(t, err)
			nbc.SubscriptionID = "00000000-0000-0000-0000-000000000001"
			nbc.ResourceGroupName = "parity-test-rg"
			nbc.ContainerService.Properties.HostedMasterProfile.FQDN = "api.example.test"
			nbc.AgentPoolProfile.VMSize = "Standard_D4s_v3"
			nbc.AgentPoolProfile.Distro = s.VHD.Distro
			nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = s.VHD.Distro
			require.NoError(t, prepareNodeConfiguration(t.Context(), s, nbc))
			require.NotNil(t, s.Runtime.AKSNodeConfig)
			require.NoError(t, nodeconfigutils.Validate(s.Runtime.AKSNodeConfig))

			dir := t.TempDir()
			stub := fmt.Sprintf("#!/bin/sh\n[ \"$1\" = \"--version\" ] || exit 1\nprintf 'containerd containerd.io %s test\\n'\n", tc.containerd)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "containerd"), []byte(stub), 0755))
			t.Setenv("PATH", dir)
			env := parser.GetCSEEnv(t.Context(), s.Runtime.AKSNodeConfig, gpuConfig)
			legacy := *nbc
			legacy.EnableScriptlessCSECmd = false
			legacy.EnableScriptlessNBCCSECmd = false
			legacy.EnableScriptlessAKSNodeConfig = false
			baker, err := agent.NewAgentBaker()
			require.NoError(t, err)
			bootstrap, err := baker.GetNodeBootstrapping(t.Context(), &legacy)
			require.NoError(t, err)
			nbcEnv, err := parityNBCEnv(bootstrap.CSE)
			require.NoError(t, err)
			inventory, problems := compareParityEnvs(nbcEnv, env, legacyParityExceptions())
			t.Logf("Compared %d NBC / %d ANC variables; difference inventory:\n%s", len(nbcEnv), len(env), strings.Join(inventory, "\n"))
			require.Empty(t, problems, "unclassified or changed differences:\n%s", strings.Join(problems, "\n"))
		})
	}
}

type parityEnvValue struct {
	present bool
	value   string
}

type parityException struct {
	nbc, anc parityEnvValue
	reason   string
}

// These exceptions apply to the legacy (non-scriptless) NBC fixtures above, not arbitrary provisioning modes.
func legacyParityExceptions() map[string]parityException {
	return map[string]parityException{
		"CLOUD_INIT_STATUS_SCRIPT": {
			nbc:    parityEnvValue{true, "/opt/azure/containers/cloud-init-status-check.sh"},
			reason: "legacy NBC waits for custom-data cloud-init; ANC does not use this command preamble",
		},
		"HYPERKUBE_URL": {
			nbc:    parityEnvValue{true, "mcr.microsoft.com/oss/kubernetes/"},
			reason: "legacy hyperkube variable is unused by these modern Kubernetes provisioning scripts",
		},
		"BLOCK_OUTBOUND_NETWORK": {
			nbc:    parityEnvValue{true, "false"},
			reason: "unused legacy variable; these fixtures have unrestricted outbound access",
		},
		"SKIP_WAAGENT_HOLD": {
			nbc:    parityEnvValue{true, "false"},
			anc:    parityEnvValue{true, "true"},
			reason: "legacy CSE holds waagent, whereas ANC always skips the legacy waagent hold",
		},
	}
}

func compareParityEnvs(nbc, anc map[string]string, exceptions map[string]parityException) (inventory, problems []string) {
	keys := make(map[string]string, len(nbc)+len(anc))
	maps.Copy(keys, nbc)
	maps.Copy(keys, anc)
	for _, key := range slices.Sorted(maps.Keys(keys)) {
		nbcValue, inNBC := nbc[key]
		ancValue, inANC := anc[key]
		if exception, ok := exceptions[key]; ok {
			actualNBC := parityEnvValue{inNBC, nbcValue}
			actualANC := parityEnvValue{inANC, ancValue}
			if actualNBC != exception.nbc || actualANC != exception.anc || exception.reason == "" {
				problems = append(problems, key+": does not match the documented exception")
			} else if actualNBC == actualANC {
				problems = append(problems, key+": stale exception (values now match)")
			} else {
				inventory = append(inventory, key+": intentional difference: "+exception.reason)
			}
			continue
		}
		if !inNBC || !inANC {
			problems = append(problems, fmt.Sprintf("%s: actual omission (NBC present=%t, ANC present=%t)", key, inNBC, inANC))
			continue
		}
		if key == "SYSCTL_CONTENT" {
			nbcPairs, nbcErr := paritySysctlPairs(nbcValue)
			ancPairs, ancErr := paritySysctlPairs(ancValue)
			switch {
			case nbcErr != nil || ancErr != nil:
				problems = append(problems, fmt.Sprintf("%s: invalid content (NBC: %v; ANC: %v)", key, nbcErr, ancErr))
			case !maps.Equal(nbcPairs, ancPairs):
				problems = append(problems, key+": actual sysctl setting mismatch")
			case nbcValue != ancValue:
				inventory = append(inventory, key+": representation-only difference: comments, ordering and whitespace around '='")
			}
		} else if nbcValue != ancValue {
			problems = append(problems, key+": actual value mismatch")
		}
	}
	for _, key := range slices.Sorted(maps.Keys(exceptions)) {
		if _, ok := keys[key]; !ok {
			problems = append(problems, key+": stale exception (variable absent on both sides)")
		}
	}
	return inventory, problems
}

func paritySysctlPairs(encoded string) (map[string]string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	pairs := map[string]string{}
	for _, line := range strings.Split(string(decoded), "\n") {
		line, _, _ = strings.Cut(line, "#")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" || value == "" {
			return nil, fmt.Errorf("invalid sysctl assignment")
		}
		// sysctl applies repeated assignments in order, so the final value wins.
		pairs[key] = value
	}
	return pairs, nil
}

// Parse assignments without running the provisioning command or any shell substitutions.
func parityNBCEnv(command string) (map[string]string, error) {
	file, err := syntax.NewParser().Parse(strings.NewReader(command), "nbc-cse")
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	var parseErr error
	provisioningCalls := 0
	syntax.Walk(file, func(node syntax.Node) bool {
		if parseErr != nil {
			return false
		}
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}
		isProvisioning := len(call.Args) > 0 && call.Args[0].Lit() == "/usr/bin/nohup"
		if isProvisioning {
			provisioningCalls++
		}
		for _, assignment := range call.Assigns {
			if assignment.Name == nil || assignment.Append || assignment.Index != nil || assignment.Array != nil {
				parseErr = fmt.Errorf("unsupported NBC environment assignment")
				return false
			}
			key := assignment.Name.Value
			if key != strings.ToUpper(key) {
				continue
			}
			if !isProvisioning {
				// Only the command preamble and custom-cloud init have assignments outside nohup.
				switch key {
				case "PROVISION_OUTPUT", "CLOUD_INIT_STATUS_SCRIPT", "INIT_AKS_CLOUD_FILEPATH", "REPO_DEPOT_ENDPOINT", "LOCATION":
				default:
					parseErr = fmt.Errorf("%s is assigned outside the provisioning invocation", key)
					return false
				}
			}
			if assignment.Value != nil {
				if len(assignment.Value.Parts) > 0 {
					if literal, ok := assignment.Value.Parts[0].(*syntax.Lit); ok && strings.HasPrefix(literal.Value, "~") {
						parseErr = fmt.Errorf("tilde expansion in NBC assignment for %s", key)
						return false
					}
				}
				syntax.Walk(assignment.Value, func(part syntax.Node) bool {
					switch part.(type) {
					case nil, *syntax.Word, *syntax.Lit, *syntax.SglQuoted, *syntax.DblQuoted:
						return true
					default:
						parseErr = fmt.Errorf("non-literal NBC assignment for %s", key)
						return false
					}
				})
			}
			if parseErr != nil {
				return false
			}
			value := ""
			var err error
			if assignment.Value != nil {
				value, err = expand.Literal(&expand.Config{NoUnset: true}, assignment.Value)
			}
			if err != nil {
				parseErr = fmt.Errorf("extract %s: %w", key, err)
				return false
			}
			if previous, ok := env[key]; ok && previous != value {
				parseErr = fmt.Errorf("conflicting assignments for %s", key)
				return false
			}
			env[key] = value
		}
		return true
	})
	if parseErr == nil && provisioningCalls != 1 {
		return nil, fmt.Errorf("expected one provisioning invocation, got %d", provisioningCalls)
	}
	return env, parseErr
}

func TestNBCANCConfigurationParityRegressionsComparator(t *testing.T) {
	encode := func(value string) string { return base64.StdEncoding.EncodeToString([]byte(value)) }
	for _, tc := range []struct {
		name       string
		nbc, anc   map[string]string
		exceptions map[string]parityException
		problem    string
	}{
		{name: "matching empty value", nbc: map[string]string{"KEY": ""}, anc: map[string]string{"KEY": ""}},
		{name: "new NBC variable", nbc: map[string]string{"NEW_KEY": ""}, anc: map[string]string{}, problem: "NEW_KEY: actual omission"},
		{name: "new ANC variable", nbc: map[string]string{}, anc: map[string]string{"NEW_KEY": ""}, problem: "NEW_KEY: actual omission"},
		{name: "changed value", nbc: map[string]string{"KEY": "true"}, anc: map[string]string{"KEY": "false"}, problem: "KEY: actual value mismatch"},
		{name: "quotes are significant", nbc: map[string]string{"KEY": `"value"`}, anc: map[string]string{"KEY": "value"}, problem: "KEY: actual value mismatch"},
		{
			name: "exact intentional difference", nbc: map[string]string{"KEY": "old"}, anc: map[string]string{"KEY": "new"},
			exceptions: map[string]parityException{"KEY": {parityEnvValue{true, "old"}, parityEnvValue{true, "new"}, "known mode difference"}},
		},
		{
			name: "changed exception value", nbc: map[string]string{"KEY": "old"}, anc: map[string]string{"KEY": "wrong"},
			exceptions: map[string]parityException{"KEY": {parityEnvValue{true, "old"}, parityEnvValue{true, "new"}, "known mode difference"}},
			problem:    "KEY: does not match",
		},
		{
			name: "changed exception direction", nbc: map[string]string{"KEY": "old"}, anc: map[string]string{"KEY": ""},
			exceptions: map[string]parityException{"KEY": {parityEnvValue{true, "old"}, parityEnvValue{}, "legacy only"}},
			problem:    "KEY: does not match",
		},
		{
			name: "removed exception variable", nbc: map[string]string{}, anc: map[string]string{},
			exceptions: map[string]parityException{"KEY": {parityEnvValue{true, "old"}, parityEnvValue{}, "legacy only"}},
			problem:    "KEY: stale exception",
		},
		{
			name: "matching exception is stale", nbc: map[string]string{"KEY": "same"}, anc: map[string]string{"KEY": "same"},
			exceptions: map[string]parityException{"KEY": {parityEnvValue{true, "same"}, parityEnvValue{true, "same"}, "no longer different"}},
			problem:    "KEY: stale exception",
		},
		{
			name: "sysctl representation", nbc: map[string]string{"SYSCTL_CONTENT": encode("# comment\nb = 2\na=1\n")},
			anc: map[string]string{"SYSCTL_CONTENT": encode("a=1\nb=2\n")},
		},
		{
			name: "sysctl changed value", nbc: map[string]string{"SYSCTL_CONTENT": encode("a=1\n")},
			anc: map[string]string{"SYSCTL_CONTENT": encode("a=2\n")}, problem: "SYSCTL_CONTENT: actual sysctl setting mismatch",
		},
		{
			name: "sysctl final override", nbc: map[string]string{"SYSCTL_CONTENT": encode("a=0\na=1\n")},
			anc: map[string]string{"SYSCTL_CONTENT": encode("a=1\n")},
		},
		{
			name: "sysctl override order matters", nbc: map[string]string{"SYSCTL_CONTENT": encode("a=0\na=1\n")},
			anc: map[string]string{"SYSCTL_CONTENT": encode("a=1\na=0\n")}, problem: "SYSCTL_CONTENT: actual sysctl setting mismatch",
		},
		{
			name: "sysctl invalid base64", nbc: map[string]string{"SYSCTL_CONTENT": "not-base64"},
			anc: map[string]string{"SYSCTL_CONTENT": encode("a=1\n")}, problem: "SYSCTL_CONTENT: invalid content",
		},
		{
			name: "sysctl invalid assignment", nbc: map[string]string{"SYSCTL_CONTENT": encode("invalid\n")},
			anc: map[string]string{"SYSCTL_CONTENT": encode("a=1\n")}, problem: "SYSCTL_CONTENT: invalid content",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, problems := compareParityEnvs(tc.nbc, tc.anc, tc.exceptions)
			if tc.problem == "" {
				require.Empty(t, problems)
			} else {
				require.Contains(t, strings.Join(problems, "\n"), tc.problem)
			}
		})
	}
}

func TestNBCANCConfigurationParityRegressionsNBCExtraction(t *testing.T) {
	env, err := parityNBCEnv(`PROVISION_OUTPUT="/tmp/output"; EMPTY= QUOTED='a "b" c' CONCAT="a"'b' ESCAPED="a\"b" /usr/bin/nohup /bin/bash -c "exit 1"`)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"PROVISION_OUTPUT": "/tmp/output", "EMPTY": "", "QUOTED": `a "b" c`, "CONCAT": "ab", "ESCAPED": `a"b`}, env)
	for _, command := range []string{
		`KEY="$(exit 1)" /usr/bin/nohup`,
		`KEY="$HOME" /usr/bin/nohup`,
		`KEY="${UNSET:-fallback}" /usr/bin/nohup`,
		`KEY=~ /usr/bin/nohup`,
		`KEY="unterminated`,
		`KEY=one; KEY=two /usr/bin/nohup`,
		`KEY=one KEY=two /usr/bin/nohup`,
		`KEY+=one /usr/bin/nohup`,
		`KEY=(one two)`,
		`KEY=one; /usr/bin/nohup`,
		`KEY=one /bin/echo not-provisioning`,
	} {
		_, err := parityNBCEnv(command)
		require.Error(t, err, command)
	}
}
