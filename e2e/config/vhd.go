package config

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

const imageGallery = "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/"

var (
	VHDUbuntu1804Gen2Containerd               = newVHDResourceIDFetcher(imageGallery + "1804Gen2")
	VHDUbuntu2204Gen2Arm64Containerd          = newVHDResourceIDFetcher(imageGallery + "2204Gen2Arm64")
	VHDUbuntu2204Gen2Containerd               = newVHDResourceIDFetcher(imageGallery + "2204Gen2")
	VHDAzureLinuxV2Gen2Arm64                  = newVHDResourceIDFetcher(imageGallery + "AzureLinuxV2Gen2Arm64")
	VHDAzureLinuxV2Gen2                       = newVHDResourceIDFetcher(imageGallery + "AzureLinuxV2Gen2")
	VHDCBLMarinerV2Gen2Arm64                  = newVHDResourceIDFetcher(imageGallery + "CBLMarinerV2Gen2Arm64")
	VHDCBLMarinerV2Gen2                       = newVHDResourceIDFetcher(imageGallery + "CBLMarinerV2Gen2")
	VHDUbuntu2204Gen2ContainerdPrivateKubePkg = func() (VHDResourceID, error) {
		return imageGallery + "2204Gen2PrivateKubePkg", nil
	}
)

// VHDResourceID represents a resource ID pointing to a VHD in Azure. This could be theoretically
// be the resource ID of a managed image or SIG image version, though for now this will always be a SIG image version.
type VHDResourceID string

// newVHDResourceIDFetcher is a factory function
// it returns a function that fetches the latest VHDResourceID for a given image
// the function is memoized and will only evaluate once on the first call
func newVHDResourceIDFetcher(image string) func() (VHDResourceID, error) {
	resourceID := VHDResourceID("")
	var err error
	once := sync.Once{}
	// evaluate the function once and cache the result
	return func() (VHDResourceID, error) {
		once.Do(func() {
			resourceID, err = findLatestImageWithTag(image, SIGVersionTagName, SIGVersionTagValue)
		})
		return resourceID, err
	}
}

func (id VHDResourceID) Short() string {
	sep := "Microsoft.Compute/galleries/"
	str := string(id)
	if strings.Contains(str, sep) && !strings.HasSuffix(str, sep) {
		return strings.Split(str, sep)[1]
	}
	return str
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
