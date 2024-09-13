package config

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

const (
	imageGallery       = "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/"
	noSelectionTagName = "abe2e-ignore"
)

var (
	VHDUbuntu1804Gen2Containerd = &Image{
		Name: "1804gen2containerd",
		OS:   "ubuntu",
		Arch: "amd64",
	}
	VHDUbuntu2204Gen2Arm64Containerd = &Image{
		Name: "2204gen2arm64containerd",
		OS:   "ubuntu",
		Arch: "arm64",
	}
	VHDUbuntu2204Gen2Containerd = &Image{
		Name: "2204gen2containerd",
		OS:   "ubuntu",
		Arch: "amd64",
	}
	VHDAzureLinuxV2Gen2Arm64 = &Image{
		Name: "AzureLinuxV2gen2arm64",
		OS:   "azurelinux",
		Arch: "arm64",
	}
	VHDAzureLinuxV2Gen2 = &Image{
		Name: "AzureLinuxV2gen2",
		OS:   "azurelinux",
		Arch: "amd64",
	}
	VHDCBLMarinerV2Gen2Arm64 = &Image{
		Name: "CBLMarinerV2gen2arm64",
		OS:   "mariner",
		Arch: "arm64",
	}
	VHDCBLMarinerV2Gen2 = &Image{
		Name: "CBLMarinerV2gen2",
		OS:   "mariner",
		Arch: "amd64",
	}
	// this is a particular 2204gen2containerd image originally built with private packages,
	// if we ever want to update this then we'd need to run a new VHD build using private package overrides
	VHDUbuntu2204Gen2ContainerdPrivateKubePkg = &Image{
		Name:    "2204Gen2",
		OS:      "ubuntu",
		Arch:    "amd64",
		Version: "1.1704411049.2812",
	}

	// without kubelet, kubectl, credential-provider and wasm
	VHDUbuntu2204Gen2ContainerdAirgapped = &Image{
		Name:    "2204gen2containerd",
		OS:      "ubuntu",
		Arch:    "amd64",
		Version: "1.1725612526.29638",
	}
)

var ErrNotFound = fmt.Errorf("not found")

type Image struct {
	Arch    string
	Name    string
	OS      string
	Version string

	vhd     VHDResourceID
	vhdOnce sync.Once
	vhdErr  error
}

func (i *Image) String() string {
	// a starter for a string for debugging.
	return fmt.Sprintf("%s %s %s %s", i.OS, i.Name, i.Version, i.Arch)
}

func (i *Image) VHDResourceID(ctx context.Context, t *testing.T) (VHDResourceID, error) {
	i.vhdOnce.Do(func() {
		imageDefinitionResourceID := imageGallery + i.Name
		if i.Version != "" {
			i.vhd, i.vhdErr = ensureStaticSIGImageVersion(ctx, t, imageDefinitionResourceID+"/versions/"+i.Version)
		} else {
			i.vhd, i.vhdErr = findLatestSIGImageVersionWithTag(ctx, t, imageDefinitionResourceID, Config.SIGVersionTagName, Config.SIGVersionTagValue)
		}
		if i.vhdErr != nil {
			i.vhdErr = fmt.Errorf("img: %s, tag %s=%s, err %w", imageDefinitionResourceID, Config.SIGVersionTagName, Config.SIGVersionTagValue, i.vhdErr)
			t.Logf("failed to find the latest image %s", i.vhdErr)
		}
	})
	return i.vhd, i.vhdErr
}

type sigImageDefinition struct {
	subscriptionID string
	resourceGroup  string
	gallery        string
	definition     string
}

type sigImageVersion struct {
	sigImageDefinition
	version string
}

func newSIGImageDefinitionFromResourceID(resourceID *arm.ResourceID) sigImageDefinition {
	return sigImageDefinition{
		subscriptionID: resourceID.SubscriptionID,
		resourceGroup:  resourceID.ResourceGroupName,
		gallery:        resourceID.Parent.Name,
		definition:     resourceID.Name,
	}
}

func newSIGImageVersionFromResourceID(resourceID *arm.ResourceID) sigImageVersion {
	return sigImageVersion{
		sigImageDefinition: newSIGImageDefinitionFromResourceID(resourceID.Parent),
		version:            resourceID.Name,
	}
}

// VHDResourceID represents a resource ID pointing to a VHD in Azure. This could be theoretically
// be the resource ID of a managed image or SIG image version, though for now this will always be a SIG image version.
type VHDResourceID string

func (id VHDResourceID) Short() string {
	sep := "Microsoft.Compute/galleries/"
	str := string(id)
	if strings.Contains(str, sep) && !strings.HasSuffix(str, sep) {
		return strings.Split(str, sep)[1]
	}
	return str
}

func ensureStaticSIGImageVersion(ctx context.Context, t *testing.T, imageVersionResourceID string) (VHDResourceID, error) {
	rid, err := arm.ParseResourceID(imageVersionResourceID)
	if err != nil {
		return "", fmt.Errorf("parsing image version resouce ID: %w", err)
	}
	version := newSIGImageVersionFromResourceID(rid)

	resp, err := Azure.GalleryImageVersionClient.Get(ctx, version.resourceGroup, version.gallery, version.definition, version.version, nil)
	if err != nil {
		return "", fmt.Errorf("getting live image version info: %w", err)
	}

	liveVersion := &resp.GalleryImageVersion
	if err := ensureProvisioningState(liveVersion); err != nil {
		return "", fmt.Errorf("ensuring image version provisioning state: %w", err)
	}

	if err := ensureReplication(ctx, t, version.sigImageDefinition, liveVersion); err != nil {
		return "", fmt.Errorf("ensuring image replication: %w", err)
	}

	return VHDResourceID(imageVersionResourceID), nil
}

func findLatestSIGImageVersionWithTag(ctx context.Context, t *testing.T, imageDefinitionResourceID, tagName, tagValue string) (VHDResourceID, error) {
	rid, err := arm.ParseResourceID(imageDefinitionResourceID)
	if err != nil {
		return "", fmt.Errorf("parsing image definition resource ID: %w", err)
	}
	definition := newSIGImageDefinitionFromResourceID(rid)

	pager := Azure.GalleryImageVersionClient.NewListByGalleryImagePager(definition.resourceGroup, definition.gallery, definition.definition, nil)
	var latestVersion *armcompute.GalleryImageVersion
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get next page: %w", err)
		}
		versions := page.Value
		for _, version := range versions {
			// skip images tagged with the no-selection tag, indicating they
			// shouldn't be selected dynmically for running abe2e scenarios
			if _, ok := version.Tags[noSelectionTagName]; ok {
				continue
			}
			tag, ok := version.Tags[tagName]
			if !ok || tag == nil || *tag != tagValue {
				continue
			}
			if err := ensureProvisioningState(version); err != nil {
				t.Logf("ensuring image version %s provisioning state: %s, will not consider for selection", *version.ID, err)
				continue
			}
			if latestVersion == nil || version.Properties.PublishingProfile.PublishedDate.After(*latestVersion.Properties.PublishingProfile.PublishedDate) {
				latestVersion = version
			}
		}
	}
	if latestVersion == nil {
		return "", ErrNotFound
	}

	if err := ensureReplication(ctx, t, definition, latestVersion); err != nil {
		return "", fmt.Errorf("ensuring image replication: %w", err)
	}

	return VHDResourceID(*latestVersion.ID), nil
}

func ensureReplication(ctx context.Context, t *testing.T, definition sigImageDefinition, version *armcompute.GalleryImageVersion) error {
	if replicatedToCurrentRegion(version) {
		t.Logf("image version %s is already replicated to region %s", *version.ID, Config.Location)
		return nil
	}
	return replicateToCurrentRegion(ctx, t, definition, version)
}

func replicatedToCurrentRegion(version *armcompute.GalleryImageVersion) bool {
	for _, targetRegion := range version.Properties.PublishingProfile.TargetRegions {
		if strings.EqualFold(strings.ReplaceAll(*targetRegion.Name, " ", ""), Config.Location) {
			return true
		}
	}
	return false
}

func replicateToCurrentRegion(ctx context.Context, t *testing.T, definition sigImageDefinition, version *armcompute.GalleryImageVersion) error {
	t.Logf("will replicate image version %s to region %s...", *version.ID, Config.Location)

	version.Properties.PublishingProfile.TargetRegions = append(version.Properties.PublishingProfile.TargetRegions, &armcompute.TargetRegion{
		Name:                 &Config.Location,
		RegionalReplicaCount: to.Ptr[int32](1),
		StorageAccountType:   to.Ptr(armcompute.StorageAccountTypeStandardLRS),
	})

	resp, err := Azure.GalleryImageVersionClient.BeginCreateOrUpdate(ctx, definition.resourceGroup, definition.gallery, definition.definition, *version.Name, *version, nil)
	if err != nil {
		return fmt.Errorf("begin updating image version target regions: %w", err)
	}
	if _, err := resp.PollUntilDone(ctx, DefaultPollUntilDoneOptions); err != nil {
		return fmt.Errorf("updating image version target regions: %w", err)
	}

	return nil
}

func ensureProvisioningState(version *armcompute.GalleryImageVersion) error {
	if *version.Properties.ProvisioningState != armcompute.GalleryProvisioningStateSucceeded {
		return fmt.Errorf("unexpected provisioning state: %q", *version.Properties.ProvisioningState)
	}
	return nil
}
