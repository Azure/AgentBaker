package nbcontractv1

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/blang/semver"
)

// NBContractBuilder is a helper struct to build the NBContract (Node Bootstrap Contract).
// It provides methods to apply configuration, get the NBContract object, and validate the contract, etc.
type NBContractBuilder struct {
	// nodeBootstrapConfig is the configuration object for the NBContract (Node Bootstrap Contract).
	nodeBootstrapConfig *Configuration
}

// Check and initialize each field if it is nil.
func initializeIfNil[T any](field **T) {
	if *field == nil {
		*field = new(T)
	}
}

// Ensure all objects are non-nil. Please add new objects here.
func ensureConfigsNonNil(nBC *Configuration) {
	initializeIfNil(&nBC.KubeBinaryConfig)
	initializeIfNil(&nBC.ApiServerConfig)
	initializeIfNil(&nBC.AuthConfig)
	initializeIfNil(&nBC.ClusterConfig)
	initializeIfNil(&nBC.ClusterConfig.LoadBalancerConfig)
	initializeIfNil(&nBC.ClusterConfig.ClusterNetworkConfig)
	initializeIfNil(&nBC.GpuConfig)
	initializeIfNil(&nBC.NetworkConfig)
	initializeIfNil(&nBC.TlsBootstrappingConfig)
	initializeIfNil(&nBC.KubeletConfig)
	initializeIfNil(&nBC.RuncConfig)
	initializeIfNil(&nBC.ContainerdConfig)
	initializeIfNil(&nBC.TeleportConfig)
	initializeIfNil(&nBC.CustomLinuxOsConfig)
	initializeIfNil(&nBC.CustomLinuxOsConfig.SysctlConfig)
	initializeIfNil(&nBC.CustomLinuxOsConfig.UlimitConfig)
	initializeIfNil(&nBC.HttpProxyConfig)
	initializeIfNil(&nBC.CustomCloudConfig)
	initializeIfNil(&nBC.CustomSearchDomainConfig)
}

// NewNBContractBuilder creates a new instance of NBContractBuilder and ensures all objects in nodeBootstrapConfig are non-nil.
func NewNBContractBuilder() *NBContractBuilder {
	nbc := Configuration{
		Version: contractVersion,
	}
	ensureConfigsNonNil(&nbc)
	nBCB := &NBContractBuilder{nodeBootstrapConfig: &nbc}
	return nBCB
}

// ApplyConfiguration Applies the configuration to the nodeBootstrapConfig object.
func (nBCB *NBContractBuilder) ApplyConfiguration(config *Configuration) {
	if config == nil {
		return
	}

	// Use deep copy to avoid modifying the original object 'config'.
	if err := nBCB.deepCopy(config, nBCB.nodeBootstrapConfig); err != nil {
		log.Printf("Failed to deep copy the configuration: %v", err)
		ensureConfigsNonNil(nBCB.nodeBootstrapConfig)
	}
}

// GetNodeBootstrapConfig gets the nodeBootstrapConfig object.
func (nBCB *NBContractBuilder) GetNodeBootstrapConfig() *Configuration {
	return nBCB.nodeBootstrapConfig
}

// Deep copy the source object to the destination object.
// Note that the existing value in the destination object will not be cleared
// if the source object doesn't have that field.
func (nBCB *NBContractBuilder) deepCopy(src, dst interface{}) error {
	if src == nil {
		return nil
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	if err := gob.NewDecoder(&buf).Decode(dst); err != nil {
		return err
	}
	return nil
}

// ValidateNBContract validates the NBContract.
// It returns an error if the contract is invalid.
// This function should be called after applying all configuration and before sending to downstream component.
func (nBCB *NBContractBuilder) ValidateNBContract() error {
	if err := nBCB.validateSemVer(); err != nil {
		return err
	}
	if err := nBCB.validateRequiredFields(); err != nil {
		return err
	}
	// Add more validations here if needed.

	return nil
}

func (nBCB *NBContractBuilder) validateSemVer() error {
	payloadContractVer, err := semver.Make(nBCB.nodeBootstrapConfig.Version)
	if err != nil {
		return fmt.Errorf("invalid contract version from contract payload: %s. It should be a semantic version", nBCB.nodeBootstrapConfig.Version)
	}

	expectedContractVer, err := semver.Make(contractVersion)

	if err != nil {
		return fmt.Errorf("invalid contract version from contract payload: %s. It should be a semantic version", nBCB.nodeBootstrapConfig.Version)
	}

	if payloadContractVer.Major != expectedContractVer.Major {
		return fmt.Errorf("contract major versions mismatch. Expecting %v, but got %v", expectedContractVer.Major, payloadContractVer.Major)
	}

	if payloadContractVer.Minor != expectedContractVer.Minor {
		// Minor version mismatch is not a breaking change. So just log a warning.
		log.Printf("Warning: Contract minor versions mismatch. Expecting %v, but got %v", expectedContractVer.Minor, payloadContractVer.Minor)
	}

	return nil
}

func (nBCB *NBContractBuilder) parseMinor(version string) (int, error) {
	// It's ensured that it's a valid semantic version, namely x.y.z.
	parts := strings.Split(version, ".")
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return -1, fmt.Errorf("failed to parse minor version from %s", version)
	}
	return minor, nil
}

func (nBCB *NBContractBuilder) validateRequiredFields() error {
	if err := nBCB.validateRequiredStringsNotEmpty(); err != nil {
		return err
	}
	// Add more required fields validations here if needed.
	// For example, check if a required string is of a specific format.
	return nil
}

func (nBCB *NBContractBuilder) validateRequiredStringsNotEmpty() error {
	requiredStrings := map[string]string{
		"AuthConfig.SubscriptionId":                     nBCB.nodeBootstrapConfig.GetAuthConfig().GetSubscriptionId(),
		"ClusterConfig.ResourceGroup":                   nBCB.nodeBootstrapConfig.GetClusterConfig().GetResourceGroup(),
		"ClusterConfig.Location":                        nBCB.nodeBootstrapConfig.GetClusterConfig().GetLocation(),
		"ClusterConfig.ClusterNetworkConfig.VnetName":   nBCB.nodeBootstrapConfig.GetClusterConfig().GetClusterNetworkConfig().GetVnetName(),
		"ClusterConfig.ClusterNetworkConfig.RouteTable": nBCB.nodeBootstrapConfig.GetClusterConfig().GetClusterNetworkConfig().GetRouteTable(),
		"ApiServerConfig.ApiServerName":                 nBCB.nodeBootstrapConfig.ApiServerConfig.GetApiServerName(),
	}

	for field, value := range requiredStrings {
		if value == "" {
			return fmt.Errorf("required field %v is missing", field)
		}
	}
	return nil
}

// getLoadBalancerSKI returns the LoadBalancerSku enum based on the input string.
func getLoadBalancerSKU(sku string) LoadBalancerConfig_LoadBalancerSku {
	if strings.EqualFold(sku, LoadBalancerStandard) {
		return LoadBalancerConfig_STANDARD
	} else if strings.EqualFold(sku, LoadBalancerBasic) {
		return LoadBalancerConfig_BASIC
	}

	return LoadBalancerConfig_UNSPECIFIED
}

// getNetworkPluginType returns the NetworkPluginType enum based on the input string.
func getNetworkPluginType(networkPlugin string) NetworkPlugin {
	if strings.EqualFold(networkPlugin, NetworkPluginAzure) {
		return NetworkPlugin_NP_AZURE
	} else if strings.EqualFold(networkPlugin, NetworkPluginkubenet) {
		return NetworkPlugin_NP_KUBENET
	}

	return NetworkPlugin_NP_NONE
}

// getNetworkPolicyType returns the NetworkPolicyType enum based on the input string.
func getNetworkPolicyType(networkPolicy string) NetworkPolicy {
	if strings.EqualFold(networkPolicy, NetworkPolicyAzure) {
		return NetworkPolicy_NPO_AZURE
	} else if strings.EqualFold(networkPolicy, NetworkPolicyCalico) {
		return NetworkPolicy_NPO_CALICO
	}

	return NetworkPolicy_NPO_NONE
}

// GetOutBoundCmd returns a proper outbound traffic command based on some cloud and Linux distro configs.
func GetOutBoundCmd(nbconfig *datamodel.NodeBootstrappingConfiguration, cloudName string) string {
	cs := nbconfig.ContainerService
	if cs.Properties.FeatureFlags.IsFeatureEnabled("BlockOutboundInternet") {
		return ""
	}

	registry := ""
	switch {
	case cloudName == AzureChinaCloud:
		registry = `gcr.azk8s.cn`
	case cs.IsAKSCustomCloud():
		registry = cs.Properties.CustomCloudEnv.McrURL
	default:
		registry = `mcr.microsoft.com`
	}

	if registry == "" {
		return ""
	}

	// curl on Ubuntu 16.04 (shipped prior to AKS 1.18) doesn't support proxy TLS.
	// so we need to use nc for the connectivity check.
	orchestratorVersion, err := semver.Make(cs.Properties.OrchestratorProfile.OrchestratorVersion)
	if err != nil {
		log.Printf("Failed to parse orchestrator version %s: %v", cs.Properties.OrchestratorProfile.OrchestratorVersion, err)
		return ""
	}

	minVersion, err := semver.Make("1.18.0")
	if err != nil {
		log.Printf("Failed to parse min version %s: %v", "1.18.0", err)
		return ""
	}

	var connectivityCheckCommand string
	if orchestratorVersion.GTE(minVersion) {
		connectivityCheckCommand = `curl -v --insecure --proxy-insecure https://` + registry + `/v2/`
	} else {
		connectivityCheckCommand = `nc -vz ` + registry + ` 443`
	}

	return connectivityCheckCommand
}

// GetDefaultOutboundCommand returns a default outbound traffic command.
func GetDefaultOutboundCommand() string {
	return "curl -v --insecure --proxy-insecure https://mcr.microsoft.com/v2/"
}
