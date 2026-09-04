package nodeexporter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	metricNamePattern          = regexp.MustCompile(`^([^{ \t]+)(?:\{[^}]*\})?[ \t]+`)
	nodeExporterVersionPattern = regexp.MustCompile(`(?m)^node_exporter_build_info\{[^\n}]*version="v([0-9]+)\.([0-9]+)\.[^"}]+"[^\n}]*\} 1$`)
)

// ValidateMetrics verifies that a node-exporter scrape contains every metric
// retained by the AKS default Prometheus profile.
func ValidateMetrics(metricsText string) error {
	metricNames := parseMetricNames(metricsText)

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

func parseMetricNames(metricsText string) map[string]struct{} {
	metricNames := make(map[string]struct{})
	for line := range strings.SplitSeq(metricsText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if match := metricNamePattern.FindStringSubmatch(line); len(match) == 2 {
			metricNames[match[1]] = struct{}{}
		}
	}
	return metricNames
}

// ValidateCollectors verifies that the collectors enabled by AgentBaker are present in the scrape.
// InfiniBand metrics are required only when the node has InfiniBand hardware.
func ValidateCollectors(metricsText string, requireInfiniBand bool) error {
	isVersion112OrNewer, err := nodeExporterVersionAtLeast(metricsText, 1, 12)
	if err != nil {
		return err
	}
	if isVersion112OrNewer {
		requiredCollectors := []string{"bcachefs", "dmmultipath", "kernel_hung"}
		var missingCollectors []string
		for _, collector := range requiredCollectors {
			if !strings.Contains(metricsText, `node_scrape_collector_success{collector="`+collector+`"}`) {
				missingCollectors = append(missingCollectors, collector)
			}
		}
		if len(missingCollectors) > 0 {
			return fmt.Errorf("enabled collectors are missing: %s", strings.Join(missingCollectors, ", "))
		}
	}

	if requireInfiniBand {
		if !strings.Contains(metricsText, `node_scrape_collector_success{collector="infiniband"} 1`) {
			return fmt.Errorf("InfiniBand collector did not succeed")
		}
		hasInfiniBandMetric := false
		for name := range parseMetricNames(metricsText) {
			if strings.HasPrefix(name, "node_infiniband_") {
				hasInfiniBandMetric = true
				break
			}
		}
		if !hasInfiniBandMetric {
			return fmt.Errorf("InfiniBand metrics are missing")
		}

		if isVersion112OrNewer {
			hardwareCounterMetrics := []string{
				"node_infiniband_duplicate_requests_packets_total",
				"node_infiniband_lifespan_seconds",
				"node_infiniband_out_of_buffer_drops_total",
				"node_infiniband_rx_write_requests_total",
			}
			hasHardwareCounterMetric := false
			metricNames := parseMetricNames(metricsText)
			for _, name := range hardwareCounterMetrics {
				if _, exists := metricNames[name]; exists {
					hasHardwareCounterMetric = true
					break
				}
			}
			if !hasHardwareCounterMetric {
				return fmt.Errorf("InfiniBand hardware counter metrics are missing")
			}
		}
	}

	return nil
}

func nodeExporterVersionAtLeast(metricsText string, requiredMajor, requiredMinor int) (bool, error) {
	match := nodeExporterVersionPattern.FindStringSubmatch(metricsText)
	if len(match) != 3 {
		return false, fmt.Errorf("node exporter build info is missing a valid version")
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return false, fmt.Errorf("parse node exporter major version: %w", err)
	}
	minor, err := strconv.Atoi(match[2])
	if err != nil {
		return false, fmt.Errorf("parse node exporter minor version: %w", err)
	}
	return major > requiredMajor || major == requiredMajor && minor >= requiredMinor, nil
}
