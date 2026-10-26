package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/stretchr/testify/require"
)

const cloudTestUbuntu = "ID=ubuntu\nNAME=Ubuntu\nVERSION_ID=22.04\nVERSION_CODENAME=jammy\n"

func newTestCloudInitializer(t *testing.T, release string) *cloudInitializer {
	t.Helper()
	c := newCloudInitializer(&App{
		cmdRun: func(cmd *exec.Cmd) error {
			if filepath.Base(cmd.Path) == "gpg" {
				_, _ = io.WriteString(cmd.Stdout, "dearmored-key")
			}
			return nil
		},
		eventLogger: helpers.NewEventLogger(t.TempDir()),
	})
	c.rootDir = t.TempDir()
	c.executable = hotfixBinaryPath
	c.wait = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	require.NoError(t, c.writeFile(defaultOSReleasePath, []byte(release)))
	require.NoError(t, c.writeFile("/etc/ssl/certs/ca-certificates.crt", []byte("system-ca-bundle")))
	t.Cleanup(c.close)
	return c
}

func cloudTestCertificate(t *testing.T) []byte {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ANC cloud initialization test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func serveCloudTestResponses(t *testing.T, c *cloudInitializer, cert []byte, optIn string) <-chan string {
	t.Helper()
	requests := make(chan string, 100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Path + "?" + r.URL.RawQuery
		switch kind := r.URL.Query().Get("type"); {
		case r.URL.Path == "/acms/isOptedInForRootCerts":
			_, _ = io.WriteString(w, optIn)
		case kind == "cacertificates":
			_ = json.NewEncoder(w).Encode(map[string]any{"Certificates": []map[string]string{
				{"Name": "root.cer", "CertBody": strings.ReplaceAll(string(cert), "\n", "\r\n")},
			}})
		case kind == "operationrequestsroot", kind == "operationrequestsintermediate":
			name := "root.crt"
			if kind == "operationrequestsintermediate" {
				name = "intermediate.crt"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"OperationsInfo": []map[string]string{{"ResouceFileName": name}}})
		case kind == "root", kind == "intermediate":
			_, _ = w.Write(cert)
		case strings.HasPrefix(r.URL.Path, "/keys/"):
			_, _ = io.WriteString(w, "armored-public-package-key")
		case strings.HasPrefix(r.URL.Path, "/microsoft/ubuntu/"):
			_, _ = io.WriteString(w, "repository available")
		default:
			t.Errorf("unexpected cloud request: %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	c.wireEndpoint = server.URL
	c.wireClient, c.repoClient = server.Client(), server.Client()
	return requests
}

func serveCloudOperationTestResponses(t *testing.T, c *cloudInitializer, roots, intermediates []string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch kind := r.URL.Query().Get("type"); {
		case r.URL.Path == "/acms/isOptedInForRootCerts":
			_, _ = io.WriteString(w, `{"IsOptedInForRootCerts":true}`)
		case kind == "operationrequestsroot", kind == "operationrequestsintermediate":
			names := roots
			if kind == "operationrequestsintermediate" {
				names = intermediates
			}
			operations := make([]map[string]string, 0, len(names))
			for _, name := range names {
				operations = append(operations, map[string]string{"ResouceFileName": name})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"OperationsInfo": operations})
		case kind == "missing":
			w.WriteHeader(http.StatusServiceUnavailable)
		case kind == "empty":
			_, _ = io.WriteString(w, "\n\n")
		default:
			_, _ = io.WriteString(w, "certificate payload\n\n")
		}
	}))
	t.Cleanup(server.Close)
	c.wireEndpoint = server.URL
}

func TestCloudFileWrites(t *testing.T) {
	t.Run("create and replace", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "certificates", "root.crt")
		require.NoError(t, writeCloudFile(path, []byte("original")))
		require.NoError(t, writeCloudFile(path, []byte("replacement")))
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "replacement", string(data))
	})
	t.Run("reject symlinks outside the destination directory", func(t *testing.T) {
		directory := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside.crt")
		require.NoError(t, os.WriteFile(outside, []byte("original"), 0o600))
		path := filepath.Join(directory, "root.crt")
		require.NoError(t, os.Symlink(outside, path))
		require.Error(t, writeCloudFile(path, []byte("replacement")))
		data, err := os.ReadFile(outside)
		require.NoError(t, err)
		require.Equal(t, "original", string(data))
	})
}

func TestCloudPlatform(t *testing.T) {
	for _, tc := range []struct {
		name, release, want string
	}{
		{"Ubuntu", cloudTestUbuntu, osReleaseIDUbuntu},
		{"Mariner", "ID=mariner\nNAME=CBL-Mariner\nVERSION_ID=2.0\n", "mariner"},
		{"AzureLinux", "ID=azurelinux\nNAME=\"Microsoft Azure Linux\"\nVERSION_ID=3.0\n", osReleaseIDAzureLinux},
		{"OSGuard", "ID=azurelinux\nVARIANT_ID=osguard\nVERSION_ID=3.0\n", osReleaseIDAzureLinux},
		{"Flatcar", "ID=flatcar\n", osReleaseIDFlatcar},
		{"ACL", "ID=azurecontainerlinux\n", osReleaseIDAzureContainerLinux},
		{"ACL variant", "ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n", osReleaseIDAzureContainerLinux},
		{"unsupported", "ID=debian\n", ""},
		{"missing ID", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCloudInitializer(t, tc.release)
			platform, err := c.platform()
			if tc.want == "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, platform.ID)
			}
		})
	}
}

func TestNormalizeCloudLocation(t *testing.T) {
	location, err := normalizeCloudLocation(" US Sec East \t\n")
	require.NoError(t, err)
	require.Equal(t, "usseceast", location)
	for _, value := range []string{`westus";echo bad`, "westus%bad", "westus\\bad"} {
		_, err = normalizeCloudLocation(value)
		require.Error(t, err)
	}
}

func TestCloudInitializationCertificateModes(t *testing.T) {
	cert := cloudTestCertificate(t)
	for _, osCase := range []struct{ name, release, trustDir, update string }{
		{"Ubuntu", cloudTestUbuntu, "/usr/local/share/ca-certificates", "update-ca-certificates"},
		{"Mariner", "ID=mariner\n", "/etc/pki/ca-trust/source/anchors", "update-ca-trust"},
		{"AzureLinux", "ID=azurelinux\n", "/etc/pki/ca-trust/source/anchors", "update-ca-trust"},
		{"Flatcar", "ID=flatcar\n", "/etc/ssl/certs", "update-ca-certificates"},
		{"ACL", "ID=azurecontainerlinux\n", "/etc/pki/ca-trust/source/anchors", "update-ca-trust"},
	} {
		for _, mode := range []struct {
			name, location, optIn string
			eligible              bool
		}{
			{"not opted in", "westus2", `{"IsOptedInForRootCerts":false}`, false},
			{"rcv1p", "westus2", `{"IsOptedInForRootCerts":true}`, true},
			{"legacy", " US NAT East ", "not used", true},
		} {
			t.Run(osCase.name+"/"+mode.name, func(t *testing.T) {
				c := newTestCloudInitializer(t, osCase.release)
				requests := serveCloudTestResponses(t, c, cert, mode.optIn)
				var commands []string
				run := c.run
				c.run = func(cmd *exec.Cmd) error {
					commands = append(commands, strings.Join(cmd.Args, " "))
					return run(cmd)
				}
				err := c.initialize(t.Context(), cloudInitConfig{Location: mode.location, Timeout: time.Minute})
				require.NoError(t, err)
				if !mode.eligible {
					require.Empty(t, commands)
					require.Len(t, requests, 1)
					return
				}
				require.Equal(t, osCase.update, commands[0])
				filename := "root.crt"
				if osCase.name == "Flatcar" {
					filename = "root.pem"
				}
				got, err := os.ReadFile(c.path(filepath.Join(osCase.trustDir, filename)))
				require.NoError(t, err)
				want := cert
				if mode.name == "legacy" {
					want = []byte(strings.TrimSuffix(string(cert), "\n") + "\r\n")
				}
				require.Equal(t, want, got)
				assertCloudSchedule(t, c, osCase.name, commands)
				if mode.name == "legacy" {
					require.Len(t, requests, 1)
				} else {
					require.Len(t, requests, 5)
				}
			})
		}
	}
}

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

func TestCloudCertificateOptInNonTrueResponsesSkip(t *testing.T) {
	for _, response := range []string{
		"not json", " ", `{}`, `null`, `[]`, `{"IsOptedInForRootCerts":false}`,
		`{"IsOptedInForRootCerts":null}`, `{"IsOptedInForRootCerts":"false"}`,
		`{"IsOptedInForRootCerts":"true"}`, `{"IsOptedInForRootCerts":1}`,
		`{"isOptedInForRootCerts":true}`,
	} {
		t.Run(response, func(t *testing.T) {
			c := newTestCloudInitializer(t, cloudTestUbuntu)
			requests := serveCloudTestResponses(t, c, nil, response)
			c.run = func(*exec.Cmd) error {
				t.Fatal("must not update trust or schedules without explicit opt-in")
				return nil
			}
			require.NoError(t, c.initialize(t.Context(), cloudInitConfig{Location: "westus2", Timeout: time.Minute}))
			require.Len(t, requests, 1)
			events := c.eventLogger.Events()
			require.NotEmpty(t, events)
			require.Equal(t, string(helpers.EventLevelInformational), events[len(events)-1].EventLevel)
		})
	}
}

func TestCloudCertificateOptInEmptyResponsesFail(t *testing.T) {
	for _, response := range []string{"", "\n", "\n\n"} {
		t.Run(fmt.Sprintf("%q", response), func(t *testing.T) {
			c := newTestCloudInitializer(t, cloudTestUbuntu)
			serveCloudTestResponses(t, c, nil, response)
			require.ErrorContains(t, c.refreshCertificates(t.Context(), "westus2"), "response is empty")
		})
	}
}

func TestCloudCertificateRefreshClearsCachedFiles(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	serveCloudTestResponses(t, c, nil, `{"IsOptedInForRootCerts":false}`)
	for _, name := range []string{"stale.crt", ".stale.crt", "certificate-without-extension"} {
		require.NoError(t, c.writeFile("/root/AzureCACertificates/"+name, []byte("stale")))
	}
	require.NoError(t, c.writeFile("/usr/local/share/ca-certificates/existing.crt", []byte("keep")))
	require.NoError(t, c.refreshCertificates(t.Context(), "westus2"))
	entries, err := os.ReadDir(c.path("/root/AzureCACertificates"))
	require.NoError(t, err)
	require.Empty(t, entries)
	data, err := os.ReadFile(c.path("/usr/local/share/ca-certificates/existing.crt"))
	require.NoError(t, err)
	require.Equal(t, "keep", string(data))
}

func TestCloudCertificatePayloadsArePassedToTrustStoreTools(t *testing.T) {
	for _, tc := range []struct{ name, location, want string }{
		{"legacy", "usnateast", "not a PEM certificate\r\n"},
		{"rcv1p", "westus2", "not a PEM certificate\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AKS_CLOUD_LOG_CERTIFICATES", "true")
			logs := installLogCapturer(t)
			c := newTestCloudInitializer(t, cloudTestUbuntu)
			serveCloudTestResponses(t, c, []byte("not a PEM certificate\n"), `{"IsOptedInForRootCerts":true}`)
			var commands []string
			c.run = func(cmd *exec.Cmd) error {
				commands = append(commands, cmd.Args[0])
				return nil
			}
			require.NoError(t, c.refreshCertificates(t.Context(), tc.location))
			got, err := os.ReadFile(c.path("/usr/local/share/ca-certificates/root.crt"))
			require.NoError(t, err)
			require.Equal(t, tc.want, string(got))
			require.Equal(t, []string{"update-ca-certificates"}, commands)
			for _, record := range logs.getRecords() {
				for _, value := range record.Attrs {
					require.NotContains(t, value, "not a PEM certificate")
				}
			}
		})
	}
}

func TestCloudCertificateOperationDownloadFailures(t *testing.T) {
	for _, tc := range []struct {
		name          string
		roots         []string
		intermediates []string
		wantInstalled []string
		wantError     bool
	}{
		{"partial group", []string{"missing.crt", "root.crt"}, []string{"intermediate.crt"}, []string{"root.crt", "intermediate.crt"}, false},
		{"empty root group after failures", []string{"missing.crt"}, []string{"intermediate.crt"}, []string{"intermediate.crt"}, false},
		{"empty certificate response", []string{"empty.crt"}, []string{"intermediate.crt"}, []string{"intermediate.crt"}, false},
		{"non-crt root", []string{"root.cer"}, []string{"intermediate.crt"}, []string{"intermediate.crt"}, false},
		{"all downloads fail", []string{"missing.crt"}, []string{"missing.crt"}, nil, true},
		{"no crt files", []string{"root.cer"}, []string{"intermediate.cer"}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCloudInitializer(t, "ID=azurelinux\n")
			serveCloudOperationTestResponses(t, c, tc.roots, tc.intermediates)
			var commands []string
			c.run = func(cmd *exec.Cmd) error {
				commands = append(commands, cmd.Args[0])
				return nil
			}
			err := c.refreshCertificates(t.Context(), "westus2")
			if tc.wantError {
				require.ErrorContains(t, err, "no *.crt files")
				require.Empty(t, commands)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []string{"update-ca-trust"}, commands)
			for _, name := range tc.wantInstalled {
				data, readErr := os.ReadFile(c.path("/etc/pki/ca-trust/source/anchors/" + name))
				require.NoError(t, readErr)
				require.Equal(t, "certificate payload\n", string(data))
			}
			for _, name := range []string{"missing.crt", "empty.crt", "root.cer"} {
				_, err = os.Stat(c.path("/etc/pki/ca-trust/source/anchors/" + name))
				require.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}

func TestCloudLegacyCertificateResponseBehavior(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		wantError      bool
	}{
		{"empty array", `{"Certificates":[]}`, false},
		{"non-crt file", `{"Certificates":[{"Name":"root.txt","CertBody":"payload"}]}`, false},
		{"empty body", `{"Certificates":[{"Name":"root.cer","CertBody":""}]}`, false},
		{"trust update failure", `{"Certificates":[{"Name":"root.cer","CertBody":"certificate payload"}]}`, false},
		{"invalid name", `{"Certificates":[{"Name":"../root.crt"}]}`, false},
		{"non-string name", `{"Certificates":[{"Name":123}]}`, false},
		{"missing array", `{}`, true},
		{"null array", `{"Certificates":null}`, true},
		{"missing body", `{"Certificates":[{"Name":"root.cer"}]}`, true},
		{"null body", `{"Certificates":[{"Name":"root.cer","CertBody":null}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCloudInitializer(t, "ID=azurelinux\n")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.response)
			}))
			defer server.Close()
			c.wireEndpoint = server.URL
			var commands []string
			c.run = func(cmd *exec.Cmd) error {
				commands = append(commands, cmd.Args[0])
				if cmd.Args[0] == "update-ca-trust" {
					return errors.New("trust store update failed")
				}
				return nil
			}
			err := c.initialize(t.Context(), cloudInitConfig{Location: "usnateast", Timeout: time.Minute})
			if tc.wantError {
				require.Error(t, err)
				require.Empty(t, commands)
			} else {
				require.NoError(t, err)
				require.Contains(t, commands, "crontab")
			}
		})
	}
}

func TestCloudLegacyCertificateInstallationDoesNotIgnoreCancellation(t *testing.T) {
	c := newTestCloudInitializer(t, "ID=azurelinux\n")
	serveCloudTestResponses(t, c, cloudTestCertificate(t), "not used")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	c.run = func(*exec.Cmd) error {
		cancel()
		return ctx.Err()
	}
	require.ErrorIs(t, c.refreshCertificates(ctx, "usnateast"), context.Canceled)
}

func TestCloudLegacyCertificateCacheWriteFailureStopsInitialization(t *testing.T) {
	c := newTestCloudInitializer(t, "ID=azurelinux\n")
	serveCloudTestResponses(t, c, cloudTestCertificate(t), "not used")
	require.NoError(t, c.writeFile("/root/AzureCACertificates/root.crt/keep", []byte("keep")))
	c.run = func(*exec.Cmd) error {
		t.Fatal("must not install certificates or schedules after a cache write failure")
		return nil
	}
	err := c.initialize(t.Context(), cloudInitConfig{Location: "usnateast", Timeout: time.Minute})
	require.ErrorContains(t, err, "cache legacy Azure cloud certificate")
}

func TestCloudCertificateRequestRetryAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		failures  int32
		wantWaits []time.Duration
	}{
		{"transient", http.StatusServiceUnavailable, "do-not-log-body", 2, []time.Duration{3 * time.Second, 3 * time.Second}},
		{"rate limited", http.StatusForbidden, "RequestRateLimitExceeded", 2, []time.Duration{3 * time.Second, 6 * time.Second}},
		{"exhausted", http.StatusServiceUnavailable, "do-not-log-body", cloudRequestAttempts, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCloudInitializer(t, cloudTestUbuntu)
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if attempts.Add(1) <= tc.failures {
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.body)
					return
				}
				_, _ = io.WriteString(w, "success")
			}))
			defer server.Close()
			c.wireEndpoint = server.URL
			var waits []time.Duration
			c.wait = func(_ context.Context, delay time.Duration) error {
				waits = append(waits, delay)
				return nil
			}
			body, err := c.requestCertificates(t.Context(), "/test")
			if tc.failures == cloudRequestAttempts {
				require.Error(t, err)
				require.NotContains(t, err.Error(), tc.body)
				require.EqualValues(t, cloudRequestAttempts, attempts.Load())
				require.Len(t, waits, cloudRequestAttempts-1)
			} else {
				require.NoError(t, err)
				require.Equal(t, "success", string(body))
				require.Equal(t, tc.wantWaits, waits)
			}
		})
	}
	t.Run("cancellation during backoff", func(t *testing.T) {
		c := newTestCloudInitializer(t, cloudTestUbuntu)
		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()
		c.wireEndpoint = server.URL
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		c.wait = func(ctx context.Context, _ time.Duration) error {
			cancel()
			return ctx.Err()
		}
		_, err := c.requestCertificates(ctx, "/test")
		require.ErrorIs(t, err, context.Canceled)
		require.EqualValues(t, 1, attempts.Load())
	})
}

func TestCloudCertificateRequestsDoNotFollowRedirects(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()
	c.wireEndpoint = source.URL
	_, err := c.requestCertificates(t.Context(), "/test")
	require.ErrorContains(t, err, "HTTP 302")
	require.Zero(t, redirected.Load())
}

func TestCloudInitializationHonorsConfiguredTimeout(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	c.wireEndpoint = server.URL
	start := time.Now()
	err := c.initialize(t.Context(), cloudInitConfig{Location: "westus2", Timeout: 25 * time.Millisecond})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), time.Second)
}

func TestCloudCertificateResourceNamesAreURLEncoded(t *testing.T) {
	c := newTestCloudInitializer(t, "ID=azurelinux\n")
	cert := cloudTestCertificate(t)
	var fetched atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch kind := r.URL.Query().Get("type"); {
		case r.URL.Path == "/acms/isOptedInForRootCerts":
			_, _ = io.WriteString(w, `{"IsOptedInForRootCerts":true}`)
		case kind == "operationrequestsroot":
			_, _ = io.WriteString(w, `{"OperationsInfo":[{"ResouceFileName":"root+ca.crt"}]}`)
		case kind == "operationrequestsintermediate":
			_, _ = io.WriteString(w, `{"OperationsInfo":[{"ResouceFileName":"intermediate.crt"}]}`)
		case kind == "root+ca", kind == "intermediate":
			fetched.Store(true)
			_, _ = w.Write(cert)
		default:
			t.Errorf("incorrectly encoded certificate resource: %s", r.URL)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	c.wireEndpoint = server.URL
	require.NoError(t, c.refreshCertificates(t.Context(), "westus2"))
	require.True(t, fetched.Load())
	installed, err := os.ReadFile(c.path("/etc/pki/ca-trust/source/anchors/root+ca.crt"))
	require.NoError(t, err)
	require.Equal(t, cert, installed)
}

func TestCloudCertificateResponseLimits(t *testing.T) {
	_, err := readCloudResponse(&http.Response{
		Body: io.NopCloser(strings.NewReader(strings.Repeat("x", cloudResponseLimit+1))),
	})
	require.ErrorContains(t, err, "size limit")
}

func TestCloudCertificateNamesAndPublicContent(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../root.crt", "/root.crt", "root\x00.crt"} {
		require.False(t, validCloudCertificateName(name), name)
	}
	for _, name := range []string{"root+ca.crt", `..\root.crt`, "root\n.crt"} {
		require.True(t, validCloudCertificateName(name), name)
	}
	cert := cloudTestCertificate(t)
	private := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("not-a-real-private-key")})
	got, err := publicCloudCertificates(append(private, cert...))
	require.NoError(t, err)
	require.Equal(t, cert, got)
	for _, bad := range [][]byte{nil, private, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid")})} {
		_, err = publicCloudCertificates(bad)
		require.Error(t, err)
	}
}

func TestCloudCertificateLoggingIsOptInAndPublicOnly(t *testing.T) {
	for _, enabled := range []string{"false", "true"} {
		t.Run(enabled, func(t *testing.T) {
			t.Setenv("AKS_CLOUD_LOG_CERTIFICATES", enabled)
			logs := installLogCapturer(t)
			c := newTestCloudInitializer(t, "ID=azurelinux\n")
			cert := cloudTestCertificate(t)
			payload := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("not-a-real-private-key")})
			payload = append(payload, cert...)
			serveCloudTestResponses(t, c, payload, `{"IsOptedInForRootCerts":true}`)
			require.NoError(t, c.refreshCertificates(t.Context(), "westus2"))
			found := false
			for _, record := range logs.getRecords() {
				for _, value := range record.Attrs {
					require.NotContains(t, value, "PRIVATE KEY")
					if strings.Contains(value, "BEGIN CERTIFICATE") {
						found = true
					}
				}
			}
			require.Equal(t, enabled == "true", found)
			got, err := os.ReadFile(c.path("/etc/pki/ca-trust/source/anchors/root.crt"))
			require.NoError(t, err)
			require.Equal(t, payload, got)
		})
	}
}

func TestCloudCertificateInvalidOperationsDoNotInstall(t *testing.T) {
	for _, response := range []string{
		`{}`, `{"OperationsInfo":[]}`, `{"OperationsInfo":[{"ResouceFileName":"../escape.crt"}]}`,
		`{"OperationsInfo":[{"ResouceFileName":"/escape.crt"}]}`, `{"OperationsInfo":[{"ResouceFileName":"missing-extension"}]}`,
	} {
		t.Run(response, func(t *testing.T) {
			c := newTestCloudInitializer(t, "ID=azurelinux\n")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/acms/isOptedInForRootCerts" {
					_, _ = io.WriteString(w, `{"IsOptedInForRootCerts":true}`)
				} else {
					_, _ = io.WriteString(w, response)
				}
			}))
			defer server.Close()
			c.wireEndpoint = server.URL
			c.run = func(*exec.Cmd) error {
				t.Fatal("invalid operations must not install certificates")
				return nil
			}
			require.Error(t, c.refreshCertificates(t.Context(), "westus2"))
			_, err := os.Stat(c.path("/etc/pki/ca-trust/source/anchors"))
			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestCloudCertificateUbuntuBundleSymlink(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	require.NoError(t, os.MkdirAll(c.path("/usr/lib/ssl"), 0o755))
	require.NoError(t, os.Symlink(c.path("/etc/ssl/certs/ca-certificates.crt"), c.path("/usr/lib/ssl/cert.pem")))
	require.NoError(t, c.copyOpenSSLBundle())
	info, err := os.Lstat(c.path("/usr/lib/ssl/cert.pem"))
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink)
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

func TestCloudInitializationStopsOnTrustAndScheduleFailures(t *testing.T) {
	for _, tc := range []struct {
		name, location, failedCommand, wantError string
	}{
		{"trust store", "westus2", "update-ca-trust", "host operation failed"},
		{"missing location", " \t", "", "requires a location"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCloudInitializer(t, "ID=azurelinux\n")
			serveCloudTestResponses(t, c, cloudTestCertificate(t), `{"IsOptedInForRootCerts":true}`)
			c.run = func(cmd *exec.Cmd) error {
				if cmd.Args[0] == tc.failedCommand {
					return errors.New("host operation failed")
				}
				require.NotEqual(t, "dnf", cmd.Args[0])
				if tc.failedCommand == "" {
					require.NotEqual(t, "crontab", cmd.Args[0])
				}
				return nil
			}
			err := c.initialize(t.Context(), cloudInitConfig{
				Location: tc.location, RepoDepotEndpoint: "https://repo.example/ubuntu", Timeout: time.Minute,
			})
			require.ErrorContains(t, err, tc.wantError)
		})
	}
}

func TestCloudPeriodicRefreshDoesNotInitializeRepositoriesOrSchedules(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	requests := serveCloudTestResponses(t, c, cloudTestCertificate(t), `{"IsOptedInForRootCerts":true}`)
	var commands []string
	c.run = func(cmd *exec.Cmd) error {
		commands = append(commands, strings.Join(cmd.Args, " "))
		return nil
	}
	require.NoError(t, c.refreshCertificates(t.Context(), "westus2"))
	require.Equal(t, []string{"update-ca-certificates"}, commands)
	require.Len(t, requests, 5)
	_, err := os.Stat(c.path(defaultAptSourcesDir))
	require.ErrorIs(t, err, os.ErrNotExist)
	events := c.eventLogger.Events()
	require.Len(t, events, 1)
	require.Equal(t, "AKS.AKSNodeController.CloudCertificateRefresh", events[0].TaskName)
}
