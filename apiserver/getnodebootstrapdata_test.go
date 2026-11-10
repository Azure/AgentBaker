package apiserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestGetNodeBootstrapDataGallerySelection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		distro      datamodel.Distro
		useEdgeZone bool
		gallery     string
		marketplace bool
		status      int
	}{
		{"regional by default", datamodel.AKSUbuntuContainerd2204Gen2, false, "regional-ubuntu", true, http.StatusOK},
		{"Extended Zone", datamodel.AKSUbuntuContainerd2204Gen2, true, datamodel.AKSUbuntuEdgeZoneGalleryName, false, http.StatusOK},
		{"legacy by default", datamodel.AKSUbuntuEdgeZoneContainerd2204Gen2, false, datamodel.AKSUbuntuEdgeZoneGalleryName, false, http.StatusOK},
		{"unsupported Extended Zone variant", datamodel.AKSUbuntuFipsContainerd2204Gen2, true, "", false, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := nodeBootstrappingConfigForTest(tc.distro)
			config.UseEdgeZoneGallery = tc.useEdgeZone
			response := serveJSONRequest(t, RoutePathNodeBootstrapData, config)
			require.Equal(t, tc.status, response.Code, response.Body.String())
			if tc.status != http.StatusOK {
				require.Contains(t, response.Body.String(), "can't find image for distro")
				return
			}

			var bootstrap datamodel.NodeBootstrapping
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &bootstrap))
			require.NotNil(t, bootstrap.SigImageConfig)
			require.Equal(t, tc.gallery, bootstrap.SigImageConfig.Gallery)
			require.Equal(t, tc.marketplace, bootstrap.OSImageConfig != nil)
			require.NotEmpty(t, bootstrap.CustomData)
			require.NotEmpty(t, bootstrap.CSE)
		})
	}
}

func nodeBootstrappingConfigForTest(distro datamodel.Distro) *datamodel.NodeBootstrappingConfiguration {
	pool := &datamodel.AgentPoolProfile{
		Name:                "agent",
		VMSize:              "Standard_DS1_v2",
		OSType:              datamodel.Linux,
		Distro:              distro,
		StorageProfile:      "ManagedDisks",
		AvailabilityProfile: datamodel.VirtualMachineScaleSets,
	}
	return &datamodel.NodeBootstrappingConfiguration{
		ContainerService: &datamodel.ContainerService{
			Location: "eastus",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    datamodel.Kubernetes,
					OrchestratorVersion: "1.32.1",
					KubernetesConfig:    &datamodel.KubernetesConfig{},
				},
				HostedMasterProfile:     &datamodel.HostedMasterProfile{DNSPrefix: "test"},
				AgentPoolProfiles:       []*datamodel.AgentPoolProfile{pool},
				LinuxProfile:            &datamodel.LinuxProfile{AdminUsername: "azureuser"},
				ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{ClientID: "client"},
			},
		},
		AgentPoolProfile: pool,
		CloudSpecConfig:  datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:    &datamodel.K8sComponents{},
		SIGConfig:        sigConfigForTest(),
	}
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
