package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func TestRunUsesOpenLoopArrivals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		time.Sleep(80 * time.Millisecond)
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config{
		target:         server.URL,
		rate:           50,
		duration:       100 * time.Millisecond,
		requestTimeout: time.Second,
		maxOutstanding: 100,
	}
	arrivals, _, err := buildSchedule(cfg)
	if err != nil {
		t.Fatal(err)
	}
	results := run(context.Background(), server.Client(), cfg, []byte(`{}`), arrivals)

	if len(results) != 5 {
		t.Fatalf("expected 5 offered requests, got %d", len(results))
	}
	for index, item := range results {
		if item.Sequence != int64(index) {
			t.Fatalf("expected sequence %d, got %d", index, item.Sequence)
		}
	}
	if results[4].StartedAt.Sub(results[0].StartedAt) >= 200*time.Millisecond {
		t.Fatalf("requests were serialized: first=%s last=%s", results[0].StartedAt, results[4].StartedAt)
	}
	for _, item := range results {
		if item.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d (%s)", item.StatusCode, item.Error)
		}
	}
}

func TestRunRecordsDroppedArrivals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config{
		target:         server.URL,
		rate:           100,
		duration:       50 * time.Millisecond,
		requestTimeout: time.Second,
		maxOutstanding: 1,
	}
	arrivals, _, err := buildSchedule(cfg)
	if err != nil {
		t.Fatal(err)
	}
	report := summarize(run(context.Background(), server.Client(), cfg, []byte(`{}`), arrivals))

	if report.Offered != 5 || report.Dropped == 0 {
		t.Fatalf("expected five offered requests and client drops, got %+v", report)
	}
}

func TestBuildScheduleSupportsRatePhasesAndConcurrencyWaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schedule.json")
	data := []byte(`{"phases":[{"name":"baseline","duration":"100ms","rps":20},{"name":"wave","duration":"1s","concurrency":3,"repeat":2}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	arrivals, duration, err := buildSchedule(config{schedulePath: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(arrivals) != 8 {
		t.Fatalf("expected 8 arrivals, got %d", len(arrivals))
	}
	if duration != 2100*time.Millisecond {
		t.Fatalf("expected 2.1s schedule, got %s", duration)
	}
	if arrivals[2].Phase != "wave" || arrivals[2].Offset != 100*time.Millisecond || arrivals[4].Offset != 100*time.Millisecond {
		t.Fatalf("first concurrency wave is not synchronized: %+v", arrivals[2:5])
	}
	if arrivals[5].Offset != 1100*time.Millisecond || arrivals[7].Offset != 1100*time.Millisecond {
		t.Fatalf("repeated concurrency wave is not synchronized: %+v", arrivals[5:])
	}
}

func TestSummarize(t *testing.T) {
	now := time.Now()
	results := []result{
		{StartedAt: now, CompletedAt: now.Add(time.Millisecond), StatusCode: http.StatusOK, ServiceLatencyMS: 1, EndToEndLatencyMS: 2, ScheduleDelayMS: 1, ResponseBytes: 10},
		{StartedAt: now, CompletedAt: now.Add(3 * time.Millisecond), StatusCode: http.StatusServiceUnavailable, RetryAfter: "3", ServiceLatencyMS: 3, EndToEndLatencyMS: 4, ScheduleDelayMS: 1},
		{StartedAt: now, CompletedAt: now.Add(5 * time.Millisecond), Error: "context deadline exceeded", ErrorKind: "timeout", ServiceLatencyMS: 5, EndToEndLatencyMS: 6, ScheduleDelayMS: 1},
		{Dropped: true},
	}

	report := summarize(results)
	if report.Offered != 4 || report.Started != 3 || report.Completed != 3 || report.Dropped != 1 {
		t.Fatalf("unexpected counts: %+v", report)
	}
	if report.HTTPResponses != 2 || report.Successful != 1 || report.Overloaded != 1 {
		t.Fatalf("unexpected outcome counts: %+v", report)
	}
	if report.StatusCodes[http.StatusOK] != 1 || report.StatusCodes[http.StatusServiceUnavailable] != 1 {
		t.Fatalf("unexpected status counts: %+v", report.StatusCodes)
	}
	if report.Timeouts != 1 || report.TransportErrors != 0 {
		t.Fatalf("unexpected error counts: timeouts=%d transport=%d", report.Timeouts, report.TransportErrors)
	}
	if report.ServiceLatency.P50 != 3 || report.ServiceLatency.P99 != 5 {
		t.Fatalf("unexpected service latency percentiles: %+v", report.ServiceLatency)
	}
	if report.SuccessLatency.P50 != 1 || report.SuccessLatency.P99 != 1 {
		t.Fatalf("unexpected success latency percentiles: %+v", report.SuccessLatency)
	}
}

func TestValidateConfig(t *testing.T) {
	valid := config{target: "http://localhost", outputPath: "results.jsonl", rate: 1, duration: time.Second, requestTimeout: time.Second, maxOutstanding: 1}
	if err := validateConfig(valid); err != nil {
		t.Fatalf("valid config failed: %v", err)
	}
	valid.rate = 0
	if err := validateConfig(valid); err == nil {
		t.Fatal("expected zero rate to fail validation")
	}
}

func TestSyntheticLinuxFixtureGeneratesBootstrapData(t *testing.T) {
	body, err := syntheticLinuxFixture()
	if err != nil {
		t.Fatal(err)
	}
	var configuration datamodel.NodeBootstrappingConfiguration
	if err := json.Unmarshal(body, &configuration); err != nil {
		t.Fatal(err)
	}
	if configuration.ContainerService == nil || configuration.AgentPoolProfile == nil || configuration.CloudSpecConfig == nil {
		t.Fatal("synthetic fixture is missing required bootstrap configuration")
	}
	if configuration.ContainerService.Properties.ServicePrincipalProfile.Secret != "synthetic-not-a-secret" {
		t.Fatal("synthetic fixture contains an unexpected credential value")
	}
	baker, err := agent.NewAgentBaker()
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, err := baker.GetNodeBootstrapping(context.Background(), &configuration)
	if err != nil {
		t.Fatal(err)
	}
	if bootstrap.CustomData == "" || bootstrap.CSE == "" {
		t.Fatal("synthetic fixture produced empty bootstrap data")
	}
}
