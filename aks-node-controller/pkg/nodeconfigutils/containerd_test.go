package nodeconfigutils

import (
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestContainerdImagePullValidationWithoutKubelet(t *testing.T) {
	for _, timeout := range []string{"0", "0s", "30s", "5m", "1m30s"} {
		cfg := &aksnodeconfigv1.Configuration{
			Version: "v1",
			ContainerdConfig: &aksnodeconfigv1.ContainerdConfig{
				ImagePullWithSyncFs:      proto.Bool(false),
				ImagePullProgressTimeout: proto.String(timeout),
			},
		}
		require.NoError(t, validateAndNormalize(cfg))
		require.NoError(t, validateAndNormalize(cfg))
		require.Equal(t, timeout, cfg.ContainerdConfig.GetImagePullProgressTimeout())
	}
	for _, timeout := range []string{"", "-1s", "invalid", "5", "999999999999999999999h", "5m\n"} {
		cfg := &aksnodeconfigv1.Configuration{
			Version:          "v1",
			ContainerdConfig: &aksnodeconfigv1.ContainerdConfig{ImagePullProgressTimeout: proto.String(timeout)},
		}
		require.ErrorContains(t, validateAndNormalize(cfg), "containerd_config.image_pull_progress_timeout")
	}
	require.NoError(t, validateAndNormalize(&aksnodeconfigv1.Configuration{Version: "v1"}))
}

func TestContainerdImagePullPresenceRoundTrip(t *testing.T) {
	for _, raw := range []string{
		`{"containerdConfig":{"imagePullWithSyncFs":false,"imagePullProgressTimeout":"0s"}}`,
		`{"containerd_config":{"image_pull_with_sync_fs":true,"image_pull_progress_timeout":"30s"}}`,
		`{"containerd_config":{}}`,
	} {
		cfg, err := UnmarshalConfigurationV1([]byte(raw))
		require.NoError(t, err)
		encoded, err := MarshalConfigurationV1(cfg)
		require.NoError(t, err)
		restored, err := UnmarshalConfigurationV1(encoded)
		require.NoError(t, err)
		require.True(t, proto.Equal(cfg, restored))
		wire, err := proto.Marshal(cfg)
		require.NoError(t, err)
		decoded := &aksnodeconfigv1.Configuration{}
		require.NoError(t, proto.Unmarshal(wire, decoded))
		require.True(t, proto.Equal(cfg, decoded))
		if cfg.ContainerdConfig.ImagePullWithSyncFs != nil {
			require.Contains(t, string(encoded), `"image_pull_with_sync_fs"`)
			require.NotNil(t, decoded.ContainerdConfig.ImagePullWithSyncFs)
			require.NotNil(t, decoded.ContainerdConfig.ImagePullProgressTimeout)
		} else {
			require.NotContains(t, string(encoded), "image_pull_")
		}
	}
}
