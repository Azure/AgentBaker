/*
Portions Copyright (c) Microsoft Corporation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package parser

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/google/go-cmp/cmp"
)

var expectedKubeletConfigFlags = "--address=0.0.0.0" +
	" --anonymous-auth=false" +
	" --authentication-token-webhook=true" +
	" --authorization-mode=Webhook" +
	" --cgroups-per-qos=true" +
	" --client-ca-file=/etc/kubernetes/certs/ca.crt" +
	" --cluster-dns=10.0.0.10" +
	" --cluster-domain=cluster.local" +
	" --enforce-node-allocatable=pods" +
	" --event-qps=0" +
	" --eviction-hard=memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%" +
	" --feature-gates=RotateKubeletServerCertificate=true,DynamicKubeletConfig=false" +
	" --image-gc-high-threshold=85" +
	" --image-gc-low-threshold=80" +
	" --kube-reserved=cpu=100m,memory=1638Mi" +
	" --max-pods=110" +
	" --node-status-update-frequency=10s" +
	" --pod-manifest-path=/etc/kubernetes/manifests" +
	" --pod-max-pids=-1" +
	" --protect-kernel-defaults=true" +
	" --read-only-port=10255" +
	" --resolv-conf=/etc/resolv.conf" +
	" --rotate-certificates=true" +
	" --rotate-server-certificates=true" +
	" --streaming-connection-idle-timeout=4h0m0s" +
	" --system-reserved=cpu=2,memory=1Gi" +
	" --tls-cert-file=/etc/kubernetes/certs/kubeletserver.crt" +
	" --tls-cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256," +
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305," +
	"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256" +
	" --tls-private-key-file=/etc/kubernetes/certs/kubeletserver.key"

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
        }
    },
    "authorization": {
        "mode": "Webhook"
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

func Test_getSysctlContent(t *testing.T) {
	// Test_getSysctlContent tests the getSysctlContent function.
	type args struct {
		s *aksnodeconfigv1.SysctlConfig
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Default SysctlConfig",
			args: args{
				s: &aksnodeconfigv1.SysctlConfig{},
			},
			want: base64.StdEncoding.EncodeToString(
				[]byte(`net.core.message_burst=80
net.core.message_cost=40
net.core.somaxconn=16384
net.ipv4.neigh.default.gc_thresh1=4096
net.ipv4.neigh.default.gc_thresh2=8192
net.ipv4.neigh.default.gc_thresh3=16384
net.ipv4.tcp_max_syn_backlog=16384
net.ipv4.tcp_retries2=8`)),
		},
		{
			name: "SysctlConfig with custom values",
			args: args{
				s: &aksnodeconfigv1.SysctlConfig{
					NetIpv4TcpMaxSynBacklog: to.Ptr(int32(9999)),
					NetCoreRmemDefault:      to.Ptr(int32(9999)),
					NetIpv4IpLocalPortRange: to.Ptr("32768 62535"),
				},
			},
			want: base64.StdEncoding.EncodeToString(
				[]byte(`net.core.message_burst=80
net.core.message_cost=40
net.core.rmem_default=9999
net.core.somaxconn=16384
net.ipv4.ip_local_port_range=32768 62535
net.ipv4.neigh.default.gc_thresh1=4096
net.ipv4.neigh.default.gc_thresh2=8192
net.ipv4.neigh.default.gc_thresh3=16384
net.ipv4.tcp_max_syn_backlog=9999
net.ipv4.tcp_retries2=8`)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getSysctlContent(tt.args.s); got != tt.want {
				t.Errorf("getSysctlContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getUlimitContent(t *testing.T) {
	type args struct {
		u *aksnodeconfigv1.UlimitConfig
	}
	str9999 := "9999"
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Default UlimitConfig",
			args: args{
				u: &aksnodeconfigv1.UlimitConfig{},
			},
			want: "[Service]\n",
		},
		{
			name: "UlimitConfig with custom values",
			args: args{
				u: &aksnodeconfigv1.UlimitConfig{
					NoFile:          &str9999,
					MaxLockedMemory: &str9999,
				},
			},
			want: "[Service]\nLimitMEMLOCK=9999 LimitNOFILE=9999",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getUlimitContent(tt.args.u); got != tt.want {
				t.Errorf("getUlimitContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_createSortedKeyValueStringPairs(t *testing.T) {
	type args struct {
		m         map[string]interface{}
		delimiter string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Empty map",
			args: args{
				m:         map[string]interface{}{},
				delimiter: ",",
			},
			want: "",
		},
		{
			name: "Single key-value pair",
			args: args{
				m:         map[string]interface{}{"key1": "value1"},
				delimiter: " ",
			},
			want: "key1=value1",
		},
		{
			name: "Multiple key-value pairs with delimiter ,",
			args: args{
				m:         map[string]interface{}{"key1": "value1", "key2": "value2"},
				delimiter: ",",
			},
			want: "key1=value1,key2=value2",
		},
		{
			name: "Multiple key-value pairs with delimiter space",
			args: args{
				m:         map[string]interface{}{"key1": "value1", "key2": "value2"},
				delimiter: " ",
			},
			want: "key1=value1 key2=value2",
		},
		{
			name: "Sorting key-value pairs",
			args: args{
				m:         map[string]interface{}{"b": "valb", "a": "vala", "c": "valc"},
				delimiter: ",",
			},
			want: "a=vala,b=valb,c=valc",
		},
		{
			name: "Multiple key-value pairs with delimiter line breaker \\n where values are a combination of strings and integers",
			args: args{
				m:         map[string]interface{}{"key1": "value1", "key2": "value2", "key3": 3, "key4": 4},
				delimiter: "\n",
			},
			want: "key1=value1\nkey2=value2\nkey3=3\nkey4=4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createSortedKeyValuePairs(tt.args.m, tt.args.delimiter); got != tt.want {
				t.Errorf("createSortedKeyValuePairs() with map[string]string = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_createSortedKeyValueInt32Pairs(t *testing.T) {
	type args struct {
		m         map[string]int32
		delimiter string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Empty map",
			args: args{
				m:         map[string]int32{},
				delimiter: ",",
			},
			want: "",
		},
		{
			name: "Single key-value pair",
			args: args{
				m:         map[string]int32{"key1": 1},
				delimiter: " ",
			},
			want: "key1=1",
		},
		{
			name: "Multiple key-value pairs",
			args: args{
				m:         map[string]int32{"key1": 1, "key2": 2},
				delimiter: ",",
			},
			want: "key1=1,key2=2",
		},
		{
			name: "Multiple key-value pairs with delimiter space",
			args: args{
				m:         map[string]int32{"key1": 1, "key2": 2},
				delimiter: " ",
			},
			want: "key1=1 key2=2",
		},
		{
			name: "Sorting key-value pairs",
			args: args{
				m:         map[string]int32{"b": 2, "a": 1, "c": 3},
				delimiter: ",",
			},
			want: "a=1,b=2,c=3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createSortedKeyValuePairs(tt.args.m, tt.args.delimiter); got != tt.want {
				t.Errorf("createSortedKeyValuePairs() with map[string]int32 = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getContainerdConfig(t *testing.T) {
	type args struct {
		aksnodeconfig *aksnodeconfigv1.Configuration
		noGpu         bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Default Containerd Configurations",
			args: args{
				aksnodeconfig: &aksnodeconfigv1.Configuration{
					NeedsCgroupv2: to.Ptr(true),
				},
			},
			want: base64.StdEncoding.EncodeToString([]byte(`version = 2
oom_score = -999
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  enable_cdi = true
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
`)),
		},
		{
			name: "Default Containerd Configurations with NeedsCgroupv2 unset",
			args: args{
				aksnodeconfig: &aksnodeconfigv1.Configuration{
					NeedsCgroupv2: to.Ptr(false),
				},
			},
			want: base64.StdEncoding.EncodeToString([]byte(`version = 2
oom_score = -999
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  enable_cdi = true
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
`)),
		},
		{
			name: "Containerd Configurations with bool noGpu set to false",
			args: args{
				aksnodeconfig: &aksnodeconfigv1.Configuration{
					NeedsCgroupv2: to.Ptr(true),
					GpuConfig: &aksnodeconfigv1.GpuConfig{
						EnableNvidia: to.Ptr(true),
					},
				},
				noGpu: false,
			},
			want: base64.StdEncoding.EncodeToString([]byte(`version = 2
oom_score = -999
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  enable_cdi = true
  [plugins."io.containerd.grpc.v1.cri".containerd]
    default_runtime_name = "nvidia-container-runtime"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia-container-runtime]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia-container-runtime.options]
      BinaryName = "/usr/bin/nvidia-container-runtime"
      SystemdCgroup = true
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/nvidia-container-runtime"
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
`)),
		},
		{
			name: "Containerd Configurations with bool noGpu set to true",
			args: args{
				aksnodeconfig: &aksnodeconfigv1.Configuration{
					NeedsCgroupv2: to.Ptr(true),
				},
				noGpu: true,
			},
			want: base64.StdEncoding.EncodeToString([]byte(`version = 2
oom_score = -999
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  enable_cdi = true
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
`)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getContainerdConfigBase64(tt.args.aksnodeconfig); got != tt.want {
				t.Errorf("getContainerdConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getKubenetTemplate(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "Kubenet template",
			want: base64.StdEncoding.EncodeToString([]byte(`{
	"cniVersion": "0.3.1",
	"name": "kubenet",
	"plugins": [{
		"type": "bridge",
		"bridge": "cbr0",
		"mtu": 1500,
		"addIf": "eth0",
		"isGateway": true,
		"ipMasq": false,
		"promiscMode": true,
		"hairpinMode": false,
		"ipam": {
			"type": "host-local",
			"ranges": [{{range $i, $range := .PodCIDRRanges}}{{if $i}}, {{end}}[{"subnet": "{{$range}}"}]{{end}}],
			"routes": [{{range $i, $route := .Routes}}{{if $i}}, {{end}}{"dst": "{{$route}}"}{{end}}]
		}
	},
	{
		"type": "portmap",
		"capabilities": {"portMappings": true},
		"externalSetMarkChain": "KUBE-MARK-MASQ"
	}]
}
`)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getKubenetTemplate(); got != tt.want {
				t.Errorf("getKubenetTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getAzureEnvironmentFilepath(t *testing.T) {
	type args struct {
		v *aksnodeconfigv1.Configuration
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Nil CustomCloudConfig",
			args: args{
				v: &aksnodeconfigv1.Configuration{},
			},
			want: "",
		},
		{
			name: "Empty AzureEnvironmentFilepath",
			args: args{
				v: &aksnodeconfigv1.Configuration{
					CustomCloudConfig: &aksnodeconfigv1.CustomCloudConfig{},
				},
			},
			want: "",
		},
		{
			name: "AzureEnvironmentFilepath when it is AKSCustomCloud",
			args: args{
				v: &aksnodeconfigv1.Configuration{
					CustomCloudConfig: &aksnodeconfigv1.CustomCloudConfig{
						CustomCloudEnvName: helpers.AksCustomCloudName,
					},
				},
			},
			want: "/etc/kubernetes/akscustom.json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getAzureEnvironmentFilepath(tt.args.v); got != tt.want {
				t.Errorf("getAzureEnvironmentFilepath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getEnsureNoDupePromiscuousBridge(t *testing.T) {
	type args struct {
		nc *aksnodeconfigv1.NetworkConfig
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "NetworkConfig with no promiscuous bridge",
			args: args{
				nc: &aksnodeconfigv1.NetworkConfig{
					NetworkPlugin: aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_AZURE,
					NetworkPolicy: aksnodeconfigv1.NetworkPolicy_NETWORK_POLICY_AZURE,
				},
			},
			want: false,
		},
		{
			name: "NetworkConfig with promiscuous bridge",
			args: args{
				nc: &aksnodeconfigv1.NetworkConfig{
					NetworkPlugin: aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_KUBENET,
					NetworkPolicy: aksnodeconfigv1.NetworkPolicy_NETWORK_POLICY_AZURE,
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getEnsureNoDupePromiscuousBridge(tt.args.nc); got != tt.want {
				t.Errorf("getEnsureNoDupePromiscuousBridge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getHasSearchDomain(t *testing.T) {
	type args struct {
		csd *aksnodeconfigv1.CustomSearchDomainConfig
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "CustomSearchDomain with empty search domain should return false",
			args: args{
				csd: &aksnodeconfigv1.CustomSearchDomainConfig{},
			},
			want: false,
		},
		{
			name: "CustomSearchDomain with empty search domain user should return false",
			args: args{
				csd: &aksnodeconfigv1.CustomSearchDomainConfig{
					DomainName:    "fakedomain.com",
					RealmPassword: "fakepassword",
				},
			},
			want: false,
		},
		{
			name: "CustomSearchDomain with search domain, user and password should return true",
			args: args{
				csd: &aksnodeconfigv1.CustomSearchDomainConfig{
					DomainName:    "fakedomain.com",
					RealmUser:     "fakeuser",
					RealmPassword: "fakepassword",
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getHasSearchDomain(tt.args.csd); got != tt.want {
				t.Errorf("getHasSearchDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getCustomCACertsStatus(t *testing.T) {
	type args struct {
		customCACerts []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Empty customCACerts",
			args: args{
				customCACerts: []string{},
			},
			want: false,
		},
		{
			name: "Non-empty customCACerts",
			args: args{
				customCACerts: []string{"cert1", "cert2"},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCustomCACertsStatus(tt.args.customCACerts); got != tt.want {
				t.Errorf("getCustomCACertsStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getStringifiedStringArray(t *testing.T) {
	type args struct {
		arr       []string
		delimiter string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Empty array input should return empty string",
			args: args{
				arr:       []string{},
				delimiter: ",",
			},
			want: "",
		},
		{
			name: "Single element array input should return the element without delimiter",
			args: args{
				arr:       []string{"element1"},
				delimiter: ",",
			},
			want: "element1",
		},
		{
			name: "Multiple element array input should return the elements separated by delimiter",
			args: args{
				arr:       []string{"element1", "element2", "element3"},
				delimiter: ",",
			},
			want: "element1,element2,element3",
		},
		{
			name: "Multiple element array input with space delimiter should return the elements separated by space",
			args: args{
				arr:       []string{"element1", "element2", "element3"},
				delimiter: " ",
			},
			want: "element1 element2 element3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getStringifiedStringArray(tt.args.arr, tt.args.delimiter); got != tt.want {
				t.Errorf("getStringifiedStringArray() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_deref(t *testing.T) {
	type args struct {
		p *interface{}
	}
	var s interface{} = "test"
	var i interface{} = 123
	var b interface{} = true
	tests := []struct {
		name string
		args args
		want interface{}
	}{
		{
			name: "Dereference nil pointer should not panic and should return interface{}(nil)",
			args: args{
				p: nil,
			},
			want: interface{}(nil),
		},
		{
			name: "Dereference pointer to string should return the string",
			args: args{
				p: &s,
			},
			want: "test",
		},
		{
			name: "Dereference pointer to int should return the int",
			args: args{
				p: &i,
			},
			want: 123,
		},
		{
			name: "Dereference pointer to bool should return the bool",
			args: args{
				p: &b,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deref(tt.args.p); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("deref() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsKubernetesVersionGe(t *testing.T) {
	type args struct {
		actualVersion string
		version       string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "1.24.5 is greater than or equal to 1.24.5",
			args: args{
				actualVersion: "1.24.5",
				version:       "1.24.5",
			},
			want: true,
		},
		{
			name: "1.23.5 is greater than or equal to 1.23.4",
			args: args{
				actualVersion: "1.23.5",
				version:       "1.23.4",
			},
			want: true,
		},
		{
			name: "1.23.5 is not greater than or equal to 1.24.4",
			args: args{
				actualVersion: "1.23.5",
				version:       "1.24.4",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := helpers.IsKubernetesVersionGe(tt.args.actualVersion, tt.args.version); got != tt.want {
				t.Errorf("IsKubernetesVersionGe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPortRangeEndValue(t *testing.T) {
	type args struct {
		portRange string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "empty string",
			args: args{
				portRange: "",
			},
			want: -1,
		},
		{
			name: "Port range with single port",
			args: args{
				portRange: "80",
			},
			want: -1,
		},
		{
			name: "Port range with valid start and end port",
			args: args{
				portRange: "80 90",
			},
			want: 90,
		},
		{
			name: "Port range with start and end port separated by - (invalid delimiter)",
			args: args{
				portRange: "80-90",
			},
			want: -1,
		},
		{
			name: "Port range with 3 ports (invalid)",
			args: args{
				portRange: "80 90 100",
			},
			want: -1,
		},
		{
			name: "start value is larger than end value (invalid)",
			args: args{
				portRange: "110 90",
			},
			want: -1,
		},
		{
			name: "start value is equal to end value (invalid)",
			args: args{
				portRange: "80 80",
			},
			want: -1,
		},
		{
			name: "either value is not a number (invalid)",
			args: args{
				portRange: "80 abc",
			},
			want: -1,
		},
		{
			name: "either value is smaller than 0 (invalid)",
			args: args{
				portRange: "-1 80",
			},
			want: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPortRangeEndValue(tt.args.portRange); got != tt.want {
				t.Errorf("getPortRangeEndValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getShouldConfigureHTTPProxy(t *testing.T) {
	type args struct {
		httpProxyConfig *aksnodeconfigv1.HttpProxyConfig
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Nil HTTPProxyConfig",
			args: args{},
			want: false,
		},
		{
			name: "Empty HTTPProxyConfig",
			args: args{
				httpProxyConfig: &aksnodeconfigv1.HttpProxyConfig{},
			},
			want: false,
		},
		{
			name: "HTTPProxyConfig with empty HttpProxy and valid HttpsProxy",
			args: args{
				httpProxyConfig: &aksnodeconfigv1.HttpProxyConfig{
					HttpProxy:  "",
					HttpsProxy: "https://fakeproxy.com:8080",
				},
			},
			want: true,
		},
		{
			name: "HTTPProxyConfig with valid HttpProxy and empty HttpsProxy",
			args: args{
				httpProxyConfig: &aksnodeconfigv1.HttpProxyConfig{
					HttpProxy:  "http://fakeproxy.com:8080",
					HttpsProxy: "",
				},
			},
			want: true,
		},
		{
			name: "HTTPProxyConfig with empty HttpProxy and empty HttpsProxy",
			args: args{
				httpProxyConfig: &aksnodeconfigv1.HttpProxyConfig{
					HttpProxy:      "",
					HttpsProxy:     "",
					NoProxyEntries: []string{"fakesite1.com", "fakesite2.com"},
				},
			},
			want: false,
		},
		{
			name: "HTTPProxyConfig with valid HttpProxy",
			args: args{
				httpProxyConfig: &aksnodeconfigv1.HttpProxyConfig{
					HttpProxy: "http://fakeproxy.com:8080",
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getShouldConfigureHTTPProxy(tt.args.httpProxyConfig); got != tt.want {
				t.Errorf("getShouldConfigureHTTPProxy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getShouldConfigureHTTPProxyCA(t *testing.T) {
	type args struct {
		httpProxyConfig *aksnodeconfigv1.HttpProxyConfig
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Nil HTTPProxyConfig",
			args: args{},
			want: false,
		},
		{
			name: "Empty HTTPProxyConfig",
			args: args{
				httpProxyConfig: &aksnodeconfigv1.HttpProxyConfig{},
			},
			want: false,
		},
		{
			name: "HTTPProxyConfig with empty CA",
			args: args{
				httpProxyConfig: &aksnodeconfigv1.HttpProxyConfig{
					HttpProxy:      "http://fakeproxy.com:8080",
					ProxyTrustedCa: "",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getShouldConfigureHTTPProxyCA(tt.args.httpProxyConfig); got != tt.want {
				t.Errorf("getShouldConfigureHTTPProxyCA() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getTargetEnvironment(t *testing.T) {
	type args struct {
		v *aksnodeconfigv1.Configuration
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Nil CustomCloudConfig",
			args: args{},
			want: helpers.DefaultCloudName,
		},
		{
			name: "Empty CustomCloudConfig",
			args: args{
				v: &aksnodeconfigv1.Configuration{
					CustomCloudConfig: &aksnodeconfigv1.CustomCloudConfig{},
				},
			},
			want: helpers.DefaultCloudName,
		},
		{
			name: "CustomCloudConfig with empty TargetEnvironment",
			args: args{
				v: &aksnodeconfigv1.Configuration{
					CustomCloudConfig: &aksnodeconfigv1.CustomCloudConfig{},
				},
			},
			want: helpers.DefaultCloudName,
		},
		{
			name: "CustomCloudConfig with TargetEnvironment",
			args: args{
				v: &aksnodeconfigv1.Configuration{
					CustomCloudConfig: &aksnodeconfigv1.CustomCloudConfig{
						CustomCloudEnvName: helpers.AksCustomCloudName,
					},
				},
			},
			want: helpers.AksCustomCloudName,
		},
		{
			name: "China location cluster config",
			args: args{
				v: &aksnodeconfigv1.Configuration{
					CustomCloudConfig: &aksnodeconfigv1.CustomCloudConfig{},
					ClusterConfig: &aksnodeconfigv1.ClusterConfig{
						Location: "china",
					},
				},
			},
			want: "AzureChinaCloud",
		},
		{
			name: "Germany location cluster config",
			args: args{
				v: &aksnodeconfigv1.Configuration{
					CustomCloudConfig: &aksnodeconfigv1.CustomCloudConfig{},
					ClusterConfig: &aksnodeconfigv1.ClusterConfig{
						Location: "germanynortheast",
					},
				},
			},
			want: "AzureGermanCloud",
		},
		{
			name: "usgov location cluster config",
			args: args{
				v: &aksnodeconfigv1.Configuration{
					CustomCloudConfig: &aksnodeconfigv1.CustomCloudConfig{},
					ClusterConfig: &aksnodeconfigv1.ClusterConfig{
						Location: "usdod",
					},
				},
			},
			want: "AzureUSGovernmentCloud",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getTargetEnvironment(tt.args.v); got != tt.want {
				t.Errorf("getTargetEnvironment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getTargetCloud(t *testing.T) {
	type args struct {
		v *aksnodeconfigv1.Configuration
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Nil CustomCloudConfig",
			args: args{},
			want: helpers.DefaultCloudName,
		},
		{
			name: "Empty CustomCloudConfig",
			args: args{
				v: &aksnodeconfigv1.Configuration{
					CustomCloudConfig: &aksnodeconfigv1.CustomCloudConfig{},
				},
			},
			want: helpers.DefaultCloudName,
		},
		{
			name: "CustomCloudConfig with TargetEnvironment",
			args: args{
				v: &aksnodeconfigv1.Configuration{
					CustomCloudConfig: &aksnodeconfigv1.CustomCloudConfig{
						CustomCloudEnvName: helpers.AksCustomCloudName,
					},
				},
			},
			want: helpers.AzureStackCloud,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getTargetCloud(tt.args.v); got != tt.want {
				t.Errorf("getTargetCloud() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getLinuxAdminUsername(t *testing.T) {
	type args struct {
		username string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Empty username",
			args: args{
				username: "",
			},
			want: helpers.DefaultLinuxUser,
		},
		{
			name: "Non-empty username",
			args: args{
				username: "testuser",
			},
			want: "testuser",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLinuxAdminUsername(tt.args.username); got != tt.want {
				t.Errorf("getLinuxAdminUsername() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getIsSgxEnabledSKU(t *testing.T) {
	type args struct {
		vmSize string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Empty vmSize",
			args: args{
				vmSize: "",
			},
			want: false,
		},
		{
			name: "Standard_D2s_v3",
			args: args{
				vmSize: "Standard_D2s_v3",
			},
			want: false,
		},
		{
			name: helpers.VMSizeStandardDc2s,
			args: args{
				vmSize: helpers.VMSizeStandardDc2s,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getIsSgxEnabledSKU(tt.args.vmSize); got != tt.want {
				t.Errorf("getIsSgxEnabledSKU() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getKubeletConfigFileContent(t *testing.T) {
	type args struct {
		kubeletConfig *aksnodeconfigv1.KubeletConfig
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Default KubeletConfig",
			args: args{
				kubeletConfig: &aksnodeconfigv1.KubeletConfig{
					KubeletConfigFileConfig: &aksnodeconfigv1.KubeletConfigFileConfig{
						Kind:              "KubeletConfiguration",
						ApiVersion:        "kubelet.config.k8s.io/v1beta1",
						StaticPodPath:     "/etc/kubernetes/manifests",
						Address:           "0.0.0.0",
						ReadOnlyPort:      10255,
						TlsCertFile:       "/etc/kubernetes/certs/kubeletserver.crt",
						TlsPrivateKeyFile: "/etc/kubernetes/certs/kubeletserver.key",
						TlsCipherSuites: []string{
							"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
							"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
							"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
							"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
							"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
							"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
							"TLS_RSA_WITH_AES_256_GCM_SHA384",
							"TLS_RSA_WITH_AES_128_GCM_SHA256",
						},
						RotateCertificates: true,
						ServerTlsBootstrap: true,
						Authentication: &aksnodeconfigv1.KubeletAuthentication{
							X509: &aksnodeconfigv1.KubeletX509Authentication{
								ClientCaFile: "/etc/kubernetes/certs/ca.crt",
							},
							Webhook: &aksnodeconfigv1.KubeletWebhookAuthentication{
								Enabled: true,
							},
						},
						Authorization: &aksnodeconfigv1.KubeletAuthorization{
							Mode: "Webhook",
						},
						EventRecordQps: to.Ptr(int32(0)),
						ClusterDomain:  "cluster.local",
						ClusterDns: []string{
							"10.0.0.10",
						},
						StreamingConnectionIdleTimeout: "4h0m0s",
						NodeStatusUpdateFrequency:      "10s",
						ImageGcHighThresholdPercent:    to.Ptr(int32(90)),
						ImageGcLowThresholdPercent:     to.Ptr(int32(70)),
						CgroupsPerQos:                  to.Ptr(true),
						CpuManagerPolicy:               "static",
						TopologyManagerPolicy:          "best-effort",
						MaxPods:                        to.Ptr(int32(110)),
						PodPidsLimit:                   to.Ptr(int32(12345)),
						ResolvConf:                     "/etc/resolv.conf",
						CpuCfsQuota:                    to.Ptr(false),
						CpuCfsQuotaPeriod:              "200ms",
						EvictionHard: map[string]string{
							"memory.available":  "750Mi",
							"nodefs.available":  "10%",
							"nodefs.inodesFree": "5%",
						},
						ProtectKernelDefaults: true,
						FeatureGates: map[string]bool{
							"CustomCPUCFSQuotaPeriod":        true,
							"RotateKubeletServerCertificate": true,
							"DynamicKubeletConfig":           false,
						},
						FailSwapOn:           to.Ptr(false),
						ContainerLogMaxSize:  "1000M",
						ContainerLogMaxFiles: to.Ptr(int32(99)),
						SystemReserved: map[string]string{
							"cpu":    "2",
							"memory": "1Gi",
						},
						KubeReserved: map[string]string{
							"cpu":    "100m",
							"memory": "1638Mi",
						},
						EnforceNodeAllocatable: []string{
							"pods",
						},
						AllowedUnsafeSysctls: []string{
							"kernel.msg*",
							"net.ipv4.route.min_pmtu",
						},
						SeccompDefault: true,
					},
				},
			},
			want: expectedKubeletJSON,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getKubeletConfigFileContent(tt.args.kubeletConfig); got != tt.want {
				// Normalize JSON strings to avoid any formatting differences such as space, indent, line breaking
				var gotJSON, wantJSON map[string]any
				if err := json.Unmarshal([]byte(got), &gotJSON); err != nil {
					t.Fatalf("Failed to unmarshal generated JSON: %v", err)
				}
				if err := json.Unmarshal([]byte(tt.want), &wantJSON); err != nil {
					t.Fatalf("Failed to unmarshal expected JSON: %v", err)
				}
				if diff := cmp.Diff(wantJSON, gotJSON); diff != "" {
					t.Errorf("Generated JSON does not match expected JSON (-want +got):\n%s", diff)
					t.Errorf("Generated config file: %s", got)
					t.Errorf("Expected config file: %s", tt.want)
				}
			}
		})
	}
}

func Test_getKubeletFlags(t *testing.T) {
	type args struct {
		kubeletConfig *aksnodeconfigv1.KubeletConfig
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Default KubeletFlags",
			args: args{
				kubeletConfig: &aksnodeconfigv1.KubeletConfig{
					KubeletFlags: map[string]string{
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
					},
				},
			},
			want: expectedKubeletConfigFlags,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getKubeletFlags(tt.args.kubeletConfig); got != tt.want {
				t.Errorf("getKubeletFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

const expectedlocalDNSCorefile = `# ***********************************************************************************
# WARNING: Changes to this file will be overwritten and not persisted.
# ***********************************************************************************
# whoami (used for health check of DNS)
health-check.localdns.local:53 {
    bind 169.254.10.10 169.254.10.11
    whoami
}
# VnetDNS overrides apply to DNS traffic from pods with dnsPolicy:default or kubelet (referred to as VnetDNS traffic).
.:53 {
    log
    bind 169.254.10.10
    forward . 168.63.129.16 {
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s immediate
        servfail 0
    }
    loop
    nsid localdns
    prometheus :9253
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
    template ANY ANY reddog.microsoft.com {
        rcode NXDOMAIN
    }
}
cluster.local:53 {
    errors
    bind 169.254.10.10
    forward . 10.0.0.10 {
        force_tcp
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600 {
        success 9984
        denial 9984
        servfail 0
    }
    loop
    nsid localdns
    prometheus :9253
}
testdomain456.com:53 {
    log
    bind 169.254.10.10
    forward . 10.0.0.10 {
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loop
    nsid localdns
    prometheus :9253
}
# KubeDNS overrides apply to DNS traffic from pods with dnsPolicy:ClusterFirst (referred to as KubeDNS traffic).
.:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
        policy sequential
        max_concurrent 2000
    }
    ready 169.254.10.11:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 72000s verify
        servfail 0
    }
    loop
    nsid localdns-pod
    prometheus :9253
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
    template ANY ANY reddog.microsoft.com {
        rcode NXDOMAIN
    }
}`

func Test_getLocalDNSCorefileBase64(t *testing.T) {
	type args struct {
		aksnodeconfig *aksnodeconfigv1.Configuration
	}
	tests := []struct {
		name         string
		args         args
		wantContains string
	}{
		{
			name: "aksnodeconfig is nil, should return empty string",
			args: args{
				aksnodeconfig: nil,
			},
			wantContains: "",
		},
		{
			name: "LocalDNSProfile is nil, should return empty string",
			args: args{
				aksnodeconfig: &aksnodeconfigv1.Configuration{},
			},
			wantContains: "",
		},
		{
			name: "EnableLocalDNS state is false, should return empty string",
			args: args{
				aksnodeconfig: &aksnodeconfigv1.Configuration{
					LocalDnsProfile: &aksnodeconfigv1.LocalDnsProfile{
						EnableLocalDns: false,
					},
				},
			},
			wantContains: "",
		},
		{
			name: "LocalDNSProfile enabled returns base64 string",
			args: args{
				aksnodeconfig: &aksnodeconfigv1.Configuration{
					LocalDnsProfile: &aksnodeconfigv1.LocalDnsProfile{
						EnableLocalDns:       true,
						CpuLimitInMilliCores: to.Ptr(int32(2008)),
						MemoryLimitInMb:      to.Ptr(int32(512)),
						VnetDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
							".": {
								QueryLogging:                "Log",
								Protocol:                    "PreferUDP",
								ForwardDestination:          "VnetDNS",
								ForwardPolicy:               "Sequential",
								MaxConcurrent:               to.Ptr(int32(1000)),
								CacheDurationInSeconds:      to.Ptr(int32(3600)),
								ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
								ServeStale:                  "Immediate",
							},
							"cluster.local": {
								QueryLogging:                "Error",
								Protocol:                    "ForceTCP",
								ForwardDestination:          "ClusterCoreDNS",
								ForwardPolicy:               "Sequential",
								MaxConcurrent:               to.Ptr(int32(1000)),
								CacheDurationInSeconds:      to.Ptr(int32(3600)),
								ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
								ServeStale:                  "Disable",
							},
							"testdomain456.com": {
								QueryLogging:                "Log",
								Protocol:                    "PreferUDP",
								ForwardDestination:          "ClusterCoreDNS",
								ForwardPolicy:               "Sequential",
								MaxConcurrent:               to.Ptr(int32(1000)),
								CacheDurationInSeconds:      to.Ptr(int32(3600)),
								ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
								ServeStale:                  "Verify",
							},
						},
						KubeDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
							".": {
								QueryLogging:                "Error",
								Protocol:                    "PreferUDP",
								ForwardDestination:          "ClusterCoreDNS",
								ForwardPolicy:               "Sequential",
								MaxConcurrent:               to.Ptr(int32(2000)),
								CacheDurationInSeconds:      to.Ptr(int32(3600)),
								ServeStaleDurationInSeconds: to.Ptr(int32(72000)),
								ServeStale:                  "Verify",
							},
						},
					},
				},
			},
			wantContains: expectedlocalDNSCorefile,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getLocalDnsCorefileBase64(tt.args.aksnodeconfig)

			if tt.wantContains == "" && got != "" {
				t.Errorf("expected empty string, got %q", got)
				return
			}

			normalize := func(s string) string {
				s = strings.ReplaceAll(s, "\r\n", "\n")
				s = strings.TrimSpace(s)

				// Collapse multiple blank lines into one
				re := regexp.MustCompile(`\n\s*\n+`)
				s = re.ReplaceAllString(s, "\n")

				// Remove extra indentation
				re = regexp.MustCompile(`[ \t]+`)
				s = re.ReplaceAllString(s, " ")

				return s
			}

			if tt.wantContains != "" {
				decoded, err := base64.StdEncoding.DecodeString(got)
				if err != nil {
					t.Fatalf("failed to decode base64: %v", err)
				}

				decodedNorm := normalize(string(decoded))
				wantNorm := normalize(tt.wantContains)

				if !strings.Contains(decodedNorm, wantNorm) {
					t.Errorf("expected decoded corefile to contain %q, got:\n%s", tt.wantContains, string(decoded))
				}
			}
		})
	}
}

func Test_shouldEnableLocalDns(t *testing.T) {
	type args struct {
		aksnodeconfig *aksnodeconfigv1.Configuration
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "nil config",
			args: args{aksnodeconfig: nil},
			want: "false",
		},
		{
			name: "nil LocalDnsProfile",
			args: args{aksnodeconfig: &aksnodeconfigv1.Configuration{}},
			want: "false",
		},
		{
			name: "LocalDns disabled",
			args: args{aksnodeconfig: &aksnodeconfigv1.Configuration{
				LocalDnsProfile: &aksnodeconfigv1.LocalDnsProfile{
					EnableLocalDns: false},
			}},
			want: "false",
		},
		{
			name: "LocalDns enabled",
			args: args{aksnodeconfig: &aksnodeconfigv1.Configuration{
				LocalDnsProfile: &aksnodeconfigv1.LocalDnsProfile{
					EnableLocalDns: true},
			}},
			want: "true",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldEnableLocalDns(tt.args.aksnodeconfig); got != tt.want {
				t.Errorf("shouldEnableLocalDns() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getLocalDnsCpuLimitInPercentage(t *testing.T) {
	type args struct {
		aksnodeconfig *aksnodeconfigv1.Configuration
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "nil LocalDnsProfile",
			args: args{aksnodeconfig: &aksnodeconfigv1.Configuration{}},
			want: defaultLocalDnsCpuLimitInPercentage,
		},
		{
			name: "zero CPU limit",
			args: args{aksnodeconfig: &aksnodeconfigv1.Configuration{
				LocalDnsProfile: &aksnodeconfigv1.LocalDnsProfile{
					CpuLimitInMilliCores: to.Ptr(int32(0))},
			}},
			want: defaultLocalDnsCpuLimitInPercentage,
		},
		{
			name: "1000m CPU (1 core -> 100%) with localdns enabled",
			args: args{aksnodeconfig: &aksnodeconfigv1.Configuration{
				LocalDnsProfile: &aksnodeconfigv1.LocalDnsProfile{
					EnableLocalDns:       true,
					CpuLimitInMilliCores: to.Ptr(int32(1000))},
			}},
			want: "100.0%",
		},
		{
			name: "localdns not enabled",
			args: args{aksnodeconfig: &aksnodeconfigv1.Configuration{
				LocalDnsProfile: &aksnodeconfigv1.LocalDnsProfile{
					EnableLocalDns:       false,
					CpuLimitInMilliCores: to.Ptr(int32(250))},
			}},
			want: "200.0%",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLocalDnsCpuLimitInPercentage(tt.args.aksnodeconfig); got != tt.want {
				t.Errorf("getLocalDnsCpuLimitInPercentage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getLocalDnsMemoryLimitInMb(t *testing.T) {
	type args struct {
		aksnodeconfig *aksnodeconfigv1.Configuration
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "nil LocalDnsProfile",
			args: args{aksnodeconfig: &aksnodeconfigv1.Configuration{}},
			want: defaultLocalDnsMemoryLimitInMb,
		},
		{
			name: "zero memory limit",
			args: args{aksnodeconfig: &aksnodeconfigv1.Configuration{
				LocalDnsProfile: &aksnodeconfigv1.LocalDnsProfile{
					EnableLocalDns:  true,
					MemoryLimitInMb: to.Ptr(int32(0))},
			}},
			want: defaultLocalDnsMemoryLimitInMb,
		},
		{
			name: "512 MB memory with localdns enabled",
			args: args{aksnodeconfig: &aksnodeconfigv1.Configuration{
				LocalDnsProfile: &aksnodeconfigv1.LocalDnsProfile{
					EnableLocalDns:  true,
					MemoryLimitInMb: to.Ptr(int32(512))},
			}},
			want: "512M",
		},
		{
			name: "localdns not enabled",
			args: args{aksnodeconfig: &aksnodeconfigv1.Configuration{
				LocalDnsProfile: &aksnodeconfigv1.LocalDnsProfile{
					EnableLocalDns:  false,
					MemoryLimitInMb: to.Ptr(int32(2048))},
			}},
			want: "128M",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLocalDnsMemoryLimitInMb(tt.args.aksnodeconfig); got != tt.want {
				t.Errorf("getLocalDnsMemoryLimitInMb() = %v, want %v", got, tt.want)
			}
		})
	}
}
