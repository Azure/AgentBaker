package image

type List struct {
	SKU     string  `json:"sku"`
	Version string  `json:"imageVersion"`
	BOM     []Image `json:"imageBom"`
}

type Image struct {
	ID          string   `json:"id"`
	Bytes       int64    `json:"bytes"`
	RepoTags    []string `json:"repoTags"`
	RepoDigests []string `json:"repoDigests"`
}
