package config

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

// VHD represents a VHD used to run AgentBaker E2E scenarios.
type VHD struct {
	ImageID string
	// ResourceID is the resource ID pointing to the underlying VHD in Azure. Based on the current setup, this will always be the resource ID
	// of an image version in a shared image gallery.
	resourceID          VHDResourceID
	fetchResourceIDOnce sync.Once
}

var (
	VHDUbuntu1804Gen2Containerd = &VHD{
		ImageID: "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/1804Gen2",
	}
	VHDUbuntu2204Gen2Arm64Containerd = &VHD{
		ImageID: "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2Arm64",
	}
	VHDUbuntu2204Gen2Containerd = &VHD{
		ImageID: "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2",
	}
	VHDUbuntu2204Gen2ContainerdPrivateKubePkg = &VHD{
		resourceID: "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2/versions/1.1704411049.2812",
	}
	VHDAzureLinuxV2Gen2Arm64 = &VHD{
		ImageID: "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/AzureLinuxV2Gen2Arm64",
	}
	VHDAzureLinuxV2Gen2 = &VHD{
		ImageID: "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/AzureLinuxV2Gen2",
	}
	VHDCBLMarinerV2Gen2Arm64 = &VHD{
		ImageID: "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2Gen2Arm64",
	}
	VHDCBLMarinerV2Gen2 = &VHD{
		ImageID: "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2Gen2",
	}
)

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

func (v *VHD) ResourceID() VHDResourceID {
	v.fetchResourceIDOnce.Do(func() {
		if v.resourceID != "" {
			return
		}
		var err error
		v.resourceID, err = findLatestImageWithTag(v.ImageID, SIGVersionTagName, SIGVersionTagValue)
		// in some cases we ignore missing image, like when we run e2e for a build containing a single image
		if errors.Is(err, ErrNotFound) {
			return
		}
		if err != nil {
			panic(fmt.Errorf("failed to find latest image with tag %q=%q for image %q: %v", SIGVersionTagName, SIGVersionTagValue, v.ImageID, err))
		}
	})

	return v.resourceID
}

var ErrNotFound = fmt.Errorf("not found")

func findLatestImageWithTag(imageID, tagName, tagValue string) (VHDResourceID, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute)
	defer cancel()
	image, err := parseImageID(imageID)
	if err != nil {
		return "", err
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return "", fmt.Errorf("failed to obtain a credential: %v", err)
	}

	client, err := armcompute.NewGalleryImageVersionsClient(image.subscriptionID, cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create a new images client: %v", err)
	}

	pager := client.NewListByGalleryImagePager(image.resourceGroup, image.galleryName, image.imageName, nil)
	var latestVersion *armcompute.GalleryImageVersion
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get next page: %v", err)
		}
		versions := page.Value
		for _, version := range versions {
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
	return VHDResourceID(*latestVersion.ID), nil
}

type imageID struct {
	subscriptionID string
	resourceGroup  string
	galleryName    string
	imageName      string
}

func parseImageID(resourceID string) (imageID, error) {
	pattern := `(?i)^/subscriptions/([^/]+)/resourceGroups/([^/]+)/providers/Microsoft\.Compute/galleries/([^/]+)/images/([^/]+)$`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(resourceID)

	if matches == nil || len(matches) != 5 {
		return imageID{}, fmt.Errorf("failed to parse image ID %q", resourceID)
	}

	return imageID{
		subscriptionID: matches[1],
		resourceGroup:  matches[2],
		galleryName:    matches[3],
		imageName:      matches[4],
	}, nil
}
