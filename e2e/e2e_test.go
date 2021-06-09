package e2e

import (
	"fmt"
	"testing"
	"os"
	"io/ioutil"
	"encoding/json"
	"encoding/base64"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
)

type customDataFields struct {
	Cloud									string		`json:"cloud"`
	TenantId								string		`json:"tenantId"`
	SubscriptionId							string		`json:"subscriptionId"`
	AadClientId								string		`json:"aadClientId"`
	AadClientSecret							string		`json:"aadClientSecret"`
	ResourceGroup							string 		`json:"resourceGroup"`
	Location								string 		`json:"location"`
	VmType									string 		`json:"vmType"`
	SubnetName								string 		`json:"subnetName"`
	SecurityGroupName						string 		`json:"securityGroupName"`
	VnetName								string 		`json:"vnetName"`
	VnetResourceGroup						string 		`json:"vnetResourceGroup"`
	RouteTableName							string 		`json:"routeTableName"`
	PrimaryAvailabilitySetName				string 		`json:"primaryAvailabilitySetName"`
	PrimaryScaleSetName						string 		`json:"primaryScaleSetName"`
	CloudProviderBackoffMode				string 		`json:"cloudProviderBackoffMode"`
	CloudProviderBackoff					bool 		`json:"cloudProviderBackoff"`
	CloudProviderBackoffRetries				int 		`json:"cloudProviderBackoffRetries"`
	CloudProviderBackoffDuration			int 		`json:"cloudProviderBackoffDuration"`
	CloudProviderRateLimit					bool 		`json:"cloudProviderRateLimit"`
	CloudProviderRateLimitQPS				float64 	`json:"cloudProviderRateLimitQPS"`
	CloudProviderRateLimitBucket			int 		`json:"cloudProviderRateLimitBucket"`
	CloudProviderRateLimitQPSWrite			float64 	`json:"cloudProviderRateLimitQPSWrite"`
	CloudProviderRateLimitBucketWrite		int 		`json:"cloudProviderRateLimitBucketWrite"`
	UseManagedIdentityExtension				bool 		`json:"useManagedIdentityExtension"`
	UserAssignedIdentityID					string 		`json:"userAssignedIdentityID"`
	UseInstanceMetadata						bool 		`json:"useInstanceMetadata"`
	LoadBalancerSku							string 		`json:"loadBalancerSku"`
	DisableOutboundSNAT						bool 		`json:"disableOutboundSNAT"`
	ExcludeMasterFromStandardLB				bool 		`json:"excludeMasterFromStandardLB"`
	ProviderVaultName						string 		`json:"providerVaultName"`
	MaximumLoadBalancerRuleCount			int 		`json:"maximumLoadBalancerRuleCount"`
	ProviderKeyName							string 		`json:"providerKeyName"`
	ProviderKeyVersion						string 		`json:"providerKeyVersion"`
	Apiservercert							string 		`json:"apiserver.crt"`
	Cacert 									string 		`json:"ca.crt"`
	Clientkey   							string  	`json:"client.key"`
	Clientcert								string 		`json:"client.crt"`
	Fqdn									string		`json:"fqdn"`
	Mode									string 		`json:"mode"`
	NodePoolName							string		`json:"nodepoolname"`
	NodeImageVersion						string 		`json:"nodeImageVersion"`
	TenantID								string 		`json:"tenantID"`
	MCRGName								string 		`json:"mcRGName"`
	ClusterID								string		`json:"clusterID"`
	SubID									string		`json:"subID"`	
	TLSBootstrapToken						string		`json:"tlsbootstraptoken"`
}

func createFile(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			fmt.Println(err)
		}

		defer f.Close()
	}
}

func TestBasic(t *testing.T) {
	entry := "Generating CustomData and cseCmd"
	fmt.Println(entry)

	fields, err := os.Open("fields.json")
	if err != nil {
		fmt.Println(err)
	}

	defer fields.Close()
	fieldsByteValue, _ := ioutil.ReadAll(fields)

	values := customDataFields{}
	json.Unmarshal([]byte(fieldsByteValue), &values)

	createFile("../e2e/cloud-init.txt")
	createFile("../e2e/cseCmd")

	caCertDecoded, _ := base64.URLEncoding.DecodeString(values.Cacert)
	apiServerCertDecoded, _ := base64.URLEncoding.DecodeString(values.Apiservercert)
	clientKeyDecoded, _ := base64.URLEncoding.DecodeString(values.Clientkey)
	clientCertDecoded, _ := base64.URLEncoding.DecodeString(values.Clientcert)
	
	cs := &datamodel.ContainerService{
		Location: values.Location,
		Type:     "Microsoft.ContainerService/ManagedClusters",
		Properties: &datamodel.Properties{
			ClusterID:	values.ClusterID,
			OrchestratorProfile: &datamodel.OrchestratorProfile{
				OrchestratorType:    datamodel.Kubernetes,
				OrchestratorVersion: "1.19.9",
				KubernetesConfig: &datamodel.KubernetesConfig{
					NetworkPlugin: "kubenet",
					LoadBalancerSku: "Standard",
					UseInstanceMetadata: to.BoolPtr(values.UseInstanceMetadata),
					CloudProviderBackoff: to.BoolPtr(values.CloudProviderBackoff),
					CloudProviderBackoffMode: values.CloudProviderBackoffMode,
					CloudProviderBackoffRetries: values.CloudProviderBackoffRetries,
					CloudProviderBackoffExponent: 0,
					CloudProviderBackoffDuration: values.CloudProviderBackoffDuration,
					CloudProviderBackoffJitter: 0,
					CloudProviderRateLimit: to.BoolPtr(values.CloudProviderRateLimit),
					CloudProviderRateLimitQPS: values.CloudProviderRateLimitQPS,
					CloudProviderRateLimitQPSWrite: values.CloudProviderRateLimitQPSWrite,
					CloudProviderRateLimitBucket: values.CloudProviderRateLimitBucket,
					CloudProviderRateLimitBucketWrite: values.CloudProviderRateLimitBucketWrite,
					CloudProviderDisableOutboundSNAT: to.BoolPtr(values.DisableOutboundSNAT),
					MaximumLoadBalancerRuleCount: values.MaximumLoadBalancerRuleCount,
					CustomKubeBinaryURL: "https://acs-mirror.azureedge.net/kubernetes/v1.19.9-hotfix.20210322/binaries/kubernetes-node-linux-amd64.tar.gz",
					CustomKubeProxyImage: "mcr.microsoft.com/oss/kubernetes/kube-proxy:v1.19.9-hotfix.20210322.1",
					AzureCNIURLLinux: "https://acs-mirror.azureedge.net/azure-cni/v1.1.8/binaries/azure-vnet-cni-linux-amd64-v1.1.8.tgz",
					// KubeletConfig: map[string]string{
					// 	"--feature-gates": "RotateKubeletServerCertificate=true,a=b, PodPriority=true, x=y",
					// },
				},
			},
			HostedMasterProfile: &datamodel.HostedMasterProfile{
				FQDN: values.Fqdn,
				IPMasqAgent: true,
			},
			AgentPoolProfiles: []*datamodel.AgentPoolProfile{
				{
					Name:                values.NodePoolName,
					Count:               1,
					VMSize:              "Standard_DS1_v2",
					StorageProfile:      "ManagedDisks",
					OSType:              datamodel.Linux,
					//VnetSubnetID:        "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet1",
					AvailabilityProfile: datamodel.VirtualMachineScaleSets,
					CustomNodeLabels: map[string]string {
															"kubernetes.azure.com/node-image-version": values.NodeImageVersion, 
															"kubernetes.azure.com/mode": "system", //values.Mode,
														},
					KubernetesConfig: &datamodel.KubernetesConfig{
						ContainerRuntime: datamodel.Containerd,
						KubeletConfig: map[string]string{
							"--address": "0.0.0.0",
							"--anonymous-auth": "false",
							"--authentication-token-webhook": "true",
							"--authorization-mode": "Webhook",
							"--azure-container-registry-config": "/etc/kubernetes/azure.json",
							"--cgroups-per-qos": "true",
							"--client-ca-file": "/etc/kubernetes/certs/ca.crt",
							"--cloud-config": "/etc/kubernetes/azure.json",
							"--cloud-provider": "azure",
							"--cluster-dns": "10.0.0.10",
							"--cluster-domain": "cluster.local",
							"--dynamic-config-dir": "/var/lib/kubelet",
							"--enforce-node-allocatable": "pods",
							"--event-qps": "0",
							"--eviction-hard": "memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%",
							"--feature-gates": "RotateKubeletServerCertificate=true",
							"--image-gc-high-threshold": "85",
							"--image-gc-low-threshold": "80",
							"--image-pull-progress-deadline": "30m",
							"--keep-terminated-pod-volumes": "false",
							"--kube-reserved": "cpu=100m,memory=1638Mi",
							"--kubeconfig": "/var/lib/kubelet/kubeconfig",
							"--max-pods": "110",
							"--network-plugin": "kubenet",
							"--node-status-update-frequency": "10s",
							"--non-masquerade-cidr": "0.0.0.0/0",
							"--pod-infra-container-image": "mcr.microsoft.com/oss/kubernetes/pause:3.5",
							"--pod-manifest-path": "/etc/kubernetes/manifests",
							"--pod-max-pids": "-1",
							"--protect-kernel-defaults": "true",
							"--read-only-port": "0",
							"--resolv-conf": "/run/systemd/resolve/resolv.conf",
							"--rotate-certificates": "false",
							"--streaming-connection-idle-timeout": "4h",
							"--tls-cert-file": "/etc/kubernetes/certs/kubeletserver.crt",
							"--tls-cipher-suites": "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256",
							"--tls-private-key-file": "/etc/kubernetes/certs/kubeletserver.key",
						},
					},
					Distro: datamodel.AKSUbuntuContainerd1804Gen2,
				},
			},
			LinuxProfile: &datamodel.LinuxProfile{
				AdminUsername: "azureuser",
			},
			ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
				ClientID: values.AadClientId,
				Secret:   values.AadClientSecret,
			},

			CertificateProfile: &datamodel.CertificateProfile{
				CaCertificate: string(caCertDecoded),
				ClientCertificate: string(clientCertDecoded),
				APIServerCertificate: string(apiServerCertDecoded),
				ClientPrivateKey: string(clientKeyDecoded),
			},
		},
	}
	cs.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{
		KeyData: string("testsshkey"),
	}}

	// AKS always pass in te customHyperKubeImage to aks-e, so we don't really rely on
	// the default component version for "hyperkube", which is not set since 1.17
	// if IsKubernetesVersionGe(k8sVersion, "1.17.0") {
	// 	cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage = fmt.Sprintf("k8s.gcr.io/hyperkube-amd64:v%v", k8sVersion)
	// }

	agentPool := cs.Properties.AgentPoolProfiles[0]
	baker := agent.InitializeTemplateGenerator()

	// fullK8sComponentsMap := K8sComponentsByVersionMap[cs.Properties.OrchestratorProfile.OrchestratorVersion]
	// // pauseImage := cs.Properties.OrchestratorProfile.KubernetesConfig.MCRKubernetesImageBase + fullK8sComponentsMap["pause"]

	// hyperkubeImageBase := cs.Properties.OrchestratorProfile.KubernetesConfig.KubernetesImageBase
	// hyperkubeImage := hyperkubeImageBase + fullK8sComponentsMap["hyperkube"]
	// if cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage != "" {
	// 	hyperkubeImage = cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage
	// }

	// windowsPackage := datamodel.AzurePublicCloudSpecForTest.KubernetesSpecConfig.KubeBinariesSASURLBase + fullK8sComponentsMap["windowszip"]
	pauseImage := "mcr.microsoft.com/oss/kubernetes/pause:3.5"
	hyperkubeImage := "mcr.microsoft.com/oss/kubernetes/"
	windowsPackage := "windowspackage"

	k8sComponents := &datamodel.K8sComponents{
		PodInfraContainerImageURL: pauseImage,
		HyperkubeImageURL:         hyperkubeImage,
		WindowsPackageURL:         windowsPackage,
	}

	config := &datamodel.NodeBootstrappingConfiguration{
		ContainerService:              	cs,
		CloudSpecConfig:              	datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:                 	k8sComponents,
		AgentPoolProfile:              	agentPool,
		TenantID:                      	values.TenantID,
		SubscriptionID:                	values.SubID,
		ResourceGroupName:             	values.MCRGName,
		UserAssignedIdentityClientID:  	values.UserAssignedIdentityID,
		ConfigGPUDriverIfNeeded:       	true,
		EnableGPUDevicePluginIfNeeded: 	false,
		EnableKubeletConfigFile:       	false,
		EnableNvidia:                  	false,
		FIPSEnabled:                   	false,
		KubeletClientTLSBootstrapToken:	to.StringPtr(values.TLSBootstrapToken),
	}

	// customData
	base64EncodedCustomData := baker.GetNodeBootstrappingPayload(config)
	customDataBytes, _ := base64.StdEncoding.DecodeString(base64EncodedCustomData)
	customData := string(customDataBytes)
	//customData := string(base64EncodedCustomData)
	err = ioutil.WriteFile("cloud-init.txt", []byte(customData), 0644)
	if err != nil {
		fmt.Println("couldnt write to file", err)
	}
	// CSE
	cseCommand := baker.GetNodeBootstrappingCmd(config)
	err = ioutil.WriteFile("csecmd", []byte(cseCommand), 0644)
	if err != nil {
		fmt.Println("couldnt write to file", err)
	}
}