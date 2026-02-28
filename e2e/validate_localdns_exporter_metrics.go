package e2e

import (
	"context"
	_ "embed"
)

//go:embed localdns/validate-localdns-exporter-metrics.sh
var validateLocalDNSExporterMetricsScript string

// ValidateLocalDNSExporterMetrics checks if the localdns metrics exporter is working
// and exports the expected VnetDNS and KubeDNS forward IP metrics.
func ValidateLocalDNSExporterMetrics(ctx context.Context, s *Scenario) {
	s.T.Helper()

	execScriptOnVMForScenarioValidateExitCode(ctx, s, validateLocalDNSExporterMetricsScript, 0, "localdns exporter metrics validation failed")
}
