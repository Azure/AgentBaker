package common

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBaseTransport(t *testing.T) {
	t.Run("disables proxy and sets the per-phase timeouts", func(t *testing.T) {
		tr := NewBaseTransport(HTTPTransportOptions{
			DialTimeout:           2 * time.Second,
			TLSHandshakeTimeout:   2 * time.Second,
			ResponseHeaderTimeout: 2 * time.Second,
		})
		assert.Nil(t, tr.Proxy, "proxy must be disabled for the forced-dial/link-local paths")
		assert.Equal(t, 2*time.Second, tr.TLSHandshakeTimeout)
		assert.Equal(t, 2*time.Second, tr.ResponseHeaderTimeout)
		require.NotNil(t, tr.DialContext)
	})

	t.Run("dialAddrOverride redirects every dial to the override addr", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
		defer srv.Close()

		tr := NewBaseTransport(HTTPTransportOptions{
			DialTimeout:      2 * time.Second,
			DialAddrOverride: srv.Listener.Addr().String(),
		})
		client := &http.Client{Transport: tr, Timeout: 3 * time.Second}

		// Request a host that does not resolve; the override must redirect the dial to the
		// test server (the curl --resolve equivalent used by the LPS SNI-pin path).
		resp, err := client.Get("http://this-host-does-not-resolve.invalid/")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("without an override the URL host is dialed", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}))
		defer srv.Close()

		tr := NewBaseTransport(HTTPTransportOptions{DialTimeout: 2 * time.Second})
		client := &http.Client{Transport: tr, Timeout: 3 * time.Second}

		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusTeapot, resp.StatusCode)
	})
}

func TestRetryStringFetch(t *testing.T) {
	t.Run("returns the first success without extra calls", func(t *testing.T) {
		calls := 0
		v, err := RetryStringFetch(context.Background(), 2, func(context.Context) (string, error) {
			calls++
			return "ok", nil
		})
		require.NoError(t, err)
		assert.Equal(t, "ok", v)
		assert.Equal(t, 1, calls)
	})

	t.Run("retries once then succeeds", func(t *testing.T) {
		calls := 0
		v, err := RetryStringFetch(context.Background(), 2, func(context.Context) (string, error) {
			calls++
			if calls == 1 {
				return "", errors.New("transient")
			}
			return "ok", nil
		})
		require.NoError(t, err)
		assert.Equal(t, "ok", v)
		assert.Equal(t, 2, calls)
	})

	t.Run("returns the last error after exhausting attempts", func(t *testing.T) {
		calls := 0
		_, err := RetryStringFetch(context.Background(), 2, func(context.Context) (string, error) {
			calls++
			return "", fmt.Errorf("attempt %d", calls)
		})
		require.Error(t, err)
		assert.Equal(t, 2, calls)
		assert.Contains(t, err.Error(), "attempt 2")
	})

	t.Run("stops early when the context is already done", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		calls := 0
		_, err := RetryStringFetch(ctx, 3, func(context.Context) (string, error) {
			calls++
			return "", errors.New("boom")
		})
		require.Error(t, err)
		assert.Equal(t, 1, calls, "must not retry once the context is done")
	})

	t.Run("normalizes maxAttempts below 1 to a single attempt", func(t *testing.T) {
		for _, maxAttempts := range []int{0, -1} {
			calls := 0
			_, err := RetryStringFetch(context.Background(), maxAttempts, func(context.Context) (string, error) {
				calls++
				return "", errors.New("boom")
			})
			require.Error(t, err, "maxAttempts=%d must surface a real error, not (\"\", nil)", maxAttempts)
			assert.Equal(t, 1, calls, "maxAttempts=%d must still attempt exactly once", maxAttempts)
		}
	})
}
