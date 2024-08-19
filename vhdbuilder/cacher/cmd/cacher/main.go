package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/containerimage"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/model"
)

type options struct {
	componentsPath string
	dryrun         bool
}

func (o *options) validate() error {
	if o.componentsPath == "" {
		return fmt.Errorf("components-path must be specified")
	}
	return nil
}

var (
	opts = &options{}
)

func parseFlags() {
	flag.StringVar(&opts.componentsPath, "components-path", "", "Path to the components file.")
	flag.BoolVar(&opts.dryrun, "dry-run", false, "Enable dry-run mode, where no bash commands will actually be executed.")
	flag.Parse()
}

func handle(err error) {
	if err != nil {
		log.Printf("%s", err)
		os.Exit(1)
	}
}

func main() {
	parseFlags()
	err := opts.validate()
	handle(err)

	components, err := model.LoadComponents(opts.componentsPath)
	handle(err)

	installer, err := containerimage.NewDockerInstaller(&containerimage.InstallerConfig{
		Parallelism: 2,
	})
	handle(err)

	err = installer.Install(components.ContainerImages)
	handle(err)
}
