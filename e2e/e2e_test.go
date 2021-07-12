package e2e

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
)

// TODO 1: How to get the most accurate url links/image links for the currently hardcoded ones for eg CustomKubeBinaryURL, Pause Image etc
// TODO 2: Update --rotate-certificate (true for TLS enabled, false otherwise, small nit)
// TODO 3: Seperate out the certificate encode/decode
// TODO 4: Investigate CloudSpecConfig and its need. Without it, the bootstrapping struct breaks.

func createFile(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			fmt.Println(err)
		}

		defer f.Close()
	}
}

func TestE2EBasic(t *testing.T) {
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
			ClusterID: values.ClusterID,
			OrchestratorProfile: &datamodel.OrchestratorProfile{
				OrchestratorType:    datamodel.Kubernetes,
				OrchestratorVersion: "1.19.9",
				KubernetesConfig: &datamodel.KubernetesConfig{
					NetworkPlugin:                     "kubenet",
					LoadBalancerSku:                   "Standard",
					UseInstanceMetadata:               to.BoolPtr(values.UseInstanceMetadata),
					CloudProviderBackoff:              to.BoolPtr(values.CloudProviderBackoff),
					CloudProviderBackoffMode:          values.CloudProviderBackoffMode,
					CloudProviderBackoffRetries:       values.CloudProviderBackoffRetries,
					CloudProviderBackoffExponent:      0,
					CloudProviderBackoffDuration:      values.CloudProviderBackoffDuration,
					CloudProviderBackoffJitter:        0,
					CloudProviderRateLimit:            to.BoolPtr(values.CloudProviderRateLimit),
					CloudProviderRateLimitQPS:         values.CloudProviderRateLimitQPS,
					CloudProviderRateLimitQPSWrite:    values.CloudProviderRateLimitQPSWrite,
					CloudProviderRateLimitBucket:      values.CloudProviderRateLimitBucket,
					CloudProviderRateLimitBucketWrite: values.CloudProviderRateLimitBucketWrite,
					CloudProviderDisableOutboundSNAT:  to.BoolPtr(values.DisableOutboundSNAT),
					MaximumLoadBalancerRuleCount:      values.MaximumLoadBalancerRuleCount,
					CustomKubeBinaryURL:               "https://acs-mirror.azureedge.net/kubernetes/v1.19.9-hotfix.20210322/binaries/kubernetes-node-linux-amd64.tar.gz",
					CustomKubeProxyImage:              "mcr.microsoft.com/oss/kubernetes/kube-proxy:v1.19.9-hotfix.20210322.1",
					AzureCNIURLLinux:                  "https://acs-mirror.azureedge.net/azure-cni/v1.1.8/binaries/azure-vnet-cni-linux-amd64-v1.1.8.tgz",
				},
			},
			HostedMasterProfile: &datamodel.HostedMasterProfile{
				FQDN:        values.Fqdn,
				IPMasqAgent: true,
			},
			AgentPoolProfiles: []*datamodel.AgentPoolProfile{
				{
					Name:                values.Name,
					VMSize:              "Standard_DS1_v2",
					StorageProfile:      "ManagedDisks",
					OSType:              "Linux",
					AvailabilityProfile: datamodel.VirtualMachineScaleSets,
					CustomNodeLabels: map[string]string{
						"kubernetes.azure.com/node-image-version": values.NodeImageVersion,
						"kubernetes.azure.com/mode":               "system", //values.Mode,
					},
					KubernetesConfig: &datamodel.KubernetesConfig{
						ContainerRuntime: datamodel.Containerd,
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
				CaCertificate:        string(caCertDecoded),
				ClientCertificate:    string(clientCertDecoded),
				APIServerCertificate: string(apiServerCertDecoded),
				ClientPrivateKey:     string(clientKeyDecoded),
			},
		},
	}

	//Adding a dummy key because we are not actually ssh'ing into the node.
	cs.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{
		KeyData: string("dummysshkey"),
	}}

	agentPool := cs.Properties.AgentPoolProfiles[0]
	baker := agent.InitializeTemplateGenerator()

	pauseImage := "mcr.microsoft.com/oss/kubernetes/pause:3.5"
	hyperkubeImage := "mcr.microsoft.com/oss/kubernetes/"
	windowsPackage := "windowspackage" //dummy string because not needed for our purpose, might want to align it better

	k8sComponents := &datamodel.K8sComponents{
		PodInfraContainerImageURL: pauseImage,
		HyperkubeImageURL:         hyperkubeImage,
		WindowsPackageURL:         windowsPackage,
	}

	kubeletConfig := map[string]string{
		"--address":                           "0.0.0.0",
		"--anonymous-auth":                    "false",
		"--authentication-token-webhook":      "true",
		"--authorization-mode":                "Webhook",
		"--azure-container-registry-config":   "/etc/kubernetes/azure.json",
		"--cgroups-per-qos":                   "true",
		"--client-ca-file":                    "/etc/kubernetes/certs/ca.crt",
		"--cloud-config":                      "/etc/kubernetes/azure.json",
		"--cloud-provider":                    "azure",
		"--cluster-dns":                       "10.0.0.10",
		"--cluster-domain":                    "cluster.local",
		"--dynamic-config-dir":                "/var/lib/kubelet",
		"--enforce-node-allocatable":          "pods",
		"--event-qps":                         "0",
		"--eviction-hard":                     "memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%",
		"--feature-gates":                     "RotateKubeletServerCertificate=true",
		"--image-gc-high-threshold":           "85",
		"--image-gc-low-threshold":            "80",
		"--image-pull-progress-deadline":      "30m",
		"--keep-terminated-pod-volumes":       "false",
		"--kube-reserved":                     "cpu=100m,memory=1638Mi",
		"--kubeconfig":                        "/var/lib/kubelet/kubeconfig",
		"--max-pods":                          "110",
		"--network-plugin":                    "kubenet",
		"--node-status-update-frequency":      "10s",
		"--non-masquerade-cidr":               "0.0.0.0/0",
		"--pod-infra-container-image":         "mcr.microsoft.com/oss/kubernetes/pause:3.5",
		"--pod-manifest-path":                 "/etc/kubernetes/manifests",
		"--pod-max-pids":                      "-1",
		"--protect-kernel-defaults":           "true",
		"--read-only-port":                    "0",
		"--resolv-conf":                       "/run/systemd/resolve/resolv.conf",
		"--rotate-certificates":               "false",
		"--streaming-connection-idle-timeout": "4h",
		"--tls-cert-file":                     "/etc/kubernetes/certs/kubeletserver.crt",
		"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256",
		"--tls-private-key-file":              "/etc/kubernetes/certs/kubeletserver.key",
	}

	config := &datamodel.NodeBootstrappingConfiguration{
		ContainerService:               cs,
		CloudSpecConfig:                datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:                  k8sComponents,
		AgentPoolProfile:               agentPool,
		TenantID:                       values.TenantID,
		SubscriptionID:                 values.SubID,
		ResourceGroupName:              values.MCRGName,
		UserAssignedIdentityClientID:   values.UserAssignedIdentityID,
		ConfigGPUDriverIfNeeded:        true,
		EnableGPUDevicePluginIfNeeded:  false,
		EnableKubeletConfigFile:        false,
		EnableNvidia:                   false,
		FIPSEnabled:                    false,
		KubeletClientTLSBootstrapToken: to.StringPtr(values.TLSBootstrapToken),
		KubeletConfig:                  kubeletConfig,
	}

	// customData
	base64EncodedCustomData := baker.GetNodeBootstrappingPayload(config)
	customDataBytes, _ := base64.StdEncoding.DecodeString(base64EncodedCustomData)
	customData := string(customDataBytes)
	err = ioutil.WriteFile("cloud-init.txt", []byte(customData), 0644)
	if err != nil {
		fmt.Println("couldnt write to file", err)
	}

	// cseCmd
	cseCommand := baker.GetNodeBootstrappingCmd(config)
	err = ioutil.WriteFile("csecmd", []byte(cseCommand), 0644)
	if err != nil {
		fmt.Println("couldnt write to file", err)
	}
}
