package config

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

// RequireDirectVirtualizationSKU rejects nested-only and subscription-restricted
// SKUs. Advertising nested virtualization is not evidence of direct virtualization.
func (a *AzureClient) RequireDirectVirtualizationSKU(ctx context.Context, location, size string) (*armcompute.ResourceSKU, error) {
	sku, err := a.getResourceSKU(ctx, location, size)
	if err != nil {
		return nil, err
	}
	if err := validateDirectVirtualizationSKU(sku); err != nil {
		return nil, fmt.Errorf("direct virtualization SKU %q in %q: %w", size, location, err)
	}
	return sku, nil
}

func validateDirectVirtualizationSKU(sku *armcompute.ResourceSKU) error {
	if sku == nil {
		return fmt.Errorf("SKU is nil")
	}
	for _, restriction := range sku.Restrictions {
		if restriction != nil && restriction.Type != nil && *restriction.Type == armcompute.ResourceSKURestrictionsTypeLocation {
			return fmt.Errorf("SKU has a location restriction for this subscription")
		}
	}
	for _, capability := range sku.Capabilities {
		if capability == nil || capability.Name == nil || capability.Value == nil || !strings.EqualFold(*capability.Name, "SupportedVirtualizationTypes") {
			continue
		}
		for _, value := range strings.Split(*capability.Value, ",") {
			if strings.EqualFold(strings.TrimSpace(value), "DirectVirtualization") {
				return nil
			}
		}
	}
	return fmt.Errorf("SKU does not advertise SupportedVirtualizationTypes=DirectVirtualization")
}

type kataGalleryAPIPolicy struct{}

func (kataGalleryAPIPolicy) Do(req *policy.Request) (*http.Response, error) {
	query := req.Raw().URL.Query()
	query.Set("api-version", "2025-12-03")
	req.Raw().URL.RawQuery = query.Encode()
	return req.Next()
}

// GetKataImageDefinition reads the parent of the actual selected image version,
// including cross-subscription gallery references from PR-build metadata.
func (a *AzureClient) GetKataImageDefinition(ctx context.Context, versionID string) (*armcompute.GalleryImage, error) {
	id, err := arm.ParseResourceID(versionID)
	if err != nil || id == nil || !strings.EqualFold(id.ResourceType.String(), "Microsoft.Compute/galleries/images/versions") {
		return nil, fmt.Errorf("expected a gallery image version resource ID, got %q", versionID)
	}
	options := arm.ClientOptions{}
	if a.ArmOptions != nil {
		options = *a.ArmOptions
	}
	options.PerCallPolicies = append([]policy.Policy{kataGalleryAPIPolicy{}}, options.PerCallPolicies...)
	client, err := armcompute.NewGalleryImagesClient(id.SubscriptionID, a.Credential, &options)
	if err != nil {
		return nil, err
	}
	result, err := client.Get(ctx, id.ResourceGroupName, id.Parent.Parent.Name, id.Parent.Name, nil)
	if err != nil {
		return nil, fmt.Errorf("read Kata image definition for %s: %w", versionID, err)
	}
	return &result.GalleryImage, nil
}
