package apiserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetNodeBootstrapDataRejectsWhenOverloaded(t *testing.T) {
	api, err := NewAPIServer(&Options{Addr: ":8080"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < cap(api.nodeBootstrapLimiter); i++ {
		api.nodeBootstrapLimiter <- struct{}{}
	}

	request := httptest.NewRequest(http.MethodPost, RoutePathNodeBootstrapData, strings.NewReader("{}"))
	response := httptest.NewRecorder()
	api.GetNodeBootstrapData(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, response.Code)
	}
	if response.Header().Get("Retry-After") != api.overloadRetryAfterSeconds {
		t.Fatalf("expected Retry-After %s, got %s", api.overloadRetryAfterSeconds, response.Header().Get("Retry-After"))
	}
}

func TestGetNodeBootstrapDataReleasesLimiter(t *testing.T) {
	api, err := NewAPIServer(&Options{Addr: ":8080"})
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, RoutePathNodeBootstrapData, strings.NewReader("{"))
	response := httptest.NewRecorder()
	api.GetNodeBootstrapData(response, request)

	if len(api.nodeBootstrapLimiter) != 0 {
		t.Fatal("expected bootstrap limiter slot to be released")
	}
}

func TestNewAPIServerUsesOverloadEnvironmentVariables(t *testing.T) {
	t.Setenv(maxConcurrentNodeBootstrapRequestsEnv, "40")
	t.Setenv(overloadRetryAfterSecondsEnv, "5")

	api, err := NewAPIServer(&Options{Addr: ":8080"})
	if err != nil {
		t.Fatal(err)
	}

	if cap(api.nodeBootstrapLimiter) != 40 {
		t.Fatalf("expected limiter capacity 40, got %d", cap(api.nodeBootstrapLimiter))
	}
	if api.overloadRetryAfterSeconds != "5" {
		t.Fatalf("expected Retry-After 5, got %s", api.overloadRetryAfterSeconds)
	}
}

func TestNewAPIServerRejectsInvalidOverloadEnvironmentVariables(t *testing.T) {
	t.Setenv(maxConcurrentNodeBootstrapRequestsEnv, "invalid")

	if _, err := NewAPIServer(&Options{Addr: ":8080"}); err == nil {
		t.Fatal("expected invalid limiter capacity to fail")
	}
}

func TestNewAPIServerRejectsPProfOnAPIAddress(t *testing.T) {
	if _, err := NewAPIServer(&Options{Addr: ":8080", PProfAddr: ":8080"}); err == nil {
		t.Fatal("expected matching API and pprof addresses to fail")
	}
}

func TestPProfHandlerIsSeparateFromAPIRouter(t *testing.T) {
	api, err := NewAPIServer(&Options{Addr: ":8080"})
	if err != nil {
		t.Fatal(err)
	}

	apiRequest := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	apiResponse := httptest.NewRecorder()
	api.NewRouter().ServeHTTP(apiResponse, apiRequest)
	if apiResponse.Code != http.StatusNotFound {
		t.Fatalf("expected API router status 404, got %d", apiResponse.Code)
	}

	profileRequest := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	profileResponse := httptest.NewRecorder()
	newPProfHandler().ServeHTTP(profileResponse, profileRequest)
	if profileResponse.Code != http.StatusOK {
		t.Fatalf("expected pprof router status 200, got %d", profileResponse.Code)
	}
}
