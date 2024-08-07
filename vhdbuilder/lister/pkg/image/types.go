package image

import (
	"encoding/json"
	"fmt"
)

type List struct {
	SKU     string   `json:"sku"`
	Version string   `json:"imageVersion"`
	BOM     []*Image `json:"imageBom"`
}

type Image struct {
	ID      string
	Bytes   int64
	Tags    map[string]struct{}
	Digests map[string]struct{}
}

func New() *Image {
	return &Image{
		Tags:    map[string]struct{}{},
		Digests: map[string]struct{}{},
	}
}

func (i *Image) SetID(id string) error {
	if i.ID != "" && i.ID != id {
		return fmt.Errorf("found multiple IDs for the same container image: %s and %s", i.ID, id)
	}
	if i.ID == "" {
		i.ID = id
	}
	return nil
}

func (i *Image) SetByteSize(bytes int64) error {
	if i.Bytes != 0 && i.Bytes != bytes {
		return fmt.Errorf("found mismatching byte sizes for the same container image: (%d, %d)", i.Bytes, bytes)
	}
	if i.Bytes == 0 {
		i.Bytes = bytes
	}
	return nil
}

func (i *Image) AddTag(tag string) {
	if i.Tags != nil {
		i.Tags[tag] = struct{}{}
	}
}

func (i *Image) AddDigest(digest string) {
	if i.Digests != nil {
		i.Digests[digest] = struct{}{}
	}
}

func (i *Image) MarshalJSON() ([]byte, error) {
	toMarshal := struct {
		ID          string   `json:"id"`
		Bytes       int64    `json:"bytes"`
		RepoTags    []string `json:"repoTags"`
		RepoDigests []string `json:"repoDigests"`
	}{
		ID:          i.ID,
		Bytes:       i.Bytes,
		RepoTags:    stringSetToSlice(i.Tags),
		RepoDigests: stringSetToSlice(i.Digests),
	}
	return json.Marshal(toMarshal)
}

func stringSetToSlice(set map[string]struct{}) []string {
	var res []string
	for element := range set {
		res = append(res, element)
	}
	return res
}
