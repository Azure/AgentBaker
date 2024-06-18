package scenario

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/Azure/agentbakere2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

var (

	// BaseVHDCatalog represents the base VHD catalog that every E2E suite will start off of.
	// It contains the set of VHDs used by AgentBaker E2Es, along with the specific versions and artifact name for each.
	// When a VHD build ID is specified, this catalog's entries will be overwritten respectively for each downloaded VHD publishing info.
	BaseVHDCatalog = VHDCatalog{
		Ubuntu1804Gen2Containerd: VHD{
			ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/1804Gen2",
			VersionTagName:  "branch",
			VersionTagValue: "refs/heads/r2k1/e2e-update",
		},
		Ubuntu2204Gen2Arm64Containerd: VHD{
			ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2Arm64",
			VersionTagName:  "branch",
			VersionTagValue: "refs/heads/r2k1/e2e-update",
		},
		Ubuntu2204Gen2Containerd: VHD{
			ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2",
			VersionTagName:  "branch",
			VersionTagValue: "refs/heads/r2k1/e2e-update",
		},
		Ubuntu2204Gen2ContainerdPrivateKubePkg: VHD{
			ResourceID: "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2/versions/1.1704411049.2812",
		},
		AzureLinuxV2Gen2Arm64: VHD{
			ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/AzureLinuxV2Gen2Arm64",
			VersionTagName:  "branch",
			VersionTagValue: "refs/heads/r2k1/e2e-update",
		},
		AzureLinuxV2Gen2: VHD{
			ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/AzureLinuxV2Gen2",
			VersionTagName:  "branch",
			VersionTagValue: "refs/heads/r2k1/e2e-update",
		},
		CBLMarinerV2Gen2Arm64: VHD{
			ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2Gen2Arm64",
			VersionTagName:  "branch",
			VersionTagValue: "refs/heads/r2k1/e2e-update",
		},
		CBLMarinerV2Gen2: VHD{
			ImageID:         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2Gen2",
			VersionTagName:  "branch",
			VersionTagValue: "refs/heads/r2k1/e2e-update",
		},
	}
)

type VHDCatalog struct {
	Ubuntu1804Gen2Containerd               VHD
	Ubuntu2204Gen2Arm64Containerd          VHD
	Ubuntu2204Gen2Containerd               VHD
	Ubuntu2204Gen2ContainerdPrivateKubePkg VHD
	AzureLinuxV2Gen2Arm64                  VHD
	AzureLinuxV2Gen2                       VHD
	CBLMarinerV2Gen2Arm64                  VHD
	CBLMarinerV2Gen2                       VHD
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

// VHD represents a VHD used to run AgentBaker E2E scenarios.
type VHD struct {
	ImageID         string
	VersionTagName  string
	VersionTagValue string
	// ResourceID is the resource ID pointing to the underlying VHD in Azure. Based on the current setup, this will always be the resource ID
	// of an image version in a shared image gallery.
	ResourceID VHDResourceID
}

func getVHDsFromBuild(ctx context.Context, tmpl *Template, scenarios []*Scenario) error {
	if config.VHDBuildID == "" {
		return nil
	}

	vhds := []*VHD{
		&tmpl.Ubuntu1804Gen2Containerd,
		&tmpl.Ubuntu2204Gen2Arm64Containerd,
		&tmpl.Ubuntu2204Gen2Containerd,
		&tmpl.Ubuntu2204Gen2ContainerdPrivateKubePkg,
		&tmpl.AzureLinuxV2Gen2Arm64,
		&tmpl.AzureLinuxV2Gen2,
		&tmpl.CBLMarinerV2Gen2Arm64,
		&tmpl.CBLMarinerV2Gen2,
	}
	wg := sync.WaitGroup{}
	wg.Add(len(vhds))
	// resourceID fetching can be slow, some concurrency to speed things up
	for _, vhd := range vhds {
		go func(vhd *VHD) {
			defer wg.Done()
			err := setResourceID(ctx, vhd, config.VHDBuildID)
			if err != nil {
				log.Printf("Failed to set resource ID for VHD %q: %v", vhd.ImageID, err)
			} else {
				log.Printf("Successfully set resource ID for VHD %q: %s", vhd.ImageID, vhd.ResourceID)
			}
		}(vhd)
	}
	wg.Wait()
	return nil
}

func setResourceID(ctx context.Context, vhd *VHD, buildID string) error {
	if vhd.ResourceID != "" { // resource ID is already set, don't modify it
		return nil
	}
	var err error

	// TODO: should we instead skip scenarios without a VHD?
	if buildID != "" {
		var err error
		vhd.ResourceID, err = findLatestImageWithTag(ctx, vhd.ImageID, "buildId", buildID)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("failed to find latest VHD for %q with build ID %d: %v", vhd.ImageID, buildID, err)
		}
		log.Printf("No image found for %q with build ID %d, falling back to default VHD", vhd.ImageID, buildID)
	}
	vhd.ResourceID, err = findLatestImageWithTag(ctx, vhd.ImageID, vhd.VersionTagName, vhd.VersionTagValue)
	if err != nil {
		return fmt.Errorf("failed to find latest image with tag %q=%q: %v", vhd.VersionTagName, vhd.VersionTagValue, err)
	}
	return nil
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
