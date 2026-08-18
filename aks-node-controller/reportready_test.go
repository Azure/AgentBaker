package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testGoalState = `<GoalState>
	<Incarnation>42</Incarnation>
	<Container>
		<ContainerId>container-id</ContainerId>
		<RoleInstanceList>
			<RoleInstance><InstanceId>instance-id</InstanceId></RoleInstance>
		</RoleInstanceList>
	</Container>
</GoalState>`

func newTestProvisioningReporter(t *testing.T, endpoint string) provisioningReporter {
	t.Helper()
	vmIDPath := filepath.Join(t.TempDir(), "product_uuid")
	require.NoError(t, os.WriteFile(vmIDPath, []byte("ABC-123\n"), 0o600))
	return provisioningReporter{
		endpoint:   endpoint,
		kvpPath:    filepath.Join(t.TempDir(), "kvp"),
		vmIDPath:   vmIDPath,
		httpClient: http.DefaultClient,
		now: func() time.Time {
			return time.Date(2026, time.August, 21, 7, 39, 36, 123456789, time.UTC)
		},
		runDmesg: func(context.Context) ([]byte, error) {
			return []byte("kernel output"), nil
		},
	}
}

func TestProvisioningReporterReportReady(t *testing.T) {
	var healthBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "WALinuxAgent", request.Header.Get("x-ms-agent-name"))
		assert.Equal(t, "2012-11-30", request.Header.Get("x-ms-version"))
		switch request.URL.RequestURI() {
		case "/machine/?comp=goalstate":
			assert.Equal(t, http.MethodGet, request.Method)
			_, _ = io.WriteString(writer, testGoalState)
		case "/machine?comp=health":
			assert.Equal(t, http.MethodPost, request.Method)
			assert.Equal(t, "text/xml; charset=utf-8", request.Header.Get("Content-Type"))
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			healthBody = string(body)
			writer.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	reporter := newTestProvisioningReporter(t, strings.TrimPrefix(server.URL, "http://"))
	err := reporter.report(context.Background(), reportReadyOptions{
		ready:      true,
		retries:    1,
		retryDelay: 0,
	})
	require.NoError(t, err)
	assert.Contains(t, healthBody, "<State>Ready</State>")
	assert.NotContains(t, healthBody, "<Details>")

	records, err := os.ReadFile(reporter.kvpPath)
	require.NoError(t, err)
	require.Len(t, records, 2*(kvpKeySize+kvpValueSize))
	assert.Equal(t, "dmesg", nullTerminatedString(records[:kvpKeySize]))
	assert.Equal(t, "kernel output", nullTerminatedString(records[kvpKeySize:kvpKeySize+kvpValueSize]))

	reportOffset := kvpKeySize + kvpValueSize
	assert.Equal(t, "PROVISIONING_REPORT", nullTerminatedString(records[reportOffset:reportOffset+kvpKeySize]))
	report := nullTerminatedString(records[reportOffset+kvpKeySize:])
	assert.Equal(
		t,
		"result=success|agent=AKS-CSE|timestamp=2026-08-21T07:39:36.123456789+00:00|vm_id=abc-123",
		report,
	)
}

func TestProvisioningReporterReportsFailureAndRetries(t *testing.T) {
	var attempts atomic.Int32
	var healthBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() == "/machine/?comp=goalstate" {
			if attempts.Add(1) == 1 {
				http.Error(writer, "retry", http.StatusInternalServerError)
				return
			}
			_, _ = io.WriteString(writer, testGoalState)
			return
		}
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		healthBody = string(body)
	}))
	defer server.Close()

	reporter := newTestProvisioningReporter(t, strings.TrimPrefix(server.URL, "http://"))
	err := reporter.report(context.Background(), reportReadyOptions{
		ready:       false,
		description: "failed | it's bad <now>",
		retries:     2,
		retryDelay:  0,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), attempts.Load())
	assert.Contains(t, healthBody, "<State>NotReady</State>")
	assert.Contains(t, healthBody, "<SubStatus>ProvisioningFailed</SubStatus>")
	assert.Contains(t, healthBody, "failed | it&#39;s bad &lt;now&gt;")

	records, err := os.ReadFile(reporter.kvpPath)
	require.NoError(t, err)
	reportOffset := kvpKeySize + kvpValueSize
	report := nullTerminatedString(records[reportOffset+kvpKeySize:])
	assert.Contains(t, report, "result=failure")
	assert.Contains(t, report, "'description=failed | it''s bad <now>'")
}

func TestProvisioningReporterReturnsFinalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	reporter := newTestProvisioningReporter(t, strings.TrimPrefix(server.URL, "http://"))
	err := reporter.report(context.Background(), reportReadyOptions{
		ready:      true,
		retries:    2,
		retryDelay: 0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to report provisioning status")
	assert.Contains(t, err.Error(), "503 Service Unavailable")
}

func TestProvisioningReporterIgnoresKVPAndDmesgFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet {
			_, _ = io.WriteString(writer, testGoalState)
		}
	}))
	defer server.Close()

	reporter := newTestProvisioningReporter(t, strings.TrimPrefix(server.URL, "http://"))
	reporter.kvpPath = filepath.Join(t.TempDir(), "missing", "kvp")
	reporter.runDmesg = func(context.Context) ([]byte, error) {
		return nil, errors.New("dmesg unavailable")
	}
	require.NoError(t, reporter.report(context.Background(), reportReadyOptions{
		ready:      true,
		retries:    1,
		retryDelay: 0,
	}))
}

func TestHealthDocumentTruncatesDescription(t *testing.T) {
	state := wireGoalState{Incarnation: "1"}
	state.Container.ID = "container"
	state.Container.RoleInstanceList.RoleInstance.ID = "instance"
	document := string(healthDocument(state, false, strings.Repeat("x", descriptionMaxLength+10)))
	assert.Contains(t, document, "<Description>"+strings.Repeat("x", descriptionMaxLength)+"</Description>")
	assert.NotContains(t, document, strings.Repeat("x", descriptionMaxLength+1))
}

func nullTerminatedString(value []byte) string {
	if index := strings.IndexByte(string(value), 0); index >= 0 {
		return string(value[:index])
	}
	return string(value)
}
