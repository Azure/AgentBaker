package main

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	imagesapi "github.com/containerd/containerd/api/services/images/v1"
	leasesapi "github.com/containerd/containerd/api/services/leases/v1"
	transferapi "github.com/containerd/containerd/api/services/transfer/v1"
	"github.com/containerd/containerd/api/types"
	transfertypes "github.com/containerd/containerd/api/types/transfer"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type transferFixture struct {
	transferapi.UnimplementedTransferServer
	imagesapi.UnimplementedImagesServer
	requests chan *transferapi.TransferRequest
	fail     bool
}

type leaseFixture struct {
	leasesapi.UnimplementedLeasesServer
}

func (leaseFixture) Create(context.Context, *leasesapi.CreateRequest) (*leasesapi.CreateResponse, error) {
	return &leasesapi.CreateResponse{Lease: &leasesapi.Lease{ID: "fetch-test"}}, nil
}

func (leaseFixture) Delete(context.Context, *leasesapi.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (f *transferFixture) Transfer(_ context.Context, r *transferapi.TransferRequest) (*emptypb.Empty, error) {
	f.requests <- r
	if f.fail {
		return nil, status.Error(codes.Unavailable, "transfer failed")
	}
	return &emptypb.Empty{}, nil
}

func (f *transferFixture) Get(_ context.Context, r *imagesapi.GetImageRequest) (*imagesapi.GetImageResponse, error) {
	return &imagesapi.GetImageResponse{Image: &imagesapi.Image{
		Name: r.Name,
		Target: &types.Descriptor{
			MediaType: ocispec.MediaTypeImageManifest,
			Digest:    digest.FromString("manifest").String(),
			Size:      10,
		},
	}}, nil
}

func TestFetchOnlyTransfer(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "transfer error"}[fail], func(t *testing.T) {
			socket := filepath.Join(t.TempDir(), "containerd.sock")
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			fixture := &transferFixture{requests: make(chan *transferapi.TransferRequest, 1), fail: fail}
			server := grpc.NewServer()
			transferapi.RegisterTransferServer(server, fixture)
			imagesapi.RegisterImagesServer(server, fixture)
			leasesapi.RegisterLeasesServer(server, leaseFixture{})
			done := make(chan error, 1)
			go func() { done <- server.Serve(listener) }()
			t.Cleanup(func() {
				server.Stop()
				if err := <-done; err != nil {
					t.Error(err)
				}
			})
			client, err := containerd.New(socket)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			platform := platforms.DefaultSpec()
			ctx, cancel := context.WithTimeout(namespaces.WithNamespace(t.Context(), "k8s.io"), 10*time.Second)
			defer cancel()
			image, err := transferImage(ctx, client, "registry.example/image:tag", platform)
			if fail {
				if err == nil || !strings.Contains(err.Error(), "transfer failed") {
					t.Fatalf("transfer error lost: %v", err)
				}
			} else if err != nil || image.Name() != "registry.example/image:tag" {
				t.Fatalf("unexpected transferred image: %v", err)
			}
			var r *transferapi.TransferRequest
			select {
			case r = <-fixture.requests:
			case <-ctx.Done():
				t.Fatal("server transfer was not invoked")
			}
			var destination transfertypes.ImageStore
			var source transfertypes.OCIRegistry
			if err := r.Destination.UnmarshalTo(&destination); err != nil {
				t.Fatal(err)
			}
			if err := r.Source.UnmarshalTo(&source); err != nil {
				t.Fatal(err)
			}
			if len(destination.Unpacks) != 0 || len(destination.Platforms) != 1 ||
				destination.Platforms[0].Architecture != platform.Architecture ||
				destination.Labels["io.cri-containerd.image"] != "managed" ||
				source.GetResolver().GetHostDir() != "/etc/containerd/certs.d" {
				t.Fatalf("incorrect fetch-only transfer: destination=%v source=%v", &destination, &source)
			}
		})
	}
}
