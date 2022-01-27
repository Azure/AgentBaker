// this manifest drives component versions installed during vhd build + cse
// this file is similar in nature to components.json, but allows broader customization per component
// it also inverts the key order to make specific components more easily patchable via automation (kubelet, containerd)
// it's effectively json, but written using cuelang for schema validation
// export it to json with cue export manifest.cue

#dep: {
	fileName:         string
	downloadLocation: string
	downloadURL:      string
	versions: [...string]
	installedVersion: string
}

containerd: #dep & {
	"fileName":         "moby-containerd_${CONTAINERD_VERSION}+azure-${CONTAINERD_PATCH_VERSION}.deb"
	"downloadURL":      "https://moby.blob.core.windows.net/moby/moby-containerd/${CONTAINERD_VERSION}+azure/bionic/linux_${CPU_ARCH}/moby-containerd_${CONTAINERD_VERSION}+azure-${CONTAINERD_PATCH_VERSION}_${CPU_ARCH}.deb"
	"downloadLocation": "/opt/containerd/downloads"
	"versions": [
		"1.4.9-3",
		"1.4.12-2",
	]
	"installedVersion": "1.5.9-2"
}

runc: #dep & {
	"fileName":         "moby-runc_${RUNC_VERSION}+azure-${RUNC_PATCH_VERSION}.deb"
	"downloadURL":      "https://moby.blob.core.windows.net/moby/moby-runc/${RUNC_VERSION}+azure/bionic/linux_${CPU_ARCH}/moby-runc_${RUNC_VERSION}+azure-${RUNC_PATCH_VERSION}_${CPU_ARCH}.deb"
	"downloadLocation": "/opt/runc/downloads"
	"versions": [
		"1.0.0-rc92",
		"1.0.0-rc95",
	]
	"installedVersion": "1.0.3"
}
