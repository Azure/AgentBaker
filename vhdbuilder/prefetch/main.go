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
		componentsPath string
		outputPath     string
	)
	flag.StringVar(&componentsPath, "components-path", "", "path to the component list JSON file.")
	flag.StringVar(&outputPath, "output-path", "", "where to place the newly generated container image prefetch script.")
	flag.Parse()

	if componentsPath == "" {
		fmt.Println("path to the component list must be specified")
		os.Exit(1)
	}
	if outputPath == "" {
		fmt.Println("CNI prefetch script destination path must be specified")
		os.Exit(1)
	}

	components, err := container.ParseComponents(componentsPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err = container.Generate(components, outputPath); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
