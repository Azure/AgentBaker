// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/uuid"
)

// CreateMockContainerService returns a mock container service for testing purposes
func CreateMockContainerService(containerServiceName, orchestratorVersion string, masterCount, agentCount int, certs bool) *ContainerService {
	cs := ContainerService{}
	cs.ID = uuid.Must(uuid.NewRandom()).String()
	cs.Location = "eastus"
	cs.Name = containerServiceName

	cs.Properties = &Properties{}

	cs.Properties.MasterProfile = &MasterProfile{}
	cs.Properties.MasterProfile.Count = masterCount
	cs.Properties.MasterProfile.DNSPrefix = "testmaster"
	cs.Properties.MasterProfile.VMSize = "Standard_D2_v2"

	cs.Properties.AgentPoolProfiles = []*AgentPoolProfile{}
	agentPool := &AgentPoolProfile{}
	agentPool.Count = agentCount
	agentPool.Name = "agentpool1"
	agentPool.VMSize = "Standard_D2_v2"
	agentPool.OSType = Linux
	agentPool.AvailabilityProfile = "AvailabilitySet"
	agentPool.StorageProfile = "StorageAccount"

	cs.Properties.AgentPoolProfiles = append(cs.Properties.AgentPoolProfiles, agentPool)

	cs.Properties.LinuxProfile = &LinuxProfile{
		AdminUsername: "azureuser",
		SSH: struct {
			PublicKeys []PublicKey `json:"publicKeys"`
		}{},
	}

	cs.Properties.LinuxProfile.AdminUsername = "azureuser"
	cs.Properties.LinuxProfile.SSH.PublicKeys = append(
		cs.Properties.LinuxProfile.SSH.PublicKeys, PublicKey{KeyData: "test"})

	cs.Properties.ServicePrincipalProfile = &ServicePrincipalProfile{}
	cs.Properties.ServicePrincipalProfile.ClientID = "DEC923E3-1EF1-4745-9516-37906D56DEC4"
	cs.Properties.ServicePrincipalProfile.Secret = "DEC923E3-1EF1-4745-9516-37906D56DEC4"

	cs.Properties.OrchestratorProfile = &OrchestratorProfile{}
	cs.Properties.OrchestratorProfile.OrchestratorType = Kubernetes
	cs.Properties.OrchestratorProfile.OrchestratorVersion = orchestratorVersion
	cs.Properties.OrchestratorProfile.KubernetesConfig = &KubernetesConfig{
		EnableSecureKubelet:     to.BoolPtr(true),
		EnableRbac:              to.BoolPtr(true),
		EtcdDiskSizeGB:          DefaultEtcdDiskSize,
		ServiceCIDR:             DefaultKubernetesServiceCIDR,
		DockerBridgeSubnet:      DefaultDockerBridgeSubnet,
		DNSServiceIP:            DefaultKubernetesDNSServiceIP,
		GCLowThreshold:          DefaultKubernetesGCLowThreshold,
		GCHighThreshold:         DefaultKubernetesGCHighThreshold,
		MaxPods:                 DefaultKubernetesMaxPodsVNETIntegrated,
		ClusterSubnet:           DefaultKubernetesSubnet,
		ContainerRuntime:        DefaultContainerRuntime,
		NetworkPlugin:           "kubenet",
		NetworkPolicy:           DefaultNetworkPolicy,
		EtcdVersion:             DefaultEtcdVersion,
		MobyVersion:             DefaultMobyVersion,
		ContainerdVersion:       DefaultContainerdVersion,
		LoadBalancerSku:         "Basic",
		KubeletConfig:           make(map[string]string),
		ControllerManagerConfig: make(map[string]string),
	}

	cs.Properties.CertificateProfile = &CertificateProfile{}
	if certs {
		cs.Properties.CertificateProfile.CaCertificate = "cacert"
		cs.Properties.CertificateProfile.CaPrivateKey = "cakey"
		cs.Properties.CertificateProfile.KubeConfigCertificate = "kubeconfigcert"
		cs.Properties.CertificateProfile.KubeConfigPrivateKey = "kubeconfigkey"
		cs.Properties.CertificateProfile.APIServerCertificate = "apiservercert"
		cs.Properties.CertificateProfile.APIServerPrivateKey = "apiserverkey"
		cs.Properties.CertificateProfile.ClientCertificate = "clientcert"
		cs.Properties.CertificateProfile.ClientPrivateKey = "clientkey"
		cs.Properties.CertificateProfile.EtcdServerCertificate = "etcdservercert"
		cs.Properties.CertificateProfile.EtcdServerPrivateKey = "etcdserverkey"
		cs.Properties.CertificateProfile.EtcdClientCertificate = "etcdclientcert"
		cs.Properties.CertificateProfile.EtcdClientPrivateKey = "etcdclientkey"
		cs.Properties.CertificateProfile.EtcdPeerCertificates = []string{"etcdpeercert1", "etcdpeercert2", "etcdpeercert3", "etcdpeercert4", "etcdpeercert5"}
		cs.Properties.CertificateProfile.EtcdPeerPrivateKeys = []string{"etcdpeerkey1", "etcdpeerkey2", "etcdpeerkey3", "etcdpeerkey4", "etcdpeerkey5"}

	}

	return &cs
}

// GetK8sDefaultProperties returns a struct of type Properties for testing purposes.
func GetK8sDefaultProperties(hasWindows bool) *Properties {
	p := &Properties{
		OrchestratorProfile: &OrchestratorProfile{
			OrchestratorType: Kubernetes,
			KubernetesConfig: &KubernetesConfig{},
		},
		MasterProfile: &MasterProfile{
			Count:     1,
			DNSPrefix: "foo",
			VMSize:    "Standard_DS2_v2",
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				Name:                "agentpool",
				VMSize:              "Standard_D2_v2",
				Count:               1,
				AvailabilityProfile: AvailabilitySet,
			},
		},
		ServicePrincipalProfile: &ServicePrincipalProfile{
			ClientID: "clientID",
			Secret:   "clientSecret",
		},
	}

	if hasWindows {
		p.AgentPoolProfiles = []*AgentPoolProfile{
			{
				Name:                "agentpool",
				VMSize:              "Standard_D2_v2",
				Count:               1,
				AvailabilityProfile: AvailabilitySet,
				OSType:              Windows,
			},
		}
		p.WindowsProfile = &WindowsProfile{
			AdminUsername: "azureuser",
			AdminPassword: "replacepassword1234$",
		}
	}

	return p
}

func getMockAddon(name string) KubernetesAddon {
	return KubernetesAddon{
		Name: name,
		Containers: []KubernetesContainerSpec{
			{
				Name:           name,
				CPURequests:    "50m",
				MemoryRequests: "150Mi",
				CPULimits:      "50m",
				MemoryLimits:   "150Mi",
			},
		},
		Pools: []AddonNodePoolsConfig{
			{
				Name: "pool1",
				Config: map[string]string{
					"min-nodes": "3",
					"max-nodes": "3",
				},
			},
		},
	}
}

var (
	AzurePublicCloudSpecForTest = &AzureEnvironmentSpecConfig{
		CloudName: AzurePublicCloud,
		//DockerSpecConfig specify the docker engine download repo
		DockerSpecConfig: DockerSpecConfig{
			DockerEngineRepo:         "https://aptdocker.azureedge.net/repo",
			DockerComposeDownloadURL: "https://github.com/docker/compose/releases/download",
		},
		//KubernetesSpecConfig is the default kubernetes container image url.
		KubernetesSpecConfig: KubernetesSpecConfig{
			KubernetesImageBase:                  "k8s.gcr.io/",
			TillerImageBase:                      "gcr.io/kubernetes-helm/",
			ACIConnectorImageBase:                "microsoft/",
			NVIDIAImageBase:                      "nvidia/",
			CalicoImageBase:                      "calico/",
			AzureCNIImageBase:                    "mcr.microsoft.com/containernetworking/",
			MCRKubernetesImageBase:               "mcr.microsoft.com/",
			EtcdDownloadURLBase:                  "mcr.microsoft.com/oss/etcd-io/",
			KubeBinariesSASURLBase:               "https://acs-mirror.azureedge.net/kubernetes/",
			WindowsTelemetryGUID:                 "fb801154-36b9-41bc-89c2-f4d4f05472b0",
			CNIPluginsDownloadURL:                "https://acs-mirror.azureedge.net/cni/cni-plugins-amd64-" + CNIPluginVer + ".tgz",
			VnetCNILinuxPluginsDownloadURL:       "https://acs-mirror.azureedge.net/azure-cni/" + AzureCniPluginVerLinux + "/binaries/azure-vnet-cni-linux-amd64-" + AzureCniPluginVerLinux + ".tgz",
			VnetCNIWindowsPluginsDownloadURL:     "https://acs-mirror.azureedge.net/azure-cni/" + AzureCniPluginVerWindows + "/binaries/azure-vnet-cni-singletenancy-windows-amd64-" + AzureCniPluginVerWindows + ".zip",
			ContainerdDownloadURLBase:            "https://storage.googleapis.com/cri-containerd-release/",
			CSIProxyDownloadURL:                  "https://acs-mirror.azureedge.net/csi-proxy/v0.1.0/binaries/csi-proxy.tar.gz",
			WindowsProvisioningScriptsPackageURL: "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-" + DefaultWindowsProvisioningScriptsPackageVersion + ".zip",
			WindowsPauseImageURL:                 "mcr.microsoft.com/oss/kubernetes/pause:" + WindowsPauseImageVersion,
			AlwaysPullWindowsPauseImage:          DefaultAlwaysPullWindowsPauseImage,
		},

		EndpointConfig: AzureEndpointConfig{
			ResourceManagerVMDNSSuffix: "cloudapp.azure.com",
		},
	}
)
