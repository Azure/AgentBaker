package datamodel

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	MockComponents = `
{
	"ContainerImages": [
		{
			"downloadURL": "mcr.microsoft.com/unittest/mockimage:*",
			"amd64OnlyVersions": [
				"mock.000000.1"
			],
			"multiArchVersions": [
				"mock.000000.2"
			]
		}
	],
	"DownloadFiles": [
		{
			"fileName": "unittestmockimage*",
			"downloadLocation": "/test/downloads",
			"downloadURL": "mcr.microsoft.com/unittest/mockimage/v*/downloads",
			"versions": [
				"mock.111111.1"
			]
		}
	]
}
`

	MockComponentsBarebone = `
{
	"ContainerImages": [
		{
			"downloadURL": "mcr.microsoft.com/unittest/mockimage:*",
			"multiArchVersions": [
				"mock.000000.1"
			]
		},
		{
			"downloadURL": "mcr.microsoft.com/unittest/mockimage2:*",
			"amd64OnlyVersions": [],
			"multiArchVersions": [
				"mock.000000.2"
			]
		}
	],
	"DownloadFiles": [
		{
			"fileName": "unittestmockimage*",
			"downloadLocation": "/test/downloads",
			"downloadURL": "mcr.microsoft.com/unittest/mockimage/v*/downloads",
			"versions": [
				"mock.111111.1"
			]
		}
	]
}
`

	MockKubeProxyImage = `
{
	"dockerKubeProxyImages": {
		"ContainerImages": [
			{
				"downloadURL": "mcr.microsoft.com/unittest/mockimage:v*",
				"amd64OnlyVersions": [
					"test.111111.0"
				],
				"multiArchVersions": [
					"test.111111.1"
				]
			}
		]
	},
	"containerdKubeProxyImages": {
		"ContainerImages": [
			{
				"downloadURL": "mcr.microsoft.com/unittest/mockimage:v*",
				"amd64OnlyVersions": [
					"test.000000.0"
				],
				"multiArchVersions": [
					"test.000000.1"
				]
			}
		]
	}
}
`

	MockKubeProxyImageBarebone = `
{
    "dockerKubeProxyImages": null,
    "containerdKubeProxyImages": {
        "ContainerImages": [
            {
                "downloadURL": "mcr.microsoft.com/unittest/mockimage:v*",
                "amd64OnlyVersions": [],
                "multiArchVersions": [
                    "test.000000.1"
                ]
            }
        ]
    }
}
`
)

var _ = Describe("Test Components", func() {
	Context("Test NewComponentsFromFile", func() {
		It("should return error when file does not exist", func() {
			_, err := NewComponentsFromFile("not-exist")
			Expect(err).To(HaveOccurred())
		})

		It("should return error when file is not valid json", func() {
			// prepare invalid json file
			testcomponent, err := os.CreateTemp("", "invalid.json")
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(testcomponent.Name())

			_, err = NewComponentsFromFile(testcomponent.Name())
			Expect(err).To(HaveOccurred())
		})

		It("should return correct components", func() {
			// prepare valid json file
			testcomponent, err := os.CreateTemp("", "valid.json")
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(testcomponent.Name())
			// write MockComponents to test file
			err = os.WriteFile(testcomponent.Name(), []byte(MockComponents), 0644)
			Expect(err).ToNot(HaveOccurred())

			components, err := NewComponentsFromFile(testcomponent.Name())
			Expect(err).ToNot(HaveOccurred())

			// verify ContainerImages
			Expect(components.ContainerImages).To(HaveLen(1))
			Expect(components.ContainerImages[0].DownloadURL).To(Equal("mcr.microsoft.com/unittest/mockimage:*"))
			Expect(components.ContainerImages[0].Amd64OnlyVersions).To(HaveLen(1))
			Expect(components.ContainerImages[0].MultiArchVersions).To(HaveLen(1))
			Expect(components.ContainerImages[0].Amd64OnlyVersions[0]).To(Equal("mock.000000.1"))
			Expect(components.ContainerImages[0].MultiArchVersions[0]).To(Equal("mock.000000.2"))

			// verify DownloadFiles
			Expect(components.DownloadFiles).To(HaveLen(1))
			Expect(components.DownloadFiles[0].FileName).To(Equal("unittestmockimage*"))
			Expect(components.DownloadFiles[0].DownloadLocation).To(Equal("/test/downloads"))
			Expect(components.DownloadFiles[0].DownloadURL).To(Equal("mcr.microsoft.com/unittest/mockimage/v*/downloads"))
			Expect(components.DownloadFiles[0].Versions).To(HaveLen(1))
			Expect(components.DownloadFiles[0].Versions[0]).To(Equal("mock.111111.1"))
		})

		It("should return correct components if some config are omitted or null", func() {
			// prepare valid json file
			testcomponent, err := os.CreateTemp("", "valid.json")
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(testcomponent.Name())
			// write MockComponents to test file
			err = os.WriteFile(testcomponent.Name(), []byte(MockComponentsBarebone), 0644)
			Expect(err).ToNot(HaveOccurred())

			components, err := NewComponentsFromFile(testcomponent.Name())
			Expect(err).ToNot(HaveOccurred())

			// verify ContainerImages
			Expect(components.ContainerImages).To(HaveLen(2))
			Expect(components.ContainerImages[0].DownloadURL).To(Equal("mcr.microsoft.com/unittest/mockimage:*"))
			Expect(components.ContainerImages[0].Amd64OnlyVersions).To(HaveLen(0))
			Expect(components.ContainerImages[0].MultiArchVersions).To(HaveLen(1))
			Expect(components.ContainerImages[0].MultiArchVersions[0]).To(Equal("mock.000000.1"))
			Expect(components.ContainerImages[1].DownloadURL).To(Equal("mcr.microsoft.com/unittest/mockimage2:*"))
			Expect(components.ContainerImages[1].Amd64OnlyVersions).To(HaveLen(0))
			Expect(components.ContainerImages[1].MultiArchVersions).To(HaveLen(1))
			Expect(components.ContainerImages[1].MultiArchVersions[0]).To(Equal("mock.000000.2"))
		})
	})

	Context("Test ToImageList", func() {
		It("should return correct image list", func() {
			// prepare valid json file
			testcomponent, err := os.CreateTemp("", "valid.json")
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(testcomponent.Name())
			// write MockComponents to test file
			err = os.WriteFile(testcomponent.Name(), []byte(MockComponents), 0644)
			Expect(err).ToNot(HaveOccurred())

			components, err := NewComponentsFromFile(testcomponent.Name())
			Expect(err).ToNot(HaveOccurred())

			imageList := components.ToImageList()
			Expect(imageList).To(HaveLen(2))
			Expect(imageList[0]).To(Equal("mcr.microsoft.com/unittest/mockimage:mock.000000.1"))
			Expect(imageList[1]).To(Equal("mcr.microsoft.com/unittest/mockimage:mock.000000.2"))
		})
	})
})

var _ = Describe("Test KubeProxyImages", func() {
	Context("Test NewKubeProxyImagesFromFile", func() {
		It("should return error when file does not exist", func() {
			_, err := NewKubeProxyImagesFromFile("not-exist")
			Expect(err).To(HaveOccurred())
		})

		It("should return error when file is not valid json", func() {
			// prepare invalid json file
			testcomponent, err := os.CreateTemp("", "invalid.json")
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(testcomponent.Name())

			_, err = NewKubeProxyImagesFromFile(testcomponent.Name())
			Expect(err).To(HaveOccurred())
		})

		It("should return correct components", func() {
			// prepare valid json file
			testcomponent, err := os.CreateTemp("", "valid.json")
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(testcomponent.Name())
			// write MockKubeProxyImage to test file
			err = os.WriteFile(testcomponent.Name(), []byte(MockKubeProxyImage), 0644)
			Expect(err).ToNot(HaveOccurred())

			kubeProxyImages, err := NewKubeProxyImagesFromFile(testcomponent.Name())
			Expect(err).ToNot(HaveOccurred())

			// verify ContainerImages
			Expect(kubeProxyImages.DockerKubeProxyImages).NotTo(BeNil())
			Expect(kubeProxyImages.DockerKubeProxyImages.ContainerImages).To(HaveLen(1))
			Expect(kubeProxyImages.DockerKubeProxyImages.ContainerImages[0].DownloadURL).To(Equal("mcr.microsoft.com/unittest/mockimage:v*"))
			Expect(kubeProxyImages.DockerKubeProxyImages.ContainerImages[0].Amd64OnlyVersions).To(HaveLen(1))
			Expect(kubeProxyImages.DockerKubeProxyImages.ContainerImages[0].Amd64OnlyVersions[0]).To(Equal("test.111111.0"))
			Expect(kubeProxyImages.DockerKubeProxyImages.ContainerImages[0].MultiArchVersions).To(HaveLen(1))
			Expect(kubeProxyImages.DockerKubeProxyImages.ContainerImages[0].MultiArchVersions[0]).To(Equal("test.111111.1"))
			Expect(kubeProxyImages.ContainerdKubeProxyImages).NotTo(BeNil())
			Expect(kubeProxyImages.ContainerdKubeProxyImages.ContainerImages).To(HaveLen(1))
			Expect(kubeProxyImages.ContainerdKubeProxyImages.ContainerImages[0].DownloadURL).To(Equal("mcr.microsoft.com/unittest/mockimage:v*"))
			Expect(kubeProxyImages.ContainerdKubeProxyImages.ContainerImages[0].Amd64OnlyVersions).To(HaveLen(1))
			Expect(kubeProxyImages.ContainerdKubeProxyImages.ContainerImages[0].Amd64OnlyVersions[0]).To(Equal("test.000000.0"))
			Expect(kubeProxyImages.ContainerdKubeProxyImages.ContainerImages[0].MultiArchVersions).To(HaveLen(1))
			Expect(kubeProxyImages.ContainerdKubeProxyImages.ContainerImages[0].MultiArchVersions[0]).To(Equal("test.000000.1"))
		})

		It("should return correct components when one of them is nil", func() {
			// prepare valid json file
			testcomponent, err := os.CreateTemp("", "valid.json")
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(testcomponent.Name())
			// write MockKubeProxyImage to test file
			err = os.WriteFile(testcomponent.Name(), []byte(MockKubeProxyImageBarebone), 0644)
			Expect(err).ToNot(HaveOccurred())

			kubeProxyImages, err := NewKubeProxyImagesFromFile(testcomponent.Name())
			Expect(err).ToNot(HaveOccurred())

			// verify ContainerImages
			Expect(kubeProxyImages.DockerKubeProxyImages).To(BeNil())
			Expect(kubeProxyImages.ContainerdKubeProxyImages).NotTo(BeNil())
			Expect(kubeProxyImages.ContainerdKubeProxyImages.ContainerImages).To(HaveLen(1))
			Expect(kubeProxyImages.ContainerdKubeProxyImages.ContainerImages[0].DownloadURL).To(Equal("mcr.microsoft.com/unittest/mockimage:v*"))
			Expect(kubeProxyImages.ContainerdKubeProxyImages.ContainerImages[0].Amd64OnlyVersions).To(HaveLen(0))
			Expect(kubeProxyImages.ContainerdKubeProxyImages.ContainerImages[0].MultiArchVersions).To(HaveLen(1))
			Expect(kubeProxyImages.ContainerdKubeProxyImages.ContainerImages[0].MultiArchVersions[0]).To(Equal("test.000000.1"))
		})
	})

	Context("test ToImageList", func() {
		It("should return correct image list", func() {
			// prepare valid json file
			testcomponent, err := os.CreateTemp("", "valid.json")
			Expect(err).ToNot(HaveOccurred())
			defer os.Remove(testcomponent.Name())
			// write MockKubeProxyImage to test file
			err = os.WriteFile(testcomponent.Name(), []byte(MockKubeProxyImage), 0644)
			Expect(err).ToNot(HaveOccurred())

			kubeProxyImages, err := NewKubeProxyImagesFromFile(testcomponent.Name())
			Expect(err).ToNot(HaveOccurred())

			imageList, err := kubeProxyImages.ToImageList()
			Expect(imageList).To(HaveLen(4))
			Expect(imageList[0]).To(Equal("mcr.microsoft.com/unittest/mockimage:vtest.111111.0"))
			Expect(imageList[1]).To(Equal("mcr.microsoft.com/unittest/mockimage:vtest.111111.1"))
			Expect(imageList[2]).To(Equal("mcr.microsoft.com/unittest/mockimage:vtest.000000.0"))
			Expect(imageList[3]).To(Equal("mcr.microsoft.com/unittest/mockimage:vtest.000000.1"))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
