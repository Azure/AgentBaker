package main

import (
	"strings"
	"testing"

	introspectionapi "github.com/containerd/containerd/api/services/introspection/v1"
	"github.com/containerd/containerd/v2/plugins"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
)

func TestValidateDmverityReferrerCapability(t *testing.T) {
	tests := []struct {
		name      string
		available []*introspectionapi.Plugin
		wantError string
	}{
		{
			name:      "transfer plugin advertises the capability",
			available: []*introspectionapi.Plugin{activeDmverityTransfer()},
		},
		{
			name: "capability is missing",
			available: []*introspectionapi.Plugin{
				{
					Type:         string(plugins.TransferPlugin),
					ID:           localTransferPluginID,
					Capabilities: []string{"other-capability"},
				},
			},
			wantError: `transfer plugin "local" does not advertise capability "dmverity-referrers"`,
		},
		{
			name: "plugin failed initialization",
			available: []*introspectionapi.Plugin{
				{
					Type:         string(plugins.TransferPlugin),
					ID:           localTransferPluginID,
					Capabilities: []string{dmverityReferrerCapability},
					InitErr:      &statuspb.Status{Message: "initialization failed"},
				},
			},
			wantError: `transfer plugin "local" failed initialization: initialization failed`,
		},
		{
			name: "plugin is unavailable",
			available: []*introspectionapi.Plugin{
				{
					Type:         string(plugins.TransferPlugin),
					ID:           "other",
					Capabilities: []string{dmverityReferrerCapability},
				},
			},
			wantError: `transfer plugin "local" is unavailable`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateDmverityReferrerCapability(test.available)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("expected error containing %q, got %v", test.wantError, err)
			}
		})
	}
}

func activeDmverityTransfer() *introspectionapi.Plugin {
	return &introspectionapi.Plugin{
		Type:         string(plugins.TransferPlugin),
		ID:           localTransferPluginID,
		Capabilities: []string{dmverityReferrerCapability},
	}
}
