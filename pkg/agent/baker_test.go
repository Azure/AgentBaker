package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/barkimedes/go-deepcopy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

func generateTestData() bool {
	return os.Getenv("GENERATE_TEST_DATA") == "true"
}

// this regex looks for groups of the following forms, returning KEY and VALUE as submatches.
/* - KEY=VALUE
- KEY="VALUE"
- KEY=
- KEY="VALUE WITH WHITSPACE". */
const cseRegexString = `([^=\s]+)=(\"[^\"]*\"|[^\s]*)`

// test certificate.
const encodedTestCert = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUgvVENDQmVXZ0F3SUJBZ0lRYUJZRTMvTTA4WEhZQ25OVm1jRkJjakFOQmdrcWhraUc5dzBCQVFzRkFEQnkKTVFzd0NRWURWUVFHRXdKVlV6RU9NQXdHQTFVRUNBd0ZWR1Y0WVhNeEVEQU9CZ05WQkFjTUIwaHZkWE4wYjI0eApFVEFQQmdOVkJBb01DRk5UVENCRGIzSndNUzR3TEFZRFZRUUREQ1ZUVTB3dVkyOXRJRVZXSUZOVFRDQkpiblJsCmNtMWxaR2xoZEdVZ1EwRWdVbE5CSUZJek1CNFhEVEl3TURRd01UQXdOVGd6TTFvWERUSXhNRGN4TmpBd05UZ3oKTTFvd2diMHhDekFKQmdOVkJBWVRBbFZUTVE0d0RBWURWUVFJREFWVVpYaGhjekVRTUE0R0ExVUVCd3dIU0c5MQpjM1J2YmpFUk1BOEdBMVVFQ2d3SVUxTk1JRU52Y25BeEZqQVVCZ05WQkFVVERVNVdNakF3T0RFMk1UUXlORE14CkZEQVNCZ05WQkFNTUMzZDNkeTV6YzJ3dVkyOXRNUjB3R3dZRFZRUVBEQlJRY21sMllYUmxJRTl5WjJGdWFYcGgKZEdsdmJqRVhNQlVHQ3lzR0FRUUJnamM4QWdFQ0RBWk9aWFpoWkdFeEV6QVJCZ3NyQmdFRUFZSTNQQUlCQXhNQwpWVk13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRREhoZVJrYmIxRkNjN3hSS3N0CndLMEpJR2FLWTh0N0piUzJiUTJiNllJSkRnbkh1SVlIcUJyQ1VWNzlvZWxpa2tva1JrRnZjdnBhS2luRkhEUUgKVXBXRUk2UlVFUlltU0NnM084V2k0MnVPY1YyQjVaYWJtWENrd2R4WTVFY2w1MUJiTThVbkdkb0FHYmRObWlSbQpTbVRqY3MrbGhNeGc0ZkZZNmxCcGlFVkZpR1VqR1JSKzYxUjY3THo2VTRLSmVMTmNDbTA3UXdGWUtCbXBpMDhnCmR5Z1N2UmRVdzU1Sm9wcmVkaitWR3RqVWtCNGhGVDRHUVgvZ2h0NjlSbHF6Lys4dTBkRVFraHVVdXVjcnFhbG0KU0d5NDNIUndCZkRLRndZZVdNN0NQTWQ1ZS9kTyt0MDh0OFBianpWVFR2NWhRRENzRVlJVjJUN0FGSTlTY054TQpraDcvQWdNQkFBR2pnZ05CTUlJRFBUQWZCZ05WSFNNRUdEQVdnQlMvd1ZxSC95ajZRVDM5dDAva0hhK2dZVmdwCnZUQi9CZ2dyQmdFRkJRY0JBUVJ6TUhFd1RRWUlLd1lCQlFVSE1BS0dRV2gwZEhBNkx5OTNkM2N1YzNOc0xtTnYKYlM5eVpYQnZjMmwwYjNKNUwxTlRUR052YlMxVGRXSkRRUzFGVmkxVFUwd3RVbE5CTFRRd09UWXRVak11WTNKMApNQ0FHQ0NzR0FRVUZCekFCaGhSb2RIUndPaTh2YjJOemNITXVjM05zTG1OdmJUQWZCZ05WSFJFRUdEQVdnZ3QzCmQzY3VjM05zTG1OdmJZSUhjM05zTG1OdmJUQmZCZ05WSFNBRVdEQldNQWNHQldlQkRBRUJNQTBHQ3lxRWFBR0cKOW5jQ0JRRUJNRHdHRENzR0FRUUJncWt3QVFNQkJEQXNNQ29HQ0NzR0FRVUZCd0lCRmg1b2RIUndjem92TDNkMwpkeTV6YzJ3dVkyOXRMM0psY0c5emFYUnZjbmt3SFFZRFZSMGxCQll3RkFZSUt3WUJCUVVIQXdJR0NDc0dBUVVGCkJ3TUJNRWdHQTFVZEh3UkJNRDh3UGFBN29EbUdOMmgwZEhBNkx5OWpjbXh6TG5OemJDNWpiMjB2VTFOTVkyOXQKTFZOMVlrTkJMVVZXTFZOVFRDMVNVMEV0TkRBNU5pMVNNeTVqY213d0hRWURWUjBPQkJZRUZBREFGVUlhenc1cgpaSUhhcG5SeElVbnB3K0dMTUE0R0ExVWREd0VCL3dRRUF3SUZvRENDQVgwR0Npc0dBUVFCMW5rQ0JBSUVnZ0Z0CkJJSUJhUUZuQUhjQTlseVVMOUYzTUNJVVZCZ0lNSlJXanVOTkV4a3p2OThNTHlBTHpFN3haT01BQUFGeE0waG8KYndBQUJBTUFTREJHQWlFQTZ4ZWxpTlI4R2svNjNwWWRuUy92T3gvQ2pwdEVNRXY4OVdXaDEvdXJXSUVDSVFEeQpCcmVIVTI1RHp3dWtRYVJRandXNjU1WkxrcUNueGJ4UVdSaU9lbWo5SkFCMUFKUWd2QjZPMVkxc2lITWZnb3NpCkxBM1IyazFlYkUrVVBXSGJUaTlZVGFMQ0FBQUJjVE5JYU53QUFBUURBRVl3UkFJZ0dSRTR3emFiTlJkRDhrcS8KdkZQM3RRZTJobTB4NW5YdWxvd2g0SWJ3M2xrQ0lGWWIvM2xTRHBsUzdBY1I0citYcFd0RUtTVEZXSm1OQ1JiYwpYSnVyMlJHQkFIVUE3c0NWN28xeVpBK1M0OE81RzhjU28ybHFDWHRMYWhvVU9PWkhzc3Z0eGZrQUFBRnhNMGhvCjh3QUFCQU1BUmpCRUFpQjZJdmJvV3NzM1I0SXRWd2plYmw3RDN5b0ZhWDBORGgyZFdoaGd3Q3hySHdJZ0NmcTcKb2NNQzV0KzFqaTVNNXhhTG1QQzRJK1dYM0kvQVJrV1N5aU83SVFjd0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dJQgpBQ2V1dXI0UW51anFtZ3VTckhVM21oZitjSm9kelRRTnFvNHRkZStQRDEvZUZkWUFFTHU4eEYrMEF0N3hKaVBZCmk1Ukt3aWx5UDU2diszaVkyVDlsdzdTOFRKMDQxVkxoYUlLcDE0TXpTVXpSeWVvT0FzSjdRQURNQ2xIS1VEbEgKVVUycE51bzg4WTZpZ292VDNic253Sk5pRVFOcXltU1NZaGt0dzB0YWR1b3FqcVhuMDZnc1Zpb1dUVkRYeXNkNQpxRXg0dDZzSWdJY01tMjZZSDF2SnBDUUVoS3BjMnkwN2dSa2tsQlpSdE1qVGh2NGNYeXlNWDd1VGNkVDdBSkJQCnVlaWZDb1YyNUp4WHVvOGQ1MTM5Z3dQMUJBZTdJQlZQeDJ1N0tOL1V5T1hkWm13TWYvVG1GR3dEZENmc3lIZi8KWnNCMndMSG96VFlvQVZtUTlGb1UxSkxnY1ZpdnFKK3ZObEJoSFhobHhNZE4wajgwUjlOejZFSWdsUWplSzNPOApJL2NGR20vQjgrNDJoT2xDSWQ5WmR0bmRKY1JKVmppMHdEMHF3ZXZDYWZBOWpKbEh2L2pzRStJOVV6NmNwQ3loCnN3K2xyRmR4VWdxVTU4YXhxZUs4OUZSK05vNHEwSUlPK0ppMXJKS3I5bmtTQjBCcVhvelZuRTFZQi9LTHZkSXMKdVlaSnVxYjJwS2t1K3p6VDZnVXdIVVRadkJpTk90WEw0Tnh3Yy9LVDdXek9TZDJ3UDEwUUk4REtnNHZmaU5EcwpIV21CMWM0S2ppNmdPZ0E1dVNVemFHbXEvdjRWbmNLNVVyK245TGJmbmZMYzI4SjVmdC9Hb3Rpbk15RGszaWFyCkYxMFlscWNPbWVYMXVGbUtiZGkvWG9yR2xrQ29NRjNURHg4cm1wOURCaUIvCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=" //nolint:lll

type nodeBootstrappingOutput struct {
	customData string
	cseCmd     string
	files      map[string]*decodedValue
	vars       map[string]string
}

type decodedValue struct {
	encoding cseVariableEncoding
	value    string
}

type cseVariableEncoding string

const (
	cseVariableEncodingBase64 cseVariableEncoding = "base64"
	cseVariableEncodingGzip   cseVariableEncoding = "gzip"
)

type outputValidator func(*nodeBootstrappingOutput)

var _ = Describe("Assert generated customData and cseCmd", func() {
	DescribeTable("Generated customData and CSE", func(folder, k8sVersion string, configUpdator func(*datamodel.NodeBootstrappingConfiguration),
		validator outputValidator) {
		cs := &datamodel.ContainerService{
			Location: "southcentralus",
			Type:     "Microsoft.ContainerService/ManagedClusters",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    datamodel.Kubernetes,
					OrchestratorVersion: k8sVersion,
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

		// AKS always pass in te customHyperKubeImage to aks-e, so we don't really rely on
		// the default component version for "hyperkube", which is not set since 1.17
		if IsKubernetesVersionGe(k8sVersion, "1.17.0") {
			cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage = fmt.Sprintf("k8s.gcr.io/hyperkube-amd64:v%v", k8sVersion)
		}

		agentPool := cs.Properties.AgentPoolProfiles[0]

		fullK8sComponentsMap := K8sComponentsByVersionMap[cs.Properties.OrchestratorProfile.OrchestratorVersion]
		pauseImage := cs.Properties.OrchestratorProfile.KubernetesConfig.MCRKubernetesImageBase + fullK8sComponentsMap["pause"]

		hyperkubeImageBase := cs.Properties.OrchestratorProfile.KubernetesConfig.KubernetesImageBase
		hyperkubeImage := hyperkubeImageBase + fullK8sComponentsMap["hyperkube"]
		if cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage != "" {
			hyperkubeImage = cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage
		}

		windowsPackage := datamodel.AzurePublicCloudSpecForTest.KubernetesSpecConfig.KubeBinariesSASURLBase + fullK8sComponentsMap["windowszip"]
		k8sComponents := &datamodel.K8sComponents{
			PodInfraContainerImageURL: pauseImage,
			HyperkubeImageURL:         hyperkubeImage,
			WindowsPackageURL:         windowsPackage,
		}

		if IsKubernetesVersionGe(k8sVersion, "1.29.0") {
			k8sComponents.WindowsCredentialProviderURL = fmt.Sprintf("https://acs-mirror.azureedge.net/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-windows-amd64-v%s.tar.gz", k8sVersion, k8sVersion) //nolint:lll
			k8sComponents.LinuxCredentialProviderURL = fmt.Sprintf("https://acs-mirror.azureedge.net/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-linux-amd64-v%s.tar.gz", k8sVersion, k8sVersion)     //nolint:lll
		}

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
			"--container-log-max-size":            "50M",
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
			IsARM64:                       false,
			DisableUnattendedUpgrades:     false,
			SSHStatus:                     datamodel.SSHUnspecified,
			SIGConfig: datamodel.SIGConfig{
				TenantID:       "tenantID",
				SubscriptionID: "subID",
				Galleries: map[string]datamodel.SIGGalleryConfig{
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
						GalleryName:   "AKSWindows",
						ResourceGroup: "AKS-Windows",
					},
					"AKSUbuntuEdgeZone": {
						GalleryName:   "AKSUbuntuEdgeZone",
						ResourceGroup: "AKS-Ubuntu-EdgeZone",
					},
				},
			},
		}

		if configUpdator != nil {
			configUpdator(config)
		}

		// !!! WARNING !!!
		// avoid mutation of the original config -- both functions mutate input.
		// GetNodeBootstrappingPayload mutates the input so it's not the same as what gets passed to GetNodeBootstrappingCmd which causes bugs.
		// unit tests should always rely on unmutated copies of the base config.
		configCustomDataInput, err := deepcopy.Anything(config)
		Expect(err).To(BeNil())

		configCseInput, err := deepcopy.Anything(config)
		Expect(err).To(BeNil())

		// customData
		ab, err := NewAgentBaker()
		Expect(err).To(BeNil())
		nodeBootstrapping, err := ab.GetNodeBootstrapping(
			context.Background(),
			configCustomDataInput.(*datamodel.NodeBootstrappingConfiguration),
		)
		Expect(err).To(BeNil())
		customDataBytes, err := base64.StdEncoding.DecodeString(nodeBootstrapping.CustomData)
		customData := string(customDataBytes)
		Expect(err).To(BeNil())

		if generateTestData() {
			backfillCustomData(folder, customData)
		}

		expectedCustomData, err := os.ReadFile(fmt.Sprintf("./testdata/%s/CustomData", folder))
		Expect(err).To(BeNil())
		Expect(customData).To(Equal(string(expectedCustomData)))

		// CSE
		ab, err = NewAgentBaker()
		Expect(err).To(BeNil())
		nodeBootstrapping, err = ab.GetNodeBootstrapping(
			context.Background(),
			configCseInput.(*datamodel.NodeBootstrappingConfiguration),
		)
		Expect(err).To(BeNil())
		cseCommand := nodeBootstrapping.CSE

		if generateTestData() {
			err = os.WriteFile(fmt.Sprintf("./testdata/%s/CSECommand", folder), []byte(cseCommand), 0644)
			Expect(err).To(BeNil())
		}

		expectedCSECommand, err := os.ReadFile(fmt.Sprintf("./testdata/%s/CSECommand", folder))
		Expect(err).To(BeNil())
		Expect(cseCommand).To(Equal(string(expectedCSECommand)))

		files, err := getDecodedFilesFromCustomdata(customDataBytes)
		Expect(err).To(BeNil())

		vars, err := getDecodedVarsFromCseCmd([]byte(cseCommand))
		Expect(err).To(BeNil())

		result := &nodeBootstrappingOutput{
			customData: customData,
			cseCmd:     cseCommand,
			files:      files,
			vars:       vars,
		}

		if validator != nil {
			validator(result)
		}

	}, Entry("AKSUbuntu1604 with k8s version less than 1.18", "AKSUbuntu1604+K8S115", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
		config.KubeletConfig["--dynamic-config-dir"] = "/var/lib/kubelet/"
	}, func(o *nodeBootstrappingOutput) {
		etcDefaultKubelet := o.files["/etc/default/kubelet"].value

		Expect(o.vars["KUBELET_FLAGS"]).NotTo(BeEmpty())
		Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "DynamicKubeletConfig")).To(BeTrue())
		Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "DynamicKubeletConfig")).To(BeTrue())
		Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--dynamic-config-dir")).To(BeFalse())
		Expect(etcDefaultKubelet).NotTo(BeEmpty())
		Expect(strings.Contains(etcDefaultKubelet, "DynamicKubeletConfig")).To(BeTrue())
		Expect(strings.Contains(etcDefaultKubelet, "--dynamic-config-dir")).To(BeFalse())
		Expect(strings.Contains(o.cseCmd, "DynamicKubeletConfig")).To(BeTrue())

		// sanity check that no other files/variables set the flag
		for _, f := range o.files {
			Expect(strings.Contains(f.value, "--dynamic-config-dir")).To(BeFalse())
		}
		for _, v := range o.vars {
			Expect(strings.Contains(v, "--dynamic-config-dir")).To(BeFalse())
		}

		kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
		Expect(err).To(BeNil())

		var kubeletConfigFile datamodel.AKSKubeletConfiguration

		err = json.Unmarshal([]byte(kubeletConfigFileContent), &kubeletConfigFile)
		Expect(err).To(BeNil())

		dynamicConfigFeatureGate, dynamicConfigFeatureGateExists := kubeletConfigFile.FeatureGates["DynamicKubeletConfig"]
		Expect(dynamicConfigFeatureGateExists).To(Equal(true))
		Expect(dynamicConfigFeatureGate).To(Equal(false))
	}),
		Entry("AKSUbuntu1604 with k8s version 1.18", "AKSUbuntu1604+K8S118", "1.18.2", nil, nil),
		Entry("AKSUbuntu1604 with k8s version 1.17", "AKSUbuntu1604+K8S117", "1.17.7", nil, nil),
		Entry("AKSUbuntu1604 with temp disk (toggle)", "AKSUbuntu1604+TempDiskToggle", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			// this tests prioritization of the new api property vs the old property i'd like to remove.
			// ContainerRuntimeConfig should take priority until we remove it entirely
			config.AgentPoolProfile.KubeletDiskType = datamodel.OSDisk
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntimeConfig: map[string]string{
					datamodel.ContainerDataDirKey: "/mnt/containers",
				},
			}

			config.KubeletConfig = map[string]string{}
		}, nil),
		Entry("AKSUbuntu11604 with containerd", "AKSUbuntu1604+Containerd", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerdPackageURL = "containerd-package-url"
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["CLI_TOOL"]).To(Equal("ctr"))
			Expect(o.vars["CONTAINERD_PACKAGE_URL"]).To(Equal("containerd-package-url"))
		}),

		Entry("AKSUbuntu11604 with docker and containerd package url", "AKSUbuntu1604+Docker", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Docker,
			}
			config.ContainerdPackageURL = "containerd-package-url"
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["CLI_TOOL"]).To(Equal("docker"))
			Expect(o.vars["CONTAINERD_PACKAGE_URL"]).To(Equal(""))
		}),
		Entry("AKSUbuntu1604 with temp disk (api field)", "AKSUbuntu1604+TempDiskExplicit", "1.15.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				// also tests prioritization, but now the API property should take precedence
				config.AgentPoolProfile.KubeletDiskType = datamodel.TempDisk
			}, nil),
		Entry("AKSUbuntu1604 with OS disk", "AKSUbuntu1604+OSKubeletDisk", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			// also tests prioritization, but now the API property should take precedence
			config.AgentPoolProfile.KubeletDiskType = datamodel.OSDisk
		}, nil),
		Entry("AKSUbuntu1604 with Temp Disk and containerd", "AKSUbuntu1604+TempDisk+Containerd", "1.15.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntimeConfig: map[string]string{
						datamodel.ContainerDataDirKey: "/mnt/containers",
					},
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}

				config.KubeletConfig = map[string]string{}
			}, nil),
		Entry("AKSUbuntu1604 with RawUbuntu", "RawUbuntu", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.Ubuntu
		}, nil),
		Entry("AKSUbuntu1604 EnablePrivateClusterHostsConfigAgent", "AKSUbuntu1604+EnablePrivateClusterHostsConfigAgent", "1.18.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				cs := config.ContainerService
				if cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster == nil {
					cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster = &datamodel.PrivateCluster{EnableHostsConfigAgent: to.BoolPtr(true)}
				} else {
					cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.EnableHostsConfigAgent = to.BoolPtr(true)
				}
			}, nil),
		Entry("AKSUbuntu1804 with GPU dedicated VHD", "AKSUbuntu1604+GPUDedicatedVHD", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuGPU1804
			config.AgentPoolProfile.VMSize = "Standard_NC6"
			config.ConfigGPUDriverIfNeeded = false
			config.EnableGPUDevicePluginIfNeeded = true
			config.EnableNvidia = true
		}, nil),
		Entry("AKSUbuntu1604 with KubeletConfigFile", "AKSUbuntu1604+KubeletConfigFile", "1.15.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableKubeletConfigFile = true
		}, nil),

		Entry("AKSUbuntu1804 with containerd and private ACR", "AKSUbuntu1804+Containerd+PrivateACR", "1.18.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}

				config.KubeletConfig = map[string]string{}
				cs := config.ContainerService
				if cs.Properties.OrchestratorProfile.KubernetesConfig == nil {
					cs.Properties.OrchestratorProfile.KubernetesConfig = &datamodel.KubernetesConfig{}
				}
				cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateAzureRegistryServer = "acr.io/privateacr"
				cs.Properties.ServicePrincipalProfile = &datamodel.ServicePrincipalProfile{
					ClientID: "clientID",
					Secret:   "clientSecret",
				}
			}, nil),
		Entry("AKSUbuntu1804 with containerd and GPU SKU", "AKSUbuntu1804+Containerd+NSeriesSku", "1.15.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
				config.EnableNvidia = true
				config.KubeletConfig = map[string]string{}
			}, nil),
		Entry("AKSUbuntu1804 with containerd and kubenet cni", "AKSUbuntu1804+Containerd+Kubenet", "1.18.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
				config.KubeletConfig = map[string]string{}
			}, nil),
		Entry("AKSUbuntu1804 with containerd and kubenet cni and calico policy", "AKSUbuntu1804+Containerd+Kubenet+Calico", "1.18.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = NetworkPolicyCalico
				config.KubeletConfig = map[string]string{}
			}, nil),
		Entry("AKSUbuntu1804 with containerd and teleport enabled", "AKSUbuntu1804+Containerd+Teleport", "1.18.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableACRTeleportPlugin = true
				config.TeleportdPluginURL = "some url"
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with containerd and ipmasqagent enabled", "AKSUbuntu1804+Containerd+IPMasqAgent", "1.18.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableACRTeleportPlugin = true
				config.TeleportdPluginURL = "some url"
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
				config.ContainerService.Properties.HostedMasterProfile.IPMasqAgent = true
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with containerd and version specified", "AKSUbuntu1804+Containerd+ContainerdVersion", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerdVersion = "1.4.4"
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1604 with custom kubeletConfig and osConfig", "AKSUbuntu1604+CustomKubeletConfig+CustomLinuxOSConfig", "1.16.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableKubeletConfigFile = false
				netIpv4TcpTwReuse := true
				failSwapOn := false
				var swapFileSizeMB int32 = 1500
				var netCoreSomaxconn int32 = 1638499
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = &datamodel.CustomKubeletConfig{
					CPUManagerPolicy:      "static",
					CPUCfsQuota:           to.BoolPtr(false),
					CPUCfsQuotaPeriod:     "200ms",
					ImageGcHighThreshold:  to.Int32Ptr(90),
					ImageGcLowThreshold:   to.Int32Ptr(70),
					TopologyManagerPolicy: "best-effort",
					AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
					FailSwapOn:            &failSwapOn,
					ContainerLogMaxSizeMB: to.Int32Ptr(1000),
					ContainerLogMaxFiles:  to.Int32Ptr(99),
					PodMaxPids:            to.Int32Ptr(12345),
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = &datamodel.CustomLinuxOSConfig{
					Sysctls: &datamodel.SysctlConfig{
						NetCoreSomaxconn:             &netCoreSomaxconn,
						NetCoreRmemDefault:           to.Int32Ptr(456000),
						NetCoreWmemDefault:           to.Int32Ptr(89000),
						NetIpv4TcpTwReuse:            &netIpv4TcpTwReuse,
						NetIpv4IpLocalPortRange:      "32768 65400",
						NetIpv4TcpMaxSynBacklog:      to.Int32Ptr(1638498),
						NetIpv4NeighDefaultGcThresh1: to.Int32Ptr(10001),
					},
					TransparentHugePageEnabled: "never",
					TransparentHugePageDefrag:  "defer+madvise",
					SwapFileSizeMB:             &swapFileSizeMB,
				}
			}, func(o *nodeBootstrappingOutput) {
				sysctlContent, err := getBase64DecodedValue([]byte(o.vars["SYSCTL_CONTENT"]))
				Expect(err).To(BeNil())
				// assert defaults for gc_thresh2 and gc_thresh3
				// assert custom values for all others.
				Expect(sysctlContent).To(ContainSubstring("net.core.somaxconn=1638499"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.tcp_max_syn_backlog=1638498"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh1=10001"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh2=8192"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh3=16384"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.ip_local_reserved_ports=65330"))
			}),

		Entry("AKSUbuntu1604 - dynamic-config-dir should always be removed with custom kubelet config",
			"AKSUbuntu1604+CustomKubeletConfig+DynamicKubeletConfig", "1.16.13", func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = &datamodel.CustomKubeletConfig{
					CPUManagerPolicy:      "static",
					CPUCfsQuota:           to.BoolPtr(false),
					CPUCfsQuotaPeriod:     "200ms",
					ImageGcHighThreshold:  to.Int32Ptr(90),
					ImageGcLowThreshold:   to.Int32Ptr(70),
					TopologyManagerPolicy: "best-effort",
					AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
					ContainerLogMaxSizeMB: to.Int32Ptr(1000),
					ContainerLogMaxFiles:  to.Int32Ptr(99),
					PodMaxPids:            to.Int32Ptr(12345),
				}
				config.KubeletConfig = map[string]string{
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
					"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint: lll
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
					"--dynamic-config-dir":                "",
				}
			}, func(o *nodeBootstrappingOutput) {
				sysctlContent, err := getBase64DecodedValue([]byte(o.vars["SYSCTL_CONTENT"]))
				Expect(err).To(BeNil())
				// assert defaults for all.
				Expect(sysctlContent).To(ContainSubstring("net.core.somaxconn=16384"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.tcp_max_syn_backlog=16384"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh1=4096"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh2=8192"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh3=16384"))
			}),

		Entry("AKSUbuntu1604 - dynamic-config-dir should always be removed", "AKSUbuntu1604+DynamicKubeletConfig", "1.16.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletConfig = map[string]string{
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
					"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint: lll
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
					"--dynamic-config-dir":                "",
				}
			}, nil),

		Entry("RawUbuntu with Containerd", "RawUbuntuContainerd", "1.19.1", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.Ubuntu
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.KubeletConfig = map[string]string{}
		}, nil),

		Entry("AKSUbuntu1604 with Disable1804SystemdResolved=true", "AKSUbuntu1604+Disable1804SystemdResolved=true", "1.16.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.Disable1804SystemdResolved = true
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Docker,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1604 with Disable1804SystemdResolved=false", "AKSUbuntu1604+Disable1804SystemdResolved=false", "1.16.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.Disable1804SystemdResolved = false
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Docker,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with Disable1804SystemdResolved=true", "AKSUbuntu1804+Disable1804SystemdResolved=true", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.Disable1804SystemdResolved = true
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with Disable1804SystemdResolved=false", "AKSUbuntu1804+Disable1804SystemdResolved=false", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.Disable1804SystemdResolved = false
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NC6"
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with kubelet client TLS bootstrapping enabled", "AKSUbuntu1804+KubeletClientTLSBootstrapping", "1.18.3",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletClientTLSBootstrapToken = to.StringPtr("07401b.f395accd246ae52d")
				config.ContainerService.Properties.CertificateProfile = &datamodel.CertificateProfile{
					CaCertificate: "fooBarBaz",
				}
			}, func(o *nodeBootstrappingOutput) {
				// Please see #2815 for more details
				etcDefaultKubelet := o.files["/etc/default/kubelet"].value
				etcDefaultKubeletService := o.files["/etc/systemd/system/kubelet.service"].value
				kubeletSh := o.files["/opt/azure/containers/kubelet.sh"].value
				bootstrapKubeconfig := o.files["/var/lib/kubelet/bootstrap-kubeconfig"].value
				caCRT := o.files["/etc/kubernetes/certs/ca.crt"].value

				Expect(etcDefaultKubelet).NotTo(BeEmpty())
				Expect(bootstrapKubeconfig).NotTo(BeEmpty())
				Expect(kubeletSh).NotTo(BeEmpty())
				Expect(tlsBootstrapDropin).ToNot(BeEmpty())
				Expect(etcDefaultKubeletService).NotTo(BeEmpty())
				Expect(caCRT).NotTo(BeEmpty())

				Expect(bootstrapKubeconfig).To(ContainSubstring("token"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("07401b.f395accd246ae52d"))
				Expect(bootstrapKubeconfig).ToNot(ContainSubstring("command: /opt/azure/tlsbootstrap/tls-bootstrap-client"))
			}),

		Entry("AKSUbuntu2204 with secure TLS bootstrapping enabled", "AKSUbuntu2204+SecureTLSBoostrapping", "1.25.6",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableSecureTLSBootstrapping = true
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_SECURE_TLS_BOOTSTRAPPING"]).To(Equal("true"))
				Expect(o.vars["CUSTOM_SECURE_TLS_BOOTSTRAP_AAD_SERVER_APP_ID"]).To(BeEmpty())

				bootstrapKubeconfig := o.files["/var/lib/kubelet/bootstrap-kubeconfig"].value
				Expect(bootstrapKubeconfig).ToNot(BeEmpty())
				Expect(bootstrapKubeconfig).To(ContainSubstring("apiVersion: client.authentication.k8s.io/v1"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("command: /opt/azure/tlsbootstrap/tls-bootstrap-client"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("- bootstrap"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("--next-proto=aks-tls-bootstrap"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("--aad-resource=6dae42f8-4368-4678-94ff-3960e28e3630"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("interactiveMode: Never"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("provideClusterInfo: true"))
				Expect(bootstrapKubeconfig).ToNot(ContainSubstring("token:"))
			}),

		Entry("AKSUbuntu2204 with secure TLS bootstrapping enabled using custom AAD server application ID", "AKSUbuntu2204+SecureTLSBootstrapping+CustomAADResource", "1.25.6",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableSecureTLSBootstrapping = true
				config.CustomSecureTLSBootstrapAADServerAppID = "appID"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_SECURE_TLS_BOOTSTRAPPING"]).To(Equal("true"))
				Expect(o.vars["CUSTOM_SECURE_TLS_BOOTSTRAP_AAD_SERVER_APP_ID"]).To(Equal("appID"))

				bootstrapKubeconfig := o.files["/var/lib/kubelet/bootstrap-kubeconfig"].value
				Expect(bootstrapKubeconfig).ToNot(BeEmpty())
				Expect(bootstrapKubeconfig).To(ContainSubstring("apiVersion: client.authentication.k8s.io/v1"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("command: /opt/azure/tlsbootstrap/tls-bootstrap-client"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("- bootstrap"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("--next-proto=aks-tls-bootstrap"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("--aad-resource=appID"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("interactiveMode: Never"))
				Expect(bootstrapKubeconfig).To(ContainSubstring("provideClusterInfo: true"))
				Expect(bootstrapKubeconfig).ToNot(ContainSubstring("token:"))
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation implicitly disabled", "AKSUbuntu2204+ImplicitlyDisableKubeletServingCertificateRotation", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("false"))
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation explicitly disabled", "AKSUbuntu2204+DisableKubeletServingCertificateRotation", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletConfig["--rotate-server-certificates"] = "false"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("false"))
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--rotate-server-certificates=false")).To(BeTrue())
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation enabled", "AKSUbuntu2204+KubeletServingCertificateRotation", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletConfig["--rotate-server-certificates"] = "true"
				config.KubeletConfig["--tls-cert-file"] = "cert.crt"
				config.KubeletConfig["--tls-private-key-file"] = "cert.key"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("true"))
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--rotate-server-certificates=true")).To(BeTrue())
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--tls-cert-file")).To(BeFalse())
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--tls-private-key-file")).To(BeFalse())
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation disabled and custom kubelet config",
			"AKSUbuntu2204+DisableKubeletServingCertificateRotation+CustomKubeletConfig", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableKubeletConfigFile = false
				failSwapOn := false
				config.KubeletConfig["--rotate-server-certificates"] = "false"
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = &datamodel.CustomKubeletConfig{
					CPUManagerPolicy:      "static",
					CPUCfsQuota:           to.BoolPtr(false),
					CPUCfsQuotaPeriod:     "200ms",
					ImageGcHighThreshold:  to.Int32Ptr(90),
					ImageGcLowThreshold:   to.Int32Ptr(70),
					TopologyManagerPolicy: "best-effort",
					AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
					FailSwapOn:            &failSwapOn,
					ContainerLogMaxSizeMB: to.Int32Ptr(1000),
					ContainerLogMaxFiles:  to.Int32Ptr(99),
					PodMaxPids:            to.Int32Ptr(12345),
				}
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("false"))
				kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
				Expect(err).To(BeNil())
				Expect(kubeletConfigFileContent).ToNot(ContainSubstring("serverTLSBootstrap")) // because of: "bool `json:"serverTLSBootstrap,omitempty"`"
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation enabled and custom kubelet config",
			"AKSUbuntu2204+KubeletServingCertificateRotation+CustomKubeletConfig", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableKubeletConfigFile = false
				failSwapOn := false
				config.KubeletConfig["--rotate-server-certificates"] = "true"
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = &datamodel.CustomKubeletConfig{
					CPUManagerPolicy:      "static",
					CPUCfsQuota:           to.BoolPtr(false),
					CPUCfsQuotaPeriod:     "200ms",
					ImageGcHighThreshold:  to.Int32Ptr(90),
					ImageGcLowThreshold:   to.Int32Ptr(70),
					TopologyManagerPolicy: "best-effort",
					AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
					FailSwapOn:            &failSwapOn,
					ContainerLogMaxSizeMB: to.Int32Ptr(1000),
					ContainerLogMaxFiles:  to.Int32Ptr(99),
					PodMaxPids:            to.Int32Ptr(12345),
				}
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("true"))
				kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
				Expect(err).To(BeNil())
				Expect(kubeletConfigFileContent).To(ContainSubstring(`"serverTLSBootstrap": true`))
			}),

		Entry("AKSUbuntu1804 with DisableCustomData = true", "AKSUbuntu1804+DisableCustomData", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableCustomData = true
			}, nil),

		Entry("Mariner v2 with kata", "MarinerV2+Kata", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2Kata
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
		}, nil),

		Entry("Mariner v2 with custom cloud", "MarinerV2+CustomCloud", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),

		Entry("AzureLinux v2 with kata", "AzureLinuxV2+Kata", "1.28.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "AzureLinux"
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2Kata
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
		}, nil),

		Entry("Mariner v2 with DisableUnattendedUpgrades=true", "Marinerv2+DisableUnattendedUpgrades=true", "1.23.8",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "Mariner"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("Mariner v2 with DisableUnattendedUpgrades=false", "Marinerv2+DisableUnattendedUpgrades=false", "1.23.8",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "Mariner"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("Mariner v2 with kata and DisableUnattendedUpgrades=true", "Marinerv2+Kata+DisableUnattendedUpgrades=true", "1.23.8",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "Mariner"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("Mariner v2 with kata and DisableUnattendedUpgrades=false", "Marinerv2+Kata+DisableUnattendedUpgrades=false", "1.23.8",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "Mariner"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("AzureLinux v2 with DisableUnattendedUpgrades=true", "AzureLinuxv2+DisableUnattendedUpgrades=true", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("AzureLinux v2 with DisableUnattendedUpgrades=false", "AzureLinuxv2+DisableUnattendedUpgrades=false", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("AzureLinux v2 with kata and DisableUnattendedUpgrades=true", "AzureLinuxv2+Kata+DisableUnattendedUpgrades=true", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("AzureLinux v2 with kata and DisableUnattendedUpgrades=false", "AzureLinuxv2+Kata+DisableUnattendedUpgrades=false", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("AKSUbuntu1804 with containerd and kubenet cni", "AKSUbuntu1804+Containerd+Kubenet+FIPSEnabled", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
				config.FIPSEnabled = true
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with http proxy config", "AKSUbuntu1804+HTTPProxy", "1.18.14", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.HTTPProxyConfig = &datamodel.HTTPProxyConfig{
				HTTPProxy:  to.StringPtr("http://myproxy.server.com:8080/"),
				HTTPSProxy: to.StringPtr("https://myproxy.server.com:8080/"),
				NoProxy: to.StringSlicePtr([]string{
					"localhost",
					"127.0.0.1",
				}),
				TrustedCA: to.StringPtr(encodedTestCert),
			}
		},
			func(o *nodeBootstrappingOutput) {
				Expect(o.files["/opt/azure/containers/provision.sh"].encoding).To(Equal(cseVariableEncodingGzip))
				cseMain := o.files["/opt/azure/containers/provision.sh"].value
				httpProxyStr := "export http_proxy=\"http://myproxy.server.com:8080/\""
				Expect(strings.Contains(cseMain, "eval $PROXY_VARS")).To(BeTrue())
				Expect(strings.Contains(cseMain, "$OUTBOUND_COMMAND")).To(BeTrue())
				// assert we eval exporting the proxy vars before checking outbound connectivity
				Expect(strings.Index(cseMain, "eval $PROXY_VARS") < strings.Index(cseMain, "$OUTBOUND_COMMAND")).To(BeTrue())
				Expect(strings.Contains(o.cseCmd, httpProxyStr)).To(BeTrue())
			},
		),

		Entry("AKSUbuntu2204 with outbound type blocked", "AKSUbuntu2204+OutboundTypeBlocked", "1.25.6", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OutboundType = datamodel.OutboundTypeBlock
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["BLOCK_OUTBOUND_NETWORK"]).To(Equal("true"))
		}),

		Entry("AKSUbuntu2204 with outbound type none", "AKSUbuntu2204+OutboundTypeNone", "1.25.6", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OutboundType = datamodel.OutboundTypeNone
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["BLOCK_OUTBOUND_NETWORK"]).To(Equal("true"))
		}),

		Entry("AKSUbuntu2204 with no outbound type", "AKSUbuntu2204+OutboundTypeNil", "1.25.6", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OutboundType = ""
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["BLOCK_OUTBOUND_NETWORK"]).To(Equal("false"))
		}),

		Entry("AKSUbuntu2204 with SerializeImagePulls=false and k8s 1.31", "AKSUbuntu2204+SerializeImagePulls", "1.31.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletConfig["--serialize-image-pulls"] = "false"
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["KUBELET_FLAGS"]).NotTo(BeEmpty())
			Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--serialize-image-pulls=false")).To(BeTrue())
		}),

		Entry("AKSUbuntu1804 with custom ca trust", "AKSUbuntu1804+CustomCATrust", "1.18.14", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
				CustomCATrustCerts: []string{encodedTestCert, encodedTestCert, encodedTestCert},
			}
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["CUSTOM_CA_TRUST_COUNT"]).To(Equal("3"))
			Expect(o.vars["SHOULD_CONFIGURE_CUSTOM_CA_TRUST"]).To(Equal("true"))
			Expect(o.vars["CUSTOM_CA_CERT_0"]).To(Equal(encodedTestCert))
			err := verifyCertsEncoding(o.vars["CUSTOM_CA_CERT_0"])
			Expect(err).To(BeNil())
		}),

		Entry("AKSUbuntu1804 with containerd and runcshimv2", "AKSUbuntu1804+Containerd+runcshimv2", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableRuncShimV2 = true
			}, nil),

		Entry("AKSUbuntu1804 with containerd and motd", "AKSUbuntu1804+Containerd+MotD", "1.19.13", func(config *datamodel.NodeBootstrappingConfiguration) {

			config.ContainerService.Properties.AgentPoolProfiles[0].MessageOfTheDay = "Zm9vYmFyDQo=" // foobar in b64
		}, nil),

		Entry("AKSUbuntu1804containerd with custom runc verison", "AKSUbuntu1804Containerd+RuncVersion", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.RuncVersion = "1.0.0-rc96"
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 with containerd+gpu and runcshimv2", "AKSUbuntu1804+Containerd++GPU+runcshimv2", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.AgentPoolProfile.VMSize = "Standard_NC6"
				config.EnableNvidia = true
				config.EnableRuncShimV2 = true
				config.KubeletConfig = map[string]string{}
			}, nil),

		Entry("AKSUbuntu1804 containerd with multi-instance GPU", "AKSUbuntu1804+Containerd+MIG", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.KubeletConfig = map[string]string{}
				config.AgentPoolProfile.VMSize = "Standard_ND96asr_v4"
				config.EnableNvidia = true
				config.GPUInstanceProfile = "MIG7g"
			}, nil),

		Entry("AKSUbuntu1804 containerd with multi-instance non-fabricmanager GPU", "AKSUbuntu1804+Containerd+MIG+NoFabricManager", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.KubeletConfig = map[string]string{}
				config.AgentPoolProfile.VMSize = "Standard_NC24ads_A100_v4"
				config.EnableNvidia = true
				config.GPUInstanceProfile = "MIG7g"
			}, nil),

		Entry("AKSUbuntu1804 with krustlet", "AKSUbuntu1804+krustlet", "1.20.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].WorkloadRuntime = datamodel.WasmWasi
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.CertificateProfile = &datamodel.CertificateProfile{
				CaCertificate: "fooBarBaz",
			}
			config.KubeletClientTLSBootstrapToken = to.StringPtr("07401b.f395accd246ae52d")
		},
			func(o *nodeBootstrappingOutput) {

				Expect(o.vars["CONTAINERD_CONFIG_CONTENT"]).NotTo(BeEmpty())
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_CONTENT"]))
				Expect(err).To(BeNil())
				expectedShimConfig := `version = 2
oom_score = 0
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  [plugins."io.containerd.grpc.v1.cri".containerd]
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin]
      runtime_type = "io.containerd.spin-v0-3-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight]
      runtime_type = "io.containerd.slight-v0-3-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin-v0-3-0]
      runtime_type = "io.containerd.spin-v0-3-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight-v0-3-0]
      runtime_type = "io.containerd.slight-v0-3-0.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin-v0-5-1]
      runtime_type = "io.containerd.spin-v0-5-1.v1"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight-v0-5-1]
      runtime_type = "io.containerd.slight-v0-5-1.v1"`

				Expect(containerdConfigFileContent).To(ContainSubstring(expectedShimConfig))
			},
		),
		Entry("AKSUbuntu2204 with artifact streaming", "AKSUbuntu1804+ArtifactStreaming", "1.25.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableArtifactStreaming = true
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
			config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.25.7"
		},
			func(o *nodeBootstrappingOutput) {

				Expect(o.vars["CONTAINERD_CONFIG_CONTENT"]).NotTo(BeEmpty())
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_CONTENT"]))
				Expect(err).To(BeNil())
				expectedOverlaybdConfig := `version = 2
oom_score = 0
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  [plugins."io.containerd.grpc.v1.cri".containerd]
    snapshotter = "overlaybd"
    disable_snapshot_annotations = false
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      SystemdCgroup = true
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"
`
				Expect(containerdConfigFileContent).To(ContainSubstring(expectedOverlaybdConfig))
				expectedOverlaybdPlugin := `[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"`
				Expect(containerdConfigFileContent).To(ContainSubstring(expectedOverlaybdPlugin))
			},
		),
		Entry("AKSUbuntu2204 w/o artifact streaming", "AKSUbuntu1804+NoArtifactStreaming", "1.25.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableArtifactStreaming = false
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
		},
			func(o *nodeBootstrappingOutput) {

				Expect(o.vars["CONTAINERD_CONFIG_CONTENT"]).NotTo(BeEmpty())
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_CONTENT"]))
				Expect(err).To(BeNil())
				expectedOverlaybdConfig := `[plugins."io.containerd.grpc.v1.cri".containerd]
    snapshotter = "overlaybd"
    disable_snapshot_annotations = false
    default_runtime_name = "runc"`
				Expect(containerdConfigFileContent).NotTo(ContainSubstring(expectedOverlaybdConfig))
				expectedOverlaybdPlugin := `[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"`
				Expect(containerdConfigFileContent).NotTo(ContainSubstring(expectedOverlaybdPlugin))
			},
		),
		Entry("AKSUbuntu1804 with NoneCNI", "AKSUbuntu1804+NoneCNI", "1.20.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = datamodel.NetworkPluginNone
		}, nil),
		Entry("AKSUbuntu1804 with Containerd and certs.d", "AKSUbuntu1804+Containerd+Certsd", "1.22.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
			}, nil),
		Entry("AKSUbuntu1804ARM64containerd with kubenet", "AKSUbuntu1804ARM64Containerd+NoCustomKubeImageandBinaries", "1.22.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorType = "azure"
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/1.22.2/binaries/kubernetes-node-linux-arm64.tar.gz" //nolint:lll
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeProxyImage = "mcr.microsoft.com/oss/kubernetes/kube-proxy:v1.22.2"                                           //nolint:lll
				config.IsARM64 = true
				config.KubeletConfig = map[string]string{}
			}, nil),
		Entry("AKSUbuntu1804ARM64containerd with kubenet", "AKSUbuntu1804ARM64Containerd+CustomKubeImageandBinaries", "1.22.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorType = datamodel.Kubernetes
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeBinaryURL = "https://acs-mirror.azureedge.net/kubernetes/1.22.2/binaries/kubernetes-node-linux-arm64.tar.gz" //nolint:lll
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.CustomKubeProxyImage = "mcr.microsoft.com/oss/kubernetes/kube-proxy:v1.22.2"                                           //nolint:lll
				config.IsARM64 = true
				config.KubeletConfig = map[string]string{}
			}, nil),
		Entry("AKSUbuntu1804 with IPAddress and FQDN", "AKSUbuntu1804+Containerd+IPAddress+FQDN", "1.22.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.HostedMasterProfile.FQDN = "a.hcp.eastus.azmk8s.io"
				config.ContainerService.Properties.HostedMasterProfile.IPAddress = "1.2.3.4"
			}, nil),
		Entry("AKSUbuntu2204 VHD, cgroupv2", "AKSUbuntu2204+cgroupv2", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
		}, nil),
		Entry("AKSUbuntu2204 containerd with multi-instance GPU", "AKSUbuntu2204+Containerd+MIG", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
				config.AgentPoolProfile.VMSize = "Standard_ND96asr_v4"
				// the purpose of this unit test is to ensure the containerd config
				// does not use the nvidia container runtime when skipping the
				// GPU driver install, since it will fail to run even non-GPU
				// pods, as it will not be installed.
				config.EnableNvidia = true
				config.ConfigGPUDriverIfNeeded = true
				config.GPUInstanceProfile = "MIG7g"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]).NotTo(BeEmpty())
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]))
				Expect(err).To(BeNil())
				expectedShimConfig := `version = 2
oom_score = 0
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  [plugins."io.containerd.grpc.v1.cri".containerd]
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      SystemdCgroup = true
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
`

				Expect(containerdConfigFileContent).To(Equal(expectedShimConfig))
			}),
		Entry("AKSUbuntu2204 containerd with multi-instance GPU and artifact streaming", "AKSUbuntu2204+Containerd+MIG+ArtifactStreaming", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.25.7"
				config.EnableArtifactStreaming = true
				config.AgentPoolProfile.VMSize = "Standard_ND96asr_v4"
				// the purpose of this unit test is to ensure the containerd config
				// does not use the nvidia container runtime when skipping the
				// GPU driver install, since it will fail to run even non-GPU
				// pods, as it will not be installed.
				config.EnableNvidia = true
				config.ConfigGPUDriverIfNeeded = true
				config.GPUInstanceProfile = "MIG7g"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]).NotTo(BeEmpty())
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]))
				Expect(err).To(BeNil())
				expectedShimConfig := `version = 2
oom_score = 0
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  [plugins."io.containerd.grpc.v1.cri".containerd]
    snapshotter = "overlaybd"
    disable_snapshot_annotations = false
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      SystemdCgroup = true
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"
`

				Expect(containerdConfigFileContent).To(Equal(expectedShimConfig))
			}),
		Entry("CustomizedImage VHD should not have provision_start.sh", "CustomizedImage", "1.24.2",
			func(c *datamodel.NodeBootstrappingConfiguration) {
				c.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				c.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.CustomizedImage
			}, func(o *nodeBootstrappingOutput) {
				_, exist := o.files["/opt/azure/containers/provision_start.sh"]

				Expect(exist).To(BeFalse())
			},
		),
		Entry("CustomizedImageKata VHD should not have provision_start.sh", "CustomizedImageKata", "1.24.2",
			func(c *datamodel.NodeBootstrappingConfiguration) {
				c.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				c.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.CustomizedImageKata
			}, func(o *nodeBootstrappingOutput) {
				_, exist := o.files["/opt/azure/containers/provision_start.sh"]

				Expect(exist).To(BeFalse())
			},
		),
		Entry("AKSUbuntu2204 DisableSSH with enabled ssh", "AKSUbuntu2204+SSHStatusOn", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.SSHStatus = datamodel.SSHOn
		}, nil),
		Entry("AKSUbuntu2204 DisableSSH with disabled ssh", "AKSUbuntu2204+SSHStatusOff", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.SSHStatus = datamodel.SSHOff
		}, nil),
		Entry("AKSUbuntu2204 in China", "AKSUbuntu2204+China", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "AzureChinaCloud",
			}
			config.ContainerService.Location = "chinaeast2"
		}, nil),
		Entry("AKSUbuntu2204 custom cloud", "AKSUbuntu2204+CustomCloud", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),
		Entry("AKSUbuntu2204 OOT credentialprovider", "AKSUbuntu2204+ootcredentialprovider", "1.29.10", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
			config.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["KUBELET_FLAGS"]).NotTo(BeEmpty())
			Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--image-credential-provider-config=/var/lib/kubelet/credential-provider-config.yaml")).To(BeTrue())
			Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--image-credential-provider-bin-dir=/var/lib/kubelet/credential-provider")).To(BeTrue())
		}),
		Entry("AKSUbuntu2204 custom cloud and OOT credentialprovider", "AKSUbuntu2204+CustomCloud+ootcredentialprovider", "1.29.10",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
					Name:                         "akscustom",
					McrURL:                       "mcr.microsoft.fakecustomcloud",
					RepoDepotEndpoint:            "https://repodepot.azure.microsoft.fakecustomcloud/ubuntu",
					ManagementPortalURL:          "https://portal.azure.microsoft.fakecustomcloud/",
					PublishSettingsURL:           "",
					ServiceManagementEndpoint:    "https://management.core.microsoft.fakecustomcloud/",
					ResourceManagerEndpoint:      "https://management.azure.microsoft.fakecustomcloud/",
					ActiveDirectoryEndpoint:      "https://login.microsoftonline.microsoft.fakecustomcloud/",
					GalleryEndpoint:              "",
					KeyVaultEndpoint:             "https://vault.cloudapi.microsoft.fakecustomcloud/",
					GraphEndpoint:                "https://graph.cloudapi.microsoft.fakecustomcloud/",
					ServiceBusEndpoint:           "",
					BatchManagementEndpoint:      "",
					StorageEndpointSuffix:        "core.microsoft.fakecustomcloud",
					SQLDatabaseDNSSuffix:         "database.cloudapi.microsoft.fakecustomcloud",
					TrafficManagerDNSSuffix:      "",
					KeyVaultDNSSuffix:            "vault.cloudapi.microsoft.fakecustomcloud",
					ServiceBusEndpointSuffix:     "",
					ServiceManagementVMDNSSuffix: "",
					ResourceManagerVMDNSSuffix:   "cloudapp.azure.microsoft.fakecustomcloud/",
					ContainerRegistryDNSSuffix:   ".azurecr.microsoft.fakecustomcloud",
					CosmosDBDNSSuffix:            "documents.core.microsoft.fakecustomcloud/",
					TokenAudience:                "https://management.core.microsoft.fakecustomcloud/",
					ResourceIdentifiers: datamodel.ResourceIdentifiers{
						Graph:               "",
						KeyVault:            "",
						Datalake:            "",
						Batch:               "",
						OperationalInsights: "",
						Storage:             "",
					},
				}
				config.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				config.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			}, func(o *nodeBootstrappingOutput) {

				Expect(o.vars["AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX"]).NotTo(BeEmpty())
				Expect(o.vars["AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX"]).To(Equal(".azurecr.microsoft.fakecustomcloud"))

				Expect(o.vars["KUBELET_FLAGS"]).NotTo(BeEmpty())
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--image-credential-provider-config=/var/lib/kubelet/credential-provider-config.yaml")).To(BeTrue())
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--image-credential-provider-bin-dir=/var/lib/kubelet/credential-provider")).To(BeTrue())
			}),
		Entry("AKSUbuntu2204 with custom kubeletConfig and osConfig", "AKSUbuntu2204+CustomKubeletConfig+CustomLinuxOSConfig", "1.24.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableKubeletConfigFile = false
				netIpv4TcpTwReuse := true
				failSwapOn := false
				var swapFileSizeMB int32 = 1500
				var netCoreSomaxconn int32 = 1638499
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = &datamodel.CustomKubeletConfig{
					CPUManagerPolicy:      "static",
					CPUCfsQuota:           to.BoolPtr(false),
					CPUCfsQuotaPeriod:     "200ms",
					ImageGcHighThreshold:  to.Int32Ptr(90),
					ImageGcLowThreshold:   to.Int32Ptr(70),
					TopologyManagerPolicy: "best-effort",
					AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
					FailSwapOn:            &failSwapOn,
					ContainerLogMaxSizeMB: to.Int32Ptr(1000),
					ContainerLogMaxFiles:  to.Int32Ptr(99),
					PodMaxPids:            to.Int32Ptr(12345),
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = &datamodel.CustomLinuxOSConfig{
					Sysctls: &datamodel.SysctlConfig{
						NetCoreSomaxconn:             &netCoreSomaxconn,
						NetCoreRmemDefault:           to.Int32Ptr(456000),
						NetCoreWmemDefault:           to.Int32Ptr(89000),
						NetIpv4TcpTwReuse:            &netIpv4TcpTwReuse,
						NetIpv4IpLocalPortRange:      "32768 65400",
						NetIpv4TcpMaxSynBacklog:      to.Int32Ptr(1638498),
						NetIpv4NeighDefaultGcThresh1: to.Int32Ptr(10001),
					},
					TransparentHugePageEnabled: "never",
					TransparentHugePageDefrag:  "defer+madvise",
					SwapFileSizeMB:             &swapFileSizeMB,
				}
			}, func(o *nodeBootstrappingOutput) {
				sysctlContent, err := getBase64DecodedValue([]byte(o.vars["SYSCTL_CONTENT"]))
				Expect(err).To(BeNil())
				// assert defaults for gc_thresh2 and gc_thresh3
				// assert custom values for all others.
				Expect(sysctlContent).To(ContainSubstring("net.core.somaxconn=1638499"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.tcp_max_syn_backlog=1638498"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh1=10001"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh2=8192"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh3=16384"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.ip_local_reserved_ports=65330"))
			}),
		Entry("AKSUbuntu2204 with k8s 1.31 and custom kubeletConfig and serializeImagePull flag", "AKSUbuntu2204+CustomKubeletConfig+SerializeImagePulls", "1.31.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableKubeletConfigFile = false
				failSwapOn := false
				config.KubeletConfig["--serialize-image-pulls"] = "false"
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = &datamodel.CustomKubeletConfig{
					CPUManagerPolicy:      "static",
					CPUCfsQuota:           to.BoolPtr(false),
					CPUCfsQuotaPeriod:     "200ms",
					ImageGcHighThreshold:  to.Int32Ptr(90),
					ImageGcLowThreshold:   to.Int32Ptr(70),
					TopologyManagerPolicy: "best-effort",
					AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
					FailSwapOn:            &failSwapOn,
					ContainerLogMaxSizeMB: to.Int32Ptr(1000),
					ContainerLogMaxFiles:  to.Int32Ptr(99),
					PodMaxPids:            to.Int32Ptr(12345),
				}
			}, func(o *nodeBootstrappingOutput) {
				kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
				Expect(err).To(BeNil())
				Expect(kubeletConfigFileContent).To(ContainSubstring(`"serializeImagePulls": false`))
			}),
		Entry("AKSUbuntu2204 with SecurityProfile", "AKSUbuntu2204+SecurityProfile", "1.26.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ProxyAddress:            "https://test-pe-proxy",
						ContainerRegistryServer: "testserver.azurecr.io",
					},
				}
			}, func(o *nodeBootstrappingOutput) {
				containerdConfigFileContent := o.files["/etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"].value
				Expect(strings.Contains(containerdConfigFileContent, "[host.\"https://testserver.azurecr.io\"]")).To(BeTrue())
				Expect(strings.Contains(containerdConfigFileContent, "capabilities = [\"pull\", \"resolve\"]")).To(BeTrue())
			}),
		Entry("AKSUbuntu2204 IMDSRestriction with enable restriction and insert to mangle table", "AKSUbuntu2204+IMDSRestrictionOnWithMangleTable", "1.24.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableIMDSRestriction = true
				config.InsertIMDSRestrictionRuleToMangleTable = true
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_IMDS_RESTRICTION"]).To(Equal("true"))
				Expect(o.vars["INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE"]).To(Equal("true"))
			}),
		Entry("AKSUbuntu2204 IMDSRestriction with enable restriction and not insert to mangle table", "AKSUbuntu2204+IMDSRestrictionOnWithFilterTable", "1.24.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableIMDSRestriction = true
				config.InsertIMDSRestrictionRuleToMangleTable = false
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_IMDS_RESTRICTION"]).To(Equal("true"))
				Expect(o.vars["INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE"]).To(Equal("false"))
			}),
		Entry("AKSUbuntu2204 IMDSRestriction with disable restriction", "AKSUbuntu2204+IMDSRestrictionOff", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableIMDSRestriction = false
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["ENABLE_IMDS_RESTRICTION"]).To(Equal("false"))
			Expect(o.vars["INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE"]).To(Equal("false"))
		}),
	)
})

var _ = Describe("Assert generated customData and cseCmd for Windows", func() {
	DescribeTable("Generated customData and CSE", func(folder, k8sVersion string, configUpdator func(*datamodel.NodeBootstrappingConfiguration)) {
		cs := &datamodel.ContainerService{
			Location: "southcentralus",
			Type:     "Microsoft.ContainerService/ManagedClusters",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    datamodel.Kubernetes,
					OrchestratorVersion: k8sVersion,
					KubernetesConfig: &datamodel.KubernetesConfig{
						ContainerRuntime:     "docker",
						KubernetesImageBase:  "mcr.microsoft.com/oss/kubernetes/",
						WindowsContainerdURL: "https://k8swin.blob.core.windows.net/k8s-windows/containerd/containerplat-aks-test-0.0.8.zip",
						LoadBalancerSku:      "Standard",
						CustomHyperkubeImage: "mcr.microsoft.com/oss/kubernetes/hyperkube:v1.16.15-hotfix.20200903",
						ClusterSubnet:        "10.240.0.0/16",
						NetworkPlugin:        "azure",
						DockerBridgeSubnet:   "172.17.0.1/16",
						ServiceCIDR:          "10.0.0.0/16",
						EnableRbac:           to.BoolPtr(true),
						EnableSecureKubelet:  to.BoolPtr(true),
						UseInstanceMetadata:  to.BoolPtr(true),
						DNSServiceIP:         "10.0.0.10",
					},
				},
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					DNSPrefix:   "uttestdom",
					FQDN:        "uttestdom-dns-5d7c849e.hcp.southcentralus.azmk8s.io",
					Subnet:      "10.240.0.0/16",
					IPMasqAgent: true,
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{
					{
						Name:                "wpool2",
						VMSize:              "Standard_D2s_v3",
						StorageProfile:      "ManagedDisks",
						OSType:              datamodel.Windows,
						VnetSubnetID:        "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-36873793/subnet/aks-subnet",
						WindowsNameVersion:  "v2",
						AvailabilityProfile: datamodel.VirtualMachineScaleSets,
						CustomNodeLabels:    map[string]string{"kubernetes.azure.com/node-image-version": "AKSWindows-2019-17763.1577.201111"},
						Distro:              datamodel.Distro("aks-windows-2019"),
					},
				},
				LinuxProfile: &datamodel.LinuxProfile{
					AdminUsername: "azureuser",
				},
				WindowsProfile: &datamodel.WindowsProfile{
					ProvisioningScriptsPackageURL: "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.4.zip",
					WindowsPauseImageURL:          "mcr.microsoft.com/oss/kubernetes/pause:1.4.0",
					AdminUsername:                 "azureuser",
					AdminPassword:                 "replacepassword1234",
					WindowsPublisher:              "microsoft-aks",
					WindowsOffer:                  "aks-windows",
					ImageVersion:                  "17763.1577.201111",
					WindowsSku:                    "aks-2019-datacenter-core-smalldisk-2011",
				},
				ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
					ClientID: "ClientID",
					Secret:   "Secret",
				},
				FeatureFlags: &datamodel.FeatureFlags{
					EnableWinDSR: false,
				},
			},
		}
		cs.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{
			KeyData: string("testsshkey"),
		}}

		// AKS always pass in te customHyperKubeImage to aks-e, so we don't really rely on
		// the default component version for "hyperkube", which is not set since 1.17
		if IsKubernetesVersionGe(k8sVersion, "1.17.0") {
			cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage = fmt.Sprintf("k8s.gcr.io/hyperkube-amd64:v%v", k8sVersion)
		}

		// WinDSR is only supported since 1.19
		if IsKubernetesVersionGe(k8sVersion, "1.19.0") {
			cs.Properties.FeatureFlags.EnableWinDSR = true
		}

		agentPool := cs.Properties.AgentPoolProfiles[0]

		fullK8sComponentsMap := K8sComponentsByVersionMap[cs.Properties.OrchestratorProfile.OrchestratorVersion]
		pauseImage := cs.Properties.OrchestratorProfile.KubernetesConfig.MCRKubernetesImageBase + fullK8sComponentsMap["pause"]

		hyperkubeImageBase := cs.Properties.OrchestratorProfile.KubernetesConfig.KubernetesImageBase
		hyperkubeImage := hyperkubeImageBase + fullK8sComponentsMap["hyperkube"]
		if cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage != "" {
			hyperkubeImage = cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage
		}

		windowsPackage := datamodel.AzurePublicCloudSpecForTest.KubernetesSpecConfig.KubeBinariesSASURLBase + fullK8sComponentsMap["windowszip"]
		k8sComponents := &datamodel.K8sComponents{
			PodInfraContainerImageURL: pauseImage,
			HyperkubeImageURL:         hyperkubeImage,
			WindowsPackageURL:         windowsPackage,
		}

		if IsKubernetesVersionGe(k8sVersion, "1.29.0") {
			// This is test only, credential provider version does not align with k8s version
			k8sComponents.WindowsCredentialProviderURL = fmt.Sprintf("https://acs-mirror.azureedge.net/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-windows-amd64-v%s.tar.gz", k8sVersion, k8sVersion) //nolint:lll
			k8sComponents.LinuxCredentialProviderURL = fmt.Sprintf("https://acs-mirror.azureedge.net/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-linux-amd64-v%s.tar.gz", k8sVersion, k8sVersion)     //nolint:lll
		}

		kubeletConfig := map[string]string{
			"--address":                           "0.0.0.0",
			"--anonymous-auth":                    "false",
			"--authentication-token-webhook":      "true",
			"--authorization-mode":                "Webhook",
			"--cloud-config":                      "c:\\k\\azure.json",
			"--cgroups-per-qos":                   "false",
			"--client-ca-file":                    "c:\\k\\ca.crt",
			"--azure-container-registry-config":   "c:\\k\\azure.json",
			"--cloud-provider":                    "azure",
			"--cluster-dns":                       "10.0.0.10",
			"--cluster-domain":                    "cluster.local",
			"--enforce-node-allocatable":          "",
			"--event-qps":                         "0",
			"--eviction-hard":                     "",
			"--feature-gates":                     "RotateKubeletServerCertificate=true",
			"--hairpin-mode":                      "promiscuous-bridge",
			"--image-gc-high-threshold":           "85",
			"--image-gc-low-threshold":            "80",
			"--keep-terminated-pod-volumes":       "false",
			"--kube-reserved":                     "cpu=100m,memory=1843Mi",
			"--kubeconfig":                        "c:\\k\\config",
			"--max-pods":                          "30",
			"--network-plugin":                    "cni",
			"--node-status-update-frequency":      "10s",
			"--pod-infra-container-image":         "mcr.microsoft.com/oss/kubernetes/pause:3.9",
			"--pod-max-pids":                      "-1",
			"--read-only-port":                    "0",
			"--resolv-conf":                       `""`,
			"--rotate-certificates":               "false",
			"--streaming-connection-idle-timeout": "4h",
			"--system-reserved":                   "memory=2Gi",
			"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint:lll
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
			KubeletConfig:                 kubeletConfig,
			PrimaryScaleSetName:           "akswpool2",
			SIGConfig: datamodel.SIGConfig{
				TenantID:       "tenantID",
				SubscriptionID: "subID",
				Galleries: map[string]datamodel.SIGGalleryConfig{
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
						GalleryName:   "AKSWindows",
						ResourceGroup: "AKS-Windows",
					},
					"AKSUbuntuEdgeZone": {
						GalleryName:   "AKSUbuntuEdgeZone",
						ResourceGroup: "AKS-Ubuntu-EdgeZone",
					},
				},
			},
		}

		if configUpdator != nil {
			configUpdator(config)
		}

		// customData
		ab, err := NewAgentBaker()
		Expect(err).To(BeNil())
		nodeBootstrapping, err := ab.GetNodeBootstrapping(context.Background(), config)
		Expect(err).To(BeNil())
		base64EncodedCustomData := nodeBootstrapping.CustomData
		customDataBytes, err := base64.StdEncoding.DecodeString(base64EncodedCustomData)
		customData := string(customDataBytes)
		Expect(err).To(BeNil())

		if generateTestData() {
			backfillCustomData(folder, customData)
		}

		expectedCustomData, err := os.ReadFile(fmt.Sprintf("./testdata/%s/CustomData", folder))
		if err != nil {
			panic(err)
		}
		Expect(customData).To(Equal(string(expectedCustomData)))

		// CSE
		ab, err = NewAgentBaker()
		Expect(err).To(BeNil())
		nodeBootstrapping, err = ab.GetNodeBootstrapping(context.Background(), config)
		Expect(err).To(BeNil())
		cseCommand := nodeBootstrapping.CSE

		if generateTestData() {
			err = os.WriteFile(fmt.Sprintf("./testdata/%s/CSECommand", folder), []byte(cseCommand), 0644)
			Expect(err).To(BeNil())
		}

		expectedCSECommand, err := os.ReadFile(fmt.Sprintf("./testdata/%s/CSECommand", folder))
		if err != nil {
			panic(err)
		}
		Expect(cseCommand).To(Equal(string(expectedCSECommand)))

	}, Entry("AKSWindows2019 with k8s version 1.16", "AKSWindows2019+K8S116", "1.16.15", func(config *datamodel.NodeBootstrappingConfiguration) {
	}),
		Entry("AKSWindows2019 with k8s version 1.17", "AKSWindows2019+K8S117", "1.17.7", func(config *datamodel.NodeBootstrappingConfiguration) {
		}),
		Entry("AKSWindows2019 with k8s version 1.18", "AKSWindows2019+K8S118", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
		}),
		Entry("AKSWindows2019 with k8s version 1.19", "AKSWindows2019+K8S119", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
		}),
		Entry("AKSWindows2019 with k8s version 1.19 + CSI", "AKSWindows2019+K8S119+CSI", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.WindowsProfile.CSIProxyURL = "https://acs-mirror.azureedge.net/csi-proxy/v0.1.0/binaries/csi-proxy.tar.gz"
			config.ContainerService.Properties.WindowsProfile.EnableCSIProxy = to.BoolPtr(true)
		}),
		Entry("AKSWindows2019 with CustomVnet", "AKSWindows2019+CustomVnet", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet = "172.17.0.0/24"
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.ServiceCIDR = "172.17.255.0/24"
			config.ContainerService.Properties.AgentPoolProfiles[0].VnetCidrs = []string{"172.17.0.0/16"}
			config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID = "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet2" //nolint:lll
			config.KubeletConfig["--cluster-dns"] = "172.17.255.10"
		}),
		Entry("AKSWindows2019 with Managed Identity", "AKSWindows2019+ManagedIdentity", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.ServicePrincipalProfile = &datamodel.ServicePrincipalProfile{ClientID: "msi"}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UserAssignedID = "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/k8s-agentpool" //nolint:lll
		}),
		Entry("AKSWindows2019 with custom cloud", "AKSWindows2019+CustomCloud", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = to.BoolPtr(true)
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name:                         "akscustom",
				McrURL:                       "mcr.microsoft.fakecustomcloud",
				RepoDepotEndpoint:            "https://repodepot.azure.microsoft.fakecustomcloud/ubuntu",
				ManagementPortalURL:          "https://portal.azure.microsoft.fakecustomcloud/",
				PublishSettingsURL:           "",
				ServiceManagementEndpoint:    "https://management.core.microsoft.fakecustomcloud/",
				ResourceManagerEndpoint:      "https://management.azure.microsoft.fakecustomcloud/",
				ActiveDirectoryEndpoint:      "https://login.microsoftonline.microsoft.fakecustomcloud/",
				GalleryEndpoint:              "",
				KeyVaultEndpoint:             "https://vault.cloudapi.microsoft.fakecustomcloud/",
				GraphEndpoint:                "https://graph.cloudapi.microsoft.fakecustomcloud/",
				ServiceBusEndpoint:           "",
				BatchManagementEndpoint:      "",
				StorageEndpointSuffix:        "core.microsoft.fakecustomcloud",
				SQLDatabaseDNSSuffix:         "database.cloudapi.microsoft.fakecustomcloud",
				TrafficManagerDNSSuffix:      "",
				KeyVaultDNSSuffix:            "vault.cloudapi.microsoft.fakecustomcloud",
				ServiceBusEndpointSuffix:     "",
				ServiceManagementVMDNSSuffix: "",
				ResourceManagerVMDNSSuffix:   "cloudapp.azure.microsoft.fakecustomcloud/",
				ContainerRegistryDNSSuffix:   ".azurecr.microsoft.fakecustomcloud",
				CosmosDBDNSSuffix:            "documents.core.microsoft.fakecustomcloud/",
				TokenAudience:                "https://management.core.microsoft.fakecustomcloud/",
				ResourceIdentifiers: datamodel.ResourceIdentifiers{
					Graph:               "",
					KeyVault:            "",
					Datalake:            "",
					Batch:               "",
					OperationalInsights: "",
					Storage:             "",
				},
			}
		}),
		Entry("AKSWindows2019 EnablePrivateClusterHostsConfigAgent", "AKSWindows2019+EnablePrivateClusterHostsConfigAgent", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				cs := config.ContainerService
				if cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster == nil {
					cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster = &datamodel.PrivateCluster{EnableHostsConfigAgent: to.BoolPtr(true)}
				} else {
					cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.EnableHostsConfigAgent = to.BoolPtr(true)
				}
			}),
		Entry("AKSWindows2019 with kubelet client TLS bootstrapping enabled", "AKSWindows2019+KubeletClientTLSBootstrapping", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletClientTLSBootstrapToken = to.StringPtr("07401b.f395accd246ae52d")
			}),
		Entry("AKSWindows2019 with kubelet serving certificate rotation enabled", "AKSWindows2019+KubeletServingCertificateRotation", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletConfig["--rotate-server-certificates"] = "true"
			}),
		Entry("AKSWindows2019 with k8s version 1.19 + FIPS", "AKSWindows2019+K8S119+FIPS", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.FIPSEnabled = true
			}),
		Entry("AKSWindows2019 with SecurityProfile", "AKSWindows2019+SecurityProfile", "1.26.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:      true,
						ProxyAddress: "https://test-pe-proxy",
					},
				}
			}),
		Entry("AKSWindows2019 with out of tree credential provider", "AKSWindows2019+ootcredentialprovider", "1.29.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = to.BoolPtr(true)
			config.KubeletConfig["--image-credential-provider-config"] = "c:\\var\\lib\\kubelet\\credential-provider-config.yaml"
			config.KubeletConfig["--image-credential-provider-bin-dir"] = "c:\\var\\lib\\kubelet\\credential-provider"
		}),
		Entry("AKSWindows2019 with custom cloud and out of tree credential provider", "AKSWindows2019+CustomCloud+ootcredentialprovider", "1.29.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = to.BoolPtr(true)
				config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
					Name:                         "akscustom",
					McrURL:                       "mcr.microsoft.fakecustomcloud",
					RepoDepotEndpoint:            "https://repodepot.azure.microsoft.fakecustomcloud/ubuntu",
					ManagementPortalURL:          "https://portal.azure.microsoft.fakecustomcloud/",
					PublishSettingsURL:           "",
					ServiceManagementEndpoint:    "https://management.core.microsoft.fakecustomcloud/",
					ResourceManagerEndpoint:      "https://management.azure.microsoft.fakecustomcloud/",
					ActiveDirectoryEndpoint:      "https://login.microsoftonline.microsoft.fakecustomcloud/",
					GalleryEndpoint:              "",
					KeyVaultEndpoint:             "https://vault.cloudapi.microsoft.fakecustomcloud/",
					GraphEndpoint:                "https://graph.cloudapi.microsoft.fakecustomcloud/",
					ServiceBusEndpoint:           "",
					BatchManagementEndpoint:      "",
					StorageEndpointSuffix:        "core.microsoft.fakecustomcloud",
					SQLDatabaseDNSSuffix:         "database.cloudapi.microsoft.fakecustomcloud",
					TrafficManagerDNSSuffix:      "",
					KeyVaultDNSSuffix:            "vault.cloudapi.microsoft.fakecustomcloud",
					ServiceBusEndpointSuffix:     "",
					ServiceManagementVMDNSSuffix: "",
					ResourceManagerVMDNSSuffix:   "cloudapp.azure.microsoft.fakecustomcloud/",
					ContainerRegistryDNSSuffix:   ".azurecr.microsoft.fakecustomcloud",
					CosmosDBDNSSuffix:            "documents.core.microsoft.fakecustomcloud/",
					TokenAudience:                "https://management.core.microsoft.fakecustomcloud/",
					ResourceIdentifiers: datamodel.ResourceIdentifiers{
						Graph:               "",
						KeyVault:            "",
						Datalake:            "",
						Batch:               "",
						OperationalInsights: "",
						Storage:             "",
					},
				}
				config.KubeletConfig["--image-credential-provider-config"] = "c:\\var\\lib\\kubelet\\credential-provider-config.yaml"
				config.KubeletConfig["--image-credential-provider-bin-dir"] = "c:\\var\\lib\\kubelet\\credential-provider"
			}),
	)

})

func backfillCustomData(folder, customData string) {
	if _, err := os.Stat(fmt.Sprintf("./testdata/%s", folder)); os.IsNotExist(err) {
		e := os.MkdirAll(fmt.Sprintf("./testdata/%s", folder), 0755)
		Expect(e).To(BeNil())
	}
	writeFileError := os.WriteFile(fmt.Sprintf("./testdata/%s/CustomData", folder), []byte(customData), 0644)
	Expect(writeFileError).To(BeNil())
	if strings.Contains(folder, "AKSWindows") {
		return
	}
	err := exec.Command("/bin/sh", "-c", fmt.Sprintf("./testdata/convert.sh testdata/%s", folder)).Run()
	Expect(err).To(BeNil())
}

func getDecodedVarsFromCseCmd(data []byte) (map[string]string, error) {
	cseRegex := regexp.MustCompile(cseRegexString)
	cseVariableList := cseRegex.FindAllStringSubmatch(string(data), -1)
	vars := make(map[string]string)

	for _, cseVar := range cseVariableList {
		if len(cseVar) < 3 {
			return nil, fmt.Errorf("expected 3 results (match, key, value) from regex, found %d, result %q", len(cseVar), cseVar)
		}

		key := cseVar[1]
		val := getValueWithoutQuotes(cseVar[2])

		vars[key] = val
	}

	return vars, nil
}

func getValueWithoutQuotes(value string) string {
	if len(value) > 1 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	return value
}

//lint:ignore U1000 this is used for test helpers in the future
func getGzipDecodedValue(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}

	output, err := io.ReadAll(gzipReader)
	if err != nil {
		return "", fmt.Errorf("read from gzipped buffered string: %w", err)
	}

	return string(output), nil
}

func getBase64DecodedValue(data []byte) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

func verifyCertsEncoding(cert string) error {
	certPEM, err := base64.StdEncoding.DecodeString(cert)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return errors.New("pem decode block is nil")
	}

	_, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	return nil
}

func getDecodedFilesFromCustomdata(data []byte) (map[string]*decodedValue, error) {
	var customData cloudInit

	if err := yaml.Unmarshal(data, &customData); err != nil {
		return nil, err
	}

	var files = make(map[string]*decodedValue)

	for _, val := range customData.WriteFiles {
		var encoding cseVariableEncoding
		maybeEncodedValue := val.Content

		if strings.Contains(val.Encoding, "gzip") {
			if maybeEncodedValue != "" {
				output, err := getGzipDecodedValue([]byte(maybeEncodedValue))
				if err != nil {
					return nil, fmt.Errorf("failed to decode gzip value: %q with error %w", maybeEncodedValue, err)
				}
				maybeEncodedValue = output
				encoding = cseVariableEncodingGzip
			}
		}

		files[val.Path] = &decodedValue{
			value:    maybeEncodedValue,
			encoding: encoding,
		}
	}

	return files, nil
}

type cloudInit struct {
	WriteFiles []struct {
		Path        string `yaml:"path"`
		Permissions string `yaml:"permissions"`
		Encoding    string `yaml:"encoding,omitempty"`
		Owner       string `yaml:"owner"`
		Content     string `yaml:"content"`
	} `yaml:"write_files"`
}

var _ = Describe("Test normalizeResourceGroupNameForLabel", func() {
	It("should return the correct normalized resource group name", func() {
		Expect(normalizeResourceGroupNameForLabel("hello")).To(Equal("hello"))
		Expect(normalizeResourceGroupNameForLabel("hel(lo")).To(Equal("hel-lo"))
		Expect(normalizeResourceGroupNameForLabel("hel)lo")).To(Equal("hel-lo"))
		var s string
		for i := 0; i < 63; i++ {
			s += "0"
		}
		Expect(normalizeResourceGroupNameForLabel(s)).To(Equal(s))
		Expect(normalizeResourceGroupNameForLabel(s + "1")).To(Equal(s))

		s = ""
		for i := 0; i < 62; i++ {
			s += "0"
		}
		Expect(normalizeResourceGroupNameForLabel(s + "(")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel(s + ")")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel(s + "-")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel(s + "_")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel(s + ".")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel("")).To(Equal(""))
		Expect(normalizeResourceGroupNameForLabel("z")).To(Equal("z"))

		// Add z, not replacing ending - with z, if name is short
		Expect(normalizeResourceGroupNameForLabel("-")).To(Equal("-z"))

		s = ""
		for i := 0; i < 61; i++ {
			s += "0"
		}
		Expect(normalizeResourceGroupNameForLabel(s + "-")).To(Equal(s + "-z"))
	})
})
