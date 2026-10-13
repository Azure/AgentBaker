// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func TestGPUDriverSelection(t *testing.T) {
	cases := []struct {
		size       string
		driverType string
	}{
		{"Standard_NV6ads_A10_v5", "grid"},
		{"Standard_NV12ads_A10_v5", "grid"},
		{"Standard_NV18ads_A10_v5", "grid"},
		{"Standard_NV36ads_A10_v5", "grid"},
		{"Standard_NV72ads_A10_v5", "grid"},
		{"Standard_NV36adms_A10_v5", "grid"},
		{"Standard_NC8ads_A10_v4", "grid"},
		{"Standard_NC16ads_A10_v4", "grid"},
		{"Standard_NC32ads_A10_v4", "grid"},
		{"Standard_NC24lds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC36ds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC36lds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC72ds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC72lds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC144ds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC144lds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC288ds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC288lds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC128ds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC128lds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC256ds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC256lds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC320ds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC320lds_xl_RTXPRO6000BSE_v6", "grid-v20"},
		{"Standard_NC4as_T4_v3", "cuda-lts"},
		{"Standard_NC6s_v3", "cuda-lts"},
		{"Standard_NC24ads_A100_v4", "cuda-lts"},
		{"Standard_NC40ads_H100_v5", "cuda-lts"},
		{"Standard_ND128isr_NDR_GB200_v6", "cuda-lts"},
		{"Standard_NC6", "cuda"},
	}
	for _, tc := range cases {
		t.Run(tc.size, func(t *testing.T) {
			var expectedVersion, expectedSuffix string
			switch tc.driverType {
			case "grid":
				expectedVersion = datamodel.NvidiaGridDriverVersion
				expectedSuffix = datamodel.AKSGPUGridVersionSuffix
			case "grid-v20":
				expectedVersion = datamodel.NvidiaGridV20DriverVersion
				expectedSuffix = datamodel.AKSGPUGridV20VersionSuffix
			case "cuda-lts":
				expectedVersion = datamodel.NvidiaCudaLTSDriverVersion
				expectedSuffix = datamodel.AKSGPUCudaLTSVersionSuffix
			case "cuda":
				expectedVersion = datamodel.Nvidia470CudaDriverVersion
				expectedSuffix = datamodel.AKSGPUCudaLTSVersionSuffix
			default:
				t.Fatalf("unexpected test driver type %q", tc.driverType)
			}
			for _, size := range []string{tc.size, strings.ToLower(tc.size), strings.ToUpper(tc.size)} {
				if got := GetGPUDriverType(size); got != tc.driverType {
					t.Errorf("GetGPUDriverType(%q) = %q, want %q", size, got, tc.driverType)
				}
				if got := GetGPUDriverVersion(size); got != expectedVersion {
					t.Errorf("GetGPUDriverVersion(%q) = %q, want %q", size, got, expectedVersion)
				}
				if got := GetAKSGPUImageSHA(size); got != expectedSuffix {
					t.Errorf("GetAKSGPUImageSHA(%q) = %q, want %q", size, got, expectedSuffix)
				}
			}
		})
	}
}
