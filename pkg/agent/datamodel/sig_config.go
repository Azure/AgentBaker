package datamodel

import (
	"fmt"
	"strings"
)

const (
	AzurePublicCloudSigTenantID     string = "33e01921-4d64-4f8c-a055-5bdaffd5e33d" // AME Tenant
	AzurePublicCloudSigSubscription string = "109a5e88-712a-48ae-9078-9ca8b3c81345" // AKS VHD
)

//SIGAzureEnvironmentSpecConfig is the overall configuration differences in different cloud environments.
// TODO(tonyxu) merge this with AzureEnvironmentSpecConfig from aks-engine(pkg/api/azenvtypes.go) once it's moved into AKS RP
type SIGAzureEnvironmentSpecConfig struct {
	CloudName                string                    `json:"cloudName,omitempty"`
	SigTenantID              string                    `json:"sigTenantID,omitempty"`
	SubscriptionID           string                    `json:"subscriptionID,omitempty"`
	SigUbuntuImageConfig     map[Distro]SigImageConfig `json:"sigUbuntuImageConfig,omitempty"`
	SigCBLMarinerImageConfig map[Distro]SigImageConfig `json:"sigCBLMarinerImageConfig,omitempty"`
	SigWindowsImageConfig    map[Distro]SigImageConfig `json:"sigWindowsImageConfig,omitempty"`
	//TODO(adadilli) add PIR constants as well
}

// SIGConfig is used to hold configuration parameters to access AKS VHDs stored in a SIG
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

//TODO(amaheshwari): these vars are not consumed by Agentbaker but by RP. do a cleanup to remove these after 20.04 work.
var AvailableUbuntu1804Distros []Distro = []Distro{
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
	AKSUbuntuFipsGPUContainerd1804,
	AKSUbuntuFipsGPUContainerd1804Gen2,
	AKSUbuntuArm64Containerd1804Gen2,
}

var AvailableUbuntu2004Distros []Distro = []Distro{
	AKSUbuntuContainerd2004CVMGen2,
}

var AvailableUbuntu2204Distros []Distro = []Distro{
	AKSUbuntuContainerd2204,
	AKSUbuntuContainerd2204Gen2,
	AKSUbuntuArm64Containerd2204Gen2,
	AKSUbuntuContainerd2204TLGen2,
}

var AvailableContainerdDistros []Distro = []Distro{
	AKSUbuntuContainerd1804,
	AKSUbuntuContainerd1804Gen2,
	AKSUbuntuGPUContainerd1804,
	AKSUbuntuGPUContainerd1804Gen2,
	AKSUbuntuFipsContainerd1804,
	AKSUbuntuFipsContainerd1804Gen2,
	AKSUbuntuFipsGPUContainerd1804,
	AKSUbuntuFipsGPUContainerd1804Gen2,
	AKSCBLMarinerV1,
	AKSCBLMarinerV2Gen2,
	AKSCBLMarinerV2Gen2Kata,
	AKSUbuntuArm64Containerd1804Gen2,
	AKSUbuntuArm64Containerd2204Gen2,
	AKSUbuntuContainerd2204,
	AKSUbuntuContainerd2204Gen2,
	AKSUbuntuContainerd2004CVMGen2,
	AKSUbuntuContainerd2204TLGen2,
}

var AvailableGPUDistros []Distro = []Distro{
	AKSUbuntuGPU1804,
	AKSUbuntuGPU1804Gen2,
	AKSUbuntuGPUContainerd1804,
	AKSUbuntuGPUContainerd1804Gen2,
	AKSUbuntuFipsGPUContainerd1804,
	AKSUbuntuFipsGPUContainerd1804Gen2,
}

var AvailableGen2Distros []Distro = []Distro{
	AKSUbuntu1804Gen2,
	AKSUbuntuGPU1804Gen2,
	AKSUbuntuContainerd1804Gen2,
	AKSUbuntuGPUContainerd1804Gen2,
	AKSUbuntuFipsContainerd1804Gen2,
	AKSUbuntuFipsGPUContainerd1804Gen2,
	AKSUbuntuArm64Containerd1804Gen2,
	AKSUbuntuArm64Containerd2204Gen2,
	AKSUbuntuContainerd2204Gen2,
	AKSUbuntuContainerd2004CVMGen2,
	AKSUbuntuContainerd2204TLGen2,
}

var AvailableCBLMarinerDistros []Distro = []Distro{
	AKSCBLMarinerV1,
	AKSCBLMarinerV2Gen2,
	AKSCBLMarinerV2Gen2Kata,
	AKSCBLMarinerV2Arm64Gen2,
}

// IsContainerdSKU returns true if distro type is containerd-enabled
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

// SigImageConfigTemplate represents the SIG image configuration template
type SigImageConfigTemplate struct {
	ResourceGroup string
	Gallery       string
	Definition    string
	Version       string
}

// SigImageConfig represents the SIG image configuration
type SigImageConfig struct {
	SigImageConfigTemplate
	SubscriptionID string
}

// WithOptions converts a SigImageConfigTemplate to SigImageConfig instance via function opts
func (template SigImageConfigTemplate) WithOptions(options ...SigImageConfigOpt) SigImageConfig {
	config := &SigImageConfig{
		SigImageConfigTemplate: template,
	}
	for _, opt := range options {
		opt(config)
	}
	return *config
}

var AvailableWindowsSIGDistros []Distro = []Distro{
	AKSWindows2019,
	AKSWindows2019Containerd,
	AKSWindows2022Containerd,
	AKSWindows2022ContainerdGen2,
	CustomizedWindowsOSImage,
}

var AvailableWindowsPIRDistros []Distro = []Distro{
	AKSWindows2019PIR,
}

// SIG const
const (
	AKSSIGImagePublisher       string = "microsoft-aks"
	AKSWindowsGalleryName      string = "AKSWindows"
	AKSWindowsResourceGroup    string = "AKS-Windows"
	AKSUbuntuGalleryName       string = "AKSUbuntu"
	AKSUbuntuResourceGroup     string = "AKS-Ubuntu"
	AKSCBLMarinerGalleryName   string = "AKSCBLMariner"
	AKSCBLMarinerResourceGroup string = "AKS-CBLMariner"
)

const (
	LinuxSIGImageVersion string = "2022.10.17"

	// DO NOT MODIFY: used for freezing linux images with docker
	FrozenLinuxSIGImageVersionForDocker string = "2022.08.29"

	Windows2019SIGImageVersion string = "17763.3534.221019"
	Windows2022SIGImageVersion string = "20348.1131.221019"

	Ubuntu2204TLSIGImageVersion string = "2022.10.13"
)

// SIG config Template
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

	// not a typo, this image was generated on 2021.05.20 UTC and assigned this version
	SIGUbuntuFipsContainerd1804Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gen2fipscontainerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuFipsGPUContainerd1804ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804fipsgpucontainerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuFipsGPUContainerd1804Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gen2fipsgpucontainerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGUbuntuArm64Containerd1804Gen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "1804gen2arm64containerd",
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
		Version:       Ubuntu2204TLSIGImageVersion,
	}

	SIGUbuntuContainerd2004CVMGen2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSUbuntuResourceGroup,
		Gallery:       AKSUbuntuGalleryName,
		Definition:    "2004gen2CVMcontainerd",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV1ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V1",
		Version:       LinuxSIGImageVersion,
	}

	SIGCBLMarinerV2ImageConfigTemplate = SigImageConfigTemplate{
		ResourceGroup: AKSCBLMarinerResourceGroup,
		Gallery:       AKSCBLMarinerGalleryName,
		Definition:    "V2gen2",
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
		AKSUbuntuFipsGPUContainerd1804:     SIGUbuntuFipsGPUContainerd1804ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuFipsGPUContainerd1804Gen2: SIGUbuntuFipsGPUContainerd1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuArm64Containerd1804Gen2:   SIGUbuntuArm64Containerd1804Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2204:            SIGUbuntuContainerd2204ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2204Gen2:        SIGUbuntuContainerd2204Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2004CVMGen2:     SIGUbuntuContainerd2004CVMGen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuArm64Containerd2204Gen2:   SIGUbuntuArm64Containerd2204Gen2ImageConfigTemplate.WithOptions(opts...),
		AKSUbuntuContainerd2204TLGen2:      SIGUbuntuContainerd2204TLGen2ImageConfigTemplate.WithOptions(opts...),
	}
}
func getSigCBLMarinerImageConfigMapWithOpts(opts ...SigImageConfigOpt) map[Distro]SigImageConfig {
	return map[Distro]SigImageConfig{
		AKSCBLMarinerV1:          SIGCBLMarinerV1ImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Gen2:      SIGCBLMarinerV2ImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Gen2Kata:  SIGCBLMarinerV2KataImageConfigTemplate.WithOptions(opts...),
		AKSCBLMarinerV2Arm64Gen2: SIGCBLMarinerV2Arm64ImageConfigTemplate.WithOptions(opts...),
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

// GetSIGAzureCloudSpecConfig get cloud specific sig config
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
		return SIGAzureEnvironmentSpecConfig{}, fmt.Errorf("unexpected error while constructing env-aware sig configuration for AKSUbuntu: %s", err)
	}
	c.SigUbuntuImageConfig = getSigUbuntuImageConfigMapWithOpts(fromACSUbuntu)

	fromACSCBLMariner, err := withACSSIGConfig(sigConfig, "AKSCBLMariner")
	if err != nil {
		return SIGAzureEnvironmentSpecConfig{}, fmt.Errorf("unexpected error while constructing env-aware sig configuration for AKSCBLMariner: %s", err)
	}
	c.SigCBLMarinerImageConfig = getSigCBLMarinerImageConfigMapWithOpts(fromACSCBLMariner)

	fromACSWindows, err := withACSSIGConfig(sigConfig, "AKSWindows")
	if err != nil {
		return SIGAzureEnvironmentSpecConfig{}, fmt.Errorf("unexpected error while constructing env-aware sig configuration for Windows: %s", err)
	}
	c.SigWindowsImageConfig = getSigWindowsImageConfigMapWithOpts(fromACSWindows)
	return *c, nil
}

// GetAzurePublicSIGConfigForTest returns a statically defined sigconfig. This should only be used for unit tests and e2es.
func GetAzurePublicSIGConfigForTest() SIGAzureEnvironmentSpecConfig {
	return SIGAzureEnvironmentSpecConfig{
		CloudName:                AzurePublicCloud,
		SigTenantID:              AzurePublicCloudSigTenantID,
		SubscriptionID:           AzurePublicCloudSigSubscription,
		SigUbuntuImageConfig:     getSigUbuntuImageConfigMapWithOpts(withSubscription(AzurePublicCloudSigSubscription)),
		SigCBLMarinerImageConfig: getSigCBLMarinerImageConfigMapWithOpts(withSubscription(AzurePublicCloudSigSubscription)),
		SigWindowsImageConfig:    getSigWindowsImageConfigMapWithOpts(withSubscription(AzurePublicCloudSigSubscription)),
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

func withSubscription(subscriptionID string) SigImageConfigOpt {
	return func(c *SigImageConfig) {
		c.SubscriptionID = subscriptionID
	}
}
