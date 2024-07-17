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
    "_template": {
        "fileName": "",
        "downloadLocation": "",
        "downloadURL": "",
        "versions": [],
    }
}
