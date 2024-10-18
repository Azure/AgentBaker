package service

import (
	"fmt"
	"os"
	"testing"

	"github.com/tj/assert"
)

func TestSetupConfig(t *testing.T) {
	cases := []struct {
		name     string
		expected *Config
		err      error
	}{
		{
			name: "should correctly initialize configuration struct",
			expected: &Config{
				KustoTable:                "<test_table>",
				KustoEndpoint:             "<https://test.kusto.endpoint>",
				KustoDatabase:             "<test_db>",
				CommonIdentityId:          "<test_id>",
				KustoIngestionMapping:     "<test_mapping>",
				SigImageName:              "<test_sig_name>",
				LocalBuildPerformanceFile: "<test_sig_name>-build-performance.json",
				SourceBranch:              "<test_branch>",
			},
			err: nil,
		},
		{
			name:     "should fail if an environment variable is missing",
			expected: nil,
			err:      fmt.Errorf("required environment variables were not set"),
		},
	}

	os.Setenv("BUILD_PERFORMANCE_TABLE_NAME", "<test_table>")
	os.Setenv("BUILD_PERFORMANCE_KUSTO_ENDPOINT", "<https://test.kusto.endpoint>")
	os.Setenv("BUILD_PERFORMANCE_DATABASE_NAME", "<test_db>")
	os.Setenv("AZURE_MSI_RESOURCE_STRING", "<test_id>")
	os.Setenv("BUILD_PERFORMANCE_INGESTION_MAPPING", "<test_mapping>")
	os.Setenv("SIG_IMAGE_NAME", "<test_sig_name>")
	os.Setenv("GIT_BRANCH", "<test_branch>")

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.name == "should fail if an environment variable is missing" {
				os.Setenv("BUILD_PERFORMANCE_TABLE_NAME", "")
				config, err := SetupConfig()
				assert.EqualError(t, err, c.err.Error())
				assert.Equal(t, config, c.expected)
			} else {
				config, err := SetupConfig()
				assert.NoError(t, err)
				assert.Equal(t, config, c.expected)
			}
		})
	}
}

func TestCreateDataMaps(t *testing.T) {
	cases := []struct {
		name     string
		expected *DataMaps
	}{
		{
			name: "should correctly return a struct of type DataMaps",
			expected: &DataMaps{
				LocalPerformanceDataMap:   EvaluationMap{},
				QueriedPerformanceDataMap: QueryMap{},
				RegressionMap:             EvaluationMap{},
				StagingMap:                StagingMap{},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := CreateDataMaps()
			assert.Equal(t, c.expected, actual)
		})
	}
}

func TestDecodeLocalPerformanceData(t *testing.T) {
	cases := []struct {
		name       string
		mapsStruct *DataMaps
		expected   EvaluationMap
	}{
		{
			name: "should correctly unmarshal local JSON file into appropriate field in DataMaps struct",
			mapsStruct: &DataMaps{
				LocalPerformanceDataMap: EvaluationMap{},
				StagingMap:              StagingMap{},
			},
			expected: EvaluationMap{
				"pre_install_dependencies": {
					"copy_packer_files":                                                 65,
					"enable_modified_log_rotate_service":                                65,
					"handle_azureLinux_and_cgroupV2":                                    65,
					"handle_mariner_and_fips_configurations":                            65,
					"make_directory_and_update_certs":                                   65,
					"pre_install_dependencies_overall":                                  65,
					"source_packer_files_declare_variables_and_set_mariner_permissions": 65,
					"start_system_logs_and_aks_log_collector":                           65,
					"sync_container_logs":                                               65,
				},
				"install_dependencies": {
					"artifact_streaming_and_download_teleportd":                                        65,
					"check_container_runtime_and_network_configurations":                               65,
					"configure_networking_and_interface":                                               65,
					"configure_telemetry_create_logging_directory":                                     65,
					"create_containerd_service_directory_download_shims_configure_runtime_and_network": 65,
					"declare_variables_and_source_packer_files":                                        65,
					"download_azure_acr_credential_provider":                                           65,
					"download_azure_cni":                                                               65,
					"download_cni_plugins":                                                             65,
					"download_containerd":                                                              65,
					"download_cri_tools":                                                               65,
					"download_gpu_device_plugin":                                                       65,
					"download_kubernetes_binaries":                                                     65,
					"download_oras":                                                                    65,
					"download_runc":                                                                    65,
					"finish_installing_bcc_tools":                                                      65,
					"install_dependencies":                                                             65,
					"install_dependencies_overall":                                                     65,
					"pull_and_retag_container_images":                                                  65,
					"pull_nvidia_driver_image_and_run_installBcc_in_subshell":                          65,
					"purge_and_reinstall_ubuntu":                                                       65,
				},
				"post_install_dependencies": {
					"determine_disk_usage":                  65,
					"install_asc_baseline":                  65,
					"list_installed_packages":               65,
					"log_and_detach_ua":                     65,
					"post_install_dependencies_overall":     65,
					"resolve_conf":                          65,
					"set_variables_and_source_packer_files": 65,
					"write_logs":                            65,
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.mapsStruct.DecodeLocalPerformanceData("../../testdata/test-build-performance-data.json")
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
		expected   QueryMap
		err        error
	}{
		{
			name: "should correctly unmarshal queried performance data into appropriate field in DataMaps struct",
			queriedSKU: &SKU{
				Name:               "AzureLinuxV6",
				SKUPerformanceData: `{"post_install_dependencies": {"resolve_conf": [15.0, 10.0], "post_install_dependencies_overall": [30.0, 20], "set_variables_and_source_packer_files": [30.0, 20], "log_and_detach_ua": [30.0, 20], "determine_disk_usage": [30.0, 20], "install_asc_baseline": [30.0, 20], "write_logs": [30.0, 20], "list_installed_packages": [30.0, 20]}, "pre_install_dependencies": {"copy_packer_files": [30.0, 20], "sync_container_logs": [30.0, 20], "handle_mariner_and_fips_configurations": [30.0, 20], "handle_azureLinux_and_cgroupV2": [30.0, 20], "enable_modified_log_rotate_service": [15.0, 10.0], "make_directory_and_update_certs": [30.0, NaN], "start_system_logs_and_aks_log_collector": [30.0, 20], "pre_install_dependencies_overall": [30.0, 20], "source_packer_files_declare_variables_and_set_mariner_permissions": [30.0, 20]}, "install_dependencies": {"download_runc": [30.0, 20], "download_azure_acr_credential_provider": [30.0, 20], "create_containerd_service_directory_download_shims_configure_runtime_and_network": [30.0, 20], "configure_telemetry_create_logging_directory": [30.0, 20], "pull_nvidia_driver_image_and_run_installBcc_in_subshell": [30.0, 20], "download_azure_cni": [15.0, 10.0], "download_gpu_device_plugin": [30.0, 20], "download_kubernetes_binaries": [30.0, 20], "declare_variables_and_source_packer_files": [30.0, 20], "download_containerd": [30.0, 20], "artifact_streaming_and_download_teleportd": [30.0, 20], "configure_networking_and_interface": [15.0, 10.0], "finish_installing_bcc_tools": [30.0, 20], "download_cri_tools": [30.0, 20], "purge_and_reinstall_ubuntu": [30.0, 20], "pull_and_retag_container_images": [30.0, 20], "install_dependencies_overall": [30.0, 20], "check_container_runtime_and_network_configurations": [30.0, 20], "download_oras": [30.0, 20], "install_dependencies": [30.0, 20], "download_cni_plugins": [30.0, 20]}}`,
			},
			mapsStruct: &DataMaps{
				QueriedPerformanceDataMap: QueryMap{},
			},
			expected: QueryMap{
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
			err: nil,
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
			expected: 90.0,
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
			if err != nil {
				assert.EqualError(t, err, c.err.Error())
			}
			assert.Equal(t, c.expected, actual)
		})
	}
}

func TestEvaluatePerformance(t *testing.T) {
	cases := []struct {
		name        string
		initialMaps *DataMaps
		expected    *DataMaps
		err         error
	}{
		{
			name: "should correctly identify regressions",
			initialMaps: &DataMaps{
				QueriedPerformanceDataMap: QueryMap{
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
				LocalPerformanceDataMap: EvaluationMap{
					"pre_install_dependencies": {
						"copy_packer_files":                                                 65,
						"enable_modified_log_rotate_service":                                65,
						"handle_azureLinux_and_cgroupV2":                                    65,
						"handle_mariner_and_fips_configurations":                            65,
						"make_directory_and_update_certs":                                   65,
						"pre_install_dependencies_overall":                                  65,
						"source_packer_files_declare_variables_and_set_mariner_permissions": 65,
						"start_system_logs_and_aks_log_collector":                           65,
						"sync_container_logs":                                               65,
					},
					"install_dependencies": {
						"artifact_streaming_and_download_teleportd":                                        65,
						"check_container_runtime_and_network_configurations":                               65,
						"configure_networking_and_interface":                                               65,
						"configure_telemetry_create_logging_directory":                                     65,
						"create_containerd_service_directory_download_shims_configure_runtime_and_network": 65,
						"declare_variables_and_source_packer_files":                                        65,
						"download_azure_acr_credential_provider":                                           65,
						"download_azure_cni":                                                               65,
						"download_cni_plugins":                                                             65,
						"download_containerd":                                                              65,
						"download_cri_tools":                                                               65,
						"download_gpu_device_plugin":                                                       65,
						"download_kubernetes_binaries":                                                     65,
						"download_oras":                                                                    65,
						"download_runc":                                                                    65,
						"finish_installing_bcc_tools":                                                      65,
						"install_dependencies":                                                             65,
						"install_dependencies_overall":                                                     65,
						"pull_and_retag_container_images":                                                  65,
						"pull_nvidia_driver_image_and_run_installBcc_in_subshell":                          65,
						"purge_and_reinstall_ubuntu":                                                       65,
					},
					"post_install_dependencies": {
						"determine_disk_usage":                  65,
						"install_asc_baseline":                  65,
						"list_installed_packages":               65,
						"log_and_detach_ua":                     65,
						"post_install_dependencies_overall":     65,
						"resolve_conf":                          65,
						"set_variables_and_source_packer_files": 65,
						"write_logs":                            65,
					},
				},
				RegressionMap: EvaluationMap{},
			},
			expected: &DataMaps{
				RegressionMap: EvaluationMap{
					"pre_install_dependencies": {
						"enable_modified_log_rotate_service": 20,
					},
					"install_dependencies": {
						"download_azure_cni":                 20,
						"configure_networking_and_interface": 20,
					},
					"post_install_dependencies": {
						"resolve_conf": 20,
					},
				},
			},
			err: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.initialMaps.EvaluatePerformance()
			assert.NoError(t, err)
			assert.Equal(t, c.expected.RegressionMap, c.initialMaps.RegressionMap)
		})
	}
}

func TestDisplayRegressions(t *testing.T) {
	cases := []struct {
		name string
		maps *DataMaps
		err  error
	}{
		{
			name: "should correctly unmarshal regression map into JSON",
			maps: &DataMaps{
				RegressionMap: EvaluationMap{
					"pre_install_dependencies": {
						"enable_modified_log_rotate_service": 20,
					},
					"install_dependencies": {
						"download_azure_cni":                 20,
						"configure_networking_and_interface": 20,
					},
					"post_install_dependencies": {
						"resolve_conf": 20,
					},
				},
				QueriedPerformanceDataMap: QueryMap{
					"pre_install_dependencies": {
						"enable_modified_log_rotate_service": []float64{15.0, 10.0},
					},
					"install_dependencies": {
						"download_azure_cni":                 []float64{15.0, 10.0},
						"configure_networking_and_interface": []float64{15.0, 10.0},
					},
					"post_install_dependencies": {
						"resolve_conf": []float64{15.0, 10.0},
					},
				},
				LocalPerformanceDataMap: EvaluationMap{
					"pre_install_dependencies": {
						"enable_modified_log_rotate_service": 65,
					},
					"install_dependencies": {
						"download_azure_cni":                 65,
						"configure_networking_and_interface": 65,
					},
					"post_install_dependencies": {
						"resolve_conf": 65,
					},
				},
			},
			err: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.maps.DisplayRegressions()
			assert.NoError(t, err)
		})
	}
}
