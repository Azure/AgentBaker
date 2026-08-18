package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	defaultWireServerEndpoint    = "168.63.129.16"
	defaultKVPFilePath           = "/var/lib/hyperv/.kvp_pool_1"
	defaultVMIDFilePath          = "/sys/class/dmi/id/product_uuid"
	defaultReportReadyRetries    = 3
	defaultReportReadyRetryDelay = 5 * time.Second
	reportReadyRequestTimeout    = 30 * time.Second
	dmesgTimeout                 = 10 * time.Second
	kvpKeySize                   = 512
	kvpValueSize                 = 2048
	kvpMaxValueSize              = 1024
	descriptionMaxLength         = 512
)

type reportReadyOptions struct {
	endpoint    string
	ready       bool
	description string
	retries     int
	retryDelay  time.Duration
}

type wireGoalState struct {
	Incarnation string `xml:"Incarnation"`
	Container   struct {
		ID               string `xml:"ContainerId"`
		RoleInstanceList struct {
			RoleInstance struct {
				ID string `xml:"InstanceId"`
			} `xml:"RoleInstance"`
		} `xml:"RoleInstanceList"`
	} `xml:"Container"`
}

type provisioningReporter struct {
	endpoint   string
	kvpPath    string
	vmIDPath   string
	httpClient *http.Client
	now        func() time.Time
	runDmesg   func(context.Context) ([]byte, error)
}

func reportProvisioningStatus(ctx context.Context, options reportReadyOptions) error {
	if options.retries < 1 {
		return errors.New("retries must be at least 1")
	}
	if options.retryDelay < 0 {
		return errors.New("retry delay cannot be negative")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	reporter := provisioningReporter{
		endpoint: options.endpoint,
		kvpPath:  defaultKVPFilePath,
		vmIDPath: defaultVMIDFilePath,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   reportReadyRequestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		now: time.Now,
		runDmesg: func(ctx context.Context) ([]byte, error) {
			commandCtx, cancel := context.WithTimeout(ctx, dmesgTimeout)
			defer cancel()
			output, err := exec.CommandContext(commandCtx, "dmesg").Output()
			if commandCtx.Err() != nil {
				return output, commandCtx.Err()
			}
			return output, err
		},
	}
	return reporter.report(ctx, options)
}

func (r provisioningReporter) report(ctx context.Context, options reportReadyOptions) error {
	r.reportDmesg(ctx)
	result := "failure"
	if options.ready {
		result = "success"
	}
	r.provisioningReport(result, options.description)

	var lastError error
	for attempt := 1; attempt <= options.retries; attempt++ {
		goalState, err := r.fetchGoalState(ctx)
		if err == nil {
			err = r.postHealth(ctx, goalState, options.ready, options.description)
		}
		if err == nil {
			slog.Info("reported provisioning status to Azure", "status", readinessStatus(options.ready))
			return nil
		}

		lastError = err
		slog.Warn("provisioning status report failed", "attempt", attempt, "retries", options.retries, "error", err)
		if attempt < options.retries {
			if err := sleepWithContext(ctx, options.retryDelay); err != nil {
				return fmt.Errorf("waiting to retry provisioning status report: %w", err)
			}
		}
	}
	return fmt.Errorf("failed to report provisioning status: %w", lastError)
}

func (r provisioningReporter) reportDmesg(ctx context.Context) {
	output, err := r.runDmesg(ctx)
	var exitError *exec.ExitError
	if err != nil && !errors.As(err, &exitError) {
		slog.Warn("failed to capture dmesg", "error", err)
		return
	}
	if len(output) > kvpMaxValueSize {
		output = output[len(output)-kvpMaxValueSize:]
	}
	r.appendKVP("dmesg", strings.ToValidUTF8(string(output), "\uFFFD"))
}

func (r provisioningReporter) provisioningReport(result string, description string) {
	timestamp := strings.TrimSuffix(r.now().UTC().Format(time.RFC3339Nano), "Z") + "+00:00"
	fields := []string{
		"result=" + result,
		"agent=AKS-CSE",
		"timestamp=" + timestamp,
		"vm_id=" + r.vmID(),
	}
	if description != "" {
		fields = append(fields, "description="+truncateString(description, descriptionMaxLength))
	}
	for i := range fields {
		fields[i] = quoteKVPField(fields[i])
	}
	r.appendKVP("PROVISIONING_REPORT", strings.Join(fields, "|"))
}

func (r provisioningReporter) vmID() string {
	value, err := os.ReadFile(r.vmIDPath)
	if err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}
	return strings.ToLower(strings.TrimSpace(string(value)))
}

func (r provisioningReporter) appendKVP(key string, value string) {
	file, err := os.OpenFile(r.kvpPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("failed to open KVP file", "error", err)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Warn("failed to close KVP file", "error", err)
		}
	}()

	fd := int(file.Fd()) //nolint:gosec // Unix file descriptors are represented as int values.
	if err := unix.Flock(fd, unix.LOCK_EX); err != nil {
		slog.Warn("failed to lock KVP file", "error", err)
		return
	}
	defer func() {
		if err := unix.Flock(fd, unix.LOCK_UN); err != nil {
			slog.Warn("failed to unlock KVP file", "error", err)
		}
	}()

	record := make([]byte, kvpKeySize+kvpValueSize)
	copy(record[:kvpKeySize], []byte(key))
	copy(record[kvpKeySize:], []byte(truncateString(value, kvpMaxValueSize-1)))
	if _, err := file.Write(record); err != nil {
		slog.Warn("failed to write KVP record", "error", err)
	}
}

func (r provisioningReporter) fetchGoalState(ctx context.Context) (wireGoalState, error) {
	var state wireGoalState
	payload, err := r.request(ctx, http.MethodGet, "/machine/?comp=goalstate", nil)
	if err != nil {
		return state, err
	}
	if err := xml.Unmarshal(payload, &state); err != nil {
		return state, fmt.Errorf("parsing goal state: %w", err)
	}
	if state.Incarnation == "" || state.Container.ID == "" || state.Container.RoleInstanceList.RoleInstance.ID == "" {
		return state, errors.New("goal state is missing required fields")
	}
	return state, nil
}

func (r provisioningReporter) postHealth(
	ctx context.Context,
	state wireGoalState,
	ready bool,
	description string,
) error {
	_, err := r.request(ctx, http.MethodPost, "/machine?comp=health", healthDocument(state, ready, description))
	return err
}

func (r provisioningReporter) request(ctx context.Context, method string, path string, body []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		method,
		"http://"+r.endpoint+path,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("creating WireServer request: %w", err)
	}
	request.Header.Set("x-ms-agent-name", "WALinuxAgent")
	request.Header.Set("x-ms-version", "2012-11-30")
	if body != nil {
		request.Header.Set("Content-Type", "text/xml; charset=utf-8")
	}

	response, err := r.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			slog.Warn("failed to close WireServer response body", "error", err)
		}
	}()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s %s response: %w", method, path, err)
	}
	if response.StatusCode != http.StatusOK &&
		response.StatusCode != http.StatusCreated &&
		response.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("%s %s returned HTTP %s", method, path, response.Status)
	}
	return payload, nil
}

func healthDocument(state wireGoalState, ready bool, description string) []byte {
	details := ""
	if !ready {
		details = "<Details><SubStatus>ProvisioningFailed</SubStatus><Description>" +
			html.EscapeString(truncateString(description, descriptionMaxLength)) +
			"</Description></Details>"
	}
	return []byte(
		`<?xml version="1.0" encoding="utf-8"?><Health>` +
			"<GoalStateIncarnation>" + html.EscapeString(state.Incarnation) + "</GoalStateIncarnation>" +
			"<Container><ContainerId>" + html.EscapeString(state.Container.ID) + "</ContainerId>" +
			"<RoleInstanceList><Role><InstanceId>" +
			html.EscapeString(state.Container.RoleInstanceList.RoleInstance.ID) +
			"</InstanceId><Health><State>" + readinessStatus(ready) + "</State>" +
			details + "</Health></Role></RoleInstanceList></Container></Health>",
	)
}

func readinessStatus(ready bool) string {
	if ready {
		return "Ready"
	}
	return "NotReady"
}

func quoteKVPField(value string) string {
	if !strings.ContainsAny(value, "|\r\n'") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func truncateString(value string, maxLength int) string {
	runes := []rune(value)
	if len(runes) <= maxLength {
		return value
	}
	return string(runes[:maxLength])
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
