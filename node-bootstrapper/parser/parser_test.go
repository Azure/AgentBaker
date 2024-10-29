package parser_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"testing"

	"github.com/Azure/agentbaker/node-bootstrapper/parser"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nodeBootstrappingOutput struct {
	cseCmd string
	vars   map[string]string
}

type outputValidator func(*nodeBootstrappingOutput)

// this regex looks for groups of the following forms, returning KEY and VALUE as submatches.
/* - KEY=VALUE
- KEY="VALUE"
- KEY=
- KEY="VALUE WITH WHITSPACE". */
const cseRegexString = `([^=\s]+)=(\"[^\"]*\"|[^\s]*)`

// test certificate.
const encodedTestCert = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUgvVENDQmVXZ0F3SUJBZ0lRYUJZRTMvTTA4WEhZQ25OVm1jRkJjakFOQmdrcWhraUc5dzBCQVFzRkFEQnkKTVFzd0NRWURWUVFHRXdKVlV6RU9NQXdHQTFVRUNBd0ZWR1Y0WVhNeEVEQU9CZ05WQkFjTUIwaHZkWE4wYjI0eApFVEFQQmdOVkJBb01DRk5UVENCRGIzSndNUzR3TEFZRFZRUUREQ1ZUVTB3dVkyOXRJRVZXSUZOVFRDQkpiblJsCmNtMWxaR2xoZEdVZ1EwRWdVbE5CSUZJek1CNFhEVEl3TURRd01UQXdOVGd6TTFvWERUSXhNRGN4TmpBd05UZ3oKTTFvd2diMHhDekFKQmdOVkJBWVRBbFZUTVE0d0RBWURWUVFJREFWVVpYaGhjekVRTUE0R0ExVUVCd3dIU0c5MQpjM1J2YmpFUk1BOEdBMVVFQ2d3SVUxTk1JRU52Y25BeEZqQVVCZ05WQkFVVERVNVdNakF3T0RFMk1UUXlORE14CkZEQVNCZ05WQkFNTUMzZDNkeTV6YzJ3dVkyOXRNUjB3R3dZRFZRUVBEQlJRY21sMllYUmxJRTl5WjJGdWFYcGgKZEdsdmJqRVhNQlVHQ3lzR0FRUUJnamM4QWdFQ0RBWk9aWFpoWkdFeEV6QVJCZ3NyQmdFRUFZSTNQQUlCQXhNQwpWVk13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRREhoZVJrYmIxRkNjN3hSS3N0CndLMEpJR2FLWTh0N0piUzJiUTJiNllJSkRnbkh1SVlIcUJyQ1VWNzlvZWxpa2tva1JrRnZjdnBhS2luRkhEUUgKVXBXRUk2UlVFUlltU0NnM084V2k0MnVPY1YyQjVaYWJtWENrd2R4WTVFY2w1MUJiTThVbkdkb0FHYmRObWlSbQpTbVRqY3MrbGhNeGc0ZkZZNmxCcGlFVkZpR1VqR1JSKzYxUjY3THo2VTRLSmVMTmNDbTA3UXdGWUtCbXBpMDhnCmR5Z1N2UmRVdzU1Sm9wcmVkaitWR3RqVWtCNGhGVDRHUVgvZ2h0NjlSbHF6Lys4dTBkRVFraHVVdXVjcnFhbG0KU0d5NDNIUndCZkRLRndZZVdNN0NQTWQ1ZS9kTyt0MDh0OFBianpWVFR2NWhRRENzRVlJVjJUN0FGSTlTY054TQpraDcvQWdNQkFBR2pnZ05CTUlJRFBUQWZCZ05WSFNNRUdEQVdnQlMvd1ZxSC95ajZRVDM5dDAva0hhK2dZVmdwCnZUQi9CZ2dyQmdFRkJRY0JBUVJ6TUhFd1RRWUlLd1lCQlFVSE1BS0dRV2gwZEhBNkx5OTNkM2N1YzNOc0xtTnYKYlM5eVpYQnZjMmwwYjNKNUwxTlRUR052YlMxVGRXSkRRUzFGVmkxVFUwd3RVbE5CTFRRd09UWXRVak11WTNKMApNQ0FHQ0NzR0FRVUZCekFCaGhSb2RIUndPaTh2YjJOemNITXVjM05zTG1OdmJUQWZCZ05WSFJFRUdEQVdnZ3QzCmQzY3VjM05zTG1OdmJZSUhjM05zTG1OdmJUQmZCZ05WSFNBRVdEQldNQWNHQldlQkRBRUJNQTBHQ3lxRWFBR0cKOW5jQ0JRRUJNRHdHRENzR0FRUUJncWt3QVFNQkJEQXNNQ29HQ0NzR0FRVUZCd0lCRmg1b2RIUndjem92TDNkMwpkeTV6YzJ3dVkyOXRMM0psY0c5emFYUnZjbmt3SFFZRFZSMGxCQll3RkFZSUt3WUJCUVVIQXdJR0NDc0dBUVVGCkJ3TUJNRWdHQTFVZEh3UkJNRDh3UGFBN29EbUdOMmgwZEhBNkx5OWpjbXh6TG5OemJDNWpiMjB2VTFOTVkyOXQKTFZOMVlrTkJMVVZXTFZOVFRDMVNVMEV0TkRBNU5pMVNNeTVqY213d0hRWURWUjBPQkJZRUZBREFGVUlhenc1cgpaSUhhcG5SeElVbnB3K0dMTUE0R0ExVWREd0VCL3dRRUF3SUZvRENDQVgwR0Npc0dBUVFCMW5rQ0JBSUVnZ0Z0CkJJSUJhUUZuQUhjQTlseVVMOUYzTUNJVVZCZ0lNSlJXanVOTkV4a3p2OThNTHlBTHpFN3haT01BQUFGeE0waG8KYndBQUJBTUFTREJHQWlFQTZ4ZWxpTlI4R2svNjNwWWRuUy92T3gvQ2pwdEVNRXY4OVdXaDEvdXJXSUVDSVFEeQpCcmVIVTI1RHp3dWtRYVJRandXNjU1WkxrcUNueGJ4UVdSaU9lbWo5SkFCMUFKUWd2QjZPMVkxc2lITWZnb3NpCkxBM1IyazFlYkUrVVBXSGJUaTlZVGFMQ0FBQUJjVE5JYU53QUFBUURBRVl3UkFJZ0dSRTR3emFiTlJkRDhrcS8KdkZQM3RRZTJobTB4NW5YdWxvd2g0SWJ3M2xrQ0lGWWIvM2xTRHBsUzdBY1I0citYcFd0RUtTVEZXSm1OQ1JiYwpYSnVyMlJHQkFIVUE3c0NWN28xeVpBK1M0OE81RzhjU28ybHFDWHRMYWhvVU9PWkhzc3Z0eGZrQUFBRnhNMGhvCjh3QUFCQU1BUmpCRUFpQjZJdmJvV3NzM1I0SXRWd2plYmw3RDN5b0ZhWDBORGgyZFdoaGd3Q3hySHdJZ0NmcTcKb2NNQzV0KzFqaTVNNXhhTG1QQzRJK1dYM0kvQVJrV1N5aU83SVFjd0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dJQgpBQ2V1dXI0UW51anFtZ3VTckhVM21oZitjSm9kelRRTnFvNHRkZStQRDEvZUZkWUFFTHU4eEYrMEF0N3hKaVBZCmk1Ukt3aWx5UDU2diszaVkyVDlsdzdTOFRKMDQxVkxoYUlLcDE0TXpTVXpSeWVvT0FzSjdRQURNQ2xIS1VEbEgKVVUycE51bzg4WTZpZ292VDNic253Sk5pRVFOcXltU1NZaGt0dzB0YWR1b3FqcVhuMDZnc1Zpb1dUVkRYeXNkNQpxRXg0dDZzSWdJY01tMjZZSDF2SnBDUUVoS3BjMnkwN2dSa2tsQlpSdE1qVGh2NGNYeXlNWDd1VGNkVDdBSkJQCnVlaWZDb1YyNUp4WHVvOGQ1MTM5Z3dQMUJBZTdJQlZQeDJ1N0tOL1V5T1hkWm13TWYvVG1GR3dEZENmc3lIZi8KWnNCMndMSG96VFlvQVZtUTlGb1UxSkxnY1ZpdnFKK3ZObEJoSFhobHhNZE4wajgwUjlOejZFSWdsUWplSzNPOApJL2NGR20vQjgrNDJoT2xDSWQ5WmR0bmRKY1JKVmppMHdEMHF3ZXZDYWZBOWpKbEh2L2pzRStJOVV6NmNwQ3loCnN3K2xyRmR4VWdxVTU4YXhxZUs4OUZSK05vNHEwSUlPK0ppMXJKS3I5bmtTQjBCcVhvelZuRTFZQi9LTHZkSXMKdVlaSnVxYjJwS2t1K3p6VDZnVXdIVVRadkJpTk90WEw0Tnh3Yy9LVDdXek9TZDJ3UDEwUUk4REtnNHZmaU5EcwpIV21CMWM0S2ppNmdPZ0E1dVNVemFHbXEvdjRWbmNLNVVyK245TGJmbmZMYzI4SjVmdC9Hb3Rpbk15RGszaWFyCkYxMFlscWNPbWVYMXVGbUtiZGkvWG9yR2xrQ29NRjNURHg4cm1wOURCaUIvCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=" //nolint:lll

func TestCustomDataAndCSE(t *testing.T) {
	tests := []struct {
		name       string
		folder     string
		k8sVersion string
		nbcUpdator func(*nbcontractv1.Configuration)
		validator  outputValidator
	}{
		{
			name:       "AKSUbuntu2204 containerd with multi-instance GPU",
			folder:     "AKSUbuntu2204+Containerd+MIG",
			k8sVersion: "1.19.13",
			nbcUpdator: func(nbc *nbcontractv1.Configuration) {
				nbc.GpuConfig.GpuInstanceProfile = "MIG7g"
				// Skip GPU driver install
				nbc.GpuConfig.EnableNvidia = to.BoolPtr(false)
				nbc.VmSize = "Standard_ND96asr_v4"
			},
			validator: func(o *nodeBootstrappingOutput) {
				assert.Equal(t, "false", o.vars["GPU_NODE"])
				assert.NotEmpty(t, o.vars["CONTAINERD_CONFIG_CONTENT"])
				// Ensure the containerd config does not use the
				// nvidia container runtime when skipping the
				// GPU driver install, since it will fail to run even non-GPU
				// pods, as it will not be installed.
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_CONTENT"]))
				require.NoError(t, err)
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
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
`
				require.Equal(t, expectedShimConfig, containerdConfigFileContent)
			},
		},
		{
			name:       "AKSUbuntu2204 DisableSSH with enabled ssh",
			folder:     "AKSUbuntu2204+SSHStatusOn",
			k8sVersion: "1.24.2",
			nbcUpdator: func(nbc *nbcontractv1.Configuration) {
				nbc.EnableSsh = to.BoolPtr(true)
			},
			validator: func(o *nodeBootstrappingOutput) {
				assert.Equal(t, "false", o.vars["DISABLE_SSH"])
			},
		},
		{
			name:       "AKSUbuntu2204 in China",
			folder:     "AKSUbuntu2204+China",
			k8sVersion: "1.24.2",
			nbcUpdator: func(nbc *nbcontractv1.Configuration) {
				nbc.ClusterConfig.Location = "chinaeast2"
				nbc.CustomCloudConfig.CustomCloudEnvName = "AzureChinaCloud"
			},
			validator: func(o *nodeBootstrappingOutput) {
				assert.Equal(t, "AzureChinaCloud", o.vars["TARGET_ENVIRONMENT"])
				assert.Equal(t, "AzureChinaCloud", o.vars["TARGET_CLOUD"])
				assert.Equal(t, "false", o.vars["IS_CUSTOM_CLOUD"])
			},
		},
		{
			name:       "AKSUbuntu2204 with custom cloud",
			folder:     "AKSUbuntu2204+CustomCloud",
			k8sVersion: "1.24.2",
			nbcUpdator: func(nbc *nbcontractv1.Configuration) {
				nbc.CustomCloudConfig.CustomCloudEnvName = nbcontractv1.AksCustomCloudName
			},
			validator: func(o *nodeBootstrappingOutput) {
				assert.Equal(t, nbcontractv1.AksCustomCloudName, o.vars["TARGET_ENVIRONMENT"])
				assert.Equal(t, nbcontractv1.AzureStackCloud, o.vars["TARGET_CLOUD"])
				assert.Equal(t, "true", o.vars["IS_CUSTOM_CLOUD"])
			},
		},
		{
			name:       "AKSUbuntu2204 with custom osConfig",
			folder:     "AKSUbuntu2204+CustomOSConfig",
			k8sVersion: "1.24.2",
			nbcUpdator: func(nbc *nbcontractv1.Configuration) {
				nbc.CustomLinuxOsConfig = &nbcontractv1.CustomLinuxOSConfig{
					EnableSwapConfig:           true,
					SwapFileSize:               int32(1500),
					TransparentHugepageSupport: "never",
					TransparentDefrag:          "defer+madvise",
					SysctlConfig: &nbcontractv1.SysctlConfig{
						NetCoreSomaxconn:             to.Int32Ptr(1638499),
						NetCoreRmemDefault:           to.Int32Ptr(456000),
						NetCoreWmemDefault:           to.Int32Ptr(89000),
						NetIpv4TcpTwReuse:            to.BoolPtr(true),
						NetIpv4IpLocalPortRange:      to.StringPtr("32768 65400"),
						NetIpv4TcpMaxSynBacklog:      to.Int32Ptr(1638498),
						NetIpv4NeighDefaultGcThresh1: to.Int32Ptr(10001),
					},
				}
			},
			validator: func(o *nodeBootstrappingOutput) {
				sysctlContent, err := getBase64DecodedValue([]byte(o.vars["SYSCTL_CONTENT"]))
				require.NoError(t, err)
				assert.Contains(t, sysctlContent, "net.core.somaxconn=1638499")
				assert.Contains(t, sysctlContent, "net.ipv4.tcp_max_syn_backlog=1638498")
				assert.Contains(t, sysctlContent, "net.ipv4.neigh.default.gc_thresh1=10001")
				assert.Contains(t, sysctlContent, "net.ipv4.neigh.default.gc_thresh2=8192")
				assert.Contains(t, sysctlContent, "net.ipv4.neigh.default.gc_thresh3=16384")
				assert.Contains(t, sysctlContent, "net.ipv4.ip_local_reserved_ports=65330")
			},
		},
		{
			name:       "AzureLinux v2 with kata and DisableUnattendedUpgrades=false",
			folder:     "AzureLinuxv2+Kata+DisableUnattendedUpgrades=false",
			k8sVersion: "1.28.0",
			nbcUpdator: func(nbc *nbcontractv1.Configuration) {
				nbc.IsKata = true
				nbc.EnableUnattendedUpgrade = true
				nbc.NeedsCgroupv2 = to.BoolPtr(true)
			},
			validator: func(o *nodeBootstrappingOutput) {
				assert.Equal(t, "true", o.vars["IS_KATA"])
				assert.Equal(t, "true", o.vars["ENABLE_UNATTENDED_UPGRADES"])
				assert.Equal(t, "true", o.vars["NEEDS_CGROUPV2"])
			},
		},
		{
			name:       "AKSUbuntu1804 with containerd and kubenet cni",
			folder:     "AKSUbuntu1804+Containerd+Kubenet",
			k8sVersion: "1.19.13",
			nbcUpdator: func(nbc *nbcontractv1.Configuration) {
				nbc.NetworkConfig.NetworkPlugin = nbcontractv1.GetNetworkPluginType(nbcontractv1.NetworkPluginKubenet)
			},
			validator: func(o *nodeBootstrappingOutput) {
				assert.NotEmpty(t, o.vars["CONTAINERD_CONFIG_CONTENT"])
				assert.Equal(t, "kubenet", o.vars["NETWORK_PLUGIN"])
			},
		},
		{
			name:       "AKSUbuntu1804 with http proxy config",
			folder:     "AKSUbuntu1804+HTTPProxy",
			k8sVersion: "1.18.14",
			nbcUpdator: func(nbc *nbcontractv1.Configuration) {
				nbc.HttpProxyConfig = &nbcontractv1.HTTPProxyConfig{
					HttpProxy:  "http://myproxy.server.com:8080/",
					HttpsProxy: "https://myproxy.server.com:8080/",
					NoProxyEntries: []string{
						"localhost",
						"127.0.0.1",
					},
					ProxyTrustedCa: encodedTestCert,
				}
			},
			validator: func(o *nodeBootstrappingOutput) {
				httpProxyStr := "export http_proxy=\"http://myproxy.server.com:8080/\""
				assert.Contains(t, o.cseCmd, httpProxyStr)
			},
		},
		{
			name:       "AKSUbuntu1804 with custom ca trust",
			folder:     "AKSUbuntu1804+CustomCATrust",
			k8sVersion: "1.18.14",
			nbcUpdator: func(nbc *nbcontractv1.Configuration) {
				nbc.CustomCaCerts = []string{encodedTestCert, encodedTestCert, encodedTestCert}
			},
			validator: func(o *nodeBootstrappingOutput) {
				assert.Equal(t, "3", o.vars["CUSTOM_CA_TRUST_COUNT"])
				assert.Equal(t, "true", o.vars["SHOULD_CONFIGURE_CUSTOM_CA_TRUST"])
				assert.Equal(t, encodedTestCert, o.vars["CUSTOM_CA_CERT_0"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := &datamodel.ContainerService{
				Location: "southcentralus",
				Type:     "Microsoft.ContainerService/ManagedClusters",
				Properties: &datamodel.Properties{
					OrchestratorProfile: &datamodel.OrchestratorProfile{
						OrchestratorType:    datamodel.Kubernetes,
						OrchestratorVersion: tt.k8sVersion,
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
				KeyData: "testsshkey",
			}}

			agentPool := cs.Properties.AgentPoolProfiles[0]

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

			nbcontractv1.ValidateAndSetLinuxKubeletFlags(kubeletConfig, cs, agentPool)
			nBCB := nbcontractv1.NewNBContractBuilder()
			nbc := &nbcontractv1.Configuration{
				LinuxAdminUsername: "azureuser",
				VmSize:             "Standard_DS1_v2",
				ClusterConfig: &nbcontractv1.ClusterConfig{
					Location:      "southcentralus",
					ResourceGroup: "resourceGroupName",
					VmType:        nbcontractv1.ClusterConfig_VMSS,
					ClusterNetworkConfig: &nbcontractv1.ClusterNetworkConfig{
						SecurityGroupName: "aks-agentpool-36873793-nsg",
						VnetName:          "aks-vnet-07752737",
						VnetResourceGroup: "MC_rg",
						Subnet:            "subnet1",
						RouteTable:        "aks-agentpool-36873793-routetable",
					},
					PrimaryScaleSet: "aks-agent2-36873793-vmss",
				},
				AuthConfig: &nbcontractv1.AuthConfig{
					ServicePrincipalId:     "ClientID",
					ServicePrincipalSecret: "Secret",
					TenantId:               "tenantID",
					SubscriptionId:         "subID",
					AssignedIdentityId:     "userAssignedID",
				},
				NetworkConfig: &nbcontractv1.NetworkConfig{
					VnetCniPluginsUrl: "https://acs-mirror.azureedge.net/azure-cni/v1.1.3/binaries/azure-vnet-cni-linux-amd64-v1.1.3.tgz",
				},
				GpuConfig: &nbcontractv1.GPUConfig{
					ConfigGpuDriver: true,
					GpuDevicePlugin: false,
				},
				EnableUnattendedUpgrade: true,
				KubernetesVersion:       tt.k8sVersion,
				ContainerdConfig: &nbcontractv1.ContainerdConfig{
					ContainerdDownloadUrlBase: "https://storage.googleapis.com/cri-containerd-release/",
				},
				OutboundCommand: nbcontractv1.GetDefaultOutboundCommand(),
				KubeletConfig: &nbcontractv1.KubeletConfig{
					EnableKubeletConfigFile: false,
					KubeletFlags:            nbcontractv1.GetKubeletConfigFlag(kubeletConfig, cs, agentPool, false),
					KubeletNodeLabels:       nbcontractv1.GetKubeletNodeLabels(agentPool),
				},
			}
			nBCB.ApplyConfiguration(nbc)
			nbc = nBCB.GetNodeBootstrapConfig()

			if tt.nbcUpdator != nil {
				tt.nbcUpdator(nbc)
			}

			inputJSON, err := json.Marshal(nbc)
			require.NoError(t, err)
			cseCmd, err := parser.Parse(inputJSON)
			assert.NoError(t, err)

			generateTestDataIfRequested(t, tt.folder, cseCmd.UnsafeValue())

			vars, err := getDecodedVarsFromCseCmd([]byte(cseCmd))
			assert.NoError(t, err)

			result := &nodeBootstrappingOutput{
				cseCmd: cseCmd.UnsafeValue(),
				vars:   vars,
			}

			if tt.validator != nil {
				tt.validator(result)
			}
		})
	}
}

func TestNBContractCompatibilityFromJsonToCSECommand(t *testing.T) {
	tests := []struct {
		name      string
		folder    string
		validator outputValidator
	}{
		{
			name:   "with empty config. Parser Should provide default values to unset fields.",
			folder: "Compatibility+EmptyConfig",
			validator: func(o *nodeBootstrappingOutput) {
				sysctlContent, err := getBase64DecodedValue([]byte(o.vars["SYSCTL_CONTENT"]))
				require.NoError(t, err)
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.ipv4.tcp_retries2=%v", 8))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.core.message_burst=%v", 80))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.core.message_cost=%v", 40))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.core.somaxconn=%v", 16384))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.ipv4.tcp_max_syn_backlog=%v", 16384))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.ipv4.neigh.default.gc_thresh1=%v", 4096))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.ipv4.neigh.default.gc_thresh2=%v", 8192))
				assert.Contains(t, sysctlContent, fmt.Sprintf("net.ipv4.neigh.default.gc_thresh3=%v", 16384))
				assert.Equal(t, "false", o.vars["IS_KATA"])
				assert.Equal(t, "false", o.vars["ENABLE_UNATTENDED_UPGRADES"])
				assert.Equal(t, "false", o.vars["NEEDS_CGROUPV2"])
				assert.Equal(t, "azureuser", o.vars["ADMINUSER"])
				assert.Equal(t, "0", o.vars["SWAP_FILE_SIZE_MB"])
				assert.Equal(t, "false", o.vars["SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE"])
				assert.Equal(t, "", o.vars["THP_ENABLED"])
				assert.Equal(t, "", o.vars["THP_DEFRAG"])
				assert.Equal(t, "false", o.vars["DISABLE_SSH"])
				assert.Equal(t, "true", o.vars["IS_VHD"])
				assert.Equal(t, "false", o.vars["NEEDS_DOCKER_LOGIN"])
				assert.Equal(t, "", o.vars["MOBY_VERSION"])
				assert.Equal(t, "", o.vars["LOAD_BALANCER_SKU"])
				assert.Equal(t, "", o.vars["NETWORK_POLICY"])
				assert.Equal(t, "", o.vars["NETWORK_PLUGIN"])
				assert.Equal(t, "", o.vars["VNET_CNI_PLUGINS_URL"])
				assert.Equal(t, "false", o.vars["GPU_NODE"])
				assert.Equal(t, "", o.vars["GPU_INSTANCE_PROFILE"])
				assert.Equal(t, "0", o.vars["CUSTOM_CA_TRUST_COUNT"])
				assert.Equal(t, "false", o.vars["SHOULD_CONFIGURE_CUSTOM_CA_TRUST"])
				assert.Equal(t, "", o.vars["KUBELET_FLAGS"])
				assert.Equal(t, "", o.vars["KUBELET_NODE_LABELS"])
				assert.Equal(t, "", o.vars["HTTP_PROXY"])
				assert.Equal(t, "", o.vars["HTTPS_PROXY"])
				assert.Equal(t, "", o.vars["NO_PROXY"])
				assert.Equal(t, "", o.vars["PROXY_TRUSTED_CA"])
				assert.Equal(t, nbcontractv1.DefaultCloudName, o.vars["TARGET_ENVIRONMENT"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nBCB := nbcontractv1.NewNBContractBuilder()
			nBCB.ApplyConfiguration(&nbcontractv1.Configuration{})

			inputJSON, err := json.Marshal(nBCB.GetNodeBootstrapConfig())
			require.NoError(t, err)
			cseCmd, err := parser.Parse(inputJSON)
			require.NoError(t, err)

			generateTestDataIfRequested(t, tt.folder, cseCmd.UnsafeValue())

			vars, err := getDecodedVarsFromCseCmd([]byte(cseCmd))
			require.NoError(t, err)

			result := &nodeBootstrappingOutput{
				cseCmd: cseCmd.UnsafeValue(),
				vars:   vars,
			}

			if tt.validator != nil {
				tt.validator(result)
			}
		})
	}
}

func TestContractCompatibilityHandledByProtobuf(t *testing.T) {
	tests := []struct {
		name          string
		nbcUTFilePath string
		validator     func(*nbcontractv1.Configuration, *nbcontractv1.Configuration)
	}{
		{
			name:          "with unexpected new fields in json should be ignored",
			nbcUTFilePath: "./testdata/test_nbc_fields_unexpected.json",
			validator: func(nbcExpected *nbcontractv1.Configuration, nbcUT *nbcontractv1.Configuration) {
				// The unexpected fields will natively be ignored when unmarshalling the json to the contract object.
				// We use this test to ensure it.
				assert.Equal(t, nbcExpected, nbcUT)
			},
		},
		{
			name:          "with missing fields in json should be set with default values",
			nbcUTFilePath: "./testdata/test_nbc_fields_missing.json",
			validator: func(_ *nbcontractv1.Configuration, nbcUT *nbcontractv1.Configuration) {
				// if a string field is unset, it will be set to empty string by protobuf by default
				assert.Equal(t, "", nbcUT.GetLinuxAdminUsername())

				// if an optional (explicit presence) bool field is unset, it will be set to nil by protobuf by default.
				// Here we don't use the getter because getter is nil safe and will default to false.
				assert.Nil(t, nbcUT.IsVhd)

				// if an optional (explicit presence) field is unset, it will be set to nil by protobuf by default.
				// Here we don't use the getter because getter is nil safe and will default to false.
				assert.Nil(t, nbcUT.ClusterConfig.LoadBalancerConfig.ExcludeMasterFromStandardLoadBalancer)

				// if an optional enum field is unset, it will be set to 0 (in this case LoadBalancerConfig_UNSPECIFIED) by protobuf by default.
				assert.Equal(t, nbcontractv1.LoadBalancerConfig_UNSPECIFIED, nbcUT.ClusterConfig.LoadBalancerConfig.GetLoadBalancerSku())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nbcExpected := getNBCInstance("./testdata/test_nbc.json")
			nbcUT := getNBCInstance(tt.nbcUTFilePath)

			if tt.validator != nil {
				tt.validator(nbcExpected, nbcUT)
			}
		})
	}
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

func getBase64DecodedValue(data []byte) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

func getNBCInstance(jsonFilePath string) *nbcontractv1.Configuration {
	nBCB := nbcontractv1.NewNBContractBuilder()
	nbc := nbcontractv1.Configuration{}
	content, err := os.ReadFile(jsonFilePath)
	if err != nil {
		log.Fatal(err)
	}
	if err = json.Unmarshal(content, &nbc); err != nil {
		log.Printf("Failed to unmarshal the nbcontractv1 from json: %v", err)
	}
	nBCB.ApplyConfiguration(&nbc)
	return nBCB.GetNodeBootstrapConfig()
}

func generateTestDataIfRequested(t *testing.T, folder, cseCmd string) {
	if os.Getenv("GENERATE_TEST_DATA") == "true" {
		if _, err := os.Stat(fmt.Sprintf("./testdata/%s", folder)); os.IsNotExist(err) {
			e := os.MkdirAll(fmt.Sprintf("./testdata/%s", folder), 0755)
			assert.NoError(t, e)
		}
		err := os.WriteFile(fmt.Sprintf("./testdata/%s/generatedCSECommand", folder), []byte(cseCmd), 0644)
		assert.NoError(t, err)
	}
}
