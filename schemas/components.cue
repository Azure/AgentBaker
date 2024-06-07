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

#Images: [...#ContainerImage]
#Files: [...#DownloadFile]
#Packages: [...#Package]
#PackageUri: {
	versions:     [...string]
	downloadURL: string
}

#OSDistro: {
	current: #PackageUri,
	1804?:   #PackageUri //1804 is optional
}

#DownloadUriEntries: {
	default: #OSDistro
	ubuntu?:  #OSDistro
	mariner?: #OSDistro
}

#Package: {
	name:                    string
	downloadLocation:        string
	downloadURIs:      #DownloadURIs
	targetContainerRuntime?: "containerd" | _|_ //this line defines an optional field named targetContainerRuntime that can either be the string "containerd" or any other value, including the absence of a value.
}

#Components: {
	ContainerImages: #Images
	Packages:        #Packages    
}

#Components
