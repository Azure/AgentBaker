package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/testutil/cloudfixture"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const cloudInitMCRURL = "mcr.microsoft.com"
const cloudInitPublicVMDNSSuffix = "cloudapp.azure.com"

var _ = Register(newCloudInitializationScenario("Ubuntu2204_NativeCloudInit_OptedIn", config.VHDUbuntu2204Gen2Containerd, true, true))
var _ = Register(newCloudInitializationScenario("Ubuntu2204_NativeCloudInit_NotOptedIn", config.VHDUbuntu2204Gen2Containerd, true, false))
var _ = Register(newCloudInitializationScenario("Ubuntu2204_CloudInit_NBCPrecedence", config.VHDUbuntu2204Gen2Containerd, false, true))
var _ = Register(newCloudInitializationScenario("AzureLinuxV3_NativeCloudInit_OptedIn", config.VHDAzureLinuxV3Gen2, true, true))
var _ = Register(newCloudInitializationScenario("AzureLinuxV3_NativeCloudInit_NotOptedIn", config.VHDAzureLinuxV3Gen2, true, false))
var _ = Register(newCloudInitializationScenario("AzureLinuxV3_CloudInit_NBCPrecedence", config.VHDAzureLinuxV3Gen2, false, true))

func newCloudInitializationScenario(name string, image *config.Image, native, optIn bool) *Scenario {
	properties := &datamodel.Properties{CustomCloudEnv: cloudInitializationPublicEnvironment()}
	customJSON, err := properties.GetCustomEnvironmentJSON(false)
	if err != nil {
		panic(fmt.Sprintf("serialize static cloud initialization fixture: %v", err))
	}
	s := &Scenario{
		Name:        name,
		Description: "PR-built ANC, actual WireServer certificates, vendor-signed RepoDepot metadata and real provisioning through Node Ready",
		Tags:        Tags{RCV1PCertMode: true, NativeCloudInit: true, Scriptless: true},
		Config: Config{
			Cluster: ClusterKubenet, VHD: image, CloudInitializationFixture: true,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.EnableScriptlessAKSNodeConfig = native
				nbc.EnableScriptlessNBCCSECmd = !native
				nbc.EnableScriptlessCSECmd = false
				nbc.ContainerService.Properties.CustomCloudEnv = cloudInitializationPublicEnvironment()
			},
			BootstrapConfigMutatorWithError: func(ctx context.Context, _ *Cluster, _ *datamodel.NodeBootstrappingConfiguration) error {
				return cloudInitializationPrerequisites(ctx, optIn)
			},
			AKSNodeConfigMutator: func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) {
				cfg.CustomCloudConfig = &aksnodeconfigv1.CustomCloudConfig{
					CustomCloudEnvName: helpers.AksCustomCloudName, RepoDepotEndpoint: cloudInitRepositoryEndpoint,
					CustomEnvJsonContent: customJSON, ContainerRegistryDnsSuffix: "azurecr.io",
				}
			},
		},
	}
	s.SkipIf = func(ctx context.Context) string {
		if strings.EqualFold(os.Getenv("REQUIRE_NATIVE_CLOUD_INTEGRATION"), "true") {
			// The strict gate must fail, rather than silently skip, on a missing prerequisite.
			// BootstrapConfigMutatorWithError reports the actual error before VM creation.
			return ""
		}
		if err := cloudInitializationPrerequisites(ctx, optIn); err != nil {
			return err.Error()
		}
		return ""
	}
	if optIn {
		s.VMConfigMutator = rcv1pOptInVMConfigMutator
	}
	s.Validator = func(ctx context.Context, s *Scenario) error {
		return validateCloudInitialization(ctx, s, native, optIn)
	}
	return s
}

func cloudInitializationPrerequisites(ctx context.Context, optIn bool) error {
	if config.Config.DisableScriptless || config.Config.DisableScriptLessCompilation || config.Config.TestPreProvision {
		return errors.New("cloud initialization integration requires scriptless provisioning, PR compilation and a non-cached VM")
	}
	if !optIn && strings.EqualFold(os.Getenv("RCV1P_TAGS_AUTO_INJECTED"), "true") {
		return errors.New("cloud initialization opt-out requires a subscription without auto-injected RCV1P tags")
	}
	registered, err := getE2ESubscriptionFeatureFlag(ctx)
	if err != nil {
		return fmt.Errorf("cloud initialization PlatformSettingsOverride prerequisite: %w", err)
	}
	if !registered {
		return errors.New("cloud initialization requires Microsoft.Compute/PlatformSettingsOverride registration")
	}
	return nil
}

// Use the supported custom-cloud input contract, but retain public Azure
// identity, registry and ARM endpoints for the real E2E cluster. Merely setting
// repo_depot_endpoint without the custom-cloud name makes the parser skip it.
func cloudInitializationPublicEnvironment() *datamodel.CustomCloudEnv {
	profile := &datamodel.CustomCloudEnv{
		Name: helpers.AksCustomCloudName, McrURL: cloudInitMCRURL, RepoDepotEndpoint: cloudInitRepositoryEndpoint,
		ResourceManagerEndpoint: "https://management.azure.com/", ServiceManagementEndpoint: "https://management.core.windows.net/",
		ActiveDirectoryEndpoint: "https://login.microsoftonline.com/",
		GalleryEndpoint:         "https://gallery.azure.com/", ManagementPortalURL: "https://portal.azure.com/",
		KeyVaultEndpoint: "https://vault.azure.net/", GraphEndpoint: "https://graph.windows.net/",
		BatchManagementEndpoint: "https://batch.core.windows.net/", StorageEndpointSuffix: "core.windows.net",
		ContainerRegistryDNSSuffix: "azurecr.io", KeyVaultDNSSuffix: "vault.azure.net",
		SQLDatabaseDNSSuffix: "database.windows.net", TrafficManagerDNSSuffix: "trafficmanager.net",
		ServiceBusEndpoint: "https://servicebus.windows.net/", ServiceBusEndpointSuffix: "servicebus.windows.net",
		ResourceManagerVMDNSSuffix: cloudInitPublicVMDNSSuffix, ServiceManagementVMDNSSuffix: "cloudapp.net",
		ResourceIdentifiers: datamodel.ResourceIdentifiers{
			Graph: "https://graph.windows.net/", KeyVault: "https://vault.azure.net/",
			Storage: "https://storage.azure.com/", OperationalInsights: "https://api.loganalytics.io/",
		},
	}
	profile.TokenAudience = profile.ServiceManagementEndpoint
	return profile
}

func validateCloudInitialization(ctx context.Context, s *Scenario, native, optIn bool) error {
	start := time.Now()
	err := validateCloudInitializationState(ctx, s, native, optIn)
	s.recordADOTestCase("CloudInitialization", s.Name, time.Since(start), err)
	if err != nil || !native || !optIn {
		return err
	}
	start = time.Now()
	_, err = execScriptOnVMForScenarioValidateExitCode(ctx, s, fmt.Sprintf(
		"sudo %s/system-integration.test -test.v -test.run '^TestCloudSystemIntegration$' -cloud-system-integration -cloud-integration-location=%s",
		cloudInitStateDir, strconv.Quote(s.Location)), 0, "real-system cloud initialization suite failed")
	s.recordADOTestCase("CloudSystemIntegration", s.Name, time.Since(start), err)
	if err != nil {
		return err
	}
	return validateCloudInitializationRefresh(ctx, s)
}

func validateCloudInitializationState(ctx context.Context, s *Scenario, native, optIn bool) error {
	identity := fmt.Sprintf(`set -euo pipefail
test "$(sudo sha256sum %[1]s | cut -d ' ' -f1)" = "$(sudo cat %[2]s/anc.sha256)"
sudo grep -q legacy-init-called /opt/azure/containers/init-aks-cloud.sh
`, cloudInitANCPath, cloudInitStateDir)
	if native {
		if err := ValidateScriptlessPhase3(ctx, s); err != nil {
			return err
		}
		identity += "sudo test ! -e " + cloudInitStateDir + "/legacy-init-called\n"
	} else {
		identity += "sudo test -s " + cloudInitStateDir + "/legacy-init-called\n"
		identity += "sudo test -s /opt/azure/containers/aks-node-controller-nbc-cmd.sh\n"
		identity += "sudo test -s /opt/azure/containers/aks-node-controller-config.json\n"
	}
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, identity, 0,
		"wrong ANC binary, lost shell instrumentation, or wrong native/NBC initialization path"); err != nil {
		return err
	}
	logPath := ancLauncherOutput
	if native {
		message := "installed Azure cloud CA certificates"
		if !optIn {
			message = "Azure cloud root certificates are not opted in, skipping certificate installation"
		}
		if err := ValidateFileHasContent(ctx, s, logPath, message); err != nil {
			return err
		}
		if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
			"if sudo grep -q 'RepoDepot package cache refresh failed' "+logPath+"; then exit 1; fi", 0,
			"happy-path cache refresh must not silently use the best-effort fallback"); err != nil {
			return err
		}
	} else if err := ValidateRCV1PCertMode(ctx, s); err != nil {
		return err
	}
	result, err := execScriptOnVMForScenarioValidateExitCode(ctx, s,
		"curl --fail --silent --show-error "+cloudInitRepositoryEndpoint+cloudfixture.StatsPath, 0,
		"repository fixture is not running")
	if err != nil {
		return err
	}
	var stats cloudfixture.RepositoryStats
	if decodeErr := json.Unmarshal([]byte(result.stdout), &stats); decodeErr != nil {
		return fmt.Errorf("decode repository fixture evidence: %w", decodeErr)
	}
	expectedMetadata := 5
	if s.VHD.OS == config.OSAzureLinux {
		expectedMetadata = 7
	}
	if stats.Metadata < expectedMetadata || s.VHD.OS != config.OSAzureLinux && stats.Keys < 2 {
		return fmt.Errorf("repository setup was not exercised: %+v", stats)
	}
	if !optIn {
		_, optOutErr := execScriptOnVMForScenarioValidateExitCode(ctx, s, `set -euo pipefail
test -z "$(sudo find /root/AzureCACertificates -maxdepth 1 -type f -print -quit)"
if sudo crontab -l 2>/dev/null | grep -q refresh-cloud-certificates; then exit 1; fi
`, 0, "opt-out must skip certificates and scheduling without skipping RepoDepot")
		return optOutErr
	}
	_, err = execScriptOnVMForScenarioValidateExitCode(ctx, s, `sudo bash -s <<'CLOUD_TRUST_VALIDATION'
set -euo pipefail
bundle=/etc/ssl/certs/ca-certificates.crt
if [ -e /etc/pki/tls/certs/ca-bundle.crt ]; then bundle=/etc/pki/tls/certs/ca-bundle.crt; fi
count=0
for certificate in /root/AzureCACertificates/*.crt; do
    test -f "$certificate"
    openssl verify -partial_chain -CAfile "$bundle" "$certificate"
    count=$((count + 1))
done
test "$count" -gt 0
CLOUD_TRUST_VALIDATION
`, 0, "actual WireServer certificates must be accepted by the real system trust bundle")
	return err
}

func validateCloudInitializationRefresh(ctx context.Context, s *Scenario) error {
	directory := "/etc/apt/sources.list.d"
	cronFile := "/var/spool/cron/crontabs/root"
	keyFiles := "/etc/apt/keyrings/microsoft.asc.gpg /etc/apt/keyrings/msopentech.asc.gpg"
	if s.VHD.OS == config.OSAzureLinux {
		directory = "/etc/yum.repos.d"
		cronFile = "/var/spool/cron/root"
		keyFiles = "/etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY"
	}
	expected := "0 19 * * * " + strconv.Quote(cloudInitANCPath) +
		" refresh-cloud-certificates --location " + strconv.Quote(strings.ToLower(s.Location))
	script := fmt.Sprintf(`set -euo pipefail
before="$(sudo find %[1]s -maxdepth 1 -type f -exec sha256sum {} + | sort)"
keys="$(sudo sha256sum %[6]s)"
schedule="$(sudo stat -c '%%y:%%i' %[5]s)"
cron="$(sudo crontab -l)"
test "$(printf '%%s\n' "$cron" | grep -c refresh-cloud-certificates)" = 1
printf '%%s\n' "$cron" | grep -Fx %[2]s
sudo %[3]s refresh-cloud-certificates --location %[4]s
test "$before" = "$(sudo find %[1]s -maxdepth 1 -type f -exec sha256sum {} + | sort)"
test "$keys" = "$(sudo sha256sum %[6]s)"
test "$schedule" = "$(sudo stat -c '%%y:%%i' %[5]s)"
test "$cron" = "$(sudo crontab -l)"
`, directory, shellQuoteCloudFixture(expected), cloudInitANCPath, strconv.Quote(s.Location), cronFile, keyFiles)
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0,
		"standalone refresh must execute the scheduled PR binary without changing repositories or schedules")
	return err
}

func shellQuoteCloudFixture(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
