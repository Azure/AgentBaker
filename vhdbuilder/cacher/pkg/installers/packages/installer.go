package packages

import (
	"fmt"

	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/azurecni"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/cni"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/critools"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/getter"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/model"
)

type InstallerConfig struct {
}

type Installer struct {
	getters map[string]getter.Getter
}

func NewInstaller(cfg *InstallerConfig) (*Installer, error) {
	return &Installer{
		getters: map[string]getter.Getter{
			"cni-plugins": cni.Getter(),
			"azure-cni":   azurecni.Getter(),
			"cri-tools":   critools.Getter(),
		},
	}, nil
}

func (i *Installer) Install(packages []*model.Package) error {
	for _, pkg := range packages {
		g, ok := i.getters[pkg.Name]
		if !ok {
			return fmt.Errorf("no installer found for package %q", pkg.Name)
		}
		if err := g.Get(pkg); err != nil {
			return fmt.Errorf("installing package %q: %w", pkg.Name, err)
		}
	}
	return nil
}
