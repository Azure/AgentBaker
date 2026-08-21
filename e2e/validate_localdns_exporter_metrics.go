package e2e

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"

	"github.com/Azure/agentbaker/e2e/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed localdns/validate-localdns-exporter-metrics.sh
var validateLocalDNSExporterMetricsScript string

// ValidateLocalDNSExporterMetrics checks if the localdns metrics exporter is working
// and exports the expected VnetDNS and KubeDNS forward IP metrics.
//
// The validation script is too large (~18KB) to send as a single command over
// bastion SSH tunnels which have an 8KB WebSocket buffer limit. To work around
// this, we encode the script in base64, upload it in small chunks via multiple
// SSH commands, then decode and execute it on the VM.
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
		s.Logger.Logf("WARNING: node %q does not have label %q — localdns exporter not installed on this VHD, skipping exporter validation",
			s.Runtime.VM.KubeName, exporterLabelKey)
		return nil
	}
	s.Logger.Logf("node %q has label %q — proceeding with full exporter validation", s.Runtime.VM.KubeName, exporterLabelKey)

	encoded := base64.StdEncoding.EncodeToString([]byte(validateLocalDNSExporterMetricsScript))
	remotePath := "/home/azureuser/validate_localdns_exporter_metrics.sh"
	remoteB64 := remotePath + ".b64"

	// Upload base64-encoded script in chunks small enough for the bastion tunnel buffer.
	// Each chunk appends to the previous one, so a failed chunk leaves a truncated script:
	// abort rather than continue.
	const chunkSize = 4096
	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]

		var cmd string
		if i == 0 {
			cmd = fmt.Sprintf("echo -n '%s' > %s", chunk, remoteB64)
		} else {
			cmd = fmt.Sprintf("echo -n '%s' >> %s", chunk, remoteB64)
		}
		if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, cmd, 0,
			fmt.Sprintf("failed to upload script chunk (offset %d)", i)); err != nil {
			return err
		}
	}

	// Decode the base64 file into the actual script and make it executable.
	decodeCmd := fmt.Sprintf("base64 -d %s > %s && chmod +x %s && rm -f %s", remoteB64, remotePath, remotePath, remoteB64)
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, decodeCmd, 0, "failed to decode uploaded script"); err != nil {
		return err
	}

	// Execute the script.
	result, err := execScriptOnVMForScenario(ctx, s, "sudo "+remotePath)
	if err != nil {
		return fmt.Errorf("failed to run localdns exporter metrics validation script: %w", err)
	}
	if err := assert.Equal(result.exitCode, "0",
		"localdns exporter metrics validation failed\nstdout: %s\nstderr: %s", result.stdout, result.stderr); err != nil {
		return err
	}
	s.Logger.Logf("localdns exporter metrics validation output:\n%s", result.stdout)
	return nil
}
