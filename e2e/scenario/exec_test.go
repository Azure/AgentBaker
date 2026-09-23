package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const execProxy502 = `Internal error occurred: error sending request: Post "https://192.0.2.10:10250/exec/pod/container": proxy error from 127.0.0.1:9443 while dialing 192.0.2.10:10250, code 502: 502 Bad Gateway`

func TestExecOnPodProxy502(t *testing.T) {
	for _, tc := range []struct {
		name         string
		failures     int32
		message      string
		nonzero      bool
		cancel       bool
		wantError    bool
		wantAttempts int32
	}{
		{name: "recovery", failures: 2, message: execProxy502, wantAttempts: 3},
		{name: "exhaustion", failures: 3, message: execProxy502, wantError: true, wantAttempts: 3},
		{name: "other proxy status", failures: 3, message: strings.ReplaceAll(execProxy502, "502", "503"), wantError: true, wantAttempts: 1},
		{name: "command exit", nonzero: true, wantAttempts: 1},
		{name: "cancellation", failures: 3, message: execProxy502, cancel: true, wantError: true, wantAttempts: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			failure, err := json.Marshal(metav1.Status{
				Status:  metav1.StatusFailure,
				Reason:  metav1.StatusReasonInternalError,
				Code:    http.StatusInternalServerError,
				Message: tc.message,
			})
			require.NoError(t, err)
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempt := attempts.Add(1)
				conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"v5.channel.k8s.io"}})
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.CloseNow()
				status := []byte(`{"status":"Success"}`)
				if attempt <= tc.failures {
					status = failure
				} else if tc.nonzero {
					status = []byte(`{"status":"Failure","reason":"NonZeroExitCode","details":{"causes":[{"reason":"ExitCode","message":"28"}]}}`)
				}
				for _, frame := range [][]byte{
					append([]byte{1}, fmt.Sprintf("stdout-%d", attempt)...),
					append([]byte{2}, execProxy502...),
					append([]byte{3}, status...),
				} {
					if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
						t.Error(err)
						return
					}
				}
				if err := conn.Close(websocket.StatusNormalClosure, ""); err != nil {
					t.Error(err)
				}
				if tc.cancel {
					cancel()
				}
			}))
			defer server.Close()
			config := &rest.Config{Host: server.URL}
			client, err := kubernetes.NewForConfig(config)
			require.NoError(t, err)
			result, err := execOnPod(ctx, &Kubeclient{RESTConfig: config, Typed: client},
				"default", "debug", []string{"systemctl", "cat", "example.service"})
			assert.Equal(t, tc.wantAttempts, attempts.Load())
			if tc.wantError {
				require.Error(t, err)
				assert.Nil(t, result)
				if tc.cancel {
					assert.ErrorIs(t, err, context.Canceled)
				} else {
					assert.ErrorContains(t, err, tc.message)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			code := "0"
			if tc.nonzero {
				code = "28"
			}
			assert.Equal(t, code, result.exitCode)
			assert.Equal(t, fmt.Sprintf("stdout-%d", tc.wantAttempts), result.stdout)
			assert.Equal(t, execProxy502, result.stderr)
		})
	}
}

func TestIsRetryableConnectionErrorProxy502(t *testing.T) {
	assert.True(t, isRetryableConnectionError(errors.New(execProxy502)))
	for _, message := range []string{
		"HTTP 502 Bad Gateway",
		strings.ReplaceAll(execProxy502, "502", "5020"),
		strings.ReplaceAll(execProxy502, "502", "403"),
		"command terminated with exit code 28",
	} {
		assert.False(t, isRetryableConnectionError(errors.New(message)), message)
	}
}
