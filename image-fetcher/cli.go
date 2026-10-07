package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/urfave/cli/v3"
)

func newCommand() *cli.Command {
	return &cli.Command{
		Name:  "image-fetcher",
		Usage: "Cache container images in containerd",
		Action: func(context.Context, *cli.Command) error {
			return errors.New("expected cache-image or cache-components command")
		},
		Commands: []*cli.Command{
			{
				Name:  "cache-image",
				Usage: "Cache one image",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "image", Required: true, Usage: "Image reference to cache"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.Args().Len() != 0 {
						return errors.New("cache-image accepts no positional arguments; use --image")
					}
					if strings.TrimSpace(cmd.String("image")) == "" {
						return errors.New("--image must not be empty")
					}
					return withContainerd(ctx, func(ctx context.Context, client *containerd.Client) error {
						ref := cmd.String("image")
						if err := fetchImage(ctx, client, ref); err != nil {
							return fmt.Errorf("%s: %w", ref, err)
						}
						return nil
					})
				},
			},
			{
				Name:  "pull-image",
				Usage: "Ensure one image is fully pulled and unpacked for CRI",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "image", Required: true, Usage: "Image reference to pull and unpack"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.Args().Len() != 0 {
						return errors.New("pull-image accepts no positional arguments; use --image")
					}
					if strings.TrimSpace(cmd.String("image")) == "" {
						return errors.New("--image must not be empty")
					}
					return withContainerd(ctx, func(ctx context.Context, client *containerd.Client) error {
						ref := cmd.String("image")
						result, err := pullAndUnpackImage(ctx, client, ref)
						if err != nil {
							return fmt.Errorf("%s: %w", ref, err)
						}
						return json.NewEncoder(os.Stdout).Encode(result)
					})
				},
			},
			{
				Name:  "cache-components",
				Usage: "Cache the Linux container images declared in components.json",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "components-file", Required: true, Usage: "Path to components.json"},
					&cli.IntFlag{Name: "concurrency", Value: 20, Usage: "Maximum concurrent images (including retries and unpacking)"},
					&cli.StringFlag{Name: "vhd-log-file", Usage: "Append successfully cached image references to this VHD inventory file"},
				},
				Action: cacheComponents,
			},
		},
	}
}

func withContainerd(ctx context.Context, action func(context.Context, *containerd.Client) error) error {
	socket := os.Getenv("CONTAINERD_SOCKET")
	if socket == "" {
		socket = defaultSocket
	}
	ns := os.Getenv("CONTAINERD_NAMESPACE")
	if ns == "" {
		ns = defaultNS
	}
	client, err := containerd.New(socket)
	if err != nil {
		return fmt.Errorf("connect to containerd at %s: %w", socket, err)
	}
	defer client.Close()
	return action(namespaces.WithNamespace(ctx, ns), client)
}

func cacheComponents(ctx context.Context, cmd *cli.Command) (result error) {
	if cmd.Args().Len() != 0 {
		return errors.New("cache-components accepts no positional arguments; use --components-file")
	}
	if strings.TrimSpace(cmd.String("components-file")) == "" {
		return errors.New("--components-file must not be empty")
	}
	concurrency := cmd.Int("concurrency")
	if concurrency < 1 {
		return errors.New("--concurrency must be at least 1")
	}
	path := cmd.String("components-file")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read components file %s: %w", path, err)
	}
	refs, err := resolveImageReferences(data, runtime.GOARCH)
	if err != nil {
		return fmt.Errorf("resolve images from %s: %w", path, err)
	}

	var inventory io.Writer = io.Discard
	if path := cmd.String("vhd-log-file"); path != "" {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open VHD inventory %s: %w", path, err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				result = errors.Join(result, fmt.Errorf("close VHD inventory %s: %w", path, err))
			}
		}()
		inventory = file
	}

	var inventoryMu sync.Mutex
	writeEntry := func(ref string) error {
		inventoryMu.Lock()
		defer inventoryMu.Unlock()
		if _, err := fmt.Fprintf(inventory, "  - %s\n", ref); err != nil {
			return fmt.Errorf("write VHD inventory for %s: %w", ref, err)
		}
		return nil
	}

	fmt.Printf("Caching %d images with concurrency %d\n", len(refs), concurrency)
	if len(refs) == 0 {
		return ctx.Err()
	}
	return withContainerd(ctx, func(ctx context.Context, client *containerd.Client) error {
		return cacheImages(ctx, client, refs, concurrency, writeEntry)
	})
}
