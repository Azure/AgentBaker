package e2e

import "testing"

func Test_isVMSizeGen2Only(t *testing.T) {
	tests := []struct {
		vmSize string
		want   bool
	}{
		{"Standard_D2ds_v5", false},
		{"Standard_D4ds_v5", false},
		{"standard_d4ds_v5", false},
		{"Standard_D2ds_v6", true},
		{"standard_d4ds_v7", true},
		{"Standard_NC6s_v3", false},
		{"Standard_D2s_v3", false},
		{"Standard_D2pds_V5", false},
		{"Standard_D2pds_V6", true},
		{"Standard_D2pds_V7", true},
		{"Standard_D2s", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.vmSize, func(t *testing.T) {
			if got := isVMSizeGen2Only(tt.vmSize); got != tt.want {
				t.Errorf("isVMSizeGen2Only(%q) = %v, want %v", tt.vmSize, got, tt.want)
			}
		})
	}
}

func Test_isVMSizeNVMeOnly(t *testing.T) {
	tests := []struct {
		vmSize string
		want   bool
	}{
		{"Standard_D2ds_v5", false},
		{"Standard_D4ds_v6", false},
		{"standard_d4ds_v7", true},
		{"Standard_D4ds_v7", true},
		{"Standard_D8ds_v8", true},
		{"Standard_NC6s_v3", false},
		{"Standard_D2pds_V7", true},
		{"Standard_D2s", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.vmSize, func(t *testing.T) {
			if got := isVMSizeNVMeOnly(tt.vmSize); got != tt.want {
				t.Errorf("isVMSizeNVMeOnly(%q) = %v, want %v", tt.vmSize, got, tt.want)
			}
		})
	}
}
