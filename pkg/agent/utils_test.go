// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

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
		"--rotate-server-certificates":        "true",
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
		SeccompDefault:        to.BoolPtr(true),
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
		"--rotate-server-certificates":        "true",
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
    "serverTLSBootstrap": true,
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
        "RotateKubeletServerCertificate": true
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
    ],
    "seccompDefault": true
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
        "RotateKubeletServerCertificate": true
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
    "serverTLSBootstrap": true,
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
        "RotateKubeletServerCertificate": true
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

func TestIsKubeletServingCertificateRotationEnabled(t *testing.T) {
	cases := []struct {
		name     string
		config   *datamodel.NodeBootstrappingConfiguration
		expected bool
	}{
		{
			name:     "nil NodeBootstrappingConfiguration",
			config:   nil,
			expected: false,
		},
		{
			name: "nil KubeletConfig",
			config: &datamodel.NodeBootstrappingConfiguration{
				KubeletConfig: nil,
			},
			expected: false,
		},
		{
			name: "KubeletConfig is missing the --rotate-server-certificates flag",
			config: &datamodel.NodeBootstrappingConfiguration{
				KubeletConfig: map[string]string{
					"--tls-cert-file":        "cert.crt",
					"--tls-private-key-file": "cert.key",
				},
			},
			expected: false,
		},
		{
			name: "KubeletConfig has --rotate-server-certificates set to false",
			config: &datamodel.NodeBootstrappingConfiguration{
				KubeletConfig: map[string]string{
					"--tls-cert-file":              "cert.crt",
					"--tls-private-key-file":       "cert.key",
					"--rotate-server-certificates": "false",
				},
			},
			expected: false,
		},
		{
			name: "KubeletConfig has --rotate-server-certificates set to true",
			config: &datamodel.NodeBootstrappingConfiguration{
				KubeletConfig: map[string]string{
					"--tls-cert-file":              "cert.crt",
					"--tls-private-key-file":       "cert.key",
					"--rotate-server-certificates": "true",
				},
			},
			expected: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := IsKubeletServingCertificateRotationEnabled(c.config)
			assert.Equal(t, c.expected, actual)
		})
	}
}

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
		SeccompDefault:        to.BoolPtr(true),
	}
	configFileStr := GetKubeletConfigFileContent(kc, customKc)
	diff := cmp.Diff(expectedKubeletJSON, configFileStr)
	if diff != "" {
		t.Errorf("Generated config file is different than expected: %s", diff)
	}
}

func TestGetKubeletConfigFileNodeMemoryHardeningFields(t *testing.T) {
	// Verifies AgentBaker renders the new Node Memory Hardening kubelet args
	// (soft eviction + cgroup tiering) into the generated kubelet config file.
	// Uses JSON unmarshaling rather than a brittle text snapshot so that future
	// non-related additions to AKSKubeletConfiguration do not break this test.
	kc := getExampleKcWithNodeStatusReportFrequency()
	kc["--eviction-soft"] = "memory.available<500Mi,nodefs.available<15%,imagefs.available<20%"
	kc["--eviction-soft-grace-period"] = "memory.available=30s,nodefs.available=2m,imagefs.available=2m"
	kc["--eviction-max-pod-grace-period"] = "60"
	kc["--enforce-node-allocatable"] = "pods,kube-reserved,system-reserved"
	kc["--kube-reserved-cgroup"] = "/kubelet.slice"
	kc["--system-reserved-cgroup"] = "/system.slice"

	configFileStr := GetKubeletConfigFileContent(kc, nil)

	var got struct {
		EvictionSoft              map[string]string `json:"evictionSoft"`
		EvictionSoftGracePeriod   map[string]string `json:"evictionSoftGracePeriod"`
		EvictionMaxPodGracePeriod int32             `json:"evictionMaxPodGracePeriod"`
		EnforceNodeAllocatable    []string          `json:"enforceNodeAllocatable"`
		KubeReservedCgroup        string            `json:"kubeReservedCgroup"`
		SystemReservedCgroup      string            `json:"systemReservedCgroup"`
	}
	if err := json.Unmarshal([]byte(configFileStr), &got); err != nil {
		t.Fatalf("failed to unmarshal generated kubelet config: %v\nconfig: %s", err, configFileStr)
	}

	wantSoft := map[string]string{
		"memory.available":  "500Mi",
		"nodefs.available":  "15%",
		"imagefs.available": "20%",
	}
	if diff := cmp.Diff(wantSoft, got.EvictionSoft); diff != "" {
		t.Errorf("evictionSoft mismatch (-want +got):\n%s", diff)
	}

	wantSoftGrace := map[string]string{
		"memory.available":  "30s",
		"nodefs.available":  "2m",
		"imagefs.available": "2m",
	}
	if diff := cmp.Diff(wantSoftGrace, got.EvictionSoftGracePeriod); diff != "" {
		t.Errorf("evictionSoftGracePeriod mismatch (-want +got):\n%s", diff)
	}

	if got.EvictionMaxPodGracePeriod != 60 {
		t.Errorf("evictionMaxPodGracePeriod=%d, want 60", got.EvictionMaxPodGracePeriod)
	}

	wantEnforce := []string{"pods", "kube-reserved", "system-reserved"}
	if diff := cmp.Diff(wantEnforce, got.EnforceNodeAllocatable); diff != "" {
		t.Errorf("enforceNodeAllocatable mismatch (-want +got):\n%s", diff)
	}

	if got.KubeReservedCgroup != "/kubelet.slice" {
		t.Errorf("kubeReservedCgroup=%q, want %q", got.KubeReservedCgroup, "/kubelet.slice")
	}
	if got.SystemReservedCgroup != "/system.slice" {
		t.Errorf("systemReservedCgroup=%q, want %q", got.SystemReservedCgroup, "/system.slice")
	}
}

func TestGetKubeletConfigFileNodeMemoryHardeningFieldsOmittedByDefault(t *testing.T) {
	// Backward-compat: when the RP does not pass the new flags, the generated
	// kubelet config must NOT contain the new fields. This guards the 6-month
	// VHD support window — non-hardened pools must see no change to these fields.
	kc := getExampleKcWithNodeStatusReportFrequency()

	configFileStr := GetKubeletConfigFileContent(kc, nil)

	for _, field := range []string{
		`"evictionSoft"`,
		`"evictionSoftGracePeriod"`,
		`"evictionMaxPodGracePeriod"`,
		`"kubeReservedCgroup"`,
		`"systemReservedCgroup"`,
	} {
		if strings.Contains(configFileStr, field) {
			t.Errorf("expected %s to be omitted from kubelet config when not set, got:\n%s", field, configFileStr)
		}
	}
}

func TestGetKubeletConfigFileFiltersUnknownEvictionSignals(t *testing.T) {
	// Kubelet only accepts a fixed set of eviction signals; any unknown key would
	// cause it to fail to start. Verify we drop unknowns from --eviction-hard,
	// --eviction-soft, and --eviction-soft-grace-period before rendering.
	kc := getExampleKcWithNodeStatusReportFrequency()
	kc["--eviction-hard"] = "memory.available<750Mi,bogus.signal<1Gi,nodefs.available<10%"
	kc["--eviction-soft"] = "memory.available<500Mi,not-a-signal<1Gi"
	kc["--eviction-soft-grace-period"] = "memory.available=30s,not-a-signal=1m"

	configFileStr := GetKubeletConfigFileContent(kc, nil)

	var got struct {
		EvictionHard            map[string]string `json:"evictionHard"`
		EvictionSoft            map[string]string `json:"evictionSoft"`
		EvictionSoftGracePeriod map[string]string `json:"evictionSoftGracePeriod"`
	}
	if err := json.Unmarshal([]byte(configFileStr), &got); err != nil {
		t.Fatalf("failed to unmarshal generated kubelet config: %v\nconfig: %s", err, configFileStr)
	}

	for _, m := range []map[string]string{got.EvictionHard, got.EvictionSoft, got.EvictionSoftGracePeriod} {
		for _, bad := range []string{"bogus.signal", "not-a-signal"} {
			if _, present := m[bad]; present {
				t.Errorf("expected unknown eviction signal %q to be filtered, got map %v", bad, m)
			}
		}
	}
	if _, ok := got.EvictionHard["memory.available"]; !ok {
		t.Errorf("expected memory.available in evictionHard, got %v", got.EvictionHard)
	}
	if _, ok := got.EvictionSoft["memory.available"]; !ok {
		t.Errorf("expected memory.available in evictionSoft, got %v", got.EvictionSoft)
	}
}

func TestIsTLSBootstrappingEnabledWithHardCodedToken(t *testing.T) {
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
		actual := IsTLSBootstrappingEnabledWithHardCodedToken(c.tlsBootstrapToken)
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

func TestGetCloudTargetEnv(t *testing.T) {
	cases := []struct {
		location string
		expected string
	}{
		{location: "chinaeast", expected: "AzureChinaCloud"},
		{location: "chinanorth2", expected: "AzureChinaCloud"},
		{location: "germanynortheast", expected: "AzureGermanCloud"},
		{location: "germanycentral", expected: "AzureGermanCloud"},
		{location: "usgovvirginia", expected: "AzureUSGovernmentCloud"},
		{location: "usdodcentral", expected: "AzureUSGovernmentCloud"},
		{location: "bleufrancecentral", expected: "AzureBleuCloud"},
		{location: "deloseast", expected: "AzureGermanyCloud"},
		{location: "singaporenorth", expected: "AzureSingaporeCloud"},
		{location: "Singapore North", expected: "AzureSingaporeCloud"},
		{location: "westus2", expected: "AzurePublicCloud"},
		{location: "", expected: "AzurePublicCloud"},
	}

	for _, c := range cases {
		actual := GetCloudTargetEnv(c.location)
		if actual != c.expected {
			t.Errorf("GetCloudTargetEnv(%q): expected=%s, actual=%s", c.location, c.expected, actual)
		}
	}
}

var _ = Describe("Test GetOrderedKubeletConfigFlagString", func() {
	It("should return expected kubelet config when custom configuration is not set", func() {
		config := &datamodel.NodeBootstrappingConfiguration{
			KubeletConfig: map[string]string{
				"--node-status-update-frequency": "10s",
				"--node-status-report-frequency": "5m0s",
				"--image-gc-high-threshold":      "85",
				"--event-qps":                    "0",
			},
			ContainerService: &datamodel.ContainerService{
				Location:   "southcentralus",
				Type:       "Microsoft.ContainerService/ManagedClusters",
				Properties: &datamodel.Properties{},
			},
			EnableKubeletConfigFile: false,
			AgentPoolProfile:        &datamodel.AgentPoolProfile{},
		}
		actucalStr := GetOrderedKubeletConfigFlagString(config)
		expectStr := "--event-qps=0 --image-gc-high-threshold=85 --node-status-update-frequency=10s"
		Expect(expectStr).To(Equal(actucalStr))
	})

	It("should return expected kubelet config when custom configuration is set", func() {
		config := &datamodel.NodeBootstrappingConfiguration{
			KubeletConfig: map[string]string{
				"--node-status-update-frequency": "10s",
				"--node-status-report-frequency": "10s",
				"--image-gc-high-threshold":      "85",
				"--event-qps":                    "0",
			},
			ContainerService: &datamodel.ContainerService{
				Location: "southcentralus",
				Type:     "Microsoft.ContainerService/ManagedClusters",
				Properties: &datamodel.Properties{
					CustomConfiguration: &datamodel.CustomConfiguration{
						KubernetesConfigurations: map[string]*datamodel.ComponentConfiguration{
							"kubelet": {
								Config: map[string]string{
									"--node-status-update-frequency":      "20s",
									"--streaming-connection-idle-timeout": "4h0m0s",
									"--seccomp-default":                   "true",
								},
							},
						},
					},
				},
			},
			EnableKubeletConfigFile: false,
			AgentPoolProfile:        &datamodel.AgentPoolProfile{},
		}

		expectStr := "--event-qps=0 --image-gc-high-threshold=85 --node-status-update-frequency=20s --seccomp-default=true --streaming-connection-idle-timeout=4h0m0s"
		actucalStr := GetOrderedKubeletConfigFlagString(config)
		Expect(expectStr).To(Equal(actucalStr))
	})

	It("should return expected kubelet command line flags when a config file is being used", func() {
		config := &datamodel.NodeBootstrappingConfiguration{
			KubeletConfig: map[string]string{
				"--node-labels":                  "topology.kubernetes.io/region=southcentralus",
				"--node-status-update-frequency": "10s",
				"--node-status-report-frequency": "5m0s",
				"--image-gc-high-threshold":      "85",
				"--event-qps":                    "0",
			},
			ContainerService: &datamodel.ContainerService{
				Location: "southcentralus",
				Type:     "Microsoft.ContainerService/ManagedClusters",
				Properties: &datamodel.Properties{
					OrchestratorProfile: &datamodel.OrchestratorProfile{
						OrchestratorType:    "Kubernetes",
						OrchestratorVersion: "1.22.11",
					},
				},
			},
			EnableKubeletConfigFile: true,
			AgentPoolProfile:        &datamodel.AgentPoolProfile{},
		}

		expectedStr := "--node-labels=topology.kubernetes.io/region=southcentralus"
		actualStr := GetOrderedKubeletConfigFlagString(config)
		Expect(expectedStr).To(Equal(actualStr))
	})

	//https://kubernetes.io/docs/tasks/administer-cluster/kubelet-config-file/#kubelet-configuration-merging-order
	// [we are producing this -> command line flags] > drop in config file > kubelet config
	It("should return expected kubelet command line flags when a config file is being used, following overriding rules", func() {
		config := &datamodel.NodeBootstrappingConfiguration{
			KubeletConfig: map[string]string{
				"--node-labels": "topology.kubernetes.io/region=southcentralus",
			},
			ContainerService: &datamodel.ContainerService{
				Location: "southcentralus",
				Type:     "Microsoft.ContainerService/ManagedClusters",
				Properties: &datamodel.Properties{
					CustomConfiguration: &datamodel.CustomConfiguration{
						KubernetesConfigurations: map[string]*datamodel.ComponentConfiguration{
							"kubelet": {
								Config: map[string]string{
									"--seccomp-default": "true",
								},
							},
						},
					},
				},
			},
			EnableKubeletConfigFile: true,
			AgentPoolProfile: &datamodel.AgentPoolProfile{
				CustomKubeletConfig: &datamodel.CustomKubeletConfig{
					SeccompDefault: to.BoolPtr(false),
				},
			},
		}

		expectedStr := "--node-labels=topology.kubernetes.io/region=southcentralus --seccomp-default=true"
		actualStr := GetOrderedKubeletConfigFlagString(config)
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

//nolint:lll
var _ = Describe("Test removeComments", func() {

	It("Should leave lines without comments unchanged", func() {
		input := []byte("#!/bin/bash\n\nCC_SERVICE_IN_TMP=/opt/azure/containers/cc-proxy.service.in\nCC_SOCKET_IN_TMP=/opt/azure/containers/cc-proxy.socket.in\nCNI_CONFIG_DIR=\"/etc/cni/net.d\"\nCNI_BIN_DIR=\"/opt/cni/bin\"\nCNI_DOWNLOADS_DIR=\"/opt/cni/downloads\"\nCRICTL_DOWNLOAD_DIR=\"/opt/crictl/downloads\"\nCRICTL_BIN_DIR=\"/opt/bin\"\nCONTAINERD_DOWNLOADS_DIR=\"/opt/containerd/downloads\"\nRUNC_DOWNLOADS_DIR=\"/opt/runc/downloads\"\nK8S_DOWNLOADS_DIR=\"/opt/kubernetes/downloads\"\nUBUNTU_RELEASE=$(lsb_release -r -s)\nSECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_DOWNLOAD_DIR=\"/opt/azure/tlsbootstrap\"\nSECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_VERSION=\"v0.1.0-alpha.2\"\nCONTAINERD_WASM_VERSIONS=\"v0.3.0 v0.5.1 v0.8.0\"\nMANIFEST_FILEPATH=\"/opt/azure/manifest.json\"\nMAN_DB_AUTO_UPDATE_FLAG_FILEPATH=\"/var/lib/man-db/auto-update\"\nCURL_OUTPUT=/tmp/curl_verbose.out")
		expected := "#!/bin/bash\n\nCC_SERVICE_IN_TMP=/opt/azure/containers/cc-proxy.service.in\nCC_SOCKET_IN_TMP=/opt/azure/containers/cc-proxy.socket.in\nCNI_CONFIG_DIR=\"/etc/cni/net.d\"\nCNI_BIN_DIR=\"/opt/cni/bin\"\nCNI_DOWNLOADS_DIR=\"/opt/cni/downloads\"\nCRICTL_DOWNLOAD_DIR=\"/opt/crictl/downloads\"\nCRICTL_BIN_DIR=\"/opt/bin\"\nCONTAINERD_DOWNLOADS_DIR=\"/opt/containerd/downloads\"\nRUNC_DOWNLOADS_DIR=\"/opt/runc/downloads\"\nK8S_DOWNLOADS_DIR=\"/opt/kubernetes/downloads\"\nUBUNTU_RELEASE=$(lsb_release -r -s)\nSECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_DOWNLOAD_DIR=\"/opt/azure/tlsbootstrap\"\nSECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_VERSION=\"v0.1.0-alpha.2\"\nCONTAINERD_WASM_VERSIONS=\"v0.3.0 v0.5.1 v0.8.0\"\nMANIFEST_FILEPATH=\"/opt/azure/manifest.json\"\nMAN_DB_AUTO_UPDATE_FLAG_FILEPATH=\"/var/lib/man-db/auto-update\"\nCURL_OUTPUT=/tmp/curl_verbose.out"
		result := removeComments(input)
		Expect(string(result)).To(Equal(expected))
	})

	It("Should remove lines that start with comments", func() {

		input := []byte("#!/bin/bash\nERR_FILE_WATCH_TIMEOUT=6 \nset -x\n# this is a test comment before if block\nif [ -f /opt/azure/containers/provision.complete ]; then\n      echo \"Already ran to success exiting...\"\n      exit 0\nfi\n# this is a test comment\n\naptmarkWALinuxAgent hold &")
		expected := "#!/bin/bash\nERR_FILE_WATCH_TIMEOUT=6 \nset -x\nif [ -f /opt/azure/containers/provision.complete ]; then\n      echo \"Already ran to success exiting...\"\n      exit 0\nfi\n\naptmarkWALinuxAgent hold &"
		result := removeComments(input)
		Expect(string(result)).To(Equal(expected))
	})

	It("Should remove lines with trailing comments", func() {
		input := []byte("#!/bin/bash\nERR_FILE_WATCH_TIMEOUT=6 \nset -x # this is first test trailing comment\nif [ -f /opt/azure/containers/provision.complete ]; then\n      echo \"Already ran to success exiting...\"\n      exit 0 # this is another test trailing comment\nfi\n\naptmarkWALinuxAgent hold &")
		expected := "#!/bin/bash\nERR_FILE_WATCH_TIMEOUT=6 \nset -x \nif [ -f /opt/azure/containers/provision.complete ]; then\n      echo \"Already ran to success exiting...\"\n      exit 0 \nfi\n\naptmarkWALinuxAgent hold &"
		result := removeComments(input)
		Expect(string(result)).To(Equal(expected))
	})

	It("Should leave lines have no whitespace after first hash sign in the beginning of line unchanged", func() {
		input := []byte("#!/bin/bash\nERR_FILE_WATCH_TIMEOUT=6 \nset -x\nif [ -f /opt/azure/containers/provision.complete ]; then\n      echo \"Already ran to success exiting...\"\n      exit 0\nfi\n#test line that has no whitespace after hash\n\naptmarkWALinuxAgent hold &")
		expected := "#!/bin/bash\nERR_FILE_WATCH_TIMEOUT=6 \nset -x\nif [ -f /opt/azure/containers/provision.complete ]; then\n      echo \"Already ran to success exiting...\"\n      exit 0\nfi\n#test line that has no whitespace after hash\n\naptmarkWALinuxAgent hold &"
		result := removeComments(input)
		Expect(string(result)).To(Equal(expected))
	})

	It("Should not remove lines that log hash signs in strings", func() {
		input := []byte("#!/bin/bash\nERR_FILE_WATCH_TIMEOUT=6 \nset -x\nif [ -f /opt/azure/containers/provision.complete ]; then\n      echo \"Already ran to success exiting...\"\n      exit 0\nfi\necho \"## modified - something\"\n\naptmarkWALinuxAgent hold &")
		expected := "#!/bin/bash\nERR_FILE_WATCH_TIMEOUT=6 \nset -x\nif [ -f /opt/azure/containers/provision.complete ]; then\n      echo \"Already ran to success exiting...\"\n      exit 0\nfi\necho \"## modified - something\"\n\naptmarkWALinuxAgent hold &"
		result := removeComments(input)
		Expect(string(result)).To(Equal(expected))
	})

})

// repoRoot returns the path to the AgentBaker repository root by walking up from the
// current test file until we find go.mod. This avoids hard-coding absolute paths.
func repoRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("unable to determine test file path")
	}
	dir := filepath.Dir(filename)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find repo root (go.mod)")
		}
		dir = parent
	}
}

// TestRemoveComments_ShellPatterns tests removeComments against realistic shell script
// patterns that have historically caused issues, particularly patterns where '#' appears
// inside string literals or in non-comment contexts.
//
// TestRemoveComments_ShellPatterns validates that removeComments correctly handles
// various shell script patterns without breaking functional code.
//
// Background: removeComments is a "best-effort" comment stripper (utils.go:202) that runs on
// all CSE shell scripts before template execution. It must not mangle code that contains
// '#' characters in non-comment contexts (string literals, variable expansions, grep patterns).
func TestRemoveComments_ShellPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "pure comment lines are removed",
			input: strings.Join([]string{
				"#!/bin/bash",
				"# This is a comment",
				"echo hello",
				"## Another comment",
				"echo world",
			}, "\n"),
			expected: strings.Join([]string{
				"#!/bin/bash",
				"echo hello",
				"echo world",
			}, "\n"),
		},
		{
			name: "hash inside quoted grep pattern is preserved",
			input: strings.Join([]string{
				`    if grep -q "^#${mod} " /proc/modules 2>/dev/null; then`,
				`        modprobe -r "$mod"`,
				`    fi`,
			}, "\n"),
			expected: strings.Join([]string{
				`    if grep -q "^#${mod} " /proc/modules 2>/dev/null; then`,
				`        modprobe -r "$mod"`,
				`    fi`,
			}, "\n"),
		},
		{
			name: "trailing comments are trimmed but code is preserved",
			input: strings.Join([]string{
				`    local mod="$1" # module name`,
				`    modprobe -r "$mod" # try to unload`,
			}, "\n"),
			expected: strings.Join([]string{
				`    local mod="$1" `,
				`    modprobe -r "$mod" `,
			}, "\n"),
		},
		{
			name:     "shebang line is preserved",
			input:    "#!/bin/bash\nset -euo pipefail",
			expected: "#!/bin/bash\nset -euo pipefail",
		},
		{
			name: "hash in variable expansion is not a comment",
			input: strings.Join([]string{
				`    local count=${#array[@]}`,
				`    echo "${str#prefix}"`,
				`    echo "${str##*/}"`,
			}, "\n"),
			expected: strings.Join([]string{
				`    local count=${#array[@]}`,
				`    echo "${str#prefix}"`,
				`    echo "${str##*/}"`,
			}, "\n"),
		},
		{
			// Documents the DOA regression from PR #8475: a line starting with "# "
			// inside a multi-line printf format string gets stripped by removeComments,
			// breaking the script. The fix (PR #8486) was to not emit "# " lines from
			// code. This test asserts the current (known-limitation) behavior.
			name: "line starting with hash-space is stripped even inside string context",
			input: strings.Join([]string{
				`myFunc() {`,
				`    local desc="$1"`,
				`    printf '# %s\ninstall %s /bin/false\n' "$desc" "$mod"`,
				`}`,
			}, "\n"),
			expected: strings.Join([]string{
				`myFunc() {`,
				`    local desc="$1"`,
				`    printf '`,
				`}`,
			}, "\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeComments([]byte(tt.input))
			if diff := cmp.Diff(tt.expected, string(result)); diff != "" {
				t.Errorf("removeComments() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestCSEScriptRoundTrip exercises the full CSE assembly pipeline for each embedded shell
// script: removeComments → gzip → base64 → base64-decode → gunzip, then validates:
//   - byte-for-byte round-trip integrity (decoded output == stripped input)
//   - bash -n syntax check on the decoded output (catches broken scripts)
//
// This exercises the comment-stripping and encoding stages of the production pipeline
// in getBase64EncodedGzippedCustomScript() (pkg/agent/utils.go). The Go template
// execution step is not included here since it requires a full NodeBootstrappingConfiguration.
// The comment stripping happens BEFORE template execution, so the stripped output must
// still be syntactically valid bash — a node cannot provision if any CSE script has a
// syntax error after stripping.
//
// The script list is dynamically derived by parsing variables.go and const.go source
// to find all .sh files passed to getBase64EncodedGzippedCustomScript(). If a new script
// is added to the CSE pipeline, it is automatically covered by this test.
func TestCSEScriptRoundTrip(t *testing.T) {
	cseScripts := discoverCSEScripts(t)
	if len(cseScripts) == 0 {
		t.Fatal("no CSE scripts discovered — check variables.go and const.go parsing")
	}
	t.Logf("discovered %d CSE shell scripts", len(cseScripts))

	artifactsDir := filepath.Join(repoRoot(), "parts")

	for _, script := range cseScripts {
		t.Run(filepath.Base(script), func(t *testing.T) {
			decoded := cseRoundTrip(t, filepath.Join(artifactsDir, script))
			cseValidateBashSyntax(t, script, decoded)
		})
	}
}

// discoverCSEScripts parses variables.go to find all constant names passed to
// getBase64EncodedGzippedCustomScript(), then resolves those constants to file
// paths from const.go, filtering to .sh files only.
func discoverCSEScripts(t *testing.T) []string {
	t.Helper()
	root := repoRoot()

	// Step 1: Read variables.go and extract constant names from getBase64EncodedGzippedCustomScript() calls
	variablesPath := filepath.Join(root, "pkg", "agent", "variables.go")
	variablesBytes, err := os.ReadFile(variablesPath)
	if err != nil {
		t.Fatalf("failed to read variables.go: %v", err)
	}

	// Match: getBase64EncodedGzippedCustomScript(constantName, config)
	callRe := regexp.MustCompile(`getBase64EncodedGzippedCustomScript\((\w+),`)
	matches := callRe.FindAllStringSubmatch(string(variablesBytes), -1)
	constNames := make(map[string]bool)
	for _, m := range matches {
		constNames[m[1]] = true
	}

	// Step 2: Read const.go and resolve constant names to file paths
	constPath := filepath.Join(root, "pkg", "agent", "const.go")
	constBytes, err := os.ReadFile(constPath)
	if err != nil {
		t.Fatalf("failed to read const.go: %v", err)
	}

	// Match: constantName = "linux/cloud-init/artifacts/..."
	constRe := regexp.MustCompile(`(\w+)\s*=\s*"([^"]+)"`)
	constMatches := constRe.FindAllStringSubmatch(string(constBytes), -1)
	constMap := make(map[string]string)
	for _, m := range constMatches {
		constMap[m[1]] = m[2]
	}

	// Step 3: Resolve and filter to .sh files
	var scripts []string
	seen := make(map[string]bool)
	for name := range constNames {
		path, ok := constMap[name]
		if !ok {
			continue
		}
		if !strings.HasSuffix(path, ".sh") {
			continue
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		scripts = append(scripts, path)
	}
	sort.Strings(scripts)
	return scripts
}

// cseRoundTrip reads a shell script, runs it through the production CSE pipeline
// (removeComments → gzip → base64 → decode → gunzip), validates byte-for-byte
// round-trip integrity, and returns the decoded output.
func cseRoundTrip(t *testing.T, path string) []byte {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	stripped := removeComments(raw)
	encoded := getBase64EncodedGzippedCustomScriptFromStr(string(stripped))

	gzipped, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	decoded, err := getGzipDecodedValue(gzipped)
	if err != nil {
		t.Fatalf("gzip decode failed: %v", err)
	}

	if diff := cmp.Diff(string(stripped), string(decoded)); diff != "" {
		t.Errorf("round-trip mismatch (-stripped +decoded):\n%s", diff)
	}

	return decoded
}

// cseValidateBashSyntax runs bash -n on the decoded script to catch syntax errors
// introduced by comment stripping. Skips scripts with Go template directives.
func cseValidateBashSyntax(t *testing.T, script string, decoded []byte) {
	t.Helper()

	if strings.Contains(string(decoded), "{{") {
		t.Logf("skipping bash -n for %s (contains Go template directives)", script)
		return
	}

	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available, skipping syntax check")
	}

	tmpFile, err := os.CreateTemp("", "cse-roundtrip-*.sh")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, writeErr := tmpFile.Write(decoded)
	tmpFile.Close()
	if writeErr != nil {
		t.Fatalf("failed to write temp file: %v", writeErr)
	}

	cmd := exec.Command(bashPath, "-O", "extglob", "-n", tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("bash -n syntax check FAILED for %s after removeComments + round-trip:\n%s\n%s",
			script, string(output), err)
	}
}

func TestValidateAndSetLinuxNodeBootstrappingConfiguration_StreamingConnectionIdleTimeout(t *testing.T) {
	testCases := []struct {
		name          string
		version       string
		expectRemoved bool
	}{
		{
			name:          "k8s 1.33 keeps streaming-connection-idle-timeout",
			version:       "1.33.0",
			expectRemoved: false,
		},
		{
			name:          "k8s 1.34.0 removes streaming-connection-idle-timeout",
			version:       "1.34.0",
			expectRemoved: true,
		},
		{
			name:          "k8s 1.35.0 removes streaming-connection-idle-timeout",
			version:       "1.35.0",
			expectRemoved: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &datamodel.NodeBootstrappingConfiguration{
				ContainerService: &datamodel.ContainerService{
					Properties: &datamodel.Properties{
						OrchestratorProfile: &datamodel.OrchestratorProfile{
							OrchestratorVersion: tc.version,
						},
					},
				},
				KubeletConfig: map[string]string{
					"--streaming-connection-idle-timeout": "4h0m0s",
					"--feature-gates":                     "",
				},
			}

			ValidateAndSetLinuxNodeBootstrappingConfiguration(config)

			_, exists := config.KubeletConfig["--streaming-connection-idle-timeout"]
			if tc.expectRemoved && exists {
				t.Fatalf("expected --streaming-connection-idle-timeout to be removed for k8s %s", tc.version)
			}
			if !tc.expectRemoved && !exists {
				t.Fatalf("expected --streaming-connection-idle-timeout to be kept for k8s %s", tc.version)
			}
		})
	}
}
