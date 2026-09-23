package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const defaultRequestTimeout = 3 * time.Second

type config struct {
	target         string
	bodyPath       string
	outputPath     string
	schedulePath   string
	rate           float64
	duration       time.Duration
	requestTimeout time.Duration
	maxOutstanding int
}

type scheduleFile struct {
	Phases []phase `json:"phases"`
}

type phase struct {
	Name        string  `json:"name"`
	Duration    string  `json:"duration"`
	RPS         float64 `json:"rps,omitempty"`
	Concurrency int     `json:"concurrency,omitempty"`
	Repeat      int     `json:"repeat,omitempty"`
}

type arrival struct {
	Sequence int64
	Phase    string
	Offset   time.Duration
}

type result struct {
	Sequence          int64     `json:"sequence"`
	Phase             string    `json:"phase,omitempty"`
	ScheduledAt       time.Time `json:"scheduledAt"`
	StartedAt         time.Time `json:"startedAt,omitempty"`
	CompletedAt       time.Time `json:"completedAt,omitempty"`
	ScheduleDelayMS   float64   `json:"scheduleDelayMs,omitempty"`
	ServiceLatencyMS  float64   `json:"serviceLatencyMs,omitempty"`
	EndToEndLatencyMS float64   `json:"endToEndLatencyMs,omitempty"`
	StatusCode        int       `json:"statusCode,omitempty"`
	RetryAfter        string    `json:"retryAfter,omitempty"`
	ResponseBytes     int64     `json:"responseBytes,omitempty"`
	Outstanding       int64     `json:"outstanding,omitempty"`
	Error             string    `json:"error,omitempty"`
	ErrorKind         string    `json:"errorKind,omitempty"`
	Dropped           bool      `json:"dropped,omitempty"`
}

type summary struct {
	Offered         int64         `json:"offered"`
	Started         int64         `json:"started"`
	Completed       int64         `json:"completed"`
	HTTPResponses   int64         `json:"httpResponses"`
	Successful      int64         `json:"successful"`
	Overloaded      int64         `json:"overloaded"`
	Dropped         int64         `json:"dropped"`
	Timeouts        int64         `json:"timeouts"`
	TransportErrors int64         `json:"transportErrors"`
	ResponseBytes   int64         `json:"responseBytes"`
	MaxOutstanding  int64         `json:"maxOutstanding"`
	StatusCodes     map[int]int64 `json:"statusCodes"`
	ServiceLatency  percentiles   `json:"serviceLatencyMs"`
	SuccessLatency  percentiles   `json:"successLatencyMs"`
	EndToEndLatency percentiles   `json:"endToEndLatencyMs"`
	ScheduleDelay   percentiles   `json:"scheduleDelayMs"`
	ElapsedSeconds  float64       `json:"elapsedSeconds"`
	OfferedRPS      float64       `json:"offeredRps"`
	CompletedRPS    float64       `json:"completedRps"`
	SuccessfulRPS   float64       `json:"successfulRps"`
}

type percentiles struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
	Max float64 `json:"max"`
}

func main() {
	cfg := parseFlags()
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	body, err := requestBody(cfg.bodyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare request body: %v\n", err)
		os.Exit(1)
	}
	if !json.Valid(body) {
		fmt.Fprintln(os.Stderr, "request body must be valid JSON")
		os.Exit(2)
	}
	arrivals, scheduleDuration, err := buildSchedule(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare schedule: %v\n", err)
		os.Exit(2)
	}

	output, err := os.Create(cfg.outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create output: %v\n", err)
		os.Exit(1)
	}
	defer output.Close()

	results := run(context.Background(), http.DefaultClient, cfg, body, arrivals)
	encoder := json.NewEncoder(output)
	for _, item := range results {
		if err := encoder.Encode(item); err != nil {
			fmt.Fprintf(os.Stderr, "write result: %v\n", err)
			os.Exit(1)
		}
	}

	report := summarize(results)
	report.ElapsedSeconds = scheduleDuration.Seconds()
	if report.ElapsedSeconds > 0 {
		report.OfferedRPS = float64(report.Offered) / report.ElapsedSeconds
		report.CompletedRPS = float64(report.Completed) / report.ElapsedSeconds
		report.SuccessfulRPS = float64(report.Successful) / report.ElapsedSeconds
	}
	formatted, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "format summary: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(formatted))
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.target, "target", "http://127.0.0.1:8080/getnodebootstrapdata", "AgentBakerSvc bootstrap endpoint")
	flag.StringVar(&cfg.bodyPath, "body", "", "optional path to a sanitized NodeBootstrappingConfiguration JSON file")
	flag.StringVar(&cfg.outputPath, "output", "agentbakersvc-load.jsonl", "per-request JSONL output path")
	flag.StringVar(&cfg.schedulePath, "schedule", "", "optional JSON phase schedule; overrides rps and duration")
	flag.Float64Var(&cfg.rate, "rps", 10, "open-loop request rate")
	flag.DurationVar(&cfg.duration, "duration", time.Minute, "test duration")
	flag.DurationVar(&cfg.requestTimeout, "timeout", defaultRequestTimeout, "per-request timeout")
	flag.IntVar(&cfg.maxOutstanding, "max-outstanding", 10000, "client safety limit; arrivals beyond it are recorded as dropped")
	flag.Parse()
	return cfg
}

func validateConfig(cfg config) error {
	switch {
	case cfg.target == "":
		return errors.New("target must not be empty")
	case cfg.outputPath == "":
		return errors.New("output must not be empty")
	case cfg.rate <= 0:
		return errors.New("rps must be greater than zero")
	case cfg.duration <= 0:
		return errors.New("duration must be greater than zero")
	case cfg.requestTimeout <= 0:
		return errors.New("timeout must be greater than zero")
	case cfg.maxOutstanding <= 0:
		return errors.New("max-outstanding must be greater than zero")
	default:
		return nil
	}
}

func requestBody(path string) ([]byte, error) {
	if path == "" {
		return syntheticLinuxFixture()
	}
	return os.ReadFile(path)
}

func buildSchedule(cfg config) ([]arrival, time.Duration, error) {
	if cfg.schedulePath == "" {
		arrivals := appendRateArrivals(nil, "steady", cfg.rate, cfg.duration, 0)
		assignSequences(arrivals)
		return arrivals, cfg.duration, nil
	}

	data, err := os.ReadFile(cfg.schedulePath)
	if err != nil {
		return nil, 0, err
	}
	var specification scheduleFile
	if err := json.Unmarshal(data, &specification); err != nil {
		return nil, 0, err
	}
	if len(specification.Phases) == 0 {
		return nil, 0, errors.New("schedule must contain at least one phase")
	}

	arrivals := make([]arrival, 0)
	var elapsed time.Duration
	for _, item := range specification.Phases {
		duration, err := time.ParseDuration(item.Duration)
		if err != nil || duration <= 0 {
			return nil, 0, fmt.Errorf("phase %q duration must be positive: %q", item.Name, item.Duration)
		}
		if (item.RPS > 0) == (item.Concurrency > 0) {
			return nil, 0, fmt.Errorf("phase %q must set exactly one of rps or concurrency", item.Name)
		}
		repeat := item.Repeat
		if repeat == 0 {
			repeat = 1
		}
		if repeat < 0 {
			return nil, 0, fmt.Errorf("phase %q repeat must not be negative", item.Name)
		}
		for repetition := 0; repetition < repeat; repetition++ {
			if item.RPS > 0 {
				arrivals = appendRateArrivals(arrivals, item.Name, item.RPS, duration, elapsed)
			} else {
				for request := 0; request < item.Concurrency; request++ {
					arrivals = append(arrivals, arrival{Phase: item.Name, Offset: elapsed})
				}
			}
			elapsed += duration
		}
	}
	assignSequences(arrivals)
	return arrivals, elapsed, nil
}

func assignSequences(arrivals []arrival) {
	for index := range arrivals {
		arrivals[index].Sequence = int64(index)
	}
}

func appendRateArrivals(arrivals []arrival, name string, rate float64, duration, offset time.Duration) []arrival {
	interval := time.Duration(float64(time.Second) / rate)
	requestCount := int(duration / interval)
	for request := 0; request < requestCount; request++ {
		arrivals = append(arrivals, arrival{Phase: name, Offset: offset + time.Duration(request)*interval})
	}
	return arrivals
}

func run(ctx context.Context, client *http.Client, cfg config, body []byte, arrivals []arrival) []result {
	start := time.Now().Add(100 * time.Millisecond)
	results := make(chan result, len(arrivals))
	semaphore := make(chan struct{}, cfg.maxOutstanding)
	var waitGroup sync.WaitGroup
	var outstanding int64

	for _, planned := range arrivals {
		scheduledAt := start.Add(planned.Offset)
		if !waitUntil(ctx, scheduledAt) {
			break
		}

		select {
		case semaphore <- struct{}{}:
			current := atomic.AddInt64(&outstanding, 1)
			waitGroup.Add(1)
			go func(planned arrival, scheduledAt time.Time, outstandingAtStart int64) {
				defer waitGroup.Done()
				defer func() {
					<-semaphore
					atomic.AddInt64(&outstanding, -1)
				}()
				results <- executeRequest(ctx, client, cfg, body, planned, scheduledAt, outstandingAtStart)
			}(planned, scheduledAt, current)
		default:
			results <- result{Sequence: planned.Sequence, Phase: planned.Phase, ScheduledAt: scheduledAt, Dropped: true, Error: "client max outstanding reached", ErrorKind: "client_capacity"}
		}
	}

	waitGroup.Wait()
	close(results)
	items := make([]result, 0, len(arrivals))
	for item := range results {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Sequence < items[j].Sequence })
	return items
}

func executeRequest(ctx context.Context, client *http.Client, cfg config, body []byte, planned arrival, scheduledAt time.Time, outstanding int64) result {
	startedAt := time.Now()
	item := result{
		Sequence:        planned.Sequence,
		Phase:           planned.Phase,
		ScheduledAt:     scheduledAt,
		StartedAt:       startedAt,
		ScheduleDelayMS: milliseconds(startedAt.Sub(scheduledAt)),
		Outstanding:     outstanding,
	}

	requestContext, cancel := context.WithTimeout(ctx, cfg.requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, cfg.target, bytes.NewReader(body))
	if err != nil {
		item.Error = err.Error()
		return item
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		item.CompletedAt = time.Now()
		item.ServiceLatencyMS = milliseconds(item.CompletedAt.Sub(startedAt))
		item.EndToEndLatencyMS = milliseconds(item.CompletedAt.Sub(scheduledAt))
		item.Error = err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			item.ErrorKind = "timeout"
		} else {
			item.ErrorKind = "transport"
		}
		return item
	}
	defer response.Body.Close()
	item.StatusCode = response.StatusCode
	item.RetryAfter = response.Header.Get("Retry-After")
	item.ResponseBytes, err = io.Copy(io.Discard, response.Body)
	item.CompletedAt = time.Now()
	item.ServiceLatencyMS = milliseconds(item.CompletedAt.Sub(startedAt))
	item.EndToEndLatencyMS = milliseconds(item.CompletedAt.Sub(scheduledAt))
	if err != nil {
		item.Error = err.Error()
		item.ErrorKind = "response_body"
	}
	return item
}

func waitUntil(ctx context.Context, target time.Time) bool {
	timer := time.NewTimer(time.Until(target))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func summarize(results []result) summary {
	report := summary{StatusCodes: make(map[int]int64)}
	serviceLatencies := make([]float64, 0, len(results))
	successLatencies := make([]float64, 0, len(results))
	endToEndLatencies := make([]float64, 0, len(results))
	scheduleDelays := make([]float64, 0, len(results))

	for _, item := range results {
		report.Offered++
		if item.Dropped {
			report.Dropped++
			continue
		}
		report.Started++
		if item.Outstanding > report.MaxOutstanding {
			report.MaxOutstanding = item.Outstanding
		}
		if !item.CompletedAt.IsZero() {
			report.Completed++
		}
		report.ResponseBytes += item.ResponseBytes
		if item.StatusCode != 0 {
			report.HTTPResponses++
			report.StatusCodes[item.StatusCode]++
			if item.StatusCode >= http.StatusOK && item.StatusCode < http.StatusMultipleChoices {
				report.Successful++
				successLatencies = append(successLatencies, item.ServiceLatencyMS)
			}
			if item.StatusCode == http.StatusServiceUnavailable {
				report.Overloaded++
			}
		}
		if item.ErrorKind != "" {
			if item.ErrorKind == "timeout" {
				report.Timeouts++
			} else {
				report.TransportErrors++
			}
		}
		serviceLatencies = append(serviceLatencies, item.ServiceLatencyMS)
		endToEndLatencies = append(endToEndLatencies, item.EndToEndLatencyMS)
		scheduleDelays = append(scheduleDelays, item.ScheduleDelayMS)
	}
	report.ServiceLatency = calculatePercentiles(serviceLatencies)
	report.SuccessLatency = calculatePercentiles(successLatencies)
	report.EndToEndLatency = calculatePercentiles(endToEndLatencies)
	report.ScheduleDelay = calculatePercentiles(scheduleDelays)
	return report
}

func calculatePercentiles(values []float64) percentiles {
	if len(values) == 0 {
		return percentiles{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return percentiles{
		P50: percentile(sorted, 0.50),
		P95: percentile(sorted, 0.95),
		P99: percentile(sorted, 0.99),
		Max: sorted[len(sorted)-1],
	}
}

func percentile(sorted []float64, quantile float64) float64 {
	index := int(quantile*float64(len(sorted)-1) + 0.5)
	return sorted[index]
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}
