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

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNvidiaEnabledSKU(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		name   string
		input  string
		output bool
	}{
		{"Valid SKU - NC6", "standard_nc6", true},
		{"Valid SKU - NC6 promo", "standard_nc6_promo", true},
		{"Valid SKU - NC12", "standard_nc12", true},
		{"Valid SKU - NC24", "standard_nc24", true},
		{"Valid SKU - NC24r", "standard_nc24r", true},
		{"Valid SKU - NC6s v2", "standard_nc6s_v2", true},
		{"Valid SKU - NC12s v2", "standard_nc12s_v2", true},
		{"Valid SKU - NC24s v2", "standard_nc24s_v2", true},
		{"Valid SKU - NC24rs v2", "standard_nc24rs_v2", true},
		{"Valid SKU - NC6s v3", "standard_nc6s_v3", true},
		{"Valid SKU - NC6s v3 Promo", "standard_nc6s_v3_promo", true},
		{"Valid SKU - NC12s v3", "standard_nc12s_v3", true},
		{"Valid SKU - NC24s v3", "standard_nc24s_v3", true},
		{"NValid SKU - C24rs v3", "standard_nc24rs_v3", true},
		{"Valid SKU - NV6", "standard_nv6", true},
		{"Valid SKU - NV12", "standard_nv12", true},
		{"Valid SKU - NV24", "standard_nv24", true},
		{"Valid SKU - NV24", "standard_nv24r", true},
		{"Valid SKU - ND6", "standard_nd6s", true},
		{"Valid SKU - ND12s", "standard_nd12s", true},
		{"Valid SKU - ND24s", "standard_nd24s", true},
		{"Valid SKU - ND24rs", "standard_nd24rs", true},
		{"Non-Existent SKU", "non_existent_sku", false},
		{"Invalid SKU", "standard_d2_v2", false},
		{"Valid SKU - T4 Series", "standard_nc4as_t4_v3", true},
		{"Empty SKU", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsNvidiaEnabledSKU(test.input)
			assert.Equal(test.output, result, "Failed for input: %s", test.input)
		})
	}
}

func TestUseWindowsCudaGPUDriver(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		vmSize   string
		expected bool
	}{
		{"Standard_NC6", "Standard_NC6", true},
		{"Standard_NC12", "Standard_NC12", true},
		{"Standard_NC24", "Standard_NC24", true},
		{"Standard_NV6", "Standard_NV6", false},
		{"Standard_ND6s", "Standard_ND6s", true},
		{"Standard_D2_v2", "Standard_D2_v2", false},
		{"Nonexistent", "gobbledygook", false},
		{"EmptyString", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := UseWindowsCudaGPUDriver(tc.vmSize)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestUseWindowsGridGPUDriver(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		vmSize   string
		expected bool
	}{
		{"Standard_NC6", "Standard_NC6", false},
		{"Standard_NC12", "Standard_NC12", false},
		{"Standard_NV6", "Standard_NV6", true},
		{"Standard_ND6s", "Standard_ND6s", false},
		{"Standard_D2_v2", "Standard_D2_v2", false},
		{"Nonexistent", "gobbledygook", false},
		{"EmptyString", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := UseWindowsGridGPUDriver(tc.vmSize)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsMarinerEnabledGPUSKU(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		name   string
		input  string
		output bool
	}{
		{"Invalid Mariner SKU - NC6", "standard_nc6", false},
		{"Invalid Mariner SKU - NC6 promo", "standard_nc6_promo", false},
		{"Invalid Mariner SKU - NC12", "standard_nc12", false},
		{"Invalid Mariner SKU - NC24", "standard_nc24", false},
		{"Invalid Mariner SKU - NC24r", "standard_nc24r", false},
		{"Invalid Mariner SKU - NC6s v2", "standard_nc6s_v2", false},
		{"Invalid Mariner SKU - NC12s v2", "standard_nc12s_v2", false},
		{"Invalid Mariner SKU - NC24s v2", "standard_nc24s_v2", false},
		{"Invalid Mariner SKU - NC24rs v2", "standard_nc24rs_v2", false},
		{"Valid Mariner SKU - NC6s v3", "standard_nc6s_v3", true},
		{"Valid Mariner SKU with Promo", "standard_nc6s_v3_promo", true},
		{"Valid Mariner SKU - NC12s v3", "standard_nc12s_v3", true},
		{"Valid Mariner SKU - NC24s v3", "standard_nc24s_v3", true},
		{"Valid Mariner SKU - NC24rs v3", "standard_nc24rs_v3", true},
		{"Invalid Mariner SKU - NV6", "standard_nv6", false},
		{"Invalid Mariner SKU - NV12", "standard_nv12", false},
		{"Invalid Mariner SKU - NV24", "standard_nv24", false},
		{"Invalid Mariner SKU - NV24", "standard_nv24r", false},
		{"Invalid Mariner SKU - ND6", "standard_nd6s", false},
		{"Invalid Mariner SKU - ND12s", "standard_nd12s", false},
		{"Invalid Mariner SKU - ND24s", "standard_nd24s", false},
		{"Invalid Mariner SKU - ND24rs", "standard_nd24rs", false},
		{"Non-Existent Mariner SKU", "non_existent_sku", false},
		{"Valid Mariner SKU - T4", "standard_nc4as_t4_v3", true},
		{"Invalid Mariner SKU", "standard_d2_v2", false},
		{"Valid Mariner SKU - ND Series", "standard_nd40s_v3", true},
		{"Invalid Mariner SKU - ND Series", "standard_nd96asr_v4", false},
		{"Nonexistent", "gobledygook", false},
		{"Empty Mariner SKU", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsMarinerEnabledGPUSKU(test.input)
			assert.Equal(test.output, result, "Failed for input: %s", test.input)
		})
	}
}

func TestGetAKSGPUImageSHA(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		name   string
		size   string
		output string
	}{
		{"GRID Driver - NC Series v4", "standard_nc8ads_a10_v4", aksGPUGridSHA},
		{"Cuda Driver - NV Series", "standard_nv6", aksGPUCudaSHA},
		{"CUDA Driver - NC Series", "standard_nc6s_v3", aksGPUCudaSHA},
		{"GRID Driver - NV Series v5", "standard_nv6ads_a10_v5", aksGPUGridSHA},
		{"Unknown SKU", "unknown_sku", aksGPUCudaSHA},
		{"CUDA Driver - NC Series v2", "standard_nc6s_v2", aksGPUCudaSHA},
		{"CUDA Driver - NV Series v3", "standard_nv12s_v3", aksGPUCudaSHA},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := GetAKSGPUImageSHA(test.size)
			assert.Equal(test.output, result, "Failed for size: %s", test.size)
		})
	}
}

func TestGetGPUDriverVersion(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		name   string
		size   string
		output string
	}{
		{"CUDA Driver - NC Series v1", "standard_nc6", nvidia470CudaDriverVersion},
		{"CUDA Driver - NCs Series v1", "standard_nc6s", nvidia470CudaDriverVersion},
		{"CUDA Driver - NC Series v2", "standard_nc6s_v2", nvidia550CudaDriverVersion},
		{"Unknown SKU", "unknown_sku", nvidia550CudaDriverVersion},
		{"CUDA Driver - NC Series v3", "standard_nc6s_v3", nvidia550CudaDriverVersion},
		{"GRID Driver - A10", "standard_nc8ads_a10_v4", nvidia535GridDriverVersion},
		{"GRID Driver - NV Series v5", "standard_nv6ads_a10_v5", nvidia535GridDriverVersion},
		{"GRID Driver - A10", "standard_nv36adms_a10_V5", nvidia535GridDriverVersion},
		// NV V1 SKUs were retired in September 2023, leaving this test just for safety
		{"CUDA Driver - NV Series v1", "standard_nv6", nvidia550CudaDriverVersion},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := GetGPUDriverVersion(test.size)
			assert.Equal(test.output, result, "Failed for size: %s", test.size)
		})
	}
}
