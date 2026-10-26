package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testGoalState = `<GoalState><Incarnation>7</Incarnation><Container>
<ContainerId>container&amp;id</ContainerId><RoleInstanceList><RoleInstance>
<InstanceId>instance&lt;id</InstanceId>
</RoleInstance></RoleInstanceList></Container></GoalState>`

func TestSendHealthReport(t *testing.T) {
	for _, tc := range []struct {
		name        string
		failed      bool
		description string
		statusCode  int
	}{
		{name: "ready", statusCode: http.StatusOK},
		{name: "created", statusCode: http.StatusCreated},
		{name: "accepted", statusCode: http.StatusAccepted},
		{name: "failure", failed: true, description: `CSE failed: <bad> & "quoted"`, statusCode: http.StatusOK},
		{name: "empty failure", failed: true, statusCode: http.StatusOK},
		{name: "long failure", failed: true, description: strings.Repeat("\u00e9", 600), statusCode: http.StatusOK},
		{name: "oversized failure", failed: true, description: strings.Repeat("x", 5000), statusCode: http.StatusOK},
		{name: "XML-escaped failure", failed: true, description: strings.Repeat("<&'\"\u00e9", 1000), statusCode: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var posted atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "WALinuxAgent", r.Header.Get("x-ms-agent-name"))
				assert.Equal(t, "2012-11-30", r.Header.Get("x-ms-version"))
				if r.Method == http.MethodGet {
					assert.Equal(t, "/machine/?comp=goalstate", r.URL.RequestURI())
					_, _ = io.WriteString(w, testGoalState)
					return
				}
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/machine?comp=health", r.URL.RequestURI())
				assert.Equal(t, "text/xml; charset=utf-8", r.Header.Get("Content-Type"))
				raw, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.True(t, bytes.HasPrefix(raw, []byte(xml.Header)))
				assert.Contains(t, string(raw), "<Container><ContainerId>container&amp;id</ContainerId><RoleInstanceList><Role><InstanceId>instance&lt;id</InstanceId><Health><State>")
				var report healthReport
				assert.NoError(t, xml.Unmarshal(raw, &report))
				assert.Equal(t, "7", report.Incarnation)
				assert.Equal(t, "container&id", report.ContainerID)
				assert.Equal(t, "instance<id", report.InstanceID)
				if tc.failed {
					assert.Equal(t, "NotReady", report.State)
					if assert.NotNil(t, report.Details) {
						assert.Equal(t, "ProvisioningFailed", report.Details.SubStatus)
						assert.True(t, strings.HasPrefix(tc.description, report.Details.Description))
						var escaped bytes.Buffer
						assert.NoError(t, xml.EscapeText(&escaped, []byte(report.Details.Description)))
						assert.LessOrEqual(t, escaped.Len(), 4096)
						assert.Equal(t, trimHealthReportDescription(tc.description), report.Details.Description)
					}
				} else {
					assert.Equal(t, "Ready", report.State)
					assert.Nil(t, report.Details)
				}
				posted.Add(1)
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			err := sendHealthReport(t.Context(), server.Client(), server.URL, tc.failed, tc.description, 0)

			require.NoError(t, err)
			require.EqualValues(t, 1, posted.Load())
		})
	}
}

func TestSendHealthReportRefetchesGoalState(t *testing.T) {
	var gets, posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			incarnation := gets.Add(1)
			_, _ = io.WriteString(w, strings.Replace(testGoalState, "<Incarnation>7", fmt.Sprintf("<Incarnation>%d", incarnation), 1))
			return
		}
		attempt := posts.Add(1)
		var report healthReport
		assert.NoError(t, xml.NewDecoder(r.Body).Decode(&report))
		assert.Equal(t, fmt.Sprint(attempt), report.Incarnation)
		if attempt == 1 {
			http.Error(w, "retry", http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()

	require.NoError(t, sendHealthReport(t.Context(), server.Client(), server.URL, false, "", 0))
	require.EqualValues(t, 2, gets.Load())
	require.EqualValues(t, 2, posts.Load())
}

func TestSendHealthReportFailures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		goalState  string
		getStatus  int
		postStatus int
		want       string
		wantPosts  int32
	}{
		{name: "GET error", getStatus: 500, want: "500", goalState: testGoalState},
		{name: "GET requires 200", getStatus: 201, want: "201", goalState: testGoalState},
		{name: "malformed XML", getStatus: 200, goalState: "<GoalState>", want: "parse GoalState"},
		{name: "missing fields", getStatus: 200, goalState: "<GoalState/>", want: "missing"},
		{name: "POST error", getStatus: 200, postStatus: 500, goalState: testGoalState, want: "500", wantPosts: 3},
		{name: "POST rejects 204", getStatus: 200, postStatus: 204, goalState: testGoalState, want: "204", wantPosts: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gets, posts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					gets.Add(1)
					w.WriteHeader(tc.getStatus)
					_, _ = io.WriteString(w, tc.goalState)
					return
				}
				posts.Add(1)
				w.WriteHeader(tc.postStatus)
			}))
			defer server.Close()

			err := sendHealthReport(t.Context(), server.Client(), server.URL, false, "", 0)

			require.ErrorContains(t, err, "after 3 attempts")
			require.ErrorContains(t, err, tc.want)
			require.EqualValues(t, 3, gets.Load())
			require.Equal(t, tc.wantPosts, posts.Load())
		})
	}
}

func TestSendHealthReportCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, "retry", http.StatusServiceUnavailable)
		cancel()
	}))
	defer server.Close()

	require.ErrorIs(t, sendHealthReport(ctx, server.Client(), server.URL, false, "", time.Hour), context.Canceled)
	require.EqualValues(t, 1, attempts.Load())
	require.ErrorIs(t, sendHealthReport(ctx, server.Client(), server.URL, false, "", 0), context.Canceled)
	require.EqualValues(t, 1, attempts.Load())
}

func TestSendHealthReportConnectionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()

	err := sendHealthReport(t.Context(), server.Client(), server.URL, false, "", 0)

	require.ErrorContains(t, err, "failed to report Ready after 3 attempts")
	require.ErrorContains(t, err, "connect")
}

func TestEncodeProvisioningReport(t *testing.T) {
	require.Equal(t, "result=success|agent=AKS-CSE|vm_id=test",
		encodeProvisioningReport([]string{"result=success", "agent=AKS-CSE", "vm_id=test"}))
	require.Equal(t, "result=failure|'description=can''t|report\r\nthis'",
		encodeProvisioningReport([]string{"result=failure", "description=can't|report\r\nthis"}))
}

func TestReportValueLimits(t *testing.T) {
	require.Equal(t, "short", trimReportDescription("short"))
	require.Equal(t, strings.Repeat("\u00e9", 512), trimReportDescription(strings.Repeat("\u00e9", 600)))
	for _, tc := range []struct {
		value string
		limit int
		want  string
	}{
		{value: "short", limit: 1023, want: "short"},
		{value: strings.Repeat("x", 1024), limit: 1023, want: strings.Repeat("x", 1023)},
		{value: strings.Repeat("\u00e9", 600), limit: 1023, want: strings.Repeat("\u00e9", 511)},
		{value: "\xffabc", limit: 1024, want: "\uFFFDabc"},
	} {
		got := truncateKVPValue(tc.value, tc.limit)
		require.Equal(t, tc.want, got)
		require.True(t, utf8.ValidString(got))
		require.LessOrEqual(t, len(got), tc.limit)
	}
}

func TestTrimHealthReportDescription(t *testing.T) {
	require.Equal(t, strings.Repeat("\u00e9", 600), trimHealthReportDescription(strings.Repeat("\u00e9", 600)))
	require.Equal(t, strings.Repeat("x", 4096), trimHealthReportDescription(strings.Repeat("x", 4096)))
	for _, input := range []string{
		strings.Repeat("x", 4097),
		strings.Repeat("\u00e9", 2049),
		strings.Repeat("<&'\"\n", 1000),
	} {
		got := trimHealthReportDescription(input)
		require.True(t, utf8.ValidString(got))
		require.True(t, strings.HasPrefix(input, got))
		var escaped bytes.Buffer
		require.NoError(t, xml.EscapeText(&escaped, []byte(got)))
		require.LessOrEqual(t, escaped.Len(), 4096)
		remaining := []rune(strings.TrimPrefix(input, got))
		require.NotEmpty(t, remaining)
		escaped.Reset()
		require.NoError(t, xml.EscapeText(&escaped, []byte(got+string(remaining[0]))))
		require.Greater(t, escaped.Len(), 4096, "must retain the largest safe prefix")
	}
}

func TestWriteKVPRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kvp")
	require.NoError(t, writeKVPRecord(path, "dmesg", "kernel output"))
	require.NoError(t, writeKVPRecord(path, "PROVISIONING_REPORT", "result=success"))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Len(t, data, 2*(512+2048))
	for i, expected := range [][2]string{{"dmesg", "kernel output"}, {"PROVISIONING_REPORT", "result=success"}} {
		offset := i * (512 + 2048)
		require.Equal(t, expected[0], string(bytes.TrimRight(data[offset:offset+512], "\x00")))
		require.Equal(t, expected[1], string(bytes.TrimRight(data[offset+512:offset+512+2048], "\x00")))
	}
	require.Error(t, writeKVPRecord(filepath.Join(t.TempDir(), "missing", "kvp"), "key", "value"))
}
