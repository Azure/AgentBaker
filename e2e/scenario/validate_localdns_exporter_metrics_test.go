package scenario

import (
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/localdns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestValidateLocalDNSExporterMetricsUsesSingleUpload(t *testing.T) {
	for _, test := range []struct {
		name          string
		uploadFailure bool
		exitCode      uint32
		wantError     string
	}{
		{name: "success"},
		{name: "validation failure", exitCode: 17, wantError: "localdns exporter metrics validation failed"},
		{name: "upload failure", uploadFailure: true, wantError: "transfer failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			script := "sudo /bin/bash <<'LOCALDNS_EXPORTER_VALIDATION'\n" +
				localdns.ExporterMetricsScript() + "\nLOCALDNS_EXPORTER_VALIDATION\n"
			var opens, uploads, executions atomic.Int32
			client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
				opens.Add(1)
				return serveSessionTestCommand(ch, func(command string, channel ssh.Channel) uint32 {
					if isSessionTestUpload(command) {
						uploads.Add(1)
						if test.uploadFailure {
							_, err := io.Copy(io.Discard, channel)
							assert.NoError(t, err)
							_, err = io.WriteString(channel.Stderr(), "transfer failed\n")
							assert.NoError(t, err)
							return 1
						}
						return receiveSessionTestScript(t, channel, script)
					}
					executions.Add(1)
					_, err := io.WriteString(channel, "exporter validation output\n")
					assert.NoError(t, err)
					_, err = io.WriteString(channel.Stderr(), "exporter diagnostic\n")
					assert.NoError(t, err)
					return test.exitCode
				})
			})
			s := &Scenario{
				Config: Config{VHD: &config.Image{}},
				Runtime: &ScenarioRuntime{
					VM: &ScenarioVM{KubeName: "scenario-node", SSHClient: client},
					Kube: &Kubeclient{Typed: fake.NewSimpleClientset(&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "scenario-node",
							Labels: map[string]string{"kubernetes.azure.com/localdns-exporter": "true"},
						},
					})},
				},
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			err := ValidateLocalDNSExporterMetrics(ctx, s)
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
			} else {
				require.NoError(t, err)
			}
			assert.EqualValues(t, 1, uploads.Load())
			if test.uploadFailure {
				assert.NotContains(t, err.Error(), "LOCALDNS_EXPORTER_VALIDATION", "upload errors must not include the full script")
				assert.EqualValues(t, 1, opens.Load())
				assert.Zero(t, executions.Load())
			} else {
				assert.EqualValues(t, 2, opens.Load())
				assert.EqualValues(t, 1, executions.Load())
				if test.exitCode != 0 {
					assert.ErrorContains(t, err, "exporter validation output")
					assert.ErrorContains(t, err, "exporter diagnostic")
				}
			}
		})
	}
}

func TestValidateLocalDNSExporterMetricsSkipsOlderVHD(t *testing.T) {
	s := &Scenario{Runtime: &ScenarioRuntime{
		VM: &ScenarioVM{KubeName: "scenario-node"},
		Kube: &Kubeclient{Typed: fake.NewSimpleClientset(&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "scenario-node"},
		})},
	}}
	require.NoError(t, ValidateLocalDNSExporterMetrics(t.Context(), s))
}
