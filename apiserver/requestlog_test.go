package apiserver

import (
	"bytes"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestRequestLoggingWritesRequestTrace(t *testing.T) {
	var buf bytes.Buffer
	defaultBuf := captureDefaultLogger(t)

	api, err := NewAPIServer(&Options{Addr: ":8080", RequestLogger: NewJSONLogger(&buf, IncomingRequestTraceSource)})
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, RoutePathLatestSIGImageConfig+"?sig=secret", strings.NewReader("not json"))
	setRequestIDHeaders(request)
	response := httptest.NewRecorder()
	api.NewRouter().ServeHTTP(response, request)

	records := decodeRecords(t, &buf)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d: %s", len(records), buf.String())
	}
	trace := records[0]
	assertFields(t, trace, wantRequestIDs)
	assertFields(t, trace, map[string]string{
		"source":        IncomingRequestTraceSource,
		"msg":           "AgentBakerHttpRequest",
		"httpMethod":    http.MethodPost,
		"targetURI":     RoutePathLatestSIGImageConfig,
		"routeName":     "GetLatestSigImageConfig",
		"operationName": "GetLatestSigImageConfig",
		"StatusCode":    "400",
		"contentLength": strconv.Itoa(response.Body.Len()),
	})
	if _, ok := trace["durationInMilliseconds"].(string); !ok {
		t.Errorf("expected durationInMilliseconds string, got %v", trace["durationInMilliseconds"])
	}

	handlerRecords := decodeRecords(t, defaultBuf)
	if len(handlerRecords) != 1 {
		t.Fatalf("expected 1 handler record, got %d: %s", len(handlerRecords), defaultBuf.String())
	}
	assertFields(t, handlerRecords[0], wantRequestIDs)
	assertFields(t, handlerRecords[0], map[string]string{
		"source": ContextlessTraceSource,
		"level":  "ERROR",
		"msg":    "decode request body",
	})
}

func TestRecoveryHandlerLogsPanicWithRequestIDs(t *testing.T) {
	defaultBuf := captureDefaultLogger(t)

	request := httptest.NewRequest(http.MethodPost, RoutePathNodeBootstrapData, nil)
	setRequestIDHeaders(request)
	response := httptest.NewRecorder()
	recoveryHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, response.Code)
	}
	records := decodeRecords(t, defaultBuf)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d: %s", len(records), defaultBuf.String())
	}
	assertFields(t, records[0], wantRequestIDs)
	assertFields(t, records[0], map[string]string{
		"level": "ERROR",
		"msg":   "recovered panic",
		"panic": "boom",
	})
	if stack, _ := records[0]["stack"].(string); !strings.Contains(stack, "TestRecoveryHandlerLogsPanicWithRequestIDs") {
		t.Errorf("expected stack to contain the panicking test, got %q", stack)
	}
}

var wantRequestIDs = map[string]string{
	"operationID":     "op-id",
	"correlationID":   "corr-id",
	"clientRequestID": "client-id",
}

func setRequestIDHeaders(r *http.Request) {
	r.Header.Set(operationIDHeader, wantRequestIDs["operationID"])
	r.Header.Set(correlationIDHeader, wantRequestIDs["correlationID"])
	r.Header.Set(clientRequestIDHeader, wantRequestIDs["clientRequestID"])
}

// captureDefaultLogger redirects the default slog logger to a buffer. slog.SetDefault also
// redirects the log package, and restoring the previous default does not undo that.
func captureDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous, previousWriter, previousFlags := slog.Default(), log.Writer(), log.Flags()
	slog.SetDefault(NewJSONLogger(&buf, ContextlessTraceSource))
	t.Cleanup(func() {
		slog.SetDefault(previous)
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})
	return &buf
}

func assertFields(t *testing.T, record map[string]any, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if record[key] != value {
			t.Errorf("record %v: expected %s=%q, got %v", record["msg"], key, value, record[key])
		}
	}
}

func TestRequestLoggingSkipsNonAPIRequests(t *testing.T) {
	var buf bytes.Buffer
	api, err := NewAPIServer(&Options{Addr: ":8080", RequestLogger: NewJSONLogger(&buf, IncomingRequestTraceSource)})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method, path string
		wantStatus   int
	}{
		{http.MethodGet, "/healthz", http.StatusOK},
		{http.MethodPost, "/unknown", http.StatusNotFound},
		{http.MethodGet, RoutePathNodeBootstrapData, http.StatusMethodNotAllowed},
	} {
		response := httptest.NewRecorder()
		api.NewRouter().ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
		if response.Code != tc.wantStatus {
			t.Errorf("%s %s: expected status %d, got %d", tc.method, tc.path, tc.wantStatus, response.Code)
		}
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
