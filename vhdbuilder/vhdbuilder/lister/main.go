package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Azure/agentbaker/vhdbuilder/lister/pkg/image"
)

type options struct {
	sku              string
	nodeImageVersion string
	outputPath       string
}

func (o *options) validate() error {
	if o.sku == "" {
		return fmt.Errorf("sku must be specified")
	}
	if o.nodeImageVersion == "" {
		return fmt.Errorf("node-image-version must be specified")
	}
	if o.outputPath == "" {
		return fmt.Errorf("output-path must be specified")
	}
	return nil
}

var (
	opts = &options{}
)

func parseFlags() {
	flag.StringVar(&opts.sku, "sku", "", "the VHD's SKU")
	flag.StringVar(&opts.nodeImageVersion, "node-image-version", "", "the VHD's node image version")
	flag.StringVar(&opts.outputPath, "output-path", "", "where to store the generated image list")
	flag.Parse()
}

func main() {
	parseFlags()
	if err := opts.validate(); err != nil {
		log.Printf("unable to validate command line options: %s", err)
		os.Exit(1)
	}

	imageList, err := image.ListImages(opts.sku, opts.nodeImageVersion)
	if err != nil {
		log.Printf("unable to list images: %s", err)
		os.Exit(1)
	}

	raw, err := json.MarshalIndent(imageList, "", "  ")
	if err != nil {
		log.Printf("unable to marshal generated image list: %s", err)
		os.Exit(1)
	}

	if err := os.WriteFile(opts.outputPath, raw, os.ModePerm); err != nil {
		log.Printf("unable to write generated image list content to file %s: %s", opts.outputPath, err)
		os.Exit(1)
	}
}
