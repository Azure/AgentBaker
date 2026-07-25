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

	err := ValidateMetrics(metrics)

	require.ErrorContains(t, err, `required metric "node_network_receive_bytes_total" is missing`)
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
