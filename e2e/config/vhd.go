package config

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	noSelectionTagName = "abe2e-ignore"
)

var (
	imageGalleryLinux = &Gallery{
		SubscriptionID:    Config.GallerySubscriptionIDLinux,
		ResourceGroupName: Config.GalleryResourceGroupNameLinux,
		Name:              Config.GalleryNameLinux,
	}
	imageGalleryWindows = &Gallery{
		SubscriptionID:    Config.GallerySubscriptionIDWindows,
		ResourceGroupName: Config.GalleryResourceGroupNameWindows,
		Name:              Config.GalleryNameWindows,
	}
)

type Gallery struct {
	SubscriptionID    string
	ResourceGroupName string
	Name              string
}

type OS string

var (
	OSWindows    OS = "windows"
	OSUbuntu     OS = "ubuntu"
	OSMariner    OS = "mariner"
	OSAzureLinux OS = "azurelinux"
	OSFlatcar    OS = "flatcar"
	OSACL        OS = "azurecontainerlinux"
)

var (
	VHDUbuntu2204Gen2Arm64Containerd = &Image{
		Name:    "2204gen2arm64containerd",
		OS:      OSUbuntu,
		Arch:    "arm64",
		Distro:  datamodel.AKSUbuntuArm64Containerd2204Gen2,
		Gallery: imageGalleryLinux,
	}

	VHDUbuntu2204Gen2Containerd = &Image{
		Name:    "2204gen2containerd",
		OS:      OSUbuntu,
		Arch:    "amd64",
		Distro:  datamodel.AKSUbuntuContainerd2204Gen2,
		Gallery: imageGalleryLinux,
	}

	VHDUbuntu2204Gen2TLContainerd = &Image{
		Name:    "2204gen2TLcontainerd",
		OS:      OSUbuntu,
		Arch:    "amd64",
		Distro:  datamodel.AKSUbuntuContainerd2204TLGen2,
		Gallery: imageGalleryLinux,
	}

	VHDUbuntu2004FIPSContainerd = &Image{
		Name:                  "2004fipscontainerd",
		OS:                    OSUbuntu,
		Arch:                  "amd64",
		Distro:                datamodel.AKSUbuntuFipsContainerd2004,
		Gallery:               imageGalleryLinux,
		UnsupportedLocalDns:   true,
		UnsupportedGen2:       true,
		SkipOldVHDValidations: true,
	}

	VHDUbuntu2004FIPSGen2Containerd = &Image{
		Name:                  "2004fipsgen2containerd",
		OS:                    OSUbuntu,
		Arch:                  "amd64",
		Distro:                datamodel.AKSUbuntuFipsContainerd2004Gen2,
		Gallery:               imageGalleryLinux,
		UnsupportedLocalDns:   true,
		UnsupportedGen2:       true,
		SkipOldVHDValidations: true,
	}

	VHDUbuntu2204FIPSContainerd = &Image{
		Name:                "2204fipscontainerd",
		OS:                  OSUbuntu,
		Arch:                "amd64",
		Distro:              datamodel.AKSUbuntuFipsContainerd2204,
		Gallery:             imageGalleryLinux,
		UnsupportedLocalDns: true,
		UnsupportedGen2:     true,
	}

	VHDUbuntu2204Gen2FIPSContainerd = &Image{
		Name:                "2204gen2fipscontainerd",
		OS:                  OSUbuntu,
		Arch:                "amd64",
		Distro:              datamodel.AKSUbuntuFipsContainerd2204Gen2,
		Gallery:             imageGalleryLinux,
		UnsupportedLocalDns: true,
	}

	VHDUbuntu2204Gen2FIPSTLContainerd = &Image{
		Name:                "2204gen2fipsTLcontainerd",
		OS:                  OSUbuntu,
		Arch:                "amd64",
		Distro:              datamodel.AKSUbuntuFipsContainerd2204TLGen2,
		Gallery:             imageGalleryLinux,
		UnsupportedLocalDns: true,
	}

	VHDAzureLinuxV2Gen2 = &Image{
		Name:                  "V2gen2",
		OS:                    OSAzureLinux,
		Arch:                  "amd64",
		Distro:                datamodel.AKSAzureLinuxV2Gen2,
		Version:               datamodel.FrozenCBLMarinerV2AndAzureLinuxV2SIGImageVersion,
		Gallery:               imageGalleryLinux,
		SkipOldVHDValidations: true,
	}

	VHDAzureLinuxV3Gen2 = &Image{
		Name:    "AzureLinuxV3gen2",
		OS:      OSAzureLinux,
		Arch:    "amd64",
		Distro:  datamodel.AKSAzureLinuxV3Gen2,
		Gallery: imageGalleryLinux,
	}

	VHDAzureLinux3OSGuard = &Image{
		Name:                "AzureLinuxOSGuardOSGuardV3gen2fipsTL",
		OS:                  OSAzureLinux,
		Arch:                "amd64",
		Distro:              datamodel.AKSAzureLinuxV3OSGuardGen2FIPSTL,
		Gallery:             imageGalleryLinux,
		UnsupportedLocalDns: true,
	}

	VHDAzureLinuxV3Gen2FIPS = &Image{
		Name:                "AzureLinuxV3gen2fips",
		OS:                  OSAzureLinux,
		Arch:                "amd64",
		Distro:              datamodel.AKSAzureLinuxV3Gen2FIPS,
		Gallery:             imageGalleryLinux,
		UnsupportedLocalDns: true,
	}

	VHDUbuntu2404Gen1Containerd = &Image{
		Name:            "2404containerd",
		OS:              OSUbuntu,
		Arch:            "amd64",
		Distro:          datamodel.AKSUbuntuContainerd2404,
		Gallery:         imageGalleryLinux,
		UnsupportedGen2: true,
	}

	VHDUbuntu2404Gen2Containerd = &Image{
		Name:    "2404gen2containerd",
		OS:      OSUbuntu,
		Arch:    "amd64",
		Distro:  datamodel.AKSUbuntuContainerd2404Gen2,
		Gallery: imageGalleryLinux,
	}

	VHDUbuntu2404ArmContainerd = &Image{
		Name:    "2404gen2arm64containerd",
		OS:      OSUbuntu,
		Arch:    "arm64",
		Distro:  datamodel.AKSUbuntuArm64Containerd2404Gen2,
		Gallery: imageGalleryLinux,
	}

	VHDFlatcarGen2 = &Image{
		Name:         "flatcargen2",
		OS:           OSFlatcar,
		Arch:         "amd64",
		Distro:       datamodel.AKSFlatcarGen2,
		Gallery:      imageGalleryLinux,
		Flatcar:      true,
		OSDiskSizeGB: 60,
	}

	VHDFlatcarGen2Arm64 = &Image{
		Name:         "flatcargen2arm64",
		OS:           OSFlatcar,
		Arch:         "arm64",
		Distro:       datamodel.AKSFlatcarArm64Gen2,
		Gallery:      imageGalleryLinux,
		Flatcar:      true,
		OSDiskSizeGB: 60,
	}

	VHDAzureLinuxV3Gen2Arm64 = &Image{
		Name:         "azurelinuxv3gen2arm64",
		OS:           OSAzureLinux,
		Arch:         "arm64",
		Distro:       datamodel.AKSAzureLinuxV3Arm64Gen2,
		Gallery:      imageGalleryLinux,
		OSDiskSizeGB: 60,
	}

	VHDACLGen2TL = &Image{
		Name:         "aclgen2TL",
		OS:           OSACL,
		Arch:         "amd64",
		Distro:       datamodel.AKSACLGen2TL,
		Gallery:      imageGalleryLinux,
		Flatcar:      true,
		OSDiskSizeGB: 60,
	}

	VHDACLArm64Gen2TL = &Image{
		Name:         "aclgen2arm64TL",
		OS:           OSACL,
		Arch:         "arm64",
		Distro:       datamodel.AKSACLArm64Gen2TL,
		Gallery:      imageGalleryLinux,
		Flatcar:      true,
		OSDiskSizeGB: 60,
	}

	VHDACLGen2FIPSTL = &Image{
		Name:                "aclgen2fipsTL",
		OS:                  OSACL,
		Arch:                "amd64",
		Distro:              datamodel.AKSACLGen2FIPSTL,
		Gallery:             imageGalleryLinux,
		Flatcar:             true,
		OSDiskSizeGB:        60,
		UnsupportedLocalDns: true,
	}

	VHDACLArm64Gen2FIPSTL = &Image{
		Name:                "aclgen2arm64fipsTL",
		OS:                  OSACL,
		Arch:                "arm64",
		Distro:              datamodel.AKSACLArm64Gen2FIPSTL,
		Gallery:             imageGalleryLinux,
		Flatcar:             true,
		OSDiskSizeGB:        60,
		UnsupportedLocalDns: true,
	}

	VHDWindows2022Containerd = &Image{
		Name:            "windows-2022-containerd",
		OS:              "windows",
		Arch:            "amd64",
		Distro:          datamodel.AKSWindows2022Containerd,
		Gallery:         imageGalleryWindows,
		UnsupportedGen2: true,
	}

	VHDWindows2022ContainerdGen2 = &Image{
		Name:    "windows-2022-containerd-gen2",
		OS:      OSWindows,
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows2022ContainerdGen2,
		Gallery: imageGalleryWindows,
	}

	VHDWindows2025 = &Image{
		Name:            "windows-2025",
		OS:              OSWindows,
		Arch:            "amd64",
		Distro:          datamodel.AKSWindows2025,
		Gallery:         imageGalleryWindows,
		UnsupportedGen2: true,
	}

	VHDWindows2025Gen2 = &Image{
		Name:    "windows-2025-gen2",
		OS:      OSWindows,
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows2025Gen2,
		Gallery: imageGalleryWindows,
	}
)

var ErrNotFound = fmt.Errorf("not found")

type perLocationVHDCache struct {
	vhd  VHDResourceID
	err  error
	once *sync.Once
}

type Image struct {
	Arch                                string
	Distro                              datamodel.Distro
	Name                                string
	OS                                  OS
	Version                             string
	Gallery                             *Gallery
	UnsupportedKubeletNodeIP            bool
	UnsupportedLocalDns                 bool
	UnsupportedNVMe                     bool
	UnsupportedGen2                     bool
	IgnoreFailedCgroupTelemetryServices bool
	Flatcar                             bool
	SkipOldVHDValidations               bool
	// OSDiskSizeGB overrides the default OS disk size (50 GB) when set.
	OSDiskSizeGB int32
}

func (i *Image) String() string {
	// a starter for a string for debugging.
	return fmt.Sprintf("%s %s %s %s", i.OS, i.Name, i.Version, i.Arch)
}

func (i *Image) SupportsScriptless() bool {
	return !i.Flatcar && !i.Distro.IsWindowsDistro()
}

func GetVHDResourceID(ctx context.Context, i Image, location string) (VHDResourceID, error) {
	switch {
	case i.Version != "":
		vhd, err := Azure.EnsureSIGImageVersion(ctx, &i, location)
		if err != nil {
			return "", fmt.Errorf("failed to ensure image version %s: %w", i.Version, err)
		}
		toolkit.Logf(ctx, "Got image by version: %s", i.azurePortalImageVersionUrl())
		return vhd, nil
	default:
		vhd, err := Azure.LatestSIGImageVersionByTag(ctx, &i, Config.SIGVersionTagName, Config.SIGVersionTagValue, location)
		if err != nil {
			return "", fmt.Errorf("failed to get latest image by tag %s=%s: %w", Config.SIGVersionTagName, Config.SIGVersionTagValue, err)
		}
		if vhd != "" {
			toolkit.Logf(ctx, "got version by tag %s=%s: %s", Config.SIGVersionTagName, Config.SIGVersionTagValue, i.azurePortalImageVersionUrl())
		} else {
			toolkit.Logf(ctx, "Could not find version by tag %s=%s: %s", Config.SIGVersionTagName, Config.SIGVersionTagValue, i.azurePortalImageUrl())
		}
		return vhd, nil
	}
}

func (i *Image) azurePortalImageUrl() string {
	return fmt.Sprintf("https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/galleries/%s/images/%s/overview",
		i.Gallery.SubscriptionID,
		i.Gallery.ResourceGroupName,
		i.Gallery.Name,
		i.Distro,
	)
}

func (i *Image) azurePortalImageVersionUrl() string {
	return fmt.Sprintf("https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/galleries/%s/images/%s/versions/%s/overview",
		i.Gallery.SubscriptionID,
		i.Gallery.ResourceGroupName,
		i.Gallery.Name,
		i.Distro,
		i.Version,
	)
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

func GetRandomLinuxAMD64VHD() *Image {
	// List of VHDs to use for generic tests, this could be expanded in the future to support a map of VHD and compatible VM Skus
	vhds := []*Image{
		VHDUbuntu2404Gen2Containerd,
		VHDUbuntu2204Gen2Containerd,
		VHDAzureLinuxV3Gen2,
	}

	// Return a random VHD from the list
	return vhds[rand.Intn(len(vhds))]
}
