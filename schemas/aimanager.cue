package aimanager

import "strings"

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
	hfRevision:        string & =~"^[0-9a-f]{40}$" // pinned commit SHA for durable/releasable images
	gated:             bool   // if true, the model needs a HF_TOKEN to pull from HuggingFace
	"_comment"?:       string // optional context for maintainers; ignored by consumers
	bakePath!:         "/opt/kaito/models/" + strings.Replace(strings.ToLower(hfRepo), "/", "-", -1) // path inside the VHD where the model weights are baked
}

// A build-time dependency baked into the VHD for a variant. The list may be empty
// for models that need nothing beyond their weights (e.g. mxfp4 models with no CUDA toolkit).
#Dependency: {
	name:                  string // OS package name, e.g. "cuda-toolkit-12-9"
	version:               string // exact package version pin, e.g. "12.9.2-1"
	"_comment"?:           string // optional context for maintainers; ignored by consumers
	repoKeyringUrl?:       string & =~"^https://" // repo keyring .deb URL to enable the package's apt source
	repoKeyringSha256?:    string & =~"^[0-9a-f]{64}$" // SHA-256 that consumers should verify before installing the downloaded keyring
	bakePath:              string & =~"^/" // absolute path inside the VHD where the package is baked for KAITO
	if repoKeyringUrl != _|_ {
		repoKeyringSha256!: string & =~"^[0-9a-f]{64}$"
	}
	if repoKeyringSha256 != _|_ {
		repoKeyringUrl!: string
	}
}

#BaseImage: {
	Ubuntu2404:      {offer: "AKSUbuntu", sku: "2404gen2containerd"}
	"AzureLinux3.0": {offer: "azure-linux-3", sku: "V3gen2"}
}

#Variant: {
	gpuSku:       "A100" | "H100"
	osSku:        "Ubuntu2404" | "AzureLinux3.0"
	"_comment"?:  string // optional context for maintainers; ignored by consumers
	dependencies: [...#Dependency]
	baseImage:    #BaseImage[osSku]
}

#Manifest: {
	model:    #Model
	variants: [#Variant, ...#Variant]
}

#Manifest
