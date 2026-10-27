//go:build linux && cloudintegration

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/Azure/agentbaker/aks-node-controller/testutil/cloudfixture"
	"github.com/stretchr/testify/require"
)

var cloudSystemIntegration = flag.Bool("cloud-system-integration", false, "mutate a disposable Linux integration VM")
var cloudIntegrationBinary = flag.String("cloud-integration-binary", hotfixBinaryPath, "PR-built ANC binary")
var cloudIntegrationLocation = flag.String("cloud-integration-location", "", "Azure location of the disposable VM")

const cloudIntegrationGuard = "/var/lib/anc-cloud-integration/allow-system-tests"

type cloudSystemWireProfile struct {
	optIn                              string
	missingOperations, missingDownload bool
	blockOptIn                         bool
	optInStarted                       atomic.Bool
}

type cloudSystemFixture struct {
	initializer          *cloudInitializer
	ctx                  context.Context
	platform             cloudPlatform
	config               cloudInitConfig
	proxy                *cloudfixture.RepositoryProxy
	profile              atomic.Pointer[cloudSystemWireProfile]
	normal               *cloudSystemWireProfile
	repositoryURL        string
	anchor, trustCommand string
	eventDir             string
}

// This suite deliberately uses the host filesystem and actual OS commands.
// The build tag, explicit flag and VM marker prevent ordinary unit tests from
// changing a developer machine's trust store, repositories or root crontab.
func TestCloudSystemIntegration(t *testing.T) {
	if !*cloudSystemIntegration {
		t.Skip("requires an explicitly designated disposable integration VM")
	}
	require.Zero(t, os.Geteuid(), "system integration must run as root")
	guard, err := os.ReadFile(cloudIntegrationGuard)
	require.NoError(t, err)
	require.Equal(t, "disposable AgentBaker cloud integration\n", string(guard))
	require.NotEmpty(t, *cloudIntegrationLocation)
	require.FileExists(t, *cloudIntegrationBinary)

	fixture := newCloudSystemFixture(t)
	t.Run("TrustRepositoriesAndCron", fixture.trustRepositoriesAndCron)
	if t.Failed() {
		return
	}
	t.Run("IndividualDownloadFailureContinues", fixture.individualDownloadFailureContinues)
	t.Run("CronPermissionFailureContinues", fixture.cronPermissionFailureContinues)
	t.Run("CacheExhaustionContinuesWithErrorTelemetry", fixture.cacheExhaustionContinuesWithErrorTelemetry)
	t.Run("CertificateFatalBoundaries", fixture.certificateFatalBoundaries)
	t.Run("InvalidOptInSkipsInstallation", fixture.invalidOptInSkipsInstallation)
	t.Run("LegacyTrustPermissionFailureContinues", fixture.legacyTrustPermissionFailureContinues)
	t.Run("CancellationIsFatal", fixture.cancellationIsFatal)
	t.Run("SystemdRefreshAndPermissionFailure", fixture.systemdRefreshAndPermissionFailure)
}

func newCloudSystemFixture(t *testing.T) *cloudSystemFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Minute)
	t.Cleanup(cancel)
	eventDir := t.TempDir()
	c := newCloudInitializer(&App{eventLogger: helpers.NewEventLogger(eventDir)})
	t.Cleanup(c.close)
	c.executable = *cloudIntegrationBinary
	platform, err := c.platform()
	require.NoError(t, err)
	require.Contains(t, []string{osReleaseIDUbuntu, osReleaseIDAzureLinux}, platform.ID)
	preserveCloudSystemRepositories(t, platform)
	preserveCloudSystemCrontab(t, c, ctx)

	ca, leaf, name := newCloudSystemCertificate(t)
	proxy := cloudfixture.NewRepositoryProxy()
	repository := httptest.NewUnstartedServer(proxy)
	repository.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{leaf}}
	repository.StartTLS()
	t.Cleanup(repository.Close)

	fixture := &cloudSystemFixture{
		initializer: c, ctx: ctx, platform: platform, proxy: proxy, repositoryURL: repository.URL, eventDir: eventDir,
		normal: &cloudSystemWireProfile{optIn: `{"IsOptedInForRootCerts":true}`},
		config: cloudInitConfig{
			Location: *cloudIntegrationLocation, RepoDepotEndpoint: repository.URL, Timeout: 10 * time.Minute,
		},
		anchor:       "/usr/local/share/ca-certificates/" + name + ".crt",
		trustCommand: "update-ca-certificates",
	}
	fixture.profile.Store(fixture.normal)
	wire := httptest.NewServer(cloudSystemWireHandler(&fixture.profile, ca, name))
	t.Cleanup(wire.Close)
	c.wireEndpoint = wire.URL
	if platform.ID == osReleaseIDAzureLinux {
		fixture.anchor = "/etc/pki/ca-trust/source/anchors/" + name + ".crt"
		fixture.trustCommand = "update-ca-trust"
	}
	t.Cleanup(func() {
		removeErr := os.Remove(fixture.anchor)
		if !errors.Is(removeErr, os.ErrNotExist) {
			require.NoError(t, removeErr)
		}
		_, err := c.command(context.Background(), nil, fixture.trustCommand)
		require.NoError(t, err)
		if platform.ID == osReleaseIDUbuntu {
			require.NoError(t, c.copyOpenSSLBundle())
		}
	})
	return fixture
}

func cloudSystemWireHandler(profile *atomic.Pointer[cloudSystemWireProfile], ca []byte, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kind := r.URL.Query().Get("type")
		state := profile.Load()
		switch {
		case r.URL.Path == "/acms/isOptedInForRootCerts":
			if state.blockOptIn {
				state.optInStarted.Store(true)
				<-r.Context().Done()
				return
			}
			_, _ = w.Write([]byte(state.optIn))
		case kind == "operationrequestsroot", kind == "operationrequestsintermediate":
			names := []map[string]string{{"ResouceFileName": name + ".crt"}}
			if state.missingOperations {
				names = nil
			}
			if state.missingDownload {
				names = append(names, map[string]string{"ResouceFileName": "unavailable.crt"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"OperationsInfo": names})
		case kind == "unavailable":
			// A successful but empty response exercises per-certificate failure
			// without introducing an unrelated multi-minute transport retry.
		case kind == "cacertificates":
			_ = json.NewEncoder(w).Encode(map[string]any{"Certificates": []map[string]string{
				{"Name": name + ".cer", "CertBody": string(ca)},
			}})
		default:
			_, _ = w.Write(ca)
		}
	})
}

func (f *cloudSystemFixture) trustRepositoriesAndCron(t *testing.T) {
	c := f.initializer
	_, err := c.command(f.ctx, nil, "curl", "--silent", "--show-error", "--noproxy", "*", f.repositoryURL+"/keys/microsoft.asc")
	var exit interface{ ExitCode() int }
	require.ErrorAs(t, err, &exit)
	require.Equal(t, 60, exit.ExitCode(), "fresh test CA must not already be trusted")
	require.NoError(t, c.initialize(f.ctx, f.config))
	require.FileExists(t, f.anchor)
	for _, event := range c.eventLogger.Events() {
		require.NotEqual(t, "Error", event.EventLevel, "happy path must not use a best-effort failure")
	}
	stats := f.proxy.Stats()
	metadataCount := 7
	if f.platform.ID == osReleaseIDUbuntu {
		metadataCount = 5
	}
	require.GreaterOrEqual(t, stats.Metadata, metadataCount, "all configured repositories need real vendor metadata")
	if f.platform.ID == osReleaseIDUbuntu {
		require.GreaterOrEqual(t, stats.Keys, 2)
		for _, key := range []string{"microsoft.asc", "msopentech.asc"} {
			_, keyErr := c.command(f.ctx, nil, "gpg", "--show-keys", "/etc/apt/keyrings/"+key+".gpg")
			require.NoError(t, keyErr)
		}
	}
	_, err = c.command(f.ctx, nil, "curl", "--fail", "--silent", "--show-error", "--noproxy", "*", f.repositoryURL+"/keys/microsoft.asc")
	require.NoError(t, err, "HTTPS must work through the actual updated system trust store")
	cron, err := c.command(f.ctx, nil, "crontab", "-l")
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(cron), "refresh-cloud-certificates"))
	require.Contains(t, string(cron), strconv.Quote(*cloudIntegrationBinary))
	require.Contains(t, string(cron), "# anc-cloud-system-integration-unrelated")
}

func (f *cloudSystemFixture) individualDownloadFailureContinues(t *testing.T) {
	f.profile.Store(&cloudSystemWireProfile{optIn: f.normal.optIn, missingDownload: true})
	defer f.profile.Store(f.normal)
	require.NoError(t, f.initializer.initialize(f.ctx, cloudInitConfig{Location: f.config.Location, Timeout: time.Minute}))
}

func (f *cloudSystemFixture) cronPermissionFailureContinues(t *testing.T) {
	c := f.initializer
	denied := 0
	c.run = func(cmd *exec.Cmd) error {
		if filepath.Base(cmd.Path) == "crontab" {
			denied++
			cmd.Args = append([]string{cmd.Args[0], "-u", "root"}, cmd.Args[1:]...)
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
		}
		return cmdRunner(cmd)
	}
	defer func() { c.run = nil }()
	require.NoError(t, c.initialize(f.ctx, cloudInitConfig{Location: f.config.Location, Timeout: time.Minute}))
	require.Equal(t, 2, denied, "both real crontab read and install must be attempted")
}

func (f *cloudSystemFixture) cacheExhaustionContinuesWithErrorTelemetry(t *testing.T) {
	c := f.initializer
	if f.platform.ID == osReleaseIDUbuntu {
		path := filepath.Join(t.TempDir(), "apt.conf")
		require.NoError(t, os.WriteFile(path, []byte(`APT::Update::Error-Mode "any"; Acquire::Retries "0";`), 0o600))
		t.Setenv("APT_CONFIG", path)
	} else {
		_, err := c.command(f.ctx, nil, "dnf", "clean", "expire-cache")
		require.NoError(t, err)
	}
	f.proxy.SetMetadataFailure(true)
	defer f.proxy.SetMetadataFailure(false)
	waits := 0
	c.wait = func(ctx context.Context, delay time.Duration) error {
		waits++
		return waitCloudRetry(ctx, delay)
	}
	defer func() { c.wait = waitCloudRetry }()
	require.NoError(t, c.initialize(f.ctx, f.config))
	require.Equal(t, cloudRequestAttempts-1, waits, "all ten real cache attempts must run")
	events, err := os.ReadDir(f.eventDir)
	require.NoError(t, err)
	found := false
	for _, event := range events {
		data, err := os.ReadFile(filepath.Join(f.eventDir, event.Name()))
		require.NoError(t, err)
		if strings.Contains(string(data), "CloudRepositoryInitialization") &&
			strings.Contains(string(data), "refresh RepoDepot package cache") {
			found = true
		}
	}
	require.True(t, found, "cache failure must retain its repository error event")
	errorEvent := false
	for _, event := range c.eventLogger.Events() {
		if event.TaskName == "AKS.AKSNodeController.CloudRepositoryInitialization" && event.EventLevel == "Error" {
			errorEvent = true
		}
	}
	require.True(t, errorEvent, "repository exhaustion must be Error telemetry, not a successful cache refresh")
}

func (f *cloudSystemFixture) certificateFatalBoundaries(t *testing.T) {
	before := f.proxy.Stats()
	f.profile.Store(&cloudSystemWireProfile{})
	defer f.profile.Store(f.normal)
	err := f.initializer.initialize(f.ctx, f.config)
	require.ErrorContains(t, err, "opt-in response is empty")
	f.profile.Store(&cloudSystemWireProfile{optIn: f.normal.optIn, missingOperations: true})
	err = f.initializer.initialize(f.ctx, f.config)
	require.ErrorContains(t, err, "contains no certificate filenames")
	require.Equal(t, before, f.proxy.Stats(), "fatal certificates must stop before any repository requests")
}

func (f *cloudSystemFixture) invalidOptInSkipsInstallation(t *testing.T) {
	defer f.profile.Store(f.normal)
	for _, response := range []string{"not-json", "{}", `{"IsOptedInForRootCerts":"true"}`} {
		f.profile.Store(&cloudSystemWireProfile{optIn: response})
		eligible, err := f.initializer.refresh(f.ctx, f.platform, f.config.Location)
		require.NoError(t, err)
		require.False(t, eligible)
	}
}

func (f *cloudSystemFixture) legacyTrustPermissionFailureContinues(t *testing.T) {
	c := f.initializer
	denied := false
	c.run = func(cmd *exec.Cmd) error {
		if filepath.Base(cmd.Path) == f.trustCommand {
			denied = true
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
		}
		return cmdRunner(cmd)
	}
	defer func() { c.run = nil }()
	require.NoError(t, c.initialize(f.ctx, cloudInitConfig{Location: "ussec-test", Timeout: time.Minute}))
	require.True(t, denied, "the real trust tool must reject an unprivileged update")
}

func (f *cloudSystemFixture) cancellationIsFatal(t *testing.T) {
	cancelled, cancel := context.WithCancel(f.ctx)
	cancel()
	require.ErrorIs(t, f.initializer.initialize(cancelled, f.config), context.Canceled)
	blocked := &cloudSystemWireProfile{optIn: f.normal.optIn, blockOptIn: true}
	f.profile.Store(blocked)
	defer f.profile.Store(f.normal)
	timed, stop := context.WithTimeout(f.ctx, 200*time.Millisecond)
	defer stop()
	require.ErrorIs(t, f.initializer.initialize(timed, f.config), context.DeadlineExceeded)
	require.True(t, blocked.optInStarted.Load(), "deadline must interrupt an actual blocked opt-in HTTP request")
}

func (f *cloudSystemFixture) systemdRefreshAndPermissionFailure(t *testing.T) {
	c := f.initializer
	paths := []string{"/etc/systemd/system/azure-ca-refresh.service", "/etc/systemd/system/azure-ca-refresh.timer"}
	for _, path := range paths {
		_, err := os.Lstat(path)
		require.ErrorIs(t, err, os.ErrNotExist, "mutable-OS test must not replace an existing systemd refresh")
	}
	t.Cleanup(func() {
		_, err := c.command(context.Background(), nil, "systemctl", "disable", "--now", "azure-ca-refresh.timer")
		require.NoError(t, err)
		for _, path := range paths {
			require.NoError(t, os.Remove(path))
		}
		_, err = c.command(context.Background(), nil, "systemctl", "daemon-reload")
		require.NoError(t, err)
	})
	command := strconv.Quote(*cloudIntegrationBinary) + " refresh-cloud-certificates --location " + strconv.Quote(f.config.Location)
	require.NoError(t, c.installSystemdRefresh(f.ctx, command))
	for _, args := range [][]string{
		{"is-enabled", "azure-ca-refresh.timer"},
		{"is-active", "azure-ca-refresh.timer"},
		{"start", "azure-ca-refresh.service"},
	} {
		_, err := c.command(f.ctx, nil, "systemctl", args...)
		require.NoError(t, err)
	}
	c.run = func(cmd *exec.Cmd) error {
		if filepath.Base(cmd.Path) == "systemctl" {
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
		}
		return cmdRunner(cmd)
	}
	defer func() { c.run = nil }()
	require.Error(t, c.installSystemdRefresh(f.ctx, command), "a real daemon authorization failure must remain fatal")
}

func newCloudSystemCertificate(t *testing.T) ([]byte, tls.Certificate, string) {
	t.Helper()
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	name := "anc-cloud-integration-" + serial.Text(16)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	ca := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, public, private)
	require.NoError(t, err)
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, public, private)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		tls.Certificate{Certificate: [][]byte{leafDER, caDER}, PrivateKey: private}, name
}

func preserveCloudSystemCrontab(t *testing.T, c *cloudInitializer, ctx context.Context) {
	t.Helper()
	original, err := c.command(ctx, nil, "crontab", "-l")
	absent := false
	if err != nil {
		var exit interface{ ExitCode() int }
		require.ErrorAs(t, err, &exit)
		require.Equal(t, 1, exit.ExitCode())
		require.Contains(t, err.Error(), "no crontab for")
		absent = true
	}
	t.Cleanup(func() {
		var restoreErr error
		if absent {
			_, restoreErr = c.command(context.Background(), nil, "crontab", "-r")
		} else {
			_, restoreErr = c.command(context.Background(), original, "crontab", "-")
		}
		require.NoError(t, restoreErr)
	})
	_, err = c.command(ctx, append(original, []byte("\n# anc-cloud-system-integration-unrelated\n")...), "crontab", "-")
	require.NoError(t, err)
}

func preserveCloudSystemRepositories(t *testing.T, platform cloudPlatform) {
	t.Helper()
	paths := cloudSystemRepositoryPaths(t, platform)
	saved := make(map[string][]byte)
	modes := make(map[string]os.FileMode)
	for _, path := range paths {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			saved[path] = nil
			continue
		}
		require.NoError(t, err)
		require.True(t, info.Mode().IsRegular(), fmt.Sprintf("unexpected non-file repository path %s", path))
		saved[path], err = os.ReadFile(path)
		require.NoError(t, err)
		modes[path] = info.Mode().Perm()
	}
	t.Cleanup(func() {
		for path, data := range saved {
			if mode, existed := modes[path]; existed {
				require.NoError(t, os.WriteFile(path, data, mode))
				require.NoError(t, os.Chmod(path, mode))
			} else {
				err := os.Remove(path)
				if !errors.Is(err, os.ErrNotExist) {
					require.NoError(t, err)
				}
			}
		}
	})
}

func cloudSystemRepositoryPaths(t *testing.T, platform cloudPlatform) []string {
	t.Helper()
	var paths []string
	if platform.ID == osReleaseIDUbuntu {
		paths = []string{"/etc/apt/sources.list", "/etc/apt/backup/sources.list"}
		for _, directory := range []string{"/etc/apt/sources.list.d", "/etc/apt/backup"} {
			matches, err := filepath.Glob(directory + "/*")
			require.NoError(t, err)
			paths = append(paths, matches...)
			for _, name := range []string{"ubuntu.sources", "microsoft-prod.sources", "microsoft-prod-testing.sources"} {
				paths = append(paths, filepath.Join(directory, name))
			}
			if directory == "/etc/apt/sources.list.d" {
				for _, path := range matches {
					paths = append(paths, "/etc/apt/backup/"+filepath.Base(path))
				}
			}
		}
		for _, key := range []string{"microsoft.asc", "msopentech.asc"} {
			paths = append(paths, "/etc/apt/keyrings/"+key+".gpg")
		}
	} else {
		var err error
		paths, err = filepath.Glob("/etc/yum.repos.d/azurelinux*")
		require.NoError(t, err)
		for _, repo := range []string{"amd", "base", "cloud-native", "extended", "ms-non-oss", "ms-oss", "nvidia"} {
			paths = append(paths, "/etc/yum.repos.d/azurelinux-"+repo+".repo")
		}
	}
	return paths
}
