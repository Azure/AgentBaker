package main

import (
	"encoding/json"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func syntheticLinuxFixture() ([]byte, error) {
	linuxProfile := &datamodel.LinuxProfile{AdminUsername: "syntheticuser"}
	linuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{KeyData: "ssh-rsa SYNTHETIC-LOAD-TEST-KEY"}}
	agentPool := &datamodel.AgentPoolProfile{
		Name:                "syntheticpool",
		VMSize:              "Standard_DS2_v2",
		StorageProfile:      "ManagedDisks",
		OSType:              datamodel.Linux,
		VnetSubnetID:        "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/synthetic/providers/Microsoft.Network/virtualNetworks/synthetic/subnets/nodes",
		AvailabilityProfile: datamodel.VirtualMachineScaleSets,
		Distro:              datamodel.AKSUbuntuContainerd2204Gen2,
	}
	containerService := &datamodel.ContainerService{
		Location: "southcentralus",
		Type:     "Microsoft.ContainerService/ManagedClusters",
		Properties: &datamodel.Properties{
			OrchestratorProfile: &datamodel.OrchestratorProfile{
				OrchestratorType:    datamodel.Kubernetes,
				OrchestratorVersion: "1.32.1",
				KubernetesConfig:    &datamodel.KubernetesConfig{},
			},
			HostedMasterProfile: &datamodel.HostedMasterProfile{DNSPrefix: "synthetic"},
			AgentPoolProfiles:   []*datamodel.AgentPoolProfile{agentPool},
			LinuxProfile:        linuxProfile,
			ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
				ClientID: "00000000-0000-0000-0000-000000000000",
				Secret:   "synthetic-not-a-secret",
			},
		},
	}
	galleries := map[string]datamodel.SIGGalleryConfig{
		"AKSUbuntu":         {GalleryName: "aksubuntu", ResourceGroup: "synthetic"},
		"AKSCBLMariner":     {GalleryName: "akscblmariner", ResourceGroup: "synthetic"},
		"AKSAzureLinux":     {GalleryName: "aksazurelinux", ResourceGroup: "synthetic"},
		"AKSWindows":        {GalleryName: "akswindows", ResourceGroup: "synthetic"},
		"AKSFlatcar":        {GalleryName: "aksflatcar", ResourceGroup: "synthetic"},
		"AKSUbuntuEdgeZone": {GalleryName: "aksubuntuedgezone", ResourceGroup: "synthetic"},
	}
	configuration := &datamodel.NodeBootstrappingConfiguration{
		ContainerService:             containerService,
		CloudSpecConfig:              datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:                &datamodel.K8sComponents{},
		AgentPoolProfile:             agentPool,
		TenantID:                     "00000000-0000-0000-0000-000000000000",
		SubscriptionID:               "00000000-0000-0000-0000-000000000000",
		ResourceGroupName:            "synthetic",
		UserAssignedIdentityClientID: "00000000-0000-0000-0000-000000000000",
		ConfigGPUDriverIfNeeded:      true,
		KubeletConfig: map[string]string{
			"--address":                           "0.0.0.0",
			"--anonymous-auth":                    "false",
			"--authorization-mode":                "Webhook",
			"--authentication-token-webhook":      "true",
			"--client-ca-file":                    "/etc/kubernetes/certs/ca.crt",
			"--cloud-config":                      "/etc/kubernetes/azure.json",
			"--cloud-provider":                    "azure",
			"--cluster-dns":                       "10.0.0.10",
			"--cluster-domain":                    "cluster.local",
			"--cgroups-per-qos":                   "true",
			"--event-qps":                         "0",
			"--feature-gates":                     "RotateKubeletServerCertificate=true,PodPriority=true",
			"--image-gc-high-threshold":           "85",
			"--image-gc-low-threshold":            "80",
			"--max-pods":                          "110",
			"--node-status-update-frequency":      "10s",
			"--pod-manifest-path":                 "/etc/kubernetes/manifests",
			"--protect-kernel-defaults":           "true",
			"--read-only-port":                    "10255",
			"--resolv-conf":                       "/etc/resolv.conf",
			"--rotate-certificates":               "true",
			"--streaming-connection-idle-timeout": "4h0m0s",
		},
		PrimaryScaleSetName: "aks-synthetic-vmss",
		SIGConfig: datamodel.SIGConfig{
			TenantID:       "00000000-0000-0000-0000-000000000000",
			SubscriptionID: "00000000-0000-0000-0000-000000000000",
			Galleries:      galleries,
		},
	}
	return json.Marshal(configuration)
}
