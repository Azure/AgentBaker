// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"encoding/json"
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
		"--feature-gates":                     "RotateKubeletServerCertificate=true,DynamicKubeletConfig=false", //nolint:lll // what if you turn off dynamic kubelet using dynamic kubelet?
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

func getExampleKcWithNodeStatusReportFrequency() map[string]string {
	kc := map[string]string{
		"--address":                           "0.0.0.0",
		"--pod-manifest-path":                 "/etc/kubernetes/manifests",
		"--cluster-domain":                    "cluster.local",
		"--cluster-dns":                       "10.0.0.10",
		"--cgroups-per-qos":                   "true",
		"--tls-cert-file":                     "/etc/kubernetes/certs/kubeletserver.crt",
		"--tls-private-key-file":              "/etc/kubernetes/certs/kubeletserver.key",
		"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint:lll
		"--max-pods":                          "110",
		"--node-status-update-frequency":      "10s",
		"--node-status-report-frequency":      "5m0s",
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
	}
	return kc
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
		"--feature-gates":                     "RotateKubeletServerCertificate=true,DynamicKubeletConfig=false",
		"--system-reserved":                   "cpu=2,memory=1Gi",
		"--kube-reserved":                     "cpu=100m,memory=1638Mi",
		"--container-log-max-size":            "50M",
	}
	return kc
}

var expectedKubeletJSON = `{
    "kind": "KubeletConfiguration",
    "apiVersion": "kubelet.config.k8s.io/v1beta1",
    "staticPodPath": "/etc/kubernetes/manifests",
    "address": "0.0.0.0",
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
    "rotateCertificates": true,
    "authentication": {
        "x509": {
            "clientCAFile": "/etc/kubernetes/certs/ca.crt"
        },
        "webhook": {
            "enabled": true
        },
        "anonymous": {}
    },
    "authorization": {
        "mode": "Webhook",
        "webhook": {}
    },
    "eventRecordQPS": 0,
    "clusterDomain": "cluster.local",
    "clusterDNS": [
        "10.0.0.10"
    ],
    "streamingConnectionIdleTimeout": "4h0m0s",
    "nodeStatusUpdateFrequency": "10s",
    "imageGCHighThresholdPercent": 90,
    "imageGCLowThresholdPercent": 70,
    "cgroupsPerQOS": true,
    "cpuManagerPolicy": "static",
    "topologyManagerPolicy": "best-effort",
    "maxPods": 110,
    "podPidsLimit": 12345,
    "resolvConf": "/etc/resolv.conf",
    "cpuCFSQuota": false,
    "cpuCFSQuotaPeriod": "200ms",
    "evictionHard": {
        "memory.available": "750Mi",
        "nodefs.available": "10%",
        "nodefs.inodesFree": "5%"
    },
    "protectKernelDefaults": true,
    "featureGates": {
        "CustomCPUCFSQuotaPeriod": true,
        "DynamicKubeletConfig": false,
        "RotateKubeletServerCertificate": true,
        "TopologyManager": true
    },
    "failSwapOn": false,
    "containerLogMaxSize": "1000M",
    "containerLogMaxFiles": 99,
    "systemReserved": {
        "cpu": "2",
        "memory": "1Gi"
    },
    "kubeReserved": {
        "cpu": "100m",
        "memory": "1638Mi"
    },
    "enforceNodeAllocatable": [
        "pods"
    ],
    "allowedUnsafeSysctls": [
        "kernel.msg*",
        "net.ipv4.route.min_pmtu"
    ]
}`

var expectedKubeletJSONWithNodeStatusReportFrequency = `{
    "kind": "KubeletConfiguration",
    "apiVersion": "kubelet.config.k8s.io/v1beta1",
    "staticPodPath": "/etc/kubernetes/manifests",
    "address": "0.0.0.0",
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
    "rotateCertificates": true,
    "authentication": {
        "x509": {
            "clientCAFile": "/etc/kubernetes/certs/ca.crt"
        },
        "webhook": {
            "enabled": true
        },
        "anonymous": {}
    },
    "authorization": {
        "mode": "Webhook",
        "webhook": {}
    },
    "eventRecordQPS": 0,
    "clusterDomain": "cluster.local",
    "clusterDNS": [
        "10.0.0.10"
    ],
    "streamingConnectionIdleTimeout": "4h0m0s",
    "nodeStatusUpdateFrequency": "10s",
    "nodeStatusReportFrequency": "5m0s",
    "imageGCHighThresholdPercent": 90,
    "imageGCLowThresholdPercent": 70,
    "cgroupsPerQOS": true,
    "cpuManagerPolicy": "static",
    "topologyManagerPolicy": "best-effort",
    "maxPods": 110,
    "podPidsLimit": 12345,
    "resolvConf": "/etc/resolv.conf",
    "cpuCFSQuota": false,
    "cpuCFSQuotaPeriod": "200ms",
    "evictionHard": {
        "memory.available": "750Mi",
        "nodefs.available": "10%",
        "nodefs.inodesFree": "5%"
    },
    "protectKernelDefaults": true,
    "featureGates": {
        "CustomCPUCFSQuotaPeriod": true,
        "DynamicKubeletConfig": false,
        "RotateKubeletServerCertificate": true,
        "TopologyManager": true
    },
    "failSwapOn": false,
    "systemReserved": {
        "cpu": "2",
        "memory": "1Gi"
    },
    "kubeReserved": {
        "cpu": "100m",
        "memory": "1638Mi"
    },
    "enforceNodeAllocatable": [
        "pods"
    ],
    "allowedUnsafeSysctls": [
        "kernel.msg*",
        "net.ipv4.route.min_pmtu"
    ]
}`

var expectedKubeletJSONWithContainerMaxLogSizeDefaultFromFlags = `{
    "kind": "KubeletConfiguration",
    "apiVersion": "kubelet.config.k8s.io/v1beta1",
    "staticPodPath": "/etc/kubernetes/manifests",
    "address": "0.0.0.0",
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
    "rotateCertificates": true,
    "authentication": {
        "x509": {
            "clientCAFile": "/etc/kubernetes/certs/ca.crt"
        },
        "webhook": {
            "enabled": true
        },
        "anonymous": {}
    },
    "authorization": {
        "mode": "Webhook",
        "webhook": {}
    },
    "eventRecordQPS": 0,
    "clusterDomain": "cluster.local",
    "clusterDNS": [
        "10.0.0.10"
    ],
    "streamingConnectionIdleTimeout": "4h0m0s",
    "nodeStatusUpdateFrequency": "10s",
    "imageGCHighThresholdPercent": 90,
    "imageGCLowThresholdPercent": 70,
    "cgroupsPerQOS": true,
    "cpuManagerPolicy": "static",
    "topologyManagerPolicy": "best-effort",
    "maxPods": 110,
    "podPidsLimit": 12345,
    "resolvConf": "/etc/resolv.conf",
    "cpuCFSQuota": false,
    "cpuCFSQuotaPeriod": "200ms",
    "evictionHard": {
        "memory.available": "750Mi",
        "nodefs.available": "10%",
        "nodefs.inodesFree": "5%"
    },
    "protectKernelDefaults": true,
    "featureGates": {
        "CustomCPUCFSQuotaPeriod": true,
        "DynamicKubeletConfig": false,
        "RotateKubeletServerCertificate": true,
        "TopologyManager": true
    },
    "failSwapOn": false,
    "containerLogMaxSize": "50M",
    "containerLogMaxFiles": 99,
    "systemReserved": {
        "cpu": "2",
        "memory": "1Gi"
    },
    "kubeReserved": {
        "cpu": "100m",
        "memory": "1638Mi"
    },
    "enforceNodeAllocatable": [
        "pods"
    ],
    "allowedUnsafeSysctls": [
        "kernel.msg*",
        "net.ipv4.route.min_pmtu"
    ]
}`

func TestGetKubeletConfigFileFlagsWithNodeStatusReportFrequency(t *testing.T) {
	kc := getExampleKcWithNodeStatusReportFrequency()
	customKc := &datamodel.CustomKubeletConfig{
		CPUManagerPolicy:      "static",
		CPUCfsQuota:           to.BoolPtr(false),
		CPUCfsQuotaPeriod:     "200ms",
		ImageGcHighThreshold:  to.Int32Ptr(90),
		ImageGcLowThreshold:   to.Int32Ptr(70),
		TopologyManagerPolicy: "best-effort",
		AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
		FailSwapOn:            to.BoolPtr(false),
		PodMaxPids:            to.Int32Ptr(12345),
	}
	configFileStr := GetKubeletConfigFileContent(kc, customKc)
	diff := cmp.Diff(expectedKubeletJSONWithNodeStatusReportFrequency, configFileStr)
	if diff != "" {
		t.Errorf("Generated config file is different than expected: %s", diff)
	}
}

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
			"--node-status-report-frequency": "5m0s",
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
			"--node-status-report-frequency": "10s",
			"--image-gc-high-threshold":      "85",
			"--event-qps":                    "0",
		}
		ap := &datamodel.AgentPoolProfile{}

		expectStr := "--event-qps=0 --image-gc-high-threshold=85 --node-status-update-frequency=20s --streaming-connection-idle-timeout=4h0m0s "
		actucalStr := GetOrderedKubeletConfigFlagString(k, cs, ap, false)
		Expect(expectStr).To(Equal(actucalStr))
	})
	It("should return expected kubelet command line flags when a config file is being used", func() {
		cs := &datamodel.ContainerService{
			Location: "southcentralus",
			Type:     "Microsoft.ContainerService/ManagedClusters",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    "Kubernetes",
					OrchestratorVersion: "1.22.11",
				},
			},
		}
		k := map[string]string{
			"--node-labels":                  "topology.kubernetes.io/region=southcentralus",
			"--node-status-update-frequency": "10s",
			"--node-status-report-frequency": "5m0s",
			"--image-gc-high-threshold":      "85",
			"--event-qps":                    "0",
		}

		ap := &datamodel.AgentPoolProfile{}
		expectedStr := "--node-labels=topology.kubernetes.io/region=southcentralus "
		actualStr := GetOrderedKubeletConfigFlagString(k, cs, ap, true)
		Expect(expectedStr).To(Equal(actualStr))
	})
})

var _ = Describe("Assert datamodel.CSEStatus can be used to parse output JSON", func() {
	It("When cse output format is correct", func() {
		testMessage := `{ "ExitCode": "51", "Output": "test",
		"Error": "", "ExecDuration": "39"}`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).To(BeNil())
		Expect(cseStatus.ExitCode).To(Equal("51"))
		Expect(cseStatus.Output).To(Equal("test"))
		Expect(cseStatus.Error).To(Equal(""))
		Expect(cseStatus.ExecDuration).To(Equal("39"))
	})

	It("When cse output format is correct and contains call known fields", func() {
		//lint:ignore lll
		testMessage := `{"ExitCode": "51", "Output": "test", "Error": "",
		"ExecDuration": "39", "KernelStartTime": "kernel start time", 
		"SystemdSummary": "systemd summary", "CSEStartTime": "cse start time",
		"GuestAgentStartTime": "guest agent start time", "BootDatapoints": {"dp1": "1"}}`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).To(BeNil())
		Expect(cseStatus.ExitCode).To(Equal("51"))
		Expect(cseStatus.Output).To(Equal("test"))
		Expect(cseStatus.Error).To(Equal(""))
		Expect(cseStatus.ExecDuration).To(Equal("39"))
		Expect(cseStatus.KernelStartTime).To(Equal("kernel start time"))
		Expect(cseStatus.SystemdSummary).To(Equal("systemd summary"))
		Expect(cseStatus.CSEStartTime).To(Equal("cse start time"))
		Expect(cseStatus.GuestAgentStartTime).To(Equal("guest agent start time"))
		Expect(len(cseStatus.BootDatapoints)).To(Equal(1))
		Expect(cseStatus.BootDatapoints["dp1"]).To(Equal("1"))
	})

	It("When cse output exitcode is missing", func() {
		testMessage := `{ "ExitCode": , "Output": "test",
		"Error": "", "ExecDuration": "39"}`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).NotTo(BeNil())
	})

	It("When ExecDuration is missing", func() {
		testMessage := `{ "ExitCode": "51", "Output": "test",
		"Error": "", "ExecDuration": }`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).ToNot(BeNil())
	})

	It("When Output is missing", func() {
		testMessage := `{ "ExitCode": "51", "Output": "test",
		"Error": "", "ExecDuration": "39", "Output": }`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).NotTo(BeNil())
	})

	It("When Error is missing", func() {
		testMessage := `{ "ExitCode": "51", "Output": "test", 
		"Error": "", "ExecDuration": "39", "Error": }`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).NotTo(BeNil())
	})

	It("When KernelStartTime is missing", func() {
		testMessage := `{ "ExitCode": "51", "Output": "test",
		"Error": "", "ExecDuration": "39", "KernelStartTime": }`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).NotTo(BeNil())
	})

	It("When SystemdSummary is missing", func() {
		testMessage := `{ "ExitCode": "51", "Output": "test", 
		"Error": "", "ExecDuration": "39", "SystemdSummary": }`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).NotTo(BeNil())
	})

	It("When CSEStartTime is missing", func() {
		testMessage := `{ "ExitCode": "51", "Output": "test",
		"Error": "", "ExecDuration": "39", "CSEStartTime": }`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).NotTo(BeNil())
	})

	It("When GuestAgentStartTime is missing", func() {
		testMessage := `{ "ExitCode": "51", "Output": "test", 
		"Error": "", "ExecDuration": "39", "GuestAgentStartTime": }`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).NotTo(BeNil())
	})

	It("When BootDatapoints is missing", func() {
		testMessage := `{ "ExitCode": "51", "Output": "test", 
		"Error": "", "ExecDuration": "39", "BootDatapoints": }`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).NotTo(BeNil())
	})

	It("When BootDatapoints is malformed", func() {
		testMessage := `{ "ExitCode": "51", "Output": "test",
		"Error": "", "ExecDuration": "39", "BootDatapoints": {datapoint:1}}`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).NotTo(BeNil())
	})

	It("when ExecDuration is an integer", func() {
		testMessage := `{ "ExitCode": "51", "Output": "test", 
		"Error": "", "ExecDuration": 39}`
		var cseStatus datamodel.CSEStatus
		err := json.Unmarshal([]byte(testMessage), &cseStatus)
		Expect(err).NotTo(BeNil())
	})
})
