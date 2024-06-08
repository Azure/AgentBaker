package cache

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cache suite", func() {
	Context("get cached data", func() {
		It("should have the correct manifest and components data cached", func() {
			// TODO: improve test logic
			manifest, err := getManifest()
			Expect(err).NotTo(HaveOccurred())

			components, err := getComponents()
			Expect(err).NotTo(HaveOccurred())

			// The indices are hardcoded based on the current components.json.
			// Add new components to the bottom of components.json, or update the indices.
			pauseIndx := 2
			azureCNSIndx := 6

			// find from components.Packages where Name is a specific value
			for _, p := range components.Packages {
				switch p.Name {
				case "containerd":
					Expect(onVHD.FromComponentPackages["containerd"].DownloadURIs["default"].Current.Versions).To(Equal(
						p.DownloadURIs["default"].Current.Versions))
					Expect(onVHD.FromComponentPackages["containerd"].DownloadURIs["mariner"].Current.Versions).To(Equal(
						p.DownloadURIs["mariner"].Current.Versions))
					Expect(onVHD.FromComponentPackages["containerd"].DownloadURIs["ubuntu"].Current.Versions).To(Equal(
						p.DownloadURIs["ubuntu"].Current.Versions))
					Expect(onVHD.FromComponentPackages["containerd"].DownloadURIs["default"].R1804.Versions).To(Equal(
						p.DownloadURIs["default"].R1804.Versions))
					Expect(onVHD.FromComponentPackages["containerd"].DownloadURIs["mariner"].R1804.Versions).To(Equal(
						p.DownloadURIs["mariner"].R1804.Versions))
					Expect(onVHD.FromComponentPackages["containerd"].DownloadURIs["ubuntu"].R1804.Versions).To(Equal(
						p.DownloadURIs["ubuntu"].R1804.Versions))
				case "runc":
					Expect(onVHD.FromComponentPackages["runc"].DownloadURIs["default"].Current.Versions).To(Equal(
						p.DownloadURIs["default"].Current.Versions))
					Expect(onVHD.FromComponentPackages["runc"].DownloadURIs["mariner"].Current.Versions).To(Equal(
						p.DownloadURIs["mariner"].Current.Versions))
					Expect(onVHD.FromComponentPackages["runc"].DownloadURIs["ubuntu"].Current.Versions).To(Equal(
						p.DownloadURIs["ubuntu"].Current.Versions))
					Expect(onVHD.FromComponentPackages["runc"].DownloadURIs["ubuntu"].R1804.Versions).To(Equal(
						p.DownloadURIs["ubuntu"].R1804.Versions))
					Expect(onVHD.FromComponentPackages["runc"].DownloadURIs["ubuntu"].R2004.Versions).To(Equal(
						p.DownloadURIs["ubuntu"].R2004.Versions))
					Expect(onVHD.FromComponentPackages["runc"].DownloadURIs["ubuntu"].R2204.Versions).To(Equal(
						p.DownloadURIs["ubuntu"].R2204.Versions))
				case "cni-plugins-linux-amd64-v*":
					Expect(onVHD.FromComponentPackages["cni-plugins"].DownloadURIs["default"].Current.Versions).To(Equal(
						p.DownloadURIs["default"].Current.Versions))
				case "azure-vnet-cni-linux-amd64-v*":
					Expect(onVHD.FromComponentPackages["azure-cni"].DownloadURIs["default"].Current.Versions).To(Equal(
						p.DownloadURIs["default"].Current.Versions))
				}
			}
			Expect(onVHD.FromManifest.Kubernetes.Versions[0]).To(Equal(manifest.Kubernetes.Versions[0]))
			Expect(onVHD.FromComponentContainerImages["pause"].MultiArchVersions[0]).To(Equal(components.ContainerImages[pauseIndx].MultiArchVersions[0]))
			Expect(onVHD.FromComponentContainerImages["azure-cns"].PrefetchOptimizations[0].Version).To(
				Equal(components.ContainerImages[azureCNSIndx].PrefetchOptimizations[0].Version))
			Expect(onVHD.FromComponentContainerImages["azure-cns"].PrefetchOptimizations[0].Binaries[0]).To(Equal("usr/local/bin/azure-cns"))
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
