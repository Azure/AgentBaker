package agent

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"maps"
	"sort"
	"strings"
	"text/template"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/staging"
)

func hasWindowsKubeletConfiguration(config *datamodel.NodeBootstrappingConfiguration) bool {
	return len(windowsKubeletMigrationValues(config)) != 0
}

func windowsKubeletMigrationValues(config *datamodel.NodeBootstrappingConfiguration) map[string]string {
	if !config.AgentPoolProfile.IsWindows() || !config.EnableKubeletConfigFile {
		return nil
	}
	selected := make(map[string]string)
	for flagName, value := range kubeletConfigMigrationValues(config) {
		if flagName == "--volume-plugin-dir" || flagName == "--container-runtime-endpoint" {
			selected[flagName] = value
		}
	}
	return selected
}

func windowsKubeletConfigurationContent(flags, selected map[string]string) string {
	flags = maps.Clone(flags)
	for name, value := range flags {
		if value == `""""` || value == `""` {
			flags[name] = ""
		}
	}
	var configuration datamodel.AKSKubeletConfiguration
	_ = json.Unmarshal([]byte(GetKubeletConfigFileContent(flags, nil)), &configuration)
	configuration.NodeStatusReportFrequency = "5m"
	configuration.ContainerLogMaxFiles = strToInt32Ptr(flags["--container-log-max-files"])
	for _, field := range []*[]string{&configuration.ClusterDNS, &configuration.TLSCipherSuites, &configuration.EnforceNodeAllocatable} {
		if len(*field) == 1 && (*field)[0] == "" {
			*field = nil
		}
	}
	applyKubeletConfigMigration(&configuration, selected)
	content, _ := json.MarshalIndent(configuration, "", "    ")
	return string(content)
}

func addWindowsKubeletConfigTemplateFuncs(config *datamodel.NodeBootstrappingConfiguration, functions template.FuncMap) {
	selected := windowsKubeletMigrationValues(config)
	content, encodedFlags := "", ""
	functions["HasWindowsKubeletConfiguration"] = func() bool { return len(selected) != 0 }
	functions["GetWindowsKubeletConfigContentBase64"] = func() string { return base64.StdEncoding.EncodeToString([]byte(content)) }
	functions["GetWindowsKubeletFlagsToOmit"] = func() string { return encodedFlags }
	if len(selected) == 0 {
		return
	}
	working := *config
	working.KubeletConfig = maps.Clone(config.KubeletConfig)
	working.GetOrderedKubeletConfigStringForPowershell(config.AgentPoolProfile.CustomKubeletConfig)
	content = windowsKubeletConfigurationContent(working.KubeletConfig, selected)
	functions["EnableKubeletServingCertificateRotation"] = func() bool { return IsKubeletServingCertificateRotationEnabled(&working) }
	flags := make([]string, 0, len(selected))
	for name := range selected {
		flags = append(flags, name)
		delete(working.KubeletConfig, name)
	}
	sort.Strings(flags)
	serialized, _ := json.Marshal(flags)
	encodedFlags = base64.RawStdEncoding.EncodeToString(serialized)
	functions["GetKubeletConfigKeyValsPsh"] = func() string {
		return working.GetOrderedKubeletConfigStringForPowershell(config.AgentPoolProfile.CustomKubeletConfig)
	}
	baseScripts, present := functions["GetKubernetesWindowsAgentFunctions"].(func() string)
	if !present {
		return
	}
	functions["GetKubernetesWindowsAgentFunctions"] = func() (string, error) {
		return appendWindowsKubeletConfigScripts(baseScripts())
	}
}

func appendWindowsKubeletConfigScripts(encoded string) (string, error) {
	contents, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	archive, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, file := range archive.File {
		if err = writer.Copy(file); err != nil {
			return "", err
		}
	}
	for _, path := range []string{"cse/windows/kubeletconfig.ps1", "cse/windows/provisioningscripts/kubeletstart.ps1"} {
		contents, readErr := staging.WindowsKubeletScripts.ReadFile(path)
		if readErr != nil {
			return "", readErr
		}
		filename := path[strings.LastIndex(path, "/")+1:]
		var entry io.Writer
		entry, err = writer.Create("windows/kubeletconfiguration/" + filename)
		if err != nil {
			return "", err
		}
		if _, err = entry.Write(contents); err != nil {
			return "", err
		}
	}
	if err = writer.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}
