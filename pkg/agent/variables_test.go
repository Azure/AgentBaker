// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Windows CSE variables check", func() {
	var (
		config *datamodel.NodeBootstrappingConfiguration
	)

	BeforeEach(func() {
		config = getDefaultNBC()
	})

	It("sets tenantId", func() {
		config.TenantID = "test tenant id"

		vars := getWindowsCustomDataVariables(config)

		Expect(vars["tenantID"]).To(Equal("test tenant id"))
	})
})

func getDefaultNBC() *datamodel.NodeBootstrappingConfiguration {
	cs := &datamodel.ContainerService{
		Location: "southcentralus",
		Type:     "Microsoft.ContainerService/ManagedClusters",
		Properties: &datamodel.Properties{
			OrchestratorProfile: &datamodel.OrchestratorProfile{
				OrchestratorType:    datamodel.Kubernetes,
				OrchestratorVersion: "1.16.15",
				KubernetesConfig:    &datamodel.KubernetesConfig{},
			},
			HostedMasterProfile: &datamodel.HostedMasterProfile{
				DNSPrefix: "uttestdom",
			},
			AgentPoolProfiles: []*datamodel.AgentPoolProfile{
				{
					Name:                "agent2",
					VMSize:              "Standard_DS1_v2",
					StorageProfile:      "ManagedDisks",
					OSType:              datamodel.Linux,
					VnetSubnetID:        "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet1",
					AvailabilityProfile: datamodel.VirtualMachineScaleSets,
					Distro:              datamodel.AKSUbuntu1604,
				},
			},
			WindowsProfile: &datamodel.WindowsProfile{},
			LinuxProfile: &datamodel.LinuxProfile{
				AdminUsername: "azureuser",
			},
			ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
				ClientID: "ClientID",
				Secret:   "Secret",
			},
		},
	}
	cs.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{
		KeyData: string("testsshkey"),
	}}

	agentPool := cs.Properties.AgentPoolProfiles[0]

	k8sComponents := &datamodel.K8sComponents{}

	kubeletConfig := map[string]string{
		"--address":                           "0.0.0.0",
		"--pod-manifest-path":                 "/etc/kubernetes/manifests",
		"--cloud-provider":                    "azure",
		"--cloud-config":                      "/etc/kubernetes/azure.json",
		"--azure-container-registry-config":   "/etc/kubernetes/azure.json",
		"--cluster-domain":                    "cluster.local",
		"--cluster-dns":                       "10.0.0.10",
		"--cgroups-per-qos":                   "true",
		"--tls-cert-file":                     "/etc/kubernetes/certs/kubeletserver.crt",
		"--tls-private-key-file":              "/etc/kubernetes/certs/kubeletserver.key",
		"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint:lll
		"--max-pods":                          "110",
		"--node-status-update-frequency":      "10s",
		"--image-gc-high-threshold":           "85",
		"--image-gc-low-threshold":            "80",
		"--event-qps":                         "0",
		"--pod-max-pids":                      "-1",
		"--enforce-node-allocatable":          "pods",
		"--streaming-connection-idle-timeout": "4h0m0s",
		"--rotate-certificates":               "true",
		"--read-only-port":                    "10255",
		"--protect-kernel-defaults":           "true",
		"--resolv-conf":                       "/etc/resolv.conf",
		"--anonymous-auth":                    "false",
		"--client-ca-file":                    "/etc/kubernetes/certs/ca.crt",
		"--authentication-token-webhook":      "true",
		"--authorization-mode":                "Webhook",
		"--eviction-hard":                     "memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%",
		"--feature-gates":                     "RotateKubeletServerCertificate=true,a=b,PodPriority=true,x=y",
		"--system-reserved":                   "cpu=2,memory=1Gi",
		"--kube-reserved":                     "cpu=100m,memory=1638Mi",
	}

	galleries := map[string]datamodel.SIGGalleryConfig{
		"AKSUbuntu": {
			GalleryName:   "aksubuntu",
			ResourceGroup: "resourcegroup",
		},
		"AKSCBLMariner": {
			GalleryName:   "akscblmariner",
			ResourceGroup: "resourcegroup",
		},
		"AKSAzureLinux": {
			GalleryName:   "aksazurelinux",
			ResourceGroup: "resourcegroup",
		},
		"AKSWindows": {
			GalleryName:   "akswindows",
			ResourceGroup: "resourcegroup",
		},
		"AKSUbuntuEdgeZone": {
			GalleryName:   "AKSUbuntuEdgeZone",
			ResourceGroup: "AKS-Ubuntu-EdgeZone",
		},
	}
	sigConfig := &datamodel.SIGConfig{
		TenantID:       "sometenantid",
		SubscriptionID: "somesubid",
		Galleries:      galleries,
	}

	config := &datamodel.NodeBootstrappingConfiguration{
		ContainerService:              cs,
		CloudSpecConfig:               datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:                 k8sComponents,
		AgentPoolProfile:              agentPool,
		TenantID:                      "tenantID",
		SubscriptionID:                "subID",
		ResourceGroupName:             "resourceGroupName",
		UserAssignedIdentityClientID:  "userAssignedID",
		ConfigGPUDriverIfNeeded:       true,
		EnableGPUDevicePluginIfNeeded: false,
		EnableKubeletConfigFile:       false,
		EnableNvidia:                  false,
		FIPSEnabled:                   false,
		KubeletConfig:                 kubeletConfig,
		PrimaryScaleSetName:           "aks-agent2-36873793-vmss",
		SIGConfig:                     *sigConfig,
	}

	return config
}
