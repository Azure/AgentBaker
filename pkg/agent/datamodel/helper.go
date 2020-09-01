// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

var (
	/* If a new GPU sku becomes available, add a key to this map, but only if you have a confirmation
	   that we have an agreement with NVIDIA for this specific gpu.
	*/
	NvidiaEnabledSKUs = map[string]bool{
		// K80
		"Standard_NC6":   true,
		"Standard_NC12":  true,
		"Standard_NC24":  true,
		"Standard_NC24r": true,
		// M60
		"Standard_NV6":      true,
		"Standard_NV12":     true,
		"Standard_NV12s_v3": true,
		"Standard_NV24":     true,
		"Standard_NV24s_v3": true,
		"Standard_NV24r":    true,
		"Standard_NV48s_v3": true,
		// P40
		"Standard_ND6s":   true,
		"Standard_ND12s":  true,
		"Standard_ND24s":  true,
		"Standard_ND24rs": true,
		// P100
		"Standard_NC6s_v2":   true,
		"Standard_NC12s_v2":  true,
		"Standard_NC24s_v2":  true,
		"Standard_NC24rs_v2": true,
		// V100
		"Standard_NC6s_v3":   true,
		"Standard_NC12s_v3":  true,
		"Standard_NC24s_v3":  true,
		"Standard_NC24rs_v3": true,
		"Standard_ND40s_v3":  true,
		"Standard_ND40rs_v2": true,
	}
)

// ValidateDNSPrefix is a helper function to check that a DNS Prefix is valid
func ValidateDNSPrefix(dnsName string) error {
	dnsNameRegex := `^([A-Za-z][A-Za-z0-9-]{1,43}[A-Za-z0-9])$`
	re, err := regexp.Compile(dnsNameRegex)
	if err != nil {
		return err
	}
	if !re.MatchString(dnsName) {
		return errors.Errorf("DNSPrefix '%s' is invalid. The DNSPrefix must contain between 3 and 45 characters and can contain only letters, numbers, and hyphens.  It must start with a letter and must end with a letter or a number. (length was %d)", dnsName, len(dnsName))
	}
	return nil
}

// IsNvidiaEnabledSKU determines if an VM SKU has nvidia driver support
func IsNvidiaEnabledSKU(vmSize string) bool {
	// Trim the optional _Promo suffix.
	vmSize = strings.TrimSuffix(vmSize, "_Promo")
	if _, ok := NvidiaEnabledSKUs[vmSize]; ok {
		return NvidiaEnabledSKUs[vmSize]
	}

	return false
}

// GetNSeriesVMCasesForTesting returns a struct w/ VM SKUs and whether or not we expect them to be nvidia-enabled
func GetNSeriesVMCasesForTesting() []struct {
	VMSKU    string
	Expected bool
} {
	cases := []struct {
		VMSKU    string
		Expected bool
	}{
		{
			"Standard_NC6",
			true,
		},
		{
			"Standard_NC6_Promo",
			true,
		},
		{
			"Standard_NC12",
			true,
		},
		{
			"Standard_NC24",
			true,
		},
		{
			"Standard_NC24r",
			true,
		},
		{
			"Standard_NV6",
			true,
		},
		{
			"Standard_NV12",
			true,
		},
		{
			"Standard_NV24",
			true,
		},
		{
			"Standard_NV24r",
			true,
		},
		{
			"Standard_ND6s",
			true,
		},
		{
			"Standard_ND12s",
			true,
		},
		{
			"Standard_ND24s",
			true,
		},
		{
			"Standard_ND24rs",
			true,
		},
		{
			"Standard_NC6s_v2",
			true,
		},
		{
			"Standard_NC12s_v2",
			true,
		},
		{
			"Standard_NC24s_v2",
			true,
		},
		{
			"Standard_NC24rs_v2",
			true,
		},
		{
			"Standard_NC24rs_v2",
			true,
		},
		{
			"Standard_NC6s_v3",
			true,
		},
		{
			"Standard_NC12s_v3",
			true,
		},
		{
			"Standard_NC24s_v3",
			true,
		},
		{
			"Standard_NC24rs_v3",
			true,
		},
		{
			"Standard_D2_v2",
			false,
		},
		{
			"gobledygook",
			false,
		},
		{
			"",
			false,
		},
	}

	return cases
}

// GetDCSeriesVMCasesForTesting returns a struct w/ VM SKUs and whether or not we expect them to be SGX-enabled
func GetDCSeriesVMCasesForTesting() []struct {
	VMSKU    string
	Expected bool
} {
	cases := []struct {
		VMSKU    string
		Expected bool
	}{
		{
			"Standard_DC2s",
			true,
		},
		{
			"Standard_DC4s",
			true,
		},
		{
			"Standard_NC12",
			false,
		},
		{
			"gobledygook",
			false,
		},
		{
			"",
			false,
		},
	}

	return cases
}

// IsSgxEnabledSKU determines if an VM SKU has SGX driver support
func IsSgxEnabledSKU(vmSize string) bool {
	switch vmSize {
	case "Standard_DC2s", "Standard_DC4s":
		return true
	}
	return false
}

// GetMasterKubernetesLabels returns a k8s API-compliant labels string.
// The `kubernetes.io/role` and `node-role.kubernetes.io` labels are disallowed
// by the kubelet `--node-labels` argument in Kubernetes 1.16 and later.
func GetMasterKubernetesLabels(rg string, deprecated bool) string {
	var buf bytes.Buffer
	buf.WriteString("kubernetes.azure.com/role=master")
	buf.WriteString(",node.kubernetes.io/exclude-from-external-load-balancers=true")
	buf.WriteString(",node.kubernetes.io/exclude-disruption=true")
	if deprecated {
		buf.WriteString(",kubernetes.io/role=master")
		buf.WriteString(",node-role.kubernetes.io/master=")
	}
	buf.WriteString(fmt.Sprintf(",kubernetes.azure.com/cluster=%s", rg))
	return buf.String()
}

// GetStorageAccountType returns the support managed disk storage tier for a give VM size
func GetStorageAccountType(sizeName string) (string, error) {
	spl := strings.Split(sizeName, "_")
	if len(spl) < 2 {
		return "", errors.Errorf("Invalid sizeName: %s", sizeName)
	}
	capability := spl[1]
	if strings.Contains(strings.ToLower(capability), "s") {
		return "Premium_LRS", nil
	}
	return "Standard_LRS", nil
}

// GetOrderedEscapedKeyValsString returns an ordered string of escaped, quoted key=val
func GetOrderedEscapedKeyValsString(config map[string]string) string {
	keys := []string{}
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, key := range keys {
		buf.WriteString(fmt.Sprintf("\"%s=%s\", ", key, config[key]))
	}
	return strings.TrimSuffix(buf.String(), ", ")
}

// SliceIntIsNonEmpty is a simple convenience to determine if a []int is non-empty
func SliceIntIsNonEmpty(s []int) bool {
	return len(s) > 0
}

// WrapAsARMVariable formats a string for inserting an ARM variable into an ARM expression
func WrapAsARMVariable(s string) string {
	return fmt.Sprintf("',variables('%s'),'", s)
}

// WrapAsParameter formats a string for inserting an ARM parameter into an ARM expression
func WrapAsParameter(s string) string {
	return fmt.Sprintf("',parameters('%s'),'", s)
}

// WrapAsVerbatim formats a string for inserting a literal string into an ARM expression
func WrapAsVerbatim(s string) string {
	return fmt.Sprintf("',%s,'", s)
}

// IndentString pads each line of an original string with N spaces and returns the new value.
func IndentString(original string, spaces int) string {
	out := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(strings.NewReader(original))
	for scanner.Scan() {
		for i := 0; i < spaces; i++ {
			out.WriteString(" ")
		}
		out.WriteString(scanner.Text())
		out.WriteString("\n")
	}
	return out.String()
}

func GetDockerConfigTestCases() map[string]string {
	return map[string]string{
		"default": defaultDockerConfigString,
		"gpu":     dockerNvidiaConfigString,
		"reroot":  dockerRerootConfigString,
		"all":     dockerAllConfigString,
	}
}

func GetContainerdConfigTestCases() map[string]string {
	return map[string]string{
		"default": containerdImageConfigString,
		"kubenet": containerdImageKubenetConfigString,
		"reroot":  containerdImageRerootConfigString,
		"all":     containerdAllConfigString,
	}
}

var defaultContainerdConfigString = `oom_score = 0
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".cni]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var containerdRerootConfigString = `oom_score = 0
root = "/mnt/containerd"
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".cni]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var containerdKubenetConfigString = `oom_score = 0
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".cni]
      conf_template = "/etc/containerd/kubenet_template.conf"
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var containerdImageConfigString = `oom_score = 0
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "foo/k8s/core/pause:1.2.0"
    [plugins."io.containerd.grpc.v1.cri".cni]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var containerdImageRerootConfigString = `oom_score = 0
root = "/mnt/containerd"
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "foo/k8s/core/pause:1.2.0"
    [plugins."io.containerd.grpc.v1.cri".cni]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var containerdImageKubenetConfigString = `oom_score = 0
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "foo/k8s/core/pause:1.2.0"
    [plugins."io.containerd.grpc.v1.cri".cni]
      conf_template = "/etc/containerd/kubenet_template.conf"
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var containerdAllConfigString = `oom_score = 0
root = "/mnt/containerd"
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "foo/k8s/core/pause:1.2.0"
    [plugins."io.containerd.grpc.v1.cri".cni]
      conf_template = "/etc/containerd/kubenet_template.conf"
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var defaultDockerConfigString = `{
    "live-restore": true,
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "50m",
        "max-file": "5"
    }
}`

var dockerRerootConfigString = `{
    "data-root": "/mnt/docker",
    "live-restore": true,
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "50m",
        "max-file": "5"
    }
}`

var dockerNvidiaConfigString = `{
    "live-restore": true,
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "50m",
        "max-file": "5"
    },
    "default-runtime": "nvidia",
    "runtimes": {
        "nvidia": {
            "path": "/usr/bin/nvidia-container-runtime",
            "runtimeArgs": []
        }
    }
}`

var dockerAllConfigString = `{
    "data-root": "/mnt/docker",
    "live-restore": true,
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "50m",
        "max-file": "5"
    },
    "default-runtime": "nvidia",
    "runtimes": {
        "nvidia": {
            "path": "/usr/bin/nvidia-container-runtime",
            "runtimeArgs": []
        }
    }
}`
