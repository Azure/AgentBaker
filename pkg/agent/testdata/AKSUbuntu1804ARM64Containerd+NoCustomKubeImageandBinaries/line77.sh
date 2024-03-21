{
    "containerd": {
        "fileName": "moby-containerd_${CONTAINERD_VERSION}+azure-${CONTAINERD_PATCH_VERSION}.deb",
        "downloadLocation": "/opt/containerd/downloads",
        "downloadURL": "https://moby.blob.core.windows.net/moby/moby-containerd/${CONTAINERD_VERSION}+azure/${UBUNTU_CODENAME}/linux_${CPU_ARCH}/moby-containerd_${CONTAINERD_VERSION}+azure-ubuntu${UBUNTU_RELEASE}u${CONTAINERD_PATCH_VERSION}_${CPU_ARCH}.deb",
        "versions": [],
        "pinned": {
            "1804": "1.7.1-1"
        },
        "edge": "1.7.14-1"
    },
    "runc": {
        "fileName": "moby-runc_${RUNC_VERSION}+azure-ubuntu${RUNC_PATCH_VERSION}_${CPU_ARCH}.deb",
        "downloadLocation": "/opt/runc/downloads",
        "downloadURL": "https://moby.blob.core.windows.net/moby/moby-runc/${RUNC_VERSION}+azure/bionic/linux_${CPU_ARCH}/moby-runc_${RUNC_VERSION}+azure-ubuntu${RUNC_PATCH_VERSION}_${CPU_ARCH}.deb",
        "versions": [],
        "pinned": {
            "1804": "1.1.12"
        },
        "installed": {
            "default": "1.1.12"
        }
    },
    "nvidia-container-runtime": {
        "fileName": "",
        "downloadLocation": "",
        "downloadURL": "",
        "versions": []
    },
    "nvidia-drivers": {
        "fileName": "",
        "downloadLocation": "",
        "downloadURL": "",
        "versions": []
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
        "versions": []
    }
}
#EOF
