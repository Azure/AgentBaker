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

#DownloadURIs: {
	default?:          #DefaultOSDistro
	ubuntu?:           #UbuntuOSDistro
	mariner?:          #MarinerOSDistro
	marinerkata?:      #MarinerOSDistro
	azurelinux?:       #AzureLinuxOSDistro
	azurelinuxkata?:   #AzureLinuxOSDistro
	windows?:          #WindowsOsDistro
	flatcar?:          #FlatcarOSDistro
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

// The dedicated AMD image installs the kernel driver, firmware and diagnostics. Keeping
// this metadata outside Packages prevents generic images from caching it.
#AMDGPUDriver: {
	repositoryURL:          string & =~"^https://repo[.]radeon[.]com/amdgpu/[0-9.]+/ubuntu$"
	signingKeyURL:          string & =~"^https://repo[.]radeon[.]com/[^?[:space:]]+$"
	signingKeyFingerprint: string & =~"^[A-F0-9]{40}$"
	distribution:          "noble"
	component:             "main"
	packageVersion:        string & =~"^[0-9]+:[0-9][0-9A-Za-z.+~-]+$"
	firmwarePackageVersion: string & =~"^[0-9]+:[0-9][0-9A-Za-z.+~-]+$"
	moduleVersion:         string & =~"^[0-9][0-9.]+$"
	dkmsVersion:           string & =~"^[0-9][0-9A-Za-z.+~-]+$"
}

#AMDGPUDiagnostics: {
	repositoryURL:          "https://stable.repo.amd.com/rocm/core/packages/ubuntu2404/"
	signingKeyURL:          "https://stable.repo.amd.com/rocm/gpg/packages.gpg"
	signingKeyFingerprint: string & =~"^[A-F0-9]{40}$"
	distribution:          "stable"
	component:             "main"
	amdsmiPackage:         string & =~"^amdrocm-amdsmi[0-9]+[.][0-9]+$"
	amdsmiVersion:         string & =~"^[0-9][0-9A-Za-z.+~-]+$"
	sysdepsPackage:        string & =~"^amdrocm-sysdeps[0-9]+[.][0-9]+$"
	sysdepsVersion:        string & =~"^[0-9][0-9A-Za-z.+~-]+$"
	cliPath:               string & =~"^/opt/rocm/core-[0-9]+[.][0-9]+/bin/amd-smi$"
}

#Components: {
	ContainerImages: #Images
	Packages:        #Packages
	GPUContainerImages?: #GPUImages
	OCIArtifacts?: #OCIArtifacts
	AMDGPUDriver?: #AMDGPUDriver
	AMDGPUDiagnostics?: #AMDGPUDiagnostics
}

#Components
