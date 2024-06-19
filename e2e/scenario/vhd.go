package scenario

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

var (
	VHDUbuntu1804Gen2Containerd = &VHD{
		ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/1804Gen2",
		VersionTagName:  "branch",
		VersionTagValue: "refs/heads/master",
	}
	VHDUbuntu2204Gen2Arm64Containerd = &VHD{
		ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2Arm64",
		VersionTagName:  "branch",
		VersionTagValue: "refs/heads/master",
	}
	VHDUbuntu2204Gen2Containerd = &VHD{
		ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2",
		VersionTagName:  "branch",
		VersionTagValue: "refs/heads/master",
	}
	VHDUbuntu2204Gen2ContainerdPrivateKubePkg = &VHD{
		resourceID: "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2/versions/1.1704411049.2812",
	}
	VHDAzureLinuxV2Gen2Arm64 = &VHD{
		ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/AzureLinuxV2Gen2Arm64",
		VersionTagName:  "branch",
		VersionTagValue: "refs/heads/master",
	}
	VHDAzureLinuxV2Gen2 = &VHD{
		ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/AzureLinuxV2Gen2",
		VersionTagName:  "branch",
		VersionTagValue: "refs/heads/master",
	}
	VHDCBLMarinerV2Gen2Arm64 = &VHD{
		ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2Gen2Arm64",
		VersionTagName:  "branch",
		VersionTagValue: "refs/heads/master",
	}
	VHDCBLMarinerV2Gen2 = &VHD{
		ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2Gen2",
		VersionTagName:  "branch",
		VersionTagValue: "refs/heads/master",
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

// VHD represents a VHD used to run AgentBaker E2E scenarios.
type VHD struct {
	ImageID         string
	VersionTagName  string
	VersionTagValue string
	// ResourceID is the resource ID pointing to the underlying VHD in Azure. Based on the current setup, this will always be the resource ID
	// of an image version in a shared image gallery.
	resourceID VHDResourceID
	sync.Once
}

func (v *VHD) ResourceID() VHDResourceID {

	v.Do(func() {
		if v.resourceID != "" {
			return
		}
		var err error
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		v.resourceID, err = findLatestResourceID(ctx, v)
		if err != nil {
			panic(err)
		}
	})
	return v.resourceID
}

func findLatestResourceID(ctx context.Context, vhd *VHD) (VHDResourceID, error) {
	if config.VHDBuildID != "" {
		resourceID, err := findLatestImageWithTag(ctx, vhd.ImageID, "buildId", config.VHDBuildID)
		if errors.Is(err, ErrNotFound) {
			return "", nil
		}
		return resourceID, err
	}
	return findLatestImageWithTag(ctx, vhd.ImageID, vhd.VersionTagName, vhd.VersionTagValue)
}

var ErrNotFound = fmt.Errorf("not found")

func findLatestImageWithTag(ctx context.Context, imageID, tagName, tagValue string) (VHDResourceID, error) {
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
	var result imageID

	// Define the regex pattern to match the desired parts of the resource ID
	pattern := `(?i)^/subscriptions/([^/]+)/resourceGroups/([^/]+)/providers/Microsoft\.Compute/galleries/([^/]+)/images/([^/]+)$`
	re := regexp.MustCompile(pattern)

	// Find the submatches in the resourceID
	matches := re.FindStringSubmatch(resourceID)
	if matches == nil || len(matches) != 5 {
		return result, fmt.Errorf("failed to parse image ID %q", resourceID)
	}

	// Assign the captured groups to the struct
	result.subscriptionID = matches[1]
	result.resourceGroup = matches[2]
	result.galleryName = matches[3]
	result.imageName = matches[4]

	return result, nil
}

type Manifest struct {
	Containerd struct {
		Edge string `json:"edge"`
	} `json:"containerd"`
}

func getVHDManifest() (*Manifest, error) {
	manifestData, err := os.ReadFile("../parts/linux/cloud-init/artifacts/manifest.json")
	if err != nil {
		return nil, err
	}
	manifestDataStr := string(manifestData)
	manifestDataStr = strings.TrimRight(manifestDataStr, "#EOF \n\r\t")
	manifestData = []byte(manifestDataStr)

	manifest := Manifest{}
	if err = json.Unmarshal([]byte(manifestData), &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}
