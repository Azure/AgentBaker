package containerimage

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/env"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/exec"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/model"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"go.uber.org/multierr"
)

type InstallerConfig struct {
	Parallelism int
	Dryrun      bool
}

type Installer struct {
	template string
	cfg      *InstallerConfig
}

func NewContainerdInstaller(cfg *InstallerConfig) (*Installer, error) {
	return NewInstaller("ctr", cfg)
}

func NewDockerInstaller(cfg *InstallerConfig) (*Installer, error) {
	return NewInstaller("docker", cfg)
}

func NewInstaller(cliTool string, cfg *InstallerConfig) (*Installer, error) {
	var (
		template    string
		initCommand *exec.Command
		err         error
	)

	switch {
	case strings.EqualFold(cliTool, ctr):
		template = "ctr --namespace k8s.io image pull %s"
		initCommand, err = exec.NewCommand("ctr namespace create k8s.io", nil)
		if err != nil {
			return nil, fmt.Errorf("creating init command for %q container image installer: %w", cliTool, err)
		}
	case strings.EqualFold(cliTool, crictl):
		template = "crictl pull %s"
	case strings.EqualFold(cliTool, docker):
		template = "docker pull %s"
	default:
		return nil, fmt.Errorf("cannot create container image installer with unrecognized cli tool: %q", cliTool)
	}

	if initCommand != nil {
		res, err := initCommand.Execute()
		if err != nil {
			return nil, fmt.Errorf("executing init command for %q container image installer: %w", cliTool, err)
		}
		if err := res.AsError(); err != nil {
			return nil, fmt.Errorf("init command for container image installer failed: %w", err)
		}
		log.Printf("executed init command for container image installer: %s", res)
	}

	return &Installer{
		template: template,
		cfg:      cfg,
	}, nil
}

func (i *Installer) Install(images []*model.ContainerImage) error {
	var pullers []puller
	for _, image := range images {
		for _, tag := range image.MultiArchTags {
			pullers = append(pullers, i.getPuller(image.Repo, tag))
		}
		if env.IsAMD() {
			for _, tag := range image.AMD64OnlyTags {
				pullers = append(pullers, i.getPuller(image.Repo, tag))
			}
		}
	}
	return pullInParallel(pullers, i.cfg.Parallelism)
}

func (i *Installer) getPuller(repo, tag string) puller {
	return func() error {
		image := strings.ReplaceAll(repo, "*", tag)
		pull := fmt.Sprintf(i.template, image)
		cmd, err := exec.NewCommand(pull, &exec.CommandConfig{
			MaxRetries: 10,
			Wait:       to.Ptr(time.Second),
			Timeout:    to.Ptr(120 * time.Second),
		})
		if err != nil {
			return err
		}
		res, err := cmd.Execute()
		if err != nil {
			return err
		}
		if err := res.AsError(); err != nil {
			return err
		}
		log.Printf("pulled container image %q: %s", image, res)
		return nil
	}
}

type puller func() error

func pullInParallel(pullers []puller, maxParallelism int) error {
	guard := make(chan struct{}, maxParallelism)
	errs := make([]error, len(pullers))
	wg := sync.WaitGroup{}

	for idx, p := range pullers {
		guard <- struct{}{}
		wg.Add(1)
		go func(p puller, idx int) {
			defer wg.Done()
			errs[idx] = p()
			<-guard
		}(p, idx)
	}

	// wait for any outstanding pullers to complete
	wg.Wait()

	var merr error
	for _, err := range errs {
		if err != nil {
			merr = multierr.Append(merr, err)
		}
	}
	return merr
}
