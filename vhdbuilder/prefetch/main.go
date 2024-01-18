package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Azure/agentbaker/vhdbuilder/prefetch/pkg/container"
)

func main() {
	var (
		// program args
		componentListPath                string
		containerImagePrefetchScriptPath string
	)
	flag.StringVar(&componentListPath, "components", "", "path to the component list JSON file.")
	flag.StringVar(&containerImagePrefetchScriptPath, "container-image-prefetch-script", "", "where to place the newly generated container image prefetch script.")
	flag.Parse()

	if componentListPath == "" {
		fmt.Println("path to the component list must be specified")
		os.Exit(1)
	}
	if containerImagePrefetchScriptPath == "" {
		fmt.Println("CNI prefetch script destination path must be specified")
		os.Exit(1)
	}

	components, err := container.ParseComponents(componentListPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err = container.Generate(components, containerImagePrefetchScriptPath); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
