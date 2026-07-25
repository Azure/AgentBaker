package nodeexporter

import (
	"fmt"
	"strings"
)

// ValidateMetrics verifies that a node-exporter scrape contains every metric
// retained by the AKS default Prometheus profile.
func ValidateMetrics(metricsText string) error {
	metricNames := make(map[string]struct{})
	for line := range strings.SplitSeq(metricsText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if end := strings.IndexAny(line, "{ \t"); end > 0 {
			metricNames[line[:end]] = struct{}{}
		}
	}

	requiredMetrics := []string{
		"node_disk_read_time_seconds_total",
		"node_disk_reads_completed_total",
		"node_disk_write_time_seconds_total",
		"node_disk_writes_completed_total",
		"node_memory_MemAvailable_bytes",
		"node_network_receive_bytes_total",
		"node_network_receive_errs_total",
		"node_network_receive_packets_total",
		"node_network_transmit_bytes_total",
		"node_network_transmit_errs_total",
		"node_network_transmit_packets_total",
		"node_netstat_Tcp_RetransSegs",
		"node_pressure_cpu_waiting_seconds_total",
		"node_filesystem_free_bytes",
		"node_filesystem_size_bytes",
	}
	var missingMetrics []string
	for _, name := range requiredMetrics {
		if _, exists := metricNames[name]; !exists {
			missingMetrics = append(missingMetrics, name)
		}
	}
	if len(missingMetrics) > 0 {
		return fmt.Errorf("required metrics are missing: %s", strings.Join(missingMetrics, ", "))
	}

	return nil
}
