// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"testing"

	"github.com/google/go-cmp/cmp"
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
	configFileStr := getDynamicKubeletConfigFileContent(kc)
	diff := cmp.Diff(expectedJSON, configFileStr)
	if diff != "" {
		t.Errorf("Generated config file is different than expected: %s", diff)
	}
}

var expectedJSON string = `{
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
    "clusterDomain": "cluster.local",
    "clusterDNS": [
        "10.0.0.10"
    ],
    "streamingConnectionIdleTimeout": "4h0m0s",
    "nodeStatusUpdateFrequency": "10s",
    "imageGCHighThresholdPercent": 85,
    "imageGCLowThresholdPercent": 80,
    "cgroupsPerQOS": true,
    "maxPods": 110,
    "podPidsLimit": -1,
    "resolvConf": "/etc/resolv.conf",
    "evictionHard": {
        "memory.available": "750Mi",
        "nodefs.available": "10%",
        "nodefs.inodesFree": "5%"
    },
    "protectKernelDefaults": true,
    "featureGates": {
        "DynamicKubeletConfig": false,
        "RotateKubeletServerCertificate": true
    },
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
    ]
}`
