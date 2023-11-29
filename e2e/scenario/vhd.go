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

	BaseVHDCatalog = mustGetVHDCatalogFromEmbeddedJSON(embeddedBaseVHDCatalog)
)

func getVHDsFromBuild(ctx context.Context, suiteConfig *suite.Config, tmpl *Template, scenarios []*Scenario) error {
	downloader, err := artifact.NewDownloader(ctx, suiteConfig)
	if err != nil {
		return fmt.Errorf("unable to construct new ADO artifact downloader: %w", err)
	}

	artifacts := make(map[string]bool)
	for _, scenario := range scenarios {
		artifact := scenario.VHDSelector().ArtifactName
		if !artifacts[artifact] {
			artifacts[artifact] = true
			log.Printf("will download publishing info artifact for: %q", artifact)
		}
	}

	err = downloader.DownloadVHDBuildPublishingInfo(ctx, artifact.PublishingInfoDownloadOpts{
		BuildID:   suiteConfig.VHDBuildID,
		TargetDir: artifact.DefaultPublishingInfoDir,
		Artifacts: artifacts,
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

// 2204gen2containerd, v2gen2, azurelinuxv2gen2arm64, etc.
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

type VHDResourceID string

func (id VHDResourceID) Short() string {
	sep := "Microsoft.Compute/galleries/"
	str := string(id)
	if strings.Contains(str, sep) && !strings.HasSuffix(str, sep) {
		return strings.Split(str, sep)[1]
	}
	return str
}

type VHD struct {
	ArtifactName string        `json:"artifactName,omitempty"`
	ResourceID   VHDResourceID `json:"resourceId,omitempty"`
}

type VHDCatalog struct {
	Ubuntu1804   Ubuntu1804   `json:"ubuntu1804,omitempty"`
	Ubuntu2204   Ubuntu2204   `json:"ubuntu2204,omitempty"`
	AzureLinuxV2 AzureLinuxV2 `json:"azurelinuxv2,omitempty"`
	CBLMarinerV2 CBLMarinerV2 `json:"cblmarinerv2,omitempty"`
}

type Ubuntu1804 struct {
	Gen2Containerd VHD `json:"gen2containerd,omitempty"`
}

type Ubuntu2204 struct {
	Gen2Arm64Containerd VHD `json:"gen2arm64containerd,omitempty"`
	Gen2Containerd      VHD `json:"gen2containerd,omitempty"`
}

type AzureLinuxV2 struct {
	Gen2Arm64 VHD `json:"gen2arm64,omitempty"`
	Gen2      VHD `json:"gen2,omitempty"`
}

type CBLMarinerV2 struct {
	Gen2Arm64 VHD `json:"gen2arm64,omitempty"`
	Gen2      VHD `json:"gen2,omitempty"`
}

func (c *VHDCatalog) Ubuntu1804Gen2Containerd() VHD {
	return c.Ubuntu1804.Gen2Containerd
}

func (c *VHDCatalog) Ubuntu2204Gen2ARM64Containerd() VHD {
	return c.Ubuntu2204.Gen2Arm64Containerd
}

func (c *VHDCatalog) Ubuntu2204Gen2Containerd() VHD {
	return c.Ubuntu2204.Gen2Containerd
}

func (c *VHDCatalog) AzureLinuxV2Gen2ARM64() VHD {
	return c.AzureLinuxV2.Gen2Arm64
}

func (c *VHDCatalog) AzureLinuxV2Gen2() VHD {
	return c.AzureLinuxV2.Gen2
}

func (c *VHDCatalog) CBLMarinerV2Gen2ARM64() VHD {
	return c.CBLMarinerV2.Gen2Arm64
}

func (c *VHDCatalog) CBLMarinerV2Gen2() VHD {
	return c.CBLMarinerV2.Gen2
}

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
