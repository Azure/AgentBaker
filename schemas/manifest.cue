// this manifest drives component versions installed during vhd build + cse
// this file is similar in nature to components.json, but allows broader customization per component
// it also inverts the key order to make specific components more easily patchable via automation (kubelet, containerd)
// it's effectively json, but written using cuelang for schema validation
// export it to json with cue export manifest.cue

// some basic json constraints for validation
#dep: {
	fileName:         string
	downloadLocation: string
	downloadURL:      string
	versions: [...string]
    installed?: {...}
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
    latest: #containerd_ver
    stable: #containerd_ver
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
        "versions": [
            "1.4.13-3",
            "1.6.4-4"
        ],
        "edge": "1.6.4-4",
        "latest": "1.5.11-2", // latest is default in vhd.
        "stable": "1.4.13-3", // we use latest vs stable at runtime depending on k8s version.
    },
    "runc": {
        "fileName": "moby-runc_${RUNC_VERSION}+azure-${RUNC_PATCH_VERSION}.deb",
        "downloadLocation": "/opt/runc/downloads",
        "downloadURL": "https://moby.blob.core.windows.net/moby/moby-runc/${RUNC_VERSION}+azure/bionic/linux_${CPU_ARCH}/moby-runc_${RUNC_VERSION}+azure-${RUNC_PATCH_VERSION}_${CPU_ARCH}.deb",
        "versions": [
            "1.0.0-rc92",
            "1.0.0-rc95"
        ],
        "installed": { 
			"default": "1.0.3"
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
        "downloadURL": "https://acs-mirror.azureedge.net/kubernetes/v${PATCHED_KUBE_BINARY_VERSION}/binaries/kubernetes-node-linux-${CPU_ARCH}.tar.gz"
        "versions": [ 
            "1.22.11-hotfix.20220620",
            "1.22.15",
            "1.23.8-hotfix.20220620",
            "1.23.12",
            "1.24.3",
            "1.24.6",
            "1.25.2-hotfix.20221006",
            "1.25.4",
            "1.26.0"
        ]
    },
    "_template": {
        "fileName": "",
        "downloadLocation": "",
        "downloadURL": "",
        "versions": [],
    }
}
