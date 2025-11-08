package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Azure/agentbaker/vhdbuilder/prefetch/internal/components"
	"github.com/Azure/agentbaker/vhdbuilder/prefetch/internal/containerimage"
)

type options struct {
	componentsPath string
	outputPath     string
}

func (o options) validate() error {
	if o.componentsPath == "" {
		return fmt.Errorf("path to the component list must be specified")
	}
	if o.outputPath == "" {
		return fmt.Errorf("output path must be specified")
	}
	return nil
}

var (
	opts options
)

func parseFlags() {
	flag.StringVar(&opts.componentsPath, "components-path", "", "path to the component list JSON file.")
	flag.StringVar(&opts.outputPath, "output-path", "", "where to place the newly generated container image prefetch script.")
	flag.Parse()
}

func main() {
	parseFlags()
	if err := opts.validate(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	list, err := components.ParseList(opts.componentsPath)
	if err != nil {
		fmt.Printf("parsing components json: %s", err)
		os.Exit(1)
	}

	content, err := containerimage.GeneratePrefetchScript(list)
	if err != nil {
		fmt.Printf("generating container prefetch script content: %s", err)
		os.Exit(1)
	}

	if err := os.WriteFile(opts.outputPath, content, os.ModePerm); err != nil {
		fmt.Printf("writing container image prefetch script to output path %s: %s", opts.outputPath, err)
		os.Exit(1)
	}

	fmt.Printf("generated container image prefetch script at %s:\n%s\n", opts.outputPath, string(content))
}
