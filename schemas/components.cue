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
#Packages: [...#Package]
#ReleaseDownloadURI: {
	versions:     [...string]
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
}

#Components