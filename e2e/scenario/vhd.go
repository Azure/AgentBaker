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
	"strconv"
	"strconv"
	"strings"
	"sync"

	"github.com/Azure/agentbakere2e/artifact"
	"github.com/Azure/agentbakere2e/config"
)

const (
	offerNameAzureLinux = "AzureLinux"

	vhdName1804Gen2                = "1804gen2containerd"
	vhdName2204Gen2ARM64Containerd = "2204gen2arm64containerd"
	vhdName2204Gen2Containerd      = "2204gen2containerd"
	vhdNameAzureLinuxV2Gen2ARM64   = "azurelinuxv2gen2arm64"
	vhdNameAzureLinuxV2Gen2        = "azurelinuxv2gen2"
	vhdNameCBLMarinerV2Gen2ARM64   = "v2gen2arm64"
	vhdNameCBLMarinerV2Gen2        = "v2gen2"
)

var (
	//go:embed base_vhd_catalog.json
	embeddedBaseVHDCatalog string

	// BaseVHDCatalog represents the base VHD catalog that every E2E suite will start off of.
	// It contains the set of VHDs used by AgentBaker E2Es, along with the specific versions and artifact name for each.
	// When a VHD build ID is specified, this catalog's entries will be overwritten respectively for each downloaded VHD publishing info.
	BaseVHDCatalog = mustGetVHDCatalogFromEmbeddedJSON(embeddedBaseVHDCatalog)
)

func getVHDsFromBuild(ctx context.Context, tmpl *Template, scenarios []*Scenario) error {
	if config.VHDBuildID == "" {
		return nil
	}

	buildID, err := strconv.Atoi(config.VHDBuildID)
	if err != nil {
		return fmt.Errorf("unable to convert build ID %s to int: %w", config.VHDBuildID, err)
	}

	vhds := []*VHD{
		&tmpl.Ubuntu1804.Gen2Containerd,
		&tmpl.Ubuntu2204.Gen2Arm64Containerd,
		&tmpl.Ubuntu2204.Gen2Containerd,
		&tmpl.Ubuntu2204.Gen2ContainerdPrivateKubePkg,
		&tmpl.AzureLinuxV2.Gen2Arm64,
		&tmpl.AzureLinuxV2.Gen2,
		&tmpl.CBLMarinerV2.Gen2Arm64,
		&tmpl.CBLMarinerV2.Gen2,
	}
	wg := sync.WaitGroup{}
	wg.Add(len(vhds))
	// resourceID fetching can be slow, some concurrency to speed things up
	for _, vhd := range vhds {
		go func(vhd *VHD) {
			defer wg.Done()
			err := setResourceID(ctx, vhd, suiteConfig.VHDBuildID)
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

func setResourceID(ctx context.Context, vhd *VHD, buildID int) error {
	if vhd.ResourceID != "" { // resource ID is already set, don't modify it
		return nil
	}
	var err error

	// TODO: should we instead skip scenarios without a VHD?
	if buildID != 0 {
		var err error
		vhd.ResourceID, err = findLatestImageWithTag(ctx, vhd.ImageID, "buildId", strconv.Itoa(buildID))
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

func mustGetVHDCatalogFromEmbeddedJSON(rawJSON string) VHDCatalog {
	if rawJSON == "" {
		panic("default_vhd_catalog.json is empty")
	}

	catalog := VHDCatalog{}
	if err := json.Unmarshal([]byte(rawJSON), &catalog); err != nil {
		panic(err)
	}

	return catalog
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
	ImageID         string `json:"imageId,omitempty"`
	VersionTagName  string `json:"versionTagName,omitempty"`
	VersionTagValue string `json:"versionTagValue,omitempty"`
	// ResourceID is the resource ID pointing to the underlying VHD in Azure. Based on the current setup, this will always be the resource ID
	// of an image version in a shared image gallery.
	ResourceID VHDResourceID `json:"resourceId,omitempty"`
}

// VHDCatalog is the "catalog" used by the scenario template to offer VHD selections to each individual E2E scenario.
// Each scenario should be configured to choose a VHD from this catalog.
type VHDCatalog struct {
	Ubuntu1804   Ubuntu1804   `json:"ubuntu1804,omitempty"`
	Ubuntu2204   Ubuntu2204   `json:"ubuntu2204,omitempty"`
	AzureLinuxV2 AzureLinuxV2 `json:"azurelinuxv2,omitempty"`
	CBLMarinerV2 CBLMarinerV2 `json:"cblmarinerv2,omitempty"`
}

// Ubuntu1804 contains all the Ubuntu1804-based VHD catalog entries.
type Ubuntu1804 struct {
	Gen2Containerd VHD `json:"gen2containerd,omitempty"`
}

// Ubuntu2204 contains all the Ubuntu2204-based VHD catalog entries.
type Ubuntu2204 struct {
	Gen2Arm64Containerd          VHD `json:"gen2arm64containerd,omitempty"`
	Gen2Containerd               VHD `json:"gen2containerd,omitempty"`
	Gen2ContainerdPrivateKubePkg VHD `json:"gen2containerdprivatekubepkg,omitempty"`
}

// AzureLinuxV2 contains all the AzureLinuxV2-based VHD catalog entries.
type AzureLinuxV2 struct {
	Gen2Arm64 VHD `json:"gen2arm64,omitempty"`
	Gen2      VHD `json:"gen2,omitempty"`
}

// CBLMarinerv2 contains all the CBLMarinerV2-based VHD catalog entries.
type CBLMarinerV2 struct {
	Gen2Arm64 VHD `json:"gen2arm64,omitempty"`
	Gen2      VHD `json:"gen2,omitempty"`
}

// Returns the Ubuntu1804/gen2 catalog entry.
func (c *VHDCatalog) Ubuntu1804Gen2Containerd() VHD {
	return c.Ubuntu1804.Gen2Containerd
}

// Returns the Ubuntu2204/gen2arm64 catalog entry.
func (c *VHDCatalog) Ubuntu2204Gen2ARM64Containerd() VHD {
	return c.Ubuntu2204.Gen2Arm64Containerd
}

// Returns the Ubuntu2204/gen2 catalog entry.
func (c *VHDCatalog) Ubuntu2204Gen2Containerd() VHD {
	return c.Ubuntu2204.Gen2Containerd
}

// Returns the gen2containerdprivatekubepkg catalog entry.
func (c *VHDCatalog) Ubuntu2204Gen2ContainerdPrivateKubePkg() VHD {
	return c.Ubuntu2204.Gen2ContainerdPrivateKubePkg
}

// Returns the AzureLinuxV/gen2arm64 catalog entry.
func (c *VHDCatalog) AzureLinuxV2Gen2ARM64() VHD {
	return c.AzureLinuxV2.Gen2Arm64
}

// Returns the AzureLinuxV2/gen2 catalog entry.
func (c *VHDCatalog) AzureLinuxV2Gen2() VHD {
	return c.AzureLinuxV2.Gen2
}

// Returns the CBLMarinerV2/gen2arm64 catalog entry.
func (c *VHDCatalog) CBLMarinerV2Gen2ARM64() VHD {
	return c.CBLMarinerV2.Gen2Arm64
}

// Returns the CBLMarinerV2/gen2 catalog entry.
func (c *VHDCatalog) CBLMarinerV2Gen2() VHD {
	return c.CBLMarinerV2.Gen2
}
