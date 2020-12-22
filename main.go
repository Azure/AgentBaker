// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package main

import (
	"fmt"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	// "os"
	// "github.com/Azure/agentbaker/cmd"
	// colorable "github.com/mattn/go-colorable"
	// log "github.com/sirupsen/logrus"
)

func main() {
	// /Users/andy/workspace/gopath/src//baker.go
	// generator := agent.InitializeTemplateGenerator()
	// generator.GetNodeBootstrappingPayload(&datamodel.NodeBootstrappingConfiguration{
	// 	AgentPoolProfile: &datamodel.AgentPoolProfile{
	// 		OSType: datamodel.Linux,
	// 	},
	// })

	cloudInitFiles := agent.GetCustomDataVariables(&datamodel.NodeBootstrappingConfiguration{
		ContainerService: &datamodel.ContainerService{
			Properties: &datamodel.Properties{
				// OrchestratorProfile: &datamodel.OrchestratorProfile{
				// KubernetesConfig: &datamodel.KubernetesConfig{
				// 	ContainerRuntime: "containerd",
				// },
				// },
			},
		},
	})

	xx := (cloudInitFiles["cloudInitData"])
	yy := (xx).(agent.ParamsMap)
	fmt.Printf("%s", yy["provisionInstalls"].(string))
	// log.SetFormatter(&log.TextFormatter{ForceColors: true})
	// log.SetOutput(colorable.NewColorableStdout())
	// if err := cmd.NewRootCmd().Execute(); err != nil {
	// 	os.Exit(1)
	// }
}
