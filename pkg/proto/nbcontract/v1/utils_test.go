// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package nbcontractv1

import (
	"reflect"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func Test_getLoadBalancerSKU(t *testing.T) {
	type args struct {
		sku string
	}
	tests := []struct {
		name string
		args args
		want LoadBalancerConfig_LoadBalancerSku
	}{
		{
			name: "LoadBalancerSKU Standard",
			args: args{
				sku: "Standard",
			},
			want: LoadBalancerConfig_STANDARD,
		},
		{
			name: "LoadBalancerSKU Basic",
			args: args{
				sku: "Basic",
			},
			want: LoadBalancerConfig_BASIC,
		},
		{
			name: "LoadBalancerSKU Unspecified",
			args: args{
				sku: "",
			},
			want: LoadBalancerConfig_UNSPECIFIED,
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
		want NetworkPlugin
	}{
		{
			name: "NetworkPlugin azure",
			args: args{
				np: "azure",
			},
			want: NetworkPlugin_NP_AZURE,
		},
		{
			name: "NetworkPlugin kubenet",
			args: args{
				np: "kubenet",
			},
			want: NetworkPlugin_NP_KUBENET,
		},
		{
			name: "NetworkPlugin Unspecified",
			args: args{
				np: "",
			},
			want: NetworkPlugin_NP_NONE,
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
		want NetworkPolicy
	}{
		{
			name: "NetworkPolicy azure",
			args: args{
				np: "azure",
			},
			want: NetworkPolicy_NPO_AZURE,
		},
		{
			name: "NetworkPolicy calico",
			args: args{
				np: "calico",
			},
			want: NetworkPolicy_NPO_CALICO,
		},
		{
			name: "NetworkPolicy Unspecified",
			args: args{
				np: "",
			},
			want: NetworkPolicy_NPO_NONE,
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

func Test_getKubeletNodeLabels(t *testing.T) {
	type args struct {
		ap *datamodel.AgentPoolProfile
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "KubeletNodeLabels default labels",
			args: args{
				ap: &datamodel.AgentPoolProfile{
					Name: "agentPool0",
				},
			},
			want: map[string]string{
				"agentpool":                      "agentPool0",
				"kubernetes.azure.com/agentpool": "agentPool0",
			},
		},
		{
			name: "KubeletNodeLabels with CustomNodeLabels",
			args: args{
				ap: &datamodel.AgentPoolProfile{
					Name: "agentPool0",
					CustomNodeLabels: map[string]string{
						"a": "b",
					},
				},
			},
			want: map[string]string{
				"agentpool":                      "agentPool0",
				"kubernetes.azure.com/agentpool": "agentPool0",
				"a":                              "b",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetKubeletNodeLabels(tt.args.ap); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetKubeletNodeLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}
