// this manifest drives component versions installed during vhd build + cse
// this file is similar in nature to components.json, but allows broader customization per component
// it also inverts the key order to make specific components more easily patchable via automation (kubelet, containerd)
// it's effectively json, but written using cuelang for schema validation
// export it to json with cue export manifest.cue

#URL: {
    amd64:  string
    arm64?: string
}

#override: {
    version?:         string // if specified, then a particular version will be downloaded/installed with apt
    downloadURL?:    #URL // if specified, this implies we need to download from somewhere separately instead of installing directly via apt
    privateStorage?: bool // denotes whether we need to fetch from a private storage account using azcopy
}

#overrides: {
    ubuntu1804?:     #override
    ubuntu2004?: #override
    ubuntu2204?: #override
    mariner?:        #override
    azurelinux?:     #override
}

// some basic json constraints for validation
#dep: {
	fileName?:         string
	downloadLocation: string
	downloadURL?:      string
	versions:         [...string]
    installed?:       {...}
    overrides?:       #overrides
    ...
}

#containerd_version_tuple: {
    edge: string
    stable: string
    latest: string
}

// semver with a revision e.g. 1.4.12-2
#containerd_ver: =~"[0-9]+.[0-9]+.[0-9]+-[0-9]+"

// containerd includes constraints from #dep and tighter bounds on version
#containerd: #dep & {
	versions: [...#containerd_ver]
    edge: #containerd_ver
}

#runc_ver: =~"[0-9]+.[0-9]+.[0-9]+-(rc)?[0-9]+" // rc92,rc95 previously used.

#runc: #dep & {
	versions: [...#runc_ver]
}

#kubernetes_ver: =~"[0-9]+.[0-9]+.[0-9]+(-hotfix.[0-9]{8})"

#kubernetes: #dep & {
	versions: [...#kubernetes_ver]
}

// root object schema enforced against manifest.json
#root: {
	runc:                       #runc
	containerd:                 #containerd
	[string]:                   #dep
}

// enforces validation of root on this object.
#root & {
    "containerd": {
        "fileName": "moby-containerd_${CONTAINERD_VERSION}+azure-${CONTAINERD_PATCH_VERSION}.deb",
        "downloadLocation": "/opt/containerd/downloads",
        "downloadURL": "https://moby.blob.core.windows.net/moby/moby-containerd/${CONTAINERD_VERSION}+azure/${UBUNTU_CODENAME}/linux_${CPU_ARCH}/moby-containerd_${CONTAINERD_VERSION}+azure-ubuntu${UBUNTU_RELEASE}u${CONTAINERD_PATCH_VERSION}_${CPU_ARCH}.deb",
        "versions": [],
        "pinned": {
            "1804": "1.7.1-1" // default in 1804 vhds.
        },
        "edge": "1.7.7-1",  // edge is default in vhd.
    },
    "runc": {
        "downloadLocation": "/opt/runc/downloads",
        "versions": [],
        "installed": {
			"default": "1.1.12",
            "1804": "1.1.12",
		},
        "overrides": {
            "ubuntu1804": {
                "downloadURL": {
                    "amd64": "https://mobyreleases.blob.core.windows.net/moby-private/moby-runc/1.1.7+azure/bionic-aks/linux_amd64/moby-runc_1.1.7+aks-ubuntu18.04u3_amd64.deb",
                    "arm64": "https://mobyreleases.blob.core.windows.net/moby-private/moby-runc/1.1.7+azure/bionic-aks/linux_arm64/moby-runc_1.1.7+aks-ubuntu18.04u3_arm64.deb",
                },
                "privateStorage": true,
            },
            "ubuntu2204": {
                "downloadURL": {
                   "amd64": "https://mobyreleases.blob.core.windows.net/moby-private/moby-runc/1.1.9+azure/jammy/linux_amd64/moby-runc_1.1.9-ubuntu22.04u2_amd64.deb",
                   "arm64": "https://mobyreleases.blob.core.windows.net/moby-private/moby-runc/1.1.9+azure/jammy/linux_arm64/moby-runc_1.1.9-ubuntu22.04u2_arm64.deb",
                },
                "privateStorage": true,
            }
        }
    },
    "nvidia-container-runtime": {
        "fileName": "",
        "downloadLocation": "",
        "downloadURL": "",
        "versions": [],
    },
    "nvidia-drivers": {
        "fileName": "",
        "downloadLocation": "",
        "downloadURL": "",
        "versions": [],
    },
    "kubernetes": {
        "fileName": "kubernetes-node-linux-arch.tar.gz",
        "downloadLocation": "",
        "downloadURL": "https://acs-mirror.azureedge.net/kubernetes/v${PATCHED_KUBE_BINARY_VERSION}/binaries/kubernetes-node-linux-${CPU_ARCH}.tar.gz",
        "versions": [
            "1.26.6",
            "1.26.10",
            "1.26.12",
            "1.27.3",
            "1.27.7",
            "1.27.9",
            "1.28.1",
            "1.28.3",
            "1.28.5",
            "1.29.0",
            "1.29.2"
        ]
    },
    "_template": {
        "fileName": "",
        "downloadLocation": "",
        "downloadURL": "",
        "versions": [],
    }
}
