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

		Expect(len(sigConfig.SigUbuntuImageConfig)).To(Equal(17))

		aksUbuntuGPU1804Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuGPU1804Gen2]
		Expect(aksUbuntuGPU1804Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuGPU1804Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuGPU1804Gen2.Definition).To(Equal("1804gen2gpu"))
		Expect(aksUbuntuGPU1804Gen2.Version).To(Equal("2022.07.18"))

		Expect(len(sigConfig.SigCBLMarinerImageConfig)).To(Equal(2))

		mariner := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV1]
		Expect(mariner.ResourceGroup).To(Equal("resourcegroup"))
		Expect(mariner.Gallery).To(Equal("akscblmariner"))
		Expect(mariner.Definition).To(Equal("V1"))
		Expect(mariner.Version).To(Equal("2022.07.18"))

		Expect(len(sigConfig.SigWindowsImageConfig)).To(Equal(3))

		windows2019 := sigConfig.SigWindowsImageConfig[AKSWindows2019]
		Expect(windows2019.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows2019.Gallery).To(Equal("AKSWindows"))
		Expect(windows2019.Definition).To(Equal("windows-2019"))
		Expect(windows2019.Version).To(Equal("17763.3232.220722"))

		windows2019Containerd := sigConfig.SigWindowsImageConfig[AKSWindows2019Containerd]
		Expect(windows2019Containerd.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows2019Containerd.Gallery).To(Equal("AKSWindows"))
		Expect(windows2019Containerd.Definition).To(Equal("windows-2019-containerd"))
		Expect(windows2019Containerd.Version).To(Equal("17763.3232.220722"))

		windows2022Containerd := sigConfig.SigWindowsImageConfig[AKSWindows2022Containerd]
		Expect(windows2022Containerd.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows2022Containerd.Gallery).To(Equal("AKSWindows"))
		Expect(windows2022Containerd.Definition).To(Equal("windows-2022-containerd"))
		Expect(windows2022Containerd.Version).To(Equal("20348.859.220722"))

		aksUbuntuArm64804Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuArm64Containerd1804Gen2]
		Expect(aksUbuntuArm64804Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuArm64804Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuArm64804Gen2.Definition).To(Equal("1804gen2arm64containerd"))
		Expect(aksUbuntuArm64804Gen2.Version).To(Equal(Arm64LinuxSIGImageVersion))

		aksUbuntu2004Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2004]
		Expect(aksUbuntu2004Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2004Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2004Containerd.Definition).To(Equal("2004containerd"))
		Expect(aksUbuntu2004Containerd.Version).To(Equal("2022.04.16"))

		aksUbuntu2004Gen2Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2004Gen2]
		Expect(aksUbuntu2004Gen2Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2004Gen2Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2004Gen2Containerd.Definition).To(Equal("2004gen2containerd"))
		Expect(aksUbuntu2004Gen2Containerd.Version).To(Equal("2022.04.16"))

		aksUbuntu2004CVMGen2Containerd := sigConfig.SigUbuntuImageConfig[AKSUbuntuContainerd2004CVMGen2]
		Expect(aksUbuntu2004CVMGen2Containerd.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntu2004CVMGen2Containerd.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntu2004CVMGen2Containerd.Definition).To(Equal("2004gen2CVMcontainerd"))
		Expect(aksUbuntu2004CVMGen2Containerd.Version).To(Equal("2022.06.16"))
	})
})
