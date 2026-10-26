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
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Azure/agentbaker/aks-node-controller/common"
	"github.com/Azure/agentbaker/aks-node-controller/helpers"
)

const (
	cloudRequestAttempts = 10
	cloudResponseLimit   = 8 << 20
	cloudCommandTimeout  = 2 * time.Minute
	cloudRefreshTimeout  = 15 * time.Minute
	cloudRefreshService  = `[Unit]
Description=Refresh Azure Custom Cloud CA certificates
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=%s
`
	cloudRefreshTimer = `[Unit]
Description=Daily refresh of Azure Custom Cloud CA certificates

[Timer]
OnCalendar=19:00
Persistent=true
RandomizedDelaySec=300

[Install]
WantedBy=timers.target
`
)

type cloudInitConfig struct {
	Location          string
	RepoDepotEndpoint string
	Timeout           time.Duration
}

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

func (a *App) initializeAKSCloud(ctx context.Context, env map[string]string) error {
	timeout, err := time.ParseDuration(env["CSE_TIMEOUT"] + "s")
	if err != nil || timeout <= 0 {
		return fmt.Errorf("invalid cloud initialization timeout %q", env["CSE_TIMEOUT"])
	}
	config := cloudInitConfig{
		Location:          env["LOCATION"],
		RepoDepotEndpoint: env["REPO_DEPOT_ENDPOINT"],
		Timeout:           timeout,
	}
	if a.initializeCloud != nil {
		return a.initializeCloud(ctx, config)
	}
	initializer := newCloudInitializer(a)
	defer initializer.close()
	return initializer.initialize(ctx, config)
}

func (a *App) runCloudCertificateRefresh(ctx context.Context, location string) error {
	if strings.TrimSpace(location) == "" {
		return errors.New("cloud certificate refresh requires a location")
	}
	var err error
	if a.refreshCloudCertificates != nil {
		err = a.refreshCloudCertificates(ctx, location)
	} else {
		initializer := newCloudInitializer(a)
		defer initializer.close()
		err = initializer.refreshCertificates(ctx, location)
	}
	if err != nil {
		slog.Error("Azure cloud certificate refresh failed", "error", err)
	}
	return err
}

func (c *cloudInitializer) initialize(ctx context.Context, config cloudInitConfig) error {
	initCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	return c.withEvent("CloudInitialization", func() error {
		location, err := normalizeCloudLocation(config.Location)
		if err != nil {
			return err
		}
		platform, err := c.platform()
		if err != nil {
			return err
		}
		eligible, err := c.refresh(initCtx, platform, location)
		if err != nil {
			return err
		}
		if eligible {
			if err := c.installRefreshSchedule(initCtx, platform, location); err != nil {
				return fmt.Errorf("schedule Azure cloud certificate refresh: %w", err)
			}
		}
		repositoryErr := c.withEvent("CloudRepositoryInitialization", func() error {
			return c.initializeRepositories(initCtx, platform, config.RepoDepotEndpoint)
		})
		if errors.Is(repositoryErr, errCloudRepositoryCacheRefresh) {
			if contextErr := initCtx.Err(); contextErr != nil {
				return contextErr
			}
			// The shell reports cache-refresh failures without propagating them to CSE.
			slog.Warn("RepoDepot package cache refresh failed, continuing cloud initialization", "error", repositoryErr)
			return nil
		}
		return repositoryErr
	})
}

func (c *cloudInitializer) refreshCertificates(ctx context.Context, location string) error {
	refreshCtx, cancel := context.WithTimeout(ctx, cloudRefreshTimeout)
	defer cancel()
	return c.withEvent("CloudCertificateRefresh", func() error {
		normalizedLocation, err := normalizeCloudLocation(location)
		if err != nil {
			return err
		}
		platform, err := c.platform()
		if err != nil {
			return err
		}
		_, err = c.refresh(refreshCtx, platform, normalizedLocation)
		return err
	})
}

func normalizeCloudLocation(location string) (string, error) {
	location = strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, location))
	if strings.IndexFunc(location, func(r rune) bool {
		return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz0123456789-", r)
	}) != -1 {
		return "", errors.New("cloud location contains unsupported characters")
	}
	if location == "" {
		slog.Warn("Azure cloud location is empty, using rcv1p certificate endpoints")
	}
	return location, nil
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

func (c *cloudInitializer) requestCertificates(ctx context.Context, path string) ([]byte, error) {
	attempt := 0
	delay, nextDelay := 3*time.Second, time.Duration(0)
	return common.RetryFetch(ctx, cloudRequestAttempts, nil, func(ctx context.Context) ([]byte, error) {
		if attempt > 0 {
			if err := c.wait(ctx, nextDelay); err != nil {
				return nil, err
			}
		}
		attempt++
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.wireEndpoint+path, nil)
		if err != nil {
			return nil, err
		}
		nextDelay = delay
		resp, err := c.wireClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := readCloudResponse(resp)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, nil
		}
		if resp.StatusCode == http.StatusForbidden && bytes.Contains(body, []byte("RequestRateLimitExceeded")) {
			delay *= 2
		}
		return nil, fmt.Errorf("WireServer certificate request %s returned HTTP %d", path, resp.StatusCode)
	})
}

func (c *cloudInitializer) installRefreshSchedule(ctx context.Context, platform cloudPlatform, location string) error {
	if location == "" {
		return errors.New("cloud certificate refresh requires a location")
	}
	binary := c.executable
	if binary == "" {
		var err error
		binary, err = os.Executable()
		if err != nil {
			return err
		}
	}
	if !filepath.IsAbs(binary) || strings.ContainsAny(binary, "\"\\%\r\n") {
		return errors.New("invalid executable path for Azure cloud certificate refresh")
	}
	// Use the running binary, not the VHD binary: an ANC hotfix can add this
	// command on images whose baked-in binary does not support it yet.
	command := strconv.Quote(binary) + " refresh-cloud-certificates --location " + strconv.Quote(location)
	if platform.ID == osReleaseIDFlatcar || platform.ID == osReleaseIDAzureContainerLinux {
		return c.installSystemdRefresh(ctx, command)
	}
	return c.installCronRefresh(ctx, command)
}

func (c *cloudInitializer) installSystemdRefresh(ctx context.Context, command string) error {
	if err := c.writeFile("/etc/systemd/system/azure-ca-refresh.service", []byte(fmt.Sprintf(cloudRefreshService, command))); err != nil {
		return err
	}
	if err := c.writeFile("/etc/systemd/system/azure-ca-refresh.timer", []byte(cloudRefreshTimer)); err != nil {
		return err
	}
	if _, err := c.command(ctx, nil, "systemctl", "daemon-reload"); err != nil {
		return err
	}
	_, err := c.command(ctx, nil, "systemctl", "enable", "--now", "azure-ca-refresh.timer")
	return err
}

func (c *cloudInitializer) installCronRefresh(ctx context.Context, command string) error {
	existing, err := c.command(ctx, nil, "crontab", "-l")
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return contextErr
		}
		var exitErr interface{ ExitCode() int }
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 || !strings.Contains(err.Error(), "no crontab for") {
			slog.Warn("could not read existing crontab, continuing with an empty crontab", "error", err)
		}
		existing = nil
	}
	var lines []string
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == "" ||
			strings.Contains(line, "init-aks-cloud.sh") && strings.Contains(line, "ca-refresh") ||
			strings.Contains(line, "refresh-cloud-certificates") && strings.Contains(line, "aks-node-controller") {
			continue
		}
		lines = append(lines, line)
	}
	lines = append(lines, "0 19 * * * "+command)
	_, err = c.command(ctx, []byte(strings.Join(lines, "\n")+"\n"), "crontab", "-")
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return contextErr
		}
		slog.Warn("could not install Azure cloud certificate refresh cron job, continuing cloud initialization", "error", err)
	}
	return nil
}
