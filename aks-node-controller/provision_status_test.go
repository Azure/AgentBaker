package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvisionFailureDescription(t *testing.T) {
	for _, tc := range []struct {
		name     string
		output   string
		metadata string
	}{
		{name: "complete", output: "configureHTTPProxyCA\nbase64: invalid input\nexit 161"},
		{name: "long", output: strings.Repeat("older log line\n", 1000) + "base64: invalid input\nexit 161"},
		{name: "XML and Unicode", output: strings.Repeat("<&'\"\u00e9\n", 1000) + "base64: invalid input\nexit 161"},
		{name: "oversized metadata", output: strings.Repeat("log\n", 1000) + "exit 161", metadata: strings.Repeat("metadata", 1000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status := map[string]any{
				"ExitCode": "161", "Output": tc.output, "Error": "",
				"ExecDuration": "4", "SystemdSummary": tc.metadata,
				"BootDatapoints": map[string]string{"KubeletStartTime": "n/a"},
			}
			raw, err := json.Marshal(status)
			require.NoError(t, err)
			result := &ProvisionResult{
				ExitCode: "161", Error: "exit status 161",
				Output: "command output before status\n" + string(raw) + "\ndate: invalid date 'n/a'",
			}

			description, err := provisionFailureDescription(result)

			require.NoError(t, err)
			require.True(t, utf8.ValidString(description))
			var escaped bytes.Buffer
			require.NoError(t, xml.EscapeText(&escaped, []byte(description)))
			require.LessOrEqual(t, escaped.Len(), 4096)
			var reported map[string]any
			require.NoError(t, json.Unmarshal([]byte(description), &reported))
			require.Equal(t, "161", reported["ExitCode"])
			output, ok := reported["Output"].(string)
			require.True(t, ok)
			if tc.name == "complete" {
				require.JSONEq(t, string(raw), description)
			} else {
				require.Equal(t, true, reported["OutputTruncated"])
				require.True(t, strings.HasPrefix(output, truncatedProvisionOutput))
				require.True(t, strings.HasSuffix(tc.output, strings.TrimPrefix(output, truncatedProvisionOutput)))
				require.True(t, strings.HasSuffix(output, "exit 161"))
			}
			if tc.metadata == "" {
				require.Equal(t, "4", reported["ExecDuration"])
				require.Equal(t, map[string]any{"KubeletStartTime": "n/a"}, reported["BootDatapoints"])
			} else {
				require.Equal(t, true, reported["DetailsTruncated"])
			}
		})
	}
}

func TestProvisionFailureDescriptionFallback(t *testing.T) {
	for _, output := range []string{
		"",
		"script failed before emitting JSON",
		`{"ExitCode":"0","Error":"","Output":"stale success"}`,
		`{"ExitCode":"161","Error":"","Output":123}`,
		`{"ExitCode":"161","Error":"","Output":"incomplete"`,
	} {
		result := &ProvisionResult{ExitCode: "161", Error: "exit status 161", Output: output}
		description, err := provisionFailureDescription(result)
		require.NoError(t, err)
		var reported ProvisionResult
		require.NoError(t, json.Unmarshal([]byte(description), &reported))
		require.Equal(t, *result, reported)
	}
}

func TestAppProvisionReportsCSEJSONToWireServer(t *testing.T) {
	output := strings.Repeat("old log line <&'\"\n", 1000) + "base64: invalid input\nexit 161"
	raw, err := json.Marshal(map[string]any{
		"ExitCode": "161", "Error": "", "Output": output, "ExecDuration": "4",
	})
	require.NoError(t, err)
	descriptions := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, testGoalState)
			return
		}
		var report healthReport
		assert.NoError(t, xml.NewDecoder(r.Body).Decode(&report))
		assert.Equal(t, "NotReady", report.State)
		if assert.NotNil(t, report.Details) {
			descriptions <- report.Details.Description
		}
	}))
	defer server.Close()
	tt := NewTestApp(t, TestAppConfig{RunFunc: cmdRunner})
	tt.App.goalStateReporter = func(ctx context.Context, failed bool, description string) error {
		return sendHealthReport(ctx, server.Client(), server.URL, failed, description, 0)
	}
	require.NoError(t, os.WriteFile(tt.App.readyReportMarkerPath, nil, 0o600))
	path := filepath.Join(t.TempDir(), "nbc-cmd.sh")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/bash\ncat <<'EOF'\n"+string(raw)+"\nEOF\nexit 161\n"), 0o600))

	_, err = tt.App.runProvision(t.Context(), ProvisionFlags{NBCCmd: path}, false)

	require.ErrorContains(t, err, "exit status 161")
	require.Len(t, descriptions, 1)
	var reported ProvisionResult
	require.NoError(t, json.Unmarshal([]byte(<-descriptions), &reported))
	require.Equal(t, "161", reported.ExitCode)
	require.True(t, strings.HasSuffix(reported.Output, "base64: invalid input\nexit 161"))
}
