package packages

import (
	"fmt"

	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/azurecni"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/cni"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/critools"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/getter"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/k8sbinaries"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/oras"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/model"
)

type Installer struct {
	getters map[string]getter.Getter
}

func NewInstaller() (*Installer, error) {
	return &Installer{
		getters: map[string]getter.Getter{
			"oras":                oras.Getter(),
			"cni-plugins":         cni.Getter(),
			"azure-cni":           azurecni.Getter(),
			"cri-tools":           critools.Getter(),
			"kubernetes-binaries": k8sbinaries.Getter(),
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
