package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const cloudRefreshService = `[Unit]
Description=Refresh Azure Custom Cloud CA certificates
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=%s
`

const cloudRefreshTimer = `[Unit]
Description=Daily refresh of Azure Custom Cloud CA certificates

[Timer]
OnCalendar=19:00
Persistent=true
RandomizedDelaySec=300

[Install]
WantedBy=timers.target
`

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
