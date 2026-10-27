package cloudfixture

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepositoryProxyPreservesVendorContent(t *testing.T) {
	var paths []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		require.Empty(t, r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("Cookie"))
		if r.URL.Path == "/ubuntu/missing" {
			http.NotFound(w, r)
			return
		}
		_, err := io.WriteString(w, "unchanged vendor-signed content")
		require.NoError(t, err)
	}))
	defer upstream.Close()
	target, err := url.Parse(upstream.URL)
	require.NoError(t, err)
	ubuntu := *target
	ubuntu.Path = "/ubuntu"
	proxy := &RepositoryProxy{ubuntu: &ubuntu, microsoft: target}
	server := httptest.NewServer(proxy)
	defer server.Close()
	for _, path := range []string{
		"/ubuntu/dists/jammy/InRelease",
		"/microsoft/ubuntu/22.04/prod/dists/jammy/Release",
		"/keys/microsoft.asc",
		"/azurelinux/3.0/prod/base/x86_64/repodata/repomd.xml",
	} {
		request, requestErr := http.NewRequest(http.MethodGet, server.URL+path, nil)
		require.NoError(t, requestErr)
		request.Header.Set("Authorization", "must-not-be-forwarded")
		request.Header.Set("Cookie", "must-not-be-forwarded")
		response, responseErr := server.Client().Do(request)
		require.NoError(t, responseErr)
		body, readErr := io.ReadAll(response.Body)
		require.NoError(t, readErr)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "unchanged vendor-signed content", string(body))
	}
	require.Equal(t, []string{
		"/ubuntu/dists/jammy/InRelease",
		"/ubuntu/22.04/prod/dists/jammy/Release",
		"/keys/microsoft.asc",
		"/azurelinux/3.0/prod/base/x86_64/repodata/repomd.xml",
	}, paths)
	require.Equal(t, RepositoryStats{Keys: 1, Metadata: 3}, proxy.Stats())
	response, err := server.Client().Get(server.URL + "/ubuntu/missing")
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusNotFound, response.StatusCode)
	require.Equal(t, 1, proxy.Stats().Failures)
}

func TestRepositoryProxyRejectsUnrelatedPathsAndMethods(t *testing.T) {
	proxy := NewRepositoryProxy()
	for _, test := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/arbitrary-upstream", http.StatusNotFound},
		{http.MethodPost, "/keys/microsoft.asc", http.StatusMethodNotAllowed},
		{http.MethodGet, StatsPath, http.StatusOK},
	} {
		response := httptest.NewRecorder()
		proxy.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		require.Equal(t, test.status, response.Code)
	}
	require.Equal(t, RepositoryStats{}, proxy.Stats())
}

func TestRepositoryProxyKeepsRedirectsOnFixture(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ubuntu/22.04/prod/", http.StatusMovedPermanently)
	}))
	defer upstream.Close()
	target, err := url.Parse(upstream.URL)
	require.NoError(t, err)
	proxy := &RepositoryProxy{ubuntu: target, microsoft: target}
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/microsoft/ubuntu/22.04/prod", nil))
	require.Equal(t, http.StatusMovedPermanently, response.Code)
	require.Equal(t, "/microsoft/ubuntu/22.04/prod/", response.Header().Get("Location"))
}

func TestRepositoryMetadataFailureIsExplicitAndRecoverable(t *testing.T) {
	proxy := NewRepositoryProxy()
	proxy.SetMetadataFailure(true)
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ubuntu/dists/jammy/InRelease", nil))
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Equal(t, RepositoryStats{Failures: 1}, proxy.Stats())
	proxy.SetMetadataFailure(false)
	response = httptest.NewRecorder()
	proxy.ServeHTTP(response, httptest.NewRequest(http.MethodGet, StatsPath, nil))
	require.Equal(t, http.StatusOK, response.Code)
}
