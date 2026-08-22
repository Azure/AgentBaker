package main

import (
	"testing"

	introspectionapi "github.com/containerd/containerd/api/services/introspection/v1"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
)

func TestValidateDmverityReferrerCapability(t *testing.T) {
	tests := []struct {
		name    string
		plugins []*introspectionapi.Plugin
		wantErr bool
	}{
		{
			name: "active capability",
			plugins: []*introspectionapi.Plugin{
				{
					ID:           erofsDifferPluginID,
					Capabilities: []string{dmverityReferrerCapability},
				},
			},
		},
		{
			name: "capability missing",
			plugins: []*introspectionapi.Plugin{
				{
					ID:           erofsDifferPluginID,
					Capabilities: []string{"other-capability"},
				},
			},
			wantErr: true,
		},
		{
			name: "capability belongs to different differ",
			plugins: []*introspectionapi.Plugin{
				{
					ID:           "other",
					Capabilities: []string{dmverityReferrerCapability},
				},
			},
			wantErr: true,
		},
		{
			name: "capable plugin failed initialization",
			plugins: []*introspectionapi.Plugin{
				{
					ID:           erofsDifferPluginID,
					Capabilities: []string{dmverityReferrerCapability},
					InitErr:      &statuspb.Status{Message: "initialization failed"},
				},
			},
			wantErr: true,
		},
		{
			name:    "no diff plugins",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDmverityReferrerCapability(tt.plugins)
			if tt.wantErr && err == nil {
				t.Fatal("expected an error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
