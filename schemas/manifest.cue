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
    stable: string
    latest: string
}

// semver with a revision e.g. 1.4.12-2
#containerd_ver: =~"[0-9]+.[0-9]+.[0-9]+-[0-9]+"

// containerd includes constraints from #dep and tighter bounds on version
#containerd: #dep & {
	versions: [...#containerd_ver]
    latest: #containerd_ver  // latest is default in vhd.
    stable: #containerd_ver // we use latest vs stable at runtime depending on k8s version.
}

#runc_ver: =~"[0-9]+.[0-9]+.[0-9]+-(rc)?[0-9]+" // rc92,rc95 previously used.

#runc: #dep & {
	versions: [...#runc_ver]
}

// root object schema enforced against manifest.json
#root: {
	runc:                       #runc
	containerd:                 #containerd
	[string]:                   #dep
}

// enforces validation of root on this object.
#root
