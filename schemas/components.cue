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
		Version:     string
		DownloadURL: string
	}

#OSDistro: {
	"Current": #PackageUri,
	"1804"?:   #PackageUri //1804 is optional
}

#DownloadUriEntries: {
	"Ubuntu":  #OSDistro
	"Mariner": #OSDistro
}

#Package: {
	Name:               string
	DownloadLocation:   string
	DownloadUriEntries: #DownloadUriEntries
}

#Components: {
	ContainerImages: #Images
	DownloadFiles:   #Files
	Binaries:        #Packages
}

#Components
