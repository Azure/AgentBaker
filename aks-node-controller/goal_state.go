package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/Azure/agentbaker/aks-node-controller/common"
)

const (
	wireServerEndpoint = "http://168.63.129.16"
	kvpPoolPath        = "/var/lib/hyperv/.kvp_pool_1"
	kvpKeySize         = 512
	kvpValueSize       = 2048
	kvpReportLimit     = 1024
	descriptionLimit   = 512
	healthReportTries  = 3
)

type goalState struct {
	Incarnation string `xml:"Incarnation"`
	ContainerID string `xml:"Container>ContainerId"`
	InstanceID  string `xml:"Container>RoleInstanceList>RoleInstance>InstanceId"`
}

type healthDetails struct {
	SubStatus   string `xml:"SubStatus"`
	Description string `xml:"Description"`
}

type healthReport struct {
	XMLName     xml.Name       `xml:"Health"`
	Incarnation string         `xml:"GoalStateIncarnation"`
	ContainerID string         `xml:"Container>ContainerId"`
	InstanceID  string         `xml:"Container>RoleInstanceList>Role>InstanceId"`
	State       string         `xml:"Container>RoleInstanceList>Role>Health>State"`
	Details     *healthDetails `xml:"Container>RoleInstanceList>Role>Health>Details,omitempty"`
}

func reportGoalState(ctx context.Context, failed bool, description string) error {
	reportProvisioningKVP(ctx, failed, description)

	transport := common.NewBaseTransport(common.HTTPTransportOptions{
		DialTimeout:           30 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	})
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		// Like the Python reporter, do not follow redirects from the wireserver.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return sendHealthReport(ctx, client, wireServerEndpoint, failed, description, 5*time.Second)
}

func sendHealthReport(ctx context.Context, client *http.Client, endpoint string, failed bool, description string, retryDelay time.Duration) error {
	description = trimReportDescription(description)
	state := "Ready"
	if failed {
		state = "NotReady"
	}
	var lastErr error
	for attempt := 1; attempt <= healthReportTries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = sendHealthReportOnce(ctx, client, endpoint, state, description)
		if lastErr == nil {
			slog.Info(fmt.Sprintf("Successfully reported %s to Azure fabric.", state))
			return nil
		}
		slog.Warn("failed to report provisioning status", "attempt", attempt, "state", state, "error", lastErr)
		if attempt < healthReportTries {
			timer := time.NewTimer(retryDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return fmt.Errorf("failed to report %s after %d attempts: %w", state, healthReportTries, lastErr)
}

func sendHealthReportOnce(ctx context.Context, client *http.Client, endpoint, state, description string) error {
	raw, err := goalStateRequest(ctx, client, http.MethodGet, endpoint+"/machine/?comp=goalstate", nil)
	if err != nil {
		return err
	}
	var goal goalState
	if err = xml.Unmarshal(raw, &goal); err != nil {
		return fmt.Errorf("parse GoalState: %w", err)
	}
	if goal.Incarnation == "" || goal.ContainerID == "" || goal.InstanceID == "" {
		return fmt.Errorf("GoalState is missing incarnation, container ID or instance ID")
	}
	report := healthReport{
		Incarnation: goal.Incarnation,
		ContainerID: goal.ContainerID,
		InstanceID:  goal.InstanceID,
		State:       state,
	}
	if state == "NotReady" {
		report.Details = &healthDetails{SubStatus: "ProvisioningFailed", Description: description}
	}
	body, err := xml.Marshal(report)
	if err != nil {
		return fmt.Errorf("encode health report: %w", err)
	}
	_, err = goalStateRequest(ctx, client, http.MethodPost, endpoint+"/machine?comp=health", append([]byte(xml.Header), body...))
	return err
}

func goalStateRequest(ctx context.Context, client *http.Client, method, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-ms-agent-name", "WALinuxAgent")
	req.Header.Set("x-ms-version", "2012-11-30")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK &&
		(method != http.MethodPost || (resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted)) {
		return nil, fmt.Errorf("%s %s returned %s", method, url, resp.Status)
	}
	if method == http.MethodPost {
		return nil, nil
	}
	return io.ReadAll(resp.Body)
}

func provisioningVMID(ctx context.Context) string {
	if id, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
		return strings.ToLower(strings.TrimSpace(string(id)))
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	id, err := exec.CommandContext(ctx, "dmidecode", "-s", "system-uuid").Output()
	if err == nil {
		return strings.ToLower(strings.TrimSpace(string(id)))
	}
	slog.Warn("failed to read VM ID for KVP report", "error", err)
	return "00000000-0000-0000-0000-000000000000"
}

func reportProvisioningKVP(ctx context.Context, failed bool, description string) {
	vmID := provisioningVMID(ctx)
	dmesgCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	dmesg, err := exec.CommandContext(dmesgCtx, "dmesg").Output()
	if err != nil {
		slog.Warn("failed to read dmesg for KVP report", "error", err)
	}
	if len(dmesg) > kvpReportLimit {
		dmesg = dmesg[len(dmesg)-kvpReportLimit:]
	}
	if err := writeKVPRecord(kvpPoolPath, "dmesg", truncateKVPValue(string(dmesg), kvpReportLimit)); err != nil {
		slog.Warn("failed to write dmesg to KVP", "error", err)
	}

	result := "success"
	if failed {
		result = "failure"
	}
	fields := []string{
		"result=" + result,
		"agent=AKS-CSE",
		"timestamp=" + time.Now().UTC().Format("2006-01-02T15:04:05.000000+00:00"),
		"vm_id=" + vmID,
	}
	if failed {
		fields = append(fields, "description="+trimReportDescription(description))
	}
	report := truncateKVPValue(encodeProvisioningReport(fields), kvpReportLimit-1)
	if err := writeKVPRecord(kvpPoolPath, "PROVISIONING_REPORT", report); err != nil {
		slog.Warn("failed to write PROVISIONING_REPORT to KVP", "error", err)
	} else {
		slog.Info("Wrote PROVISIONING_REPORT to KVP.")
	}
}

func trimReportDescription(description string) string {
	runes := []rune(description)
	return string(runes[:min(len(runes), descriptionLimit)])
}

// cloud-init uses pipe-delimited CSV with single quotes, unlike encoding/csv.
func encodeProvisioningReport(fields []string) string {
	encoded := make([]string, len(fields))
	for i, field := range fields {
		if strings.ContainsAny(field, "|'\r\n") {
			field = "'" + strings.ReplaceAll(field, "'", "''") + "'"
		}
		encoded[i] = field
	}
	return strings.Join(encoded, "|")
}

func truncateKVPValue(value string, limit int) string {
	value = strings.ToValidUTF8(value, "\uFFFD")
	if len(value) <= limit {
		return value
	}
	for !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit]
}

func writeKVPRecord(path, key, value string) error {
	record := make([]byte, kvpKeySize+kvpValueSize)
	copy(record[:kvpKeySize], key)
	copy(record[kvpKeySize:], value)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	// Closing the file releases the lock; os.File.Write is unbuffered.
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	_, err = f.Write(record)
	return err
}
