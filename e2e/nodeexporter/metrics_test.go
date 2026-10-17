package nodeexporter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateMetrics(t *testing.T) {
	err := ValidateMetrics(validMetrics(), true)

	require.NoError(t, err)
}

func TestValidateMetricsRejectsMissingMetric(t *testing.T) {
	metrics := strings.Replace(validMetrics(), `node_network_receive_bytes_total{device="eth0"} 1`, "", 1)
	metrics = strings.Replace(metrics, "node_memory_MemAvailable_bytes 1", "", 1)

	err := ValidateMetrics(metrics, true)

	require.ErrorContains(t, err, "node_memory_MemAvailable_bytes")
	require.ErrorContains(t, err, "node_network_receive_bytes_total")
}

func TestValidateMetricsRequiresPressureWhenPSIAvailable(t *testing.T) {
	err := ValidateMetrics(noPressureMetrics(), true)

	require.ErrorContains(t, err, "node_pressure_cpu_waiting_seconds_total")
}

// Kata hosts run kernel-mshv without CONFIG_PSI: the collector reports no data
// and every other metric is still required.
func TestValidateMetricsAllowsMissingPSI(t *testing.T) {
	require.NoError(t, ValidateMetrics(noPressureMetrics(), false))

	metrics := strings.Replace(noPressureMetrics(), "node_memory_MemAvailable_bytes 1", "", 1)
	require.ErrorContains(t, ValidateMetrics(metrics, false), "node_memory_MemAvailable_bytes")
}

func TestValidateMetricsRejectsPressureDataWithoutPSI(t *testing.T) {
	err := ValidateMetrics(validMetrics()+`node_scrape_collector_success{collector="pressure"} 0
`, false)
	require.ErrorContains(t, err, "present although the kernel reports no PSI")

	err = ValidateMetrics(strings.Replace(noPressureMetrics(), `collector="pressure"} 0`, `collector="pressure"} 1`, 1), false)
	require.ErrorContains(t, err, "pressure collector must report no data")
}

func noPressureMetrics() string {
	metrics := strings.Replace(validMetrics(), "# TYPE node_pressure_cpu_waiting_seconds_total counter\nnode_pressure_cpu_waiting_seconds_total 1\n", "", 1)
	return metrics + `node_scrape_collector_success{collector="pressure"} 0
`
}

func TestValidateCollectors(t *testing.T) {
	metrics := `node_exporter_build_info{version="v1.12.1"} 1
node_scrape_collector_success{collector="bcachefs"} 0
node_scrape_collector_success{collector="dmmultipath"} 1
node_scrape_collector_success{collector="kernel_hung"} 0
`

	require.NoError(t, ValidateCollectors(metrics, false, false))
}

func TestValidateCollectorsRejectsMissingCollector(t *testing.T) {
	metrics := `node_exporter_build_info{version="v1.12.1"} 1
node_scrape_collector_success{collector="bcachefs"} 0
node_scrape_collector_success{collector="kernel_hung"} 0
`

	require.ErrorContains(t, ValidateCollectors(metrics, false, false), "dmmultipath")
}

func TestValidateCollectorsRequiresInfiniBandMetricsOnInfiniBandNodes(t *testing.T) {
	metrics := `node_exporter_build_info{version="v1.12.1"} 1
node_scrape_collector_success{collector="bcachefs"} 0
node_scrape_collector_success{collector="dmmultipath"} 1
node_scrape_collector_success{collector="kernel_hung"} 0
node_scrape_collector_success{collector="infiniband"} 1
node_infiniband_state_id{device="mlx5_0",port="1"} 4
node_infiniband_out_of_buffer_drops_total{device="mlx5_0",port="1"} 0
`

	require.NoError(t, ValidateCollectors(metrics, true, false))
	require.ErrorContains(t, ValidateCollectors(strings.Replace(metrics, "node_infiniband_", "other_metric_", -1), true, false), "metrics are missing")
	// A device without optional hw_counters still satisfies the common contract.
	require.NoError(t, ValidateCollectors(strings.Replace(metrics, "node_infiniband_out_of_buffer_drops_total{device=\"mlx5_0\",port=\"1\"} 0\n", "", 1), true, false))
	require.ErrorContains(t, ValidateCollectors(strings.Replace(metrics, `collector="infiniband"} 1`, `collector="infiniband"} 0`, 1), true, false), "did not succeed")
}

func TestValidateCollectorsSupportsMainVHDExporter(t *testing.T) {
	metrics := `node_exporter_build_info{version="v1.9.1"} 1`

	require.NoError(t, ValidateCollectors(metrics, false, false))
}

func TestValidateCollectorsRequiresNewCollectorsAfter112(t *testing.T) {
	metrics := `node_exporter_build_info{version="v1.13.0"} 1`

	require.ErrorContains(t, ValidateCollectors(metrics, false, false), "bcachefs")
}

func TestValidateCollectorsRejectsMissingBuildInfo(t *testing.T) {
	require.ErrorContains(t, ValidateCollectors("", false, false), "build info")
}

func TestValidateCollectorsRequiresMANASuppression(t *testing.T) {
	metrics := `node_exporter_build_info{version="v1.12.1"} 1
node_scrape_collector_success{collector="bcachefs"} 0
node_scrape_collector_success{collector="dmmultipath"} 1
node_scrape_collector_success{collector="kernel_hung"} 0
`
	require.NoError(t, ValidateCollectors(metrics, false, true))
	for _, value := range []string{"0", "1"} {
		t.Run(value, func(t *testing.T) {
			require.ErrorContains(t, ValidateCollectors(metrics+`node_scrape_collector_success{collector="infiniband"} `+value, false, true), "enabled despite the MANA workaround")
		})
	}
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
