// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"github.com/Azure/aks-engine/pkg/api"
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

	cs.Properties.MasterProfile = &api.MasterProfile{}
	cs.Properties.MasterProfile.Count = masterCount
	cs.Properties.MasterProfile.DNSPrefix = "testmaster"
	cs.Properties.MasterProfile.VMSize = "Standard_D2_v2"

	cs.Properties.AgentPoolProfiles = []*AgentPoolProfile{}
	agentPool := &AgentPoolProfile{}
	agentPool.Count = agentCount
	agentPool.Name = "agentpool1"
	agentPool.VMSize = "Standard_D2_v2"
	agentPool.OSType = api.Linux
	agentPool.AvailabilityProfile = "AvailabilitySet"
	agentPool.StorageProfile = "StorageAccount"

	cs.Properties.AgentPoolProfiles = append(cs.Properties.AgentPoolProfiles, agentPool)

	cs.Properties.LinuxProfile = &api.LinuxProfile{
		AdminUsername: "azureuser",
		SSH: struct {
			PublicKeys []api.PublicKey `json:"publicKeys"`
		}{},
	}

	cs.Properties.LinuxProfile.AdminUsername = "azureuser"
	cs.Properties.LinuxProfile.SSH.PublicKeys = append(
		cs.Properties.LinuxProfile.SSH.PublicKeys, api.PublicKey{KeyData: "test"})

	cs.Properties.ServicePrincipalProfile = &api.ServicePrincipalProfile{}
	cs.Properties.ServicePrincipalProfile.ClientID = "DEC923E3-1EF1-4745-9516-37906D56DEC4"
	cs.Properties.ServicePrincipalProfile.Secret = "DEC923E3-1EF1-4745-9516-37906D56DEC4"

	cs.Properties.OrchestratorProfile = &api.OrchestratorProfile{}
	cs.Properties.OrchestratorProfile.OrchestratorType = api.Kubernetes
	cs.Properties.OrchestratorProfile.OrchestratorVersion = orchestratorVersion
	cs.Properties.OrchestratorProfile.KubernetesConfig = &api.KubernetesConfig{
		EnableSecureKubelet:     to.BoolPtr(api.DefaultSecureKubeletEnabled),
		EnableRbac:              to.BoolPtr(api.DefaultRBACEnabled),
		EtcdDiskSizeGB:          api.DefaultEtcdDiskSize,
		ServiceCIDR:             api.DefaultKubernetesServiceCIDR,
		DockerBridgeSubnet:      api.DefaultDockerBridgeSubnet,
		DNSServiceIP:            api.DefaultKubernetesDNSServiceIP,
		GCLowThreshold:          api.DefaultKubernetesGCLowThreshold,
		GCHighThreshold:         api.DefaultKubernetesGCHighThreshold,
		MaxPods:                 api.DefaultKubernetesMaxPodsVNETIntegrated,
		ClusterSubnet:           api.DefaultKubernetesSubnet,
		ContainerRuntime:        api.DefaultContainerRuntime,
		NetworkPlugin:           api.DefaultNetworkPlugin,
		NetworkPolicy:           api.DefaultNetworkPolicy,
		EtcdVersion:             api.DefaultEtcdVersion,
		MobyVersion:             api.DefaultMobyVersion,
		ContainerdVersion:       api.DefaultContainerdVersion,
		LoadBalancerSku:         api.DefaultLoadBalancerSku,
		KubeletConfig:           make(map[string]string),
		ControllerManagerConfig: make(map[string]string),
	}

	cs.Properties.CertificateProfile = &api.CertificateProfile{}
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
		OrchestratorProfile: &api.OrchestratorProfile{
			OrchestratorType: api.Kubernetes,
			KubernetesConfig: &api.KubernetesConfig{},
		},
		MasterProfile: &api.MasterProfile{
			Count:     1,
			DNSPrefix: "foo",
			VMSize:    "Standard_DS2_v2",
		},
		AgentPoolProfiles: []*AgentPoolProfile{
			{
				Name:                "agentpool",
				VMSize:              "Standard_D2_v2",
				Count:               1,
				AvailabilityProfile: api.AvailabilitySet,
			},
		},
		ServicePrincipalProfile: &api.ServicePrincipalProfile{
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
				AvailabilityProfile: api.AvailabilitySet,
				OSType:              api.Windows,
			},
		}
		p.WindowsProfile = &api.WindowsProfile{
			AdminUsername: "azureuser",
			AdminPassword: "replacepassword1234$",
		}
	}

	return p
}

func getMockAddon(name string) api.KubernetesAddon {
	return api.KubernetesAddon{
		Name: name,
		Containers: []api.KubernetesContainerSpec{
			{
				Name:           name,
				CPURequests:    "50m",
				MemoryRequests: "150Mi",
				CPULimits:      "50m",
				MemoryLimits:   "150Mi",
			},
		},
		Pools: []api.AddonNodePoolsConfig{
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
