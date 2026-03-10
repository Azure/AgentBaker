package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestRemoteDescriptorSizeManifest(t *testing.T) {
	t.Parallel()

	manifestDesc := ocispec.Descriptor{
		Digest:    digest.FromString("manifest"),
		MediaType: ocispec.MediaTypeImageManifest,
		Size:      100,
	}

	fetcher := staticFetcher{
		manifestDesc.Digest: `{
			"schemaVersion": 2,
			"mediaType": "` + ocispec.MediaTypeImageManifest + `",
			"config": {"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:1111111111111111111111111111111111111111111111111111111111111111","size":50},
			"layers": [
				{"mediaType":"application/vnd.oci.image.layer.v1.tar+gzip","digest":"sha256:2222222222222222222222222222222222222222222222222222222222222222","size":10},
				{"mediaType":"application/vnd.oci.image.layer.v1.tar+gzip","digest":"sha256:3333333333333333333333333333333333333333333333333333333333333333","size":20}
			]
		}`,
	}

	size, err := remoteDescriptorSize(context.Background(), fetcher, manifestDesc, nil)
	if err != nil {
		t.Fatalf("remoteDescriptorSize() error = %v", err)
	}

	if want := int64(180); size != want {
		t.Fatalf("remoteDescriptorSize() = %d, want %d", size, want)
	}
}

func TestRemoteDescriptorSizeIndexSelectsPlatformManifest(t *testing.T) {
	t.Parallel()

	amd64Manifest := ocispec.Descriptor{
		Digest:    digest.FromString("manifest-amd64"),
		MediaType: images.MediaTypeDockerSchema2Manifest,
	}
	arm64Manifest := ocispec.Descriptor{
		Digest:    digest.FromString("manifest-arm64"),
		MediaType: images.MediaTypeDockerSchema2Manifest,
	}
	indexDesc := ocispec.Descriptor{
		Digest:    digest.FromString("index"),
		MediaType: ocispec.MediaTypeImageIndex,
		Size:      80,
	}

	fetcher := staticFetcher{
		indexDesc.Digest: `{
			"schemaVersion": 2,
			"mediaType": "` + ocispec.MediaTypeImageIndex + `",
			"manifests": [
				{"mediaType":"` + arm64Manifest.MediaType + `","digest":"` + arm64Manifest.Digest.String() + `","size":220,"platform":{"os":"linux","architecture":"arm64"}},
				{"mediaType":"` + amd64Manifest.MediaType + `","digest":"` + amd64Manifest.Digest.String() + `","size":120,"platform":{"os":"linux","architecture":"amd64"}}
			]
		}`,
		amd64Manifest.Digest: `{
			"schemaVersion": 2,
			"mediaType": "` + amd64Manifest.MediaType + `",
			"config": {"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":30},
			"layers": [
				{"mediaType":"application/vnd.oci.image.layer.v1.tar+gzip","digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","size":40}
			]
		}`,
		arm64Manifest.Digest: `{
			"schemaVersion": 2,
			"mediaType": "` + arm64Manifest.MediaType + `",
			"config": {"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","size":300},
			"layers": [
				{"mediaType":"application/vnd.oci.image.layer.v1.tar+gzip","digest":"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","size":400}
			]
		}`,
	}

	size, err := remoteDescriptorSize(context.Background(), fetcher, indexDesc, platforms.OnlyStrict(platforms.MustParse("linux/amd64")))
	if err != nil {
		t.Fatalf("remoteDescriptorSize() error = %v", err)
	}

	if want := int64(270); size != want {
		t.Fatalf("remoteDescriptorSize() = %d, want %d", size, want)
	}
}

type staticFetcher map[digest.Digest]string

func (f staticFetcher) Fetch(_ context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	payload, ok := f[desc.Digest]
	if !ok {
		return nil, io.EOF
	}
	return io.NopCloser(strings.NewReader(payload)), nil
}

var _ remotes.Fetcher = staticFetcher(nil)
