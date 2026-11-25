package apiserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestRequestLoggingWritesStartAndEndRecords(t *testing.T) {
	api, err := NewAPIServer(&Options{Addr: ":8080"})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	api.requestLogger = NewJSONLogger(&buf, incomingRequestTraceSource)

	request := httptest.NewRequest(http.MethodPost, RoutePathLatestSIGImageConfig, strings.NewReader("not json"))
	request.Header.Set(operationIDHeader, "op-id")
	request.Header.Set(correlationIDHeader, "corr-id")
	response := httptest.NewRecorder()
	api.NewRouter().ServeHTTP(response, request)

	records := decodeRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d: %s", len(records), buf.String())
	}
	for _, record := range records {
		for key, want := range map[string]string{
			"source":        incomingRequestTraceSource,
			"operationID":   "op-id",
			"correlationID": "corr-id",
			"httpMethod":    http.MethodPost,
			"routeName":     "GetLatestSigImageConfig",
		} {
			if record[key] != want {
				t.Errorf("record %v: expected %s=%q, got %v", record["msg"], key, want, record[key])
			}
		}
	}
	if records[0]["msg"] != "AgentBakerHttpRequestStart" || records[1]["msg"] != "AgentBakerHttpRequestEnd" {
		t.Fatalf("unexpected messages: %v, %v", records[0]["msg"], records[1]["msg"])
	}
	end := records[1]
	if end["StatusCode"] != "400" {
		t.Errorf("expected StatusCode 400, got %v", end["StatusCode"])
	}
	if want := strconv.Itoa(response.Body.Len()); end["contentLength"] != want {
		t.Errorf("expected contentLength %s, got %v", want, end["contentLength"])
	}
	if _, ok := end["durationInMilliseconds"].(string); !ok {
		t.Errorf("expected durationInMilliseconds string, got %v", end["durationInMilliseconds"])
	}
}

func TestRequestLoggingSkipsHealthz(t *testing.T) {
	api, err := NewAPIServer(&Options{Addr: ":8080"})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	api.requestLogger = NewJSONLogger(&buf, incomingRequestTraceSource)

	response := httptest.NewRecorder()
	api.NewRouter().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no records, got %s", buf.String())
	}
}

func decodeRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	decoder := json.NewDecoder(buf)
	for decoder.More() {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	return records
}
