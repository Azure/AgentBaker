package parser

import (
	"encoding/base64"
	"strings"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
)

func TestOpenVMMImports(t *testing.T) {
	for _, version := range []string{"1.7.0", "2.2.0"} {
		for _, noGPU := range []bool{false, true} {
			for _, kata := range []bool{false, true} {
				config := &aksnodeconfigv1.Configuration{IsKata: kata}
				render := getContainerdConfigBase64
				if noGPU {
					render = getNoGPUContainerdConfigBase64
				}
				raw, err := base64.StdEncoding.DecodeString(render(config, version))
				if err != nil {
					t.Fatal(err)
				}
				text := string(raw)
				directory := "v1"
				if strings.HasPrefix(version, "2.") {
					directory = "v2"
				}
				if got := strings.Contains(text, `imports = ["/opt/kata-openvmm/containerd/`+directory+`/*.toml"]`); got != kata {
					t.Fatalf("version=%s noGPU=%t kata=%t: incorrect import:\n%s", version, noGPU, kata, text)
				}
				// Merely generating Kata configuration must not change the runtime defaults.
				if !strings.Contains(text, `default_runtime_name = "runc"`) ||
					strings.Contains(text, `runtime_type = "io.containerd.kata-openvmm.v2"`) {
					t.Fatalf("ordinary defaults changed:\n%s", text)
				}
				if kata && !strings.Contains(text, `runtime_type = "io.containerd.kata.v2"`) {
					t.Fatalf("existing Kata runtime missing:\n%s", text)
				}
			}
		}
	}
}
