package cache

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cache suite", func() {
	Context("get cached data", func() {
		It("should have the correct manifest and components data cached", func() {
			//TODO: improve test logic
			manifest, err := getManifest()
			Expect(err).NotTo(HaveOccurred())

			components, err := getComponents()
			Expect(err).NotTo(HaveOccurred())

			// The indices are hardcoded based on the current components.json.
			// Add new components to the bottom of components.json, or update the indices.
			pauseIndx := 2
			azureCNSIndx := 5
			cniPluginIndx := 0
			azureCNIIndx := 1

			Expect(onVHD.FromManifest.Runc.Installed["default"]).To(Equal(manifest.Runc.Installed["default"]))
			Expect(onVHD.FromManifest.Runc.Pinned["1804"]).To(Equal(manifest.Runc.Pinned["1804"]))
			Expect(onVHD.FromManifest.Containerd.Pinned["1804"]).To(Equal(manifest.Containerd.Pinned["1804"]))
			Expect(onVHD.FromManifest.Containerd.Edge).To(Equal(manifest.Containerd.Edge))
			Expect(onVHD.FromManifest.Kubernetes.Versions[0]).To(Equal(manifest.Kubernetes.Versions[0]))
			Expect(onVHD.FromComponentContainerImages["pause"].MultiArchVersions[0]).To(Equal(components.ContainerImages[pauseIndx].MultiArchVersions[0]))
			Expect(onVHD.FromComponentContainerImages["azure-cns"].PrefetchOptimizations[0].Version).To(
				Equal(components.ContainerImages[azureCNSIndx].PrefetchOptimizations[0].Version))
			Expect(onVHD.FromComponentContainerImages["azure-cns"].PrefetchOptimizations[0].Binaries[0]).To(Equal("usr/local/bin/azure-cns"))
			Expect(onVHD.FromComponentDownloadedFiles["cni-plugins"].Versions[0]).To(Equal(components.DownloadFiles[cniPluginIndx].Versions[0]))
			Expect(onVHD.FromComponentDownloadedFiles["azure-cni"].Versions[1]).To(Equal(components.DownloadFiles[azureCNIIndx].Versions[1]))
		})
	})

	Context("getContainerImageNameFromURL", func() {
		When("URL is empty", func() {
			It("should return an error", func() {
				var url string
				name, err := getContainerImageNameFromURL(url)
				Expect(name).To(BeEmpty())
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("container image component URL is not in the expected format: "))
			})
		})

		When("URL is valid", func() {
			It("should return the container image name", func() {
				url := "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:*"
				name, err := getContainerImageNameFromURL(url)
				Expect(err).ToNot(HaveOccurred())
				Expect(name).To(Equal("addon-resizer"))
			})
		})
	})

	Context("getFileNameFromURL", func() {
		When("URL is empty", func() {
			It("should return an error", func() {
				var url string
				name, err := getFileNameFromURL(url)
				Expect(name).To(BeEmpty())
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("download file image URL is not in the expected format: "))
			})
		})

		When("URL is valid", func() {
			It("should return the component name", func() {
				url := "https://acs-mirror.azureedge.net/cni-plugins/v*/binaries"
				name, err := getFileNameFromURL(url)
				Expect(err).ToNot(HaveOccurred())
				Expect(name).To(Equal("cni-plugins"))
			})
		})
	})
})
