package config

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

const (
	imageGallery       = "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/"
	noSelectionTagName = "abe2e-ignore"

	fetchResourceIDTimeout = 3 * time.Minute
)

var (
	VHDUbuntu1804Gen2Containerd = &Image{
		Name: "1804Gen2",
		OS:   "ubuntu",
		Arch: "amd64",
	}
	VHDUbuntu2204Gen2Arm64Containerd = &Image{
		Name: "2204Gen2Arm64",
		OS:   "ubuntu",
		Arch: "arm64",
	}
	VHDUbuntu2204Gen2Containerd = &Image{
		Name: "2204Gen2",
		OS:   "ubuntu",
		Arch: "amd64",
	}
	VHDAzureLinuxV2Gen2Arm64 = &Image{
		Name: "AzureLinuxV2Gen2Arm64",
		OS:   "azurelinux",
		Arch: "arm64",
	}
	VHDAzureLinuxV2Gen2 = &Image{
		Name: "AzureLinuxV2Gen2",
		OS:   "azurelinux",
		Arch: "amd64",
	}
	VHDCBLMarinerV2Gen2Arm64 = &Image{
		Name: "CBLMarinerV2Gen2Arm64",
		OS:   "mariner",
		Arch: "arm64",
	}
	VHDCBLMarinerV2Gen2 = &Image{
		Name: "CBLMarinerV2Gen2",
		OS:   "mariner",
		Arch: "amd64",
	}
	VHDUbuntu2204Gen2ContainerdPrivateKubePkg = &Image{
		Name:    "2204Gen2",
		OS:      "ubuntu",
		Arch:    "amd64",
		Version: "1.1704411049.2812",
	}
)

var ErrNotFound = fmt.Errorf("not found")

type Image struct {
	Name    string
	OS      string
	Arch    string
	Version string

	vhd      VHDResourceID
	vhdOnced sync.Once
	vhdErr   error
}

func (i *Image) VHDResourceID() (VHDResourceID, error) {
	i.vhdOnced.Do(func() {
		imageDefinitionResourceID := imageGallery + i.Name
		if i.Version != "" {
			i.vhd, i.vhdErr = ensureStaticSIGImageVersion(imageDefinitionResourceID + "/versions/" + i.Version)
		} else {
			i.vhd, i.vhdErr = findLatestSIGImageVersionWithTag(imageDefinitionResourceID, SIGVersionTagName, SIGVersionTagValue)
		}
		if i.vhdErr != nil {
			i.vhdErr = fmt.Errorf("img: %s, tag %s=%s, err %w", imageDefinitionResourceID, SIGVersionTagName, SIGVersionTagValue, i.vhdErr)
			log.Printf("failed to find the latest image %s", i.vhdErr)
		} else {
			log.Printf("Resource ID for %s: %s", imageDefinitionResourceID, i.vhd)
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

func ensureStaticSIGImageVersion(imageVersionResourceID string) (VHDResourceID, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), fetchResourceIDTimeout)
	defer cancel()

	rid, err := arm.ParseResourceID(imageVersionResourceID)
	if err != nil {
		return "", fmt.Errorf("parsing image version resouce ID: %w", err)
	}
	version := newSIGImageVersionFromResourceID(rid)

	resp, err := Azure.GalleryImageVersionClient.Get(ctx, version.resourceGroup, version.gallery, version.definition, version.version, nil)
	if err != nil {
		return "", fmt.Errorf("getting live image version info: %w", err)
	}

	if err := ensureReplication(ctx, version.sigImageDefinition, &resp.GalleryImageVersion); err != nil {
		return "", fmt.Errorf("ensuring image replication: %w", err)
	}

	return VHDResourceID(imageVersionResourceID), nil
}

func findLatestSIGImageVersionWithTag(imageDefinitionResourceID, tagName, tagValue string) (VHDResourceID, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), fetchResourceIDTimeout)
	defer cancel()

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
			if latestVersion == nil || version.Properties.PublishingProfile.PublishedDate.After(*latestVersion.Properties.PublishingProfile.PublishedDate) {
				latestVersion = version
			}
		}
	}
	if latestVersion == nil {
		return "", ErrNotFound
	}

	if err := ensureReplication(ctx, definition, latestVersion); err != nil {
		return "", fmt.Errorf("ensuring image replication: %w", err)
	}

	return VHDResourceID(*latestVersion.ID), nil
}

func ensureReplication(ctx context.Context, definition sigImageDefinition, version *armcompute.GalleryImageVersion) error {
	if replicatedToCurrentRegion(version) {
		return nil
	}
	return replicateToCurrentRegion(ctx, definition, version)
}

func replicatedToCurrentRegion(version *armcompute.GalleryImageVersion) bool {
	for _, targetRegion := range version.Properties.PublishingProfile.TargetRegions {
		if strings.EqualFold(strings.ReplaceAll(*targetRegion.Name, " ", ""), Location) {
			return true
		}
	}
	return false
}

func replicateToCurrentRegion(ctx context.Context, definition sigImageDefinition, version *armcompute.GalleryImageVersion) error {
	log.Printf("will replicate image version %s to region %s...", *version.ID, Location)

	version.Properties.PublishingProfile.TargetRegions = append(version.Properties.PublishingProfile.TargetRegions, &armcompute.TargetRegion{
		Name:                 &Location,
		RegionalReplicaCount: to.Ptr[int32](1),
		StorageAccountType:   to.Ptr(armcompute.StorageAccountTypeStandardLRS),
	})

	resp, err := Azure.GalleryImageVersionClient.BeginCreateOrUpdate(ctx, definition.resourceGroup, definition.gallery, definition.definition, *version.Name, *version, nil)
	if err != nil {
		return fmt.Errorf("begin updating image version target regions: %w", err)
	}
	if _, err := resp.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("updating image version target regions: %w", err)
	}

	return nil
}
