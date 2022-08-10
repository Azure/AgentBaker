// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"fmt"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/go-cmp/cmp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGetKubeletConfigFileFromFlags(t *testing.T) {
	kc := map[string]string{
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
		"--feature-gates":                     "RotateKubeletServerCertificate=true,DynamicKubeletConfig=false", // what if you turn off dynamic kubelet using dynamic kubelet?
		"--system-reserved":                   "cpu=2,memory=1Gi",
		"--kube-reserved":                     "cpu=100m,memory=1638Mi",
	}
	customKc := &datamodel.CustomKubeletConfig{
		CPUManagerPolicy:      "static",
		CPUCfsQuota:           to.BoolPtr(false),
		CPUCfsQuotaPeriod:     "200ms",
		ImageGcHighThreshold:  to.Int32Ptr(90),
		ImageGcLowThreshold:   to.Int32Ptr(70),
		TopologyManagerPolicy: "best-effort",
		AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
		FailSwapOn:            to.BoolPtr(false),
		ContainerLogMaxSizeMB: to.Int32Ptr(1000),
		ContainerLogMaxFiles:  to.Int32Ptr(99),
		PodMaxPids:            to.Int32Ptr(12345),
	}
	configFileStr := GetKubeletConfigFileContent(kc, customKc)
	diff := cmp.Diff(expectedKubeletJSON, configFileStr)
	if diff != "" {
		t.Errorf("Generated config file is different than expected: %s", diff)
	}
}

func getExampleKcWithContainerLogMaxSize() map[string]string {
	kc := map[string]string{
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
		"--feature-gates":                     "RotateKubeletServerCertificate=true,DynamicKubeletConfig=false",
		"--system-reserved":                   "cpu=2,memory=1Gi",
		"--kube-reserved":                     "cpu=100m,memory=1638Mi",
		"--container-log-max-size":            "50M",
	}
	return kc
}

var expectedKubeletJSON string = `{
    "kind": "KubeletConfiguration",
    "apiVersion": "kubelet.config.k8s.io/v1beta1",
    "enableServer": null,
    "staticPodPath": "/etc/kubernetes/manifests",
    "syncFrequency": "0s",
    "fileCheckFrequency": "0s",
    "httpCheckFrequency": "0s",
    "staticPodURL": "",
    "staticPodURLHeader": null,
    "address": "0.0.0.0",
    "port": 0,
    "readOnlyPort": 10255,
    "tlsCertFile": "/etc/kubernetes/certs/kubeletserver.crt",
    "tlsPrivateKeyFile": "/etc/kubernetes/certs/kubeletserver.key",
    "tlsCipherSuites": [
        "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
        "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
        "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
        "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
        "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
        "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
        "TLS_RSA_WITH_AES_256_GCM_SHA384",
        "TLS_RSA_WITH_AES_128_GCM_SHA256"
    ],
    "tlsMinVersion": "",
    "rotateCertificates": true,
    "serverTLSBootstrap": false,
    "authentication": {
        "x509": {
            "clientCAFile": "/etc/kubernetes/certs/ca.crt"
        },
        "webhook": {
            "enabled": true,
            "cacheTTL": "0s"
        },
        "anonymous": {
            "enabled": false
        }
    },
    "authorization": {
        "mode": "Webhook",
        "webhook": {
            "cacheAuthorizedTTL": "0s",
            "cacheUnauthorizedTTL": "0s"
        }
    },
    "registryPullQPS": null,
    "registryBurst": 0,
    "eventRecordQPS": 0,
    "eventBurst": 0,
    "enableDebuggingHandlers": null,
    "enableContentionProfiling": false,
    "healthzPort": null,
    "healthzBindAddress": "",
    "oomScoreAdj": null,
    "clusterDomain": "cluster.local",
    "clusterDNS": [
        "10.0.0.10"
    ],
    "streamingConnectionIdleTimeout": "4h0m0s",
    "nodeStatusUpdateFrequency": "10s",
    "nodeStatusReportFrequency": "0s",
    "nodeLeaseDurationSeconds": 0,
    "imageMinimumGCAge": "0s",
    "imageGCHighThresholdPercent": 90,
    "imageGCLowThresholdPercent": 70,
    "volumeStatsAggPeriod": "0s",
    "kubeletCgroups": "",
    "systemCgroups": "",
    "cgroupRoot": "",
    "cgroupsPerQOS": true,
    "cgroupDriver": "",
    "cpuManagerPolicy": "static",
    "cpuManagerPolicyOptions": null,
    "cpuManagerReconcilePeriod": "0s",
    "memoryManagerPolicy": "",
    "topologyManagerPolicy": "best-effort",
    "topologyManagerScope": "",
    "qosReserved": null,
    "runtimeRequestTimeout": "0s",
    "hairpinMode": "",
    "maxPods": 110,
    "podCIDR": "",
    "podPidsLimit": 12345,
    "resolvConf": "/etc/resolv.conf",
    "runOnce": false,
    "cpuCFSQuota": false,
    "cpuCFSQuotaPeriod": "200ms",
    "nodeStatusMaxImages": null,
    "maxOpenFiles": 0,
    "contentType": "",
    "kubeAPIQPS": null,
    "kubeAPIBurst": 0,
    "serializeImagePulls": null,
    "evictionHard": {
        "memory.available": "750Mi",
        "nodefs.available": "10%",
        "nodefs.inodesFree": "5%"
    },
    "evictionSoft": null,
    "evictionSoftGracePeriod": null,
    "evictionPressureTransitionPeriod": "0s",
    "evictionMaxPodGracePeriod": 0,
    "evictionMinimumReclaim": null,
    "podsPerCore": 0,
    "enableControllerAttachDetach": null,
    "protectKernelDefaults": true,
    "makeIPTablesUtilChains": null,
    "iptablesMasqueradeBit": null,
    "iptablesDropBit": null,
    "featureGates": {
        "CustomCPUCFSQuotaPeriod": true,
        "DynamicKubeletConfig": false,
        "RotateKubeletServerCertificate": true,
        "TopologyManager": true
    },
    "failSwapOn": false,
    "memorySwap": {},
    "containerLogMaxSize": "1000M",
    "containerLogMaxFiles": 99,
    "configMapAndSecretChangeDetectionStrategy": "",
    "systemReserved": {
        "cpu": "2",
        "memory": "1Gi"
    },
    "kubeReserved": {
        "cpu": "100m",
        "memory": "1638Mi"
    },
    "reservedSystemCPUs": "",
    "showHiddenMetricsForVersion": "",
    "systemReservedCgroup": "",
    "kubeReservedCgroup": "",
    "enforceNodeAllocatable": [
        "pods"
    ],
    "allowedUnsafeSysctls": [
        "kernel.msg*",
        "net.ipv4.route.min_pmtu"
    ],
    "volumePluginDir": "",
    "providerID": "",
    "kernelMemcgNotification": false,
    "logging": {
        "flushFrequency": 0,
        "verbosity": 0,
        "options": {
            "json": {
                "infoBufferSize": "0"
            }
        }
    },
    "enableSystemLogHandler": null,
    "shutdownGracePeriod": "0s",
    "shutdownGracePeriodCriticalPods": "0s",
    "shutdownGracePeriodByPodPriority": null,
    "reservedMemory": null,
    "enableProfilingHandler": null,
    "enableDebugFlagsHandler": null,
    "seccompDefault": null,
    "memoryThrottlingFactor": null,
    "registerWithTaints": null,
    "registerNode": null
}`

var expectedKubeletJSONWithContainerMaxLogSizeDefaultFromFlags string = `{
    "kind": "KubeletConfiguration",
    "apiVersion": "kubelet.config.k8s.io/v1beta1",
    "enableServer": null,
    "staticPodPath": "/etc/kubernetes/manifests",
    "syncFrequency": "0s",
    "fileCheckFrequency": "0s",
    "httpCheckFrequency": "0s",
    "staticPodURL": "",
    "staticPodURLHeader": null,
    "address": "0.0.0.0",
    "port": 0,
    "readOnlyPort": 10255,
    "tlsCertFile": "/etc/kubernetes/certs/kubeletserver.crt",
    "tlsPrivateKeyFile": "/etc/kubernetes/certs/kubeletserver.key",
    "tlsCipherSuites": [
        "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
        "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
        "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
        "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
        "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
        "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
        "TLS_RSA_WITH_AES_256_GCM_SHA384",
        "TLS_RSA_WITH_AES_128_GCM_SHA256"
    ],
    "tlsMinVersion": "",
    "rotateCertificates": true,
    "serverTLSBootstrap": false,
    "authentication": {
        "x509": {
            "clientCAFile": "/etc/kubernetes/certs/ca.crt"
        },
        "webhook": {
            "enabled": true,
            "cacheTTL": "0s"
        },
        "anonymous": {
            "enabled": false
        }
    },
    "authorization": {
        "mode": "Webhook",
        "webhook": {
            "cacheAuthorizedTTL": "0s",
            "cacheUnauthorizedTTL": "0s"
        }
    },
    "registryPullQPS": null,
    "registryBurst": 0,
    "eventRecordQPS": 0,
    "eventBurst": 0,
    "enableDebuggingHandlers": null,
    "enableContentionProfiling": false,
    "healthzPort": null,
    "healthzBindAddress": "",
    "oomScoreAdj": null,
    "clusterDomain": "cluster.local",
    "clusterDNS": [
        "10.0.0.10"
    ],
    "streamingConnectionIdleTimeout": "4h0m0s",
    "nodeStatusUpdateFrequency": "10s",
    "nodeStatusReportFrequency": "0s",
    "nodeLeaseDurationSeconds": 0,
    "imageMinimumGCAge": "0s",
    "imageGCHighThresholdPercent": 90,
    "imageGCLowThresholdPercent": 70,
    "volumeStatsAggPeriod": "0s",
    "kubeletCgroups": "",
    "systemCgroups": "",
    "cgroupRoot": "",
    "cgroupsPerQOS": true,
    "cgroupDriver": "",
    "cpuManagerPolicy": "static",
    "cpuManagerPolicyOptions": null,
    "cpuManagerReconcilePeriod": "0s",
    "memoryManagerPolicy": "",
    "topologyManagerPolicy": "best-effort",
    "topologyManagerScope": "",
    "qosReserved": null,
    "runtimeRequestTimeout": "0s",
    "hairpinMode": "",
    "maxPods": 110,
    "podCIDR": "",
    "podPidsLimit": 12345,
    "resolvConf": "/etc/resolv.conf",
    "runOnce": false,
    "cpuCFSQuota": false,
    "cpuCFSQuotaPeriod": "200ms",
    "nodeStatusMaxImages": null,
    "maxOpenFiles": 0,
    "contentType": "",
    "kubeAPIQPS": null,
    "kubeAPIBurst": 0,
    "serializeImagePulls": null,
    "evictionHard": {
        "memory.available": "750Mi",
        "nodefs.available": "10%",
        "nodefs.inodesFree": "5%"
    },
    "evictionSoft": null,
    "evictionSoftGracePeriod": null,
    "evictionPressureTransitionPeriod": "0s",
    "evictionMaxPodGracePeriod": 0,
    "evictionMinimumReclaim": null,
    "podsPerCore": 0,
    "enableControllerAttachDetach": null,
    "protectKernelDefaults": true,
    "makeIPTablesUtilChains": null,
    "iptablesMasqueradeBit": null,
    "iptablesDropBit": null,
    "featureGates": {
        "CustomCPUCFSQuotaPeriod": true,
        "DynamicKubeletConfig": false,
        "RotateKubeletServerCertificate": true,
        "TopologyManager": true
    },
    "failSwapOn": false,
    "memorySwap": {},
    "containerLogMaxSize": "50M",
    "containerLogMaxFiles": 99,
    "configMapAndSecretChangeDetectionStrategy": "",
    "systemReserved": {
        "cpu": "2",
        "memory": "1Gi"
    },
    "kubeReserved": {
        "cpu": "100m",
        "memory": "1638Mi"
    },
    "reservedSystemCPUs": "",
    "showHiddenMetricsForVersion": "",
    "systemReservedCgroup": "",
    "kubeReservedCgroup": "",
    "enforceNodeAllocatable": [
        "pods"
    ],
    "allowedUnsafeSysctls": [
        "kernel.msg*",
        "net.ipv4.route.min_pmtu"
    ],
    "volumePluginDir": "",
    "providerID": "",
    "kernelMemcgNotification": false,
    "logging": {
        "flushFrequency": 0,
        "verbosity": 0,
        "options": {
            "json": {
                "infoBufferSize": "0"
            }
        }
    },
    "enableSystemLogHandler": null,
    "shutdownGracePeriod": "0s",
    "shutdownGracePeriodCriticalPods": "0s",
    "shutdownGracePeriodByPodPriority": null,
    "reservedMemory": null,
    "enableProfilingHandler": null,
    "enableDebugFlagsHandler": null,
    "seccompDefault": null,
    "memoryThrottlingFactor": null,
    "registerWithTaints": null,
    "registerNode": null
}`

func TestGetKubeletConfigFileFromFlagsWithContainerLogMaxSize(t *testing.T) {
	kc := getExampleKcWithContainerLogMaxSize()
	customKc := &datamodel.CustomKubeletConfig{
		CPUManagerPolicy:      "static",
		CPUCfsQuota:           to.BoolPtr(false),
		CPUCfsQuotaPeriod:     "200ms",
		ImageGcHighThreshold:  to.Int32Ptr(90),
		ImageGcLowThreshold:   to.Int32Ptr(70),
		TopologyManagerPolicy: "best-effort",
		AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
		FailSwapOn:            to.BoolPtr(false),
		ContainerLogMaxFiles:  to.Int32Ptr(99),
		PodMaxPids:            to.Int32Ptr(12345),
	}
	configFileStr := GetKubeletConfigFileContent(kc, customKc)
	diff := cmp.Diff(expectedKubeletJSONWithContainerMaxLogSizeDefaultFromFlags, configFileStr)
	fmt.Println(configFileStr)
	if diff != "" {
		t.Errorf("Generated config file is different than expected: %s", diff)
	}
}

func TestGetKubeletConfigFileCustomKCShouldOverrideValuesPassedInKc(t *testing.T) {
	kc := getExampleKcWithContainerLogMaxSize()
	customKc := &datamodel.CustomKubeletConfig{
		CPUManagerPolicy:      "static",
		CPUCfsQuota:           to.BoolPtr(false),
		CPUCfsQuotaPeriod:     "200ms",
		ImageGcHighThreshold:  to.Int32Ptr(90),
		ImageGcLowThreshold:   to.Int32Ptr(70),
		TopologyManagerPolicy: "best-effort",
		AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
		FailSwapOn:            to.BoolPtr(false),
		ContainerLogMaxFiles:  to.Int32Ptr(99),
		ContainerLogMaxSizeMB: to.Int32Ptr(1000),
		PodMaxPids:            to.Int32Ptr(12345),
	}
	configFileStr := GetKubeletConfigFileContent(kc, customKc)
	diff := cmp.Diff(expectedKubeletJSON, configFileStr)
	if diff != "" {
		t.Errorf("Generated config file is different than expected: %s", diff)
	}
}

func TestIsKubeletClientTLSBootstrappingEnabled(t *testing.T) {
	cases := []struct {
		tlsBootstrapToken *string
		expected          bool
		reason            string
	}{
		{
			tlsBootstrapToken: nil,
			expected:          false,
			reason:            "agent pool TLS bootstrap token not set",
		},
		{
			tlsBootstrapToken: to.StringPtr("foobar.foobar"),
			expected:          true,
			reason:            "supported",
		},
	}

	for _, c := range cases {
		actual := IsKubeletClientTLSBootstrappingEnabled(c.tlsBootstrapToken)
		if actual != c.expected {
			t.Errorf("%s: expected=%t, actual=%t", c.reason, c.expected, actual)
		}
	}
}

func TestGetTLSBootstrapTokenForKubeConfig(t *testing.T) {
	cases := []struct {
		token    *string
		expected string
	}{
		{
			token:    nil,
			expected: "",
		},
		{
			token:    to.StringPtr("foo.bar"),
			expected: "foo.bar",
		},
	}

	for _, c := range cases {
		actual := GetTLSBootstrapTokenForKubeConfig(c.token)
		if actual != c.expected {
			t.Errorf("GetTLSBootstrapTokenForKubeConfig: expected=%s, actual=%s", c.expected, actual)
		}
	}
}

var _ = Describe("Test GetOrderedKubeletConfigFlagString", func() {
	It("should return expected kubelet config when custom configuration is not set", func() {

		cs := &datamodel.ContainerService{
			Location:   "southcentralus",
			Type:       "Microsoft.ContainerService/ManagedClusters",
			Properties: &datamodel.Properties{},
		}

		k := map[string]string{
			"--node-status-update-frequency": "10s",
			"--image-gc-high-threshold":      "85",
			"--event-qps":                    "0",
		}
		ap := &datamodel.AgentPoolProfile{}

		expectStr := "--event-qps=0 --image-gc-high-threshold=85 --node-status-update-frequency=10s "
		actucalStr := GetOrderedKubeletConfigFlagString(k, cs, ap, false)
		Expect(expectStr).To(Equal(actucalStr))
	})
	It("should return expected kubelet config when custom configuration is set", func() {

		cs := &datamodel.ContainerService{
			Location: "southcentralus",
			Type:     "Microsoft.ContainerService/ManagedClusters",
			Properties: &datamodel.Properties{
				CustomConfiguration: &datamodel.CustomConfiguration{
					KubernetesConfigurations: map[string]*datamodel.ComponentConfiguration{
						"kubelet": {
							Config: map[string]string{
								"--node-status-update-frequency":      "20s",
								"--streaming-connection-idle-timeout": "4h0m0s",
							},
						},
					},
				},
			},
		}

		k := map[string]string{
			"--node-status-update-frequency": "10s",
			"--image-gc-high-threshold":      "85",
			"--event-qps":                    "0",
		}
		ap := &datamodel.AgentPoolProfile{}

		expectStr := "--event-qps=0 --image-gc-high-threshold=85 --node-status-update-frequency=20s --streaming-connection-idle-timeout=4h0m0s "
		actucalStr := GetOrderedKubeletConfigFlagString(k, cs, ap, false)
		Expect(expectStr).To(Equal(actucalStr))
	})
})

var _ = Describe("Assert ParseCSE", func() {
	It("when cse output format is correct", func() {
		testMessage := "vmss aks-agentpool-test-vmss instance 0 vmssCSE message : Enable failed:\n[stdout]\n{ \"ExitCode\": \"51\", \"Output\": \"test\", \"Error\": \"\", \"ExecDuration\": \"39\"}\n\n[stderr]\n"
		res, err := ParseCSEMessage(testMessage)
		Expect(err).To(BeNil())
		Expect(res.ExitCode).To(Equal("51"))
		Expect(res.Output).To(Equal("test"))
		Expect(res.Error).To(Equal(""))
		Expect(res.ExecDuration).To(Equal("39"))
	})

	It("when cse output format is incorrect", func() {
		testMessage := "vmss aks-agentpool-test-vmss instance 0 vmssCSE message : Enable failed:\n[stdout]\n"
		_, err := ParseCSEMessage(testMessage)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("InstanceErrorCode=InvalidCSEMessage"))
	})

	It("when cse output exitcode is empty", func() {
		testMessage := "vmss aks-agentpool-test-vmss instance 0 vmssCSE message : Enable failed:\n[stdout]\n{ \"ExitCode\": \"\", \"Output\": \"test\", \"Error\": \"\"}\n\n[stderr]\n"
		_, err := ParseCSEMessage(testMessage)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("InstanceErrorCode=CSEMessageExitCodeEmptyError"))
	})

	It("when cse output exitcode is empty", func() {
		testMessage := "vmss aks-agentpool-test-vmss instance 0 vmssCSE message : Enable failed:\n[stdout]\n{ \"ExitCode\": , \"Output\": \"test\", \"Error\": \"\", \"ExecDuration\": \"39\"}\n\n[stderr]\n"
		_, err := ParseCSEMessage(testMessage)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("InstanceErrorCode=CSEMessageUnmarshalError"))
	})

	It("when ExecDuration is integer", func() {
		testMessage := "vmss aks-agentpool-test-vmss instance 0 vmssCSE message : Enable failed:\n[stdout]\n{ \"ExitCode\": \"51\", \"Output\": \"test\", \"Error\": \"\", \"ExecDuration\": 39}\n\n[stderr]\n"
		_, err := ParseCSEMessage(testMessage)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("InstanceErrorCode=CSEMessageUnmarshalError"))
	})

	It("when Windows cse output is correct with exitcode is empty", func() {
		testMessage := "vmss aks-agentpool-test-vmss instance 0 vmssCSE message : Command execution finished"
		res, err := ParseCSEMessage(testMessage)
		Expect(err).To(BeNil())
		Expect(res.ExitCode).To(Equal("0"))
		Expect(res.Output).To(Equal(testMessage))
	})

	It("when Windows cse output is correct with exitcode is not empty", func() {
		testMessage := "vmss aks-agentpool-test-vmss instance 0 vmssCSE message : Command execution finished, but failed because it returned a non-zero exit code of: '5'"
		res, err := ParseCSEMessage(testMessage)
		Expect(err).To(BeNil())
		Expect(res.ExitCode).To(Equal("5"))
		Expect(res.Output).To(Equal(testMessage))
	})

	It("when Windows cse output is incorrect", func() {
		testMessage := "vmss aks-agentpool-test-vmss instance 0 vmssCSE message : Command execution succeeded"
		_, err := ParseCSEMessage(testMessage)
		Expect(err).NotTo(BeNil())
		Expect(err.Error()).To(ContainSubstring("InstanceErrorCode=InvalidCSEMessage"))
		Expect(err.Error()).To(ContainSubstring(testMessage))
	})
})
