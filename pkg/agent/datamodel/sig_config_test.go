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

		Expect(len(sigConfig.SigUbuntuImageConfig)).To(Equal(21))

		aksUbuntuGPU1804Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuGPU1804Gen2]
		Expect(aksUbuntuGPU1804Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuGPU1804Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuGPU1804Gen2.Definition).To(Equal("1804gen2gpu"))
		Expect(aksUbuntuGPU1804Gen2.Version).To(Equal("2022.08.29"))

		Expect(len(sigConfig.SigCBLMarinerImageConfig)).To(Equal(7))

		mariner := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV1]
		Expect(mariner.ResourceGroup).To(Equal("resourcegroup"))
		Expect(mariner.Gallery).To(Equal("akscblmariner"))
		Expect(mariner.Definition).To(Equal("V1"))
		Expect(mariner.Version).To(Equal(LinuxSIGImageVersion))

		Expect(len(sigConfig.SigWindowsImageConfig)).To(Equal(4))

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

		aksUbuntuArm641804Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuArm64Containerd1804Gen2]
		Expect(aksUbuntuArm641804Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuArm641804Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuArm641804Gen2.Definition).To(Equal("1804gen2arm64containerd"))
		Expect(aksUbuntuArm641804Gen2.Version).To(Equal(LinuxSIGImageVersion))

		aksUbuntuArm642204Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuArm64Containerd2204Gen2]
		Expect(aksUbuntuArm642204Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuArm642204Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuArm642204Gen2.Definition).To(Equal("2204gen2arm64containerd"))
		Expect(aksUbuntuArm642204Gen2.Version).To(Equal(LinuxSIGImageVersion))

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
		Expect(aksUbuntu2004CVMGen2Containerd.Version).To(Equal("202304.10.0"))

		marinerV2Arm64 := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV2Arm64Gen2]
		Expect(marinerV2Arm64.ResourceGroup).To(Equal("resourcegroup"))
		Expect(marinerV2Arm64.Gallery).To(Equal("akscblmariner"))
		Expect(marinerV2Arm64.Definition).To(Equal("V2gen2arm64"))
		Expect(marinerV2Arm64.Version).To(Equal(LinuxSIGImageVersion))

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
		Expect(aksUbuntuEdgeZoneContainerd1804.Version).To(Equal(EdgeZoneSIGImageVersion))

		aksUbuntuEdgeZoneContainerd1804Gen2 := sigConfig.SigUbuntuEdgeZoneImageConfig[AKSUbuntuEdgeZoneContainerd1804Gen2]
		Expect(aksUbuntuEdgeZoneContainerd1804Gen2.ResourceGroup).To(Equal("AKS-Ubuntu-EdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd1804Gen2.Gallery).To(Equal("AKSUbuntuEdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd1804Gen2.Definition).To(Equal("1804gen2containerd"))
		Expect(aksUbuntuEdgeZoneContainerd1804Gen2.Version).To(Equal(EdgeZoneSIGImageVersion))

		aksUbuntuEdgeZoneContainerd2204 := sigConfig.SigUbuntuEdgeZoneImageConfig[AKSUbuntuEdgeZoneContainerd2204]
		Expect(aksUbuntuEdgeZoneContainerd2204.ResourceGroup).To(Equal("AKS-Ubuntu-EdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd2204.Gallery).To(Equal("AKSUbuntuEdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd2204.Definition).To(Equal("2204containerd"))
		Expect(aksUbuntuEdgeZoneContainerd2204.Version).To(Equal(EdgeZoneSIGImageVersion))

		aksUbuntuEdgeZoneContainerd2204Gen2 := sigConfig.SigUbuntuEdgeZoneImageConfig[AKSUbuntuEdgeZoneContainerd2204Gen2]
		Expect(aksUbuntuEdgeZoneContainerd2204Gen2.ResourceGroup).To(Equal("AKS-Ubuntu-EdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd2204Gen2.Gallery).To(Equal("AKSUbuntuEdgeZone"))
		Expect(aksUbuntuEdgeZoneContainerd2204Gen2.Definition).To(Equal("2204gen2containerd"))
		Expect(aksUbuntuEdgeZoneContainerd2204Gen2.Version).To(Equal(EdgeZoneSIGImageVersion))

		marinerV2Gen2TL := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV2Gen2TL]
		Expect(marinerV2Gen2TL.ResourceGroup).To(Equal("resourcegroup"))
		Expect(marinerV2Gen2TL.Gallery).To(Equal("akscblmariner"))
		Expect(marinerV2Gen2TL.Definition).To(Equal("V2gen2TL"))
		Expect(marinerV2Gen2TL.Version).To(Equal(LinuxSIGImageVersion))

		marinerV2KataGen2TL := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV2KataGen2TL]
		Expect(marinerV2KataGen2TL.ResourceGroup).To(Equal("resourcegroup"))
		Expect(marinerV2KataGen2TL.Gallery).To(Equal("akscblmariner"))
		Expect(marinerV2KataGen2TL.Definition).To(Equal("V2katagen2TL"))
		Expect(marinerV2KataGen2TL.Version).To(Equal(CBLMarinerV2KataGen2TLSIGImageVersion))
	})
})
