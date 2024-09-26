`components.json` is a components management file that defines which components and versions should be cached during the Node image VHD build time. It also includes a `RenovateTag` for Renovate to understand how to monitor and update the components automatically.

# Table of Contents

- [TL;DR](#tldr)
- [Components management](#components-management)
  - [Schema of components.json](#schema-of-componentsjson)
    - [ContainerImages](#containerimages)
    - [Packages](#packages)
- [Hands-on guide and FAQ](#hands-on-guide-and-faq)
  - [How to ask Renovate to auto-update an existing component in `components.json` to a new version?](#how-to-ask-renovate-to-auto-update-an-existing-component-in-componentsjson-to-a-new-version)
  - [How to ask Renovate not to auto-update a component version?](#how-to-ask-renovate-to-auto-update-an-existing-component-in-componentsjson-to-a-new-version)
  - [How to keep 2 patch versions for a minor version?](#how-to-keep-2-patch-versions-for-a-minor-version)
  - [How to keep multiple minor versions?](#how-to-keep-multiple-minor-versions)
  - [Can I keep only 1 patch version](#can-i-keep-only-1-patch-version)
  - [Can I avoid repeating a single version for all OS distros/releases](#can-i-avoid-repeating-a-single-version-for-all-os-distrosreleases)
  - [What components are onboarded to Renovate for auto-update and what are not yet?](#what-components-are-onboarded-to-renovate-for-auto-update-and-what-are-not-yet)

# TL;DR
This doc explains the organization of `components.json`, and how Renovate uses it to automatically update components. If you want to onboard your component, which is already in components.json, to Renovate for automatic updates, please refer to [Readme-Renovate.md](../../../../.github/README-RENOVATE.md).

To skip the details and simply add a new component to be cached in the VHD, please go directly to [Hands-on guide and FAQ](#hands-on-guide-and-faq).

# Components management
The `components.json` file centralizes the management of all components needed for building weekly node image VHDs, while allowing Renovate to automatically update the components to the latest versions to prevent CVEs.

## Schema of components.json
The `components.json` file defines two types of components: `containerImages` and `packages`.
- `ContainerImages` are container images that will be cached during VHD build time and will run at node provisioning time. As of Sept 2024, The container Images in `components.json` are all hosted in MCR and MCR is the only registry enabled in the current Renovate configuration file `renovate.json`. If there is demand for other container images registry, it will be necessary to double check if it will just work.
- `Packages` are packages that could be downloaded through apt-get (Ubuntu), http file download URL or dnf (Mariner). Additional methods such as OCI MCR could be added in the future.

Please refer to [components.cue](../../../../schemas/components.cue) for the most update-to-date schema in case the schema in this doc is not current.


### ContainerImages
`ContainerImages` is a list of `ContainerImage` where a `ContainerImage` has the following schema:
```
#ContainerImage: {
	downloadURL: string
	amd64OnlyVersions:     [...string]
	multiArchVersionsV2:   [...#VersionV2]
}
```
```
#ContainerImagePrefetchOptimization: {
	binaries: [...string]
}

#ContainerImagePrefetchOptimizations: {
	latestVersion:          #ContainerImagePrefetchOptimization
	previousLatestVersion?: #ContainerImagePrefetchOptimization
}

#VersionV2: {
	k8sVersion?:             string
	renovateTag?:            string
	latestVersion:           string
	previousLatestVersion?:  string
	containerImagePrefetch?: #ContainerImagePrefetchOptimizations
}

```
`multiArchVersionsV2` is updated from `multiArchVersions` and is a list of `VersionV2`.
1. In `versionV2`, there are a few keys.
    - `k8sVersion`: This is used to distinguish if some component versions are specific to a particular k8s version. This could also be used to further improve the logic in the VHD build/provsioning process if needed.
	- `renovateTag`: This tag is specifically for Renovate custom manager regex use. There are 2 types of `renovateTag` corresponding to `containerImage` and `package`.
	  - containerImage: `"renovateTag": "registry=<registry URL>, name=<container image full path>"`, where `<registry URL>` is always mcr.microsoft.com for now, and `<container image full path>` is path after registryURL without leading slash `/`. For example, `oss/kubernetes/windows-gmsa-webhook`,
	  - package: `"renovateTag": "name=<package name>, os=<OS distro>, release=<release version>"`, where `<package name>` is the name of the component presented in the registry list, `<OS distro>` is the distro name, `<release version>` is the distro release version.
	- Things to note:
	  - `renovateTag` must be exactly one line before `latestVersion` and the optional `previousLatestVersion`. `Renovate.json` requires this tag to parse the versions correctly.
	  - If you add anything other than the 2 types mentioned above, it won't be monitered by the current configurations of `renovate.json`. For example, you might see `"renovateTag": "<DO_NOT_UPDATE>"` which is actually equivalent to not having any `renovateTag`. Placing `"<DO_NOT_UPDATE>"` here is simply for human readability, but we still recommend including it for consistency and readability.
	- `latestVersion` and `previousLatestVersion`: to keep the last 2 patch versions in the components.json as well as VHD and keep them auto-updated by Renovate, we will put the latest version in `latestVersion` and the previous latest version `previousLatestVersion`.
  - `containerImagePrefetch` defines the prefetch optimization for the particular container image, if any. Each `ContainerImagePrefetchOptimizations` object must define a prefetch optimization _at least_ for the `latestVersion`, while optionally defining one of the `previousLatestVersion`. At the end of the day, a prefetch optimization is parameterized by an array of file paths pointing to binaries (relative to the FS of the container image, starting with `/`) to be prefetched during image builder optimization. NOTE: if ever updating prefetch optimizations for container images, please run `make generate` within vhdbuilder/prefetch to update the corresponding test data and re-run prefetch script generation tests.

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
	default?:      #DefaultOSDistro
	ubuntu?:       #UbuntuOSDistro
	mariner?:      #MarinerOSDistro
	marinerkata?:  #MarinerOSDistro
	azurelinux?:   #AzureLinuxOSDistro
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
	r2404?:   #ReleaseDownloadURI
}
#MarinerOSDistro: {
	current?: #ReleaseDownloadURI
}
#AzureLinuxOSDistro: {
	current?: #ReleaseDownloadURI
}
```

```
#VersionV2: {
	k8sVersion?:            string
	renovateTag?:           string
	latestVersion:          string
	previousLatestVersion?: string
}
```

```
#ReleaseDownloadURI: {
	versionsV2:   [...#VersionV2]
	downloadURL?:  string
}
```
Here are the explanation of the above schema.
1. A `Package` consists of `name`, `downloadLocation` and a struct of downloadURI entries `downloadURIs`.
1. In `downloadURIs`, we can define different OS distro. For now for Linux, we have _ubuntu_, _mariner_, _marinerkata_ and _default_.
1. There are 3 types of OSDistro
    - In `UbuntuOSDistro`, we can define different OS release versions. For example, `r1804` implies release 18.04.
    - In `MarinerOSDistro`, we only have `current` now, which implies that single configurations will be applied to all Mariner release versions. We can distinguish them in needed.
    - `DefaultOSDistro` means the default case of OS Distro. If an OSDistro metadata is not defined, it will fetch it from `default`. For example, if a node is Ubuntu 20.04, but we don't specify `ubuntu` in components.json, then it will fetch `default.current`. For another example, if only `default.current` is specified in the components.json, No matter what OSDistro is the node running, it will only fetch `default.current` because it's the default metadata. This provides flexibility while elimiating unnecessary duplication when defining the metadata.
1. In `ReleaseDownloadURI`, you can see 2 keys.
    - `versionsV2`: This is updated from `versions`. You can define a list of `VersionV2` for a particular package. And in the codes, it's up to the feature developer to determine how to use the list. For example, install all versions in the list or just pick the latest one. Note that in package `containerd`, `marinerkata`, the `versionV2s` array is defined as `<SKIP>`. This is to tell the install-dependencies.sh not to install any `containerd` version for Kata SKU.
	- `downloadURL`: you can define a downloadURL with unresolved variables. For example, `https://acs-mirror.azureedge.net/azure-cni/v${version}/binaries/azure-vnet-cni-linux-${CPU_ARCH}-v${version}.tgz`. But the feature developer needs to make sure all variables are resolvable in the codes. In this example, `${CPU_ARCH}` is resolvable as it's defined at global scope. `${version}` is resovled based on the `versions` list above.
1. `VersionV2`: explained in the previous section [ContainerImages](#containerimages)

# Hands-on guide and FAQ
> **Alert:** Before starting the hands-on guide, please take a moment to read [TL;DR](#tldr) section to ensure you are reading the correct doc.

## How to ask Renovate to auto-update an existing component in `components.json` to a new version?
Many of the components are already tagged as auto-update. However, there are still some components requested not to be updated for some reasons.
For example, for package `containerd` in Ubuntu 18.04, we have
```
        "r1804": {
            "versionsV2": [
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "1.7.1-1"
              }
            ]
          }
```
which means it wants to pin to that version. In this case, Renovate will not find this `latestVersion` because of the unknown renovateTag. Therefore it won't try to update this version either. 

Now if you want Renovate to monitor and update this version, you can modify the renovateTag with the `name`, `os` and `release` information. Please make sure you follow the order and format, even `,` and space. Renovate uses regex to parse the content so it's not very smart to handle unexpected characters.

## How to ask Renovate not to auto-update a component version?
In Renovate.json, we define that it uses `renovateTag` to get the metadata and parse package information to monitor and auto-update it. If a `renovateTag` is unknown to Renovate, it won't monitor that component. Therefore, If you use any renovateTag other than the 2 types mentioned in the previous section [ContainerImages](#containerimages), Renovate will not monitor it. In 1 sentence, put `"renovateTag": "<DO_NOT_UPDATE>"` before the line of the version and it won't be monitored. You can find other examples in components.json

## How to keep 2 patch versions for a minor version?
Follow this example by placing `previousLatestVersion` in the `versionsV2` or `multiArchVersionsV2` block, depending on whether you are adding a `containerImage` or a `package`.
```
        {
          "renovateTag": "registry=https://mcr.microsoft.com, name=containernetworking/azure-cns",
          "latestVersion": "v1.5.35",
          "previousLatestVersion": "v1.5.32"
        },
```

## How to keep multiple minor versions?
Please note that each minor version can only have 2 patch versions at most, which are `latestVersion` and `previousLatestVersion`. You can have only 1 version `latestVersion` for sure. Here is an example of a `containerImage` azure-cns that has multiple minor versions.

```
   {
      "downloadURL": "mcr.microsoft.com/containernetworking/azure-cns:*",
      "amd64OnlyVersions": [],
      "multiArchVersionsV2": [
        {
          "renovateTag": "registry=https://mcr.microsoft.com, name=containernetworking/azure-cns",
          "latestVersion": "v1.4.52"
        },
        {
          "renovateTag": "registry=https://mcr.microsoft.com, name=containernetworking/azure-cns",
          "latestVersion": "v1.5.35",
          "previousLatestVersion": "v1.5.32"
        },
        {
          "renovateTag": "registry=https://mcr.microsoft.com, name=containernetworking/azure-cns",
          "latestVersion": "v1.6.5",
          "previousLatestVersion": "v1.6.0"
        }
	  ]
   }
```

For a `package`, you will need to add these under `versionsV2`.

## Can I keep only 1 patch version?
Yes. Just place the latest version of the component in `latestVersion`. `previousLatestVersion` is optional.

## Can I avoid repeating a single version for all OS distros/releases?
It depends.
For a `containerImage`, you don't need to distinguish among distros and releases.
- If you are adding a `package` to Ubuntu and you want it to be monitored by Renovate, you will need to separate them into different releases. The reason behind is that each release is actually using its own PMC registry (with different URL) to host the packages. Renovate doesn't provide a custom variable for us to extract that variable to abstract the custom datasources. So far we still need to separate them into `r1804`, `r2004`, `r2204` and `r2404` unless there is a better way or Renovate supports a new custom variable to store that information.
- If you don't need the component to be monitored by Renovate, you can place it in `current` for all releases, and in `default` for all OS distros. Here is an example for azure-cni.
```
    {
      "name": "azure-cni",
      "downloadLocation": "/opt/cni/downloads",
      "downloadURIs": {
        "default": {
          "current": {
            "versionsV2": [
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "1.4.54"
              },
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "1.5.32",
                "previousLatestVersion": "1.5.35"
              },
              {
                "renovateTag": "<DO_NOT_UPDATE>",
                "latestVersion": "1.6.3"
              }
            ],
            "downloadURL": "https://acs-mirror.azureedge.net/azure-cni/v${version}/binaries/azure-vnet-cni-linux-${CPU_ARCH}-v${version}.tgz"
          }
        }
      }
    }
```
In this example the versions are defined under `default.current`, meaning that for all OS distros and releases, it caches the same versions. To learn more about the this, please read the section [Schema of components.json](#schema-of-componentsjson)

## What components are onboarded to Renovate for auto-update and what are not yet?
please refer to [Readme-Renovate.md](../../../../.github/README-RENOVATE.md#what-components-are-onboarded-to-renovate-for-auto-update-and-what-are-not-yet)