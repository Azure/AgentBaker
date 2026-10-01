package localdns

import _ "embed"

//go:embed validate-localdns-exporter-metrics.sh
var exporterMetricsScript string

func ExporterMetricsScript() string {
	return exporterMetricsScript
}
