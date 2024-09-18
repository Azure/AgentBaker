package service

import (
	"fmt"
	"testing"

	"github.com/tj/assert"
)

func TestSetupConfig(t *testing.T) {

}
func TestCreateDataMaps(t *testing.T) {
	cases := []struct {
		name     string
		maps     *DataMaps
		expected *DataMaps
	}{
		{
			name: "should correctly return a struct of type DataMaps",
			maps: CreateDataMaps(),
			expected: &DataMaps{LocalPerformanceDataMap: make(map[string]map[string]float64),
				QueriedPerformanceDataMap: make(map[string]map[string][]float64),
				RegressionMap:             make(map[string]map[string]float64),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := CreateDataMaps()
			assert.Equal(t, c.expected.LocalPerformanceDataMap, actual.LocalPerformanceDataMap)
			assert.Equal(t, c.expected.QueriedPerformanceDataMap, actual.QueriedPerformanceDataMap)
			assert.Equal(t, c.expected.RegressionMap, actual.RegressionMap)
		})
	}
}

func TestDecodeLocalPerformanceData(t *testing.T) {
	cases := []struct {
		name       string
		mapsStruct *DataMaps
		expected   map[string]map[string]float64
	}{
		{
			name: "should correctly unmarshal local JSON file into appropriate field in DataMaps struct",
			mapsStruct: &DataMaps{
				LocalPerformanceDataMap: map[string]map[string]float64{},
			},
			expected: map[string]map[string]float64{
				"pre_install_dependencies": {
					"copy_packer_files":                                                 5,
					"enable_modified_log_rotate_service":                                5,
					"handle_azureLinux_and_cgroupV2":                                    5,
					"handle_mariner_and_fips_configurations":                            5,
					"make_directory_and_update_certs":                                   5,
					"pre_install_dependencies_overall":                                  5,
					"source_packer_files_declare_variables_and_set_mariner_permissions": 5,
					"start_system_logs_and_aks_log_collector":                           5,
					"sync_container_logs":                                               5,
				},
				"install_dependencies": {
					"artifact_streaming_and_download_teleportd":                                        5,
					"check_container_runtime_and_network_configurations":                               5,
					"configure_networking_and_interface":                                               5,
					"configure_telemetry_create_logging_directory":                                     5,
					"create_containerd_service_directory_download_shims_configure_runtime_and_network": 5,
					"declare_variables_and_source_packer_files":                                        5,
					"download_azure_acr_credential_provider":                                           5,
					"download_azure_cni":                                                               5,
					"download_cni_plugins":                                                             5,
					"download_containerd":                                                              5,
					"download_cri_tools":                                                               5,
					"download_gpu_device_plugin":                                                       5,
					"download_kubernetes_binaries":                                                     5,
					"download_oras":                                                                    5,
					"download_runc":                                                                    5,
					"finish_installing_bcc_tools":                                                      5,
					"install_dependencies":                                                             5,
					"install_dependencies_overall":                                                     5,
					"pull_and_retag_container_images":                                                  5,
					"pull_nvidia_driver_image_and_run_installBcc_in_subshell":                          5,
					"purge_and_reinstall_ubuntu":                                                       5,
				},
				"post_install_dependencies": {
					"determine_disk_usage":                  5,
					"install_asc_baseline":                  5,
					"list_installed_packages":               5,
					"log_and_detach_ua":                     5,
					"post_install_dependencies_overall":     5,
					"resolve_conf":                          5,
					"set_variables_and_source_packer_files": 5,
					"write_logs":                            5,
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.mapsStruct.DecodeLocalPerformanceData("/home/zbailey/go/src/go.goms.io/aks/agentBaker/vhdbuilder/packer/buildperformance/testdata/AzureLinuxV6-build-performance-data.json")
			assert.NoError(t, err)
			assert.Equal(t, c.expected, c.mapsStruct.LocalPerformanceDataMap)
		})
	}

}

func TestParseKustoData(t *testing.T) {
	cases := []struct {
		name       string
		queriedSKU *SKU
		mapsStruct *DataMaps
		expected   map[string]map[string][]float64
	}{
		{
			name: "should correctly unmarshal queried performance data into appropriate field in datamaps struct",
			queriedSKU: &SKU{
				Name:               "AzureLinuxV6",
				SKUPerformanceData: `{"post_install_dependencies": {"resolve_conf": [15.0, 10.0], "post_install_dependencies_overall": [30.0, 20], "set_variables_and_source_packer_files": [30.0, 20], "log_and_detach_ua": [30.0, 20], "determine_disk_usage": [30.0, 20], "install_asc_baseline": [30.0, 20], "write_logs": [30.0, 20], "list_installed_packages": [30.0, 20]}, "pre_install_dependencies": {"copy_packer_files": [30.0, 20], "sync_container_logs": [30.0, 20], "handle_mariner_and_fips_configurations": [30.0, 20], "handle_azureLinux_and_cgroupV2": [30.0, 20], "enable_modified_log_rotate_service": [15.0, 10.0], "make_directory_and_update_certs": [30.0, NaN], "start_system_logs_and_aks_log_collector": [30.0, 20], "pre_install_dependencies_overall": [30.0, 20], "source_packer_files_declare_variables_and_set_mariner_permissions": [30.0, 20]}, "install_dependencies": {"download_runc": [30.0, 20], "download_azure_acr_credential_provider": [30.0, 20], "create_containerd_service_directory_download_shims_configure_runtime_and_network": [30.0, 20], "configure_telemetry_create_logging_directory": [30.0, 20], "pull_nvidia_driver_image_and_run_installBcc_in_subshell": [30.0, 20], "download_azure_cni": [15.0, 10.0], "download_gpu_device_plugin": [30.0, 20], "download_kubernetes_binaries": [30.0, 20], "declare_variables_and_source_packer_files": [30.0, 20], "download_containerd": [30.0, 20], "artifact_streaming_and_download_teleportd": [30.0, 20], "configure_networking_and_interface": [15.0, 10.0], "finish_installing_bcc_tools": [30.0, 20], "download_cri_tools": [30.0, 20], "purge_and_reinstall_ubuntu": [30.0, 20], "pull_and_retag_container_images": [30.0, 20], "install_dependencies_overall": [30.0, 20], "check_container_runtime_and_network_configurations": [30.0, 20], "download_oras": [30.0, 20], "install_dependencies": [30.0, 20], "download_cni_plugins": [30.0, 20]}}`,
			},
			mapsStruct: &DataMaps{
				QueriedPerformanceDataMap: map[string]map[string][]float64{},
			},
			expected: map[string]map[string][]float64{
				"pre_install_dependencies": {
					"copy_packer_files":                                                 []float64{30.0, 20},
					"sync_container_logs":                                               []float64{30.0, 20},
					"handle_mariner_and_fips_configurations":                            []float64{30.0, 20},
					"handle_azureLinux_and_cgroupV2":                                    []float64{30.0, 20},
					"enable_modified_log_rotate_service":                                []float64{15.0, 10.0},
					"make_directory_and_update_certs":                                   []float64{30.0, -1},
					"start_system_logs_and_aks_log_collector":                           []float64{30.0, 20},
					"pre_install_dependencies_overall":                                  []float64{30.0, 20},
					"source_packer_files_declare_variables_and_set_mariner_permissions": []float64{30.0, 20},
				},
				"install_dependencies": {
					"download_runc": []float64{30.0, 20},
					"create_containerd_service_directory_download_shims_configure_runtime_and_network": []float64{30.0, 20},
					"configure_telemetry_create_logging_directory":                                     []float64{30.0, 20},
					"pull_nvidia_driver_image_and_run_installBcc_in_subshell":                          []float64{30.0, 20},
					"download_azure_cni":                                 []float64{15.0, 10.0},
					"download_gpu_device_plugin":                         []float64{30.0, 20},
					"download_kubernetes_binaries":                       []float64{30.0, 20},
					"declare_variables_and_source_packer_files":          []float64{30.0, 20},
					"download_containerd":                                []float64{30.0, 20},
					"artifact_streaming_and_download_teleportd":          []float64{30.0, 20},
					"configure_networking_and_interface":                 []float64{15.0, 10.0},
					"finish_installing_bcc_tools":                        []float64{30.0, 20},
					"download_azure_acr_credential_provider":             []float64{30.0, 20},
					"download_cri_tools":                                 []float64{30.0, 20},
					"purge_and_reinstall_ubuntu":                         []float64{30.0, 20},
					"pull_and_retag_container_images":                    []float64{30.0, 20},
					"install_dependencies_overall":                       []float64{30.0, 20},
					"check_container_runtime_and_network_configurations": []float64{30.0, 20},
					"download_oras":                                      []float64{30.0, 20},
					"install_dependencies":                               []float64{30.0, 20},
					"download_cni_plugins":                               []float64{30.0, 20},
				},
				"post_install_dependencies": {
					"resolve_conf":                          []float64{15.0, 10.0},
					"post_install_dependencies_overall":     []float64{30.0, 20},
					"set_variables_and_source_packer_files": []float64{30.0, 20},
					"log_and_detach_ua":                     []float64{30.0, 20},
					"determine_disk_usage":                  []float64{30.0, 20},
					"install_asc_baseline":                  []float64{30.0, 20},
					"write_logs":                            []float64{30.0, 20},
					"list_installed_packages":               []float64{30.0, 20},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.mapsStruct.ParseKustoData(c.queriedSKU)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, c.mapsStruct.QueriedPerformanceDataMap)
		})
	}
}

func TestEvaluatePerformance(t *testing.T) {

}

func TestSumSlice(t *testing.T) {
	cases := []struct {
		name     string
		slice    []float64
		expected float64
		err      error
	}{
		{
			name:     "should correctly add values in slice",
			slice:    []float64{30.0, 20},
			expected: 70.0,
			err:      nil,
		},
		{
			name:     "should return -1 if either value is -1",
			slice:    []float64{30.0, -1},
			expected: -1,
			err:      nil,
		},
		{
			name:     "should return invalid slice length error",
			slice:    []float64{30, 20, 10},
			expected: 0,
			err:      fmt.Errorf("expected 2 elements in slice, got %d: %v", len([]float64{30, 20, 10}), []float64{30, 20, 10}),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := SumSlice(c.slice)
			if c.err != nil {
				assert.EqualError(t, err, c.err.Error())
			}
			assert.Equal(t, c.expected, actual)
		})
	}
}

func TestPrintRegressions(t *testing.T) {

}
