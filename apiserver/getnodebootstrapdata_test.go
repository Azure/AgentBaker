package apiserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestLogCustomDataSize(t *testing.T) {
	for _, test := range []struct {
		length int
		level  string
	}{
		{length: 78641},
		{length: 78642, level: "WARN"},
		{length: 87380, level: "WARN"},
		{length: 87381, level: "ERROR"},
	} {
		t.Run(fmt.Sprint(test.length), func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			config := &datamodel.NodeBootstrappingConfiguration{
				SubscriptionID: "test-subscription",
				AgentPoolProfile: &datamodel.AgentPoolProfile{
					Name: "test-pool", OSType: datamodel.Linux,
				},
				AKSNodeConfigJSON: `{"token":"never-log-this"}`,
			}
			logCustomDataSize(context.Background(), logger, config, test.length)
			if test.level == "" {
				require.Empty(t, output.String())
				return
			}
			var event struct {
				Level  string `json:"level"`
				Source string `json:"source"`
				Msg    string `json:"msg"`
			}
			require.NoError(t, json.Unmarshal(output.Bytes(), &event))
			require.Equal(t, test.level, event.Level)
			require.Equal(t, "AgentBakerCustomData", event.Source)
			require.Contains(t, event.Msg, `subscriptionID="test-subscription"`)
			require.True(t, strings.HasSuffix(event.Msg, fmt.Sprintf("Length is %d", test.length)))
			require.NotContains(t, output.String(), "never-log-this")
		})
	}
}

func TestGetNodeBootstrapDataLogsOversizeBeforeReturningSuccess(t *testing.T) {
	var output bytes.Buffer
	data := make([]byte, agent.MaxCustomDataLength)
	_, err := rand.Read(data)
	require.NoError(t, err)
	profile := &datamodel.AgentPoolProfile{
		Name: "test-pool", OSType: datamodel.Linux, Distro: datamodel.CustomizedImage,
	}
	config := &datamodel.NodeBootstrappingConfiguration{
		AgentPoolProfile: profile,
		ContainerService: &datamodel.ContainerService{
			Location: "eastus",
			Properties: &datamodel.Properties{
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{profile},
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    datamodel.Kubernetes,
					OrchestratorVersion: "1.32.1",
					KubernetesConfig:    &datamodel.KubernetesConfig{},
				},
				HostedMasterProfile: &datamodel.HostedMasterProfile{FQDN: "test.invalid"},
			},
		},
		CloudSpecConfig:               datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:                 &datamodel.K8sComponents{},
		EnableScriptlessAKSNodeConfig: true,
		AKSNodeConfigJSON:             `{"version":"v1"}`,
		EnabledFeatures: map[string]string{
			"TEST_SECRET": "never-log-this" + base64.StdEncoding.EncodeToString(data),
		},
	}
	body, err := json.Marshal(config)
	require.NoError(t, err)
	api, err := NewAPIServer(&Options{Addr: ":8080"})
	require.NoError(t, err)
	api.customDataLogger = slog.New(slog.NewJSONHandler(&output, nil))
	response := &customDataResponseRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		beforeHeader: func() {
			require.Contains(t, output.String(), `"level":"ERROR"`)
			require.NotContains(t, output.String(), "never-log-this")
		},
	}
	request := httptest.NewRequest(http.MethodPost, RoutePathNodeBootstrapData, bytes.NewReader(body))
	api.GetNodeBootstrapData(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	var bootstrapping datamodel.NodeBootstrapping
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &bootstrapping))
	require.Greater(t, len(bootstrapping.CustomData), agent.MaxCustomDataLength)
}

type customDataResponseRecorder struct {
	*httptest.ResponseRecorder
	beforeHeader func()
}

func (r *customDataResponseRecorder) WriteHeader(statusCode int) {
	r.beforeHeader()
	r.ResponseRecorder.WriteHeader(statusCode)
}

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
