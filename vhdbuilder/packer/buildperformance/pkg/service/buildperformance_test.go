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

}
func TestConvertTimestampsToSeconds(t *testing.T) {

}

func TestCleanData(t *testing.T) {
	cases := []struct {
		name       string
		queriedSKU *SKU
		expected   string
	}{
		{
			name: "should correctly remove NaN values from queried data to enable evaluation",
			queriedSKU: &SKU{
				Name:               "AzureLinuxV6",
				SKUPerformanceData: `{"post_install_dependencies": {"resolve_conf": [15.0, 10.0], "post_install_dependencies_overall": [30.0, 20], "set_variables_and_source_packer_files": [30.0, 20], "log_and_detach_ua": [30.0, 20], "determine_disk_usage": [30.0, 20], "install_asc_baseline": [30.0, 20], "write_logs": [30.0, 20], "list_installed_packages": [30.0, 20]}, "pre_install_dependencies": {"copy_packer_files": [30.0, 20], "sync_container_logs": [30.0, 20], "handle_mariner_and_fips_configurations": [30.0, 20], "handle_azureLinux_and_cgroupV2": [30.0, 20], "enable_modified_log_rotate_service": [15.0, 10.0], "make_directory_and_update_certs": [30.0, NaN], "start_system_logs_and_aks_log_collector": [30.0, 20], "pre_install_dependencies_overall": [30.0, 20], "source_packer_files_declare_variables_and_set_mariner_permissions": [30.0, 20]}, "install_dependencies": {"download_runc": [30.0, 20], "create_containerd_service_directory_download_shims_configure_runtime_and_network": [30.0, 20], "configure_telemetry_create_logging_directory": [30.0, 20], "pull_nvidia_driver_image_and_run_installBcc_in_subshell": [30.0, 20], "download_azure_cni": [15.0, 10.0], "download_gpu_device_plugin": [30.0, 20], "download_kubernetes_binaries": [30.0, 20], "declare_variables_and_source_packer_files": [30.0, 20], "download_containerd": [30.0, 20], "artifact_streaming_and_download_teleportd": [30.0, 20], "configure_networking_and_interface": [15.0, 10.0], "finish_installing_bcc_tools": [30.0, 20], "download_cri_tools": [30.0, 20], "purge_and_reinstall_ubuntu": [30.0, 20], "pull_and_retag_container_images": [30.0, 20], "install_dependencies_overall": [30.0, 20], "check_container_runtime_and_network_configurations": [30.0, 20], "download_oras": [30.0, 20], "install_dependencies": [30.0, 20], "download_cni_plugins": [30.0, 20]}}`,
			},
			expected: `{"post_install_dependencies": {"resolve_conf": [15.0, 10.0], "post_install_dependencies_overall": [30.0, 20], "set_variables_and_source_packer_files": [30.0, 20], "log_and_detach_ua": [30.0, 20], "determine_disk_usage": [30.0, 20], "install_asc_baseline": [30.0, 20], "write_logs": [30.0, 20], "list_installed_packages": [30.0, 20]}, "pre_install_dependencies": {"copy_packer_files": [30.0, 20], "sync_container_logs": [30.0, 20], "handle_mariner_and_fips_configurations": [30.0, 20], "handle_azureLinux_and_cgroupV2": [30.0, 20], "enable_modified_log_rotate_service": [15.0, 10.0], "make_directory_and_update_certs": [30.0, -1], "start_system_logs_and_aks_log_collector": [30.0, 20], "pre_install_dependencies_overall": [30.0, 20], "source_packer_files_declare_variables_and_set_mariner_permissions": [30.0, 20]}, "install_dependencies": {"download_runc": [30.0, 20], "create_containerd_service_directory_download_shims_configure_runtime_and_network": [30.0, 20], "configure_telemetry_create_logging_directory": [30.0, 20], "pull_nvidia_driver_image_and_run_installBcc_in_subshell": [30.0, 20], "download_azure_cni": [15.0, 10.0], "download_gpu_device_plugin": [30.0, 20], "download_kubernetes_binaries": [30.0, 20], "declare_variables_and_source_packer_files": [30.0, 20], "download_containerd": [30.0, 20], "artifact_streaming_and_download_teleportd": [30.0, 20], "configure_networking_and_interface": [15.0, 10.0], "finish_installing_bcc_tools": [30.0, 20], "download_cri_tools": [30.0, 20], "purge_and_reinstall_ubuntu": [30.0, 20], "pull_and_retag_container_images": [30.0, 20], "install_dependencies_overall": [30.0, 20], "check_container_runtime_and_network_configurations": [30.0, 20], "download_oras": [30.0, 20], "install_dependencies": [30.0, 20], "download_cni_plugins": [30.0, 20]}}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := c.queriedSKU.CleanData()
			assert.Equal(t, c.expected, actual)
		})
	}
}

func TestParseKustoData(t *testing.T) {
	cases := []struct {
		name       string
		queriedSKU *SKU
		mapsStruct *DataMaps
		expected   string
	}{
		{
			name: "should correctly unmarshal JSON file into appropriate field in DataMaps struct",
			queriedSKU: &SKU{
				Name:               "AzureLinuxV6",
				SKUPerformanceData: `{"post_install_dependencies": {"resolve_conf": [15.0, 10.0], "post_install_dependencies_overall": [30.0, 20], "set_variables_and_source_packer_files": [30.0, 20], "log_and_detach_ua": [30.0, 20], "determine_disk_usage": [30.0, 20], "install_asc_baseline": [30.0, 20], "write_logs": [30.0, 20], "list_installed_packages": [30.0, 20]}, "pre_install_dependencies": {"copy_packer_files": [30.0, 20], "sync_container_logs": [30.0, 20], "handle_mariner_and_fips_configurations": [30.0, 20], "handle_azureLinux_and_cgroupV2": [30.0, 20], "enable_modified_log_rotate_service": [15.0, 10.0], "make_directory_and_update_certs": [30.0, NaN], "start_system_logs_and_aks_log_collector": [30.0, 20], "pre_install_dependencies_overall": [30.0, 20], "source_packer_files_declare_variables_and_set_mariner_permissions": [30.0, 20]}, "install_dependencies": {"download_runc": [30.0, 20], "create_containerd_service_directory_download_shims_configure_runtime_and_network": [30.0, 20], "configure_telemetry_create_logging_directory": [30.0, 20], "pull_nvidia_driver_image_and_run_installBcc_in_subshell": [30.0, 20], "download_azure_cni": [15.0, 10.0], "download_gpu_device_plugin": [30.0, 20], "download_kubernetes_binaries": [30.0, 20], "declare_variables_and_source_packer_files": [30.0, 20], "download_containerd": [30.0, 20], "artifact_streaming_and_download_teleportd": [30.0, 20], "configure_networking_and_interface": [15.0, 10.0], "finish_installing_bcc_tools": [30.0, 20], "download_cri_tools": [30.0, 20], "purge_and_reinstall_ubuntu": [30.0, 20], "pull_and_retag_container_images": [30.0, 20], "install_dependencies_overall": [30.0, 20], "check_container_runtime_and_network_configurations": [30.0, 20], "download_oras": [30.0, 20], "install_dependencies": [30.0, 20], "download_cni_plugins": [30.0, 20]}}`,
			},
			mapsStruct: &DataMaps{
				QueriedPerformanceDataMap: map[string]map[string][]float64{},
			},
			expected: "holdingValue",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := c.mapsStruct.ParseKustoData(c.queriedSKU)
			assert.Equal(t, c.expected, actual)
		})
	}

}

func TestEvaluatePerformance(t *testing.T) {

}

func TestSumArray(t *testing.T) {

}

func TestPrintRegressions(t *testing.T) {

}
