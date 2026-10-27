// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRenderSelectedHotfixScripts(t *testing.T) {
	const initCloudBlock = `{{if IsAKSCustomCloud}}
- path: {{GetInitAKSCloudFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "initAKSCloud"}}
{{end}}
`
	const helperBlock = `- path: {{GetCSEHelpersScriptFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSource"}}
`
	for _, distro := range []datamodel.Distro{
		datamodel.AKSUbuntuContainerd2204Gen2,
		datamodel.AKSAzureLinuxV3Gen2,
	} {
		for _, selected := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/initCloudSelected=%t", distro, selected), func(t *testing.T) {
				template := "#cloud-config\nwrite_files:\n" + helperBlock
				if selected {
					template += initCloudBlock
				}
				rendered, err := agent.RenderLinuxNodeCustomDataTemplate(
					[]byte(template), newRenderConfig(distro),
				)
				require.NoError(t, err)
				var payload struct {
					WriteFiles []struct {
						Path    string `yaml:"path"`
						Content string `yaml:"content"`
					} `yaml:"write_files"`
				}
				require.NoError(t, yaml.Unmarshal([]byte(rendered), &payload))
				require.NotEmpty(t, payload.WriteFiles)
				require.Equal(t, 1, strings.Count(rendered, "- path: /opt/azure/containers/provision_source.sh"))
				wantInitCount := 0
				if selected {
					wantInitCount = 1
				}
				require.Equal(t, wantInitCount, strings.Count(rendered, "- path: /opt/azure/containers/init-aks-cloud.sh"))
				require.Len(t, payload.WriteFiles, 1+wantInitCount)
				for _, file := range payload.WriteFiles {
					require.NotEmpty(t, file.Content)
				}
			})
		}
	}
}
