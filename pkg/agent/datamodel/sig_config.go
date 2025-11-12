package datamodel

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	AzurePublicCloudSigTenantID     string = "33e01921-4d64-4f8c-a055-5bdaffd5e33d" // AME Tenant
	AzurePublicCloudSigSubscription string = "109a5e88-712a-48ae-9078-9ca8b3c81345" // AKS VHD
)

// SIGAzureEnvironmentSpecConfig is the overall configuration differences in different cloud environments.
/* TODO(tonyxu) merge this with AzureEnvironmentSpecConfig from aks-engine(pkg/api/azenvtypes.go) once
it's moved into AKS RP. */
type SIGAzureEnvironmentSpecConfig struct {
	CloudName                    string                    `json:"cloudName,omitempty"`
	SigTenantID                  string                    `json:"sigTenantID,omitempty"`
	SubscriptionID               string                    `json:"subscriptionID,omitempty"`
	SigUbuntuImageConfig         map[Distro]SigImageConfig `json:"sigUbuntuImageConfig,omitempty"`
	SigCBLMarinerImageConfig     map[Distro]SigImageConfig `json:"sigCBLMarinerImageConfig,omitempty"`
	SigAzureLinuxImageConfig     map[Distro]SigImageConfig `json:"sigAzureLinuxImageConfig,omitempty"`
	SigWindowsImageConfig        map[Distro]SigImageConfig `json:"sigWindowsImageConfig,omitempty"`
	SigUbuntuEdgeZoneImageConfig map[Distro]SigImageConfig `json:"sigUbuntuEdgeZoneImageConfig,omitempty"`
	SigFlatcarImageConfig        map[Distro]SigImageConfig `json:"sigFlatcarImageConfig,omitempty"`
	// TODO(adadilli) add PIR constants as well
}

// EnvironmentInfo represents the set of required fields to determine specifics
// about the customer's environment. This info will be used in baker APIs to
// return the correct SIG image config.
type EnvironmentInfo struct {
	// SubscriptionID is the customer's subscription ID.
	SubscriptionID string
	// TenantID is the customer's tenant ID.
	TenantID string
	// Region is the customer's region (e.g. eastus).
	Region string
}

// SIGConfig is used to hold configuration parameters to access AKS VHDs stored in a SIG.
type SIGConfig struct {
	TenantID       string                      `json:"tenantID"`
	SubscriptionID string                      `json:"subscriptionID"`
	Galleries      map[string]SIGGalleryConfig `json:"galleries"`
}

type SIGGalleryConfig struct {
	GalleryName   string `json:"galleryName"`
	ResourceGroup string `json:"resourceGroup"`
}

type SigImageConfigOpt func(*SigImageConfig)

func GetCloudTargetEnv(location string) string {
	loc := strings.ToLower(strings.Join(strings.Fields(location), ""))
	switch {
	case strings.HasPrefix(loc, "china"):
		return AzureChinaCloud
	case loc == "germanynortheast" || loc == "germanycentral":
		return AzureGermanCloud
	case strings.HasPrefix(loc, "usgov") || strings.HasPrefix(loc, "usdod"):
		return AzureUSGovernmentCloud
	case strings.HasPrefix(loc, "bleu"):
		return AzureBleuCloud
	case strings.HasPrefix(loc, "delos"):
		return AzureGermanyCloud
	case strings.HasPrefix(strings.ToLower(loc), "usnat"):
		return USNatCloud
	case strings.HasPrefix(strings.ToLower(loc), "ussec"):
		return USSecCloud
	default:
		return AzurePublicCloud
	}
}

/*
AvailableUbuntu1804Distros : TODO(amaheshwari): these vars are not consumed by Agentbaker but by RP. do a
cleanup to remove these after 20.04 work.
*/
//nolint:gochecknoglobals
var AvailableUbuntu1804Distros = []Distro{
	AKSUbuntu1804,
	AKSUbuntu1804Gen2,
	AKSUbuntuGPU1804,
	AKSUbuntuGPU1804Gen2,
	AKSUbuntuContainerd1804,
	AKSUbuntuContainerd1804Gen2,
	AKSUbuntuGPUContainerd1804,
	AKSUbuntuGPUContainerd1804Gen2,
	AKSUbuntuFipsContainerd1804,
	AKSUbuntuFipsContainerd1804Gen2,
	AKSUbuntuEdgeZoneContainerd1804,
	AKSUbuntuEdgeZoneContainerd1804Gen2,
}

//nolint:gochecknoglobals
var AvailableUbuntu2004Distros = []Distro{
	AKSUbuntuContainerd2004CVMGen2,
	AKSUbuntuFipsContainerd2004,
	AKSUbuntuFipsContainerd2004Gen2,
}

//nolint:gochecknoglobals
var AvailableUbuntu2204Distros = []Distro{
	AKSUbuntuContainerd2204,
	AKSUbuntuContainerd2204Gen2,
	AKSUbuntuArm64Containerd2204Gen2,
	AKSUbuntuContainerd2204TLGen2,
	AKSUbuntuEdgeZoneContainerd2204,
	AKSUbuntuEdgeZoneContainerd2204Gen2,
	AKSUbuntuMinimalContainerd2204,
	AKSUbuntuMinimalContainerd2204Gen2,
	AKSUbuntuFipsContainerd2204,
	AKSUbuntuFipsContainerd2204Gen2,
}

//nolint:gochecknoglobals
var AvailableUbuntu2404Distros = []Distro{
	AKSUbuntuContainerd2404,
	AKSUbuntuContainerd2404Gen2,
	AKSUbuntuArm64Containerd2404Gen2,
	AKSUbuntuArm64GB200Containerd2404Gen2,
	AKSUbuntuContainerd2404CVMGen2,
	AKSUbuntuContainerd2404TLGen2,
}

//nolint:gochecknoglobals
var AvailableContainerdDistros = []Distro{
	AKSUbuntuContainerd1804,
	AKSUbuntuContainerd1804Gen2,
	AKSUbuntuGPUContainerd1804,
	AKSUbuntuGPUContainerd1804Gen2,
	AKSUbuntuFipsContainerd1804,
	AKSUbuntuFipsContainerd1804Gen2,
	AKSUbuntuFipsContainerd2004,
	AKSUbuntuFipsContainerd2004Gen2,
	AKSUbuntuFipsContainerd2204,
	AKSUbuntuFipsContainerd2204Gen2,
	AKSUbuntuEdgeZoneContainerd1804,
	AKSUbuntuEdgeZoneContainerd1804Gen2,
	AKSFlatcarGen2,
	AKSFlatcarArm64Gen2,
	AKSCBLMarinerV1,
	AKSCBLMarinerV2,
	AKSAzureLinuxV2,
	AKSAzureLinuxV3,
	AKSCBLMarinerV2Gen2,
	AKSAzureLinuxV2Gen2,
	AKSAzureLinuxV3Gen2,
	AKSCBLMarinerV2FIPS,
	AKSAzureLinuxV2FIPS,
	AKSAzureLinuxV3FIPS,
	AKSCBLMarinerV2Gen2FIPS,
	AKSAzureLinuxV2Gen2FIPS,
	AKSAzureLinuxV3Gen2FIPS,
	AKSCBLMarinerV2Gen2Kata,
	AKSAzureLinuxV2Gen2Kata,
	AKSAzureLinuxV3Gen2Kata,
	AKSCBLMarinerV2Gen2TL,
	AKSAzureLinuxV2Gen2TL,
	AKSAzureLinuxV3Gen2TL,
	AKSCBLMarinerV2KataGen2TL,
	AKSUbuntuArm64Containerd2204Gen2,
	AKSUbuntuArm64Containerd2404Gen2,
	AKSUbuntuArm64GB200Containerd2404Gen2,
	AKSUbuntuContainerd2404CVMGen2,
	AKSCBLMarinerV2Arm64Gen2,
	AKSAzureLinuxV2Arm64Gen2,
	AKSAzureLinuxV3Arm64Gen2,
	AKSAzureLinuxV3Arm64Gen2FIPS,
	AKSUbuntuContainerd2204,
	AKSUbuntuContainerd2204Gen2,
	AKSUbuntuContainerd2004CVMGen2,
	AKSUbuntuContainerd2204TLGen2,
	AKSUbuntuEdgeZoneContainerd2204,
	AKSUbuntuEdgeZoneContainerd2204Gen2,
	AKSUbuntuMinimalContainerd2204,
	AKSUbuntuMinimalContainerd2204Gen2,
	AKSUbuntuContainerd2404,
	AKSUbuntuContainerd2404Gen2,
	AKSAzureLinuxV3CVMGen2,
	AKSUbuntuContainerd2404TLGen2,
	AKSAzureLinuxV3OSGuardGen2FIPSTL,
}

//nolint:gochecknoglobals
var AvailableGPUDistros = []Distro{
	AKSUbuntuGPU1804,
	AKSUbuntuGPU1804Gen2,
	AKSUbuntuGPUContainerd1804,
	AKSUbuntuGPUContainerd1804Gen2,
}

//nolint:gochecknoglobals
var AvailableGen2Distros = []Distro{
	AKSUbuntu1804Gen2,
	AKSUbuntuGPU1804Gen2,
	AKSUbuntuContainerd1804Gen2,
	AKSUbuntuGPUContainerd1804Gen2,
	AKSUbuntuFipsContainerd1804Gen2,
	AKSUbuntuFipsContainerd2004Gen2,
	AKSUbuntuFipsContainerd2204Gen2,
	AKSUbuntuEdgeZoneContainerd1804Gen2,
	AKSUbuntuArm64Containerd2204Gen2,
	AKSUbuntuArm64Containerd2404Gen2,
	AKSUbuntuArm64GB200Containerd2404Gen2,
	AKSUbuntuContainerd2404CVMGen2,
	AKSUbuntuContainerd2204Gen2,
	AKSUbuntuContainerd2004CVMGen2,
	AKSUbuntuContainerd2204TLGen2,
	AKSUbuntuEdgeZoneContainerd2204Gen2,
	AKSUbuntuMinimalContainerd2204Gen2,
	AKSUbuntuContainerd2404Gen2,
	AKSUbuntuContainerd2404TLGen2,
	AKSFlatcarGen2,
	AKSFlatcarArm64Gen2,
	AKSCBLMarinerV2Gen2,
	AKSAzureLinuxV2Gen2,
	AKSAzureLinuxV3Gen2,
	AKSCBLMarinerV2Gen2FIPS,
	AKSAzureLinuxV2Gen2FIPS,
	AKSAzureLinuxV3Gen2FIPS,
	AKSCBLMarinerV2Gen2Kata,
	AKSAzureLinuxV2Gen2Kata,
	AKSAzureLinuxV3Gen2Kata,
	AKSCBLMarinerV2Gen2TL,
	AKSAzureLinuxV2Gen2TL,
	AKSAzureLinuxV3Gen2TL,
	AKSCBLMarinerV2KataGen2TL,
	AKSCBLMarinerV2Arm64Gen2,
	AKSAzureLinuxV2Arm64Gen2,
	AKSAzureLinuxV3Arm64Gen2,
	AKSAzureLinuxV3Arm64Gen2FIPS,
	AKSAzureLinuxV3CVMGen2,
	AKSAzureLinuxV3OSGuardGen2FIPSTL,
}

//nolint:gochecknoglobals
var AvailableAzureLinuxDistros = []Distro{
	AKSCBLMarinerV1,
	AKSCBLMarinerV2,
	AKSAzureLinuxV2,
	AKSAzureLinuxV3,
	AKSCBLMarinerV2Gen2,
	AKSAzureLinuxV2Gen2,
	AKSAzureLinuxV3Gen2,
	AKSCBLMarinerV2FIPS,
	AKSAzureLinuxV2FIPS,
	AKSAzureLinuxV3FIPS,
	AKSCBLMarinerV2Gen2FIPS,
	AKSAzureLinuxV2Gen2FIPS,
	AKSAzureLinuxV3Gen2FIPS,
	AKSCBLMarinerV2Gen2Kata,
	AKSAzureLinuxV2Gen2Kata,
	AKSAzureLinuxV3Gen2Kata,
	AKSCBLMarinerV2Arm64Gen2,
	AKSAzureLinuxV2Arm64Gen2,
	AKSAzureLinuxV3Arm64Gen2,
	AKSAzureLinuxV3Arm64Gen2FIPS,
	AKSCBLMarinerV2Gen2TL,
	AKSAzureLinuxV2Gen2TL,
	AKSAzureLinuxV3Gen2TL,
	AKSCBLMarinerV2KataGen2TL,
	AKSAzureLinuxV3CVMGen2,
	AKSAzureLinuxV3OSGuardGen2FIPSTL,
}

//nolint:gochecknoglobals
var AvailableAzureLinuxCgroupV2Distros = []Distro{
	AKSAzureLinuxV2,
	AKSAzureLinuxV3,
	AKSAzureLinuxV2Gen2,
	AKSAzureLinuxV3Gen2,
	AKSAzureLinuxV2FIPS,
	AKSAzureLinuxV3FIPS,
	AKSAzureLinuxV2Gen2FIPS,
	AKSAzureLinuxV3Gen2FIPS,
	AKSCBLMarinerV2Gen2Kata, // Per mheberling, AKSCBLMarinerV2Gen2Kata is equal to AKSAzureLinuxV2Gen2Kata. AKSCBLMarinerV2Gen2Kata is added for now to unblock scenario_kata.
	AKSAzureLinuxV2Gen2Kata,
	AKSAzureLinuxV3Gen2Kata,
	AKSAzureLinuxV2Arm64Gen2,
	AKSAzureLinuxV3Arm64Gen2,
	AKSAzureLinuxV3Arm64Gen2FIPS,
	AKSAzureLinuxV2Gen2TL,
	AKSAzureLinuxV3Gen2TL,
	AKSAzureLinuxV3CVMGen2,
	AKSAzureLinuxV3OSGuardGen2FIPSTL,
}

//nolint:gochecknoglobals
var AvailableAzureLinuxOSGuardDistros = []Distro{
	AKSAzureLinuxV3OSGuardGen2FIPSTL,
}

//nolint:gochecknoglobals
var AvailableFlatcarDistros = []Distro{
	AKSFlatcarGen2,
	AKSFlatcarArm64Gen2,
}

// IsContainerdSKU returns true if distro type is containerd-enabled.
func (d Distro) IsContainerdDistro() bool {
	for _, distro := range AvailableContainerdDistros {
		if d == distro {
			return true
		}
	}
	return false
}

func (d Distro) IsGPUDistro() bool {
	for _, distro := range AvailableGPUDistros {
		if d == distro {
			return true
		}
	}
	return false
}
func (d Distro) IsGen2Distro() bool {
	for _, distro := range AvailableGen2Distros {
		if d == distro {
			return true
		}
	}
	return false
}
func (d Distro) IsAzureLinuxDistro() bool {
	for _, distro := range AvailableAzureLinuxDistros {
		if d == distro {
			return true
		}
	}
	return false
}
func (d Distro) IsWindowsSIGDistro() bool {
	for _, distro := range AvailableWindowsSIGDistros {
		if d == distro {
			return true
		}
	}
	return false
}

func (d Distro) IsWindowsPIRDistro() bool {
	for _, distro := range AvailableWindowsPIRDistros {
		if d == distro {
			return true
		}
	}
	return false
}

func (d Distro) IsWindowsDistro() bool {
	return d.IsWindowsSIGDistro() || d.IsWindowsPIRDistro()
}

// SigImageConfigTemplate represents the SIG image configuration template.
type SigImageConfigTemplate struct {
	ResourceGroup string
	Gallery       string
	Definition    string
	Version       string
}

// SigImageConfig represents the SIG image configuration.
type SigImageConfig struct {
	SigImageConfigTemplate
	SubscriptionID string
}

// WithOptions converts a SigImageConfigTemplate to SigImageConfig instance via function opts.
func (template SigImageConfigTemplate) WithOptions(options ...SigImageConfigOpt) SigImageConfig {
	config := &SigImageConfig{
		SigImageConfigTemplate: template,
	}
	for _, opt := range options {
		opt(config)
	}
	return *config
}

//nolint:gochecknoglobals
var AvailableWindowsSIGDistros = []Distro{
	AKSWindows2019,
	AKSWindows2019Containerd,
	AKSWindows2022Containerd,
	AKSWindows2022ContainerdGen2,
	AKSWindows23H2,
	AKSWindows23H2Gen2,
	AKSWindows2025,
	AKSWindows2025Gen2,
	CustomizedWindowsOSImage,
}

//nolint:gochecknoglobals
var AvailableWindowsPIRDistros = []Distro{
	AKSWindows2019PIR,
}

// SIG const.
const (
	AKSSIGImagePublisher           string = "microsoft-aks"
	AKSWindowsGalleryName          string = "AKSWindows"
	AKSWindowsResourceGroup        string = "AKS-Windows"
	AKSUbuntuGalleryName           string = "AKSUbuntu"
	AKSUbuntuResourceGroup         string = "AKS-Ubuntu"
	AKSCBLMarinerGalleryName       string = "AKSCBLMariner"
	AKSCBLMarinerResourceGroup     string = "AKS-CBLMariner"
	AKSAzureLinuxGalleryName       string = "AKSAzureLinux"
	AKSAzureLinuxResourceGroup     string = "AKS-AzureLinux"
	AKSUbuntuEdgeZoneGalleryName   string = "AKSUbuntuEdgeZone"
	AKSUbuntuEdgeZoneResourceGroup string = "AKS-Ubuntu-EdgeZone"
	AKSFlatcarGalleryName          string = "AKSFlatcar"
	AKSFlatcarResourceGroup        string = "AKS-Flatcar"
)

const (
	// DO NOT MODIFY: used for freezing linux images with docker.
	FrozenLinuxSIGImageVersionForDocker string = "2022.08.29"

	// DO NOT MODIFY: used for freezing linux images for Egress test.
	FrozenLinuxSIGImageVersionForEgressTest string = "2022.10.03"

	// CBLMarinerV1 pinned to the last image build as Mariner 1.0 is out
	//  of support and image builds have stopped.
	FrozenCBLMarinerV1SIGImageVersionForDeprecation string = "202308.28.0"

	// DO NOT MODIFY: All 1804 are frozen since they are EOL
	// 1804GPUContainerd Gen1 & Gen2 pinned to the last image build as GPU Dedicated preview image is being deprecated and image builds have stopped.

	Frozen1804ContainerdSIGImageVersionForDeprecation         string = "202506.16.0"
	Frozen1804Gen2ContainerdSIGImageVersionForDeprecation     string = "202506.16.0"
	Frozen1804FipsContainerdSIGImageVersionForDeprecation     string = "202506.16.0"
	Frozen1804FipsGen2ContainerdSIGImageVersionForDeprecation string = "202506.16.0"
	Frozen1804GPUContainerdSIGImageVersionForDeprecation      string = "202501.05.0"
	Frozen1804Gen2GPUContainerdSIGImageVersionForDeprecation  string = "202501.05.0"

	// DO NOT MODIFY: used for freezing MarinerV2KataGen2TL.
	FrozenCBLMarinerV2KataGen2TLSIGImageVersion = "2022.12.15"

	// DO NOT MODIFY: Used for freezing Kata VHD for kata-cc runtime, until the image is deprecated.
	FrozenCBLMarinerV2KataGen2SIGImageVersion string = "202509.05.0"
	FrozenAzureLinuxV2KataGen2SIGImageVersion string = "202509.05.0"

	// We do not use AKS Windows image versions in AgentBaker. These fake values are only used for unit tests.
	Windows2019SIGImageVersion string = "17763.2019.221114"
	Windows2022SIGImageVersion string = "20348.2022.221114"
	Windows23H2SIGImageVersion string = "25398.2022.221114"
	Windows2025SIGImageVersion string = "26100.2025.221114"
)

type sigVersion struct {
	Version string `json:"version"`
}

//go:embed linux_sig_version.json
var linuxSIGVersionJSONContent []byte

//nolint:gochecknoglobals
var LinuxSIGImageVersion = mustGetSIGVersionFromJSONContent(linuxSIGVersionJSONContent)

// mustGetSIGVersionFromJSONContent returns the SIG image version from within the specified SON content string.
// This is used to populate LinuxSIGImageVersion from linux_sig_version.json.
func mustGetSIGVersionFromJSONContent(contents []byte) string {
	if len(contents) == 0 {
		panic("linux_sig_Version.json content is empty")
	}

	var v sigVersion
	if err := json.Unmarshal(contents, &v); err != nil {
		panic(err)
	}

	return v.Version
}

// SIG config Template.
//
//nolint:gochecknoglobals
var (
	SIGUbuntu1604ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1604",
		Version:       "2021.11.06",
	}

	SIGUbuntu1804ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804",
		Version:       FrozenLinuxSIGImageVersionForDocker,
	}

	SIGUbuntu1804Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gen2",
		Version:       FrozenLinuxSIGImageVersionForDocker,
	}

	SIGUbuntuGPU1804ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gpu",
		Version:       FrozenLinuxSIGImageVersionForDocker,
	}

	SIGUbuntuGPU1804Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gen2gpu",
		Version:       FrozenLinuxSIGImageVersionForDocker,
	}

	SIGUbuntuContainerd1804ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804containerd",
		Version:       Frozen1804ContainerdSIGImageVersionForDeprecation,
	}

	SIGUbuntuContainerd1804Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gen2containerd",
		Version:       Frozen1804Gen2ContainerdSIGImageVersionForDeprecation,
	}

	SIGUbuntuGPUContainerd1804ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gpucontainerd",
		Version:       Frozen1804GPUContainerdSIGImageVersionForDeprecation,
	}

	SIGUbuntuGPUContainerd1804Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gen2gpucontainerd",
		Version:       Frozen1804Gen2GPUContainerdSIGImageVersionForDeprecation,
	}

	SIGUbuntuFipsContainerd1804ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804fipscontainerd",
		Version:       Frozen1804FipsContainerdSIGImageVersionForDeprecation,
	}

	// not a typo, this image was generated on 2021.05.20 UTC and assigned this version.
	SIGUbuntuFipsContainerd1804Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gen2fipscontainerd",
		Version:       Frozen1804FipsGen2ContainerdSIGImageVersionForDeprecation,
	}

	SIGUbuntuFipsContainerd2004ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2004fipscontainerd",
		Version:       LinuxSIGImageVersion,
	}

	// not a typo, this image was generated on 2021.05.20 UTC and assigned this version.
	SIGUbuntuFipsContainerd2004Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2004gen2fipscontainerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuFipsContainerd2204ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2204fipscontainerd",
		Version:       "202404.09.0", // TODO(artunduman): Update version when the image is ready
	}

	SIGUbuntuFipsContainerd2204Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2204gen2fipscontainerd",
		Version:       "202404.09.0", // TODO(artunduman): Update version when the image is ready
	}

	SIGUbuntuArm64Containerd2204Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2204gen2arm64containerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuArm64Containerd2404Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2404gen2arm64containerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuArm64GB200Containerd2404Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2404gen2arm64gb200containerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuContainerd2404CVMGen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2404gen2CVMcontainerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuContainerd2204ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2204containerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuContainerd2204Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2204gen2containerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuContainerd2204TLGen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2204gen2TLcontainerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuContainerd2004CVMGen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2004gen2CVMcontainerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuMinimalContainerd2204ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2204minimalcontainerd",
		Version:       "202401.12.0",
	}

	SIGUbuntuMinimalContainerd2204Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2204gen2minimalcontainerd",
		Version:       "202401.12.0",
	}

	SIGUbuntuEgressContainerd2204Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2204gen2containerd",
		Version:       FrozenLinuxSIGImageVersionForEgressTest,
	}

	SIGUbuntuContainerd2404ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2404containerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuContainerd2404Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2404gen2containerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuContainerd2404TLGen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2404gen2TLcontainerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV1ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V1",
		Version:       FrozenCBLMarinerV1SIGImageVersionForDeprecation,
	}

	SIGCBLMarinerV2Gen1ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV2Gen1ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V2",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV3Gen1ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V3",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2gen2",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV2Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V2gen2",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV3Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V3gen2",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2Gen1FIPSImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2fips",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV2Gen1FIPSImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V2fips",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV3Gen1FIPSImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V3fips",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2Gen2FIPSImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2gen2fips",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV2Gen2FIPSImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V2gen2fips",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV3Gen2FIPSImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V3gen2fips",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV3Arm64Gen2FIPSImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V3gen2arm64fips",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2KataImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2katagen2",
		Version:       FrozenCBLMarinerV2KataGen2SIGImageVersion,
	}

	SIGAzureLinuxV2KataImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V2katagen2",
		Version:       FrozenAzureLinuxV2KataGen2SIGImageVersion,
	}

	SIGAzureLinuxV3KataImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V3katagen2",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2Arm64ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2gen2arm64",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV2Arm64ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V2gen2arm64",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV3Arm64ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V3gen2arm64",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2TLImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2gen2TL",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV2TLImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V2gen2TL",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV3TLImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V3gen2TL",
		Version:       LinuxSIGImageVersion,
	}

	SIGAzureLinuxV3OSGuardGen2FIPSTLImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "OSGuardV3gen2fipsTL",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2KataGen2TLImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2katagen2TL",
		Version:       FrozenCBLMarinerV2KataGen2TLSIGImageVersion,
	}

	SIGFlatcarGen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSFlatcarResourceGroup,
		Gallery:       AKSFlatcarGalleryName,
		Definition:    "flatcargen2",
		Version:       LinuxSIGImageVersion,
	}

	SIGFlatcarArm64Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSFlatcarResourceGroup,
		Gallery:       AKSFlatcarGalleryName,
		Definition:    "flatcargen2arm64",
		Version:       LinuxSIGImageVersion,
	}

	SIGWindows2019ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSWindowsResourceGroup,
		Gallery:       AKSWindowsGalleryName,
		Definition:    "windows-2019",
		Version:       Windows2019SIGImageVersion,
	}

	SIGWindows2019ContainerdImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSWindowsResourceGroup,
		Gallery:       AKSWindowsGalleryName,
		Definition:    "windows-2019-containerd",
		Version:       Windows2019SIGImageVersion,
	}

	SIGWindows2022ContainerdImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSWindowsResourceGroup,
		Gallery:       AKSWindowsGalleryName,
		Definition:    "windows-2022-containerd",
		Version:       Windows2022SIGImageVersion,
	}

	SIGWindows2022ContainerdGen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSWindowsResourceGroup,
		Gallery:       AKSWindowsGalleryName,
		Definition:    "windows-2022-containerd-gen2",
		Version:       Windows2022SIGImageVersion,
	}

	SIGWindows23H2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSWindowsResourceGroup,
		Gallery:       AKSWindowsGalleryName,
		Definition:    "windows-23H2",
		Version:       Windows23H2SIGImageVersion,
	}

	SIGWindows23H2Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSWindowsResourceGroup,
		Gallery:       AKSWindowsGalleryName,
		Definition:    "windows-23H2-gen2",
		Version:       Windows23H2SIGImageVersion,
	}

	SIGAzureLinuxV3CVMGen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSAzureLinuxResourceGroup,
		Gallery:       AKSAzureLinuxGalleryName,
		Definition:    "V3gen2CVM",
		Version:       LinuxSIGImageVersion,
	}

	SIGWindows2025ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSWindowsResourceGroup,
		Gallery:       AKSWindowsGalleryName,
		Definition:    "windows-2025",
		Version:       Windows2025SIGImageVersion,
	}

	SIGWindows2025Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSWindowsResourceGroup,
		Gallery:       AKSWindowsGalleryName,
		Definition:    "windows-2025-gen2",
		Version:       Windows2025SIGImageVersion,
	}
)

// GomegaString provides a custom string representation for Gomega assertions to avoid truncation when comparing large maps or structs.
func (s SigImageConfig) GomegaString() string {
	return fmt.Sprintf("SigImageConfig{ResourceGroup: %s, Gallery: %s, Definition: %s, Version: %s}",
		s.ResourceGroup, s.Gallery, s.Definition, s.Version)
}

// GetMaintainedLinuxSIGImageConfigMap returns a set of Distro -> SigImageConfig mappings
// for ALL Linux distros that are currently built and maintained by AKS Node SIG (Version == LinuxSIGImageVersion).
// Note that each distro's SigImageConfig SubscriptionID field will be empty.
// This can be used downstream to make sure that all expected images have been properly replicated.
// NOTE: corresponding unit tests need to be updated whenever any new distros are added or existing distros are frozen.
func GetMaintainedLinuxSIGImageConfigMap() map[Distro]SigImageConfig {
	// no opts means subscriptionID will be empty in the corresponding image configs
	imageConfigMaps := []map[Distro]SigImageConfig{
		getSigUbuntuImageConfigMapWithOpts(),
		getSigCBLMarinerImageConfigMapWithOpts(),
		getSigAzureLinuxImageConfigMapWithOpts(),
		getSigFlatcarImageConfigMapWithOpts(),
	}

	maintained := map[Distro]SigImageConfig{}
	for _, m := range imageConfigMaps {
		for distro, config := range m {
			if config.Version == LinuxSIGImageVersion {
				maintained[distro] = config
			}
		}
	}

	return maintained
}

func getSigUbuntuImageConfigMapWithOpts(opts ...SigImageConfigOpt) map[Distro]SigImageConfig {
	return map[Distro]SigImageConfig{
		AKSUbuntu1604:                         SIGUbuntu1604ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntu1804:                         SIGUbuntu1804ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntu1804Gen2:                     SIGUbuntu1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuGPU1804:                      SIGUbuntuGPU1804ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuGPU1804Gen2:                  SIGUbuntuGPU1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd1804:               SIGUbuntuContainerd1804ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd1804Gen2:           SIGUbuntuContainerd1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuGPUContainerd1804:            SIGUbuntuGPUContainerd1804ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuGPUContainerd1804Gen2:        SIGUbuntuGPUContainerd1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuFipsContainerd1804:           SIGUbuntuFipsContainerd1804ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuFipsContainerd1804Gen2:       SIGUbuntuFipsContainerd1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuFipsContainerd2004:           SIGUbuntuFipsContainerd2004ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuFipsContainerd2004Gen2:       SIGUbuntuFipsContainerd2004Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuFipsContainerd2204:           SIGUbuntuFipsContainerd2204ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuFipsContainerd2204Gen2:       SIGUbuntuFipsContainerd2204Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2204:               SIGUbuntuContainerd2204ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2204Gen2:           SIGUbuntuContainerd2204Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2004CVMGen2:        SIGUbuntuContainerd2004CVMGen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuArm64Containerd2204Gen2:      SIGUbuntuArm64Containerd2204Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuArm64Containerd2404Gen2:      SIGUbuntuArm64Containerd2404Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuArm64GB200Containerd2404Gen2: SIGUbuntuArm64GB200Containerd2404Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2404CVMGen2:        SIGUbuntuContainerd2404CVMGen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2204TLGen2:         SIGUbuntuContainerd2204TLGen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuMinimalContainerd2204:        SIGUbuntuMinimalContainerd2204ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuMinimalContainerd2204Gen2:    SIGUbuntuMinimalContainerd2204Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuEgressContainerd2204Gen2:     SIGUbuntuEgressContainerd2204Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2404:               SIGUbuntuContainerd2404ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2404Gen2:           SIGUbuntuContainerd2404Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2404TLGen2:         SIGUbuntuContainerd2404TLGen2ImageConfigTemplate.WithOptions(opts...),
	}
}

func getSigCBLMarinerImageConfigMapWithOpts(opts ...SigImageConfigOpt) map[Distro]SigImageConfig {
	return map[Distro]SigImageConfig{
		AKSCBLMarinerV1:           SIGCBLMarinerV1ImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2:           SIGCBLMarinerV2Gen1ImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Gen2:       SIGCBLMarinerV2Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2FIPS:       SIGCBLMarinerV2Gen1FIPSImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Gen2FIPS:   SIGCBLMarinerV2Gen2FIPSImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Gen2Kata:   SIGCBLMarinerV2KataImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Arm64Gen2:  SIGCBLMarinerV2Arm64ImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Gen2TL:     SIGCBLMarinerV2TLImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2KataGen2TL: SIGCBLMarinerV2KataGen2TLImageConfigTemplate.WithOptions(opts...),
	}
}

func getSigAzureLinuxImageConfigMapWithOpts(opts ...SigImageConfigOpt) map[Distro]SigImageConfig {
	return map[Distro]SigImageConfig{
		AKSAzureLinuxV2:                  SIGAzureLinuxV2Gen1ImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV3:                  SIGAzureLinuxV3Gen1ImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV2Gen2:              SIGAzureLinuxV2Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV3Gen2:              SIGAzureLinuxV3Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV2FIPS:              SIGAzureLinuxV2Gen1FIPSImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV3FIPS:              SIGAzureLinuxV3Gen1FIPSImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV2Gen2FIPS:          SIGAzureLinuxV2Gen2FIPSImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV3Gen2FIPS:          SIGAzureLinuxV3Gen2FIPSImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV2Gen2Kata:          SIGAzureLinuxV2KataImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV3Gen2Kata:          SIGAzureLinuxV3KataImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV2Arm64Gen2:         SIGAzureLinuxV2Arm64ImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV3Arm64Gen2:         SIGAzureLinuxV3Arm64ImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV3Arm64Gen2FIPS:     SIGAzureLinuxV3Arm64Gen2FIPSImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV2Gen2TL:            SIGAzureLinuxV2TLImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV3Gen2TL:            SIGAzureLinuxV3TLImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV3CVMGen2:           SIGAzureLinuxV3CVMGen2ImageConfigTemplate.WithOptions(opts...),
		AKSAzureLinuxV3OSGuardGen2FIPSTL: SIGAzureLinuxV3OSGuardGen2FIPSTLImageConfigTemplate.WithOptions(opts...),
	}
}

func getSigFlatcarImageConfigMapWithOpts(opts ...SigImageConfigOpt) map[Distro]SigImageConfig {
	return map[Distro]SigImageConfig{
		AKSFlatcarGen2:      SIGFlatcarGen2ImageConfigTemplate.WithOptions(opts...),
		AKSFlatcarArm64Gen2: SIGFlatcarArm64Gen2ImageConfigTemplate.WithOptions(opts...),
	}
}

func getSigWindowsImageConfigMapWithOpts(opts ...SigImageConfigOpt) map[Distro]SigImageConfig {
	return map[Distro]SigImageConfig{
		AKSWindows2019:               SIGWindows2019ImageConfigTemplate.WithOptions(opts...),
		AKSWindows2019Containerd:     SIGWindows2019ContainerdImageConfigTemplate.WithOptions(opts...),
		AKSWindows2022Containerd:     SIGWindows2022ContainerdImageConfigTemplate.WithOptions(opts...),
		AKSWindows2022ContainerdGen2: SIGWindows2022ContainerdGen2ImageConfigTemplate.WithOptions(opts...),
		AKSWindows23H2:               SIGWindows23H2ImageConfigTemplate.WithOptions(opts...),
		AKSWindows23H2Gen2:           SIGWindows23H2Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSWindows2025:               SIGWindows2025ImageConfigTemplate.WithOptions(opts...),
		AKSWindows2025Gen2:           SIGWindows2025Gen2ImageConfigTemplate.WithOptions(opts...),
	}
}

func getSigUbuntuEdgeZoneImageConfigMapWithOpts(opts ...SigImageConfigOpt) map[Distro]SigImageConfig {
	// This image is using a specific resource group and gallery name for edge zone scenario.
	sigUbuntuEdgeZoneContainerd1804ImageConfigTemplate := SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuEdgeZoneResourceGroup,
		Gallery:       AKSUbuntuEdgeZoneGalleryName,
		Definition:    "1804containerd",
		Version:       Frozen1804ContainerdSIGImageVersionForDeprecation,
	}

	// This image is using a specific resource group and gallery name for edge zone scenario.
	sigUbuntuEdgeZoneContainerd1804Gen2ImageConfigTemplate := SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuEdgeZoneResourceGroup,
		Gallery:       AKSUbuntuEdgeZoneGalleryName,
		Definition:    "1804gen2containerd",
		Version:       Frozen1804Gen2ContainerdSIGImageVersionForDeprecation,
	}

	// This image is using a specific resource group and gallery name for edge zone scenario.
	sigUbuntuEdgeZoneContainerd2204ImageConfigTemplate := SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuEdgeZoneResourceGroup,
		Gallery:       AKSUbuntuEdgeZoneGalleryName,
		Definition:    "2204containerd",
		Version:       LinuxSIGImageVersion,
	}

	// This image is using a specific resource group and gallery name for edge zone scenario.
	sigUbuntuEdgeZoneContainerd2204Gen2ImageConfigTemplate := SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuEdgeZoneResourceGroup,
		Gallery:       AKSUbuntuEdgeZoneGalleryName,
		Definition:    "2204gen2containerd",
		Version:       LinuxSIGImageVersion,
	}

	return map[Distro]SigImageConfig{
		AKSUbuntuEdgeZoneContainerd1804:     sigUbuntuEdgeZoneContainerd1804ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuEdgeZoneContainerd1804Gen2: sigUbuntuEdgeZoneContainerd1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuEdgeZoneContainerd2204:     sigUbuntuEdgeZoneContainerd2204ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuEdgeZoneContainerd2204Gen2: sigUbuntuEdgeZoneContainerd2204Gen2ImageConfigTemplate.WithOptions(opts...),
	}
}

// GetSIGAzureCloudSpecConfig get cloud specific sig config.
func GetSIGAzureCloudSpecConfig(sigConfig SIGConfig, region string) (SIGAzureEnvironmentSpecConfig, error) {
	if sigConfig.Galleries == nil || strings.EqualFold(sigConfig.SubscriptionID, "") || strings.EqualFold(sigConfig.TenantID, "") {
		return SIGAzureEnvironmentSpecConfig{}, fmt.Errorf("acsConfig.rpConfig.sigConfig missing expected values - cannot generate sig env config")
	}

	c := new(SIGAzureEnvironmentSpecConfig)
	c.SigTenantID = sigConfig.TenantID
	c.SubscriptionID = sigConfig.SubscriptionID
	c.CloudName = GetCloudTargetEnv(region)

	fromACSUbuntu, err := withACSSIGConfig(sigConfig, "AKSUbuntu")
	if err != nil {
		return SIGAzureEnvironmentSpecConfig{}, fmt.Errorf("unexpected error while constructing env-aware sig configuration for AKSUbuntu: %w", err)
	}
	c.SigUbuntuImageConfig = getSigUbuntuImageConfigMapWithOpts(fromACSUbuntu)

	fromACSCBLMariner, err := withACSSIGConfig(sigConfig, "AKSCBLMariner")
	if err != nil {
		return SIGAzureEnvironmentSpecConfig{}, fmt.Errorf("unexpected error while constructing env-aware sig configuration for AKSCBLMariner: %w", err)
	}
	c.SigCBLMarinerImageConfig = getSigCBLMarinerImageConfigMapWithOpts(fromACSCBLMariner)

	fromACSAzureLinux, err := withACSSIGConfig(sigConfig, "AKSAzureLinux")
	if err != nil {
		return SIGAzureEnvironmentSpecConfig{}, fmt.Errorf("unexpected error while constructing env-aware sig configuration for AKSAzureLinux: %w", err)
	}
	c.SigAzureLinuxImageConfig = getSigAzureLinuxImageConfigMapWithOpts(fromACSAzureLinux)

	// TODO: use withACSConfig when the gallery config is available within SIGConfig (ACSConfig) provided by the resource provider.
	fromACSFlatcar := withACSSIGConfigWithDefaults(sigConfig, "AKSFlatcar", AKSFlatcarGalleryName, AKSFlatcarResourceGroup)
	c.SigFlatcarImageConfig = getSigFlatcarImageConfigMapWithOpts(fromACSFlatcar)

	fromACSWindows, err := withACSSIGConfig(sigConfig, "AKSWindows")
	if err != nil {
		return SIGAzureEnvironmentSpecConfig{}, fmt.Errorf("unexpected error while constructing env-aware sig configuration for Windows: %w", err)
	}
	c.SigWindowsImageConfig = getSigWindowsImageConfigMapWithOpts(fromACSWindows)

	fromACSUbuntuEdgeZone := withEdgeZoneConfig(sigConfig)
	c.SigUbuntuEdgeZoneImageConfig = getSigUbuntuEdgeZoneImageConfigMapWithOpts(fromACSUbuntuEdgeZone)
	return *c, nil
}

/*
GetAzurePublicSIGConfigForTest returns a statically defined sigconfig. This should only be used for
unit tests and e2es.
*/
func GetAzurePublicSIGConfigForTest() SIGAzureEnvironmentSpecConfig {
	return SIGAzureEnvironmentSpecConfig{
		CloudName:                    AzurePublicCloud,
		SigTenantID:                  AzurePublicCloudSigTenantID,
		SubscriptionID:               AzurePublicCloudSigSubscription,
		SigUbuntuImageConfig:         getSigUbuntuImageConfigMapWithOpts(withSubscription(AzurePublicCloudSigSubscription)),
		SigCBLMarinerImageConfig:     getSigCBLMarinerImageConfigMapWithOpts(withSubscription(AzurePublicCloudSigSubscription)),
		SigAzureLinuxImageConfig:     getSigAzureLinuxImageConfigMapWithOpts(withSubscription(AzurePublicCloudSigSubscription)),
		SigWindowsImageConfig:        getSigWindowsImageConfigMapWithOpts(withSubscription(AzurePublicCloudSigSubscription)),
		SigUbuntuEdgeZoneImageConfig: getSigUbuntuEdgeZoneImageConfigMapWithOpts(withSubscription(AzurePublicCloudSigSubscription)),
		SigFlatcarImageConfig:        getSigFlatcarImageConfigMapWithOpts(withSubscription(AzurePublicCloudSigSubscription)),
	}
}

// withACSSIGConfigWithDefaults functions the same as withACSSIGConfig, but uses a default gallery and resource group
// if no corresponding gallery config is found for the particular OS SKU. This is needed to support agentbaker E2E tests
// using the specified SKU while also ignoring cases where ACSConfig provided by the resource provider in production
// doesn't contain the required gallery config for the specified OS SKU.
func withACSSIGConfigWithDefaults(acsSigConfig SIGConfig, osSKU, defaultGallery, defaultResourceGroup string) SigImageConfigOpt {
	galleryName := defaultGallery
	resourceGroup := defaultResourceGroup

	gallery, k := acsSigConfig.Galleries[osSKU]
	if k {
		galleryName = gallery.GalleryName
		resourceGroup = gallery.ResourceGroup
	}

	return func(c *SigImageConfig) {
		c.Gallery = galleryName
		c.SubscriptionID = acsSigConfig.SubscriptionID
		c.ResourceGroup = resourceGroup
	}
}

func withACSSIGConfig(acsSigConfig SIGConfig, osSKU string) (SigImageConfigOpt, error) {
	gallery, k := acsSigConfig.Galleries[osSKU]
	if !k {
		return nil, fmt.Errorf("sig gallery configuration for %s not found", osSKU)
	}
	return func(c *SigImageConfig) {
		c.Gallery = gallery.GalleryName
		c.SubscriptionID = acsSigConfig.SubscriptionID
		c.ResourceGroup = gallery.ResourceGroup
	}, nil
}

func withEdgeZoneConfig(acsSigConfig SIGConfig) SigImageConfigOpt {
	return func(c *SigImageConfig) {
		c.Gallery = AKSUbuntuEdgeZoneGalleryName
		c.SubscriptionID = acsSigConfig.SubscriptionID
		c.ResourceGroup = AKSUbuntuEdgeZoneResourceGroup
	}
}

//nolint:unparam //subscriptionID only receives AzurePublicCloudSigSubscription
func withSubscription(subscriptionID string) SigImageConfigOpt {
	return func(c *SigImageConfig) {
		c.SubscriptionID = subscriptionID
	}
}
