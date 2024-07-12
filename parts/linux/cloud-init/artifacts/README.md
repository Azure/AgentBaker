# Components management
While we are working on centralizing the components (container Images, packages, and other dependencies) in a single place (`components.json` in this folder), there are still some versions logic scattering around different places such as manifest.json and some scripts. 

**Note: we will keep updating this document once more progress are made.**

## Adding/updating a component in components.json
Now there are 2 types of component in components.json, namely `ContainerImages` and `Packages`.
- `ContainerImages` are container images that will be cached during VHD build time and will run at node provisioning time. The container Images URL are mostly mcr as of now June 2024.
- `Packages` are packages that could be downloaded through apt-get (Ubuntu), http file download URL or dnf (Mariner). Additional methods could be added in the future.

### ContainerImages
Please refer to examples of ContainerImages in components.json.

### Packages
`Packages` is a list of `Package` where a `package` has the following scehma:
``` 
#Package: {
	name:              string
	downloadLocation:  string
	downloadURIs:      #DownloadURIs
}
```

```
#DownloadURIs: {
	default?: #DefaultOSDistro
	ubuntu?:  #UbuntuOSDistro
	mariner?: #MarinerOSDistro
}
```

```
#DefaultOSDistro: {
	current?: #ReleaseDownloadURI
}
#UbuntuOSDistro: {
	current?: #ReleaseDownloadURI
	r1804?:   #ReleaseDownloadURI
	r2004?:   #ReleaseDownloadURI
	r2204?:   #ReleaseDownloadURI
}
#MarinerOSDistro: {
	current?: #ReleaseDownloadURI
}
```

```
#ReleaseDownloadURI: {
	versions:     [...string]
	downloadURL:  string
}
```
Here are the explanation of the above schema.
1. A `Package` consists of `name`, `downloadLocation` and a struct of downloadURI entries `downloadURIs`.
1. In `downloadURIs`, we can define different OS distro. For now for Linux, we have _Ubuntu_ and _Mariner_.
1. There are 3 types of OSDistro
    - In `UbuntuOSDistro`, we can define different OS release versions. For example, `r1804` implies release 18.04.
    - In `MarinerOSDistro`, we only have `current` now, which implies that single configurations will be applied to all Mariner release versions. We can distinguish them in needed.
    - `DefaultOSDistro` means the default case of OS Distro. If an OSDistro metadata is not defined, it will fetch it from `DefaultOSDistro`. For example, if a node is Ubuntu 20.04, but we don't specify `UbuntuOSDistro`, then it will fetch `DefaultOSDistro.current`. For another example, if only `DefaultOSDistro.current` is specified in the components.json, No matter what OSDistro is in the node, it will only fetch `DefaultOSDistro.current` because it's the default metadata. This provides flexibility while elimiate unnecessary duplicate when defining the metadata.

## Components schema

To be added.