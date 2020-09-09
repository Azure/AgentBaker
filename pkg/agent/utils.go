// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/pkg/templates"
	"github.com/blang/semver"
	"github.com/pkg/errors"
)

var TranslatedKubeletConfigFlags map[string]bool = map[string]bool{
	"--address":                           true,
	"--anonymous-auth":                    true,
	"--client-ca-file":                    true,
	"--authentication-token-webhook":      true,
	"--authorization-mode":                true,
	"--pod-manifest-path":                 true,
	"--cluster-dns":                       true,
	"--cgroups-per-qos":                   true,
	"--tls-cert-file":                     true,
	"--tls-private-key-file":              true,
	"--tls-cipher-suites":                 true,
	"--cluster-domain":                    true,
	"--max-pods":                          true,
	"--eviction-hard":                     true,
	"--node-status-update-frequency":      true,
	"--image-gc-high-threshold":           true,
	"--image-gc-low-threshold":            true,
	"--event-qps":                         true,
	"--pod-max-pids":                      true,
	"--enforce-node-allocatable":          true,
	"--streaming-connection-idle-timeout": true,
	"--rotate-certificates":               true,
	"--read-only-port":                    true,
	"--feature-gates":                     true,
	"--protect-kernel-defaults":           true,
	"--resolv-conf":                       true,
	"--system-reserved":                   true,
	"--kube-reserved":                     true,
}

var keyvaultSecretPathRe *regexp.Regexp

func init() {
	keyvaultSecretPathRe = regexp.MustCompile(`^(/subscriptions/\S+/resourceGroups/\S+/providers/Microsoft.KeyVault/vaults/\S+)/secrets/([^/\s]+)(/(\S+))?$`)
}

type paramsMap map[string]interface{}

// generateConsecutiveIPsList takes a starting IP address and returns a string slice of length "count" of subsequent, consecutive IP addresses
func generateConsecutiveIPsList(count int, firstAddr string) ([]string, error) {
	ipaddr := net.ParseIP(firstAddr).To4()
	if ipaddr == nil {
		return nil, errors.Errorf("IPAddr '%s' is an invalid IP address", firstAddr)
	}
	if int(ipaddr[3])+count >= 255 {
		return nil, errors.Errorf("IPAddr '%s' + %d will overflow the fourth octet", firstAddr, count)
	}
	ret := make([]string, count)
	for i := 0; i < count; i++ {
		nextAddress := fmt.Sprintf("%d.%d.%d.%d", ipaddr[0], ipaddr[1], ipaddr[2], ipaddr[3]+byte(i))
		ipaddr := net.ParseIP(nextAddress).To4()
		if ipaddr == nil {
			return nil, errors.Errorf("IPAddr '%s' is an invalid IP address", nextAddress)
		}
		ret[i] = nextAddress
	}
	return ret, nil
}

func addValue(m paramsMap, k string, v interface{}) {
	m[k] = paramsMap{
		"value": v,
	}
}

func addKeyvaultReference(m paramsMap, k string, vaultID, secretName, secretVersion string) {
	m[k] = paramsMap{
		"reference": &KeyVaultRef{
			KeyVault: KeyVaultID{
				ID: vaultID,
			},
			SecretName:    secretName,
			SecretVersion: secretVersion,
		},
	}
}

func addSecret(m paramsMap, k string, v interface{}, encode bool) {
	str, ok := v.(string)
	if !ok {
		addValue(m, k, v)
		return
	}
	parts := keyvaultSecretPathRe.FindStringSubmatch(str)
	if parts == nil || len(parts) != 5 {
		if encode {
			addValue(m, k, base64.StdEncoding.EncodeToString([]byte(str)))
		} else {
			addValue(m, k, str)
		}
		return
	}
	addKeyvaultReference(m, k, parts[1], parts[2], parts[4])
}

func makeAgentExtensionScriptCommands(cs *datamodel.ContainerService, profile *datamodel.AgentPoolProfile) string {
	if profile.OSType == datamodel.Windows {
		return makeWindowsExtensionScriptCommands(profile.PreprovisionExtension,
			cs.Properties.ExtensionProfiles)
	}
	return makeExtensionScriptCommands(profile.PreprovisionExtension,
		"", cs.Properties.ExtensionProfiles)
}

func makeExtensionScriptCommands(extension *datamodel.Extension, curlCaCertOpt string, extensionProfiles []*datamodel.ExtensionProfile) string {
	var extensionProfile *datamodel.ExtensionProfile
	for _, eP := range extensionProfiles {
		if strings.EqualFold(eP.Name, extension.Name) {
			extensionProfile = eP
			break
		}
	}

	if extensionProfile == nil {
		panic(fmt.Sprintf("%s extension referenced was not found in the extension profile", extension.Name))
	}

	extensionsParameterReference := fmt.Sprintf("parameters('%sParameters')", extensionProfile.Name)
	scriptURL := getExtensionURL(extensionProfile.RootURL, extensionProfile.Name, extensionProfile.Version, extensionProfile.Script, extensionProfile.URLQuery)
	scriptFilePath := fmt.Sprintf("/opt/azure/containers/extensions/%s/%s", extensionProfile.Name, extensionProfile.Script)
	return fmt.Sprintf("- sudo /usr/bin/curl --retry 5 --retry-delay 10 --retry-max-time 30 -o %s --create-dirs %s \"%s\" \n- sudo /bin/chmod 744 %s \n- sudo %s ',%s,' > /var/log/%s-output.log",
		scriptFilePath, curlCaCertOpt, scriptURL, scriptFilePath, scriptFilePath, extensionsParameterReference, extensionProfile.Name)
}

func makeWindowsExtensionScriptCommands(extension *datamodel.Extension, extensionProfiles []*datamodel.ExtensionProfile) string {
	var extensionProfile *datamodel.ExtensionProfile
	for _, eP := range extensionProfiles {
		if strings.EqualFold(eP.Name, extension.Name) {
			extensionProfile = eP
			break
		}
	}

	if extensionProfile == nil {
		panic(fmt.Sprintf("%s extension referenced was not found in the extension profile", extension.Name))
	}

	scriptURL := getExtensionURL(extensionProfile.RootURL, extensionProfile.Name, extensionProfile.Version, extensionProfile.Script, extensionProfile.URLQuery)
	scriptFileDir := fmt.Sprintf("$env:SystemDrive:/AzureData/extensions/%s", extensionProfile.Name)
	scriptFilePath := fmt.Sprintf("%s/%s", scriptFileDir, extensionProfile.Script)
	return fmt.Sprintf("New-Item -ItemType Directory -Force -Path \"%s\" ; Invoke-WebRequest -Uri \"%s\" -OutFile \"%s\" ; powershell \"%s `\"',parameters('%sParameters'),'`\"\"\n", scriptFileDir, scriptURL, scriptFilePath, scriptFilePath, extensionProfile.Name)
}

func getVNETSubnetDependencies(properties *datamodel.Properties) string {
	agentString := `        "[concat('Microsoft.Network/networkSecurityGroups/', variables('%sNSGName'))]"`
	var buf bytes.Buffer
	for index, agentProfile := range properties.AgentPoolProfiles {
		if index > 0 {
			buf.WriteString(",\n")
		}
		buf.WriteString(fmt.Sprintf(agentString, agentProfile.Name))
	}
	return buf.String()
}

func getLBRule(name string, port int) string {
	return fmt.Sprintf(`	          {
            "name": "LBRule%d",
            "properties": {
              "backendAddressPool": {
                "id": "[concat(variables('%sLbID'), '/backendAddressPools/', variables('%sLbBackendPoolName'))]"
              },
              "backendPort": %d,
              "enableFloatingIP": false,
              "frontendIPConfiguration": {
                "id": "[variables('%sLbIPConfigID')]"
              },
              "frontendPort": %d,
              "idleTimeoutInMinutes": 5,
              "loadDistribution": "Default",
              "probe": {
                "id": "[concat(variables('%sLbID'),'/probes/tcp%dProbe')]"
              },
              "protocol": "Tcp"
            }
          }`, port, name, name, port, name, port, name, port)
}

func getProbe(port int) string {
	return fmt.Sprintf(`          {
            "name": "tcp%dProbe",
            "properties": {
              "intervalInSeconds": 5,
              "numberOfProbes": 2,
              "port": %d,
              "protocol": "Tcp"
            }
          }`, port, port)
}

func getSecurityRule(port int, portIndex int) string {
	// BaseLBPriority specifies the base lb priority.
	BaseLBPriority := 200
	return fmt.Sprintf(`          {
            "name": "Allow_%d",
            "properties": {
              "access": "Allow",
              "description": "Allow traffic from the Internet to port %d",
              "destinationAddressPrefix": "*",
              "destinationPortRange": "%d",
              "direction": "Inbound",
              "priority": %d,
              "protocol": "*",
              "sourceAddressPrefix": "Internet",
              "sourcePortRange": "*"
            }
          }`, port, port, port, BaseLBPriority+portIndex)
}

func escapeSingleLine(escapedStr string) string {
	// template.JSEscapeString leaves undesirable chars that don't work with pretty print
	escapedStr = strings.Replace(escapedStr, "\\", "\\\\", -1)
	escapedStr = strings.Replace(escapedStr, "\r\n", "\\n", -1)
	escapedStr = strings.Replace(escapedStr, "\n", "\\n", -1)
	escapedStr = strings.Replace(escapedStr, "\"", "\\\"", -1)
	return escapedStr
}

// getBase64EncodedGzippedCustomScript will return a base64 of the CSE
func getBase64EncodedGzippedCustomScript(csFilename string, config *NodeBootstrappingConfiguration) string {
	b, err := templates.Asset(csFilename)
	if err != nil {
		// this should never happen and this is a bug
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}
	// translate the parameters
	templ := template.New("ContainerService template").Option("missingkey=error").Funcs(getContainerServiceFuncMap(config))
	_, err = templ.Parse(string(b))
	if err != nil {
		// this should never happen and this is a bug
		panic(fmt.Sprintf("BUG: %s", err.Error()))
	}
	var buffer bytes.Buffer
	templ.Execute(&buffer, config.ContainerService)
	csStr := buffer.String()
	csStr = strings.Replace(csStr, "\r\n", "\n", -1)
	return getBase64EncodedGzippedCustomScriptFromStr(csStr)
}

func getStringFromBase64(str string) (string, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(str)
	return string(decodedBytes), err
}

// getBase64EncodedGzippedCustomScriptFromStr will return a base64-encoded string of the gzip'd source data
func getBase64EncodedGzippedCustomScriptFromStr(str string) string {
	var gzipB bytes.Buffer
	w := gzip.NewWriter(&gzipB)
	w.Write([]byte(str))
	w.Close()
	return base64.StdEncoding.EncodeToString(gzipB.Bytes())
}

func getExtensionURL(rootURL, extensionName, version, fileName, query string) string {
	extensionsDir := "extensions"
	url := rootURL + extensionsDir + "/" + extensionName + "/" + version + "/" + fileName
	if query != "" {
		url += "?" + query
	}
	return url
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func getSSHPublicKeysPowerShell(linuxProfile *datamodel.LinuxProfile) string {
	str := ""
	if linuxProfile != nil {
		lastItem := len(linuxProfile.SSH.PublicKeys) - 1
		for i, publicKey := range linuxProfile.SSH.PublicKeys {
			str += `"` + strings.TrimSpace(publicKey.KeyData) + `"`
			if i < lastItem {
				str += ", "
			}
		}
	}
	return str
}

func getWindowsMasterSubnetARMParam(masterProfile *datamodel.MasterProfile) string {
	if masterProfile != nil && masterProfile.IsCustomVNET() {
		return fmt.Sprintf("',parameters('vnetCidr'),'")
	}
	return fmt.Sprintf("',parameters('masterSubnet'),'")
}

// IsNvidiaEnabledSKU determines if an VM SKU has nvidia driver support
func IsNvidiaEnabledSKU(vmSize string) bool {
	/* If a new GPU sku becomes available, add a key to this map, but only if you have a confirmation
	   that we have an agreement with NVIDIA for this specific gpu.
	*/
	dm := map[string]bool{
		// K80
		"Standard_NC6":   true,
		"Standard_NC12":  true,
		"Standard_NC24":  true,
		"Standard_NC24r": true,
		// M60
		"Standard_NV6":      true,
		"Standard_NV12":     true,
		"Standard_NV12s_v3": true,
		"Standard_NV24":     true,
		"Standard_NV24s_v3": true,
		"Standard_NV24r":    true,
		"Standard_NV48s_v3": true,
		// P40
		"Standard_ND6s":   true,
		"Standard_ND12s":  true,
		"Standard_ND24s":  true,
		"Standard_ND24rs": true,
		// P100
		"Standard_NC6s_v2":   true,
		"Standard_NC12s_v2":  true,
		"Standard_NC24s_v2":  true,
		"Standard_NC24rs_v2": true,
		// V100
		"Standard_NC6s_v3":   true,
		"Standard_NC12s_v3":  true,
		"Standard_NC24s_v3":  true,
		"Standard_NC24rs_v3": true,
		"Standard_ND40s_v3":  true,
		"Standard_ND40rs_v2": true,
	}
	// Trim the optional _Promo suffix.
	vmSize = strings.TrimSuffix(vmSize, "_Promo")
	if _, ok := dm[vmSize]; ok {
		return dm[vmSize]
	}

	return false
}

// IsSgxEnabledSKU determines if an VM SKU has SGX driver support
func IsSgxEnabledSKU(vmSize string) bool {
	switch vmSize {
	case "Standard_DC2s", "Standard_DC4s":
		return true
	}
	return false
}

// GetCloudTargetEnv determines and returns whether the region is a sovereign cloud which
// have their own data compliance regulations (China/Germany/USGov) or standard
// Azure public cloud
func GetCloudTargetEnv(location string) string {
	loc := strings.ToLower(strings.Join(strings.Fields(location), ""))
	switch {
	case loc == "chinaeast" || loc == "chinanorth" || loc == "chinaeast2" || loc == "chinanorth2":
		return "AzureChinaCloud"
	case loc == "germanynortheast" || loc == "germanycentral":
		return "AzureGermanCloud"
	case strings.HasPrefix(loc, "usgov") || strings.HasPrefix(loc, "usdod"):
		return "AzureUSGovernmentCloud"
	default:
		return "AzurePublicCloud"
	}
}

// IsKubernetesVersionGe returns true if actualVersion is greater than or equal to version
func IsKubernetesVersionGe(actualVersion, version string) bool {
	v1, _ := semver.Make(actualVersion)
	v2, _ := semver.Make(version)
	return v1.GE(v2)
}

func getCustomDataFromJSON(jsonStr string) string {
	var customDataObj map[string]string
	err := json.Unmarshal([]byte(jsonStr), &customDataObj)
	if err != nil {
		panic(err)
	}
	return customDataObj["customData"]
}

// GetOrderedKubeletConfigFlagString returns an ordered string of key/val pairs
// copied from AKS-Engine and filter out flags that already translated to config file
func GetOrderedKubeletConfigFlagString(k *datamodel.KubernetesConfig, cs *datamodel.ContainerService, dynamicKubeletToggleEnabled bool) string {
	if k.KubeletConfig == nil {
		return ""
	}
	ensureKubeletConfigFlagsValue(k.KubeletConfig, cs, dynamicKubeletToggleEnabled)
	keys := []string{}
	dynamicKubeletEnabled := IsDynamicKubeletEnabled(cs, dynamicKubeletToggleEnabled)
	for key := range k.KubeletConfig {
		if !dynamicKubeletEnabled || !TranslatedKubeletConfigFlags[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, key := range keys {
		buf.WriteString(fmt.Sprintf("%s=%s ", key, k.KubeletConfig[key]))
	}
	return buf.String()
}

// IsDynamicKubeletEnabled get if dynamic kubelet is supported in AKS
func IsDynamicKubeletEnabled(cs *datamodel.ContainerService, dynamicKubeletToggleEnabled bool) bool {
	// TODO(bowa) remove toggle when backfill
	return dynamicKubeletToggleEnabled && cs.Properties.OrchestratorProfile.IsKubernetes() && IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, "1.14.0")
}

func ensureKubeletConfigFlagsValue(kc map[string]string, cs *datamodel.ContainerService, dynamicKubeletToggleEnabled bool) {
	// for now it's only dynamic kubelet, we could add more in future
	if IsDynamicKubeletEnabled(cs, dynamicKubeletToggleEnabled) && kc["--dynamic-config-dir"] == "" {
		kc["--dynamic-config-dir"] = DynamicKubeletConfigDir
	}
}

// convert kubelet flags we set to a file
func getDynamicKubeletConfigFileContent(kc map[string]string) string {
	if kc == nil {
		return ""
	}
	// translate simple values
	kubeletConfig := &AKSKubeletConfiguration{
		APIVersion:    "kubelet.config.k8s.io/v1beta1",
		Kind:          "KubeletConfiguration",
		Address:       kc["--address"],
		StaticPodPath: kc["--pod-manifest-path"],
		Authorization: KubeletAuthorization{
			Mode: KubeletAuthorizationMode(kc["--authorization-mode"]),
		},
		ClusterDNS:                     strings.Split(kc["--cluster-dns"], ","),
		CgroupsPerQOS:                  strToBool(kc["--cgroups-per-qos"]),
		TLSCertFile:                    kc["--tls-cert-file"],
		TLSPrivateKeyFile:              kc["--tls-private-key-file"],
		TLSCipherSuites:                strings.Split(kc["--tls-cipher-suites"], ","),
		ClusterDomain:                  kc["--cluster-domain"],
		MaxPods:                        strToInt32(kc["--max-pods"]),
		NodeStatusUpdateFrequency:      Duration(kc["--node-status-update-frequency"]),
		ImageGCHighThresholdPercent:    strToInt32(kc["--image-gc-high-threshold"]),
		ImageGCLowThresholdPercent:     strToInt32(kc["--image-gc-low-threshold"]),
		EventRecordQPS:                 strToInt32(kc["--event-qps"]),
		PodPidsLimit:                   strToInt64(kc["--pod-max-pids"]),
		EnforceNodeAllocatable:         strings.Split(kc["--enforce-node-allocatable"], ","),
		StreamingConnectionIdleTimeout: Duration(kc["--streaming-connection-idle-timeout"]),
		RotateCertificates:             strToBool(kc["--rotate-certificates"]),
		ReadOnlyPort:                   strToInt32(kc["--read-only-port"]),
		ProtectKernelDefaults:          strToBool(kc["--protect-kernel-defaults"]),
		ResolverConfig:                 kc["--resolv-conf"],
	}

	// Authentication
	kubeletConfig.Authentication = KubeletAuthentication{}
	if ca := kc["--client-ca-file"]; ca != "" {
		kubeletConfig.Authentication.X509 = KubeletX509Authentication{
			ClientCAFile: ca,
		}
	}
	if aw := kc["--authentication-token-webhook"]; aw != "" {
		kubeletConfig.Authentication.Webhook = KubeletWebhookAuthentication{
			Enabled: strToBool(aw),
		}
	}
	if aa := kc["--anonymous-auth"]; aa != "" {
		kubeletConfig.Authentication.Anonymous = KubeletAnonymousAuthentication{
			Enabled: strToBool(aa),
		}
	}

	// EvictionHard
	// default: "memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%"
	if eh, ok := kc["--eviction-hard"]; ok && eh != "" {
		kubeletConfig.EvictionHard = strKeyValToMap(eh, ",", "<")
	}

	// feature gates
	// look like "f1=true,f2=true"
	kubeletConfig.FeatureGates = strKeyValToMapBool(kc["--feature-gates"], ",", "=")

	// system reserve and kube reserve
	// looks like "cpu=100m,memory=1638Mi"
	kubeletConfig.SystemReserved = strKeyValToMap(kc["--system-reserved"], ",", "=")
	kubeletConfig.KubeReserved = strKeyValToMap(kc["--kube-reserved"], ",", "=")

	configStringByte, _ := json.MarshalIndent(kubeletConfig, "", "    ")
	return string(configStringByte)
}

func strToBool(str string) bool {
	b, _ := strconv.ParseBool(str)
	return b
}

func strToInt32(str string) int32 {
	i, _ := strconv.ParseInt(str, 10, 32)
	return int32(i)
}

func strToInt64(str string) int64 {
	i, _ := strconv.ParseInt(str, 10, 64)
	return i
}

func strKeyValToMap(str string, strDelim string, pairDelim string) map[string]string {
	m := make(map[string]string)
	pairs := strings.Split(str, strDelim)
	for _, pairRaw := range pairs {
		pair := strings.Split(pairRaw, pairDelim)
		if len(pair) == 2 {
			key := strings.TrimSpace(pair[0])
			val := strings.TrimSpace(pair[1])
			m[key] = val
		}
	}
	return m
}

func strKeyValToMapBool(str string, strDelim string, pairDelim string) map[string]bool {
	m := make(map[string]bool)
	pairs := strings.Split(str, strDelim)
	for _, pairRaw := range pairs {
		pair := strings.Split(pairRaw, pairDelim)
		if len(pair) == 2 {
			key := strings.TrimSpace(pair[0])
			val := strings.TrimSpace(pair[1])
			m[key] = strToBool(val)
		}
	}
	return m
}
