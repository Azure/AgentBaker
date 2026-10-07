package agent

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/template"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/staging"
)

func hasWindowsKubeletConfiguration(config *datamodel.NodeBootstrappingConfiguration) bool {
	return config.AgentPoolProfile.IsWindows() && config.EnableKubeletConfigFile && config.KubeletConfigFileConfig != nil
}

func windowsKubeletFlagsToOmit(config *datamodel.NodeBootstrappingConfiguration) ([]string, error) {
	if !hasWindowsKubeletConfiguration(config) {
		return []string{}, nil
	}
	if err := validateWindowsKubeletConfiguration(config); err != nil {
		return nil, err
	}
	return decodeWindowsKubeletOmissionRequest(config.EnabledFeatures["KUBELET_FLAGS_TO_OMIT"])
}

func validateWindowsKubeletConfiguration(config *datamodel.NodeBootstrappingConfiguration) error {
	configuration := config.KubeletConfigFileConfig
	if configuration.Kind != "KubeletConfiguration" || configuration.APIVersion != "kubelet.config.k8s.io/v1beta1" {
		return fmt.Errorf("unsupported Windows kubelet configuration kind or apiVersion")
	}
	flagMaps := []map[string]string{config.KubeletConfig}
	if config.ContainerService != nil && config.ContainerService.Properties != nil {
		if overrides := config.ContainerService.Properties.GetComponentWindowsKubernetesConfiguration(datamodel.Componentkubelet); overrides != nil {
			flagMaps = append(flagMaps, overrides.Config)
		}
	}
	rotationFlag := ""
	for _, flags := range flagMaps {
		for _, flagName := range []string{"--config", "--config-dir"} {
			if _, present := flags[flagName]; present {
				return fmt.Errorf("explicit Windows kubelet configuration conflicts with %s", flagName)
			}
		}
		if value, present := flags["--rotate-server-certificates"]; present {
			rotationFlag = value
		}
	}
	if configuration.ServerTLSBootstrap && rotationFlag != "true" {
		return fmt.Errorf("windows serverTLSBootstrap requires the retained enabling serving-certificate rotation CLI and tag pipeline")
	}
	return nil
}

func decodeWindowsKubeletOmissionRequest(encoded string) ([]string, error) {
	selected := make([]string, 0)
	if encoded == "" {
		return selected, nil
	}
	if len(encoded) > 1024 || strings.ContainsAny(encoded, "\r\n") {
		return nil, fmt.Errorf("invalid Windows kubelet omission request encoding")
	}
	decoded, decodeErr := base64.StdEncoding.Strict().DecodeString(encoded)
	if decodeErr != nil {
		decoded, decodeErr = base64.RawStdEncoding.Strict().DecodeString(encoded)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("invalid Windows kubelet omission request: %w", decodeErr)
	}
	if !bytes.HasPrefix(bytes.TrimSpace(decoded), []byte("[")) {
		return nil, fmt.Errorf("windows kubelet omission request must be an array")
	}
	var requested []json.RawMessage
	if decodeErr = json.Unmarshal(decoded, &requested); decodeErr != nil {
		return nil, fmt.Errorf("invalid Windows kubelet omission request JSON: %w", decodeErr)
	}
	if len(requested) > 16 {
		return nil, fmt.Errorf("too many Windows kubelet omission names")
	}
	return selectWindowsKubeletFlagsToOmit(requested)
}

func selectWindowsKubeletFlagsToOmit(requested []json.RawMessage) ([]string, error) {
	selected := make([]string, 0)
	seen := make(map[string]bool)
	for _, entry := range requested {
		var flagName string
		if !bytes.HasPrefix(bytes.TrimSpace(entry), []byte("\"")) {
			return nil, fmt.Errorf("windows kubelet omission names must be strings")
		}
		if err := json.Unmarshal(entry, &flagName); err != nil {
			return nil, fmt.Errorf("invalid Windows kubelet omission name: %w", err)
		}
		if (flagName == "--volume-plugin-dir" || flagName == "--container-runtime-endpoint") && !seen[flagName] {
			seen[flagName] = true
			selected = append(selected, flagName)
		}
	}
	sort.Strings(selected)
	return selected, nil
}

func addWindowsKubeletConfigTemplateFuncs(config *datamodel.NodeBootstrappingConfiguration, functions template.FuncMap) {
	functions["HasWindowsKubeletConfiguration"] = func() bool { return hasWindowsKubeletConfiguration(config) }
	functions["GetWindowsKubeletFlagsToOmit"] = func() (string, error) {
		flags, err := windowsKubeletFlagsToOmit(config)
		if err != nil {
			return "", err
		}
		serialized, err := json.Marshal(flags)
		if err != nil {
			return "", err
		}
		return base64.RawStdEncoding.EncodeToString(serialized), nil
	}
	if !hasWindowsKubeletConfiguration(config) {
		return
	}
	baseScripts, present := functions["GetKubernetesWindowsAgentFunctions"].(func() string)
	if !present {
		return
	}
	functions["GetKubernetesWindowsAgentFunctions"] = func() (string, error) {
		if _, err := windowsKubeletFlagsToOmit(config); err != nil {
			return "", err
		}
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
