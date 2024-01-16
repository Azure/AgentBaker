package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Azure/agentbaker/vhdbuilder/prefetch/pkg/cni"
	"github.com/Azure/agentbaker/vhdbuilder/prefetch/pkg/component"
)

func main() {
	var (
		// program args
		componentListPath     string
		cniPrefetchScriptPath string
	)
	flag.StringVar(&componentListPath, "components", "", "path to the component list JSON file.")
	flag.StringVar(&cniPrefetchScriptPath, "cni-prefetch-script", "", "where to place the newly generated CNI prefetch script")
	flag.Parse()

	if componentListPath == "" {
		fmt.Println("path to the component list must be specified")
		os.Exit(1)
	}
	if cniPrefetchScriptPath == "" {
		fmt.Println("CNI prefetch script destination path must be specified")
		os.Exit(1)
	}

	components, err := component.ParseList(componentListPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err = cni.Generate(components, cniPrefetchScriptPath); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
