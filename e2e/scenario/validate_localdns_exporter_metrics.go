package scenario

import (
	"context"
	"fmt"

	"github.com/Azure/agentbaker/e2e/assert"
	"github.com/Azure/agentbaker/e2e/localdns"
	"github.com/Azure/agentbaker/e2e/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValidateLocalDNSExporterMetrics checks if the localdns metrics exporter is working
// and exports the expected VnetDNS and KubeDNS forward IP metrics.
func ValidateLocalDNSExporterMetrics(ctx context.Context, s *Scenario) error {
	// Check if the node has the localdns-exporter label. This label is only set by CSE
	// when the VHD has localdns-exporter.socket installed (see cse_main.sh). If the label
	// is absent, the VHD predates the exporter feature — skip validation with a warning
	// so it's visible in test output rather than silently passing.
	// If the label IS present, the exporter must be fully working — any failure is a real bug.
	const exporterLabelKey = "kubernetes.azure.com/localdns-exporter"
	node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %q: %w", s.Runtime.VM.KubeName, err)
	}

	if _, exists := node.Labels[exporterLabelKey]; !exists {
		logging.Logf(ctx, "WARNING: node %q does not have label %q — localdns exporter not installed on this VHD, skipping exporter validation",
			s.Runtime.VM.KubeName, exporterLabelKey)
		return nil
	}
	logging.Logf(ctx, "node %q has label %q — proceeding with full exporter validation", s.Runtime.VM.KubeName, exporterLabelKey)

	command := "sudo /bin/bash <<'LOCALDNS_EXPORTER_VALIDATION'\n" +
		localdns.ExporterMetricsScript() + "\nLOCALDNS_EXPORTER_VALIDATION\n"
	result, err := execScriptOnVm(ctx, s, s.Runtime.VM, command)
	if err != nil {
		return fmt.Errorf("failed to run localdns exporter metrics validation script: %w", err)
	}
	if err := assert.Equal(result.exitCode, "0",
		"localdns exporter metrics validation failed\nstdout: %s\nstderr: %s", result.stdout, result.stderr); err != nil {
		return err
	}
	logging.Logf(ctx, "localdns exporter metrics validation output:\n%s", result.stdout)
	return nil
}
