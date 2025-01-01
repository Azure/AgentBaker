package config

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	noSelectionTagName = "abe2e-ignore"
)

var (
	linuxGallery = &Gallery{
		SubscriptionID:    Config.GallerySubscriptionIDLinux,
		ResourceGroupName: Config.GalleryResourceGroupNameLinux,
		Name:              Config.GalleryNameLinux,
	}

	windowsGallery = &Gallery{
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
		Gallery: linuxGallery,
	}
	VHDUbuntu2204Gen2Arm64Containerd = &Image{
		Name:    "2204gen2arm64containerd",
		OS:      OSUbuntu,
		Arch:    "arm64",
		Distro:  datamodel.AKSUbuntuArm64Containerd2204Gen2,
		Gallery: linuxGallery,
	}
	VHDUbuntu2204Gen2Containerd = &Image{
		Name:    "2204gen2containerd",
		OS:      OSUbuntu,
		Arch:    "amd64",
		Distro:  datamodel.AKSUbuntuContainerd2204Gen2,
		Gallery: linuxGallery,
	}
	VHDAzureLinuxV2Gen2Arm64 = &Image{
		Name:    "AzureLinuxV2gen2arm64",
		OS:      OSAzureLinux,
		Arch:    "arm64",
		Distro:  datamodel.AKSAzureLinuxV2Arm64Gen2,
		Gallery: linuxGallery,
	}
	VHDAzureLinuxV2Gen2 = &Image{
		Name:    "AzureLinuxV2gen2",
		OS:      OSAzureLinux,
		Arch:    "amd64",
		Distro:  datamodel.AKSAzureLinuxV2Gen2,
		Gallery: linuxGallery,
	}
	VHDCBLMarinerV2Gen2Arm64 = &Image{
		Name:    "CBLMarinerV2gen2arm64",
		OS:      OSMariner,
		Arch:    "arm64",
		Distro:  datamodel.AKSCBLMarinerV2Arm64Gen2,
		Gallery: linuxGallery,
	}
	VHDCBLMarinerV2Gen2 = &Image{
		Name:    "CBLMarinerV2gen2",
		OS:      OSMariner,
		Arch:    "amd64",
		Distro:  datamodel.AKSCBLMarinerV2Gen2,
		Gallery: linuxGallery,
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
		Gallery: linuxGallery,
	}

	// without kubelet, kubectl, credential-provider and wasm
	VHDUbuntu2204Gen2ContainerdAirgappedK8sNotCached = &Image{
		Name:    "2204Gen2",
		OS:      OSUbuntu,
		Arch:    "amd64",
		Version: "1.1725612526.29638",
		Distro:  datamodel.AKSUbuntuContainerd2204Gen2,
		Gallery: linuxGallery,
	}

	VHDUbuntu2404Gen1Containerd = &Image{
		Name:    "2404containerd",
		OS:      OSUbuntu,
		Arch:    "amd64",
		Distro:  datamodel.AKSUbuntuContainerd2404,
		Gallery: linuxGallery,
	}

	VHDUbuntu2404Gen2Containerd = &Image{
		Name:    "2404gen2containerd",
		OS:      OSUbuntu,
		Arch:    "amd64",
		Distro:  datamodel.AKSUbuntuContainerd2404Gen2,
		Gallery: linuxGallery,
	}

	VHDUbuntu2404ArmContainerd = &Image{
		Name:    "2404gen2arm64containerd",
		OS:      OSUbuntu,
		Arch:    "arm64",
		Distro:  datamodel.AKSUbuntuArm64Containerd2404Gen2,
		Gallery: linuxGallery,
	}

	VHDWindows2019Containerd = &Image{
		Name:    "windows-2019-containerd",
		OS:      "windows",
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows2019Containerd,
		Latest:  true,
		Gallery: windowsGallery,
	}

	VHDWindows2022Containerd = &Image{
		Name:    "windows-2022-containerd",
		OS:      "windows",
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows2022Containerd,
		Latest:  true,
		Gallery: windowsGallery,
	}

	VHDWindows2022ContainerdGen2 = &Image{
		Name:    "windows-2022-containerd-gen2",
		OS:      OSWindows,
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows2022ContainerdGen2,
		Latest:  true,
		Gallery: windowsGallery,
	}

	VHDWindows23H2 = &Image{
		Name:    "windows-23H2",
		OS:      OSWindows,
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows23H2,
		Latest:  true,
		Gallery: windowsGallery,
	}

	VHDWindows23H2Gen2 = &Image{
		Name:    "windows-23H2-gen2",
		OS:      OSWindows,
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows23H2Gen2,
		Latest:  true,
		Gallery: windowsGallery,
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
	Latest  bool // a hack to get the latest version of the image for windows, currently windows images are not tagged

	vhd     VHDResourceID
	vhdOnce sync.Once
	vhdErr  error
}

func (i *Image) String() string {
	// a starter for a string for debugging.
	return fmt.Sprintf("%s %s %s %s", i.OS, i.Name, i.Version, i.Arch)
}

func (i *Image) VHDResourceID(ctx context.Context, t *testing.T) (VHDResourceID, error) {
	i.vhdOnce.Do(func() {
		switch {
		case i.Latest:
			i.vhd, i.vhdErr = Azure.LatestSIGImageVersionByTag(ctx, i, "", "")
		case i.Version != "":
			i.vhd, i.vhdErr = Azure.EnsureSIGImageVersion(ctx, i)
		default:
			i.vhd, i.vhdErr = Azure.LatestSIGImageVersionByTag(ctx, i, Config.SIGVersionTagName, Config.SIGVersionTagValue)
		}
		if i.vhdErr != nil {
			i.vhdErr = fmt.Errorf("img: %s, tag %s=%s, err %w", i.Name, Config.SIGVersionTagName, Config.SIGVersionTagValue, i.vhdErr)
			t.Logf("failed to find the latest image version for %s", i.vhdErr)
		}
	})
	return i.vhd, i.vhdErr
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
