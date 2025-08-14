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
	windowsDownloadURL?: string
	amd64OnlyVersions:     [...string]
	multiArchVersionsV2:   [...#VersionV2]
	windowsVersions?:   [...#WindowsVersion]
}

#GPUContainerImage: {
	downloadURL: string
	gpuVersion:   #VersionV2
}

#WindowsVersion: {
	comment?:                string
	k8sVersion?:             string
	renovateTag?:            string
	latestVersion:           string
	previousLatestVersion?:  string
	windowsSkuMatch?:        string
}

#Images: [...#ContainerImage]
#GPUImages: [...#GPUContainerImage]
#Packages: [...#Package]
#OCIArtifacts: [...#OCIArtifact]
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
	windowsDownloadURL?: string
}

#UbuntuOSDistro: {
	current?: #ReleaseDownloadURI
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

#WindowsOsDistro: {
	default?: #ReleaseDownloadURI
	ws2019?: #ReleaseDownloadURI
	ws2022?: #ReleaseDownloadURI
	ws23h2?: #ReleaseDownloadURI
	ws2025?: #ReleaseDownloadURI
}

#DownloadURIs: {
	default?:          #DefaultOSDistro
	ubuntu?:           #UbuntuOSDistro
	mariner?:          #MarinerOSDistro
	marinerkata?:      #MarinerOSDistro
	azurelinux?:       #AzureLinuxOSDistro
	azurelinuxkata?:   #AzureLinuxOSDistro
	windows?:          #WindowsOsDistro
}

#Package: {
	name:              string
	downloadLocation?:  string
	windowsDownloadLocation?:  string
	downloadURIs:      #DownloadURIs
}

#OCIArtifact: {
	name: string
	registry: string
	windowsDownloadLocation?: string
	windowsVersions: [...#WindowsVersion]
}

#Components: {
	ContainerImages: #Images
	Packages:        #Packages
	GPUContainerImages?: #GPUImages
	OCIArtifacts?: #OCIArtifacts
}

#Components