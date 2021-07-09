// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package api

import (
	"io/ioutil"
	"net/url"
	"os"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	// StorageAccount means that the nodes use raw storage accounts for their os and attached volumes
	StorageAccount = "StorageAccount"
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
		"hostedMasterProfile": { "dnsPrefix": "" },
		"agentPoolProfiles": [ { "name": "linuxpool1", "vmSize": "Standard_D2_v2", "availabilityProfile": "AvailabilitySet" } ],
		"windowsProfile": { "adminUsername": "azureuser", "adminPassword": "replacepassword1234$" },
		"linuxProfile": { "adminUsername": "azureuser", "ssh": { "publicKeys": [ { "keyData": "" } ] }
		},
		"servicePrincipalProfile": { "clientId": "", "secret": "" }
	}
}
`

func TestLoadContainerServiceFromFile(t *testing.T) {
	apiloader := &Apiloader{}

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
			"hostedMasterProfile": {
				"dnsPrefix": "k111006"
			},
			"agentPoolProfiles": [
				{
					"name": "linuxpool",
					"osDiskSizeGB": 200,
					"vmSize": "Standard_D2_v2",
					"distro": "ubuntu",
					"availabilityProfile": "AvailabilitySet"
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
	apiloader := &Apiloader{}

	// Test AKS Engine api model
	cs, version, err := apiloader.DeserializeContainerService([]byte(exampleAPIModel))
	if err != nil {
		t.Errorf("unexpected error deserializing the example apimodel: %s", err)
	}
	if version != VlabsAPIVersion {
		t.Errorf("expected apiVersion %s, instead got: %s", VlabsAPIVersion, version)
	}
	if cs.Properties.OrchestratorProfile.OrchestratorType != datamodel.Kubernetes {
		t.Errorf("expected cs.Properties.OrchestratorProfile.OrchestratorType %s, instead got: %s", datamodel.Kubernetes, cs.Properties.OrchestratorProfile.OrchestratorType)
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
	apiloader := &Apiloader{}

	// Test with version vlabs
	b, err := apiloader.SerializeContainerService(cs, VlabsAPIVersion)
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
			CertificateProfile: &datamodel.CertificateProfile{
				CaCertificate:         "SampleCACert",
				APIServerCertificate:  "SampleAPIServerCert",
				ClientCertificate:     "SampleClientCert",
				ClientPrivateKey:      "SampleClientPrivateKey",
				KubeConfigCertificate: "SampleKubeConfigCert",
				KubeConfigPrivateKey:  "SampleKubeConfigPrivateKey",
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
				KubernetesConfig:    &datamodel.KubernetesConfig{},
			},
			HostedMasterProfile: &datamodel.HostedMasterProfile{
				DNSPrefix: "blueorange",
				Subnet:    "sampleSubnet",
			},
			AgentPoolProfiles: []*datamodel.AgentPoolProfile{
				{
					Name:      "sampleAgent",
					VMSize:    "sampleVM",
					DNSPrefix: "blueorange",
					OSType:    "Linux",
					Subnet:    "sampleSubnet",
				},
				{
					Name:      "sampleAgent-public",
					VMSize:    "sampleVM",
					DNSPrefix: "blueorange",
					OSType:    "Linux",
					Subnet:    "sampleSubnet",
				},
			},
		},
	}
}

const ValidSSHPublicKey = "ssh-rsa AAAAB3NzaC1yc2EAAAABJQAAAQEApD8+lRvLtUcyfO8N2Cwq0zY9DG1Un9d+tcmU3HgnAzBr6UR/dDT5M07NV7DN1lmu/0dt6Ay/ItjF9xK//nwVJL3ezEX32yhLKkCKFMB1LcANNzlhT++SB5tlRBx65CTL8z9FORe4UCWVJNafxu3as/BshQSrSaYt3hjSeYuzTpwd4+4xQutzbTXEUBDUr01zEfjjzfUu0HDrg1IFae62hnLm3ajG6b432IIdUhFUmgjZDljUt5bI3OEz5IWPsNOOlVTuo6fqU8lJHClAtAlZEZkyv0VotidC7ZSCfV153rRsEk9IWscwL2PQIQnCw7YyEYEffDeLjBwkH6MIdJ6OgQ== rsa-key-20170510"

func TestLoadDefaultContainerServiceProperties(t *testing.T) {
	m, p := LoadDefaultContainerServiceProperties()

	if m.APIVersion != defaultAPIVersion {
		t.Errorf("Expected LoadDefaultContainerServiceProperties() to return API version %s, instead got %s", defaultAPIVersion, m.APIVersion)
	}

	if p.OrchestratorProfile.OrchestratorType != defaultOrchestrator {
		t.Errorf("Expected LoadDefaultContainerServiceProperties() to return %s OrchestratorProfile.OrchestratorType, instead got %s", datamodel.Kubernetes, p.OrchestratorProfile.OrchestratorType)
	}

	if len(p.AgentPoolProfiles) != 1 {
		t.Errorf("Expected 1 agent pool, instead got %d", len(p.AgentPoolProfiles))
	}

	if p.AgentPoolProfiles[0].Name != defaultAgentPoolName {
		t.Errorf("Expected LoadDefaultContainerServiceProperties() to return %s AgentPoolProfiles[0].Name, instead got %s", defaultAgentPoolName, p.AgentPoolProfiles[0].Name)
	}

	if p.AgentPoolProfiles[0].VMSize != defaultVMSize {
		t.Errorf("Expected LoadDefaultContainerServiceProperties() to return %s AgentPoolProfiles[0].VMSize, instead got %s", defaultVMSize, p.AgentPoolProfiles[0].VMSize)
	}

	if p.AgentPoolProfiles[0].OSDiskSizeGB != defaultOSDiskSizeGB {
		t.Errorf("Expected LoadDefaultContainerServiceProperties() to return %d AgentPoolProfiles[0].OSDiskSizeGB, instead got %d", defaultOSDiskSizeGB, p.AgentPoolProfiles[0].OSDiskSizeGB)
	}

	if p.LinuxProfile.AdminUsername != defaultAdminUser {
		t.Errorf("Expected LoadDefaultContainerServiceProperties() to return %s LinuxProfile.AdminAdminUsernameUsername, instead got %s", defaultAdminUser, p.LinuxProfile.AdminUsername)
	}
}
