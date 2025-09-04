// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package helpers

import (
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
)

func Test_getLoadBalancerSKU(t *testing.T) {
	type args struct {
		sku string
	}
	tests := []struct {
		name string
		args args
		want aksnodeconfigv1.LoadBalancerSku
	}{
		{
			name: "LoadBalancerSKU Standard",
			args: args{
				sku: "Standard",
			},
			want: aksnodeconfigv1.LoadBalancerSku_LOAD_BALANCER_SKU_STANDARD,
		},
		{
			name: "LoadBalancerSKU Basic",
			args: args{
				sku: "Basic",
			},
			want: aksnodeconfigv1.LoadBalancerSku_LOAD_BALANCER_SKU_BASIC,
		},
		{
			name: "LoadBalancerSKU Unspecified",
			args: args{
				sku: "",
			},
			want: aksnodeconfigv1.LoadBalancerSku_LOAD_BALANCER_SKU_UNSPECIFIED,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetLoadBalancerSKU(tt.args.sku); got != tt.want {
				t.Errorf("GetLoadBalancerSKU() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getNetworkPluginType(t *testing.T) {
	type args struct {
		np string
	}
	tests := []struct {
		name string
		args args
		want aksnodeconfigv1.NetworkPlugin
	}{
		{
			name: "NetworkPlugin azure",
			args: args{
				np: "azure",
			},
			want: aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_AZURE,
		},
		{
			name: "NetworkPlugin kubenet",
			args: args{
				np: "kubenet",
			},
			want: aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_KUBENET,
		},
		{
			name: "NetworkPlugin Unspecified",
			args: args{
				np: "",
			},
			want: aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_NONE,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetNetworkPluginType(tt.args.np); got != tt.want {
				t.Errorf("GetNetworkPluginType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getNetworkPolicyType(t *testing.T) {
	type args struct {
		np string
	}
	tests := []struct {
		name string
		args args
		want aksnodeconfigv1.NetworkPolicy
	}{
		{
			name: "NetworkPolicy azure",
			args: args{
				np: "azure",
			},
			want: aksnodeconfigv1.NetworkPolicy_NETWORK_POLICY_AZURE,
		},
		{
			name: "NetworkPolicy calico",
			args: args{
				np: "calico",
			},
			want: aksnodeconfigv1.NetworkPolicy_NETWORK_POLICY_CALICO,
		},
		{
			name: "NetworkPolicy Unspecified",
			args: args{
				np: "",
			},
			want: aksnodeconfigv1.NetworkPolicy_NETWORK_POLICY_NONE,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetNetworkPolicyType(tt.args.np); got != tt.want {
				t.Errorf("GetNetworkPolicyType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsKubeletServingCertificateRotationEnabled(t *testing.T) {
	tests := []struct {
		name         string
		kubeletFlags map[string]string
		expected     bool
	}{
		{
			name:         "should return false with nil kubelet flags",
			kubeletFlags: nil,
			expected:     false,
		},
		{
			name: "should return false if kubelet flags does not set --rotate-server-certificates to true",
			kubeletFlags: map[string]string{
				"--rotate-certificates": "true",
			},
			expected: false,
		},
		{
			name: "should return false if kubelet flags explicitly sets --rotate-server-certificates to false",
			kubeletFlags: map[string]string{
				"--rotate-certificates":        "true",
				"--rotate-server-certificates": "false",
			},
			expected: false,
		},
		{
			name: "should return true if kubelet flags set --rotate-server-certificates to true",
			kubeletFlags: map[string]string{
				"--rotate-certificates":        "true",
				"--rotate-server-certificates": "true",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if actual := isKubeletServingCertificateRotationEnabled(tt.kubeletFlags); actual != tt.expected {
				t.Errorf("expected isKubeletServingCertificateRotationEnabled to be %t, but was %t", tt.expected, actual)
			}
		})
	}
}
