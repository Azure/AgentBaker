package e2e

import (
	"context"
	"encoding/base64"
	_ "embed"
	"fmt"

	"github.com/stretchr/testify/require"
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
}
