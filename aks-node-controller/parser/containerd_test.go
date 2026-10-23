package parser

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestContainerdImagePullTemplates(t *testing.T) {
	for _, version := range []string{"1.7.22", "2.2.4"} {
		for _, noGPU := range []bool{false, true} {
			for _, tc := range []struct {
				name     string
				settings *aksnodeconfigv1.ContainerdConfig
			}{
				{"absent", nil},
				{"empty", &aksnodeconfigv1.ContainerdConfig{}},
				{"false", &aksnodeconfigv1.ContainerdConfig{ImagePullWithSyncFs: proto.Bool(false)}},
				{"true", &aksnodeconfigv1.ContainerdConfig{ImagePullWithSyncFs: proto.Bool(true)}},
				{"zero timeout", &aksnodeconfigv1.ContainerdConfig{ImagePullProgressTimeout: proto.String("0s")}},
				{"both", &aksnodeconfigv1.ContainerdConfig{ImagePullWithSyncFs: proto.Bool(true), ImagePullProgressTimeout: proto.String("1m30s")}},
			} {
				t.Run(fmt.Sprintf("%s/noGPU=%t/%s", version, noGPU, tc.name), func(t *testing.T) {
					cfg := &aksnodeconfigv1.Configuration{
						KubernetesVersion: "1.34.0",
						GpuConfig:         &aksnodeconfigv1.GpuConfig{EnableNvidia: proto.Bool(true)},
						ContainerdConfig:  tc.settings,
					}
					rendered, err := containerdConfigFromAKSNodeConfig(cfg, noGPU, version)
					require.NoError(t, err)
					plugin := "io.containerd.grpc.v1.cri"
					if isContainerdV2(version) {
						plugin = "io.containerd.cri.v1.images"
					}
					header := `[plugins."` + plugin + `"]` + "\n"
					start := strings.Index(rendered, header)
					require.NotEqual(t, -1, start)
					section := rendered[start+len(header):]
					if next := regexp.MustCompile(`(?m)^[ \t]*\[`).FindStringIndex(section); next != nil {
						section = section[:next[0]]
					}
					if tc.settings != nil && tc.settings.ImagePullWithSyncFs != nil {
						require.Contains(t, section, fmt.Sprintf("  image_pull_with_sync_fs = %t\n", tc.settings.GetImagePullWithSyncFs()))
						require.Equal(t, 1, strings.Count(rendered, "image_pull_with_sync_fs"))
					} else {
						require.NotContains(t, rendered, "image_pull_with_sync_fs")
					}
					if tc.settings != nil && tc.settings.ImagePullProgressTimeout != nil {
						require.Contains(t, section, fmt.Sprintf("  image_pull_progress_timeout = %q\n", tc.settings.GetImagePullProgressTimeout()))
						require.Equal(t, 1, strings.Count(rendered, "image_pull_progress_timeout"))
					} else {
						require.NotContains(t, rendered, "image_pull_progress_timeout")
					}
					if tc.settings == nil || (tc.settings.ImagePullWithSyncFs == nil && tc.settings.ImagePullProgressTimeout == nil) {
						cfg.ContainerdConfig = nil
						baseline, err := containerdConfigFromAKSNodeConfig(cfg, noGPU, version)
						require.NoError(t, err)
						require.Equal(t, baseline, rendered)
					}
				})
			}
		}
	}
}

func TestBuildCSECmdContainerdImagePullSettings(t *testing.T) {
	for _, version := range []string{"1.7.22", "2.2.4"} {
		t.Run(version, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "containerd"), []byte(fmt.Sprintf(`#!/bin/sh
[ "$1" = "--version" ] || exit 1
echo "containerd containerd.io %s test"
`, version)), 0755))
			t.Setenv("PATH", dir)
			cfg := &aksnodeconfigv1.Configuration{ContainerdConfig: &aksnodeconfigv1.ContainerdConfig{
				ImagePullWithSyncFs:      proto.Bool(false),
				ImagePullProgressTimeout: proto.String("0s"),
			}}
			cmd, err := BuildCSECmd(context.Background(), cfg, nil)
			require.NoError(t, err)
			env := environToMap(cmd.Env)
			for _, key := range []string{"CONTAINERD_CONFIG_CONTENT", "CONTAINERD_CONFIG_NO_GPU_CONTENT"} {
				content, err := base64.StdEncoding.DecodeString(env[key])
				require.NoError(t, err)
				require.Contains(t, string(content), "image_pull_with_sync_fs = false")
				require.Contains(t, string(content), `image_pull_progress_timeout = "0s"`)
			}
			require.NotContains(t, env, "ANC_CONFIGURE_CONTAINERD_BINARY")
			require.NotContains(t, env, "CONTAINERD_IMAGE_PULL_WITH_SYNC_FS")
			require.NotContains(t, env, "CONTAINERD_IMAGE_PULL_PROGRESS_TIMEOUT")
		})
	}
}
