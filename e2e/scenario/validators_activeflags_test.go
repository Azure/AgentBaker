package scenario

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestValidateKubeletActiveFlagsEventUsesHost(t *testing.T) {
	for _, tt := range []struct {
		name           string
		presenceExit   uint32
		telemetryExit  uint32
		wantValidation int32
		wantError      string
	}{
		{name: "valid host telemetry", wantValidation: 1},
		{name: "service absent on host", presenceExit: 1},
		{name: "invalid host telemetry", telemetryExit: 1, wantValidation: 1, wantError: "failed to validate emit-kubelet-active-flags.service"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var apiRequests, presenceChecks, validations atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiRequests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/exec") {
					w.WriteHeader(http.StatusBadGateway)
					_, _ = io.WriteString(w, `{"kind":"Status","apiVersion":"v1","status":"Failure","reason":"InternalError","code":502,
						"message":"proxy error from localhost:9443 while dialing 10.220.112.100:10250, code 502: 502 Bad Gateway"}`)
					return
				}
				assert.Equal(t, "/api/v1/namespaces/default/pods", r.URL.Path)
				_, _ = io.WriteString(w, `{"kind":"PodList","apiVersion":"v1","items":[
					{"metadata":{"name":"debug-pod","namespace":"default"},"spec":{"nodeName":"test-node"},
					"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"}]}}]}`)
			}))
			defer server.Close()
			restConfig := &rest.Config{Host: server.URL}
			typed, err := kubernetes.NewForConfig(restConfig)
			require.NoError(t, err)
			client := newActiveFlagsTestSSHClient(t, tt.presenceExit, tt.telemetryExit, &presenceChecks, &validations)
			s := &Scenario{
				Config: Config{VHD: &config.Image{OS: config.OSUbuntu}},
				Runtime: &ScenarioRuntime{
					VM:   &ScenarioVM{SSHClient: client, KubeName: "test-node"},
					Kube: &Kubeclient{Typed: typed, RESTConfig: restConfig},
				},
			}
			ctx, cancel := context.WithTimeout(logging.WithLogger(t.Context(), t), 5*time.Second)
			defer cancel()
			err = ValidateKubeletActiveFlagsEvent(ctx, s)
			if tt.wantError != "" {
				assert.ErrorContains(t, err, tt.wantError)
			} else {
				assert.NoError(t, err)
			}
			assert.Zero(t, apiRequests.Load(), "host service validation must not use pod discovery or exec")
			assert.EqualValues(t, 1, presenceChecks.Load())
			assert.Equal(t, tt.wantValidation, validations.Load())
		})
	}
}

const activeFlagsTestScript = `set -ex
journalctl -u emit-kubelet-active-flags.service --no-pager | grep -q "Finished\|Deactivated successfully"
grep -rl 'kubeletActiveFlags' /var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/ | head -1 | xargs cat | ` +
	`jq -e '.TaskName == "AKS.CSE.ensureKubelet.kubeletActiveFlags"'`

func newActiveFlagsTestSSHClient(t *testing.T, presenceExit, telemetryExit uint32, presenceChecks, validations *atomic.Int32) *SSHClient {
	t.Helper()
	scripts := make(chan string, 1)
	return newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		return serveSessionTestCommand(ch, func(command string, channel ssh.Channel) uint32 {
			switch {
			case command == "systemctl cat emit-kubelet-active-flags.service 2>/dev/null":
				presenceChecks.Add(1)
				return presenceExit
			case strings.HasPrefix(command, "scp -qt "):
				script, readErr := readActiveFlagsTestScript(channel)
				if !assert.NoError(t, readErr) {
					return 1
				}
				scripts <- script
				return 0
			case strings.HasPrefix(command, "/home/azureuser/remote_script_"):
				validations.Add(1)
				select {
				case script := <-scripts:
					assert.Equal(t, activeFlagsTestScript, script)
				default:
					t.Error("host validation ran without uploading its script")
				}
				return telemetryExit
			default:
				t.Errorf("unexpected host command: %s", command)
				return 1
			}
		})
	})
}

func readActiveFlagsTestScript(channel ssh.Channel) (string, error) {
	reader := bufio.NewReader(channel)
	header, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	var size int
	var name string
	if _, err := fmt.Sscanf(header, "C0755 %d %s", &size, &name); err != nil {
		return "", err
	}
	if size != len(activeFlagsTestScript) || !strings.HasPrefix(name, "remote_script_") {
		return "", fmt.Errorf("unexpected script header: %q", header)
	}
	if _, err := channel.Write([]byte{0}); err != nil {
		return "", err
	}
	content := make([]byte, size+1)
	if _, err := io.ReadFull(reader, content); err != nil {
		return "", err
	}
	if _, err := channel.Write([]byte{0}); err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(content), "\x00"), nil
}
