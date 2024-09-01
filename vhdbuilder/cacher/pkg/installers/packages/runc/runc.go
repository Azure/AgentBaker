package runc

import (
	"fmt"

	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/env"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/common"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/installers/packages/getter"
	"github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/model"
)

var _ getter.Getter = (*g)(nil)

type g struct{}

func Getter() getter.Getter {
	return &g{}
}

func (g *g) Get(pkg *model.Package) error {
	if env.IsMariner() {
		// mariner runc is already included in the base image or containerd installation
		return nil
	}
	uri := common.GetRelevantDownloadURI(pkg)
	if len(uri.Versions) > 1 {
		return fmt.Errorf("found multiple runc versions to install, expected only 1")
	}
	version := uri.Versions[0]
	if version == "1.0.0-rc92" || version == "1.0.0-rc95" {
		// only moby-runc-1.0.3+azure-1 exists in ARM64 ubuntu repo now, no 1.0.0-rc92 or 1.0.0-rc95
		return nil
	}
	return common.AptGetInstall(fmt.Sprintf("moby-runc=%s", version))
}
