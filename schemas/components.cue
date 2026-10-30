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
	// windowsDownloadRequiresAzCopy indicates windowsDownloadURL points at a private/authenticated
	// blob store location that must be fetched with AzCopy using the VHD builder's managed identity,
	// rather than the default unauthenticated curl-based download. Defaults to false (rather than
	// being a plain optional bool) so the conditional constraint below can reference it directly -
	// components.json entries that don't set this field at all are unaffected.
	windowsDownloadRequiresAzCopy: *false | bool

	// This path is MSI-only: no SAS tokens or other query-string credentials are supported. Reject
	// them here, at schema-validation time (make validate-components / the validate-components CI
	// check), rather than relying solely on the matching runtime check in
	// GetAzCopyDownloadUrlsFromComponentsJson - that runtime check only guards the actual VHD build,
	// not e.g. the check-windows-packages-change.yml workflow, which posts a public PR comment
	// showing resolved URLs for every PR and would otherwise disclose a committed secret before a
	// VHD is ever built.
	if windowsDownloadRequiresAzCopy == true {
		windowsDownloadURL?: =~"^[^?]*$"
		downloadURL?:        =~"^[^?]*$"
	}
}

#UbuntuOSDistro: {
	current?: #ReleaseDownloadURI
	r2004?:   #ReleaseDownloadURI
	r2204?:   #ReleaseDownloadURI
	r2404?:   #ReleaseDownloadURI
	r2604?:   #ReleaseDownloadURI
}

#DefaultOSDistro: {
	current?: #ReleaseDownloadURI
}

#MarinerOSDistro: {
	current?: #ReleaseDownloadURI
}

#AzureLinuxOSDistro: {
	"v3.0"?:          #ReleaseDownloadURI
	"DEFAULT/v3.0"?:  #ReleaseDownloadURI
	"OSGUARD/v3.0"?:  #ReleaseDownloadURI
	current?:         #ReleaseDownloadURI
}

#WindowsOsDistro: {
	default?: #ReleaseDownloadURI
	ws2022?: #ReleaseDownloadURI
	ws2025?: #ReleaseDownloadURI
}

#FlatcarOSDistro: {
	current?: #ReleaseDownloadURI
}

#AzureContainerLinuxOSDistro: {
	current?: #ReleaseDownloadURI
}

#DownloadURIs: {
	default?:          #DefaultOSDistro
	ubuntu?:           #UbuntuOSDistro
	mariner?:          #MarinerOSDistro
	marinerkata?:      #MarinerOSDistro
	azurelinux?:       #AzureLinuxOSDistro
	azurelinuxkata?:   #AzureLinuxOSDistro
	windows?:          #WindowsOsDistro
	flatcar?:          #FlatcarOSDistro
	azurecontainerlinux?: #AzureContainerLinuxOSDistro
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
