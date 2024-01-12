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
		componentList     string
		cniPrefetchScript string
	)
	flag.StringVar(&componentList, "components", "", "path to the component list JSON file.")
	flag.StringVar(&cniPrefetchScript, "cni-prefetch-script", "", "where to place the newly generated CNI prefetch script")
	flag.Parse()

	if componentList == "" {
		fmt.Println("path to the component list must be specified")
		os.Exit(1)
	}
	if cniPrefetchScript == "" {
		fmt.Println("CNI prefetch script destination path must be specified")
		os.Exit(1)
	}

	components, err := component.ParseList(componentList)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err = cni.Generate(components, cniPrefetchScript); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
