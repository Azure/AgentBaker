package main

import (
	"flag"
	"log"
	"os"

	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/config"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/exec"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/containerimage"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/model"
)

var (
	cfg = &config.Config{}
)

func parseFlags() {
	flag.StringVar(&cfg.ComponentsPath, "components-path", "", "Path to the components file.")
	flag.BoolVar(&cfg.Dryrun, "dry-run", false, "Enable dry-run mode, where no bash commands will actually be executed.")
	flag.IntVar(&cfg.ImagePullParallelism, "image-pull-parallelism", 1, "Max parallelism to be used while pulling container images.")
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
	err := cfg.Validate()
	handle(err)

	if cfg.Dryrun {
		exec.UseFakeBackend()
	}

	components, err := model.LoadComponents(cfg.ComponentsPath)
	handle(err)

	imageInstaller, err := containerimage.NewContainerdInstaller(&containerimage.InstallerConfig{
		Parallelism: cfg.ImagePullParallelism,
	})
	handle(err)

	err = imageInstaller.Install(components.ContainerImages)
	handle(err)

	packageInstaller, err := packages.NewInstaller(nil)
	handle(err)

	err = packageInstaller.Install(components.Packages)
	handle(err)
}
