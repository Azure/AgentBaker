package e2e

import (
	"context"
	"encoding/base64"
	_ "embed"
	"fmt"

	"github.com/stretchr/testify/require"
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
func ValidateLocalDNSExporterMetrics(ctx context.Context, s *Scenario) {
	s.T.Helper()

	// Check if the node has the localdns-exporter label. This label is only set by CSE
	// when the VHD has localdns-exporter.socket installed (see cse_main.sh). If the label
	// is absent, the VHD predates the exporter feature — skip validation with a warning
	// so it's visible in test output rather than silently passing.
	// If the label IS present, the exporter must be fully working — any failure is a real bug.
	const exporterLabelKey = "kubernetes.azure.com/localdns-exporter"
	node, err := s.Runtime.Cluster.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
	require.NoError(s.T, err, "failed to get node %q", s.Runtime.VM.KubeName)

	if _, exists := node.Labels[exporterLabelKey]; !exists {
		s.T.Logf("WARNING: node %q does not have label %q — localdns exporter not installed on this VHD, skipping exporter validation",
			s.Runtime.VM.KubeName, exporterLabelKey)
		return
	}
	s.T.Logf("node %q has label %q — proceeding with full exporter validation", s.Runtime.VM.KubeName, exporterLabelKey)

	encoded := base64.StdEncoding.EncodeToString([]byte(validateLocalDNSExporterMetricsScript))
	remotePath := "/home/azureuser/validate_localdns_exporter_metrics.sh"
	remoteB64 := remotePath + ".b64"

	// Upload base64-encoded script in chunks small enough for the bastion tunnel buffer.
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
		execScriptOnVMForScenarioValidateExitCode(ctx, s, cmd, 0,
			fmt.Sprintf("failed to upload script chunk (offset %d)", i))
	}

	// Decode the base64 file into the actual script and make it executable.
	decodeCmd := fmt.Sprintf("base64 -d %s > %s && chmod +x %s && rm -f %s", remoteB64, remotePath, remotePath, remoteB64)
	execScriptOnVMForScenarioValidateExitCode(ctx, s, decodeCmd, 0, "failed to decode uploaded script")

	// Execute the script.
	result := execScriptOnVMForScenario(ctx, s, "sudo "+remotePath)
	require.Equal(s.T, "0", result.exitCode,
		"localdns exporter metrics validation failed\nstdout: %s\nstderr: %s", result.stdout, result.stderr)
	s.T.Logf("localdns exporter metrics validation output:\n%s", result.stdout)
}
