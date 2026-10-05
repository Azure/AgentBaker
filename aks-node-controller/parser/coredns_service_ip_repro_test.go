package parser

import (
	"encoding/base64"
	"strings"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

// Repro for the scriptless/ANC CoreDnsServiceIp bug.
//
// The ANC corefile template DOES honor a custom CoreDnsServiceIp when the proto
// field is populated (proven below), and silently falls back to 10.0.0.10 when
// it is unset. aks-rp's scriptless.go / scriptless_direct.go never populate the
// field, so on the scriptless path every cluster gets 10.0.0.10 regardless of a
// custom service CIDR. These tests demonstrate both halves.

func reproLocalDNSConfig(coreDNSServiceIP string) *aksnodeconfigv1.Configuration {
	cfg := &aksnodeconfigv1.Configuration{
		ClusterConfig: &aksnodeconfigv1.ClusterConfig{
			ClusterNetworkConfig: &aksnodeconfigv1.ClusterNetworkConfig{},
		},
		LocalDnsProfile: &aksnodeconfigv1.LocalDnsProfile{
			EnableLocalDns: true,
			KubeDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
				".": {
					ForwardDestination:     "ClusterCoreDNS",
					ForwardPolicy:          "Sequential",
					MaxConcurrent:          to.Ptr(int32(1000)),
					CacheDurationInSeconds: to.Ptr(int32(3600)),
				},
			},
			VnetDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
				"cluster.local": {
					ForwardDestination:     "ClusterCoreDNS",
					ForwardPolicy:          "Sequential",
					MaxConcurrent:          to.Ptr(int32(1000)),
					CacheDurationInSeconds: to.Ptr(int32(3600)),
				},
			},
		},
	}
	if coreDNSServiceIP != "" {
		cfg.ClusterConfig.ClusterNetworkConfig.CoreDnsServiceIp = coreDNSServiceIP
	}
	return cfg
}

// Proves AgentBaker's ANC side is correct: when the proto field IS populated
// (i.e. once aks-rp is fixed), the custom ClusterIP flows into the corefile.
func TestReproCustomCoreDnsServiceIpHonoredWhenSet(t *testing.T) {
	cfg := reproLocalDNSConfig("172.16.0.10")

	if got := getCoreDnsServiceIp(cfg); got != "172.16.0.10" {
		t.Fatalf("getCoreDnsServiceIp: expected custom 172.16.0.10, got %q", got)
	}

	raw, err := base64.StdEncoding.DecodeString(getLocalDnsCorefileBase64WithHostsPlugin(cfg, false))
	if err != nil {
		t.Fatalf("decode corefile: %v", err)
	}
	corefile := string(raw)
	if !strings.Contains(corefile, "forward . 172.16.0.10") {
		t.Fatalf("corefile did not honor custom ClusterIP; got:\n%s", corefile)
	}
	if strings.Contains(corefile, "forward . 10.0.0.10") {
		t.Fatalf("corefile still contains default 10.0.0.10 despite custom config")
	}
	t.Log("CONFIRMED: ANC honors custom CoreDnsServiceIp when the proto field is set")
}

// Reproduces the bug's effect: with the field UNSET (exactly what aks-rp
// scriptless does today), the ClusterIP silently defaults to 10.0.0.10.
func TestReproDefaultUsedWhenCoreDnsServiceIpUnset(t *testing.T) {
	cfg := reproLocalDNSConfig("") // mirrors aks-rp scriptless.go / scriptless_direct.go

	if got := getCoreDnsServiceIp(cfg); got != "10.0.0.10" {
		t.Fatalf("expected default 10.0.0.10 when unset, got %q", got)
	}

	raw, err := base64.StdEncoding.DecodeString(getLocalDnsCorefileBase64WithHostsPlugin(cfg, false))
	if err != nil {
		t.Fatalf("decode corefile: %v", err)
	}
	if !strings.Contains(string(raw), "forward . 10.0.0.10") {
		t.Fatalf("expected default 10.0.0.10 forward target when unset")
	}
	t.Log("CONFIRMED: field unset (aks-rp scriptless today) => corefile silently uses 10.0.0.10")
}
