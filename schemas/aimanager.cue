package aimanager

// Schema for the AKS AIManager custom-VHD content manifests under
// vhdbuilder/release-notes/AKSAIManager/<model-slug>/manifest.json
//
// One file per model. Each file declares every (gpu, os) variant
// of the custom VHD to build for that model. The internal KAITO custom-VHD capture
// pipeline consumes these files and bakes the content into Azure Compute Gallery
// image versions.
//
// Validate with:
//   cue vet -c ./schemas/aimanager.cue ./vhdbuilder/release-notes/AKSAIManager/<model>/manifest.json

// Model-level facts shared by every variant (the weights are identical across GPU/OS).
#Model: {
	slug:              string // short model id
	hfRepo:            string // HuggingFace repo, e.g. "deepseek-ai/DeepSeek-V4-Flash"
	hfRevision:        string // pin to a commit SHA for durable/releasable images
	gated:             bool   // if true, HF_TOKEN is supplied to the BUILD only, never baked
	weightsSizeGiB:    number & >0 // on-disk size of the baked weights; the consumer derives the OS/pool disk size from this (weights + base VHD + CUDA + headroom)
	bakePath:          string // weights bake dir; must be KAITO's location /opt/kaito/models/<hfRepo lowercased, '/' and '\' replaced by '-'>
}

// A build-time dependency baked into the VHD for a variant. The list may be empty
// for models that need nothing beyond their weights (e.g. mxfp4 models with no CUDA toolkit).
#Dependency: {
	name:            string // OS package name, e.g. "cuda-toolkit-12-9"
	version:         string // version identifier / pin, e.g. "0.12.0"
	repoKeyringUrl?: string // optional: repo keyring .deb URL to enable the package's apt source
	bakePath:        string // absolute path inside the VHD where the package is baked for KAITO
}

// The vanilla AKS VHD lineage a variant overlays. Consumers select the version.
#BaseImage: {
	offer: string // e.g. "AKSUbuntu"
	sku:   string // e.g. "2404gen2containerd"
}

#Variant: {
	gpuSku:       string // e.g. "A100", "H100"
	osSku:        string // e.g. "Ubuntu2404", "AzureLinux3.0"
	baseImage:    #BaseImage
	dependencies: [...#Dependency]
}

#Manifest: {
	model:    #Model
	variants: [...#Variant]
}

#Manifest
