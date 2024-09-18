package service

import (
	"testing"

	"github.com/tj/assert"
)

func TestSetupConfig(t *testing.T) {

}
func TestCreateDataMaps(t *testing.T) {

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

func TestEvaluatePerformance(t *testing.T) {

}

func TestSumArray(t *testing.T) {

}

func TestPrintRegressions(t *testing.T) {

}
