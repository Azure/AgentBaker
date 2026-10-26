package main

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCloudCertificateRefreshModes(t *testing.T) {
	cert := cloudTestCertificate(t)
	for _, platform := range []struct{ name, release, directory, filename, command string }{
		{"Ubuntu", cloudTestUbuntu, "/usr/local/share/ca-certificates", "root.crt", "update-ca-certificates"},
		{"Mariner", "ID=mariner\n", "/etc/pki/ca-trust/source/anchors", "root.crt", "update-ca-trust"},
		{"AzureLinux", "ID=azurelinux\n", "/etc/pki/ca-trust/source/anchors", "root.crt", "update-ca-trust"},
		{"Flatcar", "ID=flatcar\n", "/etc/ssl/certs", "root.pem", "update-ca-certificates"},
		{"ACL", "ID=azurecontainerlinux\n", "/etc/pki/ca-trust/source/anchors", "root.crt", "update-ca-trust"},
	} {
		for _, mode := range []struct{ name, location, optIn string }{
			{"opt-out", "westus2", `{"IsOptedInForRootCerts":false}`},
			{"rcv1p", "westus2", `{"IsOptedInForRootCerts":true}`},
			{"legacy", " US NAT East ", "not used"},
		} {
			t.Run(platform.name+"/"+mode.name, func(t *testing.T) {
				c := newTestCloudInitializer(t, platform.release)
				serveCloudTestResponses(t, c, cert, mode.optIn)
				var commands []string
				c.run = func(cmd *exec.Cmd) error {
					commands = append(commands, cmd.Args[0])
					return nil
				}
				require.NoError(t, c.refreshCertificates(t.Context(), mode.location))
				if mode.name == "opt-out" {
					require.Empty(t, commands)
					return
				}
				require.Equal(t, []string{platform.command}, commands)
				installed, err := os.ReadFile(c.path(filepath.Join(platform.directory, platform.filename)))
				require.NoError(t, err)
				expected := cert
				if mode.name == "legacy" {
					expected = []byte(strings.TrimSuffix(string(cert), "\n") + "\r\n")
				}
				require.Equal(t, expected, installed)
			})
		}
	}
}

func TestCloudCertificateRefreshNonTrueOptInResponsesSkip(t *testing.T) {
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
				t.Fatal("non-true opt-in must not update the trust store")
				return nil
			}
			require.NoError(t, c.refreshCertificates(t.Context(), "westus2"))
			require.Len(t, requests, 1)
		})
	}
}

func TestCloudLegacyCertificateRefreshTrustFailureIsBestEffort(t *testing.T) {
	c := newTestCloudInitializer(t, "ID=azurelinux\n")
	serveCloudTestResponses(t, c, cloudTestCertificate(t), "not used")
	c.run = func(cmd *exec.Cmd) error {
		require.Equal(t, "update-ca-trust", cmd.Args[0])
		return errors.New("trust store update failed")
	}
	require.NoError(t, c.refreshCertificates(t.Context(), "usnateast"))
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
