package agent

import (
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func TestContainerdV2TemplatesUseSplitCRIPluginPaths(t *testing.T) {
	for name, config := range map[string]ContainerdConfigTemplate{
		"gpu":    containerdV2ConfigTemplate,
		"no-gpu": containerdV2NoGPUConfigTemplate,
	} {
		t.Run(name, func(t *testing.T) {
			template := string(config)
			if !strings.HasPrefix(template, "version = {{GetContainerdConfigVersion}}\n") {
				t.Fatal("containerd v2 template must select the config version dynamically")
			}
			if !strings.Contains(template, `[plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata]`) {
				t.Fatal("containerd v2 template must configure Kata under the split CRI runtime plugin")
			}
			if strings.Contains(template, `plugins."io.containerd.grpc.v1.cri"`) {
				t.Fatal("containerd v2 template must not contain config-v2 CRI plugin paths")
			}
		})
	}
}

func TestGetContainerdConfigVersion(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		distro     datamodel.Distro
		wantSchema int
	}{
		{name: "containerd 2.0 uses schema v3", version: "2.0.0", distro: datamodel.AKSUbuntuContainerd2404Gen2, wantSchema: 3},
		{name: "containerd 2.2 uses schema v3", version: "2.2.4", distro: datamodel.AKSUbuntuContainerd2404Gen2, wantSchema: 3},
		{name: "containerd 2.3 uses schema v4", version: "2.3.5", distro: datamodel.AKSAzureLinuxV3Gen2, wantSchema: 4},
		{name: "Ubuntu 24.04 defaults to schema v4", distro: datamodel.AKSUbuntuContainerd2404Gen2, wantSchema: 4},
		{name: "Ubuntu 26.04 defaults to schema v4", distro: datamodel.AKSUbuntuMinimalContainerd2604Gen2, wantSchema: 4},
		{name: "Azure Linux 3 defaults to schema v3", distro: datamodel.AKSAzureLinuxV3Gen2, wantSchema: 3},
		{name: "ACL defaults to schema v3", distro: datamodel.AKSACLGen2TL, wantSchema: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &datamodel.NodeBootstrappingConfiguration{ContainerdVersion: tt.version}
			profile := &datamodel.AgentPoolProfile{Distro: tt.distro}
			if got := getContainerdConfigVersion(config, profile); got != tt.wantSchema {
				t.Fatalf("getContainerdConfigVersion() = %d, want %d", got, tt.wantSchema)
			}
		})
	}
}
