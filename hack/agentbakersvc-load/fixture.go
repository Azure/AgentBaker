package main

import (
	"encoding/json"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	syntheticID   = "00000000-0000-0000-0000-000000000000"
	syntheticName = "synthetic"
)

func syntheticLinuxFixture() ([]byte, error) {
	linuxProfile := &datamodel.LinuxProfile{AdminUsername: "syntheticuser"}
	linuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{KeyData: "ssh-rsa SYNTHETIC-LOAD-TEST-KEY"}}
	agentPool := &datamodel.AgentPoolProfile{
		Name:                "syntheticpool",
		VMSize:              "Standard_DS2_v2",
		StorageProfile:      "ManagedDisks",
		OSType:              datamodel.Linux,
		VnetSubnetID:        "/subscriptions/" + syntheticID + "/resourceGroups/synthetic/providers/Microsoft.Network/virtualNetworks/synthetic/subnets/nodes",
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
			HostedMasterProfile: &datamodel.HostedMasterProfile{DNSPrefix: syntheticName},
			AgentPoolProfiles:   []*datamodel.AgentPoolProfile{agentPool},
			LinuxProfile:        linuxProfile,
			ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
				ClientID: syntheticID,
				Secret:   "synthetic-not-a-secret",
			},
		},
	}
	galleries := map[string]datamodel.SIGGalleryConfig{
		"AKSUbuntu":         {GalleryName: "aksubuntu", ResourceGroup: syntheticName},
		"AKSCBLMariner":     {GalleryName: "akscblmariner", ResourceGroup: syntheticName},
		"AKSAzureLinux":     {GalleryName: "aksazurelinux", ResourceGroup: syntheticName},
		"AKSWindows":        {GalleryName: "akswindows", ResourceGroup: syntheticName},
		"AKSFlatcar":        {GalleryName: "aksflatcar", ResourceGroup: syntheticName},
		"AKSUbuntuEdgeZone": {GalleryName: "aksubuntuedgezone", ResourceGroup: syntheticName},
	}
	configuration := &datamodel.NodeBootstrappingConfiguration{
		ContainerService:             containerService,
		CloudSpecConfig:              datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:                &datamodel.K8sComponents{},
		AgentPoolProfile:             agentPool,
		TenantID:                     syntheticID,
		SubscriptionID:               syntheticID,
		ResourceGroupName:            syntheticName,
		UserAssignedIdentityClientID: syntheticID,
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
