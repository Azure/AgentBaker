package artifact

import adobuild "github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"

// VHDPublishingInfo represents VHD configuration as parsed from arbitrary
// vhd-publishing-info.json files produced by VHD builds
type VHDPublishingInfo struct {
	CapturedImageVersionResourceID string `json:"captured_sig_resource_id,omitempty"`
	SKUName                        string `json:"sku_name,omitempty"`
	OfferName                      string `json:"offer_name,omitempty"`
}

// PublishingInfoDownloadOpts represents options used to download
// publishing info artifacts for a given VHD build
type PublishingInfoDownloadOpts struct {
	Artifacts map[string]bool
	TargetDir string
	BuildID   int
}

// Downloader provides an API to download publishing info artifacts
// from VHD builds to use within AgentBaker E2E tests
type Downloader struct {
	basicAuth   string
	buildClient adobuild.Client

	errChan  chan error
	doneChan chan struct{}
}
