package components

#ContainerImagePrefetchOptimization: {
	binaries: [...string]
}

#ContainerImagePrefetchOptimizations: {
	latestVersion:          #ContainerImagePrefetchOptimization
	previousLatestVersion?: #ContainerImagePrefetchOptimization
}

#ContainerImage: {
	downloadURL: string
	amd64OnlyVersions:     [...string]
	multiArchVersionsV2:   [...#VersionV2]
}

#GPUContainerImage: {
	downloadURL: string
	multiArchVersionsV2:   [...#VersionV2]
}

#Images: [...#ContainerImage]
#GPUContainerImages: [...#GPUContainerImage]
#Packages: [...#Package]
#VersionV2: {
	k8sVersion?:             string
	renovateTag?:            string
	latestVersion:           string
	previousLatestVersion?:  string
	containerImagePrefetch?: #ContainerImagePrefetchOptimizations
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
	r2404?:   #ReleaseDownloadURI
}

#DefaultOSDistro: {
	current?: #ReleaseDownloadURI
}

#MarinerOSDistro: {
	current?: #ReleaseDownloadURI
}

#AzureLinuxOSDistro: {
	"v3.0"?:  #ReleaseDownloadURI
	current?: #ReleaseDownloadURI
}

#DownloadURIs: {
	default?:      #DefaultOSDistro
	ubuntu?:       #UbuntuOSDistro
	mariner?:      #MarinerOSDistro
	marinerkata?:  #MarinerOSDistro
	azurelinux?:   #AzureLinuxOSDistro
}

#Package: {
	name:              string
	downloadLocation:  string
	downloadURIs:      #DownloadURIs
}

#Components: {
	ContainerImages: #Images
	Packages:        #Packages
	GPUContainerImages: #GPUContainerImage
}

#Components