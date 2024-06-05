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
#Packages: [...#Package]
#PackageUri: {
	version:     string
	downloadURL: string
}

#OSDistro: {
	current: #PackageUri,
	1804?:   #PackageUri //1804 is optional
}

#DownloadUriEntries: {
	ubuntu:  #OSDistro
	mariner: #OSDistro
}

#Package: {
	name:               string
	downloadLocation:   string
	downloadUriEntries: #DownloadUriEntries
}

#Components: {
	containerImages: #Images
	downloadFiles:   #Files
	binaries:        #Packages
}

#Components
