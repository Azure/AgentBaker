package nodeexporter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateMetrics(t *testing.T) {
	err := ValidateMetrics(validMetrics())

	require.NoError(t, err)
}

func TestValidateMetricsRejectsMissingMetric(t *testing.T) {
	metrics := strings.Replace(validMetrics(), `node_network_receive_bytes_total{device="eth0"} 1`, "", 1)
	metrics = strings.Replace(metrics, "node_memory_MemAvailable_bytes 1", "", 1)

	err := ValidateMetrics(metrics)

	require.ErrorContains(t, err, "node_memory_MemAvailable_bytes")
	require.ErrorContains(t, err, "node_network_receive_bytes_total")
}

func TestValidateCollectors(t *testing.T) {
	metrics := `node_exporter_build_info{version="v1.12.1"} 1
node_scrape_collector_success{collector="bcachefs"} 0
node_scrape_collector_success{collector="dmmultipath"} 1
node_scrape_collector_success{collector="kernel_hung"} 0
`

	require.NoError(t, ValidateCollectors(metrics, false))
}

func TestValidateCollectorsRejectsMissingCollector(t *testing.T) {
	metrics := `node_exporter_build_info{version="v1.12.1"} 1
node_scrape_collector_success{collector="bcachefs"} 0
node_scrape_collector_success{collector="kernel_hung"} 0
`

	require.ErrorContains(t, ValidateCollectors(metrics, false), "dmmultipath")
}

func TestValidateCollectorsRequiresInfiniBandMetricsOnInfiniBandNodes(t *testing.T) {
	metrics := `node_exporter_build_info{version="v1.12.1"} 1
node_scrape_collector_success{collector="bcachefs"} 0
node_scrape_collector_success{collector="dmmultipath"} 1
node_scrape_collector_success{collector="kernel_hung"} 0
node_scrape_collector_success{collector="infiniband"} 1
node_infiniband_port_state{device="mlx5_0",port="1",state="ACTIVE"} 1
node_infiniband_out_of_buffer_drops_total{device="mlx5_0",port="1"} 0
`

	require.NoError(t, ValidateCollectors(metrics, true))
	require.ErrorContains(t, ValidateCollectors(strings.Replace(metrics, "node_infiniband_", "other_metric_", -1), true), "metrics are missing")
	require.ErrorContains(t, ValidateCollectors(strings.Replace(metrics, "\nnode_infiniband_out_of_buffer_drops_total", "\nother_metric", 1), true), "hardware counter metrics are missing")
	require.ErrorContains(t, ValidateCollectors(strings.Replace(metrics, `collector="infiniband"} 1`, `collector="infiniband"} 0`, 1), true), "did not succeed")
}

func TestValidateCollectorsSupportsMainVHDExporter(t *testing.T) {
	metrics := `node_exporter_build_info{version="v1.9.1"} 1`

	require.NoError(t, ValidateCollectors(metrics, false))
}

func TestValidateCollectorsRequiresNewCollectorsAfter112(t *testing.T) {
	metrics := `node_exporter_build_info{version="v1.13.0"} 1`

	require.ErrorContains(t, ValidateCollectors(metrics, false), "bcachefs")
}

func TestValidateCollectorsRejectsMissingBuildInfo(t *testing.T) {
	require.ErrorContains(t, ValidateCollectors("", false), "build info")
}

func validMetrics() string {
	return `# TYPE node_disk_read_time_seconds_total counter
node_disk_read_time_seconds_total{device="sda"} 1
# TYPE node_disk_reads_completed_total counter
node_disk_reads_completed_total{device="sda"} 1
# TYPE node_disk_write_time_seconds_total counter
node_disk_write_time_seconds_total{device="sda"} 1
# TYPE node_disk_writes_completed_total counter
node_disk_writes_completed_total{device="sda"} 1
# TYPE node_memory_MemAvailable_bytes gauge
node_memory_MemAvailable_bytes 1
# TYPE node_network_receive_bytes_total counter
node_network_receive_bytes_total{device="eth0"} 1
# TYPE node_network_receive_errs_total counter
node_network_receive_errs_total{device="eth0"} 0
# TYPE node_network_receive_packets_total counter
node_network_receive_packets_total{device="eth0"} 1
# TYPE node_network_transmit_bytes_total counter
node_network_transmit_bytes_total{device="eth0"} 1
# TYPE node_network_transmit_errs_total counter
node_network_transmit_errs_total{device="eth0"} 0
# TYPE node_network_transmit_packets_total counter
node_network_transmit_packets_total{device="eth0"} 1
# TYPE node_netstat_Tcp_RetransSegs untyped
node_netstat_Tcp_RetransSegs 1
# TYPE node_pressure_cpu_waiting_seconds_total counter
node_pressure_cpu_waiting_seconds_total 1
# TYPE node_filesystem_free_bytes gauge
node_filesystem_free_bytes{device="/dev/sda1",fstype="ext4",mountpoint="/"} 1
# TYPE node_filesystem_size_bytes gauge
node_filesystem_size_bytes{device="/dev/sda1",fstype="ext4",mountpoint="/"} 2
`
}
