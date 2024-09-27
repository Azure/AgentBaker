package datamodel

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetSIGAzureCloudSpecConfig", func() {
	var (
		config SIGConfig
	)

	BeforeEach(func() {
		galleries := map[string]SIGGalleryConfig{
			"AKSUbuntu": SIGGalleryConfig{
				GalleryName:   "aksubuntu",
				ResourceGroup: "resourcegroup",
			},
			"AKSCBLMariner": SIGGalleryConfig{
				GalleryName:   "akscblmariner",
				ResourceGroup: "resourcegroup",
			},
			"AKSAzureLinux": SIGGalleryConfig{
				GalleryName:   "aksazurelinux",
				ResourceGroup: "resourcegroup",
			},
			"AKSWindows": SIGGalleryConfig{
				GalleryName:   "AKSWindows",
				ResourceGroup: "AKS-Windows",
			},
			"AKSUbuntuEdgeZone": SIGGalleryConfig{
				GalleryName:   "AKSUbuntuEdgeZone",
				ResourceGroup: "AKS-Ubuntu-EdgeZone",
			},
		}
		config = SIGConfig{
			TenantID:       "sometenantid",
			SubscriptionID: "somesubid",
			Galleries:      galleries,
		}
	})

	It("should return correct value", func() {
		sigConfig, err := GetSIGAzureCloudSpecConfig(config, "westus")
		Expect(err).NotTo(HaveOccurred())
		Expect(sigConfig.CloudName).To(Equal("AzurePublicCloud"))
		Expect(sigConfig.SigTenantID).To(Equal("sometenantid"))
		Expect(sigConfig.SubscriptionID).To(Equal("somesubid"))

		Expect(len(sigConfig.SigUbuntuImageConfig)).To(Equal(26))

		aksUbuntuGPU1804Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuGPU1804Gen2]
		Expect(aksUbuntuGPU1804Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuGPU1804Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuGPU1804Gen2.Definition).To(Equal("1804gen2gpu"))
		Expect(aksUbuntuGPU1804Gen2.Version).To(Equal("2022.08.29"))

		Expect(len(sigConfig.SigCBLMarinerImageConfig)).To(Equal(9))

		mariner := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV1]
		Expect(mariner.ResourceGroup).To(Equal("resourcegroup"))
		Expect(mariner.Gallery).To(Equal("akscblmariner"))
		Expect(mariner.Definition).To(Equal("V1"))
		Expect(mariner.Version).To(Equal(FrozenCBLMarinerV1SIGImageVersionForDeprecation))

		Expect(len(sigConfig.SigAzureLinuxImageConfig)).To(Equal(12))

		azurelinuxV2 := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV2]
		Expect(azurelinuxV2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV2.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV2.Definition).To(Equal("V2"))
		Expect(azurelinuxV2.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV3 := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV3]
		Expect(azurelinuxV3.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV3.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV3.Definition).To(Equal("V3"))
		Expect(azurelinuxV3.Version).To(Equal(FrozenAzureLinuxV3SIGImageVersion))

		azurelinuxV2Gen2 := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV2Gen2]
		Expect(azurelinuxV2Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV2Gen2.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV2Gen2.Definition).To(Equal("V2gen2"))
		Expect(azurelinuxV2Gen2.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV3Gen2 := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV3Gen2]
		Expect(azurelinuxV3Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV3Gen2.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV3Gen2.Definition).To(Equal("V3gen2"))
		Expect(azurelinuxV3Gen2.Version).To(Equal(FrozenAzureLinuxV3SIGImageVersion))

		Expect(len(sigConfig.SigWindowsImageConfig)).To(Equal(6))

		windows2019 := sigConfig.SigWindowsImageConfig[AKSWindows2019]
		Expect(windows2019.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows2019.Gallery).To(Equal("AKSWindows"))
		Expect(windows2019.Definition).To(Equal("windows-2019"))
		Expect(windows2019.Version).To(Equal("17763.2019.221114"))

		windows2019Containerd := sigConfig.SigWindowsImageConfig[AKSWindows2019Containerd]
		Expect(windows2019Containerd.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows2019Containerd.Gallery).To(Equal("AKSWindows"))
		Expect(windows2019Containerd.Definition).To(Equal("windows-2019-containerd"))
		Expect(windows2019Containerd.Version).To(Equal("17763.2019.221114"))

		windows2022Containerd := sigConfig.SigWindowsImageConfig[AKSWindows2022Containerd]
		Expect(windows2022Containerd.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows2022Containerd.Gallery).To(Equal("AKSWindows"))
		Expect(windows2022Containerd.Definition).To(Equal("windows-2022-containerd"))
		Expect(windows2022Containerd.Version).To(Equal("20348.2022.221114"))

		windows2022ContainerdGen2 := sigConfig.SigWindowsImageConfig[AKSWindows2022ContainerdGen2]
		Expect(windows2022ContainerdGen2.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows2022ContainerdGen2.Gallery).To(Equal("AKSWindows"))
		Expect(windows2022ContainerdGen2.Definition).To(Equal("windows-2022-containerd-gen2"))
		Expect(windows2022ContainerdGen2.Version).To(Equal("20348.2022.221114"))

		windows23H2 := sigConfig.SigWindowsImageConfig[AKSWindows23H2]
		Expect(windows23H2.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows23H2.Gallery).To(Equal("AKSWindows"))
		Expect(windows23H2.Definition).To(Equal("windows-23H2"))
		Expect(windows23H2.Version).To(Equal("25398.2022.221114"))

		windows23H2Gen2 := sigConfig.SigWindowsImageConfig[AKSWindows23H2Gen2]
		Expect(windows23H2Gen2.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows23H2Gen2.Gallery).To(Equal("AKSWindows"))
		Expect(windows23H2Gen2.Definition).To(Equal("windows-23H2-gen2"))
		Expect(windows23H2Gen2.Version).To(Equal("25398.2022.221114"))

		aksUbuntuArm642204Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuArm64Containerd2204Gen2]
		Expect(aksUbuntuArm642204Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuArm642204Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuArm642204Gen2.Definition).To(Equal("2204gen2arm64containerd"))
		Expect(aksUbuntuArm642204Gen2.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntuArm642404Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuArm64Containerd2404Gen2]
		Expect(aksUbuntuArm642404Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuArm642404Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuArm642404Gen2.Definition).To(Equal("2404gen2arm64containerd"))
		Expect(aksUbuntuArm642404Gen2.Version).To(Equal("202405.20.0"))

		aksUbuntu2204Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2204]
		Expect(aksUbuntu2204Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2204Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2204Containerd.Definition).To(Equal("2204containerd"))
		Expect(aksUbuntu2204Containerd.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntu2204Gen2Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2204Gen2]
		Expect(aksUbuntu2204Gen2Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2204Gen2Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2204Gen2Containerd.Definition).To(Equal("2204gen2containerd"))
		Expect(aksUbuntu2204Gen2Containerd.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntu2004CVMGen2Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2004CVMGen2]
		Expect(aksUbuntu2004CVMGen2Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2004CVMGen2Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2004CVMGen2Containerd.Definition).To(Equal("2004gen2CVMcontainerd"))
		Expect(aksUbuntu2004CVMGen2Containerd.Version).To(Equal(LinuxSIGImageVersion))

		marinerV2Arm64 := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV2Arm64Gen2]
		Expect(marinerV2Arm64.ResourceGroup).To(Equal("resourcegroup"))
		Expect(marinerV2Arm64.Gallery).To(Equal("akscblmariner"))
		Expect(marinerV2Arm64.Definition).To(Equal("V2gen2arm64"))
		Expect(marinerV2Arm64.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV2Arm64 := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV2Arm64Gen2]
		Expect(azurelinuxV2Arm64.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV2Arm64.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV2Arm64.Definition).To(Equal("V2gen2arm64"))
		Expect(azurelinuxV2Arm64.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV3Arm64 := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV3Arm64Gen2]
		Expect(azurelinuxV3Arm64.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV3Arm64.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV3Arm64.Definition).To(Equal("V3gen2arm64"))
		Expect(azurelinuxV3Arm64.Version).To(Equal(FrozenAzureLinuxV3SIGImageVersion))

		aksUbuntu2204TLGen2Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2204TLGen2]
		Expect(aksUbuntu2204TLGen2Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2204TLGen2Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2204TLGen2Containerd.Definition).To(Equal("2204gen2TLcontainerd"))
		Expect(aksUbuntu2204TLGen2Containerd.Version).To(Equal(LinuxSIGImageVersion))

		Expect(len(sigConfig.SigUbuntuEdgeZoneImageConfig)).To(Equal(4))

		aksUbuntuEdgeZoneContainerd1804 := sigConfig.SigUbuntuEdgeZoneImageConfig[AKSUbuntuEdgeZoneContainerd1804]
		Expect(aksUbuntuEdgeZoneContainerd1804.ResourceGroup).To(Equal("AKS-Ubuntu-EdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd1804.Gallery).To(Equal("AKSUbuntuEdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd1804.Definition).To(Equal("1804containerd"))
		Expect(aksUbuntuEdgeZoneContainerd1804.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntuEdgeZoneContainerd1804Gen2 := sigConfig.SigUbuntuEdgeZoneImageConfig[AKSUbuntuEdgeZoneContainerd1804Gen2]
		Expect(aksUbuntuEdgeZoneContainerd1804Gen2.ResourceGroup).To(Equal("AKS-Ubuntu-EdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd1804Gen2.Gallery).To(Equal("AKSUbuntuEdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd1804Gen2.Definition).To(Equal("1804gen2containerd"))
		Expect(aksUbuntuEdgeZoneContainerd1804Gen2.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntuEdgeZoneContainerd2204 := sigConfig.SigUbuntuEdgeZoneImageConfig[AKSUbuntuEdgeZoneContainerd2204]
		Expect(aksUbuntuEdgeZoneContainerd2204.ResourceGroup).To(Equal("AKS-Ubuntu-EdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd2204.Gallery).To(Equal("AKSUbuntuEdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd2204.Definition).To(Equal("2204containerd"))
		Expect(aksUbuntuEdgeZoneContainerd2204.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntuEdgeZoneContainerd2204Gen2 := sigConfig.SigUbuntuEdgeZoneImageConfig[AKSUbuntuEdgeZoneContainerd2204Gen2]
		Expect(aksUbuntuEdgeZoneContainerd2204Gen2.ResourceGroup).To(Equal("AKS-Ubuntu-EdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd2204Gen2.Gallery).To(Equal("AKSUbuntuEdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd2204Gen2.Definition).To(Equal("2204gen2containerd"))
		Expect(aksUbuntuEdgeZoneContainerd2204Gen2.Version).To(Equal(LinuxSIGImageVersion))

		marinerV2Gen2TL := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV2Gen2TL]
		Expect(marinerV2Gen2TL.ResourceGroup).To(Equal("resourcegroup"))
		Expect(marinerV2Gen2TL.Gallery).To(Equal("akscblmariner"))
		Expect(marinerV2Gen2TL.Definition).To(Equal("V2gen2TL"))
		Expect(marinerV2Gen2TL.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV2Gen2TL := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV2Gen2TL]
		Expect(azurelinuxV2Gen2TL.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV2Gen2TL.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV2Gen2TL.Definition).To(Equal("V2gen2TL"))
		Expect(azurelinuxV2Gen2TL.Version).To(Equal(LinuxSIGImageVersion))

		marinerV2KataGen2TL := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV2KataGen2TL]
		Expect(marinerV2KataGen2TL.ResourceGroup).To(Equal("resourcegroup"))
		Expect(marinerV2KataGen2TL.Gallery).To(Equal("akscblmariner"))
		Expect(marinerV2KataGen2TL.Definition).To(Equal("V2katagen2TL"))
		Expect(marinerV2KataGen2TL.Version).To(Equal(CBLMarinerV2KataGen2TLSIGImageVersion))

		marinerV2FIPS := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV2FIPS]
		Expect(marinerV2FIPS.ResourceGroup).To(Equal("resourcegroup"))
		Expect(marinerV2FIPS.Gallery).To(Equal("akscblmariner"))
		Expect(marinerV2FIPS.Definition).To(Equal("V2fips"))
		Expect(marinerV2FIPS.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV2FIPS := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV2FIPS]
		Expect(azurelinuxV2FIPS.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV2FIPS.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV2FIPS.Definition).To(Equal("V2fips"))
		Expect(azurelinuxV2FIPS.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV3FIPS := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV3FIPS]
		Expect(azurelinuxV3FIPS.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV3FIPS.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV3FIPS.Definition).To(Equal("V3fips"))
		Expect(azurelinuxV3FIPS.Version).To(Equal(FrozenAzureLinuxV3SIGImageVersion))

		marinerV2Gen2FIPS := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV2Gen2FIPS]
		Expect(marinerV2Gen2FIPS.ResourceGroup).To(Equal("resourcegroup"))
		Expect(marinerV2Gen2FIPS.Gallery).To(Equal("akscblmariner"))
		Expect(marinerV2Gen2FIPS.Definition).To(Equal("V2gen2fips"))
		Expect(marinerV2Gen2FIPS.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV2Gen2FIPS := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV2Gen2FIPS]
		Expect(azurelinuxV2Gen2FIPS.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV2Gen2FIPS.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV2Gen2FIPS.Definition).To(Equal("V2gen2fips"))
		Expect(azurelinuxV2Gen2FIPS.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV3Gen2FIPS := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV3Gen2FIPS]
		Expect(azurelinuxV3Gen2FIPS.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV3Gen2FIPS.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV3Gen2FIPS.Definition).To(Equal("V3gen2fips"))
		Expect(azurelinuxV3Gen2FIPS.Version).To(Equal(FrozenAzureLinuxV3SIGImageVersion))

		azurelinuxV2Gen2Kata := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV2Gen2Kata]
		Expect(azurelinuxV2Gen2Kata.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV2Gen2Kata.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV2Gen2Kata.Definition).To(Equal("V2katagen2"))
		Expect(azurelinuxV2Gen2Kata.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntuMinimalContainerd2204 := sigConfig.SigUbuntuImageConfig[AKSUbuntuMinimalContainerd2204]
		Expect(aksUbuntuMinimalContainerd2204.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuMinimalContainerd2204.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuMinimalContainerd2204.Definition).To(Equal("2204minimalcontainerd"))
		Expect(aksUbuntuMinimalContainerd2204.Version).To(Equal("202401.12.0"))

		aksUbuntuMinimalContainerd2204Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuMinimalContainerd2204Gen2]
		Expect(aksUbuntuMinimalContainerd2204Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuMinimalContainerd2204Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuMinimalContainerd2204Gen2.Definition).To(Equal("2204gen2minimalcontainerd"))
		Expect(aksUbuntuMinimalContainerd2204Gen2.Version).To(Equal("202401.12.0"))

		aksUbuntuEgressContainerd2204Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuEgressContainerd2204Gen2]
		Expect(aksUbuntuEgressContainerd2204Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuEgressContainerd2204Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuEgressContainerd2204Gen2.Definition).To(Equal("2204gen2containerd"))
		Expect(aksUbuntuEgressContainerd2204Gen2.Version).To(Equal("2022.10.03"))

		aksUbuntu2404Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2404]
		Expect(aksUbuntu2404Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2404Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2404Containerd.Definition).To(Equal("2404containerd"))
		Expect(aksUbuntu2404Containerd.Version).To(Equal("202405.20.0"))

		aksUbuntu2404Gen2Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2404Gen2]
		Expect(aksUbuntu2404Gen2Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2404Gen2Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2404Gen2Containerd.Definition).To(Equal("2404gen2containerd"))
		Expect(aksUbuntu2404Gen2Containerd.Version).To(Equal("202405.20.0"))
	})
})
