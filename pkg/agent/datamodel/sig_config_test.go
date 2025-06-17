package datamodel

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetMaintainedLinuxSIGImageConfigMap", func() {
	It("should return the correct value", func() {
		expected := map[Distro]SigImageConfig{
			AKSUbuntuFipsContainerd2004:           SIGUbuntuFipsContainerd2004ImageConfigTemplate.WithOptions(),
			AKSUbuntuFipsContainerd2004Gen2:       SIGUbuntuFipsContainerd2004Gen2ImageConfigTemplate.WithOptions(),
			AKSUbuntuArm64Containerd2204Gen2:      SIGUbuntuArm64Containerd2204Gen2ImageConfigTemplate.WithOptions(),
			AKSUbuntuArm64Containerd2404Gen2:      SIGUbuntuArm64Containerd2404Gen2ImageConfigTemplate.WithOptions(),
			AKSUbuntuArm64GB200Containerd2404Gen2: SIGUbuntuArm64GB200Containerd2404Gen2ImageConfigTemplate.WithOptions(),
			AKSUbuntuContainerd2204:               SIGUbuntuContainerd2204ImageConfigTemplate.WithOptions(),
			AKSUbuntuContainerd2204Gen2:           SIGUbuntuContainerd2204Gen2ImageConfigTemplate.WithOptions(),
			AKSUbuntuContainerd2204TLGen2:         SIGUbuntuContainerd2204TLGen2ImageConfigTemplate.WithOptions(),
			AKSUbuntuContainerd2004CVMGen2:        SIGUbuntuContainerd2004CVMGen2ImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV3CVMGen2:                SIGAzureLinuxV3CVMGen2ImageConfigTemplate.WithOptions(),
			AKSUbuntuContainerd2404:               SIGUbuntuContainerd2404ImageConfigTemplate.WithOptions(),
			AKSUbuntuContainerd2404Gen2:           SIGUbuntuContainerd2404Gen2ImageConfigTemplate.WithOptions(),
			AKSCBLMarinerV2:                       SIGCBLMarinerV2Gen1ImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV2:                       SIGAzureLinuxV2Gen1ImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV3:                       SIGAzureLinuxV3Gen1ImageConfigTemplate.WithOptions(),
			AKSCBLMarinerV2Gen2:                   SIGCBLMarinerV2Gen2ImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV2Gen2:                   SIGAzureLinuxV2Gen2ImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV3Gen2:                   SIGAzureLinuxV3Gen2ImageConfigTemplate.WithOptions(),
			AKSCBLMarinerV2FIPS:                   SIGCBLMarinerV2Gen1FIPSImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV2FIPS:                   SIGAzureLinuxV2Gen1FIPSImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV3FIPS:                   SIGAzureLinuxV3Gen1FIPSImageConfigTemplate.WithOptions(),
			AKSCBLMarinerV2Gen2FIPS:               SIGCBLMarinerV2Gen2FIPSImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV2Gen2FIPS:               SIGAzureLinuxV2Gen2FIPSImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV3Gen2FIPS:               SIGAzureLinuxV3Gen2FIPSImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV3Gen2Kata:               SIGAzureLinuxV3KataImageConfigTemplate.WithOptions(),
			AKSCBLMarinerV2Arm64Gen2:              SIGCBLMarinerV2Arm64ImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV2Arm64Gen2:              SIGAzureLinuxV2Arm64ImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV3Arm64Gen2:              SIGAzureLinuxV3Arm64ImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV3Arm64Gen2FIPS:          SIGAzureLinuxV3Arm64Gen2FIPSImageConfigTemplate.WithOptions(),
			AKSCBLMarinerV2Gen2TL:                 SIGCBLMarinerV2TLImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV2Gen2TL:                 SIGAzureLinuxV2TLImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV3Gen2TL:                 SIGAzureLinuxV3TLImageConfigTemplate.WithOptions(),
			AKSAzureLinuxV3OSGuardGen2FIPSTL:      SIGAzureLinuxV3OSGuardGen2FIPSTLImageConfigTemplate.WithOptions(),
			AKSUbuntuContainerd2404CVMGen2:        SIGUbuntuContainerd2404CVMGen2ImageConfigTemplate.WithOptions(),
			AKSUbuntuContainerd2404TLGen2:         SIGUbuntuContainerd2404TLGen2ImageConfigTemplate.WithOptions(),
			AKSFlatcarGen2:                        SIGFlatcarGen2ImageConfigTemplate.WithOptions(),
			AKSFlatcarArm64Gen2:                   SIGFlatcarArm64Gen2ImageConfigTemplate.WithOptions(),
		}
		actual := GetMaintainedLinuxSIGImageConfigMap()
		for distro, config := range expected {
			Expect(actual).To(HaveKeyWithValue(distro, config))
		}
	})
})

var _ = Describe("GetSIGAzureCloudSpecConfig", func() {
	var (
		config SIGConfig
	)

	BeforeEach(func() {
		galleries := map[string]SIGGalleryConfig{
			"AKSUbuntu": {
				GalleryName:   "aksubuntu",
				ResourceGroup: "resourcegroup",
			},
			"AKSCBLMariner": {
				GalleryName:   "akscblmariner",
				ResourceGroup: "resourcegroup",
			},
			"AKSAzureLinux": {
				GalleryName:   "aksazurelinux",
				ResourceGroup: "resourcegroup",
			},
			"AKSWindows": {
				GalleryName:   "AKSWindows",
				ResourceGroup: "AKS-Windows",
			},
			"AKSUbuntuEdgeZone": {
				GalleryName:   "AKSUbuntuEdgeZone",
				ResourceGroup: "AKS-Ubuntu-EdgeZone",
			},
			"AKSFlatcar": {
				GalleryName:   "aksflatcar",
				ResourceGroup: "resourcegroup",
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

		Expect(len(sigConfig.SigUbuntuImageConfig)).To(Equal(18))

		Expect(len(sigConfig.SigCBLMarinerImageConfig)).To(Equal(9))

		mariner := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV1]
		Expect(mariner.ResourceGroup).To(Equal("resourcegroup"))
		Expect(mariner.Gallery).To(Equal("akscblmariner"))
		Expect(mariner.Definition).To(Equal("V1"))
		Expect(mariner.Version).To(Equal(FrozenCBLMarinerV1SIGImageVersionForDeprecation))

		Expect(len(sigConfig.SigAzureLinuxImageConfig)).To(Equal(17))

		azurelinuxV2 := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV2]
		Expect(azurelinuxV2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV2.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV2.Definition).To(Equal("V2"))
		Expect(azurelinuxV2.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV3 := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV3]
		Expect(azurelinuxV3.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV3.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV3.Definition).To(Equal("V3"))
		Expect(azurelinuxV3.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV2Gen2 := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV2Gen2]
		Expect(azurelinuxV2Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV2Gen2.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV2Gen2.Definition).To(Equal("V2gen2"))
		Expect(azurelinuxV2Gen2.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV3Gen2 := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV3Gen2]
		Expect(azurelinuxV3Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV3Gen2.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV3Gen2.Definition).To(Equal("V3gen2"))
		Expect(azurelinuxV3Gen2.Version).To(Equal(LinuxSIGImageVersion))

		Expect(len(sigConfig.SigWindowsImageConfig)).To(Equal(8))

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

		windows2025 := sigConfig.SigWindowsImageConfig[AKSWindows2025]
		Expect(windows2025.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows2025.Gallery).To(Equal("AKSWindows"))
		Expect(windows2025.Definition).To(Equal("windows-2025"))
		Expect(windows2025.Version).To(Equal("26100.2025.221114"))

		windows2025Gen2 := sigConfig.SigWindowsImageConfig[AKSWindows2025Gen2]
		Expect(windows2025Gen2.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows2025Gen2.Gallery).To(Equal("AKSWindows"))
		Expect(windows2025Gen2.Definition).To(Equal("windows-2025-gen2"))
		Expect(windows2025Gen2.Version).To(Equal("26100.2025.221114"))

		aksUbuntuArm642204Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuArm64Containerd2204Gen2]
		Expect(aksUbuntuArm642204Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuArm642204Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuArm642204Gen2.Definition).To(Equal("2204gen2arm64containerd"))
		Expect(aksUbuntuArm642204Gen2.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntuArm642404Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuArm64Containerd2404Gen2]
		Expect(aksUbuntuArm642404Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuArm642404Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuArm642404Gen2.Definition).To(Equal("2404gen2arm64containerd"))
		Expect(aksUbuntuArm642404Gen2.Version).To(Equal(LinuxSIGImageVersion))

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
		Expect(azurelinuxV3Arm64.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntu2204TLGen2Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2204TLGen2]
		Expect(aksUbuntu2204TLGen2Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2204TLGen2Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2204TLGen2Containerd.Definition).To(Equal("2204gen2TLcontainerd"))
		Expect(aksUbuntu2204TLGen2Containerd.Version).To(Equal(LinuxSIGImageVersion))

		Expect(len(sigConfig.SigUbuntuEdgeZoneImageConfig)).To(Equal(2))

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

		azurelinuxV3Gen2TL := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV3Gen2TL]
		Expect(azurelinuxV3Gen2TL.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV3Gen2TL.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV3Gen2TL.Definition).To(Equal("V3gen2TL"))
		Expect(azurelinuxV3Gen2TL.Version).To(Equal(LinuxSIGImageVersion))

		marinerV2KataGen2TL := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV2KataGen2TL]
		Expect(marinerV2KataGen2TL.ResourceGroup).To(Equal("resourcegroup"))
		Expect(marinerV2KataGen2TL.Gallery).To(Equal("akscblmariner"))
		Expect(marinerV2KataGen2TL.Definition).To(Equal("V2katagen2TL"))
		Expect(marinerV2KataGen2TL.Version).To(Equal(FrozenCBLMarinerV2KataGen2TLSIGImageVersion))

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
		Expect(azurelinuxV3FIPS.Version).To(Equal(LinuxSIGImageVersion))

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
		Expect(azurelinuxV3Gen2FIPS.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV3Arm64Gen2FIPS := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV3Arm64Gen2FIPS]
		Expect(azurelinuxV3Arm64Gen2FIPS.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV3Arm64Gen2FIPS.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV3Arm64Gen2FIPS.Definition).To(Equal("V3gen2arm64fips"))
		Expect(azurelinuxV3Arm64Gen2FIPS.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV2Gen2Kata := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV2Gen2Kata]
		Expect(azurelinuxV2Gen2Kata.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV2Gen2Kata.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV2Gen2Kata.Definition).To(Equal("V2katagen2"))
		Expect(azurelinuxV2Gen2Kata.Version).To(Equal(FrozenAzureLinuxV2KataGen2SIGImageVersion))

		azurelinuxV3Gen2Kata := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV3Gen2Kata]
		Expect(azurelinuxV3Gen2Kata.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV3Gen2Kata.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV3Gen2Kata.Definition).To(Equal("V3katagen2"))
		Expect(azurelinuxV3Gen2Kata.Version).To(Equal(LinuxSIGImageVersion))

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
		Expect(aksUbuntu2404Containerd.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntu2404Gen2Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2404Gen2]
		Expect(aksUbuntu2404Gen2Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2404Gen2Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2404Gen2Containerd.Definition).To(Equal("2404gen2containerd"))
		Expect(aksUbuntu2404Gen2Containerd.Version).To(Equal(LinuxSIGImageVersion))

		azurelinuxV3CVMGen2 := sigConfig.SigAzureLinuxImageConfig[AKSAzureLinuxV3CVMGen2]
		Expect(azurelinuxV3CVMGen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(azurelinuxV3CVMGen2.Gallery).To(Equal("aksazurelinux"))
		Expect(azurelinuxV3CVMGen2.Definition).To(Equal("V3gen2CVM"))
		Expect(azurelinuxV3CVMGen2.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntu2404CVMGen2Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2404CVMGen2]
		Expect(aksUbuntu2404CVMGen2Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2404CVMGen2Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2404CVMGen2Containerd.Definition).To(Equal("2404gen2CVMcontainerd"))
		Expect(aksUbuntu2404CVMGen2Containerd.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntu2404TLGen2Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2404TLGen2]
		Expect(aksUbuntu2404TLGen2Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2404TLGen2Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2404TLGen2Containerd.Definition).To(Equal("2404gen2TLcontainerd"))
		Expect(aksUbuntu2404TLGen2Containerd.Version).To(Equal(LinuxSIGImageVersion))
	})
})
