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

		Expect(len(sigConfig.SigUbuntuImageConfig)).To(Equal(14))

		aksUbuntuGPU1804Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuGPU1804Gen2]
		Expect(aksUbuntuGPU1804Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuGPU1804Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuGPU1804Gen2.Definition).To(Equal("1804gen2gpu"))
		Expect(aksUbuntuGPU1804Gen2.Version).To(Equal("2022.04.13"))

		Expect(len(sigConfig.SigCBLMarinerImageConfig)).To(Equal(1))

		mariner := sigConfig.SigCBLMarinerImageConfig[AKSCBLMarinerV1]
		Expect(mariner.ResourceGroup).To(Equal("resourcegroup"))
		Expect(mariner.Gallery).To(Equal("akscblmariner"))
		Expect(mariner.Definition).To(Equal("V1"))
		Expect(mariner.Version).To(Equal("2022.04.13"))

		Expect(len(sigConfig.SigWindowsImageConfig)).To(Equal(3))

		windows2019 := sigConfig.SigWindowsImageConfig[AKSWindows2019]
		Expect(windows2019.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows2019.Gallery).To(Equal("AKSWindows"))
		Expect(windows2019.Definition).To(Equal("windows-2019"))
		Expect(windows2019.Version).To(Equal("17763.2686.220322"))

		windows2019Containerd := sigConfig.SigWindowsImageConfig[AKSWindows2019Containerd]
		Expect(windows2019Containerd.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows2019Containerd.Gallery).To(Equal("AKSWindows"))
		Expect(windows2019Containerd.Definition).To(Equal("windows-2019-containerd"))
		Expect(windows2019Containerd.Version).To(Equal("17763.2686.220322"))

		windows2022Containerd := sigConfig.SigWindowsImageConfig[AKSWindows2022Containerd]
		Expect(windows2022Containerd.ResourceGroup).To(Equal("AKS-Windows"))
		Expect(windows2022Containerd.Gallery).To(Equal("AKSWindows"))
		Expect(windows2022Containerd.Definition).To(Equal("windows-2022-containerd"))
		Expect(windows2022Containerd.Version).To(Equal("20348.587.220322"))

		aksUbuntuArm64804Gen2 := sigConfig.SigUbuntuImageConfig[AKSUbuntuArm64Containerd1804Gen2]
		Expect(aksUbuntuArm64804Gen2.ResourceGroup).To(Equal("resourcegroup"))
		Expect(aksUbuntuArm64804Gen2.Gallery).To(Equal("aksubuntu"))
		Expect(aksUbuntuArm64804Gen2.Definition).To(Equal("1804gen2arm64containerd"))
		Expect(aksUbuntuArm64804Gen2.Version).To(Equal(Arm64LinuxSIGImageVersion))
	})
})
