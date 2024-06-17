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

#kubernetes_ver: =~"[0-9]+.[0-9]+.[0-9]+(-hotfix.[0-9]{8})"

#kubernetes: #dep & {
	versions: [...#kubernetes_ver]
}

// root object schema enforced against manifest.json
#root: {
	[string]:                   #dep
}

// enforces validation of root on this object.
#root & {
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
            "1.27.9",
            "1.27.13",
            "1.27.14",
            "1.28.5",
            "1.28.9",
            "1.28.10",
            "1.29.2",
            "1.29.4",
            "1.29.5",
            "1.30.0",
        ]
    },
    "_template": {
        "fileName": "",
        "downloadLocation": "",
        "downloadURL": "",
        "versions": [],
    }
}
