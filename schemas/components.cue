package components

#ContainerImagePrefetchOptimization: {
	version: string
	binaries: [...string]
}

#ContainerImage: {
	downloadURL: string
	amd64OnlyVersions: [...string]
	multiArchVersions: [...]
	prefetchOptimizations: [...#ContainerImagePrefetchOptimization]
}

#DownloadFile: {
	fileName:         string
	downloadLocation: string
	downloadURL:      string
	versions: [...string]
	targetContainerRuntime?: "containerd" | _|_
}

#Images: [...#ContainerImage]
#Files: [...#DownloadFile]

#Components: {
	ContainerImages: #Images
	DownloadFiles:   #Files
}

#Components
