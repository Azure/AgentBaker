package agent

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/common"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Assert generated customData and cseCmd", func() {
	DescribeTable("Generated customData and CSE", func(folder, k8sVersion string, containerServiceUpdator func(*api.ContainerService)) {
		cs := &api.ContainerService{
			Location: "southcentralus",
			Type:     "Microsoft.ContainerService/ManagedClusters",
			Properties: &api.Properties{
				OrchestratorProfile: &api.OrchestratorProfile{
					OrchestratorType:    api.Kubernetes,
					OrchestratorVersion: k8sVersion,
					KubernetesConfig: &api.KubernetesConfig{
						KubeletConfig: map[string]string{
							"--feature-gates": "RotateKubeletServerCertificate=true,a=b, PodPriority=true, x=y",
						},
					},
				},
				HostedMasterProfile: &api.HostedMasterProfile{
					DNSPrefix: "uttestdom",
				},
				AgentPoolProfiles: []*api.AgentPoolProfile{
					{
						Name:                "agent2",
						Count:               3,
						VMSize:              "Standard_DS1_v2",
						StorageProfile:      "ManagedDisks",
						OSType:              api.Linux,
						VnetSubnetID:        "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet1",
						AvailabilityProfile: api.VirtualMachineScaleSets,
						KubernetesConfig: &api.KubernetesConfig{
							KubeletConfig: map[string]string{
								"--address":                           "0.0.0.0",
								"--pod-manifest-path":                 "/etc/kubernetes/manifests",
								"--cluster-domain":                    "cluster.local",
								"--cluster-dns":                       "10.0.0.10",
								"--cgroups-per-qos":                   "true",
								"--tls-cert-file":                     "/etc/kubernetes/certs/kubeletserver.crt",
								"--tls-private-key-file":              "/etc/kubernetes/certs/kubeletserver.key",
								"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256",
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
							},
						},
						Distro: api.AKSUbuntu1604,
					},
				},
				LinuxProfile: &api.LinuxProfile{
					AdminUsername: "azureuser",
				},
				ServicePrincipalProfile: &api.ServicePrincipalProfile{
					ClientID: "ClientID",
					Secret:   "Secret",
				},
			},
		}
		cs.Properties.LinuxProfile.SSH.PublicKeys = []api.PublicKey{{
			KeyData: string("testsshkey"),
		}}

		// AKS always pass in te customHyperKubeImage to aks-e, so we don't really rely on
		// the default component version for "hyperkube", which is not set since 1.17
		if IsKubernetesVersionGe(k8sVersion, "1.17.0") {
			cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage = fmt.Sprintf("k8s.gcr.io/hyperkube-amd64:v%v", k8sVersion)
		}

		if containerServiceUpdator != nil {
			containerServiceUpdator(cs)
		}

		agentPool := cs.Properties.AgentPoolProfiles[0]
		baker := InitializeTemplateGenerator()

		// customData
		customData := baker.GetNodeBootstrappingPayload(cs, agentPool)
		// Uncomment below line to generate test data in local if agentbaker is changed in generating customData
		// backfillCustomData(folder, customData)
		expectedCustomData, err := ioutil.ReadFile(fmt.Sprintf("./testdata/%s/CustomData", folder))
		if err != nil {
			panic(err)
		}
		Expect(customData).To(Equal(string(expectedCustomData)))

		// CSE
		cseCommand := baker.GetNodeBootstrappingCmd(cs, agentPool,
			"tenantID", "subID", "resourceGroupName", "userAssignedID", true, true)
		// Uncomment below line to generate test data in local if agentbaker is changed in generating customData
		// ioutil.WriteFile(fmt.Sprintf("./testdata/%s/CSECommand", folder), []byte(cseCommand), 0644)
		expectedCSECommand, err := ioutil.ReadFile(fmt.Sprintf("./testdata/%s/CSECommand", folder))
		if err != nil {
			panic(err)
		}
		Expect(cseCommand).To(Equal(string(expectedCSECommand)))

	}, Entry("AKSUbuntu1604 with k8s version less than 1.18", "AKSUbuntu1604+K8S115", "1.15.7", nil),
		Entry("AKSUbuntu1604 with k8s version 1.18", "AKSUbuntu1604+K8S118", "1.18.2", nil),
		Entry("AKSUbuntu1604 with k8s version 1.17", "AKSUbuntu1604+K8S117", "1.17.7", nil),
		Entry("AKSUbuntu1604 with Temp Disk", "AKSUbuntu1604+TempDisk", "1.15.7", func(cs *api.ContainerService) {
			cs.Properties.OrchestratorProfile.KubernetesConfig = &api.KubernetesConfig{
				ContainerRuntimeConfig: map[string]string{
					common.ContainerDataDirKey: "/mnt/containers",
				},
			}
		}),
		Entry("AKSUbuntu1604 with Temp Disk and containerd", "AKSUbuntu1604+TempDisk+Containerd", "1.15.7", func(cs *api.ContainerService) {
			cs.Properties.OrchestratorProfile.KubernetesConfig = &api.KubernetesConfig{
				ContainerRuntimeConfig: map[string]string{
					common.ContainerDataDirKey: "/mnt/containers",
				},
			}
			cs.Properties.AgentPoolProfiles[0].KubernetesConfig = &api.KubernetesConfig{
				KubeletConfig:    map[string]string{},
				ContainerRuntime: api.Containerd,
			}
		}),
		Entry("AKSUbuntu1604 with RawUbuntu", "RawUbuntu", "1.15.7", func(cs *api.ContainerService) {
			// cs.Properties.OrchestratorProfile.KubernetesConfig = nil
			cs.Properties.AgentPoolProfiles[0].Distro = api.Ubuntu
		}),
		Entry("AKSUbuntu1604 with AKSCustomCloud", "AKSUbuntu1604+AKSCustomCloud", "1.15.7", func(cs *api.ContainerService) {
			// cs.Properties.OrchestratorProfile.KubernetesConfig = nil
			cs.Location = "usnat"
		}),
		Entry("AKSUbuntu1604 EnableHostsConfigAgent", "AKSUbuntu1604+EnableHostsConfigAgent", "1.18.2", func(cs *api.ContainerService) {
			if cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster == nil {
				cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster = &api.PrivateCluster{EnableHostsConfigAgent: to.BoolPtr(true)}
			} else {
				cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.EnableHostsConfigAgent = to.BoolPtr(true)
			}
		}))
})

func backfillCustomData(folder, customData string) {
	if _, err := os.Stat(fmt.Sprintf("./testdata/%s", folder)); os.IsNotExist(err) {
		e := os.MkdirAll(fmt.Sprintf("./testdata/%s", folder), 0755)
		Expect(e).To(BeNil())
	}
	ioutil.WriteFile(fmt.Sprintf("./testdata/%s/CustomData", folder), []byte(customData), 0644)
	err := exec.Command("/bin/sh", "-c", fmt.Sprintf("./testdata/convert.sh testdata/%s", folder)).Run()
	Expect(err).To(BeNil())
}
