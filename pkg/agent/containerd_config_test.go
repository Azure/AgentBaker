package agent

import (
	"strings"
	"testing"
)

func TestContainerdV2TemplatesUseConfigV3PluginPaths(t *testing.T) {
	for name, config := range map[string]ContainerdConfigTemplate{
		"gpu":    containerdV2ConfigTemplate,
		"no-gpu": containerdV2NoGPUConfigTemplate,
	} {
		t.Run(name, func(t *testing.T) {
			template := string(config)
			if !strings.HasPrefix(template, "version = 3\n") {
				t.Fatal("containerd v2 template must use config version 3")
			}
			if !strings.Contains(template, `[plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata]`) {
				t.Fatal("containerd v2 template must configure Kata under the config-v3 runtime plugin")
			}
			if strings.Contains(template, `plugins."io.containerd.grpc.v1.cri"`) {
				t.Fatal("containerd v2 template must not contain config-v2 CRI plugin paths")
			}
		})
	}
}
