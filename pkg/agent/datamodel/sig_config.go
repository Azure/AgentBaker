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
	SigWindowsImageConfig        map[Distro]SigImageConfig `json:"sigWindowsImageConfig,omitempty"`
	SigUbuntuEdgeZoneImageConfig map[Distro]SigImageConfig `json:"sigUbuntuEdgeZoneImageConfig,omitempty"`
	// TODO(adadilli) add PIR constants as well
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
	AKSUbuntuEdgeZoneContainerd1804,
	AKSUbuntuEdgeZoneContainerd1804Gen2,
	AKSCBLMarinerV1,
	AKSCBLMarinerV2,
	AKSCBLMarinerV2Gen2,
	AKSCBLMarinerV2FIPS,
	AKSCBLMarinerV2Gen2FIPS,
	AKSCBLMarinerV2Gen2Kata,
	AKSCBLMarinerV2Gen2TL,
	AKSCBLMarinerV2KataGen2TL,
	AKSUbuntuArm64Containerd2204Gen2,
	AKSUbuntuContainerd2204,
	AKSUbuntuContainerd2204Gen2,
	AKSUbuntuContainerd2004CVMGen2,
	AKSUbuntuContainerd2204TLGen2,
	AKSUbuntuEdgeZoneContainerd2204,
	AKSUbuntuEdgeZoneContainerd2204Gen2,
	AKSUbuntuMinimalContainerd2204,
	AKSUbuntuMinimalContainerd2204Gen2,
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
	AKSUbuntuEdgeZoneContainerd1804Gen2,
	AKSUbuntuArm64Containerd2204Gen2,
	AKSUbuntuContainerd2204Gen2,
	AKSUbuntuContainerd2004CVMGen2,
	AKSUbuntuContainerd2204TLGen2,
	AKSUbuntuEdgeZoneContainerd2204Gen2,
	AKSUbuntuMinimalContainerd2204Gen2,
}

//nolint:gochecknoglobals
var AvailableCBLMarinerDistros = []Distro{
	AKSCBLMarinerV1,
	AKSCBLMarinerV2,
	AKSCBLMarinerV2Gen2,
	AKSCBLMarinerV2FIPS,
	AKSCBLMarinerV2Gen2FIPS,
	AKSCBLMarinerV2Gen2Kata,
	AKSCBLMarinerV2Arm64Gen2,
	AKSCBLMarinerV2Gen2TL,
	AKSCBLMarinerV2KataGen2TL,
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
func (d Distro) IsCBLMarinerDistro() bool {
	for _, distro := range AvailableCBLMarinerDistros {
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

// SigImageConfigTemplate represents the SIG image configuration template.
//
//nolint:musttag // tags can be added if deemed necessary
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
	AKSUbuntuEdgeZoneGalleryName   string = "AKSUbuntuEdgeZone"
	AKSUbuntuEdgeZoneResourceGroup string = "AKS-Ubuntu-EdgeZone"
)

const (
	// DO NOT MODIFY: used for freezing linux images with docker.
	FrozenLinuxSIGImageVersionForDocker string = "2022.08.29"

	// We do not use AKS Windows image versions in AgentBaker. These fake values are only used for unit tests.
	Windows2019SIGImageVersion string = "17763.2019.221114"
	Windows2022SIGImageVersion string = "20348.2022.221114"
)

type sigVersion struct {
	OSType  string `json:"ostype"`
	Version string `json:"version"`
}

//go:embed linux_sig_version.json
var linuxVersionJSONContentsEmbedded string

//go:embed edge_zone_sig_version.json
var edgeZoneJSONContentsEmbedded string

//go:embed mariner_v2_kata_gen2_tl_sig_version.json
var marinerV2KataGen2TLJSONContentsEmbedded string

//nolint:gochecknoglobals
var LinuxSIGImageVersion = getSIGVersionFromEmbeddedString(linuxVersionJSONContentsEmbedded)

//nolint:gochecknoglobals
var EdgeZoneSIGImageVersion = getSIGVersionFromEmbeddedString(edgeZoneJSONContentsEmbedded)

//nolint:gochecknoglobals
var CBLMarinerV2KataGen2TLSIGImageVersion = getSIGVersionFromEmbeddedString(marinerV2KataGen2TLJSONContentsEmbedded)

func getSIGVersionFromEmbeddedString(contents string) string {
	if len(contents) == 0 {
		panic("SIG version is empty")
	}

	var sigImageStruct sigVersion
	err := json.Unmarshal([]byte(contents), &sigImageStruct)

	if err != nil {
		panic(err)
	}

	sigImageVersion := sigImageStruct.Version
	return sigImageVersion
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
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuContainerd1804Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gen2containerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuGPUContainerd1804ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gpucontainerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuGPUContainerd1804Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gen2gpucontainerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuFipsContainerd1804ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804fipscontainerd",
		Version:       LinuxSIGImageVersion,
	}

	// not a typo, this image was generated on 2021.05.20 UTC and assigned this version.
	SIGUbuntuFipsContainerd1804Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gen2fipscontainerd",
		Version:       LinuxSIGImageVersion,
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

	SIGUbuntuArm64Containerd2204Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2204gen2arm64containerd",
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
		Version:       "202306.30.0",
	}

	SIGUbuntuMinimalContainerd2204Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2204gen2minimalcontainerd",
		Version:       "202306.30.0",
	}

	SIGCBLMarinerV1ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V1",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2Gen1ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2gen2",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2Gen1FIPSImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2fips",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2Gen2FIPSImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2gen2fips",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2KataImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2katagen2",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2Arm64ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2gen2arm64",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2TLImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2gen2TL",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2KataGen2TLImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2katagen2TL",
		Version:       CBLMarinerV2KataGen2TLSIGImageVersion,
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
)

func getSigUbuntuImageConfigMapWithOpts(opts ...SigImageConfigOpt) map[Distro]SigImageConfig {
	return map[Distro]SigImageConfig{
		AKSUbuntu1604:                      SIGUbuntu1604ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntu1804:                      SIGUbuntu1804ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntu1804Gen2:                  SIGUbuntu1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuGPU1804:                   SIGUbuntuGPU1804ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuGPU1804Gen2:               SIGUbuntuGPU1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd1804:            SIGUbuntuContainerd1804ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd1804Gen2:        SIGUbuntuContainerd1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuGPUContainerd1804:         SIGUbuntuGPUContainerd1804ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuGPUContainerd1804Gen2:     SIGUbuntuGPUContainerd1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuFipsContainerd1804:        SIGUbuntuFipsContainerd1804ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuFipsContainerd1804Gen2:    SIGUbuntuFipsContainerd1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuFipsContainerd2004:        SIGUbuntuFipsContainerd2004ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuFipsContainerd2004Gen2:    SIGUbuntuFipsContainerd2004Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2204:            SIGUbuntuContainerd2204ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2204Gen2:        SIGUbuntuContainerd2204Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2004CVMGen2:     SIGUbuntuContainerd2004CVMGen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuArm64Containerd2204Gen2:   SIGUbuntuArm64Containerd2204Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2204TLGen2:      SIGUbuntuContainerd2204TLGen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuMinimalContainerd2204:     SIGUbuntuMinimalContainerd2204ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuMinimalContainerd2204Gen2: SIGUbuntuMinimalContainerd2204Gen2ImageConfigTemplate.WithOptions(opts...),
	}
}
func getSigCBLMarinerImageConfigMapWithOpts(opts ...SigImageConfigOpt) map[Distro]SigImageConfig {
	return map[Distro]SigImageConfig{
		AKSCBLMarinerV1:           SIGCBLMarinerV1ImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2:           SIGCBLMarinerV2Gen1ImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Gen2:       SIGCBLMarinerV2ImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2FIPS:       SIGCBLMarinerV2Gen1FIPSImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Gen2FIPS:   SIGCBLMarinerV2Gen2FIPSImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Gen2Kata:   SIGCBLMarinerV2KataImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Arm64Gen2:  SIGCBLMarinerV2Arm64ImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Gen2TL:     SIGCBLMarinerV2TLImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2KataGen2TL: SIGCBLMarinerV2KataGen2TLImageConfigTemplate.WithOptions(opts...),
	}
}

func getSigWindowsImageConfigMapWithOpts(opts ...SigImageConfigOpt) map[Distro]SigImageConfig {
	return map[Distro]SigImageConfig{
		AKSWindows2019:               SIGWindows2019ImageConfigTemplate.WithOptions(opts...),
		AKSWindows2019Containerd:     SIGWindows2019ContainerdImageConfigTemplate.WithOptions(opts...),
		AKSWindows2022Containerd:     SIGWindows2022ContainerdImageConfigTemplate.WithOptions(opts...),
		AKSWindows2022ContainerdGen2: SIGWindows2022ContainerdGen2ImageConfigTemplate.WithOptions(opts...),
	}
}

func getSigUbuntuEdgeZoneImageConfigMapWithOpts(opts ...SigImageConfigOpt) map[Distro]SigImageConfig {
	// This image is using a specific resource group and gallery name for edge zone scenario.
	sigUbuntuEdgeZoneContainerd1804ImageConfigTemplate := SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuEdgeZoneResourceGroup,
		Gallery:       AKSUbuntuEdgeZoneGalleryName,
		Definition:    "1804containerd",
		Version:       EdgeZoneSIGImageVersion,
	}

	// This image is using a specific resource group and gallery name for edge zone scenario.
	sigUbuntuEdgeZoneContainerd1804Gen2ImageConfigTemplate := SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuEdgeZoneResourceGroup,
		Gallery:       AKSUbuntuEdgeZoneGalleryName,
		Definition:    "1804gen2containerd",
		Version:       EdgeZoneSIGImageVersion,
	}

	// This image is using a specific resource group and gallery name for edge zone scenario.
	sigUbuntuEdgeZoneContainerd2204ImageConfigTemplate := SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuEdgeZoneResourceGroup,
		Gallery:       AKSUbuntuEdgeZoneGalleryName,
		Definition:    "2204containerd",
		Version:       EdgeZoneSIGImageVersion,
	}

	// This image is using a specific resource group and gallery name for edge zone scenario.
	sigUbuntuEdgeZoneContainerd2204Gen2ImageConfigTemplate := SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuEdgeZoneResourceGroup,
		Gallery:       AKSUbuntuEdgeZoneGalleryName,
		Definition:    "2204gen2containerd",
		Version:       EdgeZoneSIGImageVersion,
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
		SigWindowsImageConfig:        getSigWindowsImageConfigMapWithOpts(withSubscription(AzurePublicCloudSigSubscription)),
		SigUbuntuEdgeZoneImageConfig: getSigUbuntuEdgeZoneImageConfigMapWithOpts(withSubscription(AzurePublicCloudSigSubscription)),
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
