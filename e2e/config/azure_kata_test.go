package config

import (
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
)

func TestDirectVirtualizationSKUCapability(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"DirectVirtualization", true},
		{"NestedVirtualization, directvirtualization ", true},
		{"NestedVirtualization", false},
		{"True", false},
		{"NotDirectVirtualization", false},
		{"", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			sku := &armcompute.ResourceSKU{Capabilities: []*armcompute.ResourceSKUCapabilities{
				nil, {Name: to.Ptr("SupportedVirtualizationTypes"), Value: to.Ptr(tc.value)},
			}}
			err := validateDirectVirtualizationSKU(sku)
			if tc.valid {
				require.NoError(t, err)
				sku.Restrictions = []*armcompute.ResourceSKURestrictions{{Type: to.Ptr(armcompute.ResourceSKURestrictionsTypeLocation)}}
				require.ErrorContains(t, validateDirectVirtualizationSKU(sku), "restriction")
			} else {
				require.Error(t, err)
			}
		})
	}
	require.Error(t, validateDirectVirtualizationSKU(nil))
}

func TestGetKataImageDefinitionUsesActualVersionParent(t *testing.T) {
	client := galleryTestClient(func(req *http.Request) (int, string) {
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "/subscriptions/source-sub/resourceGroups/source-rg/providers/Microsoft.Compute/galleries/source-gallery/images/kata", req.URL.Path)
		require.Equal(t, "2025-12-03", req.URL.Query().Get("api-version"))
		return http.StatusOK, `{"properties":{"features":[{"name":"VirtualizationType","value":"Direct"}]}}`
	})
	definition, err := client.GetKataImageDefinition(t.Context(), "/subscriptions/source-sub/resourceGroups/source-rg/providers/Microsoft.Compute/galleries/source-gallery/images/kata/versions/1.2.3")
	require.NoError(t, err)
	require.Equal(t, "Direct", *definition.Properties.Features[0].Value)
	require.Len(t, client.ArmOptions.PerCallPolicies, 1)
	_, err = client.GetKataImageDefinition(t.Context(), "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/images/managed-image")
	require.ErrorContains(t, err, "expected a gallery image version")
}

func TestRequireDirectVirtualizationSKU(t *testing.T) {
	for _, value := range []string{"DirectVirtualization", "NestedVirtualization"} {
		t.Run(value, func(t *testing.T) {
			client := galleryTestClient(func(req *http.Request) (int, string) {
				require.Equal(t, "location eq 'eastus'", req.URL.Query().Get("$filter"))
				return http.StatusOK, `{"value":[{"name":"test-sku","resourceType":"virtualMachines","capabilities":[{"name":"SupportedVirtualizationTypes","value":"` + value + `"}]}]}`
			})
			var err error
			client.ResourceSKUs, err = armcompute.NewResourceSKUsClient("sub", client.Credential, client.ArmOptions)
			require.NoError(t, err)
			_, err = client.RequireDirectVirtualizationSKU(t.Context(), "eastus", "test-sku")
			if value == "DirectVirtualization" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "does not advertise")
			}
		})
	}
}
