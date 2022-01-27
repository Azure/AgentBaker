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

#containerd_ver: =~"[0-9]+.[0-9]+.[0-9]+-[0-9]+"

#containerd: #dep & {
	versions: [...#containerd_ver]
}

#runc_ver: =~"[0-9]+.[0-9]+.[0-9]+-(rc)?[0-9]+" // rc92,rc95 previously used.

#runc: #dep & {
	versions: [...#runc_ver]
}

#root: {
	containerd: #containerd
	runc:       #runc
}

#root
