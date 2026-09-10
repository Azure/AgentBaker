package e2e

import (
	"strings"
	"testing"
)

func TestValidateWindowsExporterMetrics(t *testing.T) {
	if err := validateWindowsExporterMetrics(validWindowsExporterMetrics()); err != nil {
		t.Fatalf("expected valid metrics: %v", err)
	}
}

func TestValidateWindowsExporterOwnership(t *testing.T) {
	for _, tc := range []struct {
		name    string
		output  string
		owned   bool
		wantErr bool
	}{
		{name: "old VHD without assets", output: "SKIP\r\n"},
		{name: "successful takeover", output: "PRESENT\r\n", owned: true},
		{name: "failed takeover on baked VHD", output: "MISSING\r\n", wantErr: true},
		{name: "empty output", wantErr: true},
		{name: "unexpected output containing skip", output: "unexpected SKIP text", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owned, err := validateWindowsExporterOwnership(tc.output)
			if owned != tc.owned || (err != nil) != tc.wantErr {
				t.Fatalf("got owned=%t, err=%v; want owned=%t, error=%t", owned, err, tc.owned, tc.wantErr)
			}
		})
	}
}

func TestValidateWindowsExporterMetricsRejectsMissingMetrics(t *testing.T) {
	metrics := strings.Replace(validWindowsExporterMetrics(), `windows_memory_available_bytes 1`, "", 1)
	metrics = strings.Replace(metrics, `windows_net_bytes_received_total{nic="Ethernet"} 1`, "", 1)

	err := validateWindowsExporterMetrics(metrics)

	if err == nil {
		t.Fatal("expected missing metrics to fail validation")
	}
	if !strings.Contains(err.Error(), "windows_memory_available_bytes") {
		t.Errorf("expected error to mention windows_memory_available_bytes: %v", err)
	}
	if !strings.Contains(err.Error(), "windows_net_bytes_received_total") {
		t.Errorf("expected error to mention windows_net_bytes_received_total: %v", err)
	}
}

func validWindowsExporterMetrics() string {
	return `# TYPE windows_cpu_info gauge
windows_cpu_info{device_id="CPU0"} 1
# TYPE windows_cpu_time_total counter
windows_cpu_time_total{core="0,0",mode="idle"} 1
# TYPE windows_logical_disk_free_bytes gauge
windows_logical_disk_free_bytes{volume="C:"} 1
# TYPE windows_logical_disk_size_bytes gauge
windows_logical_disk_size_bytes{volume="C:"} 2
# TYPE windows_memory_available_bytes gauge
windows_memory_available_bytes 1
# TYPE windows_net_bytes_received_total counter
windows_net_bytes_received_total{nic="Ethernet"} 1
# TYPE windows_net_bytes_sent_total counter
windows_net_bytes_sent_total{nic="Ethernet"} 1
# TYPE windows_os_info gauge
windows_os_info{product="Windows Server 2022 Datacenter"} 1
# TYPE windows_pagefile_free_bytes gauge
windows_pagefile_free_bytes{page="_Total"} 1
# TYPE windows_process_cpu_time_total counter
windows_process_cpu_time_total{process="kubelet",process_id="1",mode="user"} 1
`
}
