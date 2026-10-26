package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/common"
	"github.com/Azure/agentbaker/aks-node-controller/helpers"
)

const cloudRequestAttempts = 10

const cloudResponseLimit = 8 << 20

const cloudCommandTimeout = 2 * time.Minute

type cloudPlatform struct {
	ID, Version, Codename string
}

type cloudInitializer struct {
	run           func(*exec.Cmd) error
	wait          func(context.Context, time.Duration) error
	wireClient    *http.Client
	repoClient    *http.Client
	wireEndpoint  string
	eventLogger   *helpers.EventLogger
	rootDir       string
	osReleasePath string
	executable    string
}

func newCloudInitializer(a *App) *cloudInitializer {
	transport := common.NewBaseTransport(common.HTTPTransportOptions{
		DialTimeout:           10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	})
	repoTransport := transport.Clone()
	repoTransport.Proxy = http.ProxyFromEnvironment
	return &cloudInitializer{
		run:          a.cmdRun,
		wait:         waitCloudRetry,
		wireEndpoint: wireServerEndpoint,
		wireClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
			// WireServer responses must not redirect to another endpoint.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		repoClient:    &http.Client{Transport: repoTransport, Timeout: 30 * time.Second},
		eventLogger:   a.eventLogger,
		osReleasePath: a.osReleasePath,
	}
}

func (c *cloudInitializer) close() {
	c.wireClient.CloseIdleConnections()
	c.repoClient.CloseIdleConnections()
}

func (c *cloudInitializer) platform() (cloudPlatform, error) {
	path := c.osReleasePath
	if path == "" {
		path = c.path(defaultOSReleasePath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cloudPlatform{}, fmt.Errorf("read cloud initialization OS release: %w", err)
	}
	values := parseNodeCustomDataOSRelease(data)
	id := strings.ToLower(values["ID"])
	switch {
	case strings.Contains(values["NAME"], "Ubuntu"), id == osReleaseIDUbuntu:
		id = osReleaseIDUbuntu
	case strings.Contains(id, osReleaseIDFlatcar):
		id = osReleaseIDFlatcar
	case id == osReleaseIDAzureContainerLinux,
		id == osReleaseIDAzureLinux && values["VARIANT_ID"] == osReleaseIDAzureContainerLinux:
		id = osReleaseIDAzureContainerLinux
	case strings.Contains(values["NAME"], "Mariner"), id == osReleaseIDMariner:
		id = osReleaseIDMariner
	case strings.Contains(values["NAME"], "Microsoft Azure Linux"), id == osReleaseIDAzureLinux:
		id = osReleaseIDAzureLinux
	default:
		return cloudPlatform{}, fmt.Errorf("unsupported cloud initialization OS %q", id)
	}
	return cloudPlatform{ID: id, Version: values["VERSION_ID"], Codename: values["VERSION_CODENAME"]}, nil
}

func (c *cloudInitializer) path(path string) string {
	if c.rootDir == "" {
		return path
	}
	return filepath.Join(c.rootDir, strings.TrimPrefix(path, "/"))
}

func (c *cloudInitializer) writeFile(path string, data []byte) error {
	return writeCloudFile(c.path(path), data)
}

func writeCloudFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	// #nosec G306 -- Public CAs, package keys, and repository metadata must be readable by unprivileged clients.
	return root.WriteFile(filepath.Base(path), data, 0o644)
}

func (c *cloudInitializer) withEvent(task string, action func() error) error {
	start := time.Now()
	err := action()
	level, message := helpers.EventLevelInformational, "Completed"
	if err != nil {
		level, message = helpers.EventLevelError, err.Error()
		slog.Error("Azure cloud operation failed", "task", task, "error", err)
	}
	if c.eventLogger != nil {
		c.eventLogger.LogEvent(task, message, level, start, time.Now())
	}
	return err
}

func (c *cloudInitializer) command(ctx context.Context, input []byte, name string, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, cloudCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	run := c.run
	if run == nil {
		run = cmdRunner
	}
	if err := run(cmd); err != nil {
		return stdout.Bytes(), fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	if name == "dnf" {
		// DNF can report unavailable repositories on stderr despite exiting zero.
		_, _ = stdout.Write(stderr.Bytes())
	}
	return stdout.Bytes(), nil
}

func waitCloudRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func readCloudResponse(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, cloudResponseLimit+1))
	if err != nil {
		return nil, err
	}
	if len(body) > cloudResponseLimit {
		return nil, errors.New("cloud response exceeds size limit")
	}
	return body, nil
}
