package main

import (
	"context"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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

func TestCloudCertificateTruncatedHTTP200(t *testing.T) {
	complete := cloudTestCertificate(t)
	for _, tc := range []struct {
		name     string
		failures int32
	}{
		{"recover on retry", 1},
		{"exhaust retries", cloudRequestAttempts},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCloudInitializer(t, cloudTestUbuntu)
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempt := attempts.Add(1)
				w.Header().Set("Content-Length", strconv.Itoa(len(complete)))
				w.WriteHeader(http.StatusOK)
				if attempt <= tc.failures {
					// Returning fewer bytes than Content-Length closes the response early.
					_, _ = w.Write(complete[:len(complete)/2])
					return
				}
				_, _ = w.Write(complete)
			}))
			defer server.Close()
			c.wireEndpoint = server.URL
			waits := 0
			c.wait = func(ctx context.Context, _ time.Duration) error {
				waits++
				return ctx.Err()
			}

			body, err := c.requestCertificates(t.Context(), certificatePackagePath("root", "crt"))
			wantAttempts := tc.failures + 1
			if tc.failures == cloudRequestAttempts {
				wantAttempts = cloudRequestAttempts
				require.ErrorIs(t, err, io.ErrUnexpectedEOF)
				require.Nil(t, body)
			} else {
				require.NoError(t, err)
				require.Equal(t, complete, body)
			}
			require.Equal(t, wantAttempts, attempts.Load())
			require.EqualValues(t, wantAttempts-1, waits)
		})
	}
}

func TestCloudOperationCertificateTruncatedDownload(t *testing.T) {
	complete := cloudTestCertificate(t)
	for _, failures := range []int32{1, cloudRequestAttempts} {
		t.Run(strconv.Itoa(int(failures)), func(t *testing.T) {
			c := newTestCloudInitializer(t, cloudTestUbuntu)
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.RequestURI {
				case "/machine?comp=acmspackage&type=operationrequestsroot&ext=json":
					_, _ = io.WriteString(w, `{"OperationsInfo":[{"ResouceFileName":"truncated.crt"},{"ResouceFileName":"complete.crt"}]}`)
				case "/machine?comp=acmspackage&type=truncated&ext=crt":
					attempt := attempts.Add(1)
					w.Header().Set("Content-Length", strconv.Itoa(len(complete)))
					w.WriteHeader(http.StatusOK)
					if attempt <= failures {
						_, _ = w.Write(complete[:len(complete)/2])
						return
					}
					_, _ = w.Write(complete)
				case "/machine?comp=acmspackage&type=complete&ext=crt":
					_, _ = w.Write(complete)
				default:
					t.Errorf("unexpected certificate request: %s", r.RequestURI)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			c.wireEndpoint = server.URL

			certificates, err := c.operationCertificateGroup(t.Context(), "operationrequestsroot")
			require.NoError(t, err)
			want := map[string][]byte{"complete.crt": complete}
			wantAttempts := failures + 1
			if failures == cloudRequestAttempts {
				wantAttempts = cloudRequestAttempts
				_, statErr := os.Stat(c.path("/root/AzureCACertificates/truncated.crt"))
				require.ErrorIs(t, statErr, os.ErrNotExist)
			} else {
				want["truncated.crt"] = complete
			}
			require.Equal(t, wantAttempts, attempts.Load())
			require.Equal(t, want, certificates)
			for name, payload := range want {
				cached, readErr := os.ReadFile(c.path("/root/AzureCACertificates/" + name))
				require.NoError(t, readErr)
				require.Equal(t, payload, cached)
			}
		})
	}
}

func TestCloudCertificateCloseDelimitedResponse(t *testing.T) {
	complete := cloudTestCertificate(t)
	partial := complete[:len(complete)/2]
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("test server does not support connection hijacking")
			return
		}
		conn, writer, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("hijack test connection: %v", err)
			return
		}
		defer conn.Close()
		if _, err := writer.WriteString("HTTP/1.1 200 OK\r\nConnection: close\r\n\r\n" + string(partial)); err != nil {
			t.Errorf("write close-delimited response: %v", err)
			return
		}
		if err := writer.Flush(); err != nil {
			t.Errorf("flush close-delimited response: %v", err)
		}
	}))
	defer server.Close()
	c.wireEndpoint = server.URL

	body, err := c.requestCertificates(t.Context(), certificatePackagePath("root", "crt"))
	require.NoError(t, err)
	require.Equal(t, partial, body)
	require.EqualValues(t, 1, attempts.Load())
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
