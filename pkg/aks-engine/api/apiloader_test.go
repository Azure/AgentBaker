// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package api

import (
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/aks-engine/pkg/api/vlabs"
	"github.com/Azure/aks-engine/pkg/i18n"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/leonelquinteros/gotext"
)

const exampleCustomHyperkubeImage = `example.azurecr.io/example/hyperkube-amd64:custom`
const examplePrivateAzureRegistryServer = `example.azurecr.io`

const exampleAPIModel = `{
	"apiVersion": "vlabs",
	"properties": {
		"orchestratorProfile": {
			"orchestratorType": "Kubernetes",
			"kubernetesConfig": {
				"customHyperkubeImage": "` + exampleCustomHyperkubeImage + `"
			}
		},
		"masterProfile": { "count": 1, "dnsPrefix": "", "vmSize": "Standard_D2_v2" },
		"agentPoolProfiles": [ { "name": "linuxpool1", "count": 2, "vmSize": "Standard_D2_v2", "availabilityProfile": "AvailabilitySet" } ],
		"windowsProfile": { "adminUsername": "azureuser", "adminPassword": "replacepassword1234$" },
		"linuxProfile": { "adminUsername": "azureuser", "ssh": { "publicKeys": [ { "keyData": "" } ] }
		},
		"servicePrincipalProfile": { "clientId": "", "secret": "" }
	}
}
`

func TestLoadContainerServiceFromFile(t *testing.T) {
	locale := gotext.NewLocale(path.Join("..", "..", "translations"), "en_US")
	i18n.Initialize(locale)
	apiloader := &Apiloader{
		Translator: &i18n.Translator{
			Locale: locale,
		},
	}

	_, _, err := apiloader.LoadContainerServiceFromFile("./testdata/simple/kubernetes.json")
	if err != nil {
		t.Error(err.Error())
	}

	// Test error scenario
	_, _, err = apiloader.LoadContainerServiceFromFile("../this-file-doesnt-exist.json")
	if err == nil {
		t.Errorf("expected error passing a non-existent filepath string to apiloader.LoadContainerServiceFromFile(), instead got nil")
	}
}

func TestLoadContainerServiceWithEmptyLocationPublicCloud(t *testing.T) {
	jsonWithoutlocationpubliccloud := `{
		"apiVersion": "vlabs",
		"properties": {
			"orchestratorProfile": {
				"orchestratorType": "Kubernetes",
				"kubernetesConfig": {
					"kubernetesImageBase": "msazurestackqa/",
					"useInstanceMetadata": false,
					"networkPolicy": "none"
				}
			},
			"masterProfile": {
				"dnsPrefix": "k111006",
				"distro": "ubuntu",
				"osDiskSizeGB": 200,
				"count": 3,
				"vmSize": "Standard_D2_v2"
			},
			"agentPoolProfiles": [
				{
					"name": "linuxpool",
					"osDiskSizeGB": 200,
					"count": 3,
					"vmSize": "Standard_D2_v2",
					"distro": "ubuntu",
					"availabilityProfile": "AvailabilitySet",
					"AcceleratedNetworkingEnabled": false
				}
			],
			"linuxProfile": {
				"adminUsername": "azureuser",
				"ssh": {
					"publicKeys": [
						{
							"keyData": "ssh-rsa PblicKey"
						}
					]
				}
			},
			"servicePrincipalProfile": {
				"clientId": "clientId",
				"secret": "secret"
			}
		}
	}`

	tmpFilewithoutlocationpubliccloud, err := ioutil.TempFile("", "containerService-nolocationpubliccloud")
	fileNamewithoutlocationpubliccloud := tmpFilewithoutlocationpubliccloud.Name()
	defer os.Remove(fileNamewithoutlocationpubliccloud)

	err = ioutil.WriteFile(fileNamewithoutlocationpubliccloud, []byte(jsonWithoutlocationpubliccloud), os.ModeAppend)

	apiloaderwithoutlocationpubliccloud := &Apiloader{}
	_, _, err = apiloaderwithoutlocationpubliccloud.LoadContainerServiceFromFile(fileNamewithoutlocationpubliccloud)
	if err != nil {
		t.Errorf("Expected no error for missing loation for public cloud to be thrown. %v", err)
	}
}

func TestDeserializeContainerService(t *testing.T) {
	apiloader := &Apiloader{
		Translator: nil,
	}

	// Test AKS Engine api model
	cs, version, err := apiloader.DeserializeContainerService([]byte(exampleAPIModel))
	if err != nil {
		t.Errorf("unexpected error deserializing the example apimodel: %s", err)
	}
	if version != vlabs.APIVersion {
		t.Errorf("expected apiVersion %s, instead got: %s", vlabs.APIVersion, version)
	}
	if cs.Properties.OrchestratorProfile.OrchestratorType != api.Kubernetes {
		t.Errorf("expected cs.Properties.OrchestratorProfile.OrchestratorType %s, instead got: %s", api.Kubernetes, cs.Properties.OrchestratorProfile.OrchestratorType)
	}

	// Test error case
	_, _, err = apiloader.DeserializeContainerService([]byte(`{thisisnotson}`))
	if err == nil {
		t.Errorf("expected error from malformed api model input")
	}
}

func TestSerializeContainerService(t *testing.T) {

	// Test with HostedMasterProfile and v20170831
	cs := getDefaultContainerService()

	cs.Properties.HostedMasterProfile = &datamodel.HostedMasterProfile{
		FQDN:        "blueorange.westus2.azure.com",
		DNSPrefix:   "blueorange",
		Subnet:      "sampleSubnet",
		IPMasqAgent: true,
	}
	apiloader := &Apiloader{
		Translator: &i18n.Translator{},
	}

	// Test with version vlabs
	b, err := apiloader.SerializeContainerService(cs, vlabs.APIVersion)
	if b == nil || err != nil {
		t.Errorf("unexpected error while trying to Serialize Container Service with version vlabs: %s", err.Error())
	}
}

func getDefaultContainerService() *datamodel.ContainerService {
	u, _ := url.Parse("http://foobar.com/search")
	return &datamodel.ContainerService{
		ID:       "sampleID",
		Location: "westus2",
		Name:     "sampleCS",
		Plan: &datamodel.ResourcePurchasePlan{
			Name:          "sampleRPP",
			Product:       "sampleProduct",
			PromotionCode: "sampleCode",
			Publisher:     "samplePublisher",
		},
		Tags: map[string]string{
			"foo": "bar",
		},
		Type: "sampleType",
		Properties: &datamodel.Properties{
			WindowsProfile: &datamodel.WindowsProfile{
				AdminUsername: "sampleAdminUsername",
				AdminPassword: "sampleAdminPassword",
			},
			DiagnosticsProfile: &datamodel.DiagnosticsProfile{
				VMDiagnostics: &datamodel.VMDiagnostics{
					Enabled:    true,
					StorageURL: u,
				},
			},
			LinuxProfile: &datamodel.LinuxProfile{
				AdminUsername: "azureuser",
				SSH: struct {
					PublicKeys []datamodel.PublicKey `json:"publicKeys"`
				}{
					PublicKeys: []datamodel.PublicKey{
						{
							KeyData: ValidSSHPublicKey,
						},
					},
				},
				Secrets: []datamodel.KeyVaultSecrets{
					{
						SourceVault: &datamodel.KeyVaultID{
							ID: "sampleKeyVaultID",
						},
						VaultCertificates: []datamodel.KeyVaultCertificate{
							{
								CertificateURL:   "FooCertURL",
								CertificateStore: "BarCertStore",
							},
						},
					},
				},
				CustomNodesDNS: &datamodel.CustomNodesDNS{
					DNSServer: "SampleDNSServer",
				},
				CustomSearchDomain: &datamodel.CustomSearchDomain{
					Name:          "FooCustomSearchDomain",
					RealmUser:     "sampleRealmUser",
					RealmPassword: "sampleRealmPassword",
				},
			},
			ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
				ClientID: "fooClientID",
				Secret:   "fooSecret",
				ObjectID: "fooObjectID",
				KeyvaultSecretRef: &datamodel.KeyvaultSecretRef{
					VaultID:       "fooVaultID",
					SecretName:    "fooSecretName",
					SecretVersion: "fooSecretVersion",
				},
			},
			ExtensionProfiles: []*datamodel.ExtensionProfile{
				{
					Name:                "fooExtension",
					Version:             "fooVersion",
					ExtensionParameters: "fooExtensionParameters",
					ExtensionParametersKeyVaultRef: &datamodel.KeyvaultSecretRef{
						VaultID:       "fooVaultID",
						SecretName:    "fooSecretName",
						SecretVersion: "fooSecretVersion",
					},
					RootURL:  "fooRootURL",
					Script:   "fooSsript",
					URLQuery: "fooURL",
				},
			},
			JumpboxProfile: &datamodel.JumpboxProfile{
				OSType:    "Linux",
				DNSPrefix: "blueorange",
				FQDN:      "blueorange.westus2.com",
			},
			CertificateProfile: &datamodel.CertificateProfile{
				CaCertificate:         "SampleCACert",
				CaPrivateKey:          "SampleCAPrivateKey",
				APIServerCertificate:  "SampleAPIServerCert",
				APIServerPrivateKey:   "SampleAPIServerPrivateKey",
				ClientCertificate:     "SampleClientCert",
				ClientPrivateKey:      "SampleClientPrivateKey",
				KubeConfigCertificate: "SampleKubeConfigCert",
				KubeConfigPrivateKey:  "SampleKubeConfigPrivateKey",
				EtcdClientCertificate: "SampleEtcdClientCert",
				EtcdClientPrivateKey:  "SampleEtcdClientPrivateKey",
				EtcdServerCertificate: "SampleEtcdServerCert",
				EtcdServerPrivateKey:  "SampleEtcdServerPrivateKey",
			},
			FeatureFlags: &datamodel.FeatureFlags{
				EnableCSERunInBackground: true,
				BlockOutboundInternet:    false,
				EnableTelemetry:          false,
			},
			AADProfile: &datamodel.AADProfile{
				ClientAppID:     "SampleClientAppID",
				ServerAppID:     "ServerAppID",
				ServerAppSecret: "ServerAppSecret",
				TenantID:        "SampleTenantID",
				AdminGroupID:    "SampleAdminGroupID",
				Authenticator:   datamodel.Webhook,
			},
			CustomProfile: &datamodel.CustomProfile{
				Orchestrator: "Kubernetes",
			},
			OrchestratorProfile: &datamodel.OrchestratorProfile{
				OrchestratorType:    "Kubernetes",
				OrchestratorVersion: "1.11.6",
				DcosConfig: &api.DcosConfig{
					DcosBootstrapURL:         "SampleDcosBootstrapURL",
					DcosWindowsBootstrapURL:  "SampleWindowsDcosBootstrapURL",
					Registry:                 "SampleRegistry",
					RegistryPass:             "SampleRegistryPass",
					RegistryUser:             "SampleRegistryUser",
					DcosClusterPackageListID: "SampleDcosClusterPackageListID",
					DcosProviderPackageID:    "SampleDcosProviderPackageID",
					BootstrapProfile: &api.BootstrapProfile{
						VMSize:       "Standard_Ds1_v1",
						OSDiskSizeGB: 256,
						OAuthEnabled: true,
						StaticIP:     "172.0.0.1",
						Subnet:       "255.255.255.0",
					},
				},
				KubernetesConfig: &api.KubernetesConfig{},
			},
			MasterProfile: &datamodel.MasterProfile{
				Count:     1,
				DNSPrefix: "blueorange",
				SubjectAltNames: []string{
					"fooSubjectAltName",
				},
				CustomFiles: &[]api.CustomFile{
					{
						Source: "sampleCustomFileSource",
						Dest:   "sampleCustomFileDest",
					},
				},
				VMSize:                   "Standard_DS1_v1",
				OSDiskSizeGB:             256,
				VnetSubnetID:             "sampleVnetSubnetID",
				Subnet:                   "sampleSubnet",
				VnetCidr:                 "10.240.0.0/8",
				AgentVnetSubnetID:        "sampleAgentVnetSubnetID",
				FirstConsecutiveStaticIP: "10.240.0.0",
				IPAddressCount:           5,
				StorageProfile:           api.StorageAccount,
				HTTPSourceAddressPrefix:  "fooHTTPSourceAddressPrefix",
				OAuthEnabled:             true,
				PreprovisionExtension: &api.Extension{
					Name:        "sampleExtension",
					SingleOrAll: "single",
					Template:    "{{foobar}}",
				},
				Extensions: []api.Extension{
					{
						Name:        "sampleExtension",
						SingleOrAll: "single",
						Template:    "{{foobar}}",
					},
				},
				Distro: datamodel.Ubuntu,
				ImageRef: &datamodel.ImageReference{
					Name:          "FooImageRef",
					ResourceGroup: "FooImageRefResourceGroup",
				},
				KubernetesConfig: &api.KubernetesConfig{
					KubernetesImageBase: "quay.io",
					ClusterSubnet:       "fooClusterSubnet",
					NetworkPolicy:       "calico",
					NetworkPlugin:       "azure-cni",
					ContainerRuntime:    "docker",
					ContainerRuntimeConfig: map[string]string{
						common.ContainerDataDirKey: "/mnt/docker",
					},
					MaxPods:                         3,
					DockerBridgeSubnet:              "sampleDockerSubnet",
					DNSServiceIP:                    "172.0.0.1",
					ServiceCIDR:                     "172.0.0.1/16",
					UseManagedIdentity:              true,
					UserAssignedID:                  "fooUserAssigneID",
					UserAssignedClientID:            "fooUserAssigneClientID",
					MobyVersion:                     "3.0.0",
					CustomHyperkubeImage:            "",
					ContainerdVersion:               "1.2.4",
					CustomCcmImage:                  "sampleCCMImage",
					UseCloudControllerManager:       to.BoolPtr(true),
					CustomWindowsPackageURL:         "https://deisartifacts.windows.net",
					WindowsNodeBinariesURL:          "https://deisartifacts.windows.net",
					UseInstanceMetadata:             to.BoolPtr(true),
					ExcludeMasterFromStandardLB:     to.BoolPtr(false),
					EnableRbac:                      to.BoolPtr(true),
					EnableSecureKubelet:             to.BoolPtr(true),
					EnableAggregatedAPIs:            true,
					EnableDataEncryptionAtRest:      to.BoolPtr(true),
					EnablePodSecurityPolicy:         to.BoolPtr(true),
					EnableEncryptionWithExternalKms: to.BoolPtr(true),
					GCHighThreshold:                 85,
					GCLowThreshold:                  80,
					EtcdVersion:                     "3.0.0",
					EtcdDiskSizeGB:                  "256",
					EtcdEncryptionKey:               "sampleEncruptionKey",
					AzureCNIVersion:                 "1.1.2",
					AzureCNIURLLinux:                "https://mirror.azk8s.cn/kubernetes/azure-container-networking/linux",
					AzureCNIURLWindows:              "https://mirror.azk8s.cn/kubernetes/azure-container-networking/windows",
					KeyVaultSku:                     "Basic",
					MaximumLoadBalancerRuleCount:    3,
					ProxyMode:                       api.KubeProxyModeIPTables,
					PrivateAzureRegistryServer:      "sampleRegistryServerURL",
					KubeletConfig: map[string]string{
						"barKey": "bazValue",
					},
					Addons: []api.KubernetesAddon{
						{
							Name:    "sampleAddon",
							Enabled: to.BoolPtr(true),
							Containers: []api.KubernetesContainerSpec{
								{
									Name:           "sampleK8sContainer",
									Image:          "sampleK8sImage",
									MemoryRequests: "20Mi",
									CPURequests:    "10m",
								},
							},
							Config: map[string]string{
								"sampleKey": "sampleVal",
							},
						},
					},
					APIServerConfig: map[string]string{
						"sampleAPIServerKey": "sampleAPIServerVal",
					},
					ControllerManagerConfig: map[string]string{
						"sampleCMKey": "sampleCMVal",
					},
					CloudControllerManagerConfig: map[string]string{
						"sampleCCMKey": "sampleCCMVal",
					},
					SchedulerConfig: map[string]string{
						"sampleSchedulerKey": "sampleSchedulerVal",
					},
					PrivateCluster: &api.PrivateCluster{
						Enabled: to.BoolPtr(true),
						JumpboxProfile: &api.PrivateJumpboxProfile{
							Name:           "sampleJumpboxProfile",
							VMSize:         "Standard_DS1_v2",
							OSDiskSizeGB:   512,
							Username:       "userName",
							PublicKey:      ValidSSHPublicKey,
							StorageProfile: api.StorageAccount,
						},
					},
					PodSecurityPolicyConfig: map[string]string{
						"samplePSPConfigKey": "samplePSPConfigVal",
					},
				},
			},
			AgentPoolProfiles: []*datamodel.AgentPoolProfile{
				{
					Name:      "sampleAgent",
					Count:     2,
					VMSize:    "sampleVM",
					DNSPrefix: "blueorange",
					FQDN:      "blueorange.westus2.com",
					OSType:    "Linux",
					Subnet:    "sampleSubnet",
				},
				{
					Name:      "sampleAgent-public",
					Count:     2,
					VMSize:    "sampleVM",
					DNSPrefix: "blueorange",
					FQDN:      "blueorange.westus2.com",
					OSType:    "Linux",
					Subnet:    "sampleSubnet",
					ImageRef: &datamodel.ImageReference{
						Name:           "testImage",
						ResourceGroup:  "testRg",
						SubscriptionID: "testSub",
						Gallery:        "testGallery",
						Version:        "0.0.1",
					},
				},
			},
		},
	}
}

const ValidSSHPublicKey = "ssh-rsa AAAAB3NzaC1yc2EAAAABJQAAAQEApD8+lRvLtUcyfO8N2Cwq0zY9DG1Un9d+tcmU3HgnAzBr6UR/dDT5M07NV7DN1lmu/0dt6Ay/ItjF9xK//nwVJL3ezEX32yhLKkCKFMB1LcANNzlhT++SB5tlRBx65CTL8z9FORe4UCWVJNafxu3as/BshQSrSaYt3hjSeYuzTpwd4+4xQutzbTXEUBDUr01zEfjjzfUu0HDrg1IFae62hnLm3ajG6b432IIdUhFUmgjZDljUt5bI3OEz5IWPsNOOlVTuo6fqU8lJHClAtAlZEZkyv0VotidC7ZSCfV153rRsEk9IWscwL2PQIQnCw7YyEYEffDeLjBwkH6MIdJ6OgQ== rsa-key-20170510"
