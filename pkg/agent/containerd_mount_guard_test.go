// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"testing"

	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestContainerdMountGuardCustomData(t *testing.T) {
	template, err := parts.Templates.ReadFile(kubernetesNodeCustomDataYaml)
	require.NoError(t, err)
	for _, distro := range []datamodel.Distro{
		datamodel.AKSUbuntuContainerd2204Gen2,
		datamodel.AKSUbuntuContainerd2404Gen2,
		datamodel.AKSAzureLinuxV3Gen2,
		datamodel.AKSAzureLinuxV3Gen2FIPS,
	} {
		t.Run(string(distro), func(t *testing.T) {
			for _, diskType := range []datamodel.KubeletDiskType{datamodel.OSDisk, datamodel.TempDisk} {
				t.Run(string(diskType), func(t *testing.T) {
					for _, scriptless := range []bool{false, true} {
						config := newNodeCustomDataRenderConfig(distro)
						config.EnableScriptlessCSECmd = scriptless
						config.AgentPoolProfile.KubeletDiskType = diskType
						rendered, err := RenderLinuxNodeCustomDataTemplate(template, config)
						require.NoError(t, err)
						var data cloudInit
						require.NoError(t, yaml.Unmarshal([]byte(rendered), &data))
						files := make(map[string]cloudInitWriteFile)
						for _, file := range data.WriteFiles {
							files[file.Path] = file
						}
						for path, artifact := range map[string]string{
							"/opt/azure/containers/containerd-mount-guard.sh":   containerdMountGuardScript,
							"/opt/azure/containers/containerd-mount-guard.conf": containerdMountGuardDropin,
						} {
							if scriptless {
								require.NotContains(t, files, path, "scriptless mode uses the baked guard")
								continue
							}
							file, ok := files[path]
							require.True(t, ok, "missing guard artifact %s", path)
							require.Empty(t, file.Encoding)
							require.Equal(t, getCustomScript(artifact, config), file.Content)
							require.Equal(t, "root", file.Owner)
						}
						if !scriptless {
							require.Equal(t, "0744", files["/opt/azure/containers/containerd-mount-guard.sh"].Permissions)
							require.Equal(t, "0644", files["/opt/azure/containers/containerd-mount-guard.conf"].Permissions)
						}
						require.NotContains(t, files, "/etc/systemd/system/containerd.service.d/10-mount-guard.conf",
							"the shared containerd configuration path activates the guard only for temporary roots")
					}
				})
			}
		})
	}
}
