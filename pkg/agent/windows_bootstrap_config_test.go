package agent

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/stretchr/testify/require"
)

// windowsBootstrapHostileValue holds characters that PowerShell treats as code or quotes inside
// string literals, plus multi-line and non-ASCII text. Running the canary sets a global variable,
// which the Pester test checks for.
const windowsBootstrapHostileValue = "a'b\"c`d$(Set-Variable -Name AKSInjectionCanary -Value 1 -Scope Global)e;f#g<#h#>\r\n" +
	"i\u2018j\u2019k\u201cl\u201dm\u201en\U0001F600o\\/Date(0)\\/p\"\"q${env:PATH}r@(1)s"

// windowsBootstrapConfigBlobPattern finds the config blob in a rendered structured variables block.
var windowsBootstrapConfigBlobPattern = regexp.MustCompile(`FromBase64String\('([A-Za-z0-9+/=]+)'\)`)

func windowsBootstrapConfigBlob(t *testing.T, block string) string {
	t.Helper()
	matches := windowsBootstrapConfigBlobPattern.FindAllStringSubmatch(block, -1)
	require.Len(t, matches, 1, "expected one config blob in the structured variables block")
	return matches[0][1]
}

func decodeWindowsBootstrapConfigBlob(t *testing.T, blob string) *windowsBootstrapConfig {
	t.Helper()
	gzipped, err := base64.StdEncoding.DecodeString(blob)
	require.NoError(t, err)
	data, err := getGzipDecodedValue(gzipped)
	require.NoError(t, err)
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var bootstrapConfig windowsBootstrapConfig
	require.NoError(t, decoder.Decode(&bootstrapConfig))
	return &bootstrapConfig
}

func indentWindowsBootstrapConfig(t *testing.T, bootstrapConfig *windowsBootstrapConfig) string {
	t.Helper()
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	require.NoError(t, encoder.Encode(bootstrapConfig))
	return buf.String()
}

// windowsBootstrapExpectedValues lists the PowerShell variables that the variables block must set,
// one "<name>\t<value>" line each, in the format used by
// parts/windows/kuberneteswindowssetup.bootstrapvariables.tests.ps1. Strings are base64 of UTF-8 so
// the file stays ASCII and never goes through a JSON parser.
func windowsBootstrapExpectedValues(t *testing.T, bootstrapConfig *windowsBootstrapConfig) string {
	t.Helper()
	encodeString := func(s string) string {
		return "string:" + base64.StdEncoding.EncodeToString([]byte(s))
	}
	var lines []string
	value := reflect.ValueOf(*bootstrapConfig)
	for i := 0; i < value.NumField(); i++ {
		name := strings.Split(value.Type().Field(i).Tag.Get("json"), ",")[0]
		if name == "SchemaVersion" {
			continue
		}
		var encoded string
		switch field := value.Field(i).Interface().(type) {
		case string:
			encoded = encodeString(field)
		case *string:
			if field == nil {
				continue // The block leaves the variable unset.
			}
			encoded = encodeString(*field)
		case bool:
			encoded = "bool:" + map[bool]string{true: "True", false: "False"}[field]
		case uint32:
			encoded = "uint32:" + strconv.FormatUint(uint64(field), 10)
		case []string:
			items := make([]string, 0, len(field))
			for _, item := range field {
				items = append(items, encodeString(item))
			}
			encoded = "array:" + strings.Join(items, ",")
		default:
			t.Fatalf("windowsBootstrapConfig.%s has unsupported type %T", value.Type().Field(i).Name, field)
		}
		lines = append(lines, name+"\t"+encoded)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

func renderStructuredWindowsBootstrapConfig(t *testing.T, config *datamodel.NodeBootstrappingConfiguration) (string, *windowsBootstrapConfig) {
	t.Helper()
	customData, _ := renderWindowsBootstrap(t, structuredWindowsBootstrapConfig(config))
	return customData, decodeWindowsBootstrapConfigBlob(t, windowsBootstrapConfigBlob(t, extractWindowsBootstrapVariables(t, customData)))
}

func TestWindowsBootstrapConfigRoundTripsHostileValues(t *testing.T) {
	customData, bootstrapConfig := renderStructuredWindowsBootstrapConfig(t, newHostileWindowsBootstrapTestConfig())

	// None of the hostile text reaches the script as PowerShell code.
	require.NotContains(t, customData, "AKSInjectionCanary")
	requireASCII(t, "CustomData", customData)

	hostile := windowsBootstrapHostileValue
	require.Equal(t, hostile, bootstrapConfig.MasterIP)
	require.Equal(t, hostile, bootstrapConfig.KubeDNSServiceIP)
	require.Equal(t, hostile, bootstrapConfig.MasterFQDNPrefix)
	require.Equal(t, hostile, bootstrapConfig.Location)
	require.Equal(t, to.StringPtr(hostile), bootstrapConfig.UserAssignedClientID)
	require.Equal(t, hostile, bootstrapConfig.AADClientID)
	require.Equal(t, []string{hostile}, bootstrapConfig.SSHKeys)
	require.Equal(t, hostile, bootstrapConfig.TenantID)
	require.Equal(t, hostile, bootstrapConfig.SubscriptionID)
	require.Equal(t, hostile, bootstrapConfig.ResourceGroup)
	require.Equal(t, hostile, bootstrapConfig.PrimaryScaleSetName)
	require.Equal(t, hostile, bootstrapConfig.KubeClusterCIDR)
	require.Equal(t, hostile, bootstrapConfig.KubeServiceCIDR)
	require.Equal(t, hostile, bootstrapConfig.VNetCIDR)
	require.Contains(t, bootstrapConfig.KubeletNodeLabels, ",hostile="+hostile)
	require.Equal(t, hostile, bootstrapConfig.LoadBalancerSku)
	require.Equal(t, hostile, bootstrapConfig.TLSBootstrapToken)
	require.Equal(t, hostile, bootstrapConfig.CSEScriptsPackageURL)
	require.Equal(t, hostile, bootstrapConfig.WindowsCiliumNetworkingConfiguration)
	require.Equal(t, hostile, bootstrapConfig.BootstrapProfileContainerRegistryServer)
	require.Equal(t, hostile, bootstrapConfig.PrivateEgressProxyAddress)
	require.Equal(t, hostile, bootstrapConfig.AKSCustomCloudContainerRegistryDNSSuffix)
	require.Equal(t, hostile, bootstrapConfig.ArmResourceEndpoint)
	require.Equal(t, hostile, bootstrapConfig.SecureTLSBootstrappingAADResource)

	// Kubelet and kube-proxy arguments keep their PowerShell double-quoted string meaning ("" is one quote).
	unquoted := strings.ReplaceAll(hostile, `""`, `"`)
	require.Contains(t, bootstrapConfig.KubeletConfigArgs, "--hostile-flag="+unquoted)
	require.Contains(t, bootstrapConfig.KubeproxyConfigArgs, "--hostile-flag="+unquoted)
}

func TestWindowsBootstrapConfigKeysMatchTemplate(t *testing.T) {
	b, err := parts.Templates.ReadFile(kubernetesWindowsAgentCustomDataPS1)
	require.NoError(t, err)
	text := string(b)
	begin := strings.Index(text, "{{if EnableWindowsStructuredBootstrapConfig -}}")
	end := strings.Index(text, "{{else -}}\n$MasterIP=")
	legacyEnd := strings.Index(text, "{{- end}}\n"+windowsBootstrapVariablesEnd)
	require.True(t, begin >= 0 && end > begin && legacyEnd > end, "structured and legacy branches not found")

	scriptKeys := map[string]bool{}
	for _, match := range regexp.MustCompile(`\$cfg\.([A-Za-z0-9_]+)`).FindAllStringSubmatch(text[begin:end], -1) {
		scriptKeys[match[1]] = true
	}
	configKeys := map[string]bool{}
	variableNames := map[string]bool{}
	configType := reflect.TypeOf(windowsBootstrapConfig{})
	for i := 0; i < configType.NumField(); i++ {
		key := strings.Split(configType.Field(i).Tag.Get("json"), ",")[0]
		configKeys[key] = true
		if key != "SchemaVersion" {
			variableNames[strings.ToLower(key)] = true
		}
	}
	require.Equal(t, configKeys, scriptKeys, "every windowsBootstrapConfig key must be read by the script, and the script must only read known keys")

	// Each key is named after the variable it sets, and both branches set the same variables.
	legacyVariables := map[string]bool{}
	for _, match := range regexp.MustCompile(`(?m)^\$(?:global:)?([A-Za-z_][A-Za-z0-9_]*)=`).FindAllStringSubmatch(text[end:legacyEnd], -1) {
		legacyVariables[strings.ToLower(match[1])] = true
	}
	require.Equal(t, variableNames, legacyVariables)
}

func TestWindowsBootstrapConfigAssignmentsMatchLegacyBranch(t *testing.T) {
	b, err := parts.Templates.ReadFile(kubernetesWindowsAgentCustomDataPS1)
	require.NoError(t, err)
	text := string(b)
	begin := strings.Index(text, "{{if EnableWindowsStructuredBootstrapConfig -}}")
	end := strings.Index(text, "{{else -}}\n$MasterIP=")
	legacyEnd := strings.Index(text, "{{- end}}\n"+windowsBootstrapVariablesEnd)
	require.True(t, begin >= 0 && end > begin && legacyEnd > end, "structured and legacy branches not found")

	assignment := regexp.MustCompile(`(?m)^\s*\$(global:)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
	legacyScope := map[string]string{}
	for _, match := range assignment.FindAllStringSubmatch(text[end:legacyEnd], -1) {
		legacyScope[strings.ToLower(match[2])] = match[1]
	}

	readsConfig := regexp.MustCompile(`\$cfg\.([A-Za-z0-9_]+)`)
	assigned := 0
	for _, match := range assignment.FindAllStringSubmatch(text[begin:end], -1) {
		reads := readsConfig.FindAllStringSubmatch(match[3], -1)
		if len(reads) == 0 {
			continue
		}
		assigned++
		require.Len(t, reads, 1, "assignment to $%s reads more than one config value", match[2])
		require.True(t, strings.EqualFold(match[2], reads[0][1]), "$%s is assigned from config key %s", match[2], reads[0][1])
		scope, ok := legacyScope[strings.ToLower(match[2])]
		require.True(t, ok, "$%s is not assigned by the legacy branch", match[2])
		require.Equal(t, scope, match[1], "$%s must use the same scope as in the legacy branch", match[2])
	}
	require.Len(t, legacyScope, assigned, "every variable of the legacy branch must be assigned from the config")
}

// TestWindowsBootstrapFixturesTellValuesApart checks that, for every two config values of the
// same type, some fixture gives them different values. Otherwise the fixtures could not catch a
// value that is assigned to the wrong variable.
func TestWindowsBootstrapFixturesTellValuesApart(t *testing.T) {
	var configs []reflect.Value
	for _, fixture := range windowsBootstrapTestFixtures() {
		if fixture.hostile {
			continue
		}
		_, bootstrapConfig := renderStructuredWindowsBootstrapConfig(t, fixture.newConfig())
		configs = append(configs, reflect.ValueOf(*bootstrapConfig))
	}
	// windowsKubeBinariesURL is never set, so it is empty for every configuration.
	alwaysEmpty := map[string]bool{"WindowsKubeBinariesURL": true}
	configType := configs[0].Type()
	for i := 0; i < configType.NumField(); i++ {
		for j := i + 1; j < configType.NumField(); j++ {
			a, b := configType.Field(i), configType.Field(j)
			if a.Type != b.Type || (alwaysEmpty[a.Name] && alwaysEmpty[b.Name]) {
				continue
			}
			if !someConfigDiffers(configs, i, j) {
				t.Errorf("no fixture tells %s and %s apart", a.Name, b.Name)
			}
		}
	}
}

func someConfigDiffers(configs []reflect.Value, i, j int) bool {
	for _, c := range configs {
		if !reflect.DeepEqual(c.Field(i).Interface(), c.Field(j).Interface()) {
			return true
		}
	}
	return false
}

func TestWindowsBootstrapConfigFitsCustomDataLimit(t *testing.T) {
	// Real certificates, keys and tokens are random text that does not compress well.
	random := rand.NewChaCha8([32]byte{})
	randomText := func(n int) string {
		b := make([]byte, n)
		_, _ = random.Read(b)
		return base64.StdEncoding.EncodeToString(b)
	}
	for _, fixture := range windowsBootstrapTestFixtures() {
		if fixture.hostile {
			continue
		}
		t.Run(fixture.name, func(t *testing.T) {
			config := fixture.newConfig()
			properties := config.ContainerService.Properties
			properties.CertificateProfile.CaCertificate = randomText(1400)
			properties.CertificateProfile.ClientCertificate = randomText(1400)
			properties.LinuxProfile = &datamodel.LinuxProfile{AdminUsername: "azureuser"}
			for i := 0; i < 3; i++ {
				properties.LinuxProfile.SSH.PublicKeys = append(properties.LinuxProfile.SSH.PublicKeys,
					datamodel.PublicKey{KeyData: "ssh-rsa " + randomText(540) + " user@example"})
			}
			for i := 0; i < 20; i++ {
				config.AgentPoolProfile.CustomNodeLabels[fmt.Sprintf("example.com/label-%02d", i)] = randomText(15)
			}

			legacy := InitializeTemplateGenerator().getWindowsNodeBootstrappingPayload(withoutStructuredWindowsBootstrapConfig(config))
			structured := InitializeTemplateGenerator().getWindowsNodeBootstrappingPayload(structuredWindowsBootstrapConfig(config))
			t.Logf("Windows CustomData length: legacy %d, structured %d, limit %d", len(legacy), len(structured), MaxCustomDataLength)
			require.Less(t, len(structured), MaxCustomDataLength)
			// The config is gzip-compressed, so it must not grow CustomData by more than about 2 KB.
			require.Less(t, len(structured)-len(legacy), 3072)
		})
	}
}

// withoutStructuredWindowsBootstrapConfig returns a shallow copy of config with the flag off, so two
// renders in a test share the values but not the flag.
func withoutStructuredWindowsBootstrapConfig(config *datamodel.NodeBootstrappingConfiguration) *datamodel.NodeBootstrappingConfiguration {
	c := *config
	c.EnableWindowsStructuredBootstrapConfig = false
	return &c
}

func TestWindowsPreProvisionStructuredConfigOmitsTLSBootstrapToken(t *testing.T) {
	bake := newWindowsBootstrapTestConfig()
	bake.PreProvisionOnly = true
	bakeCustomData, bakeConfig := renderStructuredWindowsBootstrapConfig(t, bake)
	require.Empty(t, bakeConfig.TLSBootstrapToken)
	require.True(t, bakeConfig.PreProvisionOnly)
	require.Contains(t, bakeCustomData, "function NodePrep")

	_, provisionConfig := renderStructuredWindowsBootstrapConfig(t, newWindowsBootstrapTestConfig())
	require.Equal(t, "07401b.f395accd246ae52d", provisionConfig.TLSBootstrapToken)
	require.False(t, provisionConfig.PreProvisionOnly)
}

func TestWindowsBootstrapConfigLeavesUserAssignedClientIDUnsetWithoutUserAssignedIdentity(t *testing.T) {
	config := newWindowsBootstrapTestConfig()
	config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UserAssignedID = ""
	_, bootstrapConfig := renderStructuredWindowsBootstrapConfig(t, config)
	require.Nil(t, bootstrapConfig.UserAssignedClientID)
	require.NotNil(t, bootstrapConfig.SSHKeys)
}

func TestPowershellDoubleQuotedValues(t *testing.T) {
	require.Equal(t,
		[]string{`--resolv-conf=""`, `--a="b"`, "--c=$(d)`e", `--f=""`, ""},
		powershellDoubleQuotedValues([]string{`--resolv-conf=""""`, `--a=""b""`, "--c=$(d)`e", `--f="""`, ""}))
	require.Empty(t, powershellDoubleQuotedValues(nil))
	require.NotNil(t, powershellDoubleQuotedValues(nil))
}

func TestWindowsTemplateValuesReportErrors(t *testing.T) {
	failing := func() (string, error) { return "", fmt.Errorf("boom") }
	funcMap := template.FuncMap{
		"Text":    func() string { return "text" },
		"Bool":    func() bool { return true },
		"NotBool": func() string { return "yes" },
		"Uint":    func() uint32 { return 7 },
		"Fails":   failing,
		"NoValue": func() {},
	}

	v := &windowsTemplateValues{funcMap: funcMap}
	require.Equal(t, "text", v.text("Text"))
	require.True(t, v.boolean("Bool"))
	require.Equal(t, uint32(7), v.uint32("Uint"))
	require.NoError(t, v.err)

	for name, call := range map[string]func(v *windowsTemplateValues){
		"missing":  func(v *windowsTemplateValues) { v.text("Missing") },
		"error":    func(v *windowsTemplateValues) { v.text("Fails") },
		"no value": func(v *windowsTemplateValues) { v.text("NoValue") },
		"not bool": func(v *windowsTemplateValues) { v.boolean("NotBool") },
		"not uint": func(v *windowsTemplateValues) { v.uint32("Text") },
	} {
		t.Run(name, func(t *testing.T) {
			v := &windowsTemplateValues{funcMap: funcMap}
			call(v)
			require.Error(t, v.err)
			// The first error is kept and later calls return zero values.
			require.Empty(t, v.text("Text"))
			require.False(t, v.boolean("Bool"))
			require.Zero(t, v.uint32("Uint"))
		})
	}

	_, err := getWindowsBootstrapConfig(newWindowsBootstrapTestConfig(), template.FuncMap{})
	require.ErrorContains(t, err, "not found")
}

// newHostileWindowsBootstrapTestConfig puts windowsBootstrapHostileValue into the values that
// customers and RP can influence.
func newHostileWindowsBootstrapTestConfig() *datamodel.NodeBootstrappingConfiguration {
	hostile := windowsBootstrapHostileValue
	config := newWindowsBootstrapTestConfig()
	properties := config.ContainerService.Properties
	kubernetesConfig := properties.OrchestratorProfile.KubernetesConfig

	config.ContainerService.Location = hostile
	config.TenantID = hostile
	config.SubscriptionID = hostile
	config.ResourceGroupName = hostile
	config.UserAssignedIdentityClientID = hostile
	config.PrimaryScaleSetName = hostile
	config.KubeletClientTLSBootstrapToken = to.StringPtr(hostile)
	config.KubeletConfig["--hostile-flag"] = hostile
	config.KubeproxyConfig = map[string]string{"--hostile-flag": hostile}
	config.AgentPoolProfile.CustomNodeLabels["hostile"] = hostile
	config.AgentPoolProfile.VnetCidrs = []string{hostile}
	config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
		NextGenNetworkingEnabled: to.BoolPtr(true),
		NextGenNetworkingConfig:  to.StringPtr(hostile),
	}
	config.K8sComponents.WindowsPackageURL = hostile
	config.K8sComponents.WindowsCredentialProviderURL = hostile
	config.SecureTLSBootstrappingConfig = &datamodel.SecureTLSBootstrappingConfig{
		Enabled:                   true,
		AADResource:               hostile,
		UserAssignedIdentityID:    hostile,
		CustomClientDownloadURL:   hostile,
		ValidateKubeconfigTimeout: hostile,
		GetAccessTokenTimeout:     hostile,
		GetInstanceDataTimeout:    hostile,
		GetNonceTimeout:           hostile,
		GetAttestedDataTimeout:    hostile,
		GetCredentialTimeout:      hostile,
	}

	properties.HostedMasterProfile.FQDN = hostile
	properties.HostedMasterProfile.DNSPrefix = hostile
	properties.ServicePrincipalProfile.ClientID = hostile
	properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{KeyData: hostile}}
	properties.SecurityProfile = &datamodel.SecurityProfile{PrivateEgress: &datamodel.PrivateEgress{
		Enabled:                 true,
		ContainerRegistryServer: hostile,
		ProxyAddress:            hostile,
	}}
	properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
		Name:                       "akscustom",
		ContainerRegistryDNSSuffix: hostile,
		ResourceManagerEndpoint:    hostile,
	}
	properties.WindowsProfile.CSIProxyURL = hostile
	properties.WindowsProfile.WindowsPauseImageURL = hostile
	properties.WindowsProfile.CseScriptsPackageURL = hostile
	properties.WindowsProfile.WindowsGmsaPackageUrl = hostile
	properties.WindowsProfile.GpuDriverURL = hostile
	properties.WindowsProfile.WindowsCalicoPackageURL = hostile

	kubernetesConfig.DNSServiceIP = hostile
	kubernetesConfig.ClusterSubnet = hostile
	kubernetesConfig.ServiceCIDR = hostile
	kubernetesConfig.LoadBalancerSku = hostile
	kubernetesConfig.AzureCNIURLWindows = hostile
	kubernetesConfig.WindowsContainerdURL = hostile
	kubernetesConfig.UserAssignedID = hostile
	return config
}
