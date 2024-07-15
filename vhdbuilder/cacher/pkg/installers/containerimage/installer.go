package containerimage

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/env"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/exec"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/model"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"k8s.io/apimachinery/pkg/util/errors"
)

type Installer struct {
	template string
}

func NewInstaller(cliTool string) (*Installer, error) {
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
	}, nil
}

func (i *Installer) Install(image *model.ContainerImage) error {
	tags := image.MultiArchTags
	if !env.IsARM() {
		tags = append(tags, image.AMD64OnlyTags...)
	}
	return i.pullImagesInParallel(image.Repo, tags)
}

func (i *Installer) pullImagesInParallel(repo string, tags []string) error {
	var pullers []func() error
	for _, tag := range tags {
		pullers = append(pullers, i.getPuller(repo, tag))
	}
	if err := errors.AggregateGoroutines(pullers...); err != nil {
		return err
	}
	return nil
}

func (i *Installer) getPuller(repo, tag string) func() error {
	return func() error {
		imageString := strings.ReplaceAll(repo, "*", tag)
		commandString := fmt.Sprintf(i.template, imageString)
		command, err := exec.NewCommand(commandString, &exec.CommandConfig{
			MaxRetries: 60,
			Wait:       to.Ptr(1 * time.Second),
			Timeout:    to.Ptr(1200 * time.Second),
		})
		if err != nil {
			return err
		}
		res, err := command.Execute()
		if err != nil {
			return err
		}
		log.Printf("pulled container image %q: %s", imageString, res)
		return nil
	}
}
