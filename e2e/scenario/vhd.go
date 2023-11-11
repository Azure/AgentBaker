package scenario

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	offerNameAzureLinux = "AzureLinux"
)

var (
	//go:embed default_vhd_catalog.json
	embeddedDefaultVHDCatalog string

	DefaultVHDCatalog = mustGetVHDCatalogFromEmbeddedJSON(embeddedDefaultVHDCatalog)
)

type VHDResourceID string

func (id VHDResourceID) Short() string {
	sep := "Microsoft.Compute/galleries/"
	str := string(id)
	if strings.Contains(str, sep) && !strings.HasSuffix(str, sep) {
		return strings.Split(str, sep)[1]
	}
	return str
}

// VHDPublishingInfo represents VHD configuration as parsed from arbitrary
// vhd-publishing-info.json files produced by VHD builds
type VHDPublishingInfo struct {
	CapturedImageVersionResourceID VHDResourceID `json:"captured_sig_resource_id,omitempty"`
	SKUName                        string        `json:"sku_name,omitempty"`
	OfferName                      string        `json:"offer_name,omitempty"`
}

type Ubuntu1804 struct {
	Gen2Containerd VHDResourceID `json:"gen2containerd,omitempty"`
}

type Ubuntu2204 struct {
	Gen2Arm64Containerd VHDResourceID `json:"gen2arm64containerd,omitempty"`
	Gen2Containerd      VHDResourceID `json:"gen2containerd,omitempty"`
}

type AzureLinuxV2 struct {
	Gen2Arm64 VHDResourceID `json:"gen2arm64,omitempty"`
	Gen2      VHDResourceID `json:"gen2,omitempty"`
}

type CBLMarinerV2 struct {
	Gen2Arm64 VHDResourceID `json:"gen2arm64,omitempty"`
	Gen2      VHDResourceID `json:"gen2,omitempty"`
}
type VHDCatalog struct {
	Ubuntu1804   Ubuntu1804   `json:"ubuntu1804,omitempty"`
	Ubuntu2204   Ubuntu2204   `json:"ubuntu2204,omitempty"`
	AzureLinuxV2 AzureLinuxV2 `json:"azurelinuxv2,omitempty"`
	CBLMarinerV2 CBLMarinerV2 `json:"cblmarinerv2,omitempty"`
}

func (c *VHDCatalog) addEntryFromPublishingInfo(info VHDPublishingInfo) {
	switch getVHDNameFromPublishingInfo(info) {
	case "1804gen2containerd":
		c.Ubuntu1804.Gen2Containerd = info.CapturedImageVersionResourceID
	case "2204gen2arm64containerd":
		c.Ubuntu2204.Gen2Arm64Containerd = info.CapturedImageVersionResourceID
	case "2204gen2containerd":
		c.Ubuntu2204.Gen2Containerd = info.CapturedImageVersionResourceID
	case "azurelinuxv2gen2arm64":
		c.AzureLinuxV2.Gen2Arm64 = info.CapturedImageVersionResourceID
	case "azurelinuxv2gen2":
		c.AzureLinuxV2.Gen2 = info.CapturedImageVersionResourceID
	case "v2gen2arm64":
		c.CBLMarinerV2.Gen2Arm64 = info.CapturedImageVersionResourceID
	case "v2gen2":
		c.CBLMarinerV2.Gen2 = info.CapturedImageVersionResourceID
	}
}

func (c *VHDCatalog) addEntriesFromPublishingInfos(dirName string) error {
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

		info := VHDPublishingInfo{}
		if err := json.Unmarshal(data, &info); err != nil {
			return fmt.Errorf("unable to unmarshal publishing info file %s: %w", filePath, err)
		}

		c.addEntryFromPublishingInfo(info)
	}

	return nil
}

// 2204gen2containerd, v2gen2, azurelinuxv2gen2arm64, etc.
func getVHDNameFromPublishingInfo(info VHDPublishingInfo) string {
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
