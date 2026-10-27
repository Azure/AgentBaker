package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func assertCloudSchedule(t *testing.T, c *cloudInitializer, osName string, commands []string) {
	t.Helper()
	if osName != "Flatcar" && osName != "ACL" {
		require.Contains(t, commands, "crontab -")
		return
	}
	require.Contains(t, commands, "systemctl enable --now azure-ca-refresh.timer")
	service, err := os.ReadFile(c.path("/etc/systemd/system/azure-ca-refresh.service"))
	require.NoError(t, err)
	require.Contains(t, string(service), hotfixBinaryPath)
	require.Contains(t, string(service), "refresh-cloud-certificates")
	require.NotContains(t, string(service), "init-aks-cloud.sh")
}

func TestCloudRefreshScheduleReplacesLegacyCron(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	cron := "5 2 * * * /usr/bin/keep-me\n" +
		"0 19 * * * \"/opt/azure/containers/init-aks-cloud.sh\" ca-refresh \"usnateast\"\n"
	c.run = func(cmd *exec.Cmd) error {
		switch strings.Join(cmd.Args, " ") {
		case "crontab -l":
			_, _ = io.WriteString(cmd.Stdout, cron)
		case "crontab -":
			data, err := io.ReadAll(cmd.Stdin)
			require.NoError(t, err)
			cron = string(data)
		default:
			t.Fatalf("unexpected scheduling command: %s", cmd)
		}
		return nil
	}
	for range 2 {
		require.NoError(t, c.installRefreshSchedule(t.Context(), cloudPlatform{ID: osReleaseIDUbuntu}, "usnateast"))
	}
	require.Contains(t, cron, "5 2 * * * /usr/bin/keep-me")
	require.NotContains(t, cron, "init-aks-cloud.sh")
	require.Equal(t, 1, strings.Count(cron, "refresh-cloud-certificates"))
	require.Contains(t, cron, fmt.Sprintf(`0 19 * * * "%s" refresh-cloud-certificates --location "usnateast"`, hotfixBinaryPath))
}

func TestCloudRefreshScheduleCrontabErrors(t *testing.T) {
	for _, tc := range []struct {
		name, stderr string
	}{
		{"no existing crontab", "no crontab for root"},
		{"permission denied", "permission denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCloudInitializer(t, cloudTestUbuntu)
			writes := 0
			c.run = func(cmd *exec.Cmd) error {
				if cmd.Args[1] == "-l" {
					_, _ = io.WriteString(cmd.Stderr, tc.stderr)
					return &testExitError{Code: 1}
				}
				writes++
				return nil
			}
			err := c.installRefreshSchedule(t.Context(), cloudPlatform{ID: osReleaseIDUbuntu}, "westus2")
			require.NoError(t, err)
			require.Equal(t, 1, writes)
		})
	}
}

func TestCloudCronInstallationFailureIsLogged(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	logs := installLogCapturer(t)
	c.run = func(cmd *exec.Cmd) error {
		if cmd.Args[1] == "-" {
			return errors.New("cron installation failed")
		}
		return nil
	}
	require.NoError(t, c.installRefreshSchedule(t.Context(), cloudPlatform{ID: osReleaseIDUbuntu}, "westus2"))
	require.Contains(t, fmt.Sprint(logs.getRecords()), "cron installation failed")
}

func TestCloudCronCancellationIsNotIgnored(t *testing.T) {
	for _, canceledArg := range []string{"-l", "-"} {
		t.Run(canceledArg, func(t *testing.T) {
			c := newTestCloudInitializer(t, cloudTestUbuntu)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			c.run = func(cmd *exec.Cmd) error {
				if cmd.Args[1] == canceledArg {
					cancel()
					return ctx.Err()
				}
				return nil
			}
			require.ErrorIs(t, c.installRefreshSchedule(ctx, cloudPlatform{ID: osReleaseIDUbuntu}, "westus2"), context.Canceled)
		})
	}
}

func TestCloudCronDeadlineIsNotIgnored(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	c.run = func(*exec.Cmd) error {
		t.Fatal("an expired context must not execute crontab")
		return nil
	}
	require.ErrorIs(t, c.installRefreshSchedule(ctx, cloudPlatform{ID: osReleaseIDUbuntu}, "westus2"), context.DeadlineExceeded)
}

func TestCloudSystemdRefreshFailuresRemainFatal(t *testing.T) {
	for _, failedArgs := range []string{"daemon-reload", "enable --now azure-ca-refresh.timer"} {
		t.Run(failedArgs, func(t *testing.T) {
			c := newTestCloudInitializer(t, "ID=flatcar\n")
			c.run = func(cmd *exec.Cmd) error {
				if strings.Join(cmd.Args[1:], " ") == failedArgs {
					return errors.New("systemd operation failed")
				}
				return nil
			}
			require.ErrorContains(t, c.installRefreshSchedule(t.Context(), cloudPlatform{ID: osReleaseIDFlatcar}, "westus2"),
				"systemd operation failed")
		})
	}
}
