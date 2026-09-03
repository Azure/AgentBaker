// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

type platform struct {
	name   string
	distro datamodel.Distro
}

func main() {
	platforms := []platform{
		{name: "ubuntu", distro: datamodel.AKSUbuntuContainerd2204Gen2},
		{name: "mariner", distro: datamodel.AKSAzureLinuxV3Gen2},
		{name: "acl", distro: datamodel.AKSACLGen2TL},
		{name: "azlosguard", distro: datamodel.AKSAzureLinuxV3OSGuardGen2FIPSTL},
		{name: "flatcar", distro: datamodel.AKSFlatcarGen2},
	}

	templatePath := flag.String("template", "", "path to the hotfix nodecustomdata template")
	outputDir := flag.String("output-dir", "", "directory for rendered nodecustomdata files")
	flag.Parse()

	if *templatePath == "" || *outputDir == "" {
		fmt.Fprintln(os.Stderr, "--template and --output-dir are required")
		os.Exit(2)
	}

	templateContent, err := os.ReadFile(*templatePath)
	if err != nil {
		fatalf("read template: %v", err)
	}
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fatalf("create output directory: %v", err)
	}

	for _, target := range platforms {
		rendered, err := agent.RenderLinuxNodeCustomDataTemplate(
			templateContent,
			newRenderConfig(target.distro),
		)
		if err != nil {
			fatalf("render %s nodecustomdata: %v", target.name, err)
		}
		outputPath := filepath.Join(
			*outputDir,
			"rendered_nodecustomdata_"+target.name+".yml",
		)
		if err := os.WriteFile(outputPath, []byte(rendered), 0o600); err != nil {
			fatalf("write %s nodecustomdata: %v", target.name, err)
		}
	}
}

// newRenderConfig builds the minimal NodeBootstrappingConfiguration required to
// run agent.RenderLinuxNodeCustomDataTemplate. Only the Distro selector affects
// the hotfix write_files blocks we render; the remaining fields (orchestrator
// version, location, FQDN, KubernetesConfig, etc.) are placeholders whose sole
// purpose is to satisfy the renderer's unconditional dereferences of
// OrchestratorProfile/KubernetesConfig so rendering doesn't nil-panic. Their
// values are not meaningful and are not consumed by the embedded output.
func newRenderConfig(distro datamodel.Distro) *datamodel.NodeBootstrappingConfiguration {
	profile := &datamodel.AgentPoolProfile{
		Name:   "hotfix-render",
		OSType: datamodel.Linux,
		Distro: distro,
	}
	return &datamodel.NodeBootstrappingConfiguration{
		ContainerService: &datamodel.ContainerService{
			Location: "eastus",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorVersion: "1.29.0",
					OrchestratorType:    datamodel.Kubernetes,
					KubernetesConfig: &datamodel.KubernetesConfig{
						ContainerRuntimeConfig: map[string]string{},
					},
				},
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					FQDN: "hotfix-render.invalid",
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{profile},
			},
		},
		AgentPoolProfile: profile,
		CloudSpecConfig:  datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:    &datamodel.K8sComponents{},
		KubeletConfig:    map[string]string{},
	}
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
