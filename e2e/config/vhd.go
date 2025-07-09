package config

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"

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
)

var (
	VHDUbuntu1804Gen2Containerd = &Image{
		Name:    "1804gen2containerd",
		OS:      OSUbuntu,
		Arch:    "amd64",
		Distro:  datamodel.AKSUbuntuContainerd1804Gen2,
		Gallery: imageGalleryLinux,
	}
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
	VHDAzureLinuxV2Gen2Arm64 = &Image{
		Name:    "AzureLinuxV2gen2arm64",
		OS:      OSAzureLinux,
		Arch:    "arm64",
		Distro:  datamodel.AKSAzureLinuxV2Arm64Gen2,
		Gallery: imageGalleryLinux,
	}
	VHDAzureLinuxV2Gen2 = &Image{
		Name:    "AzureLinuxV2gen2",
		OS:      OSAzureLinux,
		Arch:    "amd64",
		Distro:  datamodel.AKSAzureLinuxV2Gen2,
		Gallery: imageGalleryLinux,
	}
	VHDAzureLinuxV3Gen2 = &Image{
		Name:    "AzureLinuxV3gen2",
		OS:      OSAzureLinux,
		Arch:    "amd64",
		Distro:  datamodel.AKSAzureLinuxV3Gen2,
		Gallery: imageGalleryLinux,
	}
	VHDCBLMarinerV2Gen2Arm64 = &Image{
		Name:    "CBLMarinerV2gen2arm64",
		OS:      OSMariner,
		Arch:    "arm64",
		Distro:  datamodel.AKSCBLMarinerV2Arm64Gen2,
		Gallery: imageGalleryLinux,
	}
	VHDCBLMarinerV2Gen2 = &Image{
		Name:    "CBLMarinerV2gen2",
		OS:      OSMariner,
		Arch:    "amd64",
		Distro:  datamodel.AKSCBLMarinerV2Gen2,
		Gallery: imageGalleryLinux,
	}
	// this is a particular 2204gen2containerd image originally built with private packages,
	// if we ever want to update this then we'd need to run a new VHD build using private package overrides
	VHDUbuntu2204Gen2ContainerdPrivateKubePkg = &Image{
		// 2204Gen2 is a special image definition holding historical VHDs used by agentbaker e2e's.
		Name:    "2204Gen2",
		OS:      OSUbuntu,
		Arch:    "amd64",
		Version: "1.1704411049.2812",
		Distro:  datamodel.AKSUbuntuContainerd2204Gen2,
		Gallery: imageGalleryLinux,
	}

	// without kubelet, kubectl, credential-provider and wasm
	VHDUbuntu2204Gen2ContainerdAirgappedK8sNotCached = &Image{
		Name:    "2204Gen2",
		OS:      OSUbuntu,
		Arch:    "amd64",
		Version: "1.1725612526.29638",
		Distro:  datamodel.AKSUbuntuContainerd2204Gen2,
		Gallery: imageGalleryLinux,
	}

	VHDUbuntu2404Gen1Containerd = &Image{
		Name:    "2404containerd",
		OS:      OSUbuntu,
		Arch:    "amd64",
		Distro:  datamodel.AKSUbuntuContainerd2404,
		Gallery: imageGalleryLinux,
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

	VHDWindows2019Containerd = &Image{
		Name:    "windows-2019-containerd",
		OS:      "windows",
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows2019Containerd,
		Gallery: imageGalleryWindows,
	}

	VHDWindows2022Containerd = &Image{
		Name:    "windows-2022-containerd",
		OS:      "windows",
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows2022Containerd,
		Gallery: imageGalleryWindows,
	}

	VHDWindows2022ContainerdGen2 = &Image{
		Name:    "windows-2022-containerd-gen2",
		OS:      OSWindows,
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows2022ContainerdGen2,
		Gallery: imageGalleryWindows,
	}

	VHDWindows23H2 = &Image{
		Name:    "windows-23H2",
		OS:      OSWindows,
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows23H2,
		Gallery: imageGalleryWindows,
	}

	VHDWindows23H2Gen2 = &Image{
		Name:    "windows-23H2-gen2",
		OS:      OSWindows,
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows23H2Gen2,
		Gallery: imageGalleryWindows,
	}

	VHDWindows2025 = &Image{
		Name:    "windows-2025",
		OS:      OSWindows,
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows2025,
		Gallery: imageGalleryWindows,
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

type Image struct {
	Arch    string
	Distro  datamodel.Distro
	Name    string
	OS      OS
	Version string
	Gallery *Gallery

	vhd     VHDResourceID
	vhdOnce sync.Once
	vhdErr  error
}

func (i *Image) String() string {
	// a starter for a string for debugging.
	return fmt.Sprintf("%s %s %s %s", i.OS, i.Name, i.Version, i.Arch)
}

func (i *Image) VHDResourceID(ctx context.Context, t *testing.T, location string) (VHDResourceID, error) {
	i.vhdOnce.Do(func() {
		switch {
		case i.Version != "":
			i.vhd, i.vhdErr = Azure.EnsureSIGImageVersion(ctx, t, i, location)
			if i.vhd != "" {
				t.Logf("Got image by version: %s", i.azurePortalImageVersionUrl())
			}
		default:
			i.vhd, i.vhdErr = Azure.LatestSIGImageVersionByTag(ctx, t, i, Config.SIGVersionTagName, Config.SIGVersionTagValue, location)
			if i.vhd != "" {
				t.Logf("got version by tag %s=%s: %s", Config.SIGVersionTagName, Config.SIGVersionTagValue, i.azurePortalImageVersionUrl())
			} else {
				t.Logf("Could not find version by tag %s=%s: %s", Config.SIGVersionTagName, Config.SIGVersionTagValue, i.azurePortalImageUrl())
			}
		}
		if i.vhdErr != nil {
			i.vhdErr = fmt.Errorf("img: %s, tag %s=%s, err %w", i.azurePortalImageUrl(), Config.SIGVersionTagName, Config.SIGVersionTagValue, i.vhdErr)
		}
	})
	return i.vhd, i.vhdErr
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
		VHDAzureLinuxV2Gen2,
		VHDAzureLinuxV3Gen2,
		VHDCBLMarinerV2Gen2,
	}

	// Return a random VHD from the list
	return vhds[rand.Intn(len(vhds))]
}
