package apiserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestGetLatestSIGImageConfigGallerySelection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		distro      datamodel.Distro
		useEdgeZone bool
		gallery     string
		status      int
	}{
		{"regional Ubuntu by default", datamodel.AKSUbuntuContainerd2404Gen2, false, "regional-ubuntu", http.StatusOK},
		{"Extended Zone Ubuntu", datamodel.AKSUbuntuContainerd2404Gen2, true, datamodel.AKSUbuntuEdgeZoneGalleryName, http.StatusOK},
		{"regional Azure Linux by default", datamodel.AKSAzureLinuxV3Gen2, false, "regional-azurelinux", http.StatusOK},
		{"Extended Zone Azure Linux", datamodel.AKSAzureLinuxV3Gen2, true, datamodel.AKSAzureLinuxEdgeZoneGalleryName, http.StatusOK},
		{"legacy Ubuntu by default", datamodel.AKSUbuntuEdgeZoneContainerd2404Gen2, false, datamodel.AKSUbuntuEdgeZoneGalleryName, http.StatusOK},
		{"unsupported Extended Zone variant", datamodel.AKSUbuntuFipsContainerd2204Gen2, true, "", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := serveJSONRequest(t, RoutePathLatestSIGImageConfig, datamodel.GetLatestSigImageConfigRequest{
				SIGConfig:          sigConfigForTest(),
				Region:             "eastus",
				Distro:             tc.distro,
				UseEdgeZoneGallery: tc.useEdgeZone,
			})
			require.Equal(t, tc.status, response.Code, response.Body.String())
			if tc.status != http.StatusOK {
				require.Contains(t, response.Body.String(), "can't find SIG image config")
				return
			}

			var image datamodel.SigImageConfig
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &image))
			require.Equal(t, tc.gallery, image.Gallery)
			require.Equal(t, "subscription", image.SubscriptionID)
			require.NotEmpty(t, image.Definition)
			require.NotEmpty(t, image.Version)
		})
	}
}

func TestGetDistroSIGImageConfigGallerySelection(t *testing.T) {
	for _, useEdgeZone := range []bool{false, true} {
		name := "regional by default"
		if useEdgeZone {
			name = "Extended Zone"
		}
		t.Run(name, func(t *testing.T) {
			response := serveJSONRequest(t, RoutePathDistroSIGImageConfig, datamodel.GetLatestSigImageConfigRequest{
				SIGConfig:          sigConfigForTest(),
				Region:             "eastus",
				UseEdgeZoneGallery: useEdgeZone,
			})
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())

			var images map[datamodel.Distro]datamodel.SigImageConfig
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &images))
			ubuntuGallery, azureLinuxGallery := "regional-ubuntu", "regional-azurelinux"
			if useEdgeZone {
				ubuntuGallery = datamodel.AKSUbuntuEdgeZoneGalleryName
				azureLinuxGallery = datamodel.AKSAzureLinuxEdgeZoneGalleryName
				require.NotContains(t, images, datamodel.AKSUbuntuFipsContainerd2204Gen2)
				for _, image := range images {
					require.Contains(t, []string{ubuntuGallery, azureLinuxGallery}, image.Gallery)
				}
			} else {
				require.Contains(t, images, datamodel.AKSWindows2022Containerd)
			}
			require.Equal(t, ubuntuGallery, images[datamodel.AKSUbuntuContainerd2404Gen2].Gallery)
			require.Equal(t, azureLinuxGallery, images[datamodel.AKSAzureLinuxV3Gen2].Gallery)
			require.Equal(t, datamodel.AKSUbuntuEdgeZoneGalleryName, images[datamodel.AKSUbuntuEdgeZoneContainerd2404Gen2].Gallery)
		})
	}
}

func sigConfigForTest() datamodel.SIGConfig {
	return datamodel.SIGConfig{
		TenantID:       "tenant",
		SubscriptionID: "subscription",
		Galleries: map[string]datamodel.SIGGalleryConfig{
			"AKSUbuntu":     {GalleryName: "regional-ubuntu", ResourceGroup: "regional-rg"},
			"AKSCBLMariner": {GalleryName: "regional-mariner", ResourceGroup: "regional-rg"},
			"AKSAzureLinux": {GalleryName: "regional-azurelinux", ResourceGroup: "regional-rg"},
			"AKSWindows":    {GalleryName: "regional-windows", ResourceGroup: "regional-rg"},
		},
	}
}

func serveJSONRequest(t *testing.T, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	api, err := NewAPIServer(&Options{Addr: ":8080"})
	require.NoError(t, err)
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	api.NewRouter().ServeHTTP(response, request)
	return response
}
