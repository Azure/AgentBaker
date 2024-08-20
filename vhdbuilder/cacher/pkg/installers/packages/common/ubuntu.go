package common

import (
	"fmt"
	"time"

	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/exec"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

func getAptUpdatePipeline() *exec.Pipeline {
	p := exec.NewPipeline(&exec.CommandConfig{
		MaxRetries: 10,
		Wait:       to.Ptr(5 * time.Second),
	})
	p.AddCommand("dpkg --configure -a --force-confdef")
	p.AddCommand("apt-get -o DPkg::Lock::Timeout=-1 -f -y install")
	p.AddCommand("apt-get -o DPkg::Lock::Timeout=-1 update")
	return p
}

func AptGetUpdate() error {
	p := getAptUpdatePipeline()
	res, err := p.Execute()
	if err != nil {
		return err
	}
	if err := res.AsError(); err != nil {
		return err
	}
	return nil
}

func AptGetInstall(aptPackage string) error {
	onFailure, err := getAptUpdatePipeline().AsSingleCommand()
	if err != nil {
		return err
	}
	p := exec.NewPipeline(&exec.CommandConfig{
		MaxRetries:         10,
		Wait:               to.Ptr(5 * time.Second),
		OnRetryableFailure: onFailure,
	})
	p.AddCommand("dpkg --configure -a --force-confdef")
	p.AddCommand(fmt.Sprintf(`apt-get install -o DPkg::Lock::Timeout=-1 -o Dpkg::Options::="--force-confold" --no-install-recommends -y %s`, aptPackage))
	res, err := p.Execute()
	if err != nil {
		return err
	}
	if err := res.AsError(); err != nil {
		return err
	}
	return nil
}
