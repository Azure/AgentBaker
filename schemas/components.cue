package components

#ContainerImagePrefetchOptimization: {
	version:  string
	binaries: [...string]
}

#ContainerImage: {
	downloadURL: string
	amd64OnlyVersions:     [...string]
	multiArchVersionsV2:   [...#VersionV2]
	prefetchOptimizations: [...#ContainerImagePrefetchOptimization]
}

#Images: [...#ContainerImage]
#Packages: [...#Package]
#VersionV2: {
	k8sVersion?:            string
	renovateTag?:           string
	latestVersion:          string
	previousLatestVersion?: string
}

#ReleaseDownloadURI: {
	versionsV2:   [...#VersionV2]
	downloadURL?:  string
}

#UbuntuOSDistro: {
	current?: #ReleaseDownloadURI
	r1804?:   #ReleaseDownloadURI
	r2004?:   #ReleaseDownloadURI
	r2204?:   #ReleaseDownloadURI
}

#DefaultOSDistro: {
	current?: #ReleaseDownloadURI
}

#MarinerOSDistro: {
	current?: #ReleaseDownloadURI
}

#DownloadURIs: {
	default?: #DefaultOSDistro
	ubuntu?:  #UbuntuOSDistro
	mariner?: #MarinerOSDistro
}

#Package: {
	name:              string
	downloadLocation:  string
	downloadURIs:      #DownloadURIs
}

#Components: {
	ContainerImages: #Images
	Packages:        #Packages    
}

#Components