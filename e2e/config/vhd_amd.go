package config

import "github.com/Azure/agentbaker/pkg/agent/datamodel"

var (
	// The dedicated AMD image is selected only by opt-in GPU scenarios. It uses
	// the existing Ubuntu distro and does not change production SIG defaults.
	VHDUbuntu2404Gen2AMDGPUContainerd = &Image{
		Name:         "2404gen2amdgpucontainerd",
		OS:           OSUbuntu,
		Arch:         "amd64",
		Distro:       datamodel.AKSUbuntuContainerd2404Gen2,
		Gallery:      &Config.GalleryLinux,
		OSDiskSizeGB: 256, // The on-demand PyTorch image is 20.46 GB compressed.
	}
)
