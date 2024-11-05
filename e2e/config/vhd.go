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
		SubscriptionID: "8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8",
		ResourceGroup:  "aksvhdtestbuildrg",
		Name:           "PackerSigGalleryEastUS",
	}

	windowsGallery = &Gallery{
		SubscriptionID: "4be8920b-2978-43d7-ab14-04d8549c1d05",
		ResourceGroup:  "AKS-Windows",
		Name:           "AKSWindows",
	}
)

type Gallery struct {
	SubscriptionID string
	ResourceGroup  string
	Name           string
}

var (
	VHDUbuntu1804Gen2Containerd = &Image{
		Name:    "1804gen2containerd",
		OS:      "ubuntu",
		Arch:    "amd64",
		Distro:  datamodel.AKSUbuntuContainerd1804Gen2,
		Gallery: linuxGallery,
	}
	VHDUbuntu2204Gen2Arm64Containerd = &Image{
		Name:    "2204gen2arm64containerd",
		OS:      "ubuntu",
		Arch:    "arm64",
		Distro:  datamodel.AKSUbuntuArm64Containerd2204Gen2,
		Gallery: linuxGallery,
	}
	VHDUbuntu2204Gen2Containerd = &Image{
		Name:    "2204gen2containerd",
		OS:      "ubuntu",
		Arch:    "amd64",
		Distro:  datamodel.AKSUbuntuContainerd2404Gen2,
		Gallery: linuxGallery,
	}
	VHDAzureLinuxV2Gen2Arm64 = &Image{
		Name:    "AzureLinuxV2gen2arm64",
		OS:      "azurelinux",
		Arch:    "arm64",
		Distro:  datamodel.AKSAzureLinuxV2Arm64Gen2,
		Gallery: linuxGallery,
	}
	VHDAzureLinuxV2Gen2 = &Image{
		Name:    "AzureLinuxV2gen2",
		OS:      "azurelinux",
		Arch:    "amd64",
		Distro:  datamodel.AKSAzureLinuxV2Gen2,
		Gallery: linuxGallery,
	}
	VHDCBLMarinerV2Gen2Arm64 = &Image{
		Name:    "CBLMarinerV2gen2arm64",
		OS:      "mariner",
		Arch:    "arm64",
		Distro:  datamodel.AKSCBLMarinerV2Arm64Gen2,
		Gallery: linuxGallery,
	}
	VHDCBLMarinerV2Gen2 = &Image{
		Name:    "CBLMarinerV2gen2",
		OS:      "mariner",
		Arch:    "amd64",
		Distro:  datamodel.AKSCBLMarinerV2Gen2,
		Gallery: linuxGallery,
	}
	// this is a particular 2204gen2containerd image originally built with private packages,
	// if we ever want to update this then we'd need to run a new VHD build using private package overrides
	VHDUbuntu2204Gen2ContainerdPrivateKubePkg = &Image{
		Name:    "2204Gen2",
		OS:      "ubuntu",
		Arch:    "amd64",
		Version: "1.1704411049.2812",
		Distro:  datamodel.AKSUbuntuContainerd2404Gen2,
		Gallery: linuxGallery,
	}

	// without kubelet, kubectl, credential-provider and wasm
	VHDUbuntu2204Gen2ContainerdAirgapped = &Image{
		Name:    "2204gen2containerd",
		OS:      "ubuntu",
		Arch:    "amd64",
		Version: "1.1725612526.29638",
		Distro:  datamodel.AKSUbuntuContainerd2404Gen2,
		Gallery: linuxGallery,
	}

	VHDWindowsServer2019Containerd = &Image{
		Name:    "windows-2019-containerd",
		OS:      "windows",
		Arch:    "amd64",
		Distro:  datamodel.AKSWindows2019Containerd,
		Version: "17763.6414.241010", // TODO: use the latest version
		Gallery: windowsGallery,
	}
)

var ErrNotFound = fmt.Errorf("not found")

type Image struct {
	Arch    string
	Distro  datamodel.Distro
	Name    string
	OS      string
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

func (i *Image) VHDResourceID(ctx context.Context, t *testing.T) (VHDResourceID, error) {
	i.vhdOnce.Do(func() {
		if i.Version != "" {
			i.vhd, i.vhdErr = Azure.EnsureSIGImageVersion(ctx, i)
		} else {
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
