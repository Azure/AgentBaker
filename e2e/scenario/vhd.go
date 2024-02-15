package scenario

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Azure/agentbakere2e/artifact"
	"github.com/Azure/agentbakere2e/suite"
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

func getVHDsFromBuild(ctx context.Context, suiteConfig *suite.Config, tmpl *Template, scenarios []*Scenario) error {
	downloader, err := artifact.NewDownloader(ctx, suiteConfig)
	if err != nil {
		return fmt.Errorf("unable to construct new ADO artifact downloader: %w", err)
	}

	artifactNames := make(map[string]bool)
	for _, scenario := range scenarios {
		if scenario.VHDSelector == nil {
			return fmt.Errorf("unable to download VHDs from build: scenario %q has an undefined VHDSelector", scenario.Name)
		}
		artifactName := scenario.VHDSelector().ArtifactName
		if artifactName != "" && !artifactNames[artifactName] {
			artifactNames[artifactName] = true
			log.Printf("will download publishing info artifact for: %q", artifactName)
		}
	}

	err = downloader.DownloadVHDBuildPublishingInfo(ctx, artifact.PublishingInfoDownloadOpts{
		BuildID:       suiteConfig.VHDBuildID,
		TargetDir:     artifact.DefaultPublishingInfoDir,
		ArtifactNames: artifactNames,
	})
	defer os.RemoveAll(artifact.DefaultPublishingInfoDir)
	if err != nil {
		return fmt.Errorf("unable to download VHD publishing info: %w", err)
	}

	if err = tmpl.VHDCatalog.addEntriesFromPublishingInfoDir(artifact.DefaultPublishingInfoDir); err != nil {
		return fmt.Errorf("unable to load VHD selections from publishing info dir %s: %w", artifact.DefaultPublishingInfoDir, err)
	}

	return nil
}

// getVHDNameFromPublishingInfo will resolve the name of the VHD from the specified publishing info.
// Resolved names will take the form of: 2204gen2containerd, v2gen2, azurelinuxv2gen2arm64, etc.
func getVHDNameFromPublishingInfo(info artifact.VHDPublishingInfo) string {
	vhdName := strings.ToLower(info.SKUName)
	if info.OfferName == offerNameAzureLinux {
		// explicitly prepend 'azurelinux' to azurelinux VHD names since their SKU
		// names use the same naming convention as CBLMariner-based SKUs.
		vhdName = fmt.Sprintf("azurelinux%s", vhdName)
	}
	return vhdName
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
	// ArtifactName is the name of the VHD's assocaited artifact as found in the published build aritfacts from VHD builds.
	// This is used to template the name of the publishing info artifacts when downloading from ADO - e.g. "publishing-info-<ArtifactName>".
	ArtifactName string `json:"artifactName,omitempty"`
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

// addEntryFromPublishingInfo will add an entry to the catalog based on the specified publishing info.
// Specifically, it will select and overwrite the resource ID of the given VHD object based on the name of the VHD
// inferred from the specified publishing info.
func (c *VHDCatalog) addEntryFromPublishingInfo(info artifact.VHDPublishingInfo) {
	if resourceID := info.CapturedImageVersionResourceID; resourceID != "" {
		id := VHDResourceID(resourceID)
		switch getVHDNameFromPublishingInfo(info) {
		case vhdName1804Gen2:
			c.Ubuntu1804.Gen2Containerd.ResourceID = id
		case vhdName2204Gen2ARM64Containerd:
			c.Ubuntu2204.Gen2Arm64Containerd.ResourceID = id
		case vhdName2204Gen2Containerd:
			c.Ubuntu2204.Gen2Containerd.ResourceID = id
		case vhdNameAzureLinuxV2Gen2ARM64:
			c.AzureLinuxV2.Gen2Arm64.ResourceID = id
		case vhdNameAzureLinuxV2Gen2:
			c.AzureLinuxV2.Gen2.ResourceID = id
		case vhdNameCBLMarinerV2Gen2ARM64:
			c.CBLMarinerV2.Gen2Arm64.ResourceID = id
		case vhdNameCBLMarinerV2Gen2:
			c.CBLMarinerV2.Gen2.ResourceID = id
		}
	}
}

// addEntriesFromPublishingInfoDir will read all the publishing-info-*.json files from the specified directory,
// unmarshal them, and call addEntryFromPublishingInfo on each one respectively.
func (c *VHDCatalog) addEntriesFromPublishingInfoDir(dirName string) error {
	absPath, err := filepath.Abs(dirName)
	if err != nil {
		return fmt.Errorf("unable to resolve absolute path of %s: %w", dirName, err)
	}
	files, err := os.ReadDir(absPath)
	if err != nil {
		return fmt.Errorf("unable to read publishing infos from directory %s: %w", absPath, err)
	}

	for _, file := range files {
		filePath := path.Join(absPath, file.Name())

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("unable to read publishing info file %s: %w", filePath, err)
		}

		info := artifact.VHDPublishingInfo{}
		if err := json.Unmarshal(data, &info); err != nil {
			return fmt.Errorf("unable to unmarshal publishing info file %s: %w", filePath, err)
		}

		c.addEntryFromPublishingInfo(info)
	}

	return nil
}
